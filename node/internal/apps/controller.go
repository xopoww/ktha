package apps

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/xopoww/ktha/node/internal/common"
	"go.uber.org/zap"
)

type AppControllerConfig struct {
	Image string

	RunnerBinaryPath string
	NodeBinaryPath   string
	RootfsRoot       string
	CgroupRoot       string

	Limits common.ContainerLimits

	ReadinessPollInterval time.Duration
	ReadinessTimeout      time.Duration
	IdleTimeout           time.Duration
	StopTimeout           time.Duration
}

type appContainer struct {
	id    string
	cmd   *exec.Cmd
	down  chan struct{}
	ready chan struct{}
}

type AppController struct {
	id     string
	status AppStatus
	mx     sync.Locker

	cfg AppControllerConfig
	log *zap.Logger

	container      *appContainer
	downscaleTimer *time.Timer
}

func NewAppController(id string, cfg AppControllerConfig, log *zap.Logger) *AppController {
	return &AppController{
		id:     id,
		status: STOPPED,
		mx:     &sync.Mutex{},
		cfg:    cfg,
		log:    log.With(zap.String("app", id)),
	}
}

func (a AppController) ID() string {
	return a.id
}

func (a AppController) Status() AppStatus {
	a.mx.Lock()
	defer a.mx.Unlock()
	return a.status
}

func (a *AppController) Start() error {
	a.mx.Lock()
	defer a.mx.Unlock()

	if a.status != STOPPED {
		return fmt.Errorf("cannot start from %q", a.status)
	}
	return a.startLocked()
}

func (a *AppController) startLocked() error {
	a.status = STARTING
	a.log.Info("App is starting.")

	containerID := uuid.New().String()

	flags := []string{
		"--image", a.cfg.Image,
		"--id", containerID,
		"--node-bin", a.cfg.NodeBinaryPath,
		"--rootfs", a.cfg.RootfsRoot,
		"--cgroup", a.cfg.CgroupRoot,
		"--mem-max", fmt.Sprint(a.cfg.Limits.MemoryMax),
		"--pids-max", fmt.Sprint(a.cfg.Limits.PidsMax),
		"--cpu-max", fmt.Sprint(a.cfg.Limits.CPUMax),
	}
	cmd := exec.Command(a.cfg.RunnerBinaryPath, flags...)

	a.log.Sugar().Debugf("Running '%s'...", cmd)
	if err := cmd.Start(); err != nil {
		a.status = STOPPED
		return fmt.Errorf("start: %w", err)
	}

	down := make(chan struct{})
	ready := make(chan struct{})
	a.container = &appContainer{
		id:    containerID,
		cmd:   cmd,
		down:  down,
		ready: ready,
	}

	// wait for exit
	go func() {
		err := cmd.Wait()
		a.mx.Lock()
		defer a.mx.Unlock()
		if err == nil {
			a.log.Sugar().Info("App stopped.")
			a.status = STOPPED
		} else {
			a.log.Sugar().Warnf("App crashed: %s.", err)
			a.status = DEAD
		}
		close(down)
		a.container = nil
	}()

	// poll for readiness
	go func() {
		timeout := time.After(a.cfg.ReadinessTimeout)
		ticker := time.NewTicker(a.cfg.ReadinessPollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				isReady := a.checkReadiness()
				if isReady {
					a.mx.Lock()
					defer a.mx.Unlock()

					a.log.Sugar().Info("App is running.")
					a.status = RUNNING
					close(ready)

					downscaleTimer := time.NewTimer(a.cfg.IdleTimeout)
					go func() {
						<-downscaleTimer.C
						a.log.Sugar().Info("Autoscaling the app down to zero...")
						if err := a.Stop(); err != nil {
							a.log.Sugar().Errorf("Failed to stop the app: %s", err)
						}
					}()
					a.downscaleTimer = downscaleTimer

					return
				}
			case <-timeout:
				a.log.Sugar().Warn("App readiness timeout.")
				if err := a.kill(); err != nil {
					a.log.Sugar().Error("Failed to kill the app after readiness timeout: %s.", err)
				}
				return
			case <-down:
				return
			}
		}
	}()

	return nil
}

func (a AppController) Socket() (string, error) {
	a.mx.Lock()
	defer a.mx.Unlock()
	if a.status != STARTING && a.status != RUNNING {
		return "", fmt.Errorf("invalid status: %q", a.status)
	}
	if a.container == nil {
		return "", fmt.Errorf("container in unexpectedly nil")
	}
	socket := filepath.Join(a.cfg.RootfsRoot, a.container.id, "app.sock")
	return socket, nil
}

var ErrAppIsDead = errors.New("app is dead")

func (a *AppController) EnsureRunning() error {
	var ready, down <-chan struct{}
	if err := func() error {
		a.mx.Lock()
		defer a.mx.Unlock()

		switch a.status {
		case RUNNING:
			a.log.Sugar().Debug("Reset the downscale timer.")
			a.downscaleTimer.Reset(a.cfg.IdleTimeout)
		case DEAD:
			return ErrAppIsDead
		case STOPPED:
			a.log.Sugar().Infof("Auto-scaling the app up from zero...")
			if err := a.startLocked(); err != nil {
				return fmt.Errorf("start: %w", err)
			}
		}

		// app is either STARTING or RUNNING at this point

		if a.container == nil {
			return fmt.Errorf("container is unexpectedly nil")
		}
		ready = a.container.ready
		down = a.container.down
		return nil
	}(); err != nil {
		return err
	}

	select {
	case <-ready:
		return nil
	case <-down:
		return ErrAppIsDead
	}
}

func (a AppController) checkReadiness() bool {
	socket, err := a.Socket()
	if err != nil {
		a.log.Sugar().Warnf("Failed to get socket for readiness check: %s.", err)
		return false
	}

	if err := func() error {
		client := &http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					return net.Dial("unix", socket)
				},
			},
		}
		rsp, err := client.Get("http://localhost/healthcheck")
		if err != nil {
			return fmt.Errorf("get: %w", err)
		}
		defer rsp.Body.Close()
		if rsp.StatusCode != http.StatusOK {
			return fmt.Errorf("get: status %s", rsp.Status)
		}
		// skip checking the body
		return nil
	}(); err != nil {
		a.log.Sugar().Debugf("Readiness check: app not ready (%s).", err)
		return false
	}
	return true
}

func (a *AppController) kill() error {
	a.mx.Lock()
	defer a.mx.Unlock()

	if a.container == nil {
		return nil
	}

	return a.container.cmd.Process.Kill()
}

func (a *AppController) Stop() error {
	var down chan struct{}
	if err := func() error {
		a.mx.Lock()
		defer a.mx.Unlock()

		if a.status != STARTING && a.status != RUNNING {
			return fmt.Errorf("cannot stop from %q", a.status)
		}

		if a.downscaleTimer != nil {
			a.downscaleTimer.Stop()
			a.downscaleTimer = nil
		}

		if a.container == nil {
			return fmt.Errorf("container is unexpectedly nil")
		}

		if err := a.container.cmd.Process.Signal(syscall.SIGTERM); err != nil {
			return fmt.Errorf("signal: %w", err)
		}

		down = a.container.down
		return nil
	}(); err != nil {
		return err
	}

	select {
	case <-down:
		return nil
	case <-time.After(a.cfg.StopTimeout):
		a.log.Sugar().Warn("App stop timeout.")
		if err := a.kill(); err != nil {
			return fmt.Errorf("kill: %w", err)
		}
		return nil
	}
}
