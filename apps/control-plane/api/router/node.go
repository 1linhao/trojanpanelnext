package router

import (
	"github.com/gin-gonic/gin"
	"trojan-panel/api"
)

func initNodeRouter(trojanApi *gin.RouterGroup) {
	node := trojanApi.Group("/node")
	{
		// 根据id查询节点
		node.GET("/selectNodeById", api.SelectNodeById)
		// 查询节点连接信息
		node.GET("/selectNodeInfo", api.SelectNodeInfo)
		// 创建节点
		node.POST("/createNode", api.CreateNode)
		// 分页查询节点
		node.GET("/selectNodePage", api.SelectNodePage)
		// 删除节点
		node.POST("/deleteNodeById", api.DeleteNodeById)
		// 更新节点
		node.POST("/updateNodeById", api.UpdateNodeById)
		// 获取节点二维码
		node.POST("/nodeQRCode", api.NodeQRCode)
		// 复制URL
		node.POST("/nodeURL", api.NodeURL)
		// 节点部分属性的默认值
		node.GET("/nodeDefault", api.NodeDefault)
	}
}
