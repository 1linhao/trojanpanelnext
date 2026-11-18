package api

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"trojan-panel/dao/redis"
	"trojan-panel/model/constant"
	"trojan-panel/model/vo"
	"trojan-panel/service"
)

func TestClientRoutingPublishAndSubscribe(t *testing.T) {
	dsn, redisAddr := os.Getenv("TP_CLIENT_ROUTING_TEST_DSN"), os.Getenv("TP_CLIENT_ROUTING_TEST_REDIS")
	if dsn == "" || redisAddr == "" {
		t.Skip("requires disposable MariaDB and Redis instances")
	}
	db := disposableAPIStore(t, dsn, redisAddr)
	const pass = "subscription-secret"
	for _, sql := range []string{
		`INSERT INTO account (id,username,pass,role_id,expire_time) VALUES (3,'subscriber','subscription-secret',3,4078656000000)`,
		`INSERT INTO node_server (id,name) VALUES (7,'server')`,
		`INSERT INTO node_xray (id,protocol,stream_settings) VALUES (6,'trojan','{"network":"tcp","security":"tls"}')`,
		`INSERT INTO node (id,node_server_id,node_type_id,node_sub_id,name,domain,port,client_types) VALUES
		(10,7,1,6,'Shared','visible.example',443,'clash-meta,sing-box,v2ray,shadowrocket'),
		(11,7,1,6,'Internal','hidden.example',442,'clash-meta,sing-box,v2ray,shadowrocket'),
		(12,7,1,6,'Shared','sing.example',444,'sing-box')`,
		`INSERT INTO account_node_policy (account_id) VALUES (3)`, `INSERT INTO account_node_grant VALUES (3,10),(3,12)`,
	} {
		if _, err := db.Exec(sql); err != nil {
			t.Fatal(err)
		}
	}
	system, err := json.Marshal(vo.SystemVo{ClashRule: `rules: ["MATCH,DIRECT"]`, SingBoxTun: "{}", SingBoxOutbound: "{}", XrayTemplate: "{}"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := redis.Client.String.Set("trojan-panel:system", system).Result(); err != nil {
		t.Fatal(err)
	}
	token := func(id uint, role string) string {
		tok, err := service.GenToken(vo.AccountVo{Id: id, RoleId: id, Username: role, Roles: []string{role}})
		if err != nil {
			t.Fatal(err)
		}
		return tok
	}
	sys, user := token(1, "sysadmin"), token(3, "user")
	call := func(handler gin.HandlerFunc, method, path, body, token string) map[string]interface{} {
		return remarkResponse(t, remarkRequest(t, handler, method, path, body, token))
	}
	ok := func(handler gin.HandlerFunc, method, path, body string) interface{} {
		r := call(handler, method, path, body, sys)
		if r["type"] != "success" {
			t.Fatal(r)
		}
		return r["data"]
	}
	sub := func(client, template string) string {
		writer := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(writer)
		ctx.Request = httptest.NewRequest("GET", "/subscribe?client="+client+"&template="+template, nil)
		ctx.Params = gin.Params{{Key: "token", Value: base64.RawURLEncoding.EncodeToString([]byte(pass))}}
		Subscribe(ctx)
		if writer.Header().Get("subscription-userinfo") == "" {
			t.Fatal(writer.Body.String())
		}
		return writer.Body.String()
	}
	const profile = `{"name":"office","base":{"mixedPort":7890,"tunMtu":1500,"logLevel":"info"},"traffic":{
	"groups":[{"id":"g1","name":"Office","serverId":7,"nodeIds":[10,11,12],"autoTest":true}],
	"rules":[{"kind":"domain","value":"office.test","target":"g1"}],"default":"PROXY","empty":"REJECT"},
	"dns":{"default":"1.1.1.1","rules":[]}}`
	for _, h := range []gin.HandlerFunc{ClientRouting, PublishClientRouting, ChangeClientRouting(false), ChangeClientRouting(true)} {
		if r := call(h, "POST", "/routing", "{}", user); r["message"] != constant.ForbiddenError {
			t.Fatal(r)
		}
	}
	origClash, origTun := sub("clash-meta", "default"), sub("sing-box", "tun")
	origOutbound, origURI := sub("sing-box", "outbound"), sub("v2ray", "default")
	catalog := ok(ClientRouting, "GET", "/routing", "").(map[string]interface{})
	if len(catalog["nodes"].([]interface{})) != 3 || len(catalog["releases"].([]interface{})) != 0 {
		t.Fatal(catalog)
	}
	first := int(ok(PublishClientRouting, "POST", "/publish", profile).(float64))
	body := fmt.Sprintf(`{"id":%d}`, first)
	if call(ChangeClientRouting(true), "POST", "/delete", body, sys)["type"] != "error" {
		t.Fatal("deleted active release")
	}
	source := ok(ClientRouting, "GET", fmt.Sprintf("/routing?id=%d", first), "").(map[string]interface{})
	if source["name"] != "office" {
		t.Fatal(source)
	}
	clash, tun := sub("clash-meta", "default"), sub("sing-box", "tun")
	if !strings.Contains(clash, "DOMAIN,office.test,组 · Office") || !strings.Contains(clash, "url-test") || strings.Contains(clash, "sing.example") || strings.Contains(clash, "hidden.example") || !strings.Contains(clash, pass) {
		t.Fatal("incorrect Clash rules, members or credentials")
	}
	if !strings.Contains(tun, "urltest") || !strings.Contains(tun, "节点 10 · Shared") || !strings.Contains(tun, "节点 12 · Shared") || strings.Contains(tun, "hidden.example") || !strings.Contains(tun, pass) {
		t.Fatal("incorrect sing-box groups or credentials")
	}
	if sub("sing-box", "outbound") != origOutbound || sub("v2ray", "default") != origURI {
		t.Fatal("changed outbound or URI profile")
	}
	if r := call(PublishClientRouting, "POST", "/publish", strings.Replace(profile, `"serverId":7`, `"serverId":8`, 1), sys); r["type"] != "error" {
		t.Fatal("accepted cross-server group")
	}
	if _, err := db.Exec("DELETE FROM account_node_grant WHERE account_id=3"); err != nil {
		t.Fatal(err)
	}
	if empty := sub("clash-meta", "default"); !strings.Contains(empty, "DOMAIN,office.test,REJECT") || strings.Contains(empty, pass) {
		t.Fatal("revoked account received proxy or incorrect fallback")
	}
	second := int(ok(PublishClientRouting, "POST", "/publish", strings.Replace(profile, "office.test", "second.test", 1)).(float64))
	if !strings.Contains(sub("sing-box", "tun"), "second.test") {
		t.Fatal("new snapshot not active")
	}
	ok(ChangeClientRouting(false), "POST", "/activate", body)
	if !strings.Contains(sub("sing-box", "tun"), "office.test") {
		t.Fatal("restore did not take effect")
	}
	ok(ChangeClientRouting(true), "POST", "/delete", fmt.Sprintf(`{"id":%d}`, second))
	ok(ChangeClientRouting(false), "POST", "/activate", `{"id":0}`)
	if _, err := db.Exec("INSERT INTO account_node_grant VALUES (3,10),(3,12)"); err != nil {
		t.Fatal(err)
	}
	if sub("clash-meta", "default") != origClash || sub("sing-box", "tun") != origTun {
		t.Fatal("disabling did not restore original subscriptions")
	}
}
