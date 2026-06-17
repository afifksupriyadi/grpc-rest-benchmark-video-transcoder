// Package service contains the business logic for the worker service.
package service

import (
	"context"

	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/worker/internal/model"
)

// VideoService defines the contract for video processing operations.
type VideoService interface {
	Process(ctx context.Context, data model.VideoData) (*model.TranscodeResult, error)
}

// Processor defines the contract for FFmpeg video transcoding.
type Processor interface {
	Process(input model.VideoData) (*model.TranscodeResult, error)
}
