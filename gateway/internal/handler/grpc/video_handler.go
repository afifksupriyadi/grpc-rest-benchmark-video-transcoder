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
	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/shared/resource"
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
// It also reads CPU/memory snapshots at t1, t2, t7, and after sending completes,
// to compute per-request CPU and memory usage for the gateway's own segments.
func (s *VideoServer) TranscodeVideo(stream pb.GatewayService_TranscodeVideoServer) error {
	var (
		payload    model.VideoPayload
		t1         time.Time
		snapT1     resource.Snapshot
		errSnapT1  error
		firstChunk = true
	)

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
			snapT1, errSnapT1 = resource.Read()
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
	snapT2, errSnapT2 := resource.Read()

	clientToGatewayDuration := t2.Sub(t1)
	s.metrics.RecordLatency(constant.SegmentClientToGateway, constant.ProtocolGRPC, clientToGatewayDuration)
	s.metrics.RecordThroughput(constant.SegmentClientToGateway, constant.ProtocolGRPC, int64(len(payload.Data)), clientToGatewayDuration)

	if errSnapT1 == nil && errSnapT2 == nil {
		cpuDelta := snapT2.CPUSeconds - snapT1.CPUSeconds
		if clientToGatewayDuration.Seconds() > 0 {
			s.metrics.RecordCPU(constant.SegmentClientToGateway, constant.ProtocolGRPC, cpuDelta/clientToGatewayDuration.Seconds())
		}
		avgMemory := float64(snapT1.MemoryBytes+snapT2.MemoryBytes) / 2
		s.metrics.RecordMemory(constant.SegmentClientToGateway, constant.ProtocolGRPC, avgMemory)
	}

	ctx := contextutil.SetProtocol(stream.Context(), constant.ProtocolGRPC)
	result, err := s.svc.Transcode(ctx, payload)
	if err != nil {
		return response.ParseErrorWithGRPC(err)
	}

	// t7: gateway starts sending to client
	t7 := time.Now()
	snapT7, errSnapT7 := resource.Read()

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

		if err := stream.Send(&pb.TranscodeChunk{
			Resolution: output.Resolution,
			Done:       true,
		}); err != nil {
			return err
		}
	}

	// snapshot after sending completes, used as the end point of the send phase
	gatewayToClientDuration := time.Since(t7)
	snapSendEnd, errSnapSendEnd := resource.Read()

	s.metrics.RecordLatency(constant.SegmentGatewayToClient, constant.ProtocolGRPC, gatewayToClientDuration)
	s.metrics.RecordThroughput(constant.SegmentGatewayToClient, constant.ProtocolGRPC, int64(totalSize(result)), gatewayToClientDuration)

	if errSnapT7 == nil && errSnapSendEnd == nil {
		cpuDelta := snapSendEnd.CPUSeconds - snapT7.CPUSeconds
		if gatewayToClientDuration.Seconds() > 0 {
			s.metrics.RecordCPU(constant.SegmentGatewayToClient, constant.ProtocolGRPC, cpuDelta/gatewayToClientDuration.Seconds())
		}
		avgMemory := float64(snapT7.MemoryBytes+snapSendEnd.MemoryBytes) / 2
		s.metrics.RecordMemory(constant.SegmentGatewayToClient, constant.ProtocolGRPC, avgMemory)
	}

	return nil
}

// CheckStatus handles a unary health check request from the client.
func (s *VideoServer) CheckStatus(ctx context.Context, req *pb.StatusRequest) (*pb.StatusResponse, error) {
	return &pb.StatusResponse{
		Status:  "ok",
		Message: "gateway is running",
	}, nil
}

// totalSize calculates the total bytes of all transcoded outputs.
func totalSize(result *model.TranscodeResult) int {
	total := 0
	for _, o := range result.Outputs {
		total += len(o.Data)
	}
	return total
}
