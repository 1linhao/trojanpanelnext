package dao

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/didi/gendry/scanner"
	"github.com/sirupsen/logrus"
	"trojan-panel/model/constant"
	"trojan-panel/model/vo"
)

func TrafficRank(period, startDate, endDate string) ([]vo.AccountTrafficRankVo, error) {
	var query string
	args := []any{constant.USER}
	if period == "total" {
		query = `SELECT a.username, t.upload, t.download, t.upload + t.download AS traffic_used
			FROM account_traffic_total t JOIN account a ON a.id=t.account_id
			WHERE a.role_id=? AND a.deleted=0 AND (a.quota < 0 OR a.quota > a.download + a.upload)
			ORDER BY traffic_used DESC LIMIT 15`
	} else {
		query = `SELECT a.username, SUM(d.upload) AS upload, SUM(d.download) AS download,
			SUM(d.upload + d.download) AS traffic_used
			FROM account_traffic_daily d JOIN account a ON a.id=d.account_id
			WHERE a.role_id=? AND a.deleted=0 AND (a.quota < 0 OR a.quota > a.download + a.upload)
			AND d.traffic_date>=? AND d.traffic_date<?
			GROUP BY a.id,a.username ORDER BY traffic_used DESC,a.username ASC LIMIT 15`
		args = append(args, startDate, endDate)
	}
	rows, err := db.Query(query, args...)
	if err != nil {
		logrus.Errorln(err)
		return nil, errors.New(constant.SysError)
	}
	defer rows.Close()
	result := make([]vo.AccountTrafficRankVo, 0)
	if err = scanner.Scan(rows, &result); err != nil && err != scanner.ErrEmptyResult {
		logrus.Errorln(err)
		return nil, errors.New(constant.SysError)
	}
	return result, nil
}

func ResetNodeServerTraffic(nodeServerID uint) (int64, error) {
	tx, err := db.Begin()
	if err != nil {
		logrus.Errorln(err)
		return 0, errors.New(constant.SysError)
	}
	defer tx.Rollback()

	var lockedID uint
	if err = tx.QueryRow(`SELECT id FROM node_server WHERE id=? FOR UPDATE`, nodeServerID).Scan(&lockedID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, errors.New(constant.NodeNotExist)
		}
		logrus.Errorln(err)
		return 0, errors.New(constant.SysError)
	}
	result, err := tx.Exec(`DELETE FROM account_server_traffic_daily WHERE node_server_id=?`, lockedID)
	if err != nil {
		logrus.Errorln(err)
		return 0, errors.New(constant.SysError)
	}
	deletedRows, err := result.RowsAffected()
	if err != nil {
		logrus.Errorln(err)
		return 0, errors.New(constant.SysError)
	}
	if err = tx.Commit(); err != nil {
		logrus.Errorln(err)
		return 0, errors.New(constant.SysError)
	}
	return deletedRows, nil
}

func serverTrafficConditions(startDate, endDate string, nodeServerID *uint) (string, []any) {
	conditions := []string{"1=1"}
	args := make([]any, 0, 3)
	if nodeServerID != nil && *nodeServerID != 0 {
		conditions = append(conditions, "d.node_server_id=?")
		args = append(args, *nodeServerID)
	}
	if startDate != "" && endDate != "" {
		conditions = append(conditions, "d.traffic_date>=?", "d.traffic_date<?")
		args = append(args, startDate, endDate)
	}
	return strings.Join(conditions, " AND "), args
}

func SelectServerTrafficUsage(startDate, endDate string, nodeServerID *uint, pageNum, pageSize uint) ([]vo.ServerTrafficUsageVo, uint, error) {
	where, args := serverTrafficConditions(startDate, endDate, nodeServerID)
	groupQuery := ` FROM account_server_traffic_daily d
		JOIN node_server ns ON ns.id=d.node_server_id
		WHERE ` + where + ` GROUP BY d.node_server_id,ns.name`
	var total uint
	if err := db.QueryRow("SELECT COUNT(*) FROM (SELECT d.node_server_id"+groupQuery+") grouped", args...).Scan(&total); err != nil {
		logrus.Errorln(err)
		return nil, 0, errors.New(constant.SysError)
	}
	query := `SELECT d.node_server_id,ns.name AS node_server_name,
		SUM(d.upload) AS upload,SUM(d.download) AS download,SUM(d.upload+d.download) AS total` + groupQuery +
		` ORDER BY total DESC,ns.name ASC,d.node_server_id ASC LIMIT ?,?`
	queryArgs := append(append([]any{}, args...), (pageNum-1)*pageSize, pageSize)
	rows, err := db.Query(query, queryArgs...)
	if err != nil {
		logrus.Errorln(err)
		return nil, 0, errors.New(constant.SysError)
	}
	defer rows.Close()
	result := make([]vo.ServerTrafficUsageVo, 0)
	if err = scanner.Scan(rows, &result); err != nil && err != scanner.ErrEmptyResult {
		logrus.Errorln(err)
		return nil, 0, errors.New(constant.SysError)
	}
	return result, total, nil
}

func SelectServerTrafficUserUsage(startDate, endDate string, nodeServerID, pageNum, pageSize uint) ([]vo.ServerTrafficUserUsageVo, uint, error) {
	where, args := serverTrafficConditions(startDate, endDate, &nodeServerID)
	groupQuery := ` FROM account_server_traffic_daily d
		JOIN account a ON a.id=d.account_id
		WHERE ` + where + ` GROUP BY d.account_id,a.username`
	var total uint
	if err := db.QueryRow("SELECT COUNT(*) FROM (SELECT d.account_id"+groupQuery+") grouped", args...).Scan(&total); err != nil {
		logrus.Errorln(err)
		return nil, 0, errors.New(constant.SysError)
	}
	query := `SELECT d.account_id,a.username,
		SUM(d.upload) AS upload,SUM(d.download) AS download,SUM(d.upload+d.download) AS total` + groupQuery +
		` ORDER BY total DESC,a.username ASC,d.account_id ASC LIMIT ?,?`
	queryArgs := append(append([]any{}, args...), (pageNum-1)*pageSize, pageSize)
	rows, err := db.Query(query, queryArgs...)
	if err != nil {
		logrus.Errorln(err)
		return nil, 0, errors.New(constant.SysError)
	}
	defer rows.Close()
	result := make([]vo.ServerTrafficUserUsageVo, 0)
	if err = scanner.Scan(rows, &result); err != nil && err != scanner.ErrEmptyResult {
		logrus.Errorln(err)
		return nil, 0, errors.New(constant.SysError)
	}
	return result, total, nil
}

func SelectServerTrafficStatuses(serverIDs []uint) ([]vo.ServerTrafficStatusVo, error) {
	if len(serverIDs) == 0 {
		return []vo.ServerTrafficStatusVo{}, nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(serverIDs)), ",")
	args := make([]any, len(serverIDs))
	for i, id := range serverIDs {
		args[i] = id
	}
	query := fmt.Sprintf(`SELECT ns.id AS node_server_id,ns.name AS node_server_name,ns.traffic_period,
		ns.traffic_limit_mode,ns.traffic_total_limit,ns.traffic_upload_limit,ns.traffic_download_limit,
		COALESCE(SUM(CASE
			WHEN ns.traffic_period='day' AND d.traffic_date=CURRENT_DATE() THEN d.upload
			WHEN ns.traffic_period='month' AND d.traffic_date>=DATE_FORMAT(CURRENT_DATE(),'%%Y-%%m-01') THEN d.upload
			WHEN ns.traffic_period='year' AND d.traffic_date>=MAKEDATE(YEAR(CURRENT_DATE()),1) THEN d.upload
			ELSE 0 END),0) AS upload_used,
		COALESCE(SUM(CASE
			WHEN ns.traffic_period='day' AND d.traffic_date=CURRENT_DATE() THEN d.download
			WHEN ns.traffic_period='month' AND d.traffic_date>=DATE_FORMAT(CURRENT_DATE(),'%%Y-%%m-01') THEN d.download
			WHEN ns.traffic_period='year' AND d.traffic_date>=MAKEDATE(YEAR(CURRENT_DATE()),1) THEN d.download
			ELSE 0 END),0) AS download_used
		FROM node_server ns LEFT JOIN account_server_traffic_daily d ON d.node_server_id=ns.id
		WHERE ns.id IN (%s) GROUP BY ns.id,ns.name,ns.traffic_period,ns.traffic_limit_mode,
		ns.traffic_total_limit,ns.traffic_upload_limit,ns.traffic_download_limit`, placeholders)
	rows, err := db.Query(query, args...)
	if err != nil {
		logrus.Errorln(err)
		return nil, errors.New(constant.SysError)
	}
	defer rows.Close()
	result := make([]vo.ServerTrafficStatusVo, 0)
	if err = scanner.Scan(rows, &result); err != nil && err != scanner.ErrEmptyResult {
		logrus.Errorln(err)
		return nil, errors.New(constant.SysError)
	}
	return result, nil
}
