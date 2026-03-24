package apps

import (
	"errors"
	"fmt"
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
	ReadinessPollInterval time.Duration
	ReadinessTimeout      time.Duration
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
	Image string
}

// currently starts the app as well
func (a *AppManager) AddApp(id string, spec AppSpec) error {
	a.mx.Lock()

	if _, ok := a.controllers[id]; ok {
		a.mx.Unlock()
		return fmt.Errorf("app %q already exists", id)
	}

	appCfg := AppControllerConfig{
		Image:                 spec.Image,
		RunnerBinaryPath:      a.cfg.RunnerBinaryPath,
		NodeBinaryPath:        a.cfg.NodeBinaryPath,
		RootfsRoot:            a.cfg.RootfsRoot,
		ReadinessPollInterval: a.cfg.ReadinessPollInterval,
		ReadinessTimeout:      a.cfg.ReadinessTimeout,
	}
	ac := NewAppController(id, appCfg, a.log)
	a.controllers[id] = ac

	a.mx.Unlock()

	if err := ac.Start(); err != nil {
		return fmt.Errorf("start: %w", err)
	}

	return nil
}

func (a *AppManager) Shutdown(timeout time.Duration) {
	a.mx.Lock()

	a.log.Sugar().Infof("Shutting down the manager...")
	a.shuttingDown = true

	wg := &sync.WaitGroup{}
	for id, ac := range a.controllers {
		wg.Go(func() {
			if status := ac.Status(); status != STARTING && status != RUNNING {
				return
			}
			if err := ac.Stop(timeout); err != nil {
				a.log.Sugar().Warnf("Failed to stop the app %q: %s.", id, err)
			}
		})
	}

	a.mx.Unlock()

	wg.Wait()
}

var ErrAppNotFound = errors.New("app not found")
var ErrAppNotReady = errors.New("app not ready")

func (a *AppManager) DialApp(id string) (socket string, err error) {
	a.mx.Lock()
	defer a.mx.Unlock()

	if a.shuttingDown {
		return "", fmt.Errorf("manager is shutting down")
	}

	ac, ok := a.controllers[id]
	if !ok {
		return "", ErrAppNotFound
	}

	if status := ac.Status(); status != RUNNING {
		return "", fmt.Errorf("%w (%q)", ErrAppNotReady, status)
	}

	socket, err = ac.Socket()
	if err != nil {
		return "", fmt.Errorf("get socket: %w", err)
	}
	return socket, nil
}
