package router

import (
	"github.com/gin-gonic/gin"
	"trojan-panel/api"
)

func initFileTaskRouter(trojanApi *gin.RouterGroup) {
	fileTask := trojanApi.Group("/fileTask")
	{
		// 分页查询文件任务
		fileTask.GET("/selectFileTaskPage", api.SelectFileTaskPage)
		// 删除文件任务
		fileTask.POST("/deleteFileTaskById", api.DeleteFileTaskById)
		// 下载文件任务的文件
		fileTask.POST("/downloadFileTask", api.DownloadFileTask)
		// 获取文件模板
		fileTask.POST("/downloadTemplate", api.DownloadTemplate)
	}
}
