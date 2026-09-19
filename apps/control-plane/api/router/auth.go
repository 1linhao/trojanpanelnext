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
			// 安装器本机只读凭据健康检查；公网入口必须阻断该路径。
			trojanAuth.POST("/installer-health", api.InstallerHealth)
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
