package router

import (
	"github.com/gin-gonic/gin"
	"trojan-panel/api"
)

func initRoleRouter(trojanApi *gin.RouterGroup) {
	role := trojanApi.Group("/role")
	{
		// 查询角色列表
		role.GET("/selectRoleList", api.SelectRoleList)
	}
}
