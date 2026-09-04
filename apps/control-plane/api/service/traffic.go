package service

import (
	"time"

	"trojan-panel/dao"
	"trojan-panel/model/vo"
)

func ServerTrafficStatuses(serverIDs []uint) (map[uint]vo.ServerTrafficStatusVo, error) {
	rows, err := dao.SelectServerTrafficStatuses(serverIDs)
	if err != nil {
		return nil, err
	}
	result := make(map[uint]vo.ServerTrafficStatusVo, len(rows))
	for _, row := range rows {
		hydrateTrafficStatus(&row)
		result[row.NodeServerId] = row
	}
	return result, nil
}

func hydrateTrafficStatus(status *vo.ServerTrafficStatusVo) {
	location, _ := time.LoadLocation("Asia/Shanghai")
	now := time.Now().In(location)
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
	end := start.AddDate(0, 0, 1)
	switch status.Period {
	case "month":
		start = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, location)
		end = start.AddDate(0, 1, 0)
	case "year":
		start = time.Date(now.Year(), 1, 1, 0, 0, 0, 0, location)
		end = start.AddDate(1, 0, 0)
	case "none":
		start, end = time.Time{}, time.Time{}
	}
	status.WindowStart, status.WindowEnd = start, end
	status.TotalUsed = status.UploadUsed + status.DownloadUsed
	if status.LimitMode == "separate" {
		if status.UploadLimit > 0 {
			status.UploadRemaining = remaining(status.UploadLimit, status.UploadUsed)
			status.Reached = status.Reached || status.UploadUsed >= status.UploadLimit
		}
		if status.DownloadLimit > 0 {
			status.DownloadRemaining = remaining(status.DownloadLimit, status.DownloadUsed)
			status.Reached = status.Reached || status.DownloadUsed >= status.DownloadLimit
		}
	} else if status.TotalLimit > 0 {
		status.TotalRemaining = remaining(status.TotalLimit, status.TotalUsed)
		status.Reached = status.TotalUsed >= status.TotalLimit
	}
	if status.Period == "none" {
		status.Reached = false
	}
}

func remaining(limit, used uint64) uint64 {
	if used >= limit {
		return 0
	}
	return limit - used
}
