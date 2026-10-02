package service

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/go-sql-driver/mysql"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/anypb"
	"gopkg.in/yaml.v3"
	"trojan-panel/core"
	"trojan-panel/dao"
	"trojan-panel/dao/redis"
	"trojan-panel/model/constant"
	"trojan-panel/model/dto"
	"trojan-panel/model/vo"
	"trojan-panel/service/clientcompat"
)

// This suite requires its own disposable MariaDB instance: the production
// initializer uses trojan_panel_db. It creates that database exclusively and
// refuses to run if it already exists. Redis must also be a test instance.
func TestNodeForwardingServiceIntegration(t *testing.T) {
	dsn, redisAddress := os.Getenv("TP_NODE_PORT_TEST_DSN"), os.Getenv("TP_NODE_PORT_TEST_REDIS")
	if dsn == "" || redisAddress == "" {
		t.Skip("TP_NODE_PORT_TEST_DSN and TP_NODE_PORT_TEST_REDIS are not set")
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
		t.Fatalf("need an unused disposable database instance: %v", err)
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
	config.DBName = "trojan_panel_db"
	testDB, err := sql.Open("mysql", config.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { testDB.Close() })
	system, err := json.Marshal(vo.SystemVo{ClashRule: "rules: []", SingBoxTun: "{}", SingBoxOutbound: "{}", XrayTemplate: "{}"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := redis.Client.String.Set("trojan-panel:system", system).Result(); err != nil {
		t.Fatal(err)
	}
	live := &forwardingNodeServer{}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := grpc.NewServer()
	core.RegisterApiNodeServiceServer(server, live)
	core.RegisterApiStateServiceServer(server, live)
	go server.Serve(listener)
	t.Cleanup(server.Stop)
	rpcPort := listener.Addr().(*net.TCPAddr).Port
	forwardingExec(t, testDB, `INSERT INTO node_server (id,name,ip,grpc_port,grpc_tls_mode,grpc_tls_server_name) VALUES (7,'fixture','127.0.0.1',?,'legacy','node.example.test')`, rpcPort)
	forwardingExec(t, testDB, `UPDATE account SET pass='port-export-fixture',username='fixture-user' WHERE id=1`)

	t.Run("crud_control_uses_actual_listener", func(t *testing.T) {
		create := forwardingCreateDTO("managed", constant.Xray, "trojan", 445, uintPtr(443))
		if err := CreateNode("fixture-token", create); err != nil {
			t.Fatal(err)
		}
		id := forwardingID(t, testDB, "managed")
		detail, err := SelectNodeById(&id)
		if err != nil || detail.Port != 445 || detail.ExternalPort != 443 {
			t.Fatalf("detail=%+v err=%v", detail, err)
		}
		data, _ := json.Marshal(detail)
		if !strings.Contains(string(data), `"externalPort":443`) {
			t.Fatalf("detail JSON does not include forwarding: %s", data)
		}
		update := forwardingUpdateDTO(create, id, detail.NodeSubId)
		for _, tc := range []struct {
			external *uint
			want     uint
		}{{nil, 443}, {uintPtr(0), 0}, {uintPtr(65535), 65535}} {
			update.ExternalPort = tc.external
			if err := UpdateNodeById("fixture-token", &update); err != nil {
				t.Fatal(err)
			}
			detail, err = SelectNodeById(&id)
			if err != nil || detail.Port != 445 || detail.ExternalPort != tc.want {
				t.Fatalf("updated detail=%+v err=%v", detail, err)
			}
		}
		// A type change follows a different persistence branch; omission must
		// preserve the existing forwarding configuration there too.
		update.NodeTypeId, update.ExternalPort = uintPtr(constant.Hysteria2), nil
		if err := UpdateNodeById("fixture-token", &update); err != nil {
			t.Fatal(err)
		}
		detail, err = SelectNodeById(&id)
		if err != nil || detail.ExternalPort != 65535 || detail.Port != 445 {
			t.Fatalf("type change lost forwarding: %+v %v", detail, err)
		}
		update.NodeSubId = uintPtr(detail.NodeSubId)
		update.Port, update.ExternalPort = uintPtr(446), uintPtr(443)
		if err := UpdateNodeById("fixture-token", &update); err != nil {
			t.Fatal(err)
		}
		token, err := GenToken(vo.AccountVo{Id: 1, Username: "fixture-user", Roles: []string{"sysadmin"}})
		if err != nil {
			t.Fatal(err)
		}
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		ctx.Request = httptest.NewRequest("GET", "/nodes", nil)
		ctx.Request.Header.Set("Authorization", "Bearer "+token)
		page, err := SelectNodePage(nil, nil, uintPtr(1), uintPtr(20), ctx)
		if err != nil || len(page.Nodes) != 1 || page.Nodes[0].Port != 446 || page.Nodes[0].ExternalPort != 443 || page.Nodes[0].Status != 1 {
			t.Fatalf("list=%+v err=%v", page, err)
		}
		if err := DeleteNodeById("fixture-token", &id); err != nil {
			t.Fatal(err)
		}
		if _, err := dao.SelectNodeById(&id); err == nil {
			t.Fatal("deleted node still exists")
		}
		want := []string{"add:445", "remove:445", "add:445", "remove:445", "add:445", "remove:445", "add:445", "remove:445", "add:445", "remove:445", "add:446", "state:446", "remove:446"}
		if got := live.recorded(); !reflect.DeepEqual(got, want) {
			t.Fatalf("control addressed external port or omitted operation:\n got %v\nwant %v", got, want)
		}
	})

	t.Run("all_client_exports_use_public_ports", func(t *testing.T) {
		for index, protocol := range []string{"vless", "vmess", "trojan", "shadowsocks", "socks"} {
			create := forwardingCreateDTO(protocol, constant.Xray, protocol, uint(451+index), uintPtr(443))
			if err := CreateNode("fixture-token", create); err != nil {
				t.Fatal(err)
			}
		}
		for _, tc := range []struct {
			name         string
			kind, actual uint
			external     *uint
		}{{"naive", constant.NaiveProxy, 461, uintPtr(443)}, {"hy2", constant.Hysteria2, 462, uintPtr(65535)}, {"single", constant.Xray, 463, nil}} {
			create := forwardingCreateDTO(tc.name, tc.kind, "trojan", tc.actual, tc.external)
			if tc.kind == constant.NaiveProxy {
				clients := []string{"sing-box", "v2ray", "shadowrocket"}
				create.Clients = &clients
			}
			if err := CreateNode("fixture-token", create); err != nil {
				t.Fatal(err)
			}
		}
		want := map[string]uint{"vless": 443, "vmess": 443, "trojan": 443, "shadowsocks": 443, "socks": 443, "naive": 443, "hy2": 65535, "single": 463}
		for name, expected := range want {
			id := forwardingID(t, testDB, name)
			uri, _, err := NodeURL(uintPtr(1), stringPtr("fixture-user"), &id)
			if err != nil {
				t.Fatal(err)
			}
			if got := forwardingURIPort(t, uri); got != expected {
				t.Fatalf("%s URI port=%d want %d", name, got, expected)
			}
		}
		_, _, clash, _, err := SubscribeClash("port-export-fixture")
		if err != nil {
			t.Fatal(err)
		}
		var clashConfig struct {
			Proxies []struct {
				Name string `yaml:"name"`
				Port uint   `yaml:"port"`
			} `yaml:"proxies"`
		}
		if err := yaml.Unmarshal(clash, &clashConfig); err != nil {
			t.Fatal(err)
		}
		if len(clashConfig.Proxies) != 7 {
			t.Fatalf("Clash exported %d proxies", len(clashConfig.Proxies))
		}
		for _, proxy := range clashConfig.Proxies {
			if proxy.Port != want[proxy.Name] {
				t.Fatalf("Clash %s port=%d want %d", proxy.Name, proxy.Port, want[proxy.Name])
			}
		}
		for _, template := range []string{"outbound", "tun"} {
			_, _, content, err := SubscribeSingBox("port-export-fixture", template)
			if err != nil {
				t.Fatal(err)
			}
			var singbox struct {
				Outbounds []struct {
					Tag  string `json:"tag"`
					Port uint   `json:"server_port"`
				} `json:"outbounds"`
			}
			if err := json.Unmarshal(content, &singbox); err != nil {
				t.Fatal(err)
			}
			seen := 0
			for _, outbound := range singbox.Outbounds {
				if expected, ok := want[outbound.Tag]; ok {
					seen++
					if outbound.Port != expected {
						t.Fatalf("sing-box %s %s port=%d want%d", template, outbound.Tag, outbound.Port, expected)
					}
				}
			}
			if seen != 8 {
				t.Fatalf("sing-box %s exported %d nodes", template, seen)
			}
		}
		for _, client := range []struct{ name, agent string }{{"v2ray", "v2rayNG/1.10"}, {"v2ray", "v2rayN/7.0"}, {"shadowrocket", "Shadowrocket"}} {
			_, _, content, err := SubscribeURI("port-export-fixture", client.agent, client.name)
			if err != nil {
				t.Fatal(err)
			}
			plain, err := base64.StdEncoding.DecodeString(string(content))
			if err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(string(plain), "\n")
			if len(lines) != 8 {
				t.Fatalf("%s exported %d nodes", client.agent, len(lines))
			}
			for _, line := range lines {
				name := forwardingURIName(t, line)
				if got := forwardingURIPort(t, line); got != want[name] {
					t.Fatalf("%s %s port=%d want%d", client.agent, name, got, want[name])
				}
			}
		}
		// Forwarding does not replace the existing UDP hopping destinations.
		id := forwardingID(t, testDB, "hy2")
		forwardingExec(t, testDB, `UPDATE node_hysteria2 SET port_hopping='21000-21010',hop_interval=30 WHERE id=(SELECT node_sub_id FROM node WHERE id=?)`, id)
		uri, _, err := nodeURLForClient(uintPtr(1), stringPtr("fixture-user"), &id, clientcompat.V2RaySubscriptionStandard)
		if err != nil {
			t.Fatal(err)
		}
		parsed, err := url.Parse(uri)
		if err != nil || parsed.Port() != "65535" || parsed.Query().Get("mport") != "21000-21010" {
			t.Fatalf("forwarded hopping URI=%s err=%v", uri, err)
		}
		node, err := dao.SelectNodeById(&id)
		if err != nil {
			t.Fatal(err)
		}
		outbound, err := buildSingBoxOutbound(*node, "port-export-fixture", "fixture-user")
		if err != nil || !reflect.DeepEqual(outbound["server_ports"], []string{"21000:21010"}) {
			t.Fatalf("hopping destinations=%v err=%v", outbound, err)
		}
	})
}

type forwardingNodeServer struct {
	core.UnimplementedApiNodeServiceServer
	core.UnimplementedApiStateServiceServer
	mu     sync.Mutex
	events []string
}

func (server *forwardingNodeServer) record(operation string, port uint64) {
	server.mu.Lock()
	defer server.mu.Unlock()
	server.events = append(server.events, fmt.Sprintf("%s:%d", operation, port))
}
func (server *forwardingNodeServer) recorded() []string {
	server.mu.Lock()
	defer server.mu.Unlock()
	return append([]string(nil), server.events...)
}
func (server *forwardingNodeServer) AddNode(_ context.Context, request *core.NodeAddDto) (*core.Response, error) {
	server.record("add", request.Port)
	return &core.Response{Success: true}, nil
}
func (server *forwardingNodeServer) RemoveNode(_ context.Context, request *core.NodeRemoveDto) (*core.Response, error) {
	server.record("remove", request.Port)
	return &core.Response{Success: true}, nil
}
func (server *forwardingNodeServer) GetNodeState(_ context.Context, request *core.NodeStateDto) (*core.Response, error) {
	server.record("state", request.Port)
	data, err := anypb.New(&core.NodeStateVo{Status: 1})
	return &core.Response{Success: true, Data: data}, err
}

func forwardingCreateDTO(name string, kind uint, protocol string, actual uint, external *uint) dto.NodeCreateDto {
	priority, speed := 0, 100
	return dto.NodeCreateDto{NodeServerId: uintPtr(7), NodeTypeId: uintPtr(kind), Name: stringPtr(name), Domain: stringPtr("node.example.test"), Port: uintPtr(actual), ExternalPort: external, Priority: &priority,
		XrayProtocol: stringPtr(protocol), XrayFlow: stringPtr(""), XraySSMethod: stringPtr("aes-128-gcm"), XraySettings: stringPtr(`{"encryption":"none","accounts":[{"user":"fixture-user","pass":"fixture-pass"}]}`), XrayStreamSettings: stringPtr(`{"network":"tcp","security":"tls","tlsSettings":{"serverName":"node.example.test"}}`), XrayTag: stringPtr(""), XraySniffing: stringPtr("{}"), XrayAllocate: stringPtr("{}"),
		TrojanGoMuxEnable: uintPtr(0), TrojanGoWebsocketEnable: uintPtr(0), TrojanGoSsEnable: uintPtr(0), HysteriaUpMbps: &speed, HysteriaDownMbps: &speed, Hysteria2UpMbps: &speed, Hysteria2DownMbps: &speed, Hysteria2ObfsPassword: stringPtr(""), Hysteria2ServerName: stringPtr("node.example.test"), Hysteria2Insecure: uintPtr(0)}
}
func forwardingUpdateDTO(create dto.NodeCreateDto, id, subID uint) dto.NodeUpdateDto {
	data, _ := json.Marshal(create)
	var update dto.NodeUpdateDto
	_ = json.Unmarshal(data, &update)
	update.Id, update.NodeSubId = uintPtr(id), uintPtr(subID)
	return update
}
func forwardingExec(t *testing.T, db *sql.DB, query string, args ...interface{}) {
	t.Helper()
	if _, err := db.Exec(query, args...); err != nil {
		t.Fatal(err)
	}
}
func forwardingID(t *testing.T, db *sql.DB, name string) uint {
	t.Helper()
	var id uint
	if err := db.QueryRow("SELECT id FROM node WHERE name=?", name).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}
func forwardingURIProfile(t *testing.T, uri string) map[string]interface{} {
	t.Helper()
	data, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(uri, "v2rayn://hysteria2/"))
	if err != nil {
		t.Fatal(err)
	}
	var profile map[string]interface{}
	if err := json.Unmarshal(data, &profile); err != nil {
		t.Fatal(err)
	}
	return profile
}
func forwardingURIName(t *testing.T, uri string) string {
	t.Helper()
	if strings.HasPrefix(uri, "v2rayn://") {
		return forwardingURIProfile(t, uri)["Remarks"].(string)
	}
	_, fragment, ok := strings.Cut(uri, "#")
	if !ok {
		t.Fatalf("URI has no name: %s", uri)
	}
	name, err := url.PathUnescape(fragment)
	if err != nil {
		t.Fatal(err)
	}
	return name
}
func forwardingURIPort(t *testing.T, uri string) uint {
	t.Helper()
	if strings.HasPrefix(uri, "v2rayn://") {
		return uint(forwardingURIProfile(t, uri)["Port"].(float64))
	}
	base, _, _ := strings.Cut(uri, "#")
	if strings.HasPrefix(base, "ss://") || strings.HasPrefix(base, "socks://") {
		_, payload, _ := strings.Cut(base, "://")
		decoded, err := base64.StdEncoding.DecodeString(payload)
		if err != nil {
			t.Fatal(err)
		}
		_, address, _ := strings.Cut(string(decoded), "@")
		_, port, err := net.SplitHostPort(address)
		if err != nil {
			t.Fatal(err)
		}
		value, err := strconv.ParseUint(port, 10, 16)
		if err != nil {
			t.Fatal(err)
		}
		return uint(value)
	}
	parsed, err := url.Parse(uri)
	if err != nil {
		t.Fatal(err)
	}
	value, err := strconv.ParseUint(parsed.Port(), 10, 16)
	if err != nil {
		t.Fatalf("URI port parse: %s: %v", uri, err)
	}
	return uint(value)
}
