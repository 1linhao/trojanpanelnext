package router

import (
	"github.com/gin-gonic/gin"
)

// api路由
func initApiRouter(router *gin.Engine) {
	trojanApi := router.Group("/api")
	{
		initDashboardRouter(trojanApi)
		initAccountRouter(trojanApi)
		initRoleRouter(trojanApi)
		initNodeServerRouter(trojanApi)
		initNodeRouter(trojanApi)
		initNodeTypeRouter(trojanApi)
		initSystemRouter(trojanApi)
		initBlackListRouter(trojanApi)
		initEmailRecordRouter(trojanApi)
		initFileTaskRouter(trojanApi)
		initKernelUpgradeRouter(trojanApi)
	}
}
