package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/net/proxy"
	"gopkg.in/yaml.v3"
	"trojan-panel/dao"
	"trojan-panel/model"
	"trojan-panel/model/constant"
	"trojan-panel/service"
)

// Set TP_CREDENTIAL_NODE_TEST to a compiled apps/node-agent/app test executable,
// TP_CREDENTIAL_XRAY to an installed kernel, and supply disposable DB/Valkey endpoints.
func TestAccountSOCKSProxy(t *testing.T) {
	fixture := os.Getenv("TP_CREDENTIAL_NODE_TEST")
	if fixture == "" {
		t.Skip("requires Node fixture, Xray, MariaDB and Valkey")
	}
	db := disposableAPIStore(t, os.Getenv("TP_CREDENTIAL_TEST_DSN"), os.Getenv("TP_CREDENTIAL_REDIS"))
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(oldDir) })
	if err = os.MkdirAll(constant.TemplatePath, 0700); err != nil {
		t.Fatal(err)
	}
	for path, content := range map[string]string{constant.ClashTemplateFilePath: "rules: []", constant.XrayTemplateFilePath: "{}", constant.SingBoxTunTemplateFilePath: "{}", constant.SingBoxOutboundTemplateFilePath: "{}"} {
		if err = os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	bindPort := 21493 // the Node API uses the service port + 30000
	for _, port := range []int{bindPort, bindPort + 30000} {
		l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err != nil {
			t.Fatal(err)
		}
		l.Close()
	}
	for _, query := range []string{"INSERT INTO node_server(id,name,ip,grpc_port) VALUES(7,'fixture','127.0.0.1',1)", `INSERT INTO node_xray(id,protocol,settings) VALUES(21,'socks','{"auth":"password","accounts":[{"user":"fixture","pass":"fixture-shared"}]}')`, fmt.Sprintf("INSERT INTO node(id,node_server_id,node_type_id,node_sub_id,name,domain,port) VALUES(21,7,1,21,'socks','127.0.0.1',%d)", bindPort)} {
		if _, err := db.Exec(query); err != nil {
			t.Fatal(err)
		}
	}
	password, newPassword := "fixture-initial", "fixture-updated"
	quota, expiry := -1, time.Now().Add(time.Hour).UnixMilli()
	makeAccount := func(name string) *model.Account {
		a := model.Account{Username: &name, Pass: &password}
		if err := dao.CreateAccount(&a); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec("UPDATE account SET quota=?,expire_time=? WHERE username=?", quota, expiry, name); err != nil {
			t.Fatal(err)
		}
		record, err := dao.SelectAccountByUsername(&name)
		if err != nil {
			t.Fatal(err)
		}
		return record
	}
	a, b := makeAccount("first"), makeAccount("second")
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Second)
	defer cancel()
	node := exec.CommandContext(ctx, fixture, "-test.run=^TestCredentialNodeFixture$")
	node.Dir = t.TempDir()
	node.Env = append(os.Environ(), "TP_CREDENTIAL_NODE_FIXTURE=1", fmt.Sprintf("TP_CREDENTIAL_SOCKS_PORT=%d", bindPort))
	stdout, err := node.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdin, err := node.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	var log bytes.Buffer
	node.Stderr = &log
	if err = node.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		stdin.Close()
		if err := node.Wait(); err != nil {
			t.Errorf("fixture failed: %v %s", err, log.String())
		}
	}()
	reader := bufio.NewScanner(stdout)
	wait := func(marker string) {
		t.Helper()
		for reader.Scan() {
			if reader.Text() == marker {
				return
			}
		}
		t.Fatal("Node fixture ended before", marker)
	}
	wait("CREDENTIAL_FIXTURE_READY")
	syncAccounts := func() { t.Helper(); fmt.Fprintln(stdin, "sync"); wait("CREDENTIAL_FIXTURE_SYNCED") }
	api := gin.New()
	api.GET("/api/auth/subscribe/:token", Subscribe)
	server := httptest.NewServer(api)
	defer server.Close()
	download := func(a *model.Account, client, template string) []byte {
		t.Helper()
		r, err := http.Get(server.URL + buildSubscribePath(*a.Pass, client, template))
		if err != nil {
			t.Fatal(err)
		}
		defer r.Body.Close()
		if r.Header.Get("subscription-userinfo") == "" {
			t.Fatal("subscription rejected", client)
		}
		data, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	type credential struct {
		Username string `json:"username" yaml:"username"`
		Password string `json:"password" yaml:"password"`
	}
	linkCred := func(link string) credential {
		t.Helper()
		base, _, _ := strings.Cut(link, "#")
		raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(base, "socks://"))
		if err != nil {
			t.Fatal("invalid SOCKS URI")
		}
		u, err := url.Parse("socks://" + string(raw))
		if err != nil || u.User == nil {
			t.Fatal("missing SOCKS credentials")
		}
		pass, _ := u.User.Password()
		return credential{u.User.Username(), pass}
	}
	exported := func(a *model.Account) credential {
		t.Helper()
		id := uint(21)
		link, _, err := service.NodeURL(a.Id, a.Username, &id)
		if err != nil {
			t.Fatal(err)
		}
		c := linkCred(link)
		for _, client := range []string{"v2ray", "shadowrocket"} {
			raw, err := base64.StdEncoding.DecodeString(string(download(a, client, "default")))
			if err != nil || linkCred(string(raw)) != c {
				t.Fatal(client, "differs from share link")
			}
		}
		var clash struct {
			Proxies []credential `yaml:"proxies"`
		}
		if err := yaml.Unmarshal(download(a, "clash-meta", "default"), &clash); err != nil || len(clash.Proxies) != 1 || clash.Proxies[0] != c {
			t.Fatal("Clash credentials differ", err)
		}
		var sb struct {
			Outbounds []credential `json:"outbounds"`
		}
		if err := json.Unmarshal(download(a, "sing-box", "outbound"), &sb); err != nil || len(sb.Outbounds) < 1 {
			t.Fatal("sing-box missing outbounds", err)
		}
		for _, entry := range sb.Outbounds {
			if entry == c {
				return c
			}
		}
		t.Fatal("sing-box credentials differ")
		return credential{}
	}
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, "fixture-content") }))
	defer origin.Close()
	connects := func(c credential) bool {
		d, err := proxy.SOCKS5("tcp", fmt.Sprintf("127.0.0.1:%d", bindPort), &proxy.Auth{User: c.Username, Password: c.Password}, &net.Dialer{Timeout: time.Second})
		if err != nil {
			t.Fatal(err)
		}
		transport := &http.Transport{DialContext: d.(proxy.ContextDialer).DialContext, DisableKeepAlives: true}
		defer transport.CloseIdleConnections()
		client := &http.Client{Transport: transport, Timeout: 2 * time.Second}
		r, err := client.Get(origin.URL)
		if err != nil {
			return false
		}
		defer r.Body.Close()
		data, err := io.ReadAll(r.Body)
		return err == nil && string(data) == "fixture-content"
	}
	legacy := credential{"fixture", "fixture-shared"}
	if connects(legacy) {
		t.Fatal("legacy SOCKS listener accepted before first sync")
	}
	ac, bc := exported(a), exported(b)
	origB := download(b, "v2ray", "default")
	syncAccounts()
	if ac == bc || !connects(ac) || !connects(bc) || connects(legacy) {
		t.Fatalf("initial SOCKS authorization differs from exports: %s", log.String())
	}
	if err := service.UpdateAccountPass("fixture-token", &password, &newPassword, *a.Id); err != nil {
		t.Fatal(err)
	}
	updated, err := dao.SelectAccountByUsername(a.Username)
	if err != nil {
		t.Fatal(err)
	}
	syncAccounts()
	updatedCredential := exported(updated)
	if connects(ac) || !connects(updatedCredential) || !connects(bc) || !bytes.Equal(origB, download(b, "v2ray", "default")) {
		t.Fatal("A rotation affected B or retained A's old password")
	}
	for _, step := range []string{"INSERT INTO account_node_policy(account_id) VALUES(?)", "INSERT INTO account_node_grant(account_id,node_id) VALUES(?,21)", "UPDATE account SET deleted=1 WHERE id=?", "UPDATE account SET deleted=0,expire_time=1 WHERE id=?"} {
		if _, err = db.Exec(step, *a.Id); err != nil {
			t.Fatal(err)
		}
		syncAccounts()
		want := strings.HasPrefix(step, "INSERT INTO account_node_grant")
		if connects(updatedCredential) != want || !connects(bc) {
			t.Fatal("unexpected SOCKS authorization after policy change")
		}
	}
}
