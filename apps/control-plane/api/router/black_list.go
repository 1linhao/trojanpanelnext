package router

import (
	"github.com/gin-gonic/gin"
	"trojan-panel/api"
)

func initBlackListRouter(trojanApi *gin.RouterGroup) {
	blackList := trojanApi.Group("/blackList")
	{
		// 分页查询黑名单
		blackList.GET("/selectBlackListPage", api.SelectBlackListPage)
		// 删除黑名单
		blackList.POST("/deleteBlackListByIp", api.DeleteBlackListByIp)
		// 创建黑名单
		blackList.POST("/createBlackList", api.CreateBlackList)
	}
}
