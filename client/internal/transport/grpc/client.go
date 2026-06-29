// Package grpc provides a gRPC implementation of the GatewayClient interface.
package grpc

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/client/internal/model"
	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/client/internal/transport"
	pb "github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/gen/video"
	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/shared/timing"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

const chunkSize = 32 * 1024 // 32KB per chunk

// GrpcGatewayClient implements transport.GatewayClient using gRPC bidirectional streaming.
type GrpcGatewayClient struct {
	stub pb.GatewayServiceClient
}

// NewGrpcGatewayClient creates a new GrpcGatewayClient connected to the given gateway address.
func NewGrpcGatewayClient(gatewayAddr string) (transport.GatewayClient, error) {
	conn, err := grpc.NewClient(gatewayAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to gateway: %w", err)
	}
	return &GrpcGatewayClient{stub: pb.NewGatewayServiceClient(conn)}, nil
}

// Transcode streams the video to gateway, carrying t1 as outgoing metadata.
// It reads t7 back as header metadata from gateway before collecting result chunks.
func (c *GrpcGatewayClient) Transcode(ctx context.Context, filename string, data []byte, t1 time.Time) (*model.TranscodeResult, time.Time, error) {
	md := metadata.Pairs(timing.HeaderTimestamp, timing.EncodeTimestamp(t1))
	ctx = metadata.NewOutgoingContext(ctx, md)

	stream, err := c.stub.TranscodeVideo(ctx)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("failed to open gateway stream: %w", err)
	}

	remaining := data
	for len(remaining) > 0 {
		size := chunkSize
		if len(remaining) < size {
			size = len(remaining)
		}

		if err := stream.Send(&pb.VideoChunk{
			Data:     remaining[:size],
			Filename: filename,
			Done:     false,
		}); err != nil {
			return nil, time.Time{}, fmt.Errorf("failed to send chunk: %w", err)
		}
		remaining = remaining[size:]
	}

	if err := stream.Send(&pb.VideoChunk{Done: true}); err != nil {
		return nil, time.Time{}, fmt.Errorf("failed to send done signal: %w", err)
	}

	if err := stream.CloseSend(); err != nil {
		return nil, time.Time{}, fmt.Errorf("failed to close send stream: %w", err)
	}

	result := &model.TranscodeResult{}
	currentOutputs := make(map[string]*model.ResolutionOutput)

	var t7 time.Time
	t7Read := false

	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, time.Time{}, fmt.Errorf("failed to receive result chunk: %w", err)
		}

		if !t7Read {
			headerMD, err := stream.Header()
			if err != nil {
				return nil, time.Time{}, fmt.Errorf("failed to read header metadata: %w", err)
			}
			t7Vals := headerMD.Get(timing.HeaderTimestamp)
			if len(t7Vals) == 0 {
				return nil, time.Time{}, fmt.Errorf("missing timestamp in gateway header metadata")
			}
			t7, err = timing.DecodeTimestamp(t7Vals[0])
			if err != nil {
				return nil, time.Time{}, fmt.Errorf("invalid timestamp in gateway header metadata: %w", err)
			}
			t7Read = true
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

	return result, t7, nil
}
