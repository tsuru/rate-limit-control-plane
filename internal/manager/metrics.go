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
}, []string{"pod_name", "service_name", "rpaas_instance", "zone"})

var aggregateLatencyHistogramVec = prometheus.NewHistogramVec(prometheus.HistogramOpts{
	Namespace: "rate_limit_control_plane",
	Name:      "rpaas_instance_rate_limit_aggregation_duration_seconds",
	Help:      "Histogram of aggregation duration for rate-limit RPaaS instances in seconds",
	Buckets:   prometheus.ExponentialBuckets(0.001, 2, 12),
}, []string{"rpaas_instance", "service_name", "zone"})

// Error/Reliability Metrics
var readOperationsCounterVec = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: "rate_limit_control_plane",
	Name:      "rpaas_nginx_pod_read_operations_total",
	Help:      "Total number of read operations on RPaaS nginx pods",
}, []string{"pod_name", "service_name", "rpaas_instance", "zone", "status"})

var aggregationFailuresCounterVec = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: "rate_limit_control_plane",
	Name:      "rpaas_instance_aggregation_failures_total",
	Help:      "Total number of aggregation failures for RPaaS instances",
}, []string{"rpaas_instance", "service_name", "zone", "error_type"})

var podHealthStatusGaugeVec = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Namespace: "rate_limit_control_plane",
	Name:      "rpaas_nginx_pod_health_status",
	Help:      "Health status of RPaaS nginx pods (1=healthy, 0=unhealthy)",
}, []string{"pod_name", "service_name", "rpaas_instance", "zone"})

// Performance/Throughput Metrics
var requestsPerSecondGaugeVec = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Namespace: "rate_limit_control_plane",
	Name:      "rpaas_nginx_pod_requests_per_second",
	Help:      "Current requests per second processed by RPaaS nginx pods",
}, []string{"pod_name", "service_name", "rpaas_instance", "zone"})

var zoneDataSizeHistogramVec = prometheus.NewHistogramVec(prometheus.HistogramOpts{
	Namespace: "rate_limit_control_plane",
	Name:      "rpaas_zone_data_size_bytes",
	Help:      "Size distribution of zone data processed in bytes",
	Buckets:   prometheus.ExponentialBuckets(1024, 2, 12),
}, []string{"rpaas_instance", "service_name", "zone"})

var activeWorkersGaugeVec = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Namespace: "rate_limit_control_plane",
	Name:      "rpaas_active_workers_count",
	Help:      "Number of active workers by type",
}, []string{"service_name", "worker_type"})

// Rate Limiting Metrics
var rateLimitEntriesCounterVec = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: "rate_limit_control_plane",
	Name:      "rpaas_rate_limit_entries_total",
	Help:      "Total number of rate limit entries by action",
}, []string{"rpaas_instance", "service_name", "zone", "action"})

var rateLimitRulesActiveGaugeVec = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Namespace: "rate_limit_control_plane",
	Name:      "rpaas_rate_limit_rules_active",
	Help:      "Number of active rate limit rules per zone",
}, []string{"rpaas_instance", "service_name", "zone"})

// System/Resource Metrics
var workerUptimeGaugeVec = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Namespace: "rate_limit_control_plane",
	Name:      "rpaas_worker_uptime_seconds",
	Help:      "Worker uptime in seconds",
}, []string{"worker_id", "worker_type", "rpaas_instance", "service_name"})

var zoneDataRepositoryMemoryGauge = prometheus.NewGauge(prometheus.GaugeOpts{
	Namespace: "rate_limit_control_plane",
	Name:      "rpaas_zone_data_repository_memory_bytes",
	Help:      "Memory usage of the zone data repository in bytes",
})

// GetZoneDataRepositoryMemoryGauge returns the memory gauge for external use
func GetZoneDataRepositoryMemoryGauge() prometheus.Gauge {
	return zoneDataRepositoryMemoryGauge
}

func init() {
	metrics.Registry.MustRegister(readLatencyHistogramVec)
	metrics.Registry.MustRegister(aggregateLatencyHistogramVec)

	// Register error/reliability metrics
	metrics.Registry.MustRegister(readOperationsCounterVec)
	metrics.Registry.MustRegister(aggregationFailuresCounterVec)
	metrics.Registry.MustRegister(podHealthStatusGaugeVec)

	// Register performance/throughput metrics
	metrics.Registry.MustRegister(requestsPerSecondGaugeVec)
	metrics.Registry.MustRegister(zoneDataSizeHistogramVec)
	metrics.Registry.MustRegister(activeWorkersGaugeVec)

	// Register rate limiting metrics
	metrics.Registry.MustRegister(rateLimitEntriesCounterVec)
	metrics.Registry.MustRegister(rateLimitRulesActiveGaugeVec)

	// Register system/resource metrics
	metrics.Registry.MustRegister(workerUptimeGaugeVec)
	metrics.Registry.MustRegister(zoneDataRepositoryMemoryGauge)
}
