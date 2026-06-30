// Package rest provides HTTP REST handlers for the worker service.
package rest

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/shared/label"
	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/shared/resource"
	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/shared/timing"
	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/worker/internal/constant"
	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/worker/internal/model"
	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/worker/internal/service"
	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/worker/lib/metrics"
	"github.com/gin-gonic/gin"
)

// VideoHandler handles REST requests for video processing.
type VideoHandler struct {
	svc     service.VideoService
	metrics metrics.MetricsRecorder
}

// NewVideoHandler creates a new VideoHandler with the given service and metrics recorder.
func NewVideoHandler(svc service.VideoService, metrics metrics.MetricsRecorder) *VideoHandler {
	return &VideoHandler{svc: svc, metrics: metrics}
}

// HandleProcess receives a raw video upload from gateway, processes it, and streams results back.
// It reads t3 and scenario labels from the request headers sent by gateway, to compute
// SegmentGatewayToWorker locally. It also reads CPU/memory snapshots at t3, t4, t5, and
// after sending completes, to compute per-request CPU and memory usage, excluding the
// FFmpeg phase entirely.
func (h *VideoHandler) HandleProcess(c *gin.Context) {
	t3Str := c.Request.Header.Get(timing.HeaderTimestamp)
	t3, err := timing.DecodeTimestamp(t3Str)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing or invalid timestamp header"})
		return
	}

	labels := label.Labels{
		Scenario:         c.Request.Header.Get(label.HeaderScenario),
		PayloadSize:      c.Request.Header.Get(label.HeaderPayloadSize),
		ConcurrencyLevel: c.Request.Header.Get(label.HeaderConcurrencyLevel),
	}

	snapT3, errSnapT3 := resource.Read()

	data, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
		return
	}

	// t4: worker finishes receiving from gateway
	t4 := time.Now()
	snapT4, errSnapT4 := resource.Read()

	gatewayToWorkerDuration := t4.Sub(t3)
	h.metrics.RecordLatency(constant.SegmentGatewayToWorker, constant.ProtocolREST, labels, gatewayToWorkerDuration)
	h.metrics.RecordThroughput(constant.SegmentGatewayToWorker, constant.ProtocolREST, labels, int64(len(data)), gatewayToWorkerDuration)

	filename := c.Request.Header.Get("X-Video-Filename")
	videoData := model.VideoData{
		Filename: filename,
		Data:     data,
	}

	result, err := h.svc.Process(c.Request.Context(), videoData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// t5: worker starts sending results to gateway
	t5 := time.Now()
	snapT5, errSnapT5 := resource.Read()

	c.Header(timing.HeaderTimestamp, timing.EncodeTimestamp(t5))
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Transfer-Encoding", "chunked")
	c.Status(http.StatusOK)

	for _, output := range result.Outputs {
		chunkHeader := model.ChunkHeader{
			Resolution: output.Resolution,
			Size:       int64(len(output.Data)),
			Done:       false,
		}
		json.NewEncoder(c.Writer).Encode(chunkHeader)
		c.Writer.Write(output.Data)
		c.Writer.Flush()

		json.NewEncoder(c.Writer).Encode(model.ChunkHeader{
			Resolution: output.Resolution,
			Done:       true,
		})
		c.Writer.Flush()
	}

	// snapshot after sending completes, used as the end point of the send phase
	sendEndDuration := time.Since(t5)
	snapSendEnd, errSnapSendEnd := resource.Read()

	if errSnapT3 != nil || errSnapT4 != nil || errSnapT5 != nil || errSnapSendEnd != nil {
		// resource read failed at one or more points; skip CPU/RAM recording for this request
		return
	}

	recordCPUAndMemory(h.metrics, constant.SegmentGatewayToWorker, constant.ProtocolREST, labels,
		snapT3, snapT4, snapT5, snapSendEnd, gatewayToWorkerDuration, sendEndDuration)
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
