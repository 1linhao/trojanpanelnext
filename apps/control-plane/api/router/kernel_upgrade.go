package router

import (
	"github.com/gin-gonic/gin"
	"trojan-panel/api"
)

func initKernelUpgradeRouter(trojanApi *gin.RouterGroup) {
	kernel := trojanApi.Group("/kernel")
	{
		kernel.GET("/releases", api.KernelReleases)
		kernel.GET("/inventory", api.KernelInventory)
		kernel.POST("/createTask", api.CreateKernelTask)
		kernel.GET("/selectTaskPage", api.SelectKernelTaskPage)
		kernel.GET("/selectTaskById", api.SelectKernelTaskById)
		kernel.POST("/retryTask", api.RetryKernelTask)
		kernel.POST("/probeMTLS", api.ProbeKernelMTLS)
	}
}
