package api

import (
	"encoding/base64"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"trojan-panel/dao/redis"
	"trojan-panel/middleware"
	"trojan-panel/model/constant"
	"trojan-panel/model/vo"
	"trojan-panel/service"
)

// Exercise the normal management handler, signed session middleware, common
// subscription entry point and quota-only read used by older node agents.
func TestDisabledAccountLifecycle(t *testing.T) {
	dsn, cache := os.Getenv("TP_ACCOUNT_REMARK_TEST_DSN"), os.Getenv("TP_ACCOUNT_REMARK_TEST_REDIS")
	if dsn == "" || cache == "" {
		t.Skip("disposable MariaDB and Redis required")
	}
	db := disposableAPIStore(t, dsn, cache)
	if _, err := db.Exec("INSERT INTO account(id,username,pass,quota,download,upload,expire_time) VALUES(7,'statususer','fixture',67108864,10,20,4078656000000)"); err != nil {
		t.Fatal(err)
	}
	mint := func(identity vo.AccountVo) string {
		t.Helper()
		token, err := service.GenToken(identity)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := redis.Client.String.Set("trojan-panel:token:"+identity.Username, token).Result(); err != nil {
			t.Fatal(err)
		}
		return token
	}
	admin := mint(vo.AccountVo{Id: 1, Username: "sysadmin", RoleId: 1, Roles: []string{"sysadmin"}})
	identity := vo.AccountVo{Id: 7, Username: "statususer", RoleId: 3, Roles: []string{"user"}}
	user := mint(identity)
	router := gin.New()
	router.GET("/account", middleware.JWTHandler(), GetAccountInfo)
	router.GET("/subscribe/:token", Subscribe)
	get := func(path, token string) *httptest.ResponseRecorder {
		t.Helper()
		w := httptest.NewRecorder()
		request := httptest.NewRequest("GET", path, nil)
		if token != "" {
			request.Header.Set("Authorization", "Bearer "+token)
		}
		router.ServeHTTP(w, request)
		return w
	}
	checkSession := func(token string, want int) {
		t.Helper()
		if got := remarkResponse(t, get("/account", token))["code"]; got != float64(want) {
			t.Fatalf("session code %v, want %d", got, want)
		}
	}
	update := func(disabled string) {
		t.Helper()
		body := `{"id":7,"username":"statususer","roleId":3,"expireTime":4078656000000,"deleted":` + disabled + "}"
		if got := remarkResponse(t, remarkRequest(t, UpdateAccountById, "POST", "/account/updateAccountById", body, admin))["code"]; got != float64(constant.CodeSuccess) {
			t.Fatalf("update code %v", got)
		}
	}
	checkSession(user, constant.CodeSuccess)
	update("1")
	checkSession(user, constant.CodeUnauthorizedError)
	for _, client := range []string{"clash-meta&template=default", "sing-box&template=tun", "v2ray&template=default", "shadowrocket&template=default"} {
		w := get("/subscribe/"+base64.RawURLEncoding.EncodeToString([]byte("fixture"))+"?client="+client, "")
		if w.Code != 403 || w.Header().Get("subscription-userinfo") != "" || w.Header().Get("content-disposition") != "" {
			t.Fatalf("disabled subscription returned %d", w.Code)
		}
	}
	service.CronScanAccounts()
	var quota, saved, down, up int
	if err := db.QueryRow("SELECT quota,disabled_quota,download,upload FROM account WHERE id=7").Scan(&quota, &saved, &down, &up); err != nil || quota != 0 || saved != 67108864 || down != 10 || up != 20 {
		t.Fatalf("disabled limits or usage changed (%d %d %d %d): %v", quota, saved, down, up, err)
	}
	update("0")
	checkSession(user, constant.CodeIllegalTokenError)
	identity.SessionVersion = 1
	checkSession(mint(identity), constant.CodeSuccess)
	if err := db.QueryRow("SELECT quota FROM account WHERE id=7").Scan(&quota); err != nil || quota != 67108864 {
		t.Fatalf("reactivated quota %d: %v", quota, err)
	}
}

func TestNaturalExpiryAndDateOnlyRenewal(t *testing.T) {
	dsn, cache := os.Getenv("TP_ACCOUNT_REMARK_TEST_DSN"), os.Getenv("TP_ACCOUNT_REMARK_TEST_REDIS")
	if dsn == "" || cache == "" {
		t.Skip("disposable MariaDB and Redis required")
	}
	db := disposableAPIStore(t, dsn, cache)
	if _, err := db.Exec(`INSERT INTO account(id,username,pass,quota,download,expire_time) VALUES
		(7,'expireduser','expiry-fixture',67108864,100,1),
		(8,'beforecron','pre-cron-fixture',67108864,100,1)`); err != nil {
		t.Fatal(err)
	}
	admin, err := service.GenToken(vo.AccountVo{Id: 1, Username: "sysadmin", RoleId: 1, Roles: []string{"sysadmin"}})
	if err != nil {
		t.Fatal(err)
	}
	renew := func(body string) {
		t.Helper()
		response := remarkResponse(t, remarkRequest(t, UpdateAccountById, "POST", "/account/updateAccountById", body, admin))
		data, ok := response["data"].(map[string]interface{})
		if !ok || data["quota"] != float64(0) || data["expireTime"] != float64(4078656000000) {
			t.Fatalf("renewal result: %v", response)
		}
	}
	renew(`{"id":8,"username":"beforecron","roleId":3,"expireTime":4078656000000,"deleted":0}`)
	service.CronScanAccounts()
	var deleted, total int
	if err := db.QueryRow("SELECT deleted,quota FROM account WHERE id=7").Scan(&deleted, &total); err != nil || deleted != 0 || total != 0 {
		t.Fatalf("expiry state %d %d: %v", deleted, total, err)
	}
	account, info, _, err := service.SubscribeURI("expiry-fixture", "", "v2ray")
	if err != nil || account == nil || !strings.Contains(info, "total=0; expire=0") {
		t.Fatalf("expired subscription: %v %v", info, err)
	}
	renew(`{"id":7,"username":"expireduser","roleId":3,"expireTime":4078656000000,"deleted":0}`)
	_, info, _, err = service.SubscribeURI("expiry-fixture", "", "v2ray")
	if err != nil || !strings.Contains(info, "total=0; expire=4078656000") {
		t.Fatalf("renewed subscription: %v %v", info, err)
	}
}
