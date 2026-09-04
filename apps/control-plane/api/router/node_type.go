package router

import (
	"github.com/gin-gonic/gin"
	"trojan-panel/api"
)

func initNodeTypeRouter(trojanApi *gin.RouterGroup) {
	nodeType := trojanApi.Group("/nodeType")
	{
		// 查询节点类型列表
		nodeType.GET("/selectNodeTypeList", api.SelectNodeTypeList)
	}
}
