// Package metrics provides an interface and implementations for recording service metrics.
package metrics

import (
	"time"

	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/shared/label"
)

// MetricsRecorder defines the interface for recording benchmark metrics.
// labels carries scenario metadata (scenario, payload_size, concurrency_level)
// used to separate data across different research scenarios. ConcurrencyLevel
// is ignored by RecordCPU and RecordMemory, since CPU/RAM for Scenario B is
// never recorded per-request (see Section 13.4).
type MetricsRecorder interface {
	RecordLatency(segment string, protocol string, labels label.Labels, duration time.Duration)
	RecordThroughput(segment string, protocol string, labels label.Labels, bytes int64, duration time.Duration)
	RecordCPU(segment string, protocol string, labels label.Labels, usage float64)
	RecordMemory(segment string, protocol string, labels label.Labels, bytes float64)
}
