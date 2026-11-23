package dao

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
	"trojan-panel/service/clientrouting"

	"trojan-panel/model/constant"
)

func migrateClientRouting() error {
	for _, q := range []string{
		`CREATE TABLE IF NOT EXISTS client_routing_release (
			id bigint unsigned PRIMARY KEY AUTO_INCREMENT, name varchar(128) NOT NULL,
			source longtext NOT NULL, snapshot longtext NOT NULL,
			created_at datetime NOT NULL DEFAULT CURRENT_TIMESTAMP
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS client_routing_state (
			id tinyint PRIMARY KEY, release_id bigint unsigned NULL,
			FOREIGN KEY (release_id) REFERENCES client_routing_release(id)
		) ENGINE=InnoDB`,
		`INSERT IGNORE INTO client_routing_state (id) VALUES (1)`,
	} {
		if _, err := db.Exec(q); err != nil {
			return err
		}
	}
	for _, perm := range []struct{ path, method string }{
		{"/api/system/clientRouting", "GET"}, {"/api/system/clientRouting/publish", "POST"},
		{"/api/system/clientRouting/activate", "POST"}, {"/api/system/clientRouting/delete", "POST"},
	} {
		if _, err := db.Exec(`INSERT INTO casbin_rule (p_type,v0,v1,v2,v3,v4,v5)
			SELECT 'p','sysadmin',?,?,'','','' FROM DUAL WHERE NOT EXISTS
			(SELECT 1 FROM casbin_rule WHERE p_type='p' AND v0='sysadmin' AND v1=? AND v2=?)`,
			perm.path, perm.method, perm.path, perm.method); err != nil {
			return err
		}
	}
	return nil
}

type RoutingNode struct {
	ID         uint   `json:"id"`
	Name       string `json:"name"`
	ServerID   uint   `json:"serverId"`
	ServerName string `json:"serverName"`
}

func RoutingNodes() ([]RoutingNode, error) {
	return routingNodes(db)
}

type routingReader interface {
	Query(string, ...interface{}) (*sql.Rows, error)
}

func routingNodes(reader routingReader) ([]RoutingNode, error) {
	rows, err := reader.Query(`SELECT n.id,n.name,s.id,s.name FROM node n JOIN node_server s ON s.id=n.node_server_id
		WHERE n.node_type_id NOT IN (?,?) ORDER BY s.name,n.priority DESC,n.id`, constant.TrojanGo, constant.Hysteria)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	nodes := []RoutingNode{}
	for rows.Next() {
		var n RoutingNode
		if err := rows.Scan(&n.ID, &n.Name, &n.ServerID, &n.ServerName); err != nil {
			return nil, err
		}
		nodes = append(nodes, n)
	}
	return nodes, rows.Err()
}

func RoutingNodeServers(nodes []RoutingNode) map[uint]uint {
	servers := make(map[uint]uint, len(nodes))
	for _, node := range nodes {
		servers[node.ID] = node.ServerID
	}
	return servers
}

type RoutingDeleteImpact struct {
	Empty       string   `json:"empty"`
	EmptyGroups []string `json:"emptyGroups"`
	OtherGroups []string `json:"otherGroups"`
}

func NodeRoutingDeleteImpact(id uint) (RoutingDeleteImpact, error) {
	impact := RoutingDeleteImpact{EmptyGroups: []string{}, OtherGroups: []string{}}
	tx, err := db.Begin()
	if err != nil {
		return impact, err
	}
	defer tx.Rollback()
	var data []byte
	err = tx.QueryRow(`SELECT r.snapshot FROM client_routing_state s LEFT JOIN client_routing_release r ON s.release_id=r.id WHERE s.id=1`).Scan(&data)
	if err != nil || len(data) == 0 {
		return impact, err
	}
	var snapshot clientrouting.Snapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return impact, err
	}
	nodes, err := routingNodes(tx)
	if err != nil {
		return impact, err
	}
	servers := RoutingNodeServers(nodes)
	impact.Empty = snapshot.Empty
	for _, g := range snapshot.Groups {
		var affected, remaining bool
		for _, member := range g.NodeIDs {
			if servers[member] != g.ServerID {
				continue
			}
			if member == id {
				affected = true
			} else {
				remaining = true
			}
		}
		if affected {
			if remaining {
				impact.OtherGroups = append(impact.OtherGroups, g.Name)
			} else {
				impact.EmptyGroups = append(impact.EmptyGroups, g.Name)
			}
		}
	}
	return impact, nil
}

type RoutingRelease struct {
	ID        uint      `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
	Active    bool      `json:"active"`
}

func RoutingHistory() ([]RoutingRelease, error) {
	rows, err := db.Query(`SELECT r.id,r.name,r.created_at,COALESCE(s.release_id=r.id,0)
		FROM client_routing_release r CROSS JOIN client_routing_state s WHERE s.id=1 ORDER BY r.id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	releases := []RoutingRelease{}
	for rows.Next() {
		var r RoutingRelease
		if err := rows.Scan(&r.ID, &r.Name, &r.CreatedAt, &r.Active); err != nil {
			return nil, err
		}
		releases = append(releases, r)
	}
	return releases, rows.Err()
}

func RoutingSource(id uint) ([]byte, error) {
	var source []byte
	err := db.QueryRow("SELECT source FROM client_routing_release WHERE id=?", id).Scan(&source)
	return source, err
}

func ActiveRoutingSnapshot() ([]byte, error) {
	var snapshot []byte
	err := db.QueryRow(`SELECT r.snapshot FROM client_routing_state s LEFT JOIN client_routing_release r
		ON r.id=s.release_id WHERE s.id=1`).Scan(&snapshot)
	return snapshot, err
}

func lockRoutingState(tx *sql.Tx) (uint, error) {
	var active uint
	err := tx.QueryRow("SELECT COALESCE(release_id,0) FROM client_routing_state WHERE id=1 FOR UPDATE").Scan(&active)
	return active, err
}

func PublishRouting(name string, source, snapshot []byte) (uint, error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	if _, err := lockRoutingState(tx); err != nil {
		return 0, err
	}
	row, err := tx.Exec("INSERT INTO client_routing_release (name,source,snapshot) VALUES (?,?,?)", name, source, snapshot)
	if err != nil {
		return 0, err
	}
	id, err := row.LastInsertId()
	if err != nil {
		return 0, err
	}
	if _, err := tx.Exec("UPDATE client_routing_state SET release_id=? WHERE id=1", id); err != nil {
		return 0, err
	}
	return uint(id), tx.Commit()
}

var ErrStaleRouting = errors.New("stale routing references")

var ErrActiveRouting = errors.New("活动版本不能删除")

// An ID of zero on activation restores the built-in client templates.
func ChangeRoutingRelease(id uint, remove bool) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	active, err := lockRoutingState(tx)
	if err != nil {
		return err
	}
	if remove && id == active {
		return ErrActiveRouting
	}
	if id != 0 && !remove {
		var source []byte
		if err := tx.QueryRow("SELECT source FROM client_routing_release WHERE id=?", id).Scan(&source); err != nil {
			return err
		}
		var p clientrouting.Profile
		if err := json.Unmarshal(source, &p); err != nil {
			return err
		}
		nodes, err := routingNodes(tx)
		if err != nil {
			return err
		}
		if err := p.Traffic.CheckMembers(RoutingNodeServers(nodes)); err != nil {
			return fmt.Errorf("%w: %v", ErrStaleRouting, err)
		}
	} else if id != 0 {
		var found uint
		if err := tx.QueryRow("SELECT id FROM client_routing_release WHERE id=?", id).Scan(&found); err != nil {
			return err
		}
	}
	if remove {
		_, err = tx.Exec("DELETE FROM client_routing_release WHERE id=?", id)
	} else {
		_, err = tx.Exec("UPDATE client_routing_state SET release_id=NULLIF(?,0) WHERE id=1", id)
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}
