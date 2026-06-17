// Package rest provides HTTP REST handlers for the worker service.
package rest

import (
	"io"
	"net/http"
	"time"

	"encoding/json"

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
// It records t4 after reading the full request body and t5 before streaming the response.
func (h *VideoHandler) HandleProcess(c *gin.Context) {
	data, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
		return
	}

	// t4: worker finishes receiving from gateway
	t4 := time.Now()

	filename := c.Request.Header.Get("X-Video-Filename")
	videoData := model.VideoData{
		Filename: filename,
		Data:     data,
	}

	gatewayToWorkerDuration := t4.Sub(t4) // placeholder, t3 comes from gateway
	h.metrics.RecordLatency(constant.SegmentGatewayToWorker, gatewayToWorkerDuration)
	h.metrics.RecordThroughput(constant.SegmentGatewayToWorker, int64(len(data)), gatewayToWorkerDuration)

	result, err := h.svc.Process(c.Request.Context(), videoData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// t5: worker starts sending results to gateway
	t5 := time.Now()

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

	// record t5 → t6 duration (t6 recorded by gateway when it finishes receiving)
	workerToGatewayDuration := time.Since(t5)
	h.metrics.RecordLatency(constant.SegmentWorkerToGateway, workerToGatewayDuration)
	h.metrics.RecordThroughput(constant.SegmentWorkerToGateway, int64(totalSize(result)), workerToGatewayDuration)
}

// totalSize calculates the total bytes of all transcoded outputs.
func totalSize(result *model.TranscodeResult) int {
	total := 0
	for _, o := range result.Outputs {
		total += len(o.Data)
	}
	return total
}
