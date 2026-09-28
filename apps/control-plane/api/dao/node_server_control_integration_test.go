package dao

import (
	"database/sql"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"

	_ "github.com/go-sql-driver/mysql"
)

// The receipt CLI integration test invokes this against its live MariaDB at
// each lifecycle transition. Registered non-active identities cannot be used
// for outbound control, while an unmanaged legacy server remains compatible.
func TestNodeControlTargetsRespectIdentityStatus(t *testing.T) {
	dsn := os.Getenv("TP_NODE_CONTROL_TEST_DSN")
	if dsn == "" {
		t.Skip("TP_NODE_CONTROL_TEST_DSN is not set")
	}
	testDB, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer testDB.Close()
	db = testDB
	legacy, err := db.Exec(`INSERT INTO node_server (name, ip) VALUES ('legacy-control-test', '127.0.0.1')`)
	if err != nil {
		t.Fatal(err)
	}
	legacyID, err := legacy.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Exec(`DELETE FROM node_server WHERE id=?`, legacyID)
	if _, err := db.Exec(`INSERT INTO node (node_server_id) VALUES (?)`, legacyID); err != nil {
		t.Fatal(err)
	}
	defer db.Exec(`DELETE FROM node WHERE node_server_id=?`, legacyID)
	servers, err := SelectNodeServersForControl()
	if err != nil {
		t.Fatal(err)
	}
	actual := make([]string, 0, len(servers))
	for _, server := range servers {
		actual = append(actual, strconv.FormatUint(uint64(*server.Id), 10))
	}
	sort.Strings(actual)
	want := strings.Split(os.Getenv("TP_NODE_CONTROL_TEST_EXPECT"), ",")
	if len(want) == 1 && want[0] == "" {
		want = nil
	}
	want = append(want, strconv.FormatInt(legacyID, 10))
	sort.Strings(want)
	if strings.Join(actual, ",") != strings.Join(want, ",") {
		t.Fatalf("control targets %v, want %v", actual, want)
	}
	rows, err := db.Query(`SELECT id FROM node_server WHERE id <> ?`, legacyID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	allowed := make(map[string]bool, len(want))
	for _, id := range want {
		allowed[id] = true
	}
	for rows.Next() {
		var id uint
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		_, err := SelectNodeServerForControl(id)
		if allowed[strconv.FormatUint(uint64(id), 10)] != (err == nil) {
			t.Fatalf("server %d direct control eligibility error = %v", id, err)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if _, err := SelectNodeServerForControl(uint(legacyID)); err != nil {
		t.Fatalf("unmanaged legacy server was rejected: %v", err)
	}
}
