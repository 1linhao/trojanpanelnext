package router

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	redigo "github.com/gomodule/redigo/redis"
	"trojan-panel/api"
	"trojan-panel/core"
	"trojan-panel/dao"
)

const managedNodeDeleteError = "this server is managed by a Node identity; use node-identity force-evict"

type deleteNodeServerResponse struct {
	Code    int    `json:"code"`
	Type    string `json:"type"`
	Message string `json:"message"`
}

type managedNodeFixture struct {
	identityID        string
	nodeServerID      uint64
	mariaDBUsername   string
	mariaDBPassword   string
	redisUsername     string
	redisPassword     string
	redisAuthUsername string
	redisAuthPassword string
	identitySnapshot  string
}

func TestDeleteNodeServerHTTPProtectsManagedIdentityLifecycle(t *testing.T) {
	adminDB, redisAddress, redisPassword := openNodeServerDeleteIntegration(t)
	resetNodeServerDeleteSchema(t, adminDB)

	api.InitValidator()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	initNodeServerRouter(engine.Group("/api"))

	for index, status := range []string{"active", "revoked", "provisioning", "rotating"} {
		t.Run(status, func(t *testing.T) {
			fixture := createManagedNodeFixture(t, adminDB, redisAddress, redisPassword, status, index)
			assertManagedCredentialsWork(t, fixture, redisAddress)

			response := deleteNodeServerRequest(t, engine, fixture.nodeServerID)
			if response.Type != "error" || response.Message != managedNodeDeleteError {
				t.Fatalf("managed %s delete response = %#v, want lifecycle conflict", status, response)
			}
			assertManagedFixtureUnchanged(t, adminDB, fixture)
			assertManagedCredentialsWork(t, fixture, redisAddress)
		})
	}

	t.Run("unmanaged legacy server", func(t *testing.T) {
		serverID := insertNodeServer(t, adminDB, "legacy-unmanaged")
		response := deleteNodeServerRequest(t, engine, serverID)
		if response.Type != "success" {
			t.Fatalf("unmanaged delete response = %#v, want success", response)
		}
		if countRows(t, adminDB, "SELECT COUNT(1) FROM node_server WHERE id=?", serverID) != 0 {
			t.Fatal("unmanaged node_server was not deleted")
		}
	})

	t.Run("missing server is idempotent", func(t *testing.T) {
		response := deleteNodeServerRequest(t, engine, 4294967295)
		if response.Type != "success" {
			t.Fatalf("missing server delete response = %#v, want idempotent success", response)
		}
	})

	t.Run("concurrent managed deletes all fail closed", func(t *testing.T) {
		fixture := createManagedNodeFixture(t, adminDB, redisAddress, redisPassword, "active", 10)
		const callers = 8
		responses := make(chan deleteNodeServerResponse, callers)
		var wait sync.WaitGroup
		for caller := 0; caller < callers; caller++ {
			wait.Add(1)
			go func() {
				defer wait.Done()
				response, err := executeDeleteNodeServerRequest(engine, fixture.nodeServerID)
				if err != nil {
					responses <- deleteNodeServerResponse{Message: err.Error()}
					return
				}
				responses <- response
			}()
		}
		wait.Wait()
		close(responses)
		for response := range responses {
			if response.Type != "error" || response.Message != managedNodeDeleteError {
				t.Fatalf("concurrent managed delete response = %#v, want lifecycle conflict", response)
			}
		}
		assertManagedFixtureUnchanged(t, adminDB, fixture)
		assertManagedCredentialsWork(t, fixture, redisAddress)
	})
}

func openNodeServerDeleteIntegration(t *testing.T) (*sql.DB, string, string) {
	t.Helper()
	host := os.Getenv("TP_TEST_MARIADB_HOST")
	port := os.Getenv("TP_TEST_MARIADB_PORT")
	password := os.Getenv("TP_TEST_MARIADB_PASSWORD")
	redisAddress := os.Getenv("TP_TEST_REDIS_ADDRESS")
	redisPassword := os.Getenv("TP_TEST_REDIS_PASSWORD")
	if host == "" || port == "" || password == "" || redisAddress == "" || redisPassword == "" {
		t.Skip("real MariaDB/Redis integration environment is not configured")
	}
	dsn := fmt.Sprintf("root:%s@tcp(%s:%s)/trojan_panel_db?parseTime=true&interpolateParams=true", password, host, port)
	adminDB, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	if err = adminDB.Ping(); err != nil {
		adminDB.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = adminDB.Close() })

	core.Config.MySQLConfig = core.MySQLConfig{Host: host, User: "root", Password: password}
	if _, err = fmt.Sscan(port, &core.Config.MySQLConfig.Port); err != nil {
		t.Fatal(err)
	}
	if err = dao.InitMySQLReadOnly(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(dao.CloseDb)
	return adminDB, redisAddress, redisPassword
}

func resetNodeServerDeleteSchema(t *testing.T, adminDB *sql.DB) {
	t.Helper()
	statements := []string{
		"DROP TABLE IF EXISTS node_identity_event",
		"DROP TABLE IF EXISTS node_identity",
		"DROP TABLE IF EXISTS node",
		"DROP TABLE IF EXISTS node_server",
		`CREATE TABLE node_server (
			id bigint unsigned NOT NULL AUTO_INCREMENT,
			ip varchar(64) NOT NULL DEFAULT '',
			name varchar(64) NOT NULL DEFAULT '',
			grpc_port int unsigned NOT NULL DEFAULT 8100,
			grpc_tls_mode varchar(16) NOT NULL DEFAULT 'mtls',
			grpc_tls_server_name varchar(253) NOT NULL DEFAULT '',
			traffic_period varchar(8) NOT NULL DEFAULT 'none',
			traffic_limit_mode varchar(8) NOT NULL DEFAULT 'combined',
			traffic_total_limit bigint unsigned NOT NULL DEFAULT 0,
			traffic_upload_limit bigint unsigned NOT NULL DEFAULT 0,
			traffic_download_limit bigint unsigned NOT NULL DEFAULT 0,
			create_time datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
			update_time datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			PRIMARY KEY (id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE node (
			id bigint unsigned NOT NULL AUTO_INCREMENT,
			node_server_id bigint NOT NULL DEFAULT 0,
			name varchar(64) NOT NULL DEFAULT '',
			PRIMARY KEY (id), KEY idx_node_server (node_server_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	}
	for _, statement := range statements {
		if _, err := adminDB.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	if err := dao.EnsureNodeIdentitySchema(context.Background(), adminDB); err != nil {
		t.Fatal(err)
	}
}

func createManagedNodeFixture(t *testing.T, adminDB *sql.DB, redisAddress, redisAdminPassword, status string, index int) managedNodeFixture {
	t.Helper()
	suffix := fmt.Sprintf("%s-%d", status, index)
	fixture := managedNodeFixture{
		identityID:        fmt.Sprintf("00000000-0000-4000-8000-%012d", index+1),
		mariaDBUsername:   fmt.Sprintf("tpn_delete_%d", index),
		mariaDBPassword:   randomTestPassword(t),
		redisUsername:     fmt.Sprintf("tpn-delete-%d", index),
		redisPassword:     randomTestPassword(t),
		redisAuthUsername: fmt.Sprintf("tpn-delete-auth-%d", index),
		redisAuthPassword: randomTestPassword(t),
	}
	fixture.nodeServerID = insertNodeServer(t, adminDB, "managed-"+suffix)
	_, err := adminDB.Exec(`INSERT INTO node_identity
		(identity_id,node_server_id,name,domain,public_ip,generation,mariadb_username,redis_username,redis_auth_username,credential_path,credential_nonce,credential_sha256,status)
		VALUES (?,?,?,?,?,2,?,?,?,?,?,?,?)`, fixture.identityID, fixture.nodeServerID, "managed-"+suffix,
		"managed-"+suffix+".example.com", "203.0.113.10", fixture.mariaDBUsername, fixture.redisUsername,
		fixture.redisAuthUsername, "/tmp/managed-"+suffix+".json", strings.Repeat("n", 43), strings.Repeat("a", 64), status)
	if err != nil {
		t.Fatal(err)
	}
	account := fmt.Sprintf("`%s`@'%%'", fixture.mariaDBUsername)
	if _, err = adminDB.Exec("CREATE USER "+account+" IDENTIFIED BY ?", fixture.mariaDBPassword); err != nil {
		t.Fatal(err)
	}
	if _, err = adminDB.Exec("GRANT SELECT ON trojan_panel_db.node_server TO " + account); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = adminDB.Exec("DROP USER IF EXISTS " + account) })

	redisAdmin := dialRedis(t, redisAddress, "default", redisAdminPassword)
	defer redisAdmin.Close()
	if _, err = redisAdmin.Do("ACL", "SETUSER", fixture.redisUsername, "reset", "on", ">"+fixture.redisPassword, "+ping", "~trojan-panel-core:*"); err != nil {
		t.Fatal(err)
	}
	if _, err = redisAdmin.Do("ACL", "SETUSER", fixture.redisAuthUsername, "reset", "on", ">"+fixture.redisAuthPassword, "+ping", "~trojan-panel:*"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup := dialRedis(t, redisAddress, "default", redisAdminPassword)
		defer cleanup.Close()
		_, _ = cleanup.Do("ACL", "DELUSER", fixture.redisUsername, fixture.redisAuthUsername)
	})
	fixture.identitySnapshot = identitySnapshot(t, adminDB, fixture.identityID)
	return fixture
}

func insertNodeServer(t *testing.T, adminDB *sql.DB, name string) uint64 {
	t.Helper()
	result, err := adminDB.Exec(`INSERT INTO node_server
		(ip,name,grpc_port,grpc_tls_mode,grpc_tls_server_name,traffic_period,traffic_limit_mode,traffic_total_limit,traffic_upload_limit,traffic_download_limit)
		VALUES ('203.0.113.10',?,8100,'mtls',?,'none','combined',0,0,0)`, name, name+".example.com")
	if err != nil {
		t.Fatal(err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return uint64(id)
}

func deleteNodeServerRequest(t *testing.T, engine http.Handler, id uint64) deleteNodeServerResponse {
	t.Helper()
	response, err := executeDeleteNodeServerRequest(engine, id)
	if err != nil {
		t.Fatal(err)
	}
	return response
}

func executeDeleteNodeServerRequest(engine http.Handler, id uint64) (deleteNodeServerResponse, error) {
	request := httptest.NewRequest(http.MethodPost, "/api/nodeServer/deleteNodeServerById", strings.NewReader(fmt.Sprintf(`{"id":%d}`, id)))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		return deleteNodeServerResponse{}, fmt.Errorf("HTTP status = %d, want 200", recorder.Code)
	}
	var response deleteNodeServerResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		return deleteNodeServerResponse{}, fmt.Errorf("decode response %q: %w", recorder.Body.String(), err)
	}
	return response, nil
}

func assertManagedFixtureUnchanged(t *testing.T, adminDB *sql.DB, fixture managedNodeFixture) {
	t.Helper()
	if countRows(t, adminDB, "SELECT COUNT(1) FROM node_server WHERE id=?", fixture.nodeServerID) != 1 {
		t.Fatal("managed node_server was deleted")
	}
	if got := identitySnapshot(t, adminDB, fixture.identityID); got != fixture.identitySnapshot {
		t.Fatalf("node_identity changed:\n got %s\nwant %s", got, fixture.identitySnapshot)
	}
}

func assertManagedCredentialsWork(t *testing.T, fixture managedNodeFixture, redisAddress string) {
	t.Helper()
	adminHost := os.Getenv("TP_TEST_MARIADB_HOST")
	adminPort := os.Getenv("TP_TEST_MARIADB_PORT")
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/trojan_panel_db?timeout=2s", fixture.mariaDBUsername, fixture.mariaDBPassword, adminHost, adminPort)
	database, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err = database.Ping(); err != nil {
		t.Fatalf("MariaDB credential changed: %v", err)
	}
	for _, credential := range []struct{ username, password string }{
		{fixture.redisUsername, fixture.redisPassword},
		{fixture.redisAuthUsername, fixture.redisAuthPassword},
	} {
		connection := dialRedis(t, redisAddress, credential.username, credential.password)
		if reply, pingErr := redigo.String(connection.Do("PING")); pingErr != nil || reply != "PONG" {
			connection.Close()
			t.Fatalf("Redis credential changed for %s: %v", credential.username, pingErr)
		}
		connection.Close()
	}
}

func identitySnapshot(t *testing.T, adminDB *sql.DB, identityID string) string {
	t.Helper()
	var snapshot string
	err := adminDB.QueryRow(`SELECT CONCAT_WS('|',identity_id,node_server_id,name,domain,public_ip,generation,mariadb_username,redis_username,redis_auth_username,credential_path,credential_nonce,credential_sha256,status)
		FROM node_identity WHERE identity_id=?`, identityID).Scan(&snapshot)
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func countRows(t *testing.T, adminDB *sql.DB, query string, arguments ...interface{}) int {
	t.Helper()
	var count int
	if err := adminDB.QueryRow(query, arguments...).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func dialRedis(t *testing.T, address, username, password string) redigo.Conn {
	t.Helper()
	connection, err := redigo.Dial("tcp", address, redigo.DialUsername(username), redigo.DialPassword(password))
	if err != nil {
		t.Fatal(err)
	}
	return connection
}

func randomTestPassword(t *testing.T) string {
	t.Helper()
	value := make([]byte, 24)
	if _, err := rand.Read(value); err != nil {
		t.Fatal(err)
	}
	return base64.RawURLEncoding.EncodeToString(value)
}
