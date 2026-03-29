package container

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/xopoww/ktha/node/internal/config"
)

func (c *AppContainer) pollForReadiness(cfg config.ReadinessConfig) {
	timeout := time.After(cfg.Timeout)
	ticker := time.NewTicker(cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if !c.checkReadiness() {
				continue
			}
			c.l.Info("Container is running.")
			close(c.ready)
			return
		case <-c.down:
			return
		case <-timeout:
			c.l.Warn("Readiness timeout, killing the container.")
			c.kill()
			return
		}
	}
}

const healthcheckTimeout = 5 * time.Second

func (c *AppContainer) checkReadiness() bool {
	if err := func() error {
		client := &http.Client{
			Timeout: healthcheckTimeout,
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					return c.dial()
				},
				DisableKeepAlives: true,
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
		c.l.Debugf("Readiness check: app not ready (%s).", err)
		return false
	}
	return true
}
