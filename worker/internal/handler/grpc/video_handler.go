// Package grpc provides gRPC handlers for the worker service.
package grpc

import (
	"errors"
	"io"
	"time"

	pb "github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/gen/video"
	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/shared/timing"
	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/worker/internal/constant"
	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/worker/internal/model"
	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/worker/internal/service"
	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/worker/lib/metrics"
	"google.golang.org/grpc/metadata"
)

const chunkSize = 32 * 1024 // 32KB per chunk

// VideoServer implements pb.WorkerServiceServer.
type VideoServer struct {
	pb.UnimplementedWorkerServiceServer
	svc     service.VideoService
	metrics metrics.MetricsRecorder
}

// NewVideoServer creates a new VideoServer with the given service and metrics recorder.
func NewVideoServer(svc service.VideoService, metrics metrics.MetricsRecorder) *VideoServer {
	return &VideoServer{svc: svc, metrics: metrics}
}

// ProcessVideo receives a video stream from gateway, processes it, and streams results back.
// It reads t3 from incoming metadata to compute SegmentGatewayToWorker locally.
// It sets t5 as outgoing header metadata before sending the first result chunk.
func (s *VideoServer) ProcessVideo(stream pb.WorkerService_ProcessVideoServer) error {
	md, ok := metadata.FromIncomingContext(stream.Context())
	if !ok || len(md.Get(timing.HeaderTimestamp)) == 0 {
		return errors.New("missing timestamp metadata")
	}
	t3, err := timing.DecodeTimestamp(md.Get(timing.HeaderTimestamp)[0])
	if err != nil {
		return err
	}

	var (
		videoData  model.VideoData
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
			videoData.Filename = chunk.Filename
			firstChunk = false
		}

		if chunk.Done {
			break
		}

		videoData.Data = append(videoData.Data, chunk.Data...)
	}

	// t4: worker finishes receiving from gateway
	t4 := time.Now()

	gatewayToWorkerDuration := t4.Sub(t3)
	s.metrics.RecordLatency(constant.SegmentGatewayToWorker, constant.ProtocolGRPC, gatewayToWorkerDuration)
	s.metrics.RecordThroughput(constant.SegmentGatewayToWorker, constant.ProtocolGRPC, int64(len(videoData.Data)), gatewayToWorkerDuration)

	result, err := s.svc.Process(stream.Context(), videoData)
	if err != nil {
		return err
	}

	// t5: worker starts sending results to gateway
	t5 := time.Now()

	headerMD := metadata.Pairs(timing.HeaderTimestamp, timing.EncodeTimestamp(t5))
	if err := stream.SetHeader(headerMD); err != nil {
		return err
	}

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

	return nil
}
