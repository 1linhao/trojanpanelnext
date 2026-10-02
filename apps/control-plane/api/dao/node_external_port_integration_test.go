package dao

import (
	"os"
	"strings"
	"testing"
	"trojan-panel/model"
)

// This reuses the removal suite's disposable, per-test database connection.
func TestNodeExternalPortIntegration(t *testing.T) {
	dsn := os.Getenv("TP_REMOVAL_TEST_DSN")
	if dsn == "" {
		t.Skip("TP_REMOVAL_TEST_DSN is not set")
	}
	start := strings.Index(sqlInitStr, "CREATE TABLE `node` (")
	if start < 0 {
		t.Fatal("bootstrap node schema is missing")
	}
	end := strings.Index(sqlInitStr[start:], ";")
	if end < 0 {
		t.Fatal("bootstrap node schema is incomplete")
	}
	schema := sqlInitStr[start : start+end]

	t.Run("existing_database_and_repeat_migration", func(t *testing.T) {
		testDB := newNodeRemovalTestDB(t, dsn)
		var legacy []string
		for _, line := range strings.Split(schema, "\n") {
			if !strings.Contains(line, "`external_port`") {
				legacy = append(legacy, line)
			}
		}
		removalExec(t, testDB, strings.Join(legacy, "\n"))
		removalExec(t, testDB, `INSERT INTO node (id,node_server_id,node_sub_id,node_type_id,name,node_server_ip,domain,port) VALUES (41,7,1,1,'existing','127.0.0.1','node.example.test',445)`)
		for i := 0; i < 2; i++ {
			if err := migrateNodeExternalPortColumn(); err != nil {
				t.Fatal(err)
			}
		}
		assertRemovalRows(t, testDB, `SELECT id,port,external_port FROM node`, [][]string{{"41", "445", "0"}})
		removalExec(t, testDB, `UPDATE node SET external_port=65535 WHERE id=41`)
		if err := migrateNodeExternalPortColumn(); err != nil {
			t.Fatal(err)
		}
		assertRemovalRows(t, testDB, `SELECT id,port,external_port FROM node`, [][]string{{"41", "445", "65535"}})
	})

	t.Run("bootstrap_crud_and_client_selection", func(t *testing.T) {
		testDB := newNodeRemovalTestDB(t, dsn)
		removalExec(t, testDB, schema)
		for _, migrate := range []func() error{migrateNodeExternalPortColumn, migrateNodeUotColumns, migrateNodeClientTypesColumn} {
			if err := migrate(); err != nil {
				t.Fatal(err)
			}
		}
		removalExec(t, testDB, `CREATE TABLE node_server (id bigint unsigned PRIMARY KEY, removing tinyint unsigned NOT NULL DEFAULT 0)`)
		removalExec(t, testDB, `INSERT INTO node_server (id) VALUES (7)`)
		ptr := func(v uint) *uint { return &v }
		text := func(v string) *string { return &v }
		for _, node := range []*model.Node{
			{NodeServerId: ptr(7), NodeSubId: ptr(1), NodeTypeId: ptr(1), Name: text("single"), NodeServerIp: text("127.0.0.1"), Domain: text("node.example.test"), Port: ptr(445)},
			{NodeServerId: ptr(7), NodeSubId: ptr(2), NodeTypeId: ptr(1), Name: text("forwarded"), NodeServerIp: text("127.0.0.1"), Domain: text("node.example.test"), Port: ptr(446), ExternalPort: ptr(443)},
			{NodeServerId: ptr(7), NodeSubId: ptr(3), NodeTypeId: ptr(1), Name: text("shared_public"), NodeServerIp: text("127.0.0.1"), Domain: text("other.example.test"), Port: ptr(447), ExternalPort: ptr(443)},
		} {
			if err := CreateNode(node); err != nil {
				t.Fatal(err)
			}
		}
		assertRemovalRows(t, testDB, `SELECT port,external_port FROM node ORDER BY id`, [][]string{{"445", "0"}, {"446", "443"}, {"447", "443"}})
		id := uint(2)
		if err := UpdateNodeById(&model.Node{Id: &id, Name: text("renamed")}); err != nil {
			t.Fatal(err)
		}
		node, err := SelectNodeById(&id)
		if err != nil || node.ExternalPort == nil || *node.ExternalPort != 443 {
			t.Fatalf("omitted update lost external port: %+v %v", node, err)
		}
		if err := UpdateNodeById(&model.Node{Id: &id, ExternalPort: ptr(0)}); err != nil {
			t.Fatal(err)
		}
		node, err = SelectNodeById(&id)
		if err != nil || node.ClientPort() != 446 || *node.ExternalPort != 0 {
			t.Fatalf("explicit zero did not disable forwarding: %+v %v", node, err)
		}
		if err := UpdateNodeById(&model.Node{Id: &id, ExternalPort: ptr(65535)}); err != nil {
			t.Fatal(err)
		}
		page, total, err := SelectNodePage(nil, nil, ptr(1), ptr(20))
		if err != nil || total != 3 {
			t.Fatalf("page total=%d err=%v", total, err)
		}
		found := false
		for _, item := range *page {
			if *item.Id == id {
				found = item.ClientPort() == 65535 && *item.Port == 446
			}
		}
		if !found {
			t.Fatal("page omitted public port or changed actual port")
		}
		nodes, err := SelectNodes()
		if err != nil {
			t.Fatal(err)
		}
		found = false
		for _, item := range nodes {
			if *item.Id == id {
				found = item.ClientPort() == 65535 && *item.Port == 446
			}
		}
		if !found {
			t.Fatal("subscription DAO selection omitted public port")
		}
		count, err := CountNodeByIpAndPort(text("127.0.0.1"), ptr(443))
		if err != nil || count != 0 {
			t.Fatalf("conflict check used public port: count=%d err=%v", count, err)
		}
		count, err = CountNodeByIpAndPort(text("127.0.0.1"), ptr(446))
		if err != nil || count != 1 {
			t.Fatalf("conflict check lost actual port: count=%d err=%v", count, err)
		}
	})
}
