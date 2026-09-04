package api

import (
	"github.com/gin-gonic/gin"
	"trojan-panel/model/constant"
	"trojan-panel/model/dto"
	"trojan-panel/model/vo"
	"trojan-panel/service"
)

func PanelGroup(c *gin.Context) {
	panelGroup, err := service.PanelGroup(c)
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	vo.Success(panelGroup, c)
}

// TrafficRank 流量排行榜
func TrafficRank(c *gin.Context) {
	var query dto.TrafficRankDto
	_ = c.ShouldBindQuery(&query)
	if query.Period == "" {
		query.Period = "total"
	}
	if err := validate.Struct(&query); err != nil {
		vo.Fail(constant.ValidateFailed, c)
		return
	}
	trafficRank, err := service.TrafficRank(query.Period, query.Date)
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	vo.Success(trafficRank, c)
}

func ServerTrafficUsage(c *gin.Context) {
	var query dto.ServerTrafficUsageDto
	_ = c.ShouldBindQuery(&query)
	if query.Period == "" {
		query.Period = "total"
	}
	if err := validate.Struct(&query); err != nil {
		vo.Fail(constant.ValidateFailed, c)
		return
	}
	if *query.PageSize > 100 {
		vo.Fail(constant.ValidateFailed, c)
		return
	}
	result, err := service.ServerTrafficUsage(query.Period, query.Date, query.NodeServerId, *query.PageNum, *query.PageSize)
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	vo.Success(result, c)
}

func ServerTrafficUserUsage(c *gin.Context) {
	var query dto.ServerTrafficUserUsageDto
	_ = c.ShouldBindQuery(&query)
	if query.Period == "" {
		query.Period = "total"
	}
	if err := validate.Struct(&query); err != nil {
		vo.Fail(constant.ValidateFailed, c)
		return
	}
	if *query.PageSize > 100 {
		vo.Fail(constant.ValidateFailed, c)
		return
	}
	result, err := service.ServerTrafficUserUsage(query.Period, query.Date, query.NodeServerId, *query.PageNum, *query.PageSize)
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	vo.Success(result, c)
}
