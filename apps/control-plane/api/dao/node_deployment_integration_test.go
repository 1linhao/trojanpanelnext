package dao

import (
	"os"
	"testing"

	"github.com/casbin/casbin/v2"
	casbinmodel "github.com/casbin/casbin/v2/model"
	"trojan-panel/model"
)

func TestNodeDeploymentRegistrationAndPermissions(t *testing.T) {
	dsn := os.Getenv("TP_REMOVAL_TEST_DSN")
	if dsn == "" {
		t.Skip("TP_REMOVAL_TEST_DSN is not set")
	}
	t.Run("creation_returns_its_actual_insert_ID", func(t *testing.T) {
		testDB := newNodeRemovalTestDB(t, dsn)
		removalExec(t, testDB, `CREATE TABLE node_server (
			id bigint unsigned PRIMARY KEY AUTO_INCREMENT,name varchar(64) NOT NULL,ip varchar(253) NOT NULL,
			grpc_port int unsigned NOT NULL DEFAULT 8100,grpc_tls_mode varchar(16) NOT NULL,grpc_tls_server_name varchar(253) NOT NULL,
			traffic_period varchar(8) NOT NULL,traffic_limit_mode varchar(8) NOT NULL,
			traffic_total_limit bigint unsigned NOT NULL,traffic_upload_limit bigint unsigned NOT NULL,traffic_download_limit bigint unsigned NOT NULL
		) ENGINE=InnoDB AUTO_INCREMENT=41`)
		name, ip, tlsName, period, limitMode := "fixture", "192.0.2.41", "node.example.test", "none", "combined"
		zero := uint64(0)
		makeServer := func() *model.NodeServer {
			return &model.NodeServer{Name: &name, Ip: &ip, GrpcTLSServerName: &tlsName, TrafficPeriod: &period, TrafficLimitMode: &limitMode, TrafficTotalLimit: &zero, TrafficUploadLimit: &zero, TrafficDownloadLimit: &zero}
		}
		first, second := makeServer(), makeServer()
		if err := CreateNodeServer(first); err != nil {
			t.Fatal(err)
		}
		if err := CreateNodeServer(second); err != nil {
			t.Fatal(err)
		}
		if first.Id == nil || second.Id == nil || *first.Id != 41 || *second.Id != 42 {
			t.Fatalf("incorrect returned identities: first=%v second=%v", first.Id, second.Id)
		}
		// Duplicate fixture names prove identity is not guessed by querying name.
		assertRemovalRows(t, testDB, `SELECT id,name,ip,grpc_port,grpc_tls_server_name FROM node_server ORDER BY id`, [][]string{{"41", name, ip, "8100", tlsName}, {"42", name, ip, "8100", tlsName}})
		if first.GrpcPort == nil || *first.GrpcPort != 8100 {
			t.Fatal("returned default port differs from persisted value")
		}
	})
	t.Run("deployment_credentials_are_authorized_only_for_sysadmin", func(t *testing.T) {
		testDB := newNodeRemovalTestDB(t, dsn)
		removalExec(t, testDB, `CREATE TABLE casbin_rule (p_type varchar(32),v0 varchar(255),v1 varchar(255),v2 varchar(255),v3 varchar(255),v4 varchar(255),v5 varchar(255))`)
		for i := 0; i < 2; i++ {
			if err := migrateNodeDeploymentPermissions(); err != nil {
				t.Fatal(err)
			}
		}
		assertRemovalRows(t, testDB, `SELECT v0,v1,v2 FROM casbin_rule ORDER BY v1`, [][]string{{"sysadmin", "/api/nodeServer/deployment", "GET"}, {"sysadmin", "/api/nodeServer/downloadDeployment", "POST"}})
		m, err := casbinmodel.NewModelFromString(`[request_definition]
r = sub, obj, act
[policy_definition]
p = sub, obj, act
[role_definition]
g = _, _
[policy_effect]
e = some(where (p.eft == allow))
[matchers]
m = g(r.sub, p.sub) && r.obj == p.obj && r.act == p.act
`)
		if err != nil {
			t.Fatal(err)
		}
		enforcer, err := casbin.NewEnforcer(m)
		if err != nil {
			t.Fatal(err)
		}
		rows, err := testDB.Query(`SELECT v0,v1,v2 FROM casbin_rule`)
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			var role, path, method string
			if err := rows.Scan(&role, &path, &method); err != nil {
				t.Fatal(err)
			}
			if _, err := enforcer.AddPolicy(role, path, method); err != nil {
				t.Fatal(err)
			}
		}
		if err := rows.Close(); err != nil {
			t.Fatal(err)
		}
		for _, role := range []string{"sysadmin", "admin", "user"} {
			for _, permission := range []struct{ path, method string }{{"/api/nodeServer/deployment", "GET"}, {"/api/nodeServer/downloadDeployment", "POST"}} {
				allowed, err := enforcer.Enforce(role, permission.path, permission.method)
				if err != nil || allowed != (role == "sysadmin") {
					t.Fatalf("unexpected %s permission %s: allowed=%v err=%v", role, permission.path, allowed, err)
				}
			}
		}
	})
}
