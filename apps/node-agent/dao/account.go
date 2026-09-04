package dao

import (
	"errors"
	"fmt"
	"github.com/didi/gendry/builder"
	"github.com/didi/gendry/scanner"
	"github.com/sirupsen/logrus"
	"trojan-panel-core/core"
	"trojan-panel-core/model"
	"trojan-panel-core/model/bo"
	"trojan-panel-core/model/constant"
	"trojan-panel-core/model/vo"
)

func UpdateAccountFlowByPassOrHash(pass *string, hash *string, download int, upload int) error {
	if download == 0 && upload == 0 {
		return nil
	}

	if download < 0 || upload < 0 {
		return errors.New(constant.SysError)
	}
	serverID := core.Config.NodeConfig.ServerID
	if serverID == 0 {
		return errors.New("node.server_id is required for traffic accounting")
	}
	tx, err := db.Begin()
	if err != nil {
		return errors.New(constant.SysError)
	}
	defer tx.Rollback()
	var lockedServerID uint
	if err = tx.QueryRow(`SELECT id FROM node_server WHERE id=? FOR UPDATE`, serverID).Scan(&lockedServerID); err != nil {
		logrus.Errorln(err)
		return errors.New(constant.SysError)
	}
	where, credential := "pass", ""
	if pass != nil && *pass != "" {
		credential = *pass
	} else if hash != nil && *hash != "" {
		where, credential = "hash", *hash
	}
	if credential == "" {
		return errors.New(constant.SysError)
	}
	var accountID uint
	var oldDownload, oldUpload uint64
	query := fmt.Sprintf("SELECT id,download,upload FROM %s WHERE %s=? FOR UPDATE", core.Config.MySQLConfig.AccountTable, where)
	if err = tx.QueryRow(query, credential).Scan(&accountID, &oldDownload, &oldUpload); err != nil {
		logrus.Errorln(err)
		return errors.New(constant.SysError)
	}
	if _, err = tx.Exec(fmt.Sprintf("UPDATE %s SET download=download+?,upload=upload+? WHERE id=?", core.Config.MySQLConfig.AccountTable), download, upload, accountID); err != nil {
		return errors.New(constant.SysError)
	}
	if _, err = tx.Exec(`INSERT INTO account_traffic_total(account_id,upload,download) VALUES(?,?,?)
		ON DUPLICATE KEY UPDATE upload=GREATEST(upload,?)+?,download=GREATEST(download,?)+?`,
		accountID, oldUpload+uint64(upload), oldDownload+uint64(download), oldUpload, upload, oldDownload, download); err != nil {
		return errors.New(constant.SysError)
	}
	if _, err = tx.Exec(`INSERT INTO account_traffic_daily(traffic_date,account_id,upload,download)
		VALUES(CURRENT_DATE(),?,?,?) ON DUPLICATE KEY UPDATE upload=upload+VALUES(upload),download=download+VALUES(download)`,
		accountID, upload, download); err != nil {
		return errors.New(constant.SysError)
	}
	if _, err = tx.Exec(`INSERT INTO account_server_traffic_daily(traffic_date,account_id,node_server_id,upload,download)
		VALUES(CURRENT_DATE(),?,?,?,?) ON DUPLICATE KEY UPDATE upload=upload+VALUES(upload),download=download+VALUES(download)`,
		accountID, serverID, upload, download); err != nil {
		return errors.New(constant.SysError)
	}
	if err = tx.Commit(); err != nil {
		return errors.New(constant.SysError)
	}
	return nil
}

// SelectAccounts query all valid accounts
func SelectAccounts() ([]bo.AccountBo, error) {
	mySQLConfig := core.Config.MySQLConfig
	var accounts []model.Account
	var (
		values []interface{}
		err    error
	)

	sql := fmt.Sprintf("select id,username,pass,hash from %s where quota < 0 or (quota > download + upload)", mySQLConfig.AccountTable)
	rows, err := db.Query(sql, values...)
	if err != nil {
		logrus.Errorln(err.Error())
		return nil, errors.New(constant.SysError)
	}
	defer rows.Close()

	if err = scanner.Scan(rows, &accounts); err != nil && err != scanner.ErrEmptyResult {
		logrus.Errorln(err.Error())
		return nil, errors.New(constant.SysError)
	}
	accountBos := make([]bo.AccountBo, 0)
	if len(accounts) > 0 {
		for _, item := range accounts {
			accountBo := bo.AccountBo{
				Username: *item.Username,
				Pass:     *item.Pass,
				Hash:     *item.Hash,
			}
			accountBos = append(accountBos, accountBo)
		}
	}
	return accountBos, nil
}

func SelectAccountByPass(pass string) (*vo.AccountHysteriaVo, error) {
	mySQLConfig := core.Config.MySQLConfig
	var account model.Account

	buildSelect, values, err := builder.NamedQuery(fmt.Sprintf("select id from %s where (quota < 0 or quota > download + upload) and pass = {{pass}}", mySQLConfig.AccountTable),
		map[string]interface{}{
			"pass": pass,
		})
	if err != nil {
		logrus.Errorln(err.Error())
		return nil, errors.New(constant.SysError)
	}
	rows, err := db.Query(buildSelect, values...)
	if err != nil {
		logrus.Errorln(err.Error())
		return nil, errors.New(constant.SysError)
	}
	defer rows.Close()

	err = scanner.Scan(rows, &account)
	if err == scanner.ErrEmptyResult {
		return nil, errors.New(constant.UsernameOrPassError)
	} else if err != nil {
		logrus.Errorln(err.Error())
		return nil, errors.New(constant.SysError)
	}

	AccountHysteriaVo := vo.AccountHysteriaVo{
		Id: *account.Id,
	}
	return &AccountHysteriaVo, nil
}
