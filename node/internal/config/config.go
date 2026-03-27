package config

import (
	"time"
)

type RunnerConfig struct {
	BinaryPath     string `yaml:"binary_path"`
	RootBasePath   string `yaml:"root_base_path"`
	CgroupBasePath string `yaml:"cgroup_base_path"`
	NodeJS         struct {
		BinaryPath string `yaml:"binary_path"`
	} `yaml:"nodejs"`
}

type ContainerLimits struct {
	// in bytes
	MemoryMax int `yaml:"memory_max"`
	PidsMax   int `yaml:"pids_max"`
	// in µs (100000µs window)
	CPUMax int `yaml:"cpu_max"`
}

type ReadinessConfig struct {
	PollInterval time.Duration `yaml:"poll_interval"`
	Timeout      time.Duration `yaml:"timeout"`
}

type AppTimeoutsConfig struct {
	IdleTimeout  time.Duration `yaml:"idle_timeout"`
	DrainTimeout time.Duration `yaml:"drain_timeout"`
	StopTimeout  time.Duration `yaml:"stop_timeout"`
}

type ProxyConfig struct {
	Port uint16 `yaml:"port"`
}

type AdminConfig struct {
	Port    uint16 `yaml:"port"`
	AuthKey string `yaml:"auth_key"`
}

type ApplicationConfig struct {
	ImagesBasePath string `yaml:"images_base_path"`

	Limits    ContainerLimits   `yaml:"limits"`
	Readiness ReadinessConfig   `yaml:"readiness"`
	Timeouts  AppTimeoutsConfig `yaml:"timeouts"`

	// map appID -> AppConfig
	Apps map[string]AppConfig `yaml:"apps"`
}

type AppConfig struct {
	Image string `yaml:"image"`
}

type Config struct {
	Admin       AdminConfig       `yaml:"admin"`
	Proxy       ProxyConfig       `yaml:"proxy"`
	Runner      RunnerConfig      `yaml:"runner"`
	Application ApplicationConfig `yaml:"application"`
}
