// Package grpc provides gRPC handlers for the worker service.
package grpc

import (
	"io"
	"time"

	pb "github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/gen/video"
	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/worker/internal/constant"
	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/worker/internal/model"
	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/worker/internal/service"
	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/worker/lib/metrics"
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
// It records t4 after all chunks are received and t5 before streaming the response.
func (s *VideoServer) ProcessVideo(stream pb.WorkerService_ProcessVideoServer) error {
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

	s.metrics.RecordLatency(constant.SegmentGatewayToWorker, time.Since(t4))
	s.metrics.RecordThroughput(constant.SegmentGatewayToWorker, int64(len(videoData.Data)), time.Since(t4))

	result, err := s.svc.Process(stream.Context(), videoData)
	if err != nil {
		return err
	}

	// t5: worker starts sending results to gateway
	t5 := time.Now()

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

	workerToGatewayDuration := time.Since(t5)
	s.metrics.RecordLatency(constant.SegmentWorkerToGateway, workerToGatewayDuration)
	s.metrics.RecordThroughput(constant.SegmentWorkerToGateway, int64(totalSize(result)), workerToGatewayDuration)

	return nil
}

// totalSize calculates the total bytes of all transcoded outputs.
func totalSize(result *model.TranscodeResult) int {
	total := 0
	for _, o := range result.Outputs {
		total += len(o.Data)
	}
	return total
}
