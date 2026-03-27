package controller

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/xopoww/ktha/node/internal/config"
	"github.com/xopoww/ktha/node/internal/container"
	"go.uber.org/zap"
)

type AppControllerConfig struct {
	ImagePath string

	Runner    config.RunnerConfig
	Limits    config.ContainerLimits
	Readiness config.ReadinessConfig
	Timeouts  config.AppTimeoutsConfig
}

type AppController struct {
	cfg AppControllerConfig

	mx             sync.Locker
	active         *container.AppContainer
	downscaleTimer *time.Timer

	l *zap.SugaredLogger
}

func NewAppController(id string, cfg AppControllerConfig, l *zap.SugaredLogger) (*AppController, error) {
	if err := validateImage(cfg.ImagePath); err != nil {
		return nil, err
	}
	return &AppController{
		cfg: cfg,

		mx: &sync.Mutex{},

		l: l.With(zap.String("app", id)),
	}, nil
}

func (ac *AppController) startLocked() error {
	if ac.active != nil {
		if ac.active.Alive() {
			return fmt.Errorf("app is already started")
		}
		ac.active = nil
	}

	spec := container.StartContainerSpec{
		ID:        uuid.New().String(),
		ImagePath: ac.cfg.ImagePath,
		Runner:    ac.cfg.Runner,
		Limits:    ac.cfg.Limits,
		Readiness: ac.cfg.Readiness,
	}
	c, err := container.StartContainer(spec, ac.l)
	if err != nil {
		return fmt.Errorf("start container: %w", err)
	}
	ac.active = c

	// arm the downscaler

	go func() {
		// don't start the timer until app is ready
		if err := c.WaitForReady(context.Background()); err != nil {
			ac.l.Warnf("Failed to arm the downscale timer: wait for ready: %s.", err)
			return
		}
		downscaleTimer := time.NewTimer(ac.cfg.Timeouts.IdleTimeout)

		ac.mx.Lock()
		ac.downscaleTimer = downscaleTimer
		ac.mx.Unlock()

		<-downscaleTimer.C

		if !c.Alive() {
			return
		}

		ac.l.Info("Auto-scaling the app down...")
		c.GracefulStop(ac.cfg.Timeouts.DrainTimeout, ac.cfg.Timeouts.StopTimeout)
	}()

	return nil
}

// Dial scales the app from zero (if needed), dials it and resets downscale timer
func (ac *AppController) Dial(ctx context.Context) (net.Conn, error) {
	ac.mx.Lock()
	if ac.active == nil || !ac.active.Alive() {
		if err := ac.startLocked(); err != nil {
			ac.mx.Unlock()
			return nil, fmt.Errorf("start: %w", err)
		}
	} else if ac.downscaleTimer != nil {
		ac.downscaleTimer.Reset(ac.cfg.Timeouts.IdleTimeout)
	}
	c := ac.active
	ac.mx.Unlock()

	if err := c.WaitForReady(ctx); err != nil {
		return nil, fmt.Errorf("wait for ready: %w", err)
	}

	return c.Dial()
}

func (ac *AppController) Stop() {
	ac.mx.Lock()
	if ac.active == nil || !ac.active.Alive() {
		ac.mx.Unlock()
		return
	}
	c := ac.active
	ac.active = nil
	if ac.downscaleTimer != nil {
		ac.downscaleTimer.Stop()
	}
	ac.mx.Unlock()

	c.GracefulStop(ac.cfg.Timeouts.DrainTimeout, ac.cfg.Timeouts.StopTimeout)
}

// Upgrade is neither strict start-first, nor stop-first. The inflight connections keep the old
// container alive, while new incoming traffic will get a cold start on a new image, leaving a
// small window where there could be two instances of the same app.
func (ac *AppController) Upgrade(newImagePath string) error {
	if err := validateImage(newImagePath); err != nil {
		return err
	}

	ac.mx.Lock()
	ac.cfg.ImagePath = newImagePath
	ac.mx.Unlock()

	go ac.Stop()
	return nil
}
