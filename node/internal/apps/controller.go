package apps

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type AppControllerConfig struct {
	Image string

	RunnerBinaryPath string
	NodeBinaryPath   string
	RootfsRoot       string

	ReadinessPollInterval time.Duration
	ReadinessTimeout      time.Duration
}

type appContainer struct {
	id   string
	cmd  *exec.Cmd
	down chan struct{}
}

type AppController struct {
	id     string
	status AppStatus
	mx     sync.Locker

	cfg AppControllerConfig
	log *zap.Logger

	container *appContainer
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
	a.status = STARTING
	a.log.Info("App is starting.")

	containerID := uuid.New().String()

	flags := []string{
		"--image", a.cfg.Image,
		"--id", containerID,
		"--node-bin", a.cfg.NodeBinaryPath,
		"--rootfs", a.cfg.RootfsRoot,
	}
	cmd := exec.Command(a.cfg.RunnerBinaryPath, flags...)

	a.log.Sugar().Debugf("Running '%s'...", cmd)
	if err := cmd.Start(); err != nil {
		a.status = STOPPED
		return fmt.Errorf("start: %w", err)
	}

	down := make(chan struct{})
	a.container = &appContainer{
		id:   containerID,
		cmd:  cmd,
		down: down,
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
				ready := a.checkReadiness()
				if ready {
					a.mx.Lock()
					defer a.mx.Unlock()
					a.log.Sugar().Info("App is running.")
					a.status = RUNNING
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

func (a *AppController) Stop(timeout time.Duration) error {
	var down chan struct{}
	if err := func() error {
		a.mx.Lock()
		defer a.mx.Unlock()

		if a.status != STARTING && a.status != RUNNING {
			return fmt.Errorf("cannot stop from %q", a.status)
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
	case <-time.After(timeout):
		a.log.Sugar().Warn("App stop timeout.")
		if err := a.kill(); err != nil {
			return fmt.Errorf("kill: %w", err)
		}
		return nil
	}
}
