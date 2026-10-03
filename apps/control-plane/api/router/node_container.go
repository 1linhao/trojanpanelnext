package router

import (
	"github.com/gin-gonic/gin"
	"trojan-panel/api"
)

func initNodeContainerRouter(trojanApi *gin.RouterGroup) {
	container := trojanApi.Group("/container")
	container.GET("/inventory", api.NodeContainerInventory)
	container.POST("/update", api.UpdateNodeContainer)
}
