package router

import (
	"github.com/gin-gonic/gin"
	"trojan-panel/api"
	"trojan-panel/middleware"
)

// Router 主路由
func Router(router *gin.Engine) {
	// 限流和日志
	router.Use(middleware.RateLimiterHandler(), middleware.LogHandler())
	initAuthRouter(router)
	router.POST("/api/nodeServer/completeHostRemoval", api.CompleteHostRemoval)
	// 认证和权限
	router.Use(middleware.JWTHandler(), middleware.CasbinHandler())
	initApiRouter(router)
}
