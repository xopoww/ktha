package manager

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/xopoww/ktha/node/internal/controller"
)

func (a *AppManager) Describe(ch chan<- *prometheus.Desc) {
	controller.DescribeMetrics(ch)
}

func (a *AppManager) Collect(ch chan<- prometheus.Metric) {
	wg := sync.WaitGroup{}
	a.mx.Lock()
	for _, ac := range a.controllers {
		wg.Go(func() {
			ac.Collect(ch)
		})
	}
	a.mx.Unlock()
	wg.Wait()
}
