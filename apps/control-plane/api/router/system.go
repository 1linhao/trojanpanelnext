package router

import (
	"github.com/gin-gonic/gin"
	"trojan-panel/api"
)

func initSystemRouter(trojanApi *gin.RouterGroup) {
	system := trojanApi.Group("/system")
	{
		// 查询系统设置
		system.GET("/selectSystemByName", api.SelectSystemByName)
		// 更新系统配置
		system.POST("/updateSystemById", api.UpdateSystemById)
		// 上传静态网站文件
		system.POST("/uploadWebFile", api.UploadWebFile)
		// 上传logo
		system.POST("/uploadLogo", api.UploadLogo)
	}
}
