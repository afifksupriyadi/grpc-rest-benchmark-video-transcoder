// Package grpc provides gRPC handlers for the gateway service.
package grpc

import (
	"context"
	"io"
	"time"

	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/gateway/internal/constant"
	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/gateway/internal/model"
	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/gateway/internal/service"
	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/gateway/internal/util/contextutil"
	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/gateway/lib/metrics"
	pb "github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/gen/video"
	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/shared/response"
)

const chunkSize = 32 * 1024 // 32KB per chunk

// VideoServer implements pb.GatewayServiceServer.
type VideoServer struct {
	pb.UnimplementedGatewayServiceServer
	svc     service.VideoService
	metrics metrics.MetricsRecorder
}

// NewVideoServer creates a new VideoServer with the given service and metrics recorder.
func NewVideoServer(svc service.VideoService, metrics metrics.MetricsRecorder) *VideoServer {
	return &VideoServer{svc: svc, metrics: metrics}
}

// TranscodeVideo receives a video stream from the client, transcodes it, and streams results back.
// It records t1 on first chunk received and t2 after all chunks are received.
// It records t7 before streaming response and t8 after streaming completes.
func (s *VideoServer) TranscodeVideo(stream pb.GatewayService_TranscodeVideoServer) error {
	var (
		payload    model.VideoPayload
		t1         time.Time
		firstChunk = true
	)

	// receive all chunks from client
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		if firstChunk {
			// t1: client starts sending to gateway
			t1 = time.Now()
			payload.Filename = chunk.Filename
			firstChunk = false
		}

		if chunk.Done {
			break
		}

		payload.Data = append(payload.Data, chunk.Data...)
	}

	// t2: gateway finishes receiving from client
	t2 := time.Now()

	clientToGatewayDuration := t2.Sub(t1)
	s.metrics.RecordLatency(constant.SegmentClientToGateway, constant.ProtocolGRPC, clientToGatewayDuration)
	s.metrics.RecordThroughput(constant.SegmentClientToGateway, constant.ProtocolGRPC, int64(len(payload.Data)), clientToGatewayDuration)

	ctx := contextutil.SetProtocol(stream.Context(), constant.ProtocolGRPC)
	result, err := s.svc.Transcode(ctx, payload)
	if err != nil {
		return response.ParseErrorWithGRPC(err)
	}

	// t7: gateway starts sending to client
	t7 := time.Now()

	// stream results back to client
	for _, output := range result.Outputs {
		data := output.Data
		for len(data) > 0 {
			size := chunkSize
			if len(data) < size {
				size = len(data)
			}

			if err := stream.Send(&pb.TranscodeChunk{
				Resolution: output.Resolution,
				Data:       data[:size],
				Done:       false,
			}); err != nil {
				return err
			}
			data = data[size:]
		}

		// send done signal for this resolution
		if err := stream.Send(&pb.TranscodeChunk{
			Resolution: output.Resolution,
			Done:       true,
		}); err != nil {
			return err
		}
	}

	// t8: gateway finishes sending to client
	t8 := time.Now()

	gatewayToClientDuration := t8.Sub(t7)
	s.metrics.RecordLatency(constant.SegmentGatewayToClient, constant.ProtocolGRPC, gatewayToClientDuration)
	s.metrics.RecordThroughput(constant.SegmentGatewayToClient, constant.ProtocolGRPC, int64(len(payload.Data)), gatewayToClientDuration)

	return nil
}

// CheckStatus handles a unary health check request from the client.
func (s *VideoServer) CheckStatus(ctx context.Context, req *pb.StatusRequest) (*pb.StatusResponse, error) {
	return &pb.StatusResponse{
		Status:  "ok",
		Message: "gateway is running",
	}, nil
}
