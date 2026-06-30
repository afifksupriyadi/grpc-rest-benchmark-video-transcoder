// Package grpc provides gRPC handlers for the worker service.
package grpc

import (
	"errors"
	"io"
	"time"

	pb "github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/gen/video"
	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/shared/label"
	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/shared/resource"
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
// It reads t3 and scenario labels from incoming metadata sent by gateway, to compute
// SegmentGatewayToWorker locally. It also reads CPU/memory snapshots at t3, t4, t5, and
// after sending completes, to compute per-request CPU and memory usage, excluding the
// FFmpeg phase entirely.
func (s *VideoServer) ProcessVideo(stream pb.WorkerService_ProcessVideoServer) error {
	md, ok := metadata.FromIncomingContext(stream.Context())
	if !ok || len(md.Get(timing.HeaderTimestamp)) == 0 {
		return errors.New("missing timestamp metadata")
	}
	t3, err := timing.DecodeTimestamp(md.Get(timing.HeaderTimestamp)[0])
	if err != nil {
		return err
	}

	labels := label.Labels{
		Scenario:         getMetadataValue(md, label.HeaderScenario),
		PayloadSize:      getMetadataValue(md, label.HeaderPayloadSize),
		ConcurrencyLevel: getMetadataValue(md, label.HeaderConcurrencyLevel),
	}

	snapT3, errSnapT3 := resource.Read()

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
	snapT4, errSnapT4 := resource.Read()

	gatewayToWorkerDuration := t4.Sub(t3)
	s.metrics.RecordLatency(constant.SegmentGatewayToWorker, constant.ProtocolGRPC, labels, gatewayToWorkerDuration)
	s.metrics.RecordThroughput(constant.SegmentGatewayToWorker, constant.ProtocolGRPC, labels, int64(len(videoData.Data)), gatewayToWorkerDuration)

	result, err := s.svc.Process(stream.Context(), videoData)
	if err != nil {
		return err
	}

	// t5: worker starts sending results to gateway
	t5 := time.Now()
	snapT5, errSnapT5 := resource.Read()

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

	// snapshot after sending completes, used as the end point of the send phase
	sendEndDuration := time.Since(t5)
	snapSendEnd, errSnapSendEnd := resource.Read()

	if errSnapT3 != nil || errSnapT4 != nil || errSnapT5 != nil || errSnapSendEnd != nil {
		return nil
	}

	recordCPUAndMemory(s.metrics, constant.SegmentGatewayToWorker, constant.ProtocolGRPC, labels,
		snapT3, snapT4, snapT5, snapSendEnd, gatewayToWorkerDuration, sendEndDuration)

	return nil
}

// recordCPUAndMemory computes per-request CPU and memory usage from four snapshots,
// combining the receive phase (t3 to t4) and send phase (t5 to send-end) while
// excluding the FFmpeg phase (t4 to t5) entirely, then records one CPU and one
// memory value for the request.
func recordCPUAndMemory(
	m metrics.MetricsRecorder,
	segment string,
	protocol string,
	labels label.Labels,
	snapT3, snapT4, snapT5, snapSendEnd resource.Snapshot,
	receiveDuration, sendDuration time.Duration,
) {
	cpuDeltaReceive := snapT4.CPUSeconds - snapT3.CPUSeconds
	cpuDeltaSend := snapSendEnd.CPUSeconds - snapT5.CPUSeconds

	totalCPUDelta := cpuDeltaReceive + cpuDeltaSend
	totalDuration := receiveDuration.Seconds() + sendDuration.Seconds()

	if totalDuration > 0 {
		m.RecordCPU(segment, protocol, labels, totalCPUDelta/totalDuration)
	}

	avgMemory := float64(snapT3.MemoryBytes+snapT4.MemoryBytes+snapT5.MemoryBytes+snapSendEnd.MemoryBytes) / 4
	m.RecordMemory(segment, protocol, labels, avgMemory)
}

// getMetadataValue safely extracts the first value for a metadata key, returning
// an empty string if the key is absent.
func getMetadataValue(md metadata.MD, key string) string {
	vals := md.Get(key)
	if len(vals) == 0 {
		return ""
	}
	return vals[0]
}
