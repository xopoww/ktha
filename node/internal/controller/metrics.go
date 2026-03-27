package controller

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/xopoww/ktha/node/internal/metrics"
)

var (
	memoryMetricDesc = prometheus.NewDesc(
		metrics.BuildName("app_memory_bytes"),
		"App memory usage.",
		[]string{"app_id"},
		nil,
	)

	cpuMetricDesc = prometheus.NewDesc(
		metrics.BuildName("app_cpu_seconds"),
		"App CPU time.",
		[]string{"app_id"},
		nil,
	)

	pidsMetricDesc = prometheus.NewDesc(
		metrics.BuildName("app_processes_count"),
		"App processes count.",
		[]string{"app_id"},
		nil,
	)
)

func DescribeMetrics(ch chan<- *prometheus.Desc) {
	ch <- memoryMetricDesc
	ch <- cpuMetricDesc
	ch <- pidsMetricDesc
}

func (ac *AppController) Collect(ch chan<- prometheus.Metric) {
	ac.mx.Lock()
	if ac.active == nil || !ac.active.Alive() {
		ac.mx.Unlock()
		return
	}
	c := ac.active
	ac.mx.Unlock()

	cm := c.CollectMetrics()
	labels := []string{ac.id}
	ch <- prometheus.MustNewConstMetric(
		memoryMetricDesc,
		prometheus.GaugeValue,
		cm.MemoryBytes,
		labels...,
	)
	ch <- prometheus.MustNewConstMetric(
		cpuMetricDesc,
		prometheus.CounterValue,
		cm.CPUSeconds,
		labels...,
	)
	ch <- prometheus.MustNewConstMetric(
		pidsMetricDesc,
		prometheus.GaugeValue,
		cm.Pids,
		labels...,
	)
}
