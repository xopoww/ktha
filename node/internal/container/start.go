package container

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/xopoww/ktha/node/internal/config"
	"github.com/xopoww/ktha/node/internal/runner"
	"go.uber.org/zap"
)

const socketName = "app.sock"

// StopInfo is passed to OnStop with details about how the container stopped.
type StopInfo struct {
	// Killed is true if the container exited with SIGKILL (exit code 137).
	// This typically means OOM kill by cgroups, but can also be a stop timeout.
	Killed bool
}

type StartContainerSpec struct {
	ID        string
	ImagePath string
	Env       config.AppEnv
	Runner    config.RunnerConfig
	Limits    config.ContainerLimits
	Readiness config.ReadinessConfig

	// OnStop is called exactly once when the container stops being alive,
	// and it is guaranteed not be called if StartContainer returns an error
	OnStop func(StopInfo)
}

func StartContainer(spec StartContainerSpec, l *zap.SugaredLogger) (*AppContainer, error) {
	l = l.With(zap.String("container", spec.ID))

	flags := []string{
		"--" + runner.FlagContainerID, spec.ID,
		"--" + runner.FlagImagePath, spec.ImagePath,
		"--" + runner.FlagEnv, serializeEnv(spec.Env),
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
		cmd:        cmd,
		inflight:   sync.WaitGroup{},
		ready:      make(chan struct{}),
		down:       make(chan struct{}),
		rootPath:   filepath.Join(spec.Runner.RootBasePath, spec.ID),
		cgroupPath: filepath.Join(spec.Runner.CgroupBasePath, spec.ID),
		l:          l,
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

		var info StopInfo
		if exitErr, ok := err.(*exec.ExitError); ok {
			// Exit code 137 = killed by SIGKILL (128+9), which is what the
			// cgroup OOM killer sends.
			if exitErr.ExitCode() == 137 {
				info.Killed = true
			}
		}

		close(c.down)
		if spec.OnStop != nil {
			spec.OnStop(info)
		}
	}()

	go c.pollForReadiness(spec.Readiness)

	return c, nil
}

func serializeEnv(env config.AppEnv) string {
	parts := make([]string, 0, len(env))
	for key, val := range env {
		parts = append(parts, fmt.Sprintf("%s=%s", key, val))
	}
	return strings.Join(parts, ",")
}
