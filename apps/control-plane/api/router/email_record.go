package router

import (
	"github.com/gin-gonic/gin"
	"trojan-panel/api"
)

func initEmailRecordRouter(trojanApi *gin.RouterGroup) {
	emailRecord := trojanApi.Group("/emailRecord")
	{
		// 查询邮件发送记录
		emailRecord.GET("/selectEmailRecordPage", api.SelectEmailRecordPage)
	}
}
