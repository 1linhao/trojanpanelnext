package dao

import (
	"crypto/subtle"
	"database/sql"
	"errors"
	"strings"
)

type RemovalCleanup struct {
	NodeID     uint
	IP         string
	Port       uint
	ServerName string
	Purge      bool
	Receipt    string
}

func migrateNodeRemovalSchema() error {
	if _, err := db.Exec(`ALTER TABLE node_server ADD COLUMN removing tinyint unsigned NOT NULL DEFAULT 0`); err != nil {
		// Match the same idempotent migration convention as the other columns.
		if !strings.Contains(err.Error(), "Duplicate column name") {
			return err
		}
	}
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS node_removal_cleanup (
		node_server_id bigint unsigned PRIMARY KEY, ip varchar(253) NOT NULL,
		port int unsigned NOT NULL, server_name varchar(253) NOT NULL,
		purge_data tinyint unsigned NOT NULL, receipt char(64) NOT NULL,
		create_time datetime NOT NULL DEFAULT CURRENT_TIMESTAMP
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`)
	return err
}

func BeginNodeServerRemoval(id uint) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var locked uint
	if err = tx.QueryRow(`SELECT id FROM node_server WHERE id=? FOR UPDATE`, id).Scan(&locked); err != nil {
		return err
	}
	var active int
	if err = tx.QueryRow(`SELECT COUNT(*) FROM kernel_upgrade_task_item WHERE node_server_id=? AND result=''`, id).Scan(&active); err != nil {
		return err
	}
	if active > 0 {
		return errors.New("wait for this server's kernel operations to finish before removal")
	}
	if _, err = tx.Exec(`UPDATE node_server SET removing=1 WHERE id=?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

func CompleteNodeServerRemoval(item RemovalCleanup) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var locked uint
	if err = tx.QueryRow(`SELECT id FROM node_server WHERE id=? AND removing=1 FOR UPDATE`, item.NodeID).Scan(&locked); err != nil {
		return err
	}
	for _, table := range []struct {
		Name string
		Type uint
	}{{"node_xray", 1}, {"node_trojan_go", 2}, {"node_hysteria", 3}, {"node_hysteria2", 5}} {
		if _, err = tx.Exec(`DELETE detail FROM `+table.Name+` detail INNER JOIN node n ON n.node_sub_id=detail.id WHERE n.node_server_id=? AND n.node_type_id=?`, item.NodeID, table.Type); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(`DELETE FROM node WHERE node_server_id=?`, item.NodeID); err != nil {
		return err
	}
	if item.Purge {
		if _, err = tx.Exec(`DELETE FROM account_server_traffic_daily WHERE node_server_id=?`, item.NodeID); err != nil {
			return err
		}
		// Snapshot only this node's task IDs before removing its task items.
		rows, queryErr := tx.Query(`SELECT DISTINCT task_id FROM kernel_upgrade_task_item WHERE node_server_id=?`, item.NodeID)
		if queryErr != nil {
			return queryErr
		}
		var tasks []uint64
		for rows.Next() {
			var id uint64
			if err = rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			tasks = append(tasks, id)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		if _, err = tx.Exec(`DELETE FROM kernel_upgrade_task_item WHERE node_server_id=?`, item.NodeID); err != nil {
			return err
		}
		if _, err = tx.Exec(`UPDATE kernel_upgrade_task SET canary_node_id=0 WHERE canary_node_id=?`, item.NodeID); err != nil {
			return err
		}
		for _, id := range tasks {
			if _, err = tx.Exec(`DELETE FROM kernel_upgrade_task WHERE id=? AND NOT EXISTS (SELECT 1 FROM kernel_upgrade_task_item WHERE task_id=?)`, id, id); err != nil {
				return err
			}
		}
	}
	if _, err = tx.Exec(`INSERT INTO node_removal_cleanup (node_server_id,ip,port,server_name,purge_data,receipt) VALUES (?,?,?,?,?,?)`, item.NodeID, item.IP, item.Port, item.ServerName, item.Purge, item.Receipt); err != nil {
		return err
	}
	if _, err = tx.Exec(`DELETE FROM node_server WHERE id=?`, item.NodeID); err != nil {
		return err
	}
	return tx.Commit()
}

func PendingRemovalCleanups() ([]RemovalCleanup, error) {
	rows, err := db.Query(`SELECT node_server_id,ip,port,server_name,purge_data,receipt FROM node_removal_cleanup ORDER BY create_time LIMIT 50`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []RemovalCleanup
	for rows.Next() {
		var item RemovalCleanup
		if err = rows.Scan(&item.NodeID, &item.IP, &item.Port, &item.ServerName, &item.Purge, &item.Receipt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

var ErrInvalidRemovalReceipt = errors.New("invalid host removal receipt")

// A missing outbox item is an idempotent success: Web may have committed this
// callback before its response was lost. An existing item requires its receipt.
func DeleteRemovalCleanup(id uint, receipt string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var expected string
	err = tx.QueryRow(`SELECT receipt FROM node_removal_cleanup WHERE node_server_id=? FOR UPDATE`, id).Scan(&expected)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if subtle.ConstantTimeCompare([]byte(receipt), []byte(expected)) != 1 {
		return ErrInvalidRemovalReceipt
	}
	if _, err = tx.Exec(`DELETE FROM node_removal_cleanup WHERE node_server_id=? AND BINARY receipt=?`, id, receipt); err != nil {
		return err
	}
	return tx.Commit()
}
