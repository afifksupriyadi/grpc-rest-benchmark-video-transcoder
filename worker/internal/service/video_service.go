package service

import (
	"context"

	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/worker/internal/model"
	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/worker/lib/metrics"
)

// VideoServiceImpl implements VideoService.
type VideoServiceImpl struct {
	processor Processor
	metrics   metrics.MetricsRecorder
}

// NewVideoService creates a new VideoServiceImpl with the given processor and metrics recorder.
func NewVideoService(processor Processor, metrics metrics.MetricsRecorder) VideoService {
	return &VideoServiceImpl{
		processor: processor,
		metrics:   metrics,
	}
}

// Process runs FFmpeg transcoding on the received video and records t4 and t5 timestamps.
// t4 is recorded when worker finishes receiving from gateway.
// t5 is recorded when worker starts sending results back to gateway.
func (s *VideoServiceImpl) Process(ctx context.Context, data model.VideoData) (*model.TranscodeResult, error) {
	// t4: worker finishes receiving from gateway (recorded by handler before calling this)
	// FFmpeg runs here — not measured
	result, err := s.processor.Process(data)
	if err != nil {
		return nil, err
	}
	// t5: worker starts sending results to gateway (recorded by handler after this returns)

	_ = result
	return result, nil
}
