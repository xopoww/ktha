package manager

import (
	"context"
	"fmt"
	"net"
	"path/filepath"
	"sync"

	"github.com/xopoww/ktha/node/internal/config"
	"github.com/xopoww/ktha/node/internal/controller"
	"github.com/xopoww/ktha/node/internal/metrics"
	"go.uber.org/zap"
)

type AppManagerConfig struct {
	ImagesBasePath string

	Runner    config.RunnerConfig
	Limits    config.ContainerLimits
	Readiness config.ReadinessConfig
	Timeouts  config.AppTimeoutsConfig
}

type AppManager struct {
	cfg AppManagerConfig

	mx           sync.Locker
	controllers  map[string]*controller.AppController
	shuttingDown bool

	l *zap.SugaredLogger
}

func NewAppManager(cfg AppManagerConfig, l *zap.SugaredLogger) *AppManager {
	return &AppManager{
		cfg: cfg,

		controllers: make(map[string]*controller.AppController),
		mx:          &sync.Mutex{},

		l: l,
	}
}

type AppSpec struct {
	ID    string
	Image string
	Env   config.AppEnv
}

func (a *AppManager) AddApp(spec AppSpec) error {
	a.mx.Lock()
	defer a.mx.Unlock()
	return a.addAppLocked(spec)
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

func (a *AppManager) DeleteApp(id string) error {
	a.mx.Lock()

	if a.shuttingDown {
		a.mx.Unlock()
		return ErrManagerShuttingDown
	}

	ac, ok := a.controllers[id]
	if !ok {
		a.mx.Unlock()
		return ErrAppNotFound
	}
	delete(a.controllers, id)
	metrics.AppCount.Add(-1)
	a.mx.Unlock()

	ac.Stop()
	return nil
}

func (a *AppManager) addAppLocked(spec AppSpec) error {
	if a.shuttingDown {
		return ErrManagerShuttingDown
	}
	if _, ok := a.controllers[spec.ID]; ok {
		return ErrAppAlreadyExists
	}

	appCfg := controller.AppControllerConfig{
		ImagePath: filepath.Join(a.cfg.ImagesBasePath, spec.Image),
		Env:       spec.Env,
		Runner:    a.cfg.Runner,
		Limits:    a.cfg.Limits,
		Readiness: a.cfg.Readiness,
		Timeouts:  a.cfg.Timeouts,
	}
	ac, err := controller.NewAppController(spec.ID, appCfg, a.l)
	if err != nil {
		return fmt.Errorf("new controller: %w", err)
	}
	a.controllers[spec.ID] = ac

	metrics.AppCount.Add(1)

	return nil
}

func (a *AppManager) Shutdown() {
	a.mx.Lock()

	a.l.Info("Shutting down the manager...")
	a.shuttingDown = true

	wg := &sync.WaitGroup{}
	for _, ac := range a.controllers {
		wg.Go(func() {
			ac.Stop()
		})
	}

	a.mx.Unlock()

	wg.Wait()
}

func (a *AppManager) getAppController(id string) (*controller.AppController, error) {
	a.mx.Lock()
	defer a.mx.Unlock()

	if a.shuttingDown {
		return nil, ErrManagerShuttingDown
	}

	ac, ok := a.controllers[id]
	if !ok {
		return nil, ErrAppNotFound
	}

	return ac, nil
}

func (a *AppManager) DialApp(ctx context.Context, id string) (conn net.Conn, coldStart bool, err error) {
	ac, err := a.getAppController(id)
	if err != nil {
		return nil, false, err
	}
	return ac.Dial(ctx)
}

func (a *AppManager) UpgradeApp(id string, newImage string) error {
	ac, err := a.getAppController(id)
	if err != nil {
		return err
	}
	return ac.Upgrade(filepath.Join(a.cfg.ImagesBasePath, newImage))
}
