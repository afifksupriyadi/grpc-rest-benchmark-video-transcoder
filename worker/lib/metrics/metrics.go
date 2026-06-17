// Package metrics provides an interface and implementations for recording service metrics.
package metrics

import "time"

// MetricsRecorder defines the interface for recording benchmark metrics.
type MetricsRecorder interface {
	RecordLatency(segment string, duration time.Duration)
	RecordThroughput(segment string, bytes int64, duration time.Duration)
}
