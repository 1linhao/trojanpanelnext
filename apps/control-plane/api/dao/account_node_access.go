package dao

import (
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"trojan-panel/model/constant"
)

var ErrNodeAccessAgentsNotReady = errors.New("node agents must be upgraded and online before restricting access")
var ErrNodeAccessInvalidNode = errors.New(constant.NodeNotExist)

type AccountNodeAccess struct {
	Restricted bool   `json:"restricted"`
	NodeIDs    []uint `json:"nodeIds"`
}

type AccountNodeOption struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

func migrateAccountNodeAccess() error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS account_node_policy (
			account_id bigint unsigned PRIMARY KEY,
			FOREIGN KEY (account_id) REFERENCES account(id) ON DELETE CASCADE
		) ENGINE=InnoDB`,
		`CREATE TABLE IF NOT EXISTS account_node_grant (
			account_id bigint unsigned NOT NULL,
			node_id bigint unsigned NOT NULL,
			PRIMARY KEY (account_id, node_id),
			FOREIGN KEY (account_id) REFERENCES account_node_policy(account_id) ON DELETE CASCADE,
			FOREIGN KEY (node_id) REFERENCES node(id) ON DELETE CASCADE
		) ENGINE=InnoDB`,
		`CREATE TABLE IF NOT EXISTS node_access_agent (
			node_server_id bigint PRIMARY KEY,
			last_sync datetime NOT NULL,
			FOREIGN KEY (node_server_id) REFERENCES node_server(id) ON DELETE CASCADE
		) ENGINE=InnoDB`,
		`CREATE TABLE IF NOT EXISTS node_access_generation (
			id tinyint PRIMARY KEY, revision bigint unsigned NOT NULL
		) ENGINE=InnoDB`,
		`INSERT IGNORE INTO node_access_generation(id,revision) VALUES(1,1)`,
		`CREATE TABLE IF NOT EXISTS account_node_change (
			account_id bigint unsigned PRIMARY KEY, revision bigint unsigned NOT NULL,
			FOREIGN KEY (account_id) REFERENCES account(id) ON DELETE CASCADE
		) ENGINE=InnoDB`,
		`INSERT IGNORE INTO account_node_change(account_id,revision)
			SELECT account_id,1 FROM account_node_policy`,
		`CREATE TABLE IF NOT EXISTS node_access_ack (
			node_server_id bigint PRIMARY KEY, revision bigint unsigned NULL, reported_at datetime NOT NULL,
			FOREIGN KEY (node_server_id) REFERENCES node_server(id) ON DELETE CASCADE
		) ENGINE=InnoDB`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			return err
		}
	}
	for _, method := range []string{"GET", "POST"} {
		if _, err := db.Exec(`INSERT INTO casbin_rule (p_type,v0,v1,v2,v3,v4,v5)
			SELECT 'p','sysadmin','/api/account/nodeAccess',?,'','','' FROM DUAL
			WHERE NOT EXISTS (SELECT 1 FROM casbin_rule WHERE p_type='p' AND v0='sysadmin'
				AND v1='/api/account/nodeAccess' AND v2=?)`, method, method); err != nil {
			return err
		}
	}
	return nil
}

func SelectAccountNodeAccess(accountID uint) (AccountNodeAccess, error) {
	access := AccountNodeAccess{NodeIDs: []uint{}}
	rows, err := db.Query(`SELECT p.account_id IS NOT NULL, g.node_id
		FROM account a LEFT JOIN account_node_policy p ON p.account_id=a.id
		LEFT JOIN account_node_grant g ON g.account_id=p.account_id
		WHERE a.id=? ORDER BY g.node_id`, accountID)
	if err != nil {
		return access, err
	}
	defer rows.Close()
	found := false
	for rows.Next() {
		found = true
		var nodeID *uint
		if err := rows.Scan(&access.Restricted, &nodeID); err != nil {
			return access, err
		}
		if nodeID != nil {
			access.NodeIDs = append(access.NodeIDs, *nodeID)
		}
	}
	if err := rows.Err(); err != nil {
		return access, err
	}
	if !found {
		return access, errors.New(constant.UnauthorizedError)
	}
	return access, nil
}

func SelectAccountNodeOptions() ([]AccountNodeOption, error) {
	rows, err := db.Query(`SELECT id,name FROM node WHERE node_type_id NOT IN (?,?) ORDER BY name,id`,
		constant.TrojanGo, constant.Hysteria)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	options := []AccountNodeOption{}
	for rows.Next() {
		var option AccountNodeOption
		if err := rows.Scan(&option.ID, &option.Name); err != nil {
			return nil, err
		}
		options = append(options, option)
	}
	return options, rows.Err()
}

func UpdateAccountNodeAccess(accountID uint, access AccountNodeAccess) (uint64, error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var id uint
	if err = tx.QueryRow(`SELECT id FROM account WHERE id=? FOR UPDATE`, accountID).Scan(&id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, errors.New(constant.UnauthorizedError)
		}
		return 0, err
	}
	if _, err = tx.Exec(`DELETE FROM account_node_policy WHERE account_id=?`, id); err != nil {
		return 0, err
	}
	if !access.Restricted {
		return commitNodeAccessChange(tx, id)
	}
	var agentMissing bool
	if err = tx.QueryRow(`SELECT EXISTS (
		SELECT 1 FROM node n LEFT JOIN node_access_agent a ON a.node_server_id=n.node_server_id
		WHERE n.node_type_id NOT IN (?,?)
			AND (a.last_sync IS NULL OR a.last_sync < CURRENT_TIMESTAMP - INTERVAL 2 MINUTE)
	)`, constant.TrojanGo, constant.Hysteria).Scan(&agentMissing); err != nil {
		return 0, err
	}
	if agentMissing {
		return 0, ErrNodeAccessAgentsNotReady
	}
	if _, err = tx.Exec(`INSERT INTO account_node_policy (account_id) VALUES (?)`, id); err != nil {
		return 0, err
	}
	nodeIDs := append([]uint(nil), access.NodeIDs...)
	sort.Slice(nodeIDs, func(i, j int) bool { return nodeIDs[i] < nodeIDs[j] })
	for i, nodeID := range nodeIDs {
		if i > 0 && nodeID == nodeIDs[i-1] {
			continue
		}
		result, err := tx.Exec(`INSERT INTO account_node_grant (account_id,node_id)
			SELECT ?,id FROM node WHERE id=? AND node_type_id NOT IN (?,?)`,
			id, nodeID, constant.TrojanGo, constant.Hysteria)
		if err != nil {
			return 0, err
		}
		count, err := result.RowsAffected()
		if err != nil {
			return 0, err
		}
		if count != 1 {
			return 0, fmt.Errorf("%w: %d", ErrNodeAccessInvalidNode, nodeID)
		}
	}
	return commitNodeAccessChange(tx, id)
}

// Serialize revision assignment with commit, including across Web replicas.
// A node acknowledging a revision has seen all earlier committed changes.
func commitNodeAccessChange(tx *sql.Tx, id uint) (uint64, error) {
	if _, err := tx.Exec("UPDATE node_access_generation SET revision=revision+1 WHERE id=1"); err != nil {
		return 0, err
	}
	var revision uint64
	if err := tx.QueryRow("SELECT revision FROM node_access_generation WHERE id=1").Scan(&revision); err != nil {
		return 0, err
	}
	if _, err := tx.Exec(`INSERT INTO account_node_change(account_id,revision) VALUES(?,?)
		ON DUPLICATE KEY UPDATE revision=VALUES(revision)`, id, revision); err != nil {
		return 0, err
	}
	return revision, tx.Commit()
}

type AccountNodeSync struct {
	Revision   uint64 `json:"revision,string"`
	Restricted bool   `json:"restricted"`
	Granted    int    `json:"granted"`
	Servers    int    `json:"servers"`
	Confirmed  int    `json:"confirmed"`
	Legacy     int    `json:"legacy"`
	Unreported int    `json:"unreported"`
}

func SelectAccountNodeSync(id uint) (AccountNodeSync, error) {
	var status AccountNodeSync
	err := db.QueryRow(`SELECT COALESCE(c.revision,0), p.account_id IS NOT NULL,
		(SELECT COUNT(*) FROM account_node_grant g WHERE g.account_id=a.id),COUNT(n.node_server_id),
		COUNT(CASE WHEN k.revision >= c.revision THEN 1 END),
		COUNT(CASE WHEN k.node_server_id IS NULL AND h.node_server_id IS NOT NULL THEN 1 END),
		COUNT(CASE WHEN n.node_server_id IS NOT NULL AND k.node_server_id IS NULL AND h.node_server_id IS NULL THEN 1 END)
		FROM account a LEFT JOIN account_node_policy p ON p.account_id=a.id
		LEFT JOIN account_node_change c ON c.account_id=a.id
		LEFT JOIN (SELECT DISTINCT node_server_id FROM node WHERE node_type_id NOT IN (?,?)) n ON TRUE
		LEFT JOIN node_access_ack k ON k.node_server_id=n.node_server_id AND k.reported_at >= CURRENT_TIMESTAMP - INTERVAL 2 MINUTE
		LEFT JOIN node_access_agent h ON h.node_server_id=n.node_server_id AND h.last_sync >= CURRENT_TIMESTAMP - INTERVAL 2 MINUTE
		WHERE a.id=? GROUP BY a.id,p.account_id,c.revision`, constant.TrojanGo, constant.Hysteria, id).
		Scan(&status.Revision, &status.Restricted, &status.Granted, &status.Servers, &status.Confirmed, &status.Legacy, &status.Unreported)
	if errors.Is(err, sql.ErrNoRows) {
		return status, errors.New(constant.UnauthorizedError)
	}
	return status, err
}

func RequireAccountNodeAccess(accountID, nodeID uint) error {
	var allowed bool
	err := db.QueryRow(`SELECT NOT EXISTS (SELECT 1 FROM account_node_policy WHERE account_id=?)
		OR EXISTS (SELECT 1 FROM account_node_grant WHERE account_id=? AND node_id=?)`,
		accountID, accountID, nodeID).Scan(&allowed)
	if err != nil {
		return err
	}
	if !allowed {
		return errors.New(constant.ForbiddenError)
	}
	return nil
}

func RequireNodeAccessCompatibleServer(serverID uint) error {
	var ready bool
	err := db.QueryRow(`SELECT NOT EXISTS (SELECT 1 FROM account_node_policy)
		OR EXISTS (SELECT 1 FROM node_access_agent WHERE node_server_id=?
			AND last_sync >= CURRENT_TIMESTAMP - INTERVAL 2 MINUTE)`, serverID).Scan(&ready)
	if err != nil {
		return err
	}
	if !ready {
		return ErrNodeAccessAgentsNotReady
	}
	return nil
}
