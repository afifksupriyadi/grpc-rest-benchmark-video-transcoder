// Package grpc provides a gRPC implementation of the WorkerClient interface.
package grpc

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/gateway/internal/model"
	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/gateway/internal/service"
	pb "github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/gen/video"
	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/shared/timing"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

const chunkSize = 32 * 1024 // 32KB per chunk

// GrpcWorkerClient implements service.WorkerClient using gRPC bidirectional streaming.
type GrpcWorkerClient struct {
	stub pb.WorkerServiceClient
}

// NewGrpcWorkerClient creates a new GrpcWorkerClient connected to the given worker address.
func NewGrpcWorkerClient(workerAddr string) (service.WorkerClient, error) {
	conn, err := grpc.NewClient(workerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to worker: %w", err)
	}
	return &GrpcWorkerClient{stub: pb.NewWorkerServiceClient(conn)}, nil
}

// ProcessVideo streams the video payload to the worker, carrying t3 as outgoing metadata.
// It reads t5 back as header metadata from the worker before collecting result chunks.
func (c *GrpcWorkerClient) ProcessVideo(ctx context.Context, payload model.VideoPayload, t3 time.Time) (*model.TranscodeResult, time.Time, error) {
	md := metadata.Pairs(timing.HeaderTimestamp, timing.EncodeTimestamp(t3))
	ctx = metadata.NewOutgoingContext(ctx, md)

	stream, err := c.stub.ProcessVideo(ctx)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("failed to open worker stream: %w", err)
	}

	data := payload.Data
	for len(data) > 0 {
		size := chunkSize
		if len(data) < size {
			size = len(data)
		}

		if err := stream.Send(&pb.VideoChunk{
			Data:     data[:size],
			Filename: payload.Filename,
			Done:     false,
		}); err != nil {
			return nil, time.Time{}, fmt.Errorf("failed to send chunk: %w", err)
		}
		data = data[size:]
	}

	if err := stream.Send(&pb.VideoChunk{Done: true}); err != nil {
		return nil, time.Time{}, fmt.Errorf("failed to send done signal: %w", err)
	}

	if err := stream.CloseSend(); err != nil {
		return nil, time.Time{}, fmt.Errorf("failed to close send stream: %w", err)
	}

	result := &model.TranscodeResult{}
	currentOutputs := make(map[string]*model.ResolutionOutput)

	var t5 time.Time
	t5Read := false

	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, time.Time{}, fmt.Errorf("failed to receive result chunk: %w", err)
		}

		if !t5Read {
			headerMD, err := stream.Header()
			if err != nil {
				return nil, time.Time{}, fmt.Errorf("failed to read header metadata: %w", err)
			}
			t5Vals := headerMD.Get(timing.HeaderTimestamp)
			if len(t5Vals) == 0 {
				return nil, time.Time{}, fmt.Errorf("missing timestamp in worker header metadata")
			}
			t5, err = timing.DecodeTimestamp(t5Vals[0])
			if err != nil {
				return nil, time.Time{}, fmt.Errorf("invalid timestamp in worker header metadata: %w", err)
			}
			t5Read = true
		}

		if chunk.Done {
			if output, ok := currentOutputs[chunk.Resolution]; ok {
				result.Outputs = append(result.Outputs, *output)
				delete(currentOutputs, chunk.Resolution)
			}
			continue
		}

		if _, ok := currentOutputs[chunk.Resolution]; !ok {
			currentOutputs[chunk.Resolution] = &model.ResolutionOutput{
				Resolution: chunk.Resolution,
			}
		}
		currentOutputs[chunk.Resolution].Data = append(currentOutputs[chunk.Resolution].Data, chunk.Data...)
	}

	return result, t5, nil
}
