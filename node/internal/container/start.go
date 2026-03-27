package container

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/xopoww/ktha/node/internal/config"
	"github.com/xopoww/ktha/node/internal/runner"
	"go.uber.org/zap"
)

const socketName = "app.sock"

type StartContainerSpec struct {
	ID        string
	ImagePath string
	Runner    config.RunnerConfig
	Limits    config.ContainerLimits
	Readiness config.ReadinessConfig
}

func StartContainer(spec StartContainerSpec, l *zap.SugaredLogger) (*AppContainer, error) {
	l = l.With(zap.String("container", spec.ID))

	flags := []string{
		"--" + runner.FlagContainerID, spec.ID,
		"--" + runner.FlagImagePath, spec.ImagePath,
		"--" + runner.FlagMemoryMax, fmt.Sprint(spec.Limits.MemoryMax),
		"--" + runner.FlagPidsMax, fmt.Sprint(spec.Limits.PidsMax),
		"--" + runner.FlagCPUMax, fmt.Sprint(spec.Limits.CPUMax),
		"--" + runner.FlagNodeBinaryPath, spec.Runner.NodeJS.BinaryPath,
		"--" + runner.FlagRootBasePath, spec.Runner.RootBasePath,
		"--" + runner.FlagCgroupBasePath, spec.Runner.CgroupBasePath,
		"--" + runner.FlagSocket, socketName,
	}
	cmd := exec.Command(spec.Runner.BinaryPath, flags...)

	l.Debugf("Running '%s'...", cmd)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start: %w", err)
	}

	c := &AppContainer{
		cmd:      cmd,
		inflight: sync.WaitGroup{},
		ready:    make(chan struct{}),
		down:     make(chan struct{}),
		rootPath: filepath.Join(spec.Runner.RootBasePath, spec.ID),
		l:        l,
	}

	// wait for exit
	go func() {
		err := cmd.Wait()
		if err == nil {
			l.Info("Container stopped.")
		} else {
			// TODO: dump stdout/stderr
			l.Warnf("Container crashed: %s.", err)
		}
		close(c.down)
	}()

	go c.pollForReadiness(spec.Readiness)

	return c, nil
}
