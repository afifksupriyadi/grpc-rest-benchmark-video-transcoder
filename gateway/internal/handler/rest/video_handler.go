// Package rest provides HTTP REST handlers for the gateway service.
package rest

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/gateway/config"
	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/gateway/internal/constant"
	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/gateway/internal/model"
	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/gateway/internal/service"
	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/gateway/lib/metrics"
	"github.com/gin-gonic/gin"
)

// VideoHandler handles REST requests for video transcoding.
type VideoHandler struct {
	svc     service.VideoService
	metrics metrics.MetricsRecorder
	cfg     *config.Config
}

// NewVideoHandler creates a new VideoHandler with the given service and metrics recorder.
func NewVideoHandler(svc service.VideoService, metrics metrics.MetricsRecorder, cfg *config.Config) *VideoHandler {
	return &VideoHandler{svc: svc, metrics: metrics, cfg: cfg}
}

// HandleTranscode receives a video upload, forwards it for transcoding, and streams the result back.
// It records t1 on request received and t2 after reading the full request body.
// It records t7 before streaming response and t8 after streaming completes.
func (h *VideoHandler) HandleTranscode(c *gin.Context) {
	// t1: client starts sending to gateway
	t1 := time.Now()

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, h.cfg.MaxFileSizeBytes)
	data, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file too large or failed to read"})
		return
	}
	filename := c.Request.Header.Get("X-Video-Filename")

	// t2: gateway finishes receiving from client
	t2 := time.Now()

	clientToGatewayDuration := t2.Sub(t1)
	h.metrics.RecordLatency(constant.SegmentClientToGateway, clientToGatewayDuration)
	h.metrics.RecordThroughput(constant.SegmentClientToGateway, int64(len(data)), clientToGatewayDuration)

	payload := model.VideoPayload{
		Filename: filename,
		Data:     data,
	}

	result, err := h.svc.Transcode(c.Request.Context(), payload)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// t7: gateway starts sending to client
	t7 := time.Now()

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

	// t8: gateway finishes sending to client
	t8 := time.Now()

	gatewayToClientDuration := t8.Sub(t7)
	h.metrics.RecordLatency(constant.SegmentGatewayToClient, gatewayToClientDuration)
	h.metrics.RecordThroughput(constant.SegmentGatewayToClient, int64(totalSize(result)), gatewayToClientDuration)
}

// totalSize calculates the total bytes of all transcoded outputs.
func totalSize(result *model.TranscodeResult) int {
	total := 0
	for _, o := range result.Outputs {
		total += len(o.Data)
	}
	return total
}
