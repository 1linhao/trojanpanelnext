package router

import (
	"github.com/gin-gonic/gin"
	"trojan-panel/api"
)

func initNodeServerRouter(trojanApi *gin.RouterGroup) {
	nodeServer := trojanApi.Group("/nodeServer")
	{
		// 根据id查询服务器
		nodeServer.GET("/selectNodeServerById", api.SelectNodeServerById)
		// 创建服务器
		nodeServer.POST("/createNodeServer", api.CreateNodeServer)
		// 分页查询服务器
		nodeServer.GET("/selectNodeServerPage", api.SelectNodeServerPage)
		// 删除服务器
		nodeServer.POST("/deleteNodeServerById", api.DeleteNodeServerById)
		// 重置服务器流量统计
		nodeServer.POST("/resetNodeServerTraffic", api.ResetNodeServerTraffic)
		// 更新服务器
		nodeServer.POST("/updateNodeServerById", api.UpdateNodeServerById)
		// 更新服务器列表
		nodeServer.GET("/selectNodeServerList", api.SelectNodeServerList)
		// 查询服务器状态
		nodeServer.GET("/nodeServerState", api.GetNodeServerInfo)
		// 导出服务器
		nodeServer.POST("/exportNodeServer", api.ExportNodeServer)
		// 导入服务器
		nodeServer.POST("/importNodeServer", api.ImportNodeServer)
	}
}
