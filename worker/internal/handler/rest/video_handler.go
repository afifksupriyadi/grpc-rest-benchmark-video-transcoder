// Package rest provides HTTP REST handlers for the worker service.
package rest

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

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
// It reads t3 from the request header to compute SegmentGatewayToWorker locally.
// It sets t5 in the response header before streaming results back.
func (h *VideoHandler) HandleProcess(c *gin.Context) {
	t3Str := c.Request.Header.Get(timing.HeaderTimestamp)
	t3, err := timing.DecodeTimestamp(t3Str)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing or invalid timestamp header"})
		return
	}

	data, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
		return
	}

	// t4: worker finishes receiving from gateway
	t4 := time.Now()

	gatewayToWorkerDuration := t4.Sub(t3)
	h.metrics.RecordLatency(constant.SegmentGatewayToWorker, constant.ProtocolREST, gatewayToWorkerDuration)
	h.metrics.RecordThroughput(constant.SegmentGatewayToWorker, constant.ProtocolREST, int64(len(data)), gatewayToWorkerDuration)

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
}
