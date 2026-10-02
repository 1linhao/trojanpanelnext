package api

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"net"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/go-sql-driver/mysql"
	"trojan-panel/core"
	"trojan-panel/dao"
	"trojan-panel/dao/redis"
	"trojan-panel/model"
	"trojan-panel/model/constant"
	"trojan-panel/model/vo"
	"trojan-panel/service"
	"trojan-panel/util"
)

// Run only against disposable MariaDB/Redis instances. The production database
// initializer uses trojan_panel_db; exclusive creation prevents overwriting an
// existing database, and cleanup only drops the database created by this test.
func TestAccountRemarkAPIPrivacy(t *testing.T) {
	dsn, redisAddress := os.Getenv("TP_ACCOUNT_REMARK_TEST_DSN"), os.Getenv("TP_ACCOUNT_REMARK_TEST_REDIS")
	if dsn == "" || redisAddress == "" {
		t.Skip("TP_ACCOUNT_REMARK_TEST_DSN and TP_ACCOUNT_REMARK_TEST_REDIS are not set")
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
		t.Fatalf("requires unused disposable database instance: %v", err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec("DROP DATABASE trojan_panel_db"); err != nil {
			t.Errorf("cleanup database: %v", err)
		}
	})
	previousConfig := *core.Config
	core.Config.MySQLConfig = core.MySQLConfig{Host: host, Port: port, User: config.User, Password: config.Passwd}
	core.Config.RedisConfig = core.RedisConfig{Host: redisHost, Port: redisPort, MaxIdle: 2, MaxActive: 8, Wait: true}
	t.Cleanup(func() { *core.Config = previousConfig })
	dao.InitMySQL()
	t.Cleanup(dao.CloseDb)
	redis.InitRedis()
	t.Cleanup(redis.CloseRedis)
	InitValidator()
	config.DBName = "trojan_panel_db"
	testDB, err := sql.Open("mysql", config.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { testDB.Close() })
	var bootstrapUser, bootstrapRemark string
	if err := testDB.QueryRow("SELECT username,remark FROM account WHERE id=1").Scan(&bootstrapUser, &bootstrapRemark); err != nil || bootstrapUser != "sysadmin" || bootstrapRemark != "" {
		t.Fatalf("fresh bootstrap/migration corrupted initial account: %s %s %v", bootstrapUser, bootstrapRemark, err)
	}
	note := "private-note-内部🙂"
	pass := util.Sha1String("fixtureuserfixturepass")
	if _, err := testDB.Exec(`INSERT INTO account (id,username,pass,hash,role_id,email,quota,expire_time,remark) VALUES (3,'fixtureuser',?,'fixture-hash',3,'',-1,4078656000000,?)`, pass, note); err != nil {
		t.Fatal(err)
	}
	system, err := json.Marshal(vo.SystemVo{ClashRule: "rules: []", SingBoxTun: "{}", SingBoxOutbound: "{}", XrayTemplate: "{}"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := redis.Client.String.Set("trojan-panel:system", system).Result(); err != nil {
		t.Fatal(err)
	}
	roles := []struct {
		name  string
		id    uint
		roles []string
	}{{"sysadmin", 1, []string{"sysadmin", "admin", "user"}}, {"admin", 2, []string{"admin", "user"}}, {"user", 3, []string{"user"}}}
	tokens := make(map[string]string)
	for _, role := range roles {
		token, err := service.GenToken(vo.AccountVo{Id: role.id, RoleId: role.id, Username: role.name, Roles: role.roles})
		if err != nil {
			t.Fatal(err)
		}
		tokens[role.name] = token
	}

	t.Run("management_reads_filtered_without_route_middleware", func(t *testing.T) {
		for _, role := range roles {
			for _, endpoint := range []struct {
				name, path string
				handler    gin.HandlerFunc
			}{{"detail", "/account/selectAccountById?id=3", SelectAccountById}, {"list", "/account/selectAccountPage?pageNum=1&pageSize=20", SelectAccountPage}} {
				writer := remarkRequest(t, endpoint.handler, "GET", endpoint.path, "", tokens[role.name])
				response := remarkResponse(t, writer)
				if response["type"] != "success" {
					t.Fatalf("%s %s: %s", role.name, endpoint.name, writer.Body.String())
				}
				if writer.Header().Get("Cache-Control") != "private, no-store" {
					t.Fatal("management response permits note caching")
				}
				if role.name == "sysadmin" {
					if !strings.Contains(writer.Body.String(), note) || !strings.Contains(writer.Body.String(), `"remark":`) {
						t.Fatalf("sysadmin cannot read note: %s", writer.Body.String())
					}
				} else if strings.Contains(writer.Body.String(), note) || strings.Contains(writer.Body.String(), `"remark":`) {
					t.Fatalf("%s received private note: %s", role.name, writer.Body.String())
				}
			}
		}
	})

	t.Run("non_sysadmin_cannot_update_or_clear_note", func(t *testing.T) {
		for _, role := range []string{"admin", "user"} {
			for _, value := range []string{"unauthorized", ""} {
				encoded, _ := json.Marshal(value)
				writer := remarkRequest(t, UpdateAccountById, "POST", "/account/updateAccountById", strings.Replace(accountRemarkUpdateJSON, `"id":2`, `"id":3`, 1)+string(encoded)+"}", tokens[role])
				if got := remarkResponse(t, writer)["message"]; got != constant.ForbiddenError {
					t.Fatalf("%s altered private note: %s", role, writer.Body.String())
				}
				id := uint(3)
				if err := service.UpdateAccountById(tokens[role], &model.Account{Id: &id, Remark: &value}); err == nil || err.Error() != constant.ForbiddenError {
					t.Fatalf("service bypass permits %s note modification: %v", role, err)
				}
			}
		}
		if got := remarkStored(t, testDB); got != note {
			t.Fatalf("unauthorized operation changed stored note: %s", got)
		}
	})

	t.Run("current_role_revokes_privacy_even_with_old_sysadmin_token", func(t *testing.T) {
		if _, err := testDB.Exec(`INSERT INTO account (id,username,pass,hash,role_id,email,quota,expire_time) VALUES (4,'demotedadmin','fixture-pass','fixture-hash',1,'',-1,4078656000000)`); err != nil {
			t.Fatal(err)
		}
		token, err := service.GenToken(vo.AccountVo{Id: 4, RoleId: 1, Username: "demotedadmin", Roles: []string{"sysadmin", "admin", "user"}})
		if err != nil {
			t.Fatal(err)
		}
		before := remarkRequest(t, SelectAccountById, "GET", "/account/selectAccountById?id=3", "", token)
		if !strings.Contains(before.Body.String(), note) {
			t.Fatal("fixture sysadmin cannot read note before downgrade")
		}
		if _, err := testDB.Exec("UPDATE account SET role_id=2 WHERE id=4"); err != nil {
			t.Fatal(err)
		}
		for _, endpoint := range []struct {
			path    string
			handler gin.HandlerFunc
		}{{"/account/selectAccountById?id=3", SelectAccountById}, {"/account/selectAccountPage?pageNum=1&pageSize=20", SelectAccountPage}} {
			writer := remarkRequest(t, endpoint.handler, "GET", endpoint.path, "", token)
			if remarkResponse(t, writer)["type"] != "success" || strings.Contains(writer.Body.String(), `"remark":`) || strings.Contains(writer.Body.String(), note) {
				t.Fatalf("old sysadmin JWT bypassed role downgrade: %s", writer.Body.String())
			}
		}
		for _, value := range []string{"forbidden-after-downgrade", ""} {
			encoded, _ := json.Marshal(value)
			writer := remarkRequest(t, UpdateAccountById, "POST", "/account/updateAccountById", strings.Replace(accountRemarkUpdateJSON, `"id":2`, `"id":3`, 1)+string(encoded)+"}", token)
			if remarkResponse(t, writer)["message"] != constant.ForbiddenError {
				t.Fatalf("old JWT edits note after downgrade: %s", writer.Body.String())
			}
			id := uint(3)
			if err := service.UpdateAccountById(token, &model.Account{Id: &id, Remark: &value}); err == nil || err.Error() != constant.ForbiddenError {
				t.Fatalf("service permits note modification after downgrade: %v", err)
			}
		}
		if got := remarkStored(t, testDB); got != note {
			t.Fatal("downgraded session changed stored note")
		}
	})

	t.Run("missing_identity_or_claim_fails_closed", func(t *testing.T) {
		for _, identity := range []vo.AccountVo{
			{Id: 999, RoleId: 1, Username: "missing", Roles: []string{"sysadmin"}},
			{Id: 1, RoleId: 2, Username: "sysadmin", Roles: []string{"admin", "user"}},
		} {
			token, err := service.GenToken(identity)
			if err != nil {
				t.Fatal(err)
			}
			writer := remarkRequest(t, SelectAccountById, "GET", "/account/selectAccountById?id=3", "", token)
			if strings.Contains(writer.Body.String(), `"remark":`) || strings.Contains(writer.Body.String(), note) {
				t.Fatalf("unverified identity exposes note: %s", writer.Body.String())
			}
			body := strings.Replace(accountRemarkUpdateJSON, `"id":2`, `"id":3`, 1) + `"unauthorized"}`
			writer = remarkRequest(t, UpdateAccountById, "POST", "/account/updateAccountById", body, token)
			if remarkResponse(t, writer)["type"] != "error" || remarkStored(t, testDB) != note {
				t.Fatalf("unverified identity modifies note: %s", writer.Body.String())
			}
		}
	})

	t.Run("missing_invalid_token_returns_one_error_response", func(t *testing.T) {
		for _, token := range []string{"", "invalid-token"} {
			for _, endpoint := range []struct {
				method, path, body string
				handler            gin.HandlerFunc
			}{
				{"GET", "/account/selectAccountById?id=3", "", SelectAccountById},
				{"GET", "/account/selectAccountPage?pageNum=1&pageSize=20", "", SelectAccountPage},
				{"POST", "/account/updateAccountById", strings.Replace(accountRemarkUpdateJSON, `"id":2`, `"id":3`, 1) + `"unauthorized"}`, UpdateAccountById},
			} {
				writer := remarkRequest(t, endpoint.handler, endpoint.method, endpoint.path, endpoint.body, token)
				// Unmarshal rejects concatenated error+success JSON responses.
				response := remarkResponse(t, writer)
				if response["type"] != "error" || strings.Contains(writer.Body.String(), note) || strings.Contains(writer.Body.String(), `"remark":`) {
					t.Fatalf("invalid token response exposes note or succeeds: %s", writer.Body.String())
				}
			}
		}
		if remarkStored(t, testDB) != note {
			t.Fatal("invalid token changed stored note")
		}
	})

	t.Run("profile_login_subscription_never_expose_note", func(t *testing.T) {
		for _, token := range tokens {
			writer := remarkRequest(t, GetAccountInfo, "GET", "/account/getAccountInfo", "", token)
			if strings.Contains(writer.Body.String(), note) || strings.Contains(writer.Body.String(), `"remark":`) {
				t.Fatalf("profile leaks note: %s", writer.Body.String())
			}
		}
		writer := remarkRequest(t, Login, "POST", "/login", `{"username":"fixtureuser","pass":"fixturepass"}`, "")
		response := remarkResponse(t, writer)
		if response["type"] != "success" {
			t.Fatalf("login failed: %s", writer.Body.String())
		}
		token := response["data"].(map[string]interface{})["token"].(string)
		claims, err := service.ParseToken(token)
		if err != nil {
			t.Fatal(err)
		}
		claimJSON, _ := json.Marshal(claims)
		if strings.Contains(string(claimJSON), note) || strings.Contains(string(claimJSON), `"remark":`) {
			t.Fatalf("login claims leak note: %s", claimJSON)
		}
		if got := remarkStored(t, testDB); got != note {
			t.Fatal("login changed remark")
		}
		for _, client := range []string{"sing-box", "clash-meta", "v2ray", "shadowrocket"} {
			ctx, writer := remarkContext("GET", "/subscribe?client="+client, "", "")
			ctx.Params = gin.Params{{Key: "token", Value: base64.RawURLEncoding.EncodeToString([]byte(pass))}}
			Subscribe(ctx)
			if strings.Contains(writer.Body.String(), note) || strings.Contains(writer.Body.String(), `"remark":`) {
				t.Fatalf("%s subscription exposed note", client)
			}
		}
	})

	t.Run("sysadmin_updates_clears_and_omission_preserves", func(t *testing.T) {
		base := strings.Replace(accountRemarkUpdateJSON, `"id":2`, `"id":3`, 1)
		value := strings.Repeat("🙂", 500)
		encoded, _ := json.Marshal(value)
		writer := remarkRequest(t, UpdateAccountById, "POST", "/account/updateAccountById", base+string(encoded)+"}", tokens["sysadmin"])
		if remarkResponse(t, writer)["type"] != "success" || remarkStored(t, testDB) != value {
			t.Fatalf("sysadmin Unicode update failed: %s", writer.Body.String())
		}
		omitted := strings.TrimSuffix(base, `,"remark":`) + "}"
		writer = remarkRequest(t, UpdateAccountById, "POST", "/account/updateAccountById", omitted, tokens["admin"])
		if remarkResponse(t, writer)["type"] != "success" || remarkStored(t, testDB) != value {
			t.Fatalf("omitted remark did not preserve value: %s", writer.Body.String())
		}
		writer = remarkRequest(t, UpdateAccountById, "POST", "/account/updateAccountById", base+"null}", tokens["sysadmin"])
		if remarkResponse(t, writer)["type"] != "success" || remarkStored(t, testDB) != value {
			t.Fatalf("explicit null did not preserve remark: %s", writer.Body.String())
		}
		for _, invalid := range []string{`123`, `{}`, `[]`} {
			writer = remarkRequest(t, UpdateAccountById, "POST", "/account/updateAccountById", base+invalid+"}", tokens["sysadmin"])
			if remarkResponse(t, writer)["message"] != constant.ValidateFailed || remarkStored(t, testDB) != value {
				t.Fatalf("malformed note reached persistence: %s", writer.Body.String())
			}
		}
		overlong, _ := json.Marshal(strings.Repeat("🙂", 501))
		writer = remarkRequest(t, UpdateAccountById, "POST", "/account/updateAccountById", base+string(overlong)+"}", tokens["sysadmin"])
		if remarkResponse(t, writer)["message"] != constant.ValidateFailed || remarkStored(t, testDB) != value {
			t.Fatal("overlong note reached persistence")
		}
		writer = remarkRequest(t, UpdateAccountById, "POST", "/account/updateAccountById", base+`""}`, tokens["sysadmin"])
		if remarkResponse(t, writer)["type"] != "success" || remarkStored(t, testDB) != "" {
			t.Fatalf("explicit empty did not clear remark: %s", writer.Body.String())
		}
	})
}

func remarkContext(method, path, body, token string) (*gin.Context, *httptest.ResponseRecorder) {
	writer := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(writer)
	ctx.Request = httptest.NewRequest(method, path, strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	if token != "" {
		ctx.Request.Header.Set("Authorization", "Bearer "+token)
	}
	return ctx, writer
}
func remarkRequest(t *testing.T, handler gin.HandlerFunc, method, path, body, token string) *httptest.ResponseRecorder {
	t.Helper()
	ctx, writer := remarkContext(method, path, body, token)
	handler(ctx)
	return writer
}
func remarkResponse(t *testing.T, writer *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var response map[string]interface{}
	if err := json.Unmarshal(writer.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	return response
}
func remarkStored(t *testing.T, testDB *sql.DB) string {
	t.Helper()
	var value string
	if err := testDB.QueryRow("SELECT remark FROM account WHERE id=3").Scan(&value); err != nil {
		t.Fatal(err)
	}
	return value
}
