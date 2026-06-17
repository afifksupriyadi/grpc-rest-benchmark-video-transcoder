package service

import (
	"context"
	"time"

	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/gateway/internal/constant"
	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/gateway/internal/model"
	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/gateway/lib/metrics"
)

// VideoServiceImpl implements VideoService.
type VideoServiceImpl struct {
	workerClient WorkerClient
	metrics      metrics.MetricsRecorder
}

// NewVideoService creates a new VideoServiceImpl with the given worker client and metrics recorder.
func NewVideoService(workerClient WorkerClient, metrics metrics.MetricsRecorder) VideoService {
	return &VideoServiceImpl{
		workerClient: workerClient,
		metrics:      metrics,
	}
}

// Transcode forwards the video payload to the worker and returns the transcoded result.
// It records t3 before forwarding and t6 after receiving the result for metrics measurement.
func (s *VideoServiceImpl) Transcode(ctx context.Context, payload model.VideoPayload) (*model.TranscodeResult, error) {
	// t3: gateway starts sending to worker
	t3 := time.Now()

	result, err := s.workerClient.ProcessVideo(ctx, payload)
	if err != nil {
		return nil, err
	}

	// t6: gateway finishes receiving from worker
	t6 := time.Now()

	gatewayToWorkerDuration := t6.Sub(t3)
	s.metrics.RecordLatency(constant.SegmentGatewayToWorker, gatewayToWorkerDuration)
	s.metrics.RecordThroughput(constant.SegmentGatewayToWorker, int64(len(payload.Data)), gatewayToWorkerDuration)

	return result, nil
}
