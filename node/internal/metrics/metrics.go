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
	Name: BuildName("proxy_request_duration_seconds"),
	Help: "Histogram of proxy request latencies.",
}, []string{"app_id", "cold_start", "dial_ok"})

var AppCount = prometheus.NewGauge(prometheus.GaugeOpts{
	Name: BuildName("app_count"),
	Help: "Number of managed apps.",
})

var ContainerCount = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Name: BuildName("container_count"),
	Help: "Number of alive containers by app.",
}, []string{"app_id"})
