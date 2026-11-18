package dao

import (
	"errors"
	"sort"

	"trojan-panel/model/vo"
)

var ErrServerTrafficDisplayInvalidServer = errors.New("a selected server does not exist or is being removed")

func migrateServerTrafficDisplay() error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS server_traffic_display (
			id tinyint unsigned PRIMARY KEY,
			enabled boolean NOT NULL DEFAULT FALSE
		) ENGINE=InnoDB`,
		`INSERT IGNORE INTO server_traffic_display (id) VALUES (1)`,
		`CREATE TABLE IF NOT EXISTS server_traffic_display_selection (
			node_server_id bigint PRIMARY KEY,
			FOREIGN KEY (node_server_id) REFERENCES node_server(id) ON DELETE CASCADE
		) ENGINE=InnoDB`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			return err
		}
	}
	for _, permission := range []struct{ role, path, method string }{
		{"sysadmin", "/api/node/serverTrafficDisplay", "GET"},
		{"sysadmin", "/api/node/serverTrafficDisplay", "POST"},
		{"admin", "/api/node/serverTrafficDisplay", "GET"},
		{"admin", "/api/node/serverTrafficDisplay", "POST"},
		{"sysadmin", "/api/node/serverTrafficSummaries", "GET"},
		{"admin", "/api/node/serverTrafficSummaries", "GET"},
		{"user", "/api/node/serverTrafficSummaries", "GET"},
	} {
		if _, err := db.Exec(`INSERT INTO casbin_rule (p_type,v0,v1,v2,v3,v4,v5)
			SELECT 'p',?,?,?,'','','' FROM DUAL WHERE NOT EXISTS (
				SELECT 1 FROM casbin_rule WHERE p_type='p' AND v0=? AND v1=? AND v2=?
			)`, permission.role, permission.path, permission.method,
			permission.role, permission.path, permission.method); err != nil {
			return err
		}
	}
	return nil
}

func SelectServerTrafficDisplay() (vo.ServerTrafficDisplayVo, error) {
	config := vo.ServerTrafficDisplayVo{
		ServerIDs: []uint{},
		Servers:   []vo.ServerTrafficDisplayOptionVo{},
	}
	rows, err := db.Query(`SELECT d.enabled, ns.id, ns.name, selected.node_server_id IS NOT NULL
		FROM server_traffic_display d LEFT JOIN node_server ns ON ns.removing=0
		LEFT JOIN server_traffic_display_selection selected ON selected.node_server_id=ns.id
		WHERE d.id=1 ORDER BY ns.name,ns.id`)
	if err != nil {
		return config, err
	}
	defer rows.Close()
	for rows.Next() {
		var id *uint
		var name *string
		var selected bool
		if err := rows.Scan(&config.Enabled, &id, &name, &selected); err != nil {
			return config, err
		}
		if id != nil {
			config.Servers = append(config.Servers, vo.ServerTrafficDisplayOptionVo{ID: *id, Name: *name})
			if selected {
				config.ServerIDs = append(config.ServerIDs, *id)
			}
		}
	}
	return config, rows.Err()
}

func UpdateServerTrafficDisplay(enabled bool, serverIDs []uint) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Lock the singleton first so replacing the server selection is serialized.
	if _, err = tx.Exec(`UPDATE server_traffic_display SET enabled=? WHERE id=1`, enabled); err != nil {
		return err
	}
	if _, err = tx.Exec(`DELETE FROM server_traffic_display_selection`); err != nil {
		return err
	}
	ids := append([]uint(nil), serverIDs...)
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for i, id := range ids {
		if i > 0 && ids[i-1] == id {
			continue
		}
		result, err := tx.Exec(`INSERT INTO server_traffic_display_selection (node_server_id)
			SELECT id FROM node_server WHERE id=? AND removing=0`, id)
		if err != nil {
			return err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if affected == 0 {
			return ErrServerTrafficDisplayInvalidServer
		}
	}
	return tx.Commit()
}

func SelectServerTrafficSummaries() ([]vo.ServerTrafficStatusVo, error) {
	return selectServerTrafficStatuses(`ns.removing=0
		AND EXISTS (SELECT 1 FROM server_traffic_display WHERE id=1 AND enabled=TRUE)
		AND EXISTS (SELECT 1 FROM server_traffic_display_selection WHERE node_server_id=ns.id)`)
}
