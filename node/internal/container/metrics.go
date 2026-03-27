package container

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type ContainerMetrics struct {
	MemoryBytes float64
	CPUSeconds  float64
	Pids        float64
}

func (c *AppContainer) CollectMetrics() ContainerMetrics {
	cm := ContainerMetrics{}

	if value, err := collectMemoryBytes(c.cgroupPath); err != nil {
		c.l.Warnf("Failed to collect memory metric: %s.", err)
	} else {
		cm.MemoryBytes = value
	}
	if value, err := collectCPUSeconds(c.cgroupPath); err != nil {
		c.l.Warnf("Failed to collect cpu metric: %s.", err)
	} else {
		cm.CPUSeconds = value
	}
	if value, err := collectPids(c.cgroupPath); err != nil {
		c.l.Warnf("Failed to collect pids metric: %s.", err)
	} else {
		cm.Pids = value
	}

	return cm
}

func collectMemoryBytes(cgroup string) (float64, error) {
	return collectFloat(cgroup, "memory.current")
}

func collectCPUSeconds(cgroup string) (float64, error) {
	f, err := os.Open(filepath.Join(cgroup, "cpu.stat"))
	if err != nil {
		return 0, fmt.Errorf("open: %w", err)
	}
	defer f.Close()

	var data string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		key, val, ok := strings.Cut(scanner.Text(), " ")
		if ok && key == "usage_usec" {
			data = val
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("scan: %w", err)
	}

	value, err := strconv.ParseFloat(data, 64)
	if err != nil {
		return 0, fmt.Errorf("parse float: %w", err)
	}

	// microseconds to seconds
	value = value / 1000000
	return value, nil
}

func collectPids(cgroup string) (float64, error) {
	return collectFloat(cgroup, "pids.current")
}

func collectFloat(cgroup string, controller string) (float64, error) {
	data, err := os.ReadFile(filepath.Join(cgroup, controller))
	if err != nil {
		return 0, fmt.Errorf("read file: %w", err)
	}
	value, err := strconv.ParseFloat(strings.TrimSpace(string(data)), 64)
	if err != nil {
		return 0, fmt.Errorf("parse float: %w", err)
	}
	return value, nil
}
