package api

import (
	"context"
	"database/sql"
	"net"
	"os"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/go-sql-driver/mysql"
	"trojan-panel/core"
	"trojan-panel/dao"
	"trojan-panel/dao/redis"
	"trojan-panel/model/constant"
	"trojan-panel/model/vo"
	"trojan-panel/service"
)

// Uses disposable MariaDB/Redis instances only. The production initializer has
// a fixed database name; exclusive creation prevents changing an existing DB.
func TestNodeContainerAPICurrentRoleIntegration(t *testing.T) {
	dsn, redisAddress := os.Getenv("TP_NODE_CONTAINER_TEST_DSN"), os.Getenv("TP_NODE_CONTAINER_TEST_REDIS")
	if dsn == "" || redisAddress == "" {
		t.Skip("TP_NODE_CONTAINER_TEST_DSN and TP_NODE_CONTAINER_TEST_REDIS are not set")
	}
	config, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatal(err)
	}
	host, portText, err := net.SplitHostPort(config.Addr)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatal(err)
	}
	redisHost, redisPortText, err := net.SplitHostPort(redisAddress)
	if err != nil {
		t.Fatal(err)
	}
	redisPort, err := strconv.Atoi(redisPortText)
	if err != nil {
		t.Fatal(err)
	}
	config.DBName = ""
	admin, err := sql.Open("mysql", config.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { admin.Close() })
	if _, err := admin.Exec("CREATE DATABASE trojan_panel_db CHARACTER SET utf8mb4"); err != nil {
		t.Fatalf("requires an unused disposable database instance: %v", err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec("DROP DATABASE trojan_panel_db"); err != nil {
			t.Errorf("cleanup database: %v", err)
		}
	})
	previous := *core.Config
	core.Config.MySQLConfig = core.MySQLConfig{Host: host, Port: port, User: config.User, Password: config.Passwd}
	core.Config.RedisConfig = core.RedisConfig{Host: redisHost, Port: redisPort, Password: os.Getenv("TP_NODE_CONTAINER_TEST_REDIS_PASSWORD"), MaxIdle: 2, MaxActive: 8, Wait: true}
	t.Cleanup(func() { *core.Config = previous })
	dao.InitMySQL()
	t.Cleanup(dao.CloseDb)
	redis.InitRedis()
	t.Cleanup(redis.CloseRedis)
	config.DBName = "trojan_panel_db"
	testDB, err := sql.Open("mysql", config.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { testDB.Close() })
	exec := func(query string, args ...interface{}) {
		t.Helper()
		if _, err := testDB.Exec(query, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`INSERT INTO node_server(id,name,ip,grpc_port,grpc_tls_mode,grpc_tls_server_name) VALUES(7,'fixture','192.0.2.7',8100,'legacy','node.example.test')`)
	exec(`INSERT INTO account(id,username,role_id) VALUES(2,'fixture-admin',2),(3,'fixture-user',3),(4,'fixture-demoted',1),(5,'fixture-disabled',1)`)
	tokens := make(map[uint]string)
	for _, id := range []uint{1, 2, 3, 4, 5} {
		// Give every signed fixture a sysadmin claim: the persisted role must
		// independently reject admin/user and a subsequent role downgrade.
		token, err := service.GenToken(vo.AccountVo{Id: id, RoleId: 1, Roles: []string{"sysadmin", "admin", "user"}})
		if err != nil {
			t.Fatal(err)
		}
		tokens[id] = token
	}
	endpoints := []struct {
		method, path, body string
		handler            gin.HandlerFunc
	}{{"GET", "/api/container/inventory?nodeServerId=7", "", NodeContainerInventory}, {"POST", "/api/container/update", `{"nodeServerId":7}`, UpdateNodeContainer}}
	check := func(token, message string) {
		t.Helper()
		for _, endpoint := range endpoints {
			writer := remarkRequest(t, endpoint.handler, endpoint.method, endpoint.path, endpoint.body, token)
			response := remarkResponse(t, writer)
			if response["type"] != "error" || response["message"] != message || response["data"] != nil || writer.Header().Get("Cache-Control") != "private, no-store" {
				t.Fatalf("%s guard response: %s", endpoint.path, writer.Body.String())
			}
		}
		if _, err := service.GetNodeContainerInventory(context.Background(), token, 7); err == nil || err.Error() != message {
			t.Fatalf("direct inventory guard: %v", err)
		}
		if _, err := service.StartNodeContainerUpdate(context.Background(), token, 7); err == nil || err.Error() != message {
			t.Fatalf("direct update guard: %v", err)
		}
	}
	t.Run("persisted_role_required_beyond_signed_claim", func(t *testing.T) {
		check(tokens[1], core.ErrHostContainerUnsupported.Error())
		check(tokens[2], constant.ForbiddenError)
		check(tokens[3], constant.ForbiddenError)
		nonSysadminClaim, err := service.GenToken(vo.AccountVo{Id: 1, RoleId: 3, Roles: []string{"user"}})
		if err != nil {
			t.Fatal(err)
		}
		check(nonSysadminClaim, constant.ForbiddenError)
	})
	t.Run("stale_jwt_downgrade_and_disable_take_effect_immediately", func(t *testing.T) {
		check(tokens[4], core.ErrHostContainerUnsupported.Error())
		check(tokens[5], core.ErrHostContainerUnsupported.Error())
		exec(`UPDATE account SET role_id=2 WHERE id=4`)
		exec(`UPDATE account SET deleted=1 WHERE id=5`)
		check(tokens[4], constant.ForbiddenError)
		check(tokens[5], constant.ForbiddenError)
	})
	t.Run("missing_or_malformed_bearer_returns_one_error", func(t *testing.T) {
		for _, header := range []string{"", "Bearer", "Basic fixture", "Bearer invalid", "Bearer invalid extra"} {
			for _, endpoint := range endpoints {
				ctx, writer := remarkContext(endpoint.method, endpoint.path, endpoint.body, "")
				ctx.Request.Header.Set("Authorization", header)
				endpoint.handler(ctx)
				response := remarkResponse(t, writer)
				if response["type"] != "error" || response["message"] != constant.IllegalTokenError || response["data"] != nil {
					t.Fatalf("malformed bearer response: %s", writer.Body.String())
				}
			}
		}
	})
	t.Run("startup_migration_new_and_existing_database", func(t *testing.T) {
		for iteration := 0; iteration < 2; iteration++ {
			var permissions int
			if err := testDB.QueryRow(`SELECT COUNT(*) FROM casbin_rule WHERE v1 LIKE '/api/container/%'`).Scan(&permissions); err != nil || permissions != 2 {
				t.Fatalf("startup permissions=%d err=%v", permissions, err)
			}
			var otherPermissions int
			if err := testDB.QueryRow(`SELECT COUNT(*) FROM casbin_rule WHERE v1 LIKE '/api/container/%' AND v0!='sysadmin'`).Scan(&otherPermissions); err != nil || otherPermissions != 0 {
				t.Fatalf("non-sysadmin permissions=%d err=%v", otherPermissions, err)
			}
			if iteration == 0 {
				dao.CloseDb()
				dao.InitMySQL()
			}
		}
		var nodes, items int
		if err := testDB.QueryRow(`SELECT COUNT(*) FROM node_server WHERE id=7 AND removing=0`).Scan(&nodes); err != nil || nodes != 1 {
			t.Fatalf("registration changed: %d %v", nodes, err)
		}
		if err := testDB.QueryRow(`SELECT COUNT(*) FROM kernel_upgrade_task_item`).Scan(&items); err != nil || items != 0 {
			t.Fatalf("guards changed kernel tasks: %d %v", items, err)
		}
	})
}
