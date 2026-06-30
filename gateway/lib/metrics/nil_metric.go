package metrics

import (
	"time"

	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/shared/label"
)

// NilMetricsRecorder is a no-op implementation of MetricsRecorder used in testing.
type NilMetricsRecorder struct{}

// NewNilMetricsRecorder creates a new NilMetricsRecorder.
func NewNilMetricsRecorder() *NilMetricsRecorder {
	return &NilMetricsRecorder{}
}

// RecordLatency does nothing.
func (n *NilMetricsRecorder) RecordLatency(segment string, protocol string, labels label.Labels, duration time.Duration) {
}

// RecordThroughput does nothing.
func (n *NilMetricsRecorder) RecordThroughput(segment string, protocol string, labels label.Labels, bytes int64, duration time.Duration) {
}

// RecordCPU does nothing.
func (n *NilMetricsRecorder) RecordCPU(segment string, protocol string, labels label.Labels, usage float64) {
}

// RecordMemory does nothing.
func (n *NilMetricsRecorder) RecordMemory(segment string, protocol string, labels label.Labels, bytes float64) {
}
