package dao

import (
	"errors"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"github.com/sirupsen/logrus"
	"strings"
	"trojan-panel-core/core"
	"trojan-panel-core/model/bo"
	"trojan-panel-core/model/constant"
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

// Match the internal listener, independently of its public forwarding port.
const listenerAccountAccess = ` AND (
	NOT EXISTS (SELECT 1 FROM account_node_policy p WHERE p.account_id=a.id)
	OR EXISTS (SELECT 1 FROM account_node_grant g JOIN node n ON n.id=g.node_id
		WHERE g.account_id=a.id AND n.node_server_id=? AND n.node_type_id=? AND n.port=?)
)`

func SelectListenerAccounts(nodeType, port uint) ([]bo.AccountBo, error) {
	query := fmt.Sprintf(`SELECT a.username,a.pass FROM %s a WHERE (a.quota < 0 OR a.quota > a.download + a.upload)`,
		core.Config.MySQLConfig.AccountTable)
	rows, err := db.Query(query+listenerAccountAccess, core.Config.NodeConfig.ServerID, nodeType, port)
	if missingNodeAccessTable(err, "account_node_policy") {
		rows, err = db.Query(query)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var accounts []bo.AccountBo
	for rows.Next() {
		var account bo.AccountBo
		if err := rows.Scan(&account.Username, &account.Pass); err != nil {
			return nil, err
		}
		accounts = append(accounts, account)
	}
	return accounts, rows.Err()
}

func AuthorizeHysteria2Account(pass string, port uint) error {
	query := fmt.Sprintf(`SELECT a.id FROM %s a WHERE a.pass=? AND (a.quota < 0 OR a.quota > a.download + a.upload)`,
		core.Config.MySQLConfig.AccountTable)
	var id uint
	err := db.QueryRow(query+listenerAccountAccess, pass, core.Config.NodeConfig.ServerID, constant.Hysteria2, port).Scan(&id)
	if missingNodeAccessTable(err, "account_node_policy") {
		err = db.QueryRow(query, pass).Scan(&id)
	}
	return err
}

func MarkNodeAccessSynced() error {
	serverID := core.Config.NodeConfig.ServerID
	if serverID == 0 {
		return nil
	}
	_, err := db.Exec(`INSERT INTO node_access_agent (node_server_id,last_sync)
		SELECT id,CURRENT_TIMESTAMP FROM node_server WHERE id=?
		ON DUPLICATE KEY UPDATE last_sync=CURRENT_TIMESTAMP`, serverID)
	if missingNodeAccessTable(err, "node_access_agent") {
		return nil
	}
	return err
}

// Allow a new agent to start against a Web version that predates the feature.
func missingNodeAccessTable(err error, table string) bool {
	var mysqlErr *mysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1146 && strings.HasSuffix(mysqlErr.Message, "."+table+"' doesn't exist")
}
