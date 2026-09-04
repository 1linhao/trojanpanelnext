package dao

import (
	"errors"
	"trojan-panel-core/core"
	"trojan-panel-core/model/bo"
)

func CurrentServerQuota() (bo.QuotaStatus, error) {
	status := bo.QuotaStatus{ServerID: core.Config.NodeConfig.ServerID}
	if status.ServerID == 0 {
		return status, errors.New("node.server_id is required")
	}
	if err := db.QueryRow(`SELECT traffic_period,traffic_limit_mode,traffic_total_limit,traffic_upload_limit,traffic_download_limit
		FROM node_server WHERE id=?`, status.ServerID).Scan(&status.Period, &status.Mode, &status.TotalLimit, &status.UploadLimit, &status.DownloadLimit); err != nil {
		return status, err
	}
	if status.Period == "none" {
		return status, nil
	}
	if status.Period != "day" && status.Period != "month" && status.Period != "year" {
		return status, errors.New("invalid node server traffic period")
	}
	if status.Mode != "combined" && status.Mode != "separate" {
		return status, errors.New("invalid node server traffic limit mode")
	}
	dateCondition := "traffic_date=CURRENT_DATE()"
	switch status.Period {
	case "month":
		dateCondition = "traffic_date>=DATE_FORMAT(CURRENT_DATE(), '%Y-%m-01')"
	case "year":
		dateCondition = "traffic_date>=MAKEDATE(YEAR(CURRENT_DATE()),1)"
	}
	query := `SELECT COALESCE(SUM(upload),0),COALESCE(SUM(download),0) FROM account_server_traffic_daily WHERE node_server_id=? AND ` + dateCondition
	if err := db.QueryRow(query, status.ServerID).Scan(&status.UploadUsed, &status.DownloadUsed); err != nil {
		return status, err
	}
	status.Reached = quotaReached(status)
	return status, nil
}

func quotaReached(status bo.QuotaStatus) bool {
	if status.Period == "none" {
		return false
	}
	if status.Mode == "separate" {
		return (status.UploadLimit > 0 && status.UploadUsed >= status.UploadLimit) || (status.DownloadLimit > 0 && status.DownloadUsed >= status.DownloadLimit)
	}
	return status.TotalLimit > 0 && status.UploadUsed+status.DownloadUsed >= status.TotalLimit
}
