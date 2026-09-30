package dao

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	"trojan-panel/model"
)

// TP_REMOVAL_TEST_DSN must point to a disposable MySQL/MariaDB instance whose
// user can create and drop databases. Each subtest owns a fresh database and
// leaves the database named in the DSN untouched. Do not run in parallel:
// these DAO functions use the package-level db connection.
func TestNodeRemovalIntegration(t *testing.T) {
	dsn := os.Getenv("TP_REMOVAL_TEST_DSN")
	if dsn == "" {
		t.Skip("TP_REMOVAL_TEST_DSN is not set")
	}

	t.Run("migration_is_idempotent", func(t *testing.T) {
		testDB := newNodeRemovalTestDB(t, dsn)
		removalExec(t, testDB, `CREATE TABLE node_server (id bigint unsigned PRIMARY KEY) ENGINE=InnoDB`)
		removalExec(t, testDB, `INSERT INTO node_server VALUES (11)`)
		if err := migrateNodeRemovalSchema(); err != nil {
			t.Fatal(err)
		}
		assertRemovalRows(t, testDB, `SELECT removing FROM node_server WHERE id=11`, [][]string{{"0"}})
		removalExec(t, testDB, `UPDATE node_server SET removing=1 WHERE id=11`)
		item := nodeRemovalTestCleanup(false)
		removalExec(t, testDB, `INSERT INTO node_removal_cleanup (node_server_id,ip,port,server_name,purge_data,receipt) VALUES (?,?,?,?,?,?)`,
			item.NodeID, item.IP, item.Port, item.ServerName, item.Purge, item.Receipt)
		before := nodeRemovalRows(t, testDB, `SELECT * FROM node_removal_cleanup`)
		if err := migrateNodeRemovalSchema(); err != nil {
			t.Fatalf("migration is not idempotent: %v", err)
		}
		assertRemovalRows(t, testDB, `SELECT removing FROM node_server WHERE id=11`, [][]string{{"1"}})
		assertRemovalRows(t, testDB, `SELECT * FROM node_removal_cleanup`, before)
	})

	t.Run("begin_blocks_active_kernel_items_and_create", func(t *testing.T) {
		testDB := newNodeRemovalTestDB(t, dsn)
		seedNodeRemovalTestDB(t, testDB)
		// A completed stage with an empty result is still an active operation.
		removalExec(t, testDB, `INSERT INTO kernel_upgrade_task_item (id,task_id,node_server_id,node_server_name,stage,result,idempotency_key) VALUES (99,501,11,'remove-me','done','','fixture-99')`)
		if err := BeginNodeServerRemoval(11); err == nil || !strings.Contains(err.Error(), "kernel operations") {
			t.Fatalf("Begin should reject an empty-result kernel item: %v", err)
		}
		assertRemovalRows(t, testDB, `SELECT removing FROM node_server WHERE id=11`, [][]string{{"0"}})
		assertRemovalRows(t, testDB, `SELECT COUNT(*) FROM node_removal_cleanup`, [][]string{{"0"}})
		removalExec(t, testDB, `UPDATE kernel_upgrade_task_item SET result='success' WHERE id=99`)
		// An operation on another server must not prevent this removal.
		removalExec(t, testDB, `UPDATE kernel_upgrade_task_item SET result='' WHERE id=5`)
		if err := BeginNodeServerRemoval(11); err != nil {
			t.Fatal(err)
		}
		if err := BeginNodeServerRemoval(11); err != nil {
			t.Fatalf("repeating Begin should allow a removal retry: %v", err)
		}
		assertRemovalRows(t, testDB, `SELECT id,removing FROM node_server ORDER BY id`, [][]string{{"11", "1"}, {"12", "0"}})
		before := nodeRemovalRows(t, testDB, `SELECT * FROM node ORDER BY id`)
		if err := CreateNode(nodeRemovalTestNode(11)); err == nil || !strings.Contains(err.Error(), "being removed") {
			t.Fatalf("CreateNode should reject a removing server: %v", err)
		}
		assertRemovalRows(t, testDB, `SELECT * FROM node ORDER BY id`, before)
		if err := CreateNode(nodeRemovalTestNode(12)); err != nil {
			t.Fatalf("CreateNode should accept an available server: %v", err)
		}
		assertRemovalRows(t, testDB, `SELECT COUNT(*) FROM node WHERE node_server_id=12 AND name='new-node'`, [][]string{{"1"}})
		if err := BeginNodeServerRemoval(999); !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("Begin for an unknown server: %v", err)
		}
	})

	t.Run("kernel_create_and_retry_reject_removing_servers", func(t *testing.T) {
		testDB := newNodeRemovalTestDB(t, dsn)
		seedNodeRemovalTestDB(t, testDB)
		if err := BeginNodeServerRemoval(11); err != nil {
			t.Fatal(err)
		}
		removalExec(t, testDB, `UPDATE kernel_upgrade_task_item SET stage='failed',result='failed',error_message='fixture failure',rollback_result='fixture rollback',core_operation_id='fixture operation' WHERE id=3`)
		removalExec(t, testDB, `UPDATE kernel_upgrade_task_item SET stage='rolled_back',result='failed',error_message='fixture failure',rollback_result='fixture rollback',core_operation_id='fixture operation' WHERE id=4`)
		taskQuery := `SELECT * FROM kernel_upgrade_task ORDER BY id`
		itemQuery := `SELECT * FROM kernel_upgrade_task_item ORDER BY id`
		beforeTasks := nodeRemovalRows(t, testDB, taskQuery)
		beforeItems := nodeRemovalRows(t, testDB, itemQuery)
		task := model.KernelUpgradeTask{OperatorId: 7, OperatorName: "fixture operator", CanaryNodeId: 12}
		items := []model.KernelUpgradeTaskItem{
			{NodeServerId: 12, NodeServerName: "keep-me", Kernel: "xray", TargetVersion: "1.0.0", Channel: "stable", Action: "install", IdempotencyKey: "guard-kept", Attempt: 1},
			{NodeServerId: 11, NodeServerName: "remove-me", Kernel: "xray", TargetVersion: "1.0.0", Channel: "stable", Action: "install", IdempotencyKey: "guard-removed", Attempt: 1},
		}
		if err := CreateKernelUpgradeTask(&task, items); err == nil || !strings.Contains(err.Error(), "being removed") {
			t.Fatalf("CreateKernelUpgradeTask should reject a removing server: %v", err)
		}
		assertRemovalRows(t, testDB, taskQuery, beforeTasks)
		assertRemovalRows(t, testDB, itemQuery, beforeItems)
		if _, err := ResetKernelTaskItems(502, nil); err == nil || !strings.Contains(err.Error(), "being removed") {
			t.Fatalf("ResetKernelTaskItems should reject a task containing a removing server: %v", err)
		}
		assertRemovalRows(t, testDB, taskQuery, beforeTasks)
		assertRemovalRows(t, testDB, itemQuery, beforeItems)
		if err := CreateKernelUpgradeTask(&task, items[:1]); err != nil {
			t.Fatalf("CreateKernelUpgradeTask should accept an available server: %v", err)
		}
		if task.Id == 0 {
			t.Fatal("successful kernel task creation did not assign an ID")
		}
		assertRemovalRows(t, testDB, `SELECT operator_id,operator_name,canary_node_id,status FROM kernel_upgrade_task WHERE id=?`,
			[][]string{{"7", "fixture operator", "12", "queued"}}, task.Id)
		assertRemovalRows(t, testDB, `SELECT node_server_id,stage,result,attempt,idempotency_key FROM kernel_upgrade_task_item WHERE task_id=?`,
			[][]string{{"12", "queued", "", "1", "guard-kept"}}, task.Id)
		reset, err := ResetKernelTaskItems(502, []uint64{4})
		if err != nil || len(reset) != 1 || reset[0].Id != 4 || reset[0].NodeServerId != 12 || reset[0].Stage != "queued" || reset[0].Result != "" || reset[0].Attempt != 2 {
			t.Fatalf("retrying only the available server should succeed: %#v err=%v", reset, err)
		}
		assertRemovalRows(t, testDB, `SELECT id,stage,result,error_message,rollback_result,core_operation_id,attempt,idempotency_key FROM kernel_upgrade_task_item WHERE task_id=502 ORDER BY id`,
			[][]string{
				{"3", "failed", "failed", "fixture failure", "fixture rollback", "fixture operation", "1", "fixture-3"},
				{"4", "queued", "", "", "", "", "2", "task-502-item-4-attempt-2"},
			})
		assertRemovalRows(t, testDB, `SELECT status FROM kernel_upgrade_task WHERE id=502`, [][]string{{"queued"}})
	})

	t.Run("node_update_rejects_old_and_target_removing_servers", func(t *testing.T) {
		testDB := newNodeRemovalTestDB(t, dsn)
		seedNodeRemovalTestDB(t, testDB)
		if err := BeginNodeServerRemoval(11); err != nil {
			t.Fatal(err)
		}
		before := nodeRemovalRows(t, testDB, `SELECT * FROM node ORDER BY id`)
		removedNodeID, keptNodeID := uint(11), uint(21)
		removedServerID, keptServerID := uint(11), uint(12)
		name := "updated-fixture"
		for _, update := range []*model.Node{
			{Id: &removedNodeID, Name: &name},
			{Id: &removedNodeID, NodeServerId: &keptServerID},
			{Id: &keptNodeID, NodeServerId: &removedServerID},
		} {
			if err := UpdateNodeById(update); err == nil || !strings.Contains(err.Error(), "being removed") {
				t.Fatalf("UpdateNodeById should reject an old or target removing server: %v", err)
			}
			assertRemovalRows(t, testDB, `SELECT * FROM node ORDER BY id`, before)
		}
		if err := UpdateNodeById(&model.Node{Id: &keptNodeID, Name: &name}); err != nil {
			t.Fatalf("updating an available server's node should succeed: %v", err)
		}
		assertRemovalRows(t, testDB, `SELECT node_server_id,name FROM node WHERE id=21`, [][]string{{"12", "updated-fixture"}})
		removalExec(t, testDB, `INSERT INTO node_server (id,name,ip,grpc_port,grpc_tls_server_name) VALUES (13,'available-target','203.0.113.13',18102,'available-target.example.test')`)
		targetID := uint(13)
		if err := UpdateNodeById(&model.Node{Id: &keptNodeID, NodeServerId: &targetID}); err != nil {
			t.Fatalf("moving a node between available servers should succeed: %v", err)
		}
		assertRemovalRows(t, testDB, `SELECT node_server_id,name FROM node WHERE id=21`, [][]string{{"13", "updated-fixture"}})
	})

	for _, purge := range []bool{false, true} {
		name := "ordinary_preserves_history"
		if purge {
			name = "purge_preserves_other_servers_and_shared_tasks"
		}
		t.Run(name, func(t *testing.T) {
			testDB := newNodeRemovalTestDB(t, dsn)
			seedNodeRemovalTestDB(t, testDB)
			item := nodeRemovalTestCleanup(purge)
			if err := CompleteNodeServerRemoval(item); !errors.Is(err, sql.ErrNoRows) {
				t.Fatalf("Complete should require Begin: %v", err)
			}
			assertRemovalRows(t, testDB, `SELECT COUNT(*) FROM node WHERE node_server_id=11`, [][]string{{"5"}})
			if err := BeginNodeServerRemoval(item.NodeID); err != nil {
				t.Fatal(err)
			}

			retained := []struct {
				beforeQuery string
				afterQuery  string
			}{
				{`SELECT * FROM node_server WHERE id<>11 ORDER BY id`, `SELECT * FROM node_server ORDER BY id`},
				{`SELECT * FROM node WHERE node_server_id<>11 ORDER BY id`, `SELECT * FROM node ORDER BY id`},
				{`SELECT * FROM node_xray WHERE id<>101 ORDER BY id`, `SELECT * FROM node_xray ORDER BY id`},
				{`SELECT * FROM node_trojan_go WHERE id<>102 ORDER BY id`, `SELECT * FROM node_trojan_go ORDER BY id`},
				{`SELECT * FROM node_hysteria WHERE id<>103 ORDER BY id`, `SELECT * FROM node_hysteria ORDER BY id`},
				{`SELECT * FROM node_hysteria2 WHERE id<>105 ORDER BY id`, `SELECT * FROM node_hysteria2 ORDER BY id`},
				{`SELECT * FROM account_traffic_total ORDER BY account_id`, `SELECT * FROM account_traffic_total ORDER BY account_id`},
				{`SELECT * FROM account_traffic_daily ORDER BY traffic_date,account_id`, `SELECT * FROM account_traffic_daily ORDER BY traffic_date,account_id`},
			}
			trafficQuery := `SELECT * FROM account_server_traffic_daily ORDER BY traffic_date,account_id,node_server_id`
			itemQuery := `SELECT * FROM kernel_upgrade_task_item ORDER BY id`
			taskQuery := `SELECT id,canary_node_id,status FROM kernel_upgrade_task ORDER BY id`
			if purge {
				retained = append(retained,
					struct{ beforeQuery, afterQuery string }{`SELECT * FROM account_server_traffic_daily WHERE node_server_id<>11 ORDER BY traffic_date,account_id,node_server_id`, trafficQuery},
					struct{ beforeQuery, afterQuery string }{`SELECT * FROM kernel_upgrade_task_item WHERE node_server_id<>11 ORDER BY id`, itemQuery},
					struct{ beforeQuery, afterQuery string }{`SELECT id,CASE WHEN canary_node_id=11 THEN 0 ELSE canary_node_id END,status FROM kernel_upgrade_task WHERE id<>501 ORDER BY id`, taskQuery},
				)
			} else {
				retained = append(retained,
					struct{ beforeQuery, afterQuery string }{trafficQuery, trafficQuery},
					struct{ beforeQuery, afterQuery string }{itemQuery, itemQuery},
					struct{ beforeQuery, afterQuery string }{taskQuery, taskQuery},
				)
			}
			expected := make([][][]string, len(retained))
			for i, check := range retained {
				expected[i] = nodeRemovalRows(t, testDB, check.beforeQuery)
			}
			if err := CompleteNodeServerRemoval(item); err != nil {
				t.Fatal(err)
			}
			for i, check := range retained {
				assertRemovalRows(t, testDB, check.afterQuery, expected[i])
			}
			if purge {
				assertRemovalRows(t, testDB, `SELECT COUNT(*) FROM kernel_upgrade_task WHERE canary_node_id=11`, [][]string{{"0"}})
				assertRemovalRows(t, testDB, `SELECT id FROM kernel_upgrade_task ORDER BY id`, [][]string{{"502"}, {"503"}, {"504"}, {"505"}})
			}
			pending, err := PendingRemovalCleanups()
			if err != nil || !reflect.DeepEqual(pending, []RemovalCleanup{item}) {
				t.Fatalf("outbox lost the removal endpoint or receipt: %#v err=%v", pending, err)
			}
			if err := DeleteRemovalCleanup(item.NodeID, strings.Repeat("b", 64)); !errors.Is(err, ErrInvalidRemovalReceipt) {
				t.Fatalf("a different receipt should be rejected: %v", err)
			}
			pending, err = PendingRemovalCleanups()
			if err != nil || !reflect.DeepEqual(pending, []RemovalCleanup{item}) {
				t.Fatalf("a different receipt removed the outbox entry: %#v err=%v", pending, err)
			}
			if err := DeleteRemovalCleanup(item.NodeID, item.Receipt); err != nil {
				t.Fatal(err)
			}
			pending, err = PendingRemovalCleanups()
			if err != nil || len(pending) != 0 {
				t.Fatalf("completed outbox entry remains: %#v err=%v", pending, err)
			}
			// Connection data survives only in the outbox until its acknowledgement.
			assertRemovalRows(t, testDB, `SELECT COUNT(*) FROM node_server WHERE id=11 OR ip=?`, [][]string{{"0"}}, item.IP)
			assertRemovalRows(t, testDB, `SELECT COUNT(*) FROM node WHERE node_server_id=11 OR node_server_ip=?`, [][]string{{"0"}}, item.IP)
			assertRemovalRows(t, testDB, `SELECT COUNT(*) FROM node_removal_cleanup WHERE node_server_id=11 OR ip=? OR server_name=? OR receipt=?`, [][]string{{"0"}}, item.IP, item.ServerName, item.Receipt)
		})
	}

	t.Run("cleanup_acknowledgement_is_scoped_and_idempotent", func(t *testing.T) {
		testDB := newNodeRemovalTestDB(t, dsn)
		removalExec(t, testDB, `CREATE TABLE node_server (id bigint unsigned PRIMARY KEY) ENGINE=InnoDB`)
		if err := migrateNodeRemovalSchema(); err != nil {
			t.Fatal(err)
		}
		item := nodeRemovalTestCleanup(false)
		other := RemovalCleanup{NodeID: 12, IP: "203.0.113.12", Port: 18101, ServerName: "keep-me.example.test", Purge: true, Receipt: item.Receipt}
		for _, cleanup := range []RemovalCleanup{item, other} {
			removalExec(t, testDB, `INSERT INTO node_removal_cleanup (node_server_id,ip,port,server_name,purge_data,receipt) VALUES (?,?,?,?,?,?)`,
				cleanup.NodeID, cleanup.IP, cleanup.Port, cleanup.ServerName, cleanup.Purge, cleanup.Receipt)
		}
		before := nodeRemovalRows(t, testDB, `SELECT * FROM node_removal_cleanup ORDER BY node_server_id`)
		for _, receipt := range []string{strings.Repeat("b", 64), strings.ToUpper(item.Receipt)} {
			if err := DeleteRemovalCleanup(item.NodeID, receipt); !errors.Is(err, ErrInvalidRemovalReceipt) {
				t.Fatalf("a different or differently cased receipt should be rejected: %v", err)
			}
			assertRemovalRows(t, testDB, `SELECT * FROM node_removal_cleanup ORDER BY node_server_id`, before)
		}
		if err := DeleteRemovalCleanup(item.NodeID, item.Receipt); err != nil {
			t.Fatal(err)
		}
		if err := DeleteRemovalCleanup(item.NodeID, item.Receipt); err != nil {
			t.Fatalf("repeating an acknowledged cleanup should succeed: %v", err)
		}
		if err := DeleteRemovalCleanup(999, strings.Repeat("b", 64)); err != nil {
			t.Fatalf("an unknown cleanup should be an idempotent success: %v", err)
		}
		pending, err := PendingRemovalCleanups()
		if err != nil || !reflect.DeepEqual(pending, []RemovalCleanup{other}) {
			t.Fatalf("acknowledgement removed another server's cleanup with the same receipt: %#v err=%v", pending, err)
		}
	})

	t.Run("outbox_failure_rolls_back_the_entire_removal", func(t *testing.T) {
		testDB := newNodeRemovalTestDB(t, dsn)
		seedNodeRemovalTestDB(t, testDB)
		item := nodeRemovalTestCleanup(true)
		if err := BeginNodeServerRemoval(item.NodeID); err != nil {
			t.Fatal(err)
		}
		removalExec(t, testDB, `INSERT INTO node_removal_cleanup (node_server_id,ip,port,server_name,purge_data,receipt) VALUES (?,?,?,?,?,?)`,
			item.NodeID, item.IP, item.Port, item.ServerName, item.Purge, item.Receipt)
		queries := []string{
			`SELECT * FROM node_server ORDER BY id`,
			`SELECT * FROM node ORDER BY id`,
			`SELECT * FROM node_xray ORDER BY id`,
			`SELECT * FROM node_trojan_go ORDER BY id`,
			`SELECT * FROM node_hysteria ORDER BY id`,
			`SELECT * FROM node_hysteria2 ORDER BY id`,
			`SELECT * FROM account_server_traffic_daily ORDER BY traffic_date,account_id,node_server_id`,
			`SELECT * FROM kernel_upgrade_task ORDER BY id`,
			`SELECT * FROM kernel_upgrade_task_item ORDER BY id`,
			`SELECT * FROM node_removal_cleanup ORDER BY node_server_id`,
		}
		before := make([][][]string, len(queries))
		for i, query := range queries {
			before[i] = nodeRemovalRows(t, testDB, query)
		}
		err := CompleteNodeServerRemoval(item)
		var mysqlErr *mysql.MySQLError
		if !errors.As(err, &mysqlErr) || mysqlErr.Number != 1062 {
			t.Fatalf("expected duplicate outbox key to fail the transaction: %v", err)
		}
		for i, query := range queries {
			assertRemovalRows(t, testDB, query, before[i])
		}
	})
}

func newNodeRemovalTestDB(t *testing.T, dsn string) *sql.DB {
	t.Helper()
	config, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("parse TP_REMOVAL_TEST_DSN: %v", err)
	}
	config.DBName = ""
	admin, err := sql.Open("mysql", config.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { admin.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := admin.PingContext(ctx); err != nil {
		t.Fatalf("connect to removal test database: %v", err)
	}
	databaseName := fmt.Sprintf("tpn_removal_test_%d", time.Now().UnixNano())
	removalExec(t, admin, "CREATE DATABASE `"+databaseName+"` CHARACTER SET utf8mb4")
	t.Cleanup(func() {
		if _, err := admin.Exec("DROP DATABASE `" + databaseName + "`"); err != nil {
			t.Errorf("drop removal test database: %v", err)
		}
	})
	config.DBName = databaseName
	testDB, err := sql.Open("mysql", config.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	previousDB := db
	db = testDB
	t.Cleanup(func() {
		db = previousDB
		testDB.Close()
	})
	return testDB
}

func seedNodeRemovalTestDB(t *testing.T, testDB *sql.DB) {
	t.Helper()
	statements := []string{
		`CREATE TABLE node_server (
			id bigint unsigned PRIMARY KEY, name varchar(64) NOT NULL, ip varchar(253) NOT NULL,
			grpc_port int unsigned NOT NULL, grpc_tls_server_name varchar(253) NOT NULL
		) ENGINE=InnoDB`,
		`CREATE TABLE node (
			id bigint unsigned PRIMARY KEY AUTO_INCREMENT, node_server_id bigint unsigned NOT NULL,
			node_sub_id bigint unsigned NOT NULL, node_type_id bigint unsigned NOT NULL,
			name varchar(64) NOT NULL, node_server_ip varchar(253) NOT NULL, domain varchar(253) NOT NULL,
			port int unsigned NOT NULL DEFAULT 443, node_server_grpc_port int unsigned NOT NULL DEFAULT 8100,
			priority int NOT NULL DEFAULT 0, client_types varchar(64) NOT NULL DEFAULT '',
			naive_uot_enable tinyint unsigned NOT NULL DEFAULT 0, naive_uot_version tinyint unsigned NOT NULL DEFAULT 2
		) ENGINE=InnoDB`,
		`CREATE TABLE account_server_traffic_daily (
			traffic_date date NOT NULL, account_id bigint unsigned NOT NULL, node_server_id bigint unsigned NOT NULL,
			upload bigint unsigned NOT NULL, download bigint unsigned NOT NULL,
			PRIMARY KEY (traffic_date,account_id,node_server_id)
		) ENGINE=InnoDB`,
		`CREATE TABLE account_traffic_total (account_id bigint unsigned PRIMARY KEY, upload bigint unsigned NOT NULL, download bigint unsigned NOT NULL) ENGINE=InnoDB`,
		`CREATE TABLE account_traffic_daily (traffic_date date NOT NULL, account_id bigint unsigned NOT NULL, upload bigint unsigned NOT NULL, download bigint unsigned NOT NULL, PRIMARY KEY (traffic_date,account_id)) ENGINE=InnoDB`,
		`CREATE TABLE kernel_upgrade_task (
			id bigint unsigned PRIMARY KEY AUTO_INCREMENT, operator_id bigint unsigned NOT NULL DEFAULT 0,
			operator_name varchar(64) NOT NULL DEFAULT '', canary_node_id bigint unsigned NOT NULL DEFAULT 0,
			status varchar(16) NOT NULL DEFAULT 'queued', create_time datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
			update_time datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
		) ENGINE=InnoDB`,
		`CREATE TABLE kernel_upgrade_task_item (
			id bigint unsigned PRIMARY KEY AUTO_INCREMENT, task_id bigint unsigned NOT NULL, node_server_id bigint unsigned NOT NULL,
			node_server_name varchar(64) NOT NULL, kernel_name varchar(32) NOT NULL DEFAULT 'xray',
			from_version varchar(64) NOT NULL DEFAULT '', target_version varchar(64) NOT NULL DEFAULT '1.0.0',
			channel_name varchar(16) NOT NULL DEFAULT 'stable', action_name varchar(16) NOT NULL DEFAULT 'install',
			sha256 char(64) NOT NULL DEFAULT '', stage varchar(32) NOT NULL DEFAULT 'queued', result varchar(16) NOT NULL DEFAULT '',
			error_message varchar(2048) NOT NULL DEFAULT '', rollback_result varchar(32) NOT NULL DEFAULT '',
			core_operation_id varchar(64) NOT NULL DEFAULT '', idempotency_key varchar(128) NOT NULL,
			attempt int unsigned NOT NULL DEFAULT 1, create_time datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
			update_time datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			UNIQUE KEY uk_kernel_item_idempotency (idempotency_key)
		) ENGINE=InnoDB`,
		`INSERT INTO node_server VALUES (11,'remove-me','203.0.113.11',18100,'remove-me.example.test'),(12,'keep-me','203.0.113.12',18101,'keep-me.example.test')`,
		`INSERT INTO node (id,node_server_id,node_sub_id,node_type_id,name,node_server_ip,domain) VALUES
			(11,11,101,1,'remove-xray','203.0.113.11','remove-me.example.test'),
			(12,11,102,2,'remove-trojan','203.0.113.11','remove-me.example.test'),
			(13,11,103,3,'remove-hysteria','203.0.113.11','remove-me.example.test'),
			(14,11,104,4,'remove-naive','203.0.113.11','remove-me.example.test'),
			(15,11,105,5,'remove-hysteria2','203.0.113.11','remove-me.example.test'),
			(21,12,102,1,'keep-xray','203.0.113.12','keep-me.example.test'),
			(22,12,103,2,'keep-trojan','203.0.113.12','keep-me.example.test'),
			(23,12,105,3,'keep-hysteria','203.0.113.12','keep-me.example.test'),
			(24,12,204,4,'keep-naive','203.0.113.12','keep-me.example.test'),
			(25,12,101,5,'keep-hysteria2','203.0.113.12','keep-me.example.test')`,
		`INSERT INTO account_server_traffic_daily VALUES ('2026-09-01',7,11,10,20),('2026-09-01',8,11,30,40),('2026-09-01',7,12,50,60)`,
		`INSERT INTO account_traffic_total VALUES (7,600,800),(8,300,400)`,
		`INSERT INTO account_traffic_daily VALUES ('2026-09-01',7,60,80),('2026-09-01',8,30,40)`,
		`INSERT INTO kernel_upgrade_task (id,canary_node_id,status) VALUES (501,11,'done'),(502,11,'done'),(503,12,'done'),(504,12,'done'),(505,11,'done')`,
		`INSERT INTO kernel_upgrade_task_item (id,task_id,node_server_id,node_server_name,stage,result,idempotency_key) VALUES
			(1,501,11,'remove-me','done','success','fixture-1'),(2,501,11,'remove-me','done','failed','fixture-2'),
			(3,502,11,'remove-me','done','success','fixture-3'),(4,502,12,'keep-me','done','success','fixture-4'),
			(5,503,12,'keep-me','done','success','fixture-5'),(6,505,12,'keep-me','done','success','fixture-6')`,
	}
	for _, statement := range statements {
		removalExec(t, testDB, statement)
	}
	for _, detail := range []struct {
		table             string
		removedID, keptID uint
	}{{"node_xray", 101, 102}, {"node_trojan_go", 102, 103}, {"node_hysteria", 103, 105}, {"node_hysteria2", 105, 101}} {
		removalExec(t, testDB, "CREATE TABLE "+detail.table+" (id bigint unsigned PRIMARY KEY, config text NOT NULL) ENGINE=InnoDB")
		// Overlapping IDs across detail tables catch missing node_type_id filters.
		removalExec(t, testDB, "INSERT INTO "+detail.table+" VALUES (?, 'removed-fixture'), (?, 'kept-fixture'), (999, 'unrelated-fixture')", detail.removedID, detail.keptID)
	}
	if err := migrateNodeRemovalSchema(); err != nil {
		t.Fatal(err)
	}
}

func nodeRemovalTestCleanup(purge bool) RemovalCleanup {
	return RemovalCleanup{
		NodeID: 11, IP: "203.0.113.11", Port: 18100,
		ServerName: "remove-me.example.test", Purge: purge, Receipt: strings.Repeat("a", 64),
	}
}

func nodeRemovalTestNode(serverID uint) *model.Node {
	subID, typeID := uint(1000), uint(4)
	name, ip, domain := "new-node", "203.0.113.12", "new-node.example.test"
	return &model.Node{NodeServerId: &serverID, NodeSubId: &subID, NodeTypeId: &typeID, Name: &name, NodeServerIp: &ip, Domain: &domain}
}

func removalExec(t *testing.T, testDB *sql.DB, query string, args ...interface{}) {
	t.Helper()
	if _, err := testDB.Exec(query, args...); err != nil {
		t.Fatalf("execute fixture SQL: %v", err)
	}
}

func nodeRemovalRows(t *testing.T, testDB *sql.DB, query string, args ...interface{}) [][]string {
	t.Helper()
	rows, err := testDB.Query(query, args...)
	if err != nil {
		t.Fatalf("query %s: %v", query, err)
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		t.Fatal(err)
	}
	result := make([][]string, 0)
	for rows.Next() {
		values := make([]interface{}, len(columns))
		pointers := make([]interface{}, len(columns))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			t.Fatal(err)
		}
		row := make([]string, len(values))
		for i, value := range values {
			if bytes, ok := value.([]byte); ok {
				row[i] = string(bytes)
			} else {
				row[i] = fmt.Sprint(value)
			}
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return result
}

func assertRemovalRows(t *testing.T, testDB *sql.DB, query string, expected [][]string, args ...interface{}) {
	t.Helper()
	actual := nodeRemovalRows(t, testDB, query, args...)
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("query %s: got %v, want %v", query, actual, expected)
	}
}
