package dao

import (
	"database/sql"
	"os"
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
		`DROP TABLE IF EXISTS account_traffic_daily,account_server_traffic_daily,account_traffic_total,casbin_rule,account,node_server`,
		`CREATE TABLE account (id bigint unsigned primary key,username varchar(64) not null,role_id bigint unsigned not null,deleted tinyint unsigned not null,quota bigint not null,download bigint unsigned not null,upload bigint unsigned not null)`,
		`CREATE TABLE node_server (id bigint unsigned primary key,name varchar(64) not null,ip varchar(64) not null,grpc_port int unsigned not null,grpc_tls_mode varchar(16) not null,grpc_tls_server_name varchar(253) not null)`,
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
}
