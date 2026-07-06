package metrics

import (
	"time"

	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/shared/label"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

// PrometheusRecorder implements MetricsRecorder using Prometheus histograms.
type PrometheusRecorder struct {
	latency    *prometheus.HistogramVec
	throughput *prometheus.HistogramVec
	cpu        *prometheus.HistogramVec
	memory     *prometheus.HistogramVec
}

// NewPrometheusRecorder creates and registers a new PrometheusRecorder.
// It also registers default process and Go runtime collectors.
func NewPrometheusRecorder(reg *prometheus.Registry) *PrometheusRecorder {
	reg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	reg.MustRegister(collectors.NewGoCollector())

	latency := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "worker_segment_latency_seconds",
		Help:    "Latency of each communication segment in seconds.",
		Buckets: prometheus.DefBuckets,
	}, []string{"segment", "protocol", "scenario", "payload_size", "concurrency_level"})

	throughput := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "worker_segment_throughput_bytes_per_second",
		Help:    "Throughput of each communication segment in bytes per second.",
		Buckets: prometheus.ExponentialBuckets(1024, 2, 24),
	}, []string{"segment", "protocol", "scenario", "payload_size", "concurrency_level"})

	// cpu/memory deliberately exclude concurrency_level: Scenario B's CPU/RAM
	// is never recorded per-request via Observe(), so this label would never
	// be populated for these two histograms (see Section 13.4).
	cpu := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "worker_segment_cpu_usage_ratio",
		Help:    "Per-request CPU usage during a segment, as a fraction of one core.",
		Buckets: prometheus.LinearBuckets(0, 0.25, 32),
	}, []string{"segment", "protocol", "scenario", "payload_size"})

	memory := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "worker_segment_memory_usage_bytes",
		Help:    "Per-request average resident memory during a segment, in bytes.",
		Buckets: prometheus.ExponentialBuckets(1024*1024, 2, 16),
	}, []string{"segment", "protocol", "scenario", "payload_size"})

	reg.MustRegister(latency, throughput, cpu, memory)

	return &PrometheusRecorder{
		latency:    latency,
		throughput: throughput,
		cpu:        cpu,
		memory:     memory,
	}
}

// RecordLatency records the latency for a given segment, protocol, and scenario labels.
func (p *PrometheusRecorder) RecordLatency(segment string, protocol string, labels label.Labels, duration time.Duration) {
	p.latency.WithLabelValues(segment, protocol, labels.Scenario, labels.PayloadSize, labels.ConcurrencyLevel).Observe(duration.Seconds())
}

// RecordThroughput records the throughput for a given segment, protocol, and scenario labels.
func (p *PrometheusRecorder) RecordThroughput(segment string, protocol string, labels label.Labels, bytes int64, duration time.Duration) {
	if duration.Seconds() <= 0 {
		return
	}
	bps := float64(bytes) / duration.Seconds()
	p.throughput.WithLabelValues(segment, protocol, labels.Scenario, labels.PayloadSize, labels.ConcurrencyLevel).Observe(bps)
}

// RecordCPU records the per-request CPU usage for a given segment, protocol, and scenario labels.
func (p *PrometheusRecorder) RecordCPU(segment string, protocol string, labels label.Labels, usage float64) {
	p.cpu.WithLabelValues(segment, protocol, labels.Scenario, labels.PayloadSize).Observe(usage)
}

// RecordMemory records the per-request average memory usage for a given segment, protocol, and scenario labels.
func (p *PrometheusRecorder) RecordMemory(segment string, protocol string, labels label.Labels, bytes float64) {
	p.memory.WithLabelValues(segment, protocol, labels.Scenario, labels.PayloadSize).Observe(bytes)
}
