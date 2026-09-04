package service

import (
	"errors"
	"github.com/gin-gonic/gin"
	"time"
	"trojan-panel/dao"
	"trojan-panel/model/constant"
	"trojan-panel/model/vo"
	"trojan-panel/util"
)

// CronTrafficRank 流量排行榜 一小时更新一次
func CronTrafficRank() {
	// Rankings are now queried from indexed durable ledgers. Kept as a no-op
	// for compatibility with existing cron configuration.
}

func TrafficRank(period, date string) ([]vo.AccountTrafficRankVo, error) {
	start, end, err := trafficDateRange(period, date, time.Now())
	if err != nil {
		return nil, err
	}
	trafficRank, err := dao.TrafficRank(period, start, end)
	if err != nil {
		return nil, err
	}
	for index := range trafficRank {
		trafficRank[index].Username = maskUsername(trafficRank[index].Username)
	}
	return trafficRank, nil
}

func trafficDateRange(period, value string, now time.Time) (string, string, error) {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return "", "", errors.New(constant.SysError)
	}
	now = now.In(location)
	switch period {
	case "total":
		if value != "" {
			return "", "", errors.New(constant.ValidateFailed)
		}
		return "", "", nil
	case "day":
		if value == "" {
			value = now.Format("2006-01-02")
		}
		parsed, parseErr := time.ParseInLocation("2006-01-02", value, location)
		if parseErr != nil || parsed.Format("2006-01-02") != value {
			return "", "", errors.New(constant.ValidateFailed)
		}
		return value, parsed.AddDate(0, 0, 1).Format("2006-01-02"), nil
	case "month":
		if value == "" {
			value = now.Format("2006-01")
		}
		parsed, parseErr := time.ParseInLocation("2006-01", value, location)
		if parseErr != nil || parsed.Format("2006-01") != value {
			return "", "", errors.New(constant.ValidateFailed)
		}
		return parsed.Format("2006-01-02"), parsed.AddDate(0, 1, 0).Format("2006-01-02"), nil
	default:
		return "", "", errors.New(constant.ValidateFailed)
	}
}

func maskUsername(username string) string {
	runes := []rune(username)
	if len(runes) <= 2 {
		return string(runes[:1]) + "****"
	}
	if len(runes) <= 4 {
		return string(runes[:1]) + "****" + string(runes[len(runes)-1:])
	}
	return string(runes[:2]) + "****" + string(runes[len(runes)-2:])
}

func ServerTrafficUsage(period, date string, nodeServerID *uint, pageNum, pageSize uint) (*vo.ServerTrafficUsagePageVo, error) {
	start, end, err := trafficDateRange(period, date, time.Now())
	if err != nil {
		return nil, err
	}
	rows, total, err := dao.SelectServerTrafficUsage(start, end, nodeServerID, pageNum, pageSize)
	if err != nil {
		return nil, err
	}
	return &vo.ServerTrafficUsagePageVo{BaseVoPage: vo.BaseVoPage{PageNum: pageNum, PageSize: pageSize, Total: total}, Rows: rows}, nil
}

func ServerTrafficUserUsage(period, date string, nodeServerID, pageNum, pageSize uint) (*vo.ServerTrafficUserUsagePageVo, error) {
	start, end, err := trafficDateRange(period, date, time.Now())
	if err != nil {
		return nil, err
	}
	rows, total, err := dao.SelectServerTrafficUserUsage(start, end, nodeServerID, pageNum, pageSize)
	if err != nil {
		return nil, err
	}
	return &vo.ServerTrafficUserUsagePageVo{BaseVoPage: vo.BaseVoPage{PageNum: pageNum, PageSize: pageSize, Total: total}, Rows: rows}, nil
}

func PanelGroup(c *gin.Context) (*vo.PanelGroupVo, error) {
	accountInfo, err := GetAccountInfo(c)
	if err != nil {
		return nil, err
	}
	account, err := SelectAccountById(&accountInfo.Id)
	if err != nil {
		return nil, err
	}
	nodeCount, err := CountNode()
	if err != nil {
		return nil, err
	}
	systemName := constant.SystemName
	systemConfig, err := SelectSystemByName(&systemName)
	if err != nil {
		return nil, err
	}
	panelGroupVo := vo.PanelGroupVo{
		Quota:                       *account.Quota,
		ResidualFlow:                *account.Quota - *account.Upload - *account.Download,
		NodeCount:                   nodeCount,
		ExpireTime:                  *account.ExpireTime,
		ResetDownloadAndUploadMonth: &systemConfig.ResetDownloadAndUploadMonth,
	}
	if util.IsAdmin(accountInfo.Roles) {
		var err error
		accountCount, err := CountAccountByUsername(nil)
		cpuUsed, err := util.GetCpuPercent()
		memUsed, err := util.GetMemPercent()
		diskUsed, err := util.GetDiskPercent()
		if err != nil {
			return nil, err
		}
		panelGroupVo.AccountCount = accountCount
		panelGroupVo.CpuUsed = cpuUsed
		panelGroupVo.MemUsed = memUsed
		panelGroupVo.DiskUsed = diskUsed
	}
	return &panelGroupVo, nil
}
