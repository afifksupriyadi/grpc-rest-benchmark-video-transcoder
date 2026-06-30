// Package service contains the business logic for the gateway service.
package service

import (
	"context"
	"time"

	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/gateway/internal/model"
	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/shared/label"
)

// VideoService defines the contract for video transcoding operations.
type VideoService interface {
	Transcode(ctx context.Context, payload model.VideoPayload, labels label.Labels) (*model.TranscodeResult, error)
}

// WorkerClient defines the contract for communicating with the worker service.
// It is implemented by both REST and gRPC worker clients.
// t3 is passed in so the worker can compute SegmentGatewayToWorker locally.
// labels carries scenario metadata to be propagated alongside the video.
// t5 is returned so the service can compute SegmentWorkerToGateway.
type WorkerClient interface {
	ProcessVideo(ctx context.Context, payload model.VideoPayload, t3 time.Time, labels label.Labels) (result *model.TranscodeResult, t5 time.Time, err error)
}
