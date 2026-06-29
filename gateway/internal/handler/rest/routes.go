package rest

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all REST routes for the gateway service.
func RegisterRoutes(r *gin.Engine, videoHandler *VideoHandler, metricHandler *MetricHandler) {
	v1 := r.Group("/v1")
	{
		v1.POST("/transcode", videoHandler.HandleTranscode)
		v1.POST("/report-metric", metricHandler.HandleReportMetric)
	}
}
