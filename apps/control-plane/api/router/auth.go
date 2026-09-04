package router

import (
	"github.com/gin-gonic/gin"
	"trojan-panel/api"
)

// 认证路由
func initAuthRouter(router *gin.Engine) {
	trojan := router.Group("/api")
	{
		trojanAuth := trojan.Group("/auth")
		{
			// 登录
			trojanAuth.POST("/login", api.Login)
			// 创建账户
			trojanAuth.POST("/register", api.Register)
			// 系统默认设置
			trojanAuth.GET("/setting", api.Setting)
			// 订阅
			trojanAuth.GET("/subscribe/:token", api.Subscribe)
			// 验证码
			trojanAuth.GET("/generateCaptcha", api.GenerateCaptcha)
		}
		trojanImage := trojan.Group("/image")
		{
			// logo
			trojanImage.GET("/logo", api.GetLogo)
		}
	}
}
