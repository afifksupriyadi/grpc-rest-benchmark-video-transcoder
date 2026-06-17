// Package grpc provides a gRPC implementation of the WorkerClient interface.
package grpc

import (
	"context"
	"fmt"
	"io"

	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/gateway/internal/model"
	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/gateway/internal/service"
	pb "github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/gen/video"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
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

// ProcessVideo streams the video payload to the worker and collects transcoded output chunks.
// It sends all video chunks first, then receives all result chunks from the worker.
func (c *GrpcWorkerClient) ProcessVideo(ctx context.Context, payload model.VideoPayload) (*model.TranscodeResult, error) {
	stream, err := c.stub.ProcessVideo(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to open worker stream: %w", err)
	}

	// send video chunks to worker
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
			return nil, fmt.Errorf("failed to send chunk: %w", err)
		}
		data = data[size:]
	}

	// send done signal
	if err := stream.Send(&pb.VideoChunk{Done: true}); err != nil {
		return nil, fmt.Errorf("failed to send done signal: %w", err)
	}

	// close send side and receive results
	if err := stream.CloseSend(); err != nil {
		return nil, fmt.Errorf("failed to close send stream: %w", err)
	}

	result := &model.TranscodeResult{}
	currentOutputs := make(map[string]*model.ResolutionOutput)

	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to receive result chunk: %w", err)
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

	return result, nil
}
