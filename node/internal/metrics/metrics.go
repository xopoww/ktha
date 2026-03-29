package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var registry = prometheus.NewRegistry()

func init() {
	registry.MustRegister(collectors.NewGoCollector())
	registry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	registry.MustRegister(ProxyRequestDuration)
	registry.MustRegister(AppCount)
	registry.MustRegister(ContainerCount)
	registry.MustRegister(ContainerKills)
}

// Registry returns the metrics registry for registering additional collectors.
func Registry() *prometheus.Registry {
	return registry
}

// Handler returns an HTTP handler serving the /metrics endpoint.
func Handler() http.Handler {
	return promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
}

const Namespace = "ktha"

// BuildName returns a fully qualified metric name under the ktha namespace.
func BuildName(name string) string {
	return prometheus.BuildFQName(Namespace, "", name)
}

var ProxyRequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
	Name:    BuildName("proxy_request_duration_seconds"),
	Help:    "Histogram of proxy request latencies.",
	Buckets: []float64{0.001, 0.002, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10, 30},
}, []string{"app_id", "cold_start", "dial_ok"})

var AppCount = prometheus.NewGauge(prometheus.GaugeOpts{
	Name: BuildName("app_count"),
	Help: "Number of managed apps.",
})

var ContainerCount = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Name: BuildName("container_count"),
	Help: "Number of alive containers by app.",
}, []string{"app_id"})

var ContainerKills = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: BuildName("app_container_kills_total"),
	Help: "Number of container SIGKILL exits by app (OOM or stop timeout).",
}, []string{"app_id"})
