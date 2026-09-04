package dao

import (
	"database/sql"
	"os"
	"testing"

	_ "github.com/go-sql-driver/mysql"
	"trojan-panel-core/core"
)

func TestUpdateAccountFlowWritesAllLedgers(t *testing.T) {
	dsn := os.Getenv("TP_TRAFFIC_CORE_TEST_DSN")
	if dsn == "" {
		t.Skip("TP_TRAFFIC_CORE_TEST_DSN is not set")
	}
	testDB, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer testDB.Close()
	db = testDB

	previousConfig := *core.Config
	defer func() { *core.Config = previousConfig }()
	core.Config.NodeConfig.ServerID = 2
	core.Config.MySQLConfig.AccountTable = "account"

	statements := []string{
		`DROP TABLE IF EXISTS account_traffic_daily,account_server_traffic_daily,account_traffic_total,account,node_server`,
		`CREATE TABLE node_server (id bigint unsigned PRIMARY KEY) ENGINE=InnoDB`,
		`CREATE TABLE account (id bigint unsigned PRIMARY KEY, pass varchar(64) NOT NULL, hash varchar(64) NOT NULL, download bigint unsigned NOT NULL, upload bigint unsigned NOT NULL) ENGINE=InnoDB`,
		`CREATE TABLE account_traffic_total (account_id bigint unsigned PRIMARY KEY, upload bigint unsigned NOT NULL, download bigint unsigned NOT NULL) ENGINE=InnoDB`,
		`CREATE TABLE account_traffic_daily (traffic_date date NOT NULL, account_id bigint unsigned NOT NULL, upload bigint unsigned NOT NULL, download bigint unsigned NOT NULL, PRIMARY KEY (traffic_date,account_id)) ENGINE=InnoDB`,
		`CREATE TABLE account_server_traffic_daily (traffic_date date NOT NULL, account_id bigint unsigned NOT NULL, node_server_id bigint unsigned NOT NULL, upload bigint unsigned NOT NULL, download bigint unsigned NOT NULL, PRIMARY KEY (traffic_date,account_id,node_server_id)) ENGINE=InnoDB`,
		`INSERT INTO node_server VALUES (2)`,
		`INSERT INTO account VALUES (7,'credential','hash',10,20)`,
	}
	for _, statement := range statements {
		if _, err = db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	pass := "credential"
	if err = UpdateAccountFlowByPassOrHash(&pass, nil, 7, 5); err != nil {
		t.Fatal(err)
	}

	assertTraffic := func(query string, args []any, wantUpload, wantDownload uint64) {
		t.Helper()
		var upload, download uint64
		if scanErr := db.QueryRow(query, args...).Scan(&upload, &download); scanErr != nil {
			t.Fatal(scanErr)
		}
		if upload != wantUpload || download != wantDownload {
			t.Fatalf("traffic=(%d,%d) want=(%d,%d) for %s", upload, download, wantUpload, wantDownload, query)
		}
	}
	assertTraffic(`SELECT upload,download FROM account WHERE id=?`, []any{7}, 25, 17)
	assertTraffic(`SELECT upload,download FROM account_traffic_total WHERE account_id=?`, []any{7}, 25, 17)
	assertTraffic(`SELECT upload,download FROM account_traffic_daily WHERE account_id=? AND traffic_date=CURRENT_DATE()`, []any{7}, 5, 7)
	assertTraffic(`SELECT upload,download FROM account_server_traffic_daily WHERE account_id=? AND node_server_id=? AND traffic_date=CURRENT_DATE()`, []any{7, 2}, 5, 7)
}
