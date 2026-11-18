package dao

import (
	"database/sql"
	"errors"
	"os"
	"reflect"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"trojan-panel/model/vo"
)

func TestTrafficMigrationAndQueries(t *testing.T) {
	dsn := os.Getenv("TP_TRAFFIC_TEST_DSN")
	if dsn == "" {
		t.Skip("TP_TRAFFIC_TEST_DSN is not set")
	}
	testDB, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer testDB.Close()
	db = testDB
	statements := []string{
		`DROP TABLE IF EXISTS server_traffic_display_selection,server_traffic_display,node_removal_cleanup,account_traffic_daily,account_server_traffic_daily,account_traffic_total,casbin_rule,account,node_server`,
		`CREATE TABLE account (id bigint unsigned primary key,username varchar(64) not null,role_id bigint unsigned not null,deleted tinyint unsigned not null,quota bigint not null,download bigint unsigned not null,upload bigint unsigned not null)`,
		`CREATE TABLE node_server (id bigint primary key,name varchar(64) not null,ip varchar(64) not null,grpc_port int unsigned not null,grpc_tls_mode varchar(16) not null,grpc_tls_server_name varchar(253) not null)`,
		`CREATE TABLE casbin_rule (p_type varchar(32),v0 varchar(255),v1 varchar(255),v2 varchar(255),v3 varchar(255),v4 varchar(255),v5 varchar(255))`,
		`INSERT INTO account VALUES (7,'alice',3,0,-1,20,10),(8,'bob',1,0,-1,0,0)`,
		`INSERT INTO node_server VALUES (2,'sf','127.0.0.1',8100,'mtls','sf.example.com'),(3,'hk','127.0.0.2',8100,'mtls','hk.example.com')`,
	}
	for _, statement := range statements {
		if _, err = db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	if err = migrateTrafficAccountingSchema(); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`UPDATE node_server SET traffic_period='month',traffic_limit_mode='combined',traffic_total_limit=100 WHERE id=2`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`UPDATE node_server SET traffic_period='month',traffic_limit_mode='combined' WHERE id=3`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO account_server_traffic_daily VALUES
		(CURRENT_DATE(),7,2,30,40,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP),
		(CURRENT_DATE(),8,2,5,7,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP),
		(CURRENT_DATE(),7,3,10,20,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`); err != nil {
		t.Fatal(err)
	}
	if err = migrateTrafficAccountingSchema(); err != nil {
		t.Fatalf("migration is not idempotent: %v", err)
	}
	rank, err := TrafficRank("total", "", "")
	if err != nil || len(rank) != 1 || rank[0].TrafficUsed != 30 {
		t.Fatalf("unexpected total rank: %#v %v", rank, err)
	}
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	today := time.Now().In(location)
	daily, err := TrafficRank("day", today.Format("2006-01-02"), today.AddDate(0, 0, 1).Format("2006-01-02"))
	if err != nil || len(daily) != 1 || daily[0].TrafficUsed != 100 {
		t.Fatalf("unexpected daily rank: %#v %v", daily, err)
	}
	startDate := today.Format("2006-01-02")
	endDate := today.AddDate(0, 0, 1).Format("2006-01-02")
	summaries, total, err := SelectServerTrafficUsage(startDate, endDate, nil, 1, 20)
	if err != nil || total != 2 || len(summaries) != 2 || summaries[0].NodeServerId != 2 || summaries[0].Upload != 35 || summaries[0].Download != 47 {
		t.Fatalf("unexpected server summaries: %#v total=%d err=%v", summaries, total, err)
	}
	users, total, err := SelectServerTrafficUserUsage(startDate, endDate, 2, 1, 20)
	if err != nil || total != 2 || len(users) != 2 || users[0].Username != "alice" || users[0].Total != 70 || users[1].Username != "bob" || users[1].Total != 12 {
		t.Fatalf("unexpected server user details: %#v total=%d err=%v", users, total, err)
	}
	statuses, err := SelectServerTrafficStatuses([]uint{2})
	if err != nil || len(statuses) != 1 || statuses[0].UploadUsed != 35 || statuses[0].DownloadUsed != 47 {
		t.Fatalf("unexpected status: %#v %v", statuses, err)
	}
	deleted, err := ResetNodeServerTraffic(2)
	if err != nil || deleted != 2 {
		t.Fatalf("unexpected reset result: deleted=%d err=%v", deleted, err)
	}
	statuses, err = SelectServerTrafficStatuses([]uint{2, 3})
	statusByServer := make(map[uint]vo.ServerTrafficStatusVo, len(statuses))
	for _, status := range statuses {
		statusByServer[status.NodeServerId] = status
	}
	if err != nil || len(statuses) != 2 || statusByServer[2].UploadUsed != 0 || statusByServer[2].DownloadUsed != 0 || statusByServer[3].UploadUsed != 10 || statusByServer[3].DownloadUsed != 20 {
		t.Fatalf("unexpected post-reset statuses: %#v %v", statuses, err)
	}
	daily, err = TrafficRank("day", today.Format("2006-01-02"), today.AddDate(0, 0, 1).Format("2006-01-02"))
	if err != nil || len(daily) != 1 || daily[0].TrafficUsed != 100 {
		t.Fatalf("server reset changed account rank: %#v %v", daily, err)
	}
	if _, err = ResetNodeServerTraffic(999); err == nil {
		t.Fatal("resetting an unknown server should fail")
	}
	var resetPermissions int
	if err = db.QueryRow(`SELECT COUNT(*) FROM casbin_rule WHERE v0 IN ('sysadmin','admin') AND v1='/api/nodeServer/resetNodeServerTraffic' AND v2='POST'`).Scan(&resetPermissions); err != nil || resetPermissions != 2 {
		t.Fatalf("unexpected reset permissions: count=%d err=%v", resetPermissions, err)
	}
	var trafficPermissions int
	if err = db.QueryRow(`SELECT COUNT(*) FROM casbin_rule WHERE v0 IN ('sysadmin','admin') AND v1 IN ('/api/dashboard/serverTrafficUsage','/api/dashboard/serverTrafficUserUsage') AND v2='GET'`).Scan(&trafficPermissions); err != nil || trafficPermissions != 4 {
		t.Fatalf("unexpected traffic permissions: count=%d err=%v", trafficPermissions, err)
	}

	t.Run("unlimited_server_counts_all_history", func(t *testing.T) {
		if _, err := db.Exec(`INSERT INTO node_server
			(id,name,ip,grpc_port,grpc_tls_mode,grpc_tls_server_name,traffic_period,traffic_limit_mode,traffic_total_limit)
			VALUES (4,'unlimited','203.0.113.4',8100,'mtls','unlimited.example.test','none','combined',1)`); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO account_server_traffic_daily
			(traffic_date,account_id,node_server_id,upload,download) VALUES
			(CURRENT_DATE(),7,4,13,17),
			(CURRENT_DATE(),8,4,19,23),
			(DATE_SUB(CURRENT_DATE(),INTERVAL 1 YEAR),7,4,101,103),
			(DATE_SUB(CURRENT_DATE(),INTERVAL 3 YEAR),8,4,1009,1013)`); err != nil {
			t.Fatal(err)
		}
		statuses, err := SelectServerTrafficStatuses([]uint{4})
		if err != nil || len(statuses) != 1 || statuses[0].Period != "none" ||
			statuses[0].UploadUsed != 1142 || statuses[0].DownloadUsed != 1156 || statuses[0].Reached {
			t.Fatalf("unlimited server should report all historical traffic without reaching a limit: %#v err=%v", statuses, err)
		}
	})

	t.Run("server_traffic_display_settings_and_selection", func(t *testing.T) {
		if err := migrateNodeRemovalSchema(); err != nil {
			t.Fatal(err)
		}
		if err := migrateServerTrafficDisplay(); err != nil {
			t.Fatal(err)
		}
		initial, err := SelectServerTrafficDisplay()
		if err != nil || initial.Enabled || len(initial.ServerIDs) != 0 || len(initial.Servers) != 3 {
			t.Fatalf("unexpected initial traffic display settings: %#v err=%v", initial, err)
		}
		if err := UpdateServerTrafficDisplay(true, []uint{4, 3, 3}); err != nil {
			t.Fatal(err)
		}
		if err := migrateServerTrafficDisplay(); err != nil {
			t.Fatalf("migration is not idempotent: %v", err)
		}
		var permissionCount int
		if err := db.QueryRow(`SELECT COUNT(*) FROM casbin_rule WHERE v1 IN
			('/api/node/serverTrafficDisplay','/api/node/serverTrafficSummaries')`).Scan(&permissionCount); err != nil || permissionCount != 7 {
			t.Fatalf("unexpected permissions: count=%d err=%v", permissionCount, err)
		}
		if err := db.QueryRow(`SELECT COUNT(*) FROM casbin_rule WHERE v0='user'
			AND v1='/api/node/serverTrafficDisplay'`).Scan(&permissionCount); err != nil || permissionCount != 0 {
			t.Fatalf("user has management permissions: count=%d err=%v", permissionCount, err)
		}

		if err := UpdateServerTrafficDisplay(false, []uint{2, 999}); !errors.Is(err, ErrServerTrafficDisplayInvalidServer) {
			t.Fatalf("expected invalid selection to fail: %v", err)
		}
		active, err := SelectServerTrafficDisplay()
		if err != nil || !active.Enabled || !reflect.DeepEqual(active.ServerIDs, []uint{3, 4}) {
			t.Fatalf("a failed update replaced the previous settings: %#v err=%v", active, err)
		}
		summaries, err := SelectServerTrafficSummaries()
		if err != nil || len(summaries) != 2 || summaries[0].NodeServerId != 3 ||
			summaries[0].UploadUsed != 10 || summaries[0].DownloadUsed != 20 || summaries[1].NodeServerId != 4 ||
			summaries[1].UploadUsed != 1142 || summaries[1].DownloadUsed != 1156 {
			t.Fatalf("unexpected summaries for selected servers: %#v err=%v", summaries, err)
		}

		if err := UpdateServerTrafficDisplay(false, active.ServerIDs); err != nil {
			t.Fatal(err)
		}
		summaries, err = SelectServerTrafficSummaries()
		if err != nil || len(summaries) != 0 {
			t.Fatalf("disabled display returned summaries: %#v err=%v", summaries, err)
		}
		inactive, err := SelectServerTrafficDisplay()
		if err != nil || inactive.Enabled || !reflect.DeepEqual(inactive.ServerIDs, active.ServerIDs) {
			t.Fatalf("disabling did not preserve the selection: %#v err=%v", inactive, err)
		}

		if err := UpdateServerTrafficDisplay(true, inactive.ServerIDs); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`UPDATE node_server SET removing=1 WHERE id=3`); err != nil {
			t.Fatal(err)
		}
		summaries, err = SelectServerTrafficSummaries()
		if err != nil || len(summaries) != 1 || summaries[0].NodeServerId != 4 {
			t.Fatalf("a server being removed was published: %#v err=%v", summaries, err)
		}
		if _, err := db.Exec(`DELETE FROM node_server WHERE id IN (3,4)`); err != nil {
			t.Fatal(err)
		}
		summaries, err = SelectServerTrafficSummaries()
		if err != nil || len(summaries) != 0 {
			t.Fatalf("removal left visible summaries: %#v err=%v", summaries, err)
		}
		var selectionCount int
		if err := db.QueryRow(`SELECT COUNT(*) FROM server_traffic_display_selection`).Scan(&selectionCount); err != nil || selectionCount != 0 {
			t.Fatalf("removal left stale selections: count=%d err=%v", selectionCount, err)
		}
	})
}
