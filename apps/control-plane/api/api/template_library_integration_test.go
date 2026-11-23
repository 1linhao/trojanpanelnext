package api

import (
	"context"
	"encoding/json"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"net"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"trojan-panel/core"

	"trojan-panel/dao/redis"
	"trojan-panel/model"
	"trojan-panel/model/constant"
	"trojan-panel/model/vo"
	"trojan-panel/service"
)

func TestTemplateLibrarySettingsUpgrade(t *testing.T) {
	dsn, addr := os.Getenv("TP_CLIENT_ROUTING_TEST_DSN"), os.Getenv("TP_CLIENT_ROUTING_TEST_REDIS")
	if dsn == "" || addr == "" {
		t.Skip("requires disposable MariaDB and Redis instances")
	}
	db := disposableAPIStore(t, dsn, addr)
	if err := redis.Client.Key.RetryDel("trojan-panel:system"); err != nil {
		t.Fatal(err)
	}
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldDir); err != nil {
			t.Error(err)
		}
	})
	if err := os.MkdirAll(constant.TemplatePath, 0700); err != nil {
		t.Fatal(err)
	}
	for path, content := range map[string]string{
		constant.ClashTemplateFilePath: "rules: [legacy-rule]", constant.XrayTemplateFilePath: `{"log":{"loglevel":"debug"}}`, constant.SingBoxTunTemplateFilePath: "{}", constant.SingBoxOutboundTemplateFilePath: "{}",
	} {
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec("UPDATE `system` SET template_config=? WHERE id=1", `{"clashTemplateName":"旧规则","xrayTemplateName":"旧节点","systemName":"Panel"}`); err != nil {
		t.Fatal(err)
	}
	name := constant.SystemName
	get := func() vo.SystemVo {
		t.Helper()
		v, err := service.SelectSystemByName(&name)
		if err != nil {
			t.Fatal(err)
		}
		if v.ClashTemplateName != "旧规则" || v.XrayTemplateName != "旧节点" || v.ClashRule != "rules: [legacy-rule]" || v.XrayTemplate != `{"log":{"loglevel":"debug"}}` {
			t.Fatal("upgrade or library edit modified legacy templates", v)
		}
		return v
	}
	old := get()
	token, err := service.GenToken(vo.AccountVo{Id: 1, Username: "fixture-admin", Roles: []string{"sysadmin", "admin"}})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(old)
	if err != nil {
		t.Fatal(err)
	}
	var request map[string]interface{}
	if err := json.Unmarshal(payload, &request); err != nil {
		t.Fatal(err)
	}
	// Only the named catalog is edited. File-backed templates stay at their legacy locations.
	for _, k := range []string{"clashRule", "xrayTemplate", "singBoxTun", "singBoxOutbound"} {
		delete(request, k)
	}
	request["emailPort"] = 25
	save := func(want string) string {
		t.Helper()
		b, err := json.Marshal(request)
		if err != nil {
			t.Fatal(err)
		}
		got := remarkResponse(t, remarkRequest(t, UpdateSystemById, "POST", "/system/updateSystemById", string(b), token))
		if got["type"] != want {
			t.Fatal(got)
		}
		message, _ := got["message"].(string)
		return message
	}
	lib := model.TemplateLibrary{Clash: []model.NamedTemplate{{ID: "one", Name: "第一", Content: "rules: []"}}, Xray: []model.NamedTemplate{{ID: "server", Name: "节点", Content: "{}"}}, ActiveXray: "server"}
	request["templateLibrary"] = lib
	save("success")
	if !reflect.DeepEqual(get().TemplateLibrary, lib) {
		t.Fatal("failed persistence")
	}
	delete(request, "templateLibrary")
	delete(request, "clashTemplateName")
	delete(request, "xrayTemplateName")
	save("success")
	if !reflect.DeepEqual(get().TemplateLibrary, lib) {
		t.Fatal("old settings request erased the library")
	}
	lib.Clash[0].Name = "第二"
	request["templateLibrary"] = lib
	save("success")
	if get().TemplateLibrary.Clash[0].ID != "one" || get().TemplateLibrary.Clash[0].Name != "第二" {
		t.Fatal("rename changed ID or did not persist")
	}
	lib.Xray = nil
	request["templateLibrary"] = lib
	save("error")
	if len(get().TemplateLibrary.Xray) != 1 {
		t.Fatal("partially applied invalid library")
	}
	request["templateLibrary"] = model.TemplateLibrary{ActiveXray: "default"}
	save("success")
	if len(get().TemplateLibrary.Clash) != 0 || len(get().TemplateLibrary.Xray) != 0 {
		t.Fatal("deletion did not persist")
	}
	// Every failed save leaves both default files and the active selection intact.
	lib = model.TemplateLibrary{ActiveXray: "default", Xray: []model.NamedTemplate{{ID: "office", Name: "Office", Content: `{"log":"warning"}`}}}
	request["templateLibrary"] = lib
	request["clashRule"] = "rules: [should-never-be-written]"
	save("error")
	get()
	delete(request, "clashRule")
	const custom = `{"outbounds":[{"protocol":"freedom","tag":"office"}]}`
	lib.Xray[0].Content = custom
	request["templateLibrary"] = lib
	if msg := save("error"); !strings.Contains(msg, "online Node") {
		t.Fatal(msg)
	}
	// Register an earlier Node Agent (no check RPC).
	listen := func(s *grpc.Server) int {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		go s.Serve(listener)
		t.Cleanup(s.Stop)
		return listener.Addr().(*net.TCPAddr).Port
	}
	register := func(id, port int) {
		if _, err := db.Exec("INSERT INTO node_server (id,name,ip,grpc_port,grpc_tls_mode) VALUES (?,?,'127.0.0.1',?,'legacy')", id, strconv.Itoa(id), port); err != nil {
			t.Fatal(err)
		}
	}
	first := grpc.NewServer()
	fake := &templateCheckRecorder{token: token}
	core.RegisterApiKernelServiceServer(first, fake)
	port := listen(first)
	if live := os.Getenv("TP_TEST_XRAY_GRPC_PORT"); live != "" {
		port, err = strconv.Atoi(live)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = redis.Client.String.Set("trojan-panel:token:fixture-admin", token, 60).Result(); err != nil {
			t.Fatal(err)
		}
	}
	register(5, port)
	register(6, listen(grpc.NewServer()))
	if msg := save("error"); !strings.Contains(msg, "#6") {
		t.Fatal(msg)
	}
	if _, err := db.Exec("DELETE FROM node_server WHERE id=6"); err != nil {
		t.Fatal(err)
	}
	save("success")
	lib.ActiveXray = "office"
	request["templateLibrary"] = lib
	save("success")
	before := get().TemplateLibrary
	// Invalid protocol reaches a kernel check and must not partially apply the default.
	request["xrayTemplate"] = `{"outbounds":[{"protocol":"bogus-protocol"}]}`
	request["clashRule"] = "rules: [should-never-be-written]"
	if msg := save("error"); !strings.Contains(msg, "#5") {
		t.Fatal(msg)
	}
	if !reflect.DeepEqual(before, get().TemplateLibrary) {
		t.Fatal("rejected edit changed catalog")
	}
	delete(request, "clashRule")
	delete(request, "xrayTemplate")
	// A legacy DB may contain an already-active broken template. Recover by
	// changing only the active pointer. Reactivating the broken entry must fail.
	lib.Xray[0].Content = `{"log":"warning"}`
	raw, _ := json.Marshal(lib)
	if _, err := db.Exec("UPDATE `system` SET template_library=? WHERE id=1", string(raw)); err != nil {
		t.Fatal(err)
	}
	if err := redis.Client.Key.RetryDel("trojan-panel:system"); err != nil {
		t.Fatal(err)
	}
	lib.ActiveXray = "default"
	request["templateLibrary"] = lib
	save("success")
	lib.ActiveXray = "office"
	request["templateLibrary"] = lib
	save("error")
	if get().TemplateLibrary.ActiveXray != "default" {
		t.Fatal("broken history was reactivated")
	}
}

type templateCheckRecorder struct {
	core.UnimplementedApiKernelServiceServer
	token string
}

func (s *templateCheckRecorder) CheckXrayTemplates(ctx context.Context, in *core.XrayTemplatesRequest) (*core.Response, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	tokens := md.Get("token")
	if len(tokens) != 1 || tokens[0] != s.token {
		return &core.Response{Msg: "missing admin token"}, nil
	}
	for _, content := range in.Contents {
		if strings.Contains(content, "bogus-protocol") {
			return &core.Response{Msg: "unsupported protocol"}, nil
		}
	}
	return &core.Response{Success: true}, nil
}
