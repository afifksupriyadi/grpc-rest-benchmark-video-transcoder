package service

import (
	"context"
	"time"

	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/gateway/internal/constant"
	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/gateway/internal/model"
	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/gateway/internal/util/contextutil"
	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/gateway/lib/metrics"
	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/shared/response"
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
// t3 is recorded here and passed to the worker so it can compute SegmentGatewayToWorker locally.
// t5 is received back from the worker so SegmentWorkerToGateway can be computed here at t6.
func (s *VideoServiceImpl) Transcode(ctx context.Context, payload model.VideoPayload) (*model.TranscodeResult, error) {
	protocol := contextutil.GetProtocol(ctx)

	// t3: gateway starts sending to worker
	t3 := time.Now()

	result, t5, err := s.workerClient.ProcessVideo(ctx, payload, t3)
	if err != nil {
		return nil, response.WrapAppError(ctx, err, response.ErrWorkerUnavailable, "worker failed to process video")
	}

	// t6: gateway finishes receiving from worker
	t6 := time.Now()

	workerToGatewayDuration := t6.Sub(t5)
	s.metrics.RecordLatency(constant.SegmentWorkerToGateway, protocol, workerToGatewayDuration)
	s.metrics.RecordThroughput(constant.SegmentWorkerToGateway, protocol, int64(totalSize(result)), workerToGatewayDuration)

	return result, nil
}

// totalSize calculates the total bytes of all transcoded outputs.
func totalSize(result *model.TranscodeResult) int {
	total := 0
	for _, o := range result.Outputs {
		total += len(o.Data)
	}
	return total
}
