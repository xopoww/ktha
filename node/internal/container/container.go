package container

import (
	"context"
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"go.uber.org/zap"
)

type AppContainer struct {
	cmd      *exec.Cmd
	inflight sync.WaitGroup

	ready chan struct{} // closed when can accept requests
	down  chan struct{} // closed when stopped/dead

	// rootPath is the path to container's fs root (including container ID)
	rootPath string

	l *zap.SugaredLogger
}

func (c *AppContainer) WaitForReady(ctx context.Context) error {
	select {
	case <-c.down:
		return ErrContainerDown
	case <-c.ready:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Alive reports whether the container is not down
func (c *AppContainer) Alive() bool {
	select {
	case <-c.down:
		return false
	default:
		return true
	}
}

// Dial dials the container's socket without checking if the container is alive.
// It also adds to inflight requests count.
func (c *AppContainer) Dial() (net.Conn, error) {
	c.inflight.Add(1)
	onClose := func() {
		c.inflight.Done()
	}

	conn, err := c.dial()
	if err != nil {
		onClose()
		return nil, err
	}

	return wrapConn(conn, onClose), nil
}

func (c *AppContainer) dial() (net.Conn, error) {
	socket := filepath.Join(c.rootPath, socketName)
	return net.Dial("unix", socket)
}

func (c *AppContainer) GracefulStop(drainTimeout time.Duration, stopTimeout time.Duration) {
	c.l.Debug("Stopping the container...")

	// drain the connections
	drained := make(chan struct{})
	go func() {
		c.inflight.Wait()
		close(drained)
	}()

	select {
	case <-drained:
		// pass
	case <-time.After(drainTimeout):
		c.l.Warn("Drain timeout, forcefully stopping the container.")
	}

	err := c.cmd.Process.Signal(syscall.SIGTERM)
	if err != nil {
		if errors.Is(err, os.ErrProcessDone) {
			return
		} else {
			c.l.Warnf("Failed to send SIGTERM to the container: %s.", err)
		}
	}

	select {
	case <-c.down:
		c.l.Debugf("Container is down.")
	case <-time.After(stopTimeout):
		c.l.Warnf("Stop timeout, killing the container.")
		c.kill()
	}
}

func (c *AppContainer) kill() {
	if err := c.cmd.Process.Signal(syscall.SIGKILL); err != nil && !errors.Is(err, os.ErrProcessDone) {
		c.l.Errorf("Failed to send SIGKILL to the container: %s.", err)
	}
}
