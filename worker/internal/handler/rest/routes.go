package rest

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all REST routes for the worker service.
func RegisterRoutes(r *gin.Engine, videoHandler *VideoHandler) {
	v1 := r.Group("/v1")
	{
		v1.POST("/process", videoHandler.HandleProcess)
	}
}