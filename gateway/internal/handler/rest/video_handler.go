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
	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/gateway/internal/util/contextutil"
	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/gateway/lib/metrics"
	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/shared/resource"
	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/shared/response"
	"github.com/gin-gonic/gin"
)

// VideoHandler handles REST requests for video transcoding.
type VideoHandler struct {
	svc     service.VideoService
	metrics metrics.MetricsRecorder
	cfg     *config.Config
}

// NewVideoHandler creates a new VideoHandler with the given service, metrics recorder, and config.
func NewVideoHandler(svc service.VideoService, metrics metrics.MetricsRecorder, cfg *config.Config) *VideoHandler {
	return &VideoHandler{svc: svc, metrics: metrics, cfg: cfg}
}

// HandleTranscode receives a video upload, forwards it for transcoding, and streams the result back.
// It also reads CPU/memory snapshots at t1, t2, t7, and after sending completes,
// to compute per-request CPU and memory usage for the gateway's own segments
// (client_to_gateway and gateway_to_client).
func (h *VideoHandler) HandleTranscode(c *gin.Context) {
	// t1: client starts sending to gateway
	t1 := time.Now()
	snapT1, errSnapT1 := resource.Read()

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, h.cfg.MaxFileSizeBytes)
	data, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.BuildError(c.Request.Context(),
			response.WrapAppError(c.Request.Context(), err, response.ErrInvalidRequest, "failed to read request body")))
		return
	}

	// t2: gateway finishes receiving from client
	t2 := time.Now()
	snapT2, errSnapT2 := resource.Read()

	clientToGatewayDuration := t2.Sub(t1)
	h.metrics.RecordLatency(constant.SegmentClientToGateway, constant.ProtocolREST, clientToGatewayDuration)
	h.metrics.RecordThroughput(constant.SegmentClientToGateway, constant.ProtocolREST, int64(len(data)), clientToGatewayDuration)

	if errSnapT1 == nil && errSnapT2 == nil {
		cpuDelta := snapT2.CPUSeconds - snapT1.CPUSeconds
		if clientToGatewayDuration.Seconds() > 0 {
			h.metrics.RecordCPU(constant.SegmentClientToGateway, constant.ProtocolREST, cpuDelta/clientToGatewayDuration.Seconds())
		}
		avgMemory := float64(snapT1.MemoryBytes+snapT2.MemoryBytes) / 2
		h.metrics.RecordMemory(constant.SegmentClientToGateway, constant.ProtocolREST, avgMemory)
	}

	filename := c.Request.Header.Get("X-Video-Filename")
	payload := model.VideoPayload{
		Filename: filename,
		Data:     data,
	}

	ctx := contextutil.SetProtocol(c.Request.Context(), constant.ProtocolREST)
	result, err := h.svc.Transcode(ctx, payload)
	if err != nil {
		res := response.BuildError(c.Request.Context(), err)
		c.JSON(res.Status, res.Body)
		return
	}

	// t7: gateway starts sending to client
	t7 := time.Now()
	snapT7, errSnapT7 := resource.Read()

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
	gatewayToClientDuration := time.Since(t7)
	snapSendEnd, errSnapSendEnd := resource.Read()

	h.metrics.RecordLatency(constant.SegmentGatewayToClient, constant.ProtocolREST, gatewayToClientDuration)
	h.metrics.RecordThroughput(constant.SegmentGatewayToClient, constant.ProtocolREST, int64(totalSize(result)), gatewayToClientDuration)

	if errSnapT7 == nil && errSnapSendEnd == nil {
		cpuDelta := snapSendEnd.CPUSeconds - snapT7.CPUSeconds
		if gatewayToClientDuration.Seconds() > 0 {
			h.metrics.RecordCPU(constant.SegmentGatewayToClient, constant.ProtocolREST, cpuDelta/gatewayToClientDuration.Seconds())
		}
		avgMemory := float64(snapT7.MemoryBytes+snapSendEnd.MemoryBytes) / 2
		h.metrics.RecordMemory(constant.SegmentGatewayToClient, constant.ProtocolREST, avgMemory)
	}
}

// totalSize calculates the total bytes of all transcoded outputs.
func totalSize(result *model.TranscodeResult) int {
	total := 0
	for _, o := range result.Outputs {
		total += len(o.Data)
	}
	return total
}
