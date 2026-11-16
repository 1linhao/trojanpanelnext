package dao

import (
	"errors"
	"os"
	"reflect"
	"testing"
)

func TestAccountNodeAccessLifecycle(t *testing.T) {
	dsn := os.Getenv("TP_REMOVAL_TEST_DSN")
	if dsn == "" {
		t.Skip("TP_REMOVAL_TEST_DSN is not set")
	}
	testDB := newNodeRemovalTestDB(t, dsn)
	for _, query := range []string{
		`CREATE TABLE account (id bigint unsigned PRIMARY KEY) ENGINE=InnoDB`,
		`CREATE TABLE node_server (id bigint PRIMARY KEY) ENGINE=InnoDB`,
		`CREATE TABLE node (id bigint unsigned PRIMARY KEY, node_server_id bigint NOT NULL,
			node_type_id bigint NOT NULL, name varchar(64)) ENGINE=InnoDB`,
		`CREATE TABLE casbin_rule (p_type varchar(16), v0 varchar(128), v1 varchar(128),
			v2 varchar(128), v3 varchar(128), v4 varchar(128), v5 varchar(128))`,
		`INSERT INTO account VALUES (1),(2)`,
		`INSERT INTO node_server VALUES (1)`,
		`INSERT INTO node VALUES (1,1,1,'alpha'),(2,1,5,'beta'),(3,1,3,'retired')`,
	} {
		removalExec(t, testDB, query)
	}
	for i := 0; i < 2; i++ {
		if err := migrateAccountNodeAccess(); err != nil {
			t.Fatal(err)
		}
	}
	assertRemovalRows(t, testDB, "SELECT COUNT(*) FROM casbin_rule WHERE v0='sysadmin'", [][]string{{"2"}})
	checkAccess := func(restricted bool, ids ...uint) {
		t.Helper()
		access, err := SelectAccountNodeAccess(1)
		if err != nil || access.Restricted != restricted || !reflect.DeepEqual(access.NodeIDs, append([]uint{}, ids...)) {
			t.Fatalf("unexpected access: %+v, %v", access, err)
		}
	}
	checkAccess(false)
	if err := RequireAccountNodeAccess(1, 2); err != nil {
		t.Fatal(err)
	}
	access := AccountNodeAccess{Restricted: true, NodeIDs: []uint{2, 1, 2}}
	if err := UpdateAccountNodeAccess(1, access); !errors.Is(err, ErrNodeAccessAgentsNotReady) {
		t.Fatalf("expected upgrade gate, got %v", err)
	}
	removalExec(t, testDB, "INSERT INTO node_access_agent VALUES (1,CURRENT_TIMESTAMP)")
	if err := UpdateAccountNodeAccess(1, access); err != nil {
		t.Fatal(err)
	}
	checkAccess(true, 1, 2)
	if err := RequireNodeAccessCompatibleServer(1); err != nil {
		t.Fatal(err)
	}
	if err := RequireNodeAccessCompatibleServer(2); !errors.Is(err, ErrNodeAccessAgentsNotReady) {
		t.Fatalf("expected to reject an older server while policies are active, got %v", err)
	}
	if err := UpdateAccountNodeAccess(1, AccountNodeAccess{Restricted: true, NodeIDs: []uint{3}}); !errors.Is(err, ErrNodeAccessInvalidNode) {
		t.Fatalf("expected retired node to be rejected, got %v", err)
	}
	checkAccess(true, 1, 2)
	removalExec(t, testDB, "DELETE FROM node WHERE id IN (1,2)")
	checkAccess(true)
	if err := RequireAccountNodeAccess(1, 2); err == nil {
		t.Fatal("restricted account retained access after its nodes were deleted")
	}
	if err := RequireAccountNodeAccess(2, 2); err != nil {
		t.Fatalf("changed another account's access: %v", err)
	}
	if err := UpdateAccountNodeAccess(1, AccountNodeAccess{}); err != nil {
		t.Fatal(err)
	}
	checkAccess(false)
}
