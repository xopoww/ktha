package apps

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
)

type AppManager struct {
	controllers  map[string]*AppController
	shuttingDown bool
	mx           sync.Locker

	cfg AppManagerConfig
	log *zap.Logger
}

type AppManagerConfig struct {
	RunnerBinaryPath      string
	NodeBinaryPath        string
	RootfsRoot            string
	ImagesBasePath        string
	ReadinessPollInterval time.Duration
	ReadinessTimeout      time.Duration
	IdleTimeout           time.Duration
	StopTimeout           time.Duration
}

func NewAppManager(cfg AppManagerConfig, log *zap.Logger) *AppManager {
	return &AppManager{
		controllers: make(map[string]*AppController),
		mx:          &sync.Mutex{},

		cfg: cfg,
		log: log,
	}
}

type AppSpec struct {
	ID    string
	Image string
}

func (a *AppManager) AddApps(specs []AppSpec) error {
	a.mx.Lock()
	defer a.mx.Unlock()

	for _, spec := range specs {
		if err := a.addAppLocked(spec); err != nil {
			return fmt.Errorf("add app: %w", err)
		}
	}

	return nil
}

func (a *AppManager) addAppLocked(spec AppSpec) error {
	if _, ok := a.controllers[spec.ID]; ok {
		return fmt.Errorf("app %q already exists", spec.ID)
	}

	appCfg := AppControllerConfig{
		Image:                 filepath.Join(a.cfg.ImagesBasePath, spec.Image),
		RunnerBinaryPath:      a.cfg.RunnerBinaryPath,
		NodeBinaryPath:        a.cfg.NodeBinaryPath,
		RootfsRoot:            a.cfg.RootfsRoot,
		ReadinessPollInterval: a.cfg.ReadinessPollInterval,
		ReadinessTimeout:      a.cfg.ReadinessTimeout,
		IdleTimeout:           a.cfg.IdleTimeout,
		StopTimeout:           a.cfg.StopTimeout,
	}
	ac := NewAppController(spec.ID, appCfg, a.log)
	a.controllers[spec.ID] = ac

	return nil
}

func (a *AppManager) Shutdown() {
	a.mx.Lock()

	a.log.Sugar().Infof("Shutting down the manager...")
	a.shuttingDown = true

	wg := &sync.WaitGroup{}
	for id, ac := range a.controllers {
		wg.Go(func() {
			if status := ac.Status(); status != STARTING && status != RUNNING {
				return
			}
			if err := ac.Stop(); err != nil {
				a.log.Sugar().Warnf("Failed to stop the app %q: %s.", id, err)
			}
		})
	}

	a.mx.Unlock()

	wg.Wait()
}

var ErrAppNotFound = errors.New("app not found")

func (a *AppManager) DialApp(ctx context.Context, id string) (socket string, err error) {
	a.mx.Lock()

	if a.shuttingDown {
		a.mx.Unlock()
		return "", fmt.Errorf("manager is shutting down")
	}

	ac, ok := a.controllers[id]
	if !ok {
		return "", ErrAppNotFound
	}

	a.mx.Unlock()

	ensureRunning := make(chan error)
	go func() {
		ensureRunning <- ac.EnsureRunning()
	}()

	select {
	case err := <-ensureRunning:
		if err != nil {
			return "", fmt.Errorf("ensure running: %w", err)
		}
	case <-ctx.Done():
		return "", ctx.Err()
	}

	socket, err = ac.Socket()
	if err != nil {
		return "", fmt.Errorf("get socket: %w", err)
	}
	return socket, nil
}
