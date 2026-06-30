package rest

import (
	"net/http"
	"time"

	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/gateway/internal/model"
	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/gateway/lib/metrics"
	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/shared/label"
	"github.com/gin-gonic/gin"
)

// MetricHandler handles client-reported metrics for segments gateway cannot measure itself.
type MetricHandler struct {
	metrics metrics.MetricsRecorder
}

// NewMetricHandler creates a new MetricHandler with the given metrics recorder.
func NewMetricHandler(metrics metrics.MetricsRecorder) *MetricHandler {
	return &MetricHandler{metrics: metrics}
}

// HandleReportMetric receives a client-measured duration and byte count, then
// records it into gateway's own histograms, exactly like a locally-measured segment.
func (h *MetricHandler) HandleReportMetric(c *gin.Context) {
	var req model.ReportMetricRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid report payload"})
		return
	}

	labels := label.Labels{
		Scenario:         req.Scenario,
		PayloadSize:      req.PayloadSize,
		ConcurrencyLevel: req.ConcurrencyLevel,
	}

	duration := time.Duration(req.DurationSeconds * float64(time.Second))
	h.metrics.RecordLatency(req.Segment, req.Protocol, labels, duration)
	h.metrics.RecordThroughput(req.Segment, req.Protocol, labels, req.Bytes, duration)

	c.Status(http.StatusNoContent)
}
