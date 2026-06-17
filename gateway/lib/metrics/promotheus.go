package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

// PrometheusRecorder implements MetricsRecorder using Prometheus histograms.
type PrometheusRecorder struct {
	latency    *prometheus.HistogramVec
	throughput *prometheus.HistogramVec
}

// NewPrometheusRecorder creates and registers a new PrometheusRecorder.
// It also registers default process and Go runtime collectors.
func NewPrometheusRecorder(reg *prometheus.Registry) *PrometheusRecorder {
	reg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	reg.MustRegister(collectors.NewGoCollector())

	latency := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "gateway_segment_latency_seconds",
		Help:    "Latency of each communication segment in seconds.",
		Buckets: prometheus.DefBuckets,
	}, []string{"segment", "protocol"})

	throughput := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "gateway_segment_throughput_bytes_per_second",
		Help:    "Throughput of each communication segment in bytes per second.",
		Buckets: prometheus.ExponentialBuckets(1024, 2, 20),
	}, []string{"segment", "protocol"})

	reg.MustRegister(latency, throughput)

	return &PrometheusRecorder{
		latency:    latency,
		throughput: throughput,
	}
}

// RecordLatency records the latency for a given segment and protocol.
func (p *PrometheusRecorder) RecordLatency(segment string, duration time.Duration) {
	p.latency.WithLabelValues(segment).Observe(duration.Seconds())
}

// RecordThroughput records the throughput for a given segment and protocol.
func (p *PrometheusRecorder) RecordThroughput(segment string, bytes int64, duration time.Duration) {
	if duration.Seconds() <= 0 {
		return
	}
	bps := float64(bytes) / duration.Seconds()
	p.throughput.WithLabelValues(segment).Observe(bps)
}
