package common

type ContainerLimits struct {
	// in bytes
	MemoryMax int `yaml:"memory_max"`
	PidsMax   int `yaml:"pids_max"`
	// in µs (100000µs window)
	CPUMax int `yaml:"cpu_max"`
}
