// Package service contains the business logic for the gateway service.
package service

import (
	"context"

	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/gateway/internal/model"
)

// VideoService defines the contract for video transcoding operations.
type VideoService interface {
	Transcode(ctx context.Context, payload model.VideoPayload) (*model.TranscodeResult, error)
}

// WorkerClient defines the contract for communicating with the worker service.
// It is implemented by both REST and gRPC worker clients.
type WorkerClient interface {
	ProcessVideo(ctx context.Context, payload model.VideoPayload) (*model.TranscodeResult, error)
}
