package metrics

import "time"

// NilMetricsRecorder is a no-op implementation of MetricsRecorder used in testing.
type NilMetricsRecorder struct{}

// NewNilMetricsRecorder creates a new NilMetricsRecorder.
func NewNilMetricsRecorder() *NilMetricsRecorder {
	return &NilMetricsRecorder{}
}

// RecordLatency does nothing.
func (n *NilMetricsRecorder) RecordLatency(segment string, protocol string, duration time.Duration) {
}

// RecordThroughput does nothing.
func (n *NilMetricsRecorder) RecordThroughput(segment string, protocol string, bytes int64, duration time.Duration) {
}
