package config

import (
	"time"

	"github.com/xopoww/ktha/node/internal/common"
)

type RunnerConfig struct {
	BinaryPath string `yaml:"binary_path"`
	RootfsRoot string `yaml:"rootfs_root"`
	CgroupRoot string `yaml:"cgroup_root"`
}

type NodeJSConfig struct {
	BinaryPath string `yaml:"binary_path"`
}

type ProxyConfig struct {
	Port uint16 `yaml:"port"`
}

type ApplicationConfig struct {
	ReadinessPollInterval time.Duration `yaml:"readiness_poll_interval"`
	ReadinessTimeout      time.Duration `yaml:"readiness_timeout"`
	IdleTimeout           time.Duration `yaml:"idle_timeout"`
	StopTimeout           time.Duration `yaml:"stop_timeout"`

	ImagesBasePath string `yaml:"images_base_path"`

	Limits common.ContainerLimits `yaml:"limits"`

	// map appID -> AppConfig
	Apps map[string]AppConfig `yaml:"apps"`
}

type AppConfig struct {
	Image string `yaml:"image"`
}

type Config struct {
	Runner      RunnerConfig      `yaml:"runner"`
	NodeJS      NodeJSConfig      `yaml:"nodejs"`
	Proxy       ProxyConfig       `yaml:"proxy"`
	Application ApplicationConfig `yaml:"application"`
}
