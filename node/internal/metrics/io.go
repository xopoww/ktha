package metrics

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

var nodeCgroupPath string

// SetNodeCgroupPath sets the cgroup path used for I/O metrics collection.
func SetNodeCgroupPath(path string) {
	nodeCgroupPath = path
}

var (
	ioReadBytes = prometheus.NewDesc(
		BuildName("node_io_read_bytes_total"),
		"Total bytes read by ktha-node (from cgroup io.stat).",
		nil, nil,
	)
	ioWriteBytes = prometheus.NewDesc(
		BuildName("node_io_write_bytes_total"),
		"Total bytes written by ktha-node (from cgroup io.stat).",
		nil, nil,
	)
)

type ioCollector struct{}

func init() {
	registry.MustRegister(&ioCollector{})
}

func (c *ioCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- ioReadBytes
	ch <- ioWriteBytes
}

func (c *ioCollector) Collect(ch chan<- prometheus.Metric) {
	if nodeCgroupPath == "" {
		return
	}

	rbytes, wbytes := parseIOStat(filepath.Join(nodeCgroupPath, "io.stat"))

	ch <- prometheus.MustNewConstMetric(ioReadBytes, prometheus.CounterValue, float64(rbytes))
	ch <- prometheus.MustNewConstMetric(ioWriteBytes, prometheus.CounterValue, float64(wbytes))
}

// parseIOStat reads io.stat and sums rbytes/wbytes across all devices.
// Format: "major:minor rbytes=N wbytes=N rios=N wios=N ..."
func parseIOStat(path string) (rbytes, wbytes uint64) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		for _, field := range fields {
			key, val, ok := strings.Cut(field, "=")
			if !ok {
				continue
			}
			n, err := strconv.ParseUint(val, 10, 64)
			if err != nil {
				continue
			}
			switch key {
			case "rbytes":
				rbytes += n
			case "wbytes":
				wbytes += n
			}
		}
	}
	return rbytes, wbytes
}
