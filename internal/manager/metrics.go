package manager

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var readLatencyHistogramVec = prometheus.NewHistogramVec(prometheus.HistogramOpts{
	Namespace: "rate_limit_control_plane",
	Name:      "rpaas_nginx_pod_read_request_latency_seconds",
	Help:      "Histogram of request latency for rate-limit RPaaS pod reads in seconds",
	Buckets:   prometheus.ExponentialBuckets(0.001, 2, 12),
}, []string{"pod_name", "zone"})

func init() {
	metrics.Registry.MustRegister(readLatencyHistogramVec)
}
