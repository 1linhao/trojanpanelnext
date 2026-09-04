package router

import (
	"github.com/gin-gonic/gin"
	"trojan-panel/api"
)

func initDashboardRouter(trojanApi *gin.RouterGroup) {
	dashboard := trojanApi.Group("/dashboard")
	{
		// 仪表板
		dashboard.GET("/panelGroup", api.PanelGroup)
		// 流量排行榜
		dashboard.GET("/trafficRank", api.TrafficRank)
		dashboard.GET("/serverTrafficUsage", api.ServerTrafficUsage)
		dashboard.GET("/serverTrafficUserUsage", api.ServerTrafficUserUsage)
	}
}
