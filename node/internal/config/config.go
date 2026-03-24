package config

type RunnerConfig struct {
	BinaryPath string `yaml:"binary_path"`
	RootfsRoot string `yaml:"rootfs_root"`
}

type NodeJSConfig struct {
	BinaryPath string `yaml:"binary_path"`
}

type ProxyConfig struct {
	Port uint16 `yaml:"port"`
}

type Config struct {
	Runner RunnerConfig `yaml:"runner"`
	NodeJS NodeJSConfig `yaml:"nodejs"`
	Proxy  ProxyConfig  `yaml:"proxy"`
}
