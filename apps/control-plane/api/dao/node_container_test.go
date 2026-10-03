package dao

import (
	"os"
	"testing"
)

func TestNodeContainerPermissionsAndActiveKernelDAO(t *testing.T) {
	dsn := os.Getenv("TP_REMOVAL_TEST_DSN")
	if dsn == "" {
		t.Skip("TP_REMOVAL_TEST_DSN is not set")
	}
	testDB := newNodeRemovalTestDB(t, dsn)
	removalExec(t, testDB, `CREATE TABLE casbin_rule(p_type varchar(32),v0 varchar(255),v1 varchar(255),v2 varchar(255),v3 varchar(255),v4 varchar(255),v5 varchar(255))`)
	removalExec(t, testDB, `INSERT INTO casbin_rule VALUES('p','admin','/api/nodeServer/selectNodeServerPage','GET','','','')`)
	for i := 0; i < 2; i++ {
		if err := migrateNodeContainerPermissions(); err != nil {
			t.Fatal(err)
		}
	}
	assertRemovalRows(t, testDB, `SELECT v0,v1,v2 FROM casbin_rule ORDER BY v0,v1`, [][]string{{"admin", "/api/nodeServer/selectNodeServerPage", "GET"}, {"sysadmin", "/api/container/inventory", "GET"}, {"sysadmin", "/api/container/update", "POST"}})
	removalExec(t, testDB, `CREATE TABLE kernel_upgrade_task_item(node_server_id bigint unsigned,result varchar(32))`)
	removalExec(t, testDB, `INSERT INTO kernel_upgrade_task_item VALUES(7,''),(8,'failed'),(8,'succeeded')`)
	for _, test := range []struct {
		id     uint
		active bool
	}{{7, true}, {8, false}, {9, false}} {
		active, err := NodeHasActiveKernelTask(test.id)
		if err != nil || active != test.active {
			t.Fatalf("node %d active %v error %v", test.id, active, err)
		}
	}
}
