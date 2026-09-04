package router

import (
	"github.com/gin-gonic/gin"
	"trojan-panel-core/api"
	"trojan-panel-core/middleware"
)

func Router(router *gin.Engine) {
	router.Use(middleware.RateLimiterHandler(), middleware.LogHandler())
	auth := router.Group("/api/auth")
	{
		// Hysteria2 api
		auth.POST("/hysteria2", api.Hysteria2Api)
	}
}
