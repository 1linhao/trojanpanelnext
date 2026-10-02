package api

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"net"
	"net/http/httptest"
	"os"
	"reflect"
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
	var resetPermissionCount, allResetPermissions int
	if err := testDB.QueryRow(`SELECT COUNT(*) FROM casbin_rule WHERE p_type='p' AND v0='sysadmin' AND v1='/api/account/resetAccountLoginLimit' AND v2='POST'`).Scan(&resetPermissionCount); err != nil || resetPermissionCount != 1 {
		t.Fatalf("new database reset permission count=%d err=%v", resetPermissionCount, err)
	}
	if err := testDB.QueryRow(`SELECT COUNT(*) FROM casbin_rule WHERE v1='/api/account/resetAccountLoginLimit'`).Scan(&allResetPermissions); err != nil || allResetPermissions != 1 {
		t.Fatalf("new database granted reset outside sysadmin POST: count=%d err=%v", allResetPermissions, err)
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
			base := strings.Replace(accountRemarkUpdateJSON, `"id":2`, `"id":3`, 1)
			for _, body := range []string{base + "null}", strings.TrimSuffix(base, `,"remark":`) + "}"} {
				writer := remarkRequest(t, UpdateAccountById, "POST", "/account/updateAccountById", body, tokens[role])
				if remarkResponse(t, writer)["message"] != constant.ForbiddenError {
					t.Fatalf("%s edited account through null/omitted remark: %s", role, writer.Body.String())
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
		promote := `{"id":4,"username":"demotedadmin","quota":-1,"roleId":1,"deleted":0,"expireTime":4078656000000}`
		writer := remarkRequest(t, UpdateAccountById, "POST", "/account/updateAccountById", promote, token)
		if remarkResponse(t, writer)["message"] != constant.ForbiddenError {
			t.Fatalf("old sysadmin JWT can re-promote itself without a remark: %s", writer.Body.String())
		}
		var currentRole uint
		if err := testDB.QueryRow("SELECT role_id FROM account WHERE id=4").Scan(&currentRole); err != nil || currentRole != 2 {
			t.Fatalf("re-promotion changed persisted role: %d %v", currentRole, err)
		}
		id, role, deleted := uint(4), uint(1), uint(0)
		if err := service.UpdateAccountById(token, &model.Account{Id: &id, RoleId: &role, Deleted: &deleted}); err == nil || err.Error() != constant.ForbiddenError {
			t.Fatalf("service permits role re-promotion after downgrade: %v", err)
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

	t.Run("reset_login_limit_is_authorized_and_isolated", func(t *testing.T) {
		if _, err := testDB.Exec(`INSERT INTO account (id,username,pass,hash,role_id,email,quota,expire_time) VALUES (5,'fixtureother','fixture-pass','fixture-hash',3,'',-1,4078656000000),(6,'blockedadmin','fixture-pass','fixture-hash',1,'',-1,4078656000000)`); err != nil {
			t.Fatal(err)
		}
		stale, err := service.GenToken(vo.AccountVo{Id: 6, RoleId: 1, Username: "blockedadmin", Roles: []string{"sysadmin", "admin", "user"}})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := testDB.Exec("UPDATE account SET role_id=2 WHERE id=6"); err != nil {
			t.Fatal(err)
		}
		targetKey, otherKey, similarKey := "trojan-panel:login-limit:fixtureuser", "trojan-panel:login-limit:fixtureother", "trojan-panel:login-limit:fixtureuser-extra"
		for key, value := range map[string]int{targetKey: -1, otherKey: -1, similarKey: 2} {
			if _, err := redis.Client.String.Set(key, value, 1800).Result(); err != nil {
				t.Fatal(err)
			}
		}
		for _, body := range []string{`{}`, `{"id":0}`, `{"id":null}`, `{"id":-1}`, `{"id":"3"}`, `{"id":3.5}`, `{"id":3,"username":"fixtureother"}`, `{"id":3} {"id":5}`, `{`} {
			writer := remarkRequest(t, ResetAccountLoginLimit, "POST", "/api/account/resetAccountLoginLimit", body, tokens["sysadmin"])
			if remarkResponse(t, writer)["message"] != constant.ValidateFailed {
				t.Fatalf("invalid reset payload accepted: %s", writer.Body.String())
			}
			if got, err := redis.Client.String.Get(targetKey).Int(); err != nil || got != -1 {
				t.Fatalf("invalid reset modified target: %d %v", got, err)
			}
		}
		for _, token := range []string{"", "invalid-token", tokens["admin"], tokens["user"], stale} {
			writer := remarkRequest(t, ResetAccountLoginLimit, "POST", "/api/account/resetAccountLoginLimit", `{"id":3}`, token)
			if remarkResponse(t, writer)["type"] != "error" {
				t.Fatalf("unauthorized login reset accepted: %s", writer.Body.String())
			}
			id := uint(3)
			if err := service.ResetAccountLoginLimit(token, &id); err == nil {
				t.Fatal("service accepted unauthorized reset")
			}
			if got, err := redis.Client.String.Get(targetKey).Int(); err != nil || got != -1 {
				t.Fatalf("unauthorized reset modified target: %d %v", got, err)
			}
		}
		for _, authorization := range []string{"Bearer", "not-a-token", "Bearer ", "Bearer invalid-token extra"} {
			ctx, writer := remarkContext("POST", "/api/account/resetAccountLoginLimit", `{"id":3}`, "")
			ctx.Request.Header.Set("Authorization", authorization)
			ResetAccountLoginLimit(ctx)
			if remarkResponse(t, writer)["type"] != "error" {
				t.Fatalf("malformed Authorization accepted: %s", writer.Body.String())
			}
			if got, err := redis.Client.String.Get(targetKey).Int(); err != nil || got != -1 {
				t.Fatal("malformed Authorization reset modified target")
			}
		}
		writer := remarkRequest(t, ResetAccountLoginLimit, "POST", "/api/account/resetAccountLoginLimit", `{"id":999}`, tokens["sysadmin"])
		if remarkResponse(t, writer)["type"] != "error" {
			t.Fatal("missing account reset accepted")
		}
		if got, err := redis.Client.String.Get(targetKey).Int(); err != nil || got != -1 {
			t.Fatal("missing account reset changed target")
		}
		// Start with no failures, then use the real login handler to lock it.
		if _, err := redis.Client.Key.Del(targetKey).Result(); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 3; i++ {
			writer = remarkRequest(t, Login, "POST", "/login", `{"username":"fixtureuser","pass":"wrongpass"}`, "")
			if remarkResponse(t, writer)["message"] != constant.UsernameOrPassError {
				t.Fatalf("wrong login %d: %s", i+1, writer.Body.String())
			}
		}
		writer = remarkRequest(t, Login, "POST", "/login", `{"username":"fixtureuser","pass":"fixturepass"}`, "")
		if remarkResponse(t, writer)["message"] != constant.LoginLimitError {
			t.Fatalf("three wrong logins did not lock account: %s", writer.Body.String())
		}
		if got, err := redis.Client.String.Get(targetKey).Int(); err != nil || got != -1 {
			t.Fatalf("lock sentinel=%d err=%v", got, err)
		}
		if ttl, err := redis.Client.Key.TTL(targetKey).Int(); err != nil || ttl <= 0 || ttl > 1800 {
			t.Fatalf("lock TTL=%d err=%v", ttl, err)
		}
		if _, err := redis.Client.String.Set("trojan-panel:token:fixtureuser", "session-sentinel").Result(); err != nil {
			t.Fatal(err)
		}
		jwtBefore, err := redis.Client.String.Get("trojan-panel:jwt-key").String()
		if err != nil {
			t.Fatal(err)
		}
		var before, after [7]string
		query := "SELECT pass,deleted,quota,download,upload,remark,role_id FROM account WHERE id=3"
		if err := testDB.QueryRow(query).Scan(&before[0], &before[1], &before[2], &before[3], &before[4], &before[5], &before[6]); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 2; i++ {
			writer = remarkRequest(t, ResetAccountLoginLimit, "POST", "/api/account/resetAccountLoginLimit", `{"id":3}`, tokens["sysadmin"])
			if remarkResponse(t, writer)["type"] != "success" {
				t.Fatalf("reset/repeat failed: %s", writer.Body.String())
			}
			if exists, err := redis.Client.Key.Exists(targetKey).Bool(); err != nil || exists {
				t.Fatalf("reset retained target failure key: %v %v", exists, err)
			}
		}
		if err := testDB.QueryRow(query).Scan(&after[0], &after[1], &after[2], &after[3], &after[4], &after[5], &after[6]); err != nil || !reflect.DeepEqual(before, after) {
			t.Fatalf("reset changed account data: %v %v %v", before, after, err)
		}
		if jwtAfter, err := redis.Client.String.Get("trojan-panel:jwt-key").String(); err != nil || jwtAfter != jwtBefore {
			t.Fatal("reset changed JWT signing key")
		}
		if session, err := redis.Client.String.Get("trojan-panel:token:fixtureuser").String(); err != nil || session != "session-sentinel" {
			t.Fatal("reset revoked account session")
		}
		if value, err := redis.Client.String.Get(otherKey).Int(); err != nil || value != -1 {
			t.Fatal("reset removed another account lock")
		}
		if ttl, err := redis.Client.Key.TTL(otherKey).Int(); err != nil || ttl < 1790 || ttl > 1800 {
			t.Fatalf("reset changed another account lock TTL: %d %v", ttl, err)
		}
		if value, err := redis.Client.String.Get(similarKey).Int(); err != nil || value != 2 {
			t.Fatal("reset cleared similarly prefixed failure counter")
		}
		// A counter that has not yet become a timed lock is cleared as well.
		if _, err := redis.Client.String.Set(targetKey, 2).Result(); err != nil {
			t.Fatal(err)
		}
		writer = remarkRequest(t, ResetAccountLoginLimit, "POST", "/api/account/resetAccountLoginLimit", `{"id":3}`, tokens["sysadmin"])
		if remarkResponse(t, writer)["type"] != "success" {
			t.Fatal("counter reset failed")
		}
		writer = remarkRequest(t, Login, "POST", "/login", `{"username":"fixtureuser","pass":"fixturepass"}`, "")
		if remarkResponse(t, writer)["type"] != "success" {
			t.Fatalf("correct login after reset failed: %s", writer.Body.String())
		}
		if value, err := redis.Client.String.Get(otherKey).Int(); err != nil || value != -1 {
			t.Fatal("target login changed another account failure state")
		}
	})

	t.Run("profile_login_subscription_never_expose_note", func(t *testing.T) {
		for _, token := range tokens {
			writer := remarkRequest(t, GetAccountInfo, "GET", "/account/getAccountInfo", "", token)
			if strings.Contains(writer.Body.String(), note) || strings.Contains(writer.Body.String(), `"remark":`) {
				t.Fatalf("profile leaks note: %s", writer.Body.String())
			}
		}
		if _, err := testDB.Exec("UPDATE account SET last_login_time=0,preset_expire=7,preset_quota=1048576 WHERE id=3"); err != nil {
			t.Fatal(err)
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
		var lastLoginTime uint
		var quota int
		if err := testDB.QueryRow("SELECT last_login_time,quota FROM account WHERE id=3").Scan(&lastLoginTime, &quota); err != nil || lastLoginTime == 0 || quota != 1048576 {
			t.Fatalf("ordinary first-login bookkeeping was blocked: lastLoginTime=%d quota=%d err=%v", lastLoginTime, quota, err)
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
		writer = remarkRequest(t, UpdateAccountById, "POST", "/account/updateAccountById", omitted, tokens["sysadmin"])
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
