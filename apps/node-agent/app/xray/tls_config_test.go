package xray

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"
	"time"

	"trojan-panel-core/core"
	"trojan-panel-core/core/process"
	"trojan-panel-core/model/constant"
	"trojan-panel-core/model/dto"
)

func isolatedXrayDirectory(t *testing.T) {
	t.Helper()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previous); err != nil {
			t.Error(err)
		}
	})
	if err := os.MkdirAll(constant.XrayPath, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TP_KERNEL_RUNTIME", filepath.Join(t.TempDir(), "runtime"))
}

func TestInitXrayOmitsServerAllowInsecure(t *testing.T) {
	isolatedXrayDirectory(t)
	previous := core.Config
	core.Config = &core.AppConfig{CertConfig: core.CertConfig{CrtPath: "/cert/fullchain.pem", KeyPath: "/cert/key.pem"}}
	t.Cleanup(func() { core.Config = previous })
	for _, setting := range []string{"true", "false"} {
		t.Run(setting, func(t *testing.T) {
			input := dto.XrayConfigDto{
				ApiPort: 65431, Port: 443, Protocol: "trojan", Tag: "tls-inbound", Settings: `{ "clients": [] }`,
				StreamSettings: `{"network":"tcp","security":"tls","tlsSettings":{"allowInsecure":` + setting + `,"alpn":["h2","http/1.1"],"serverName":"node.example.test","fingerprint":"chrome"}}`,
				Template:       `{"inbounds":[],"outbounds":[{"protocol":"trojan","streamSettings":{"security":"tls","tlsSettings":{"allowInsecure":true,"serverName":"upstream.example.test"}}}]}`,
			}
			originalStreamSettings := input.StreamSettings
			if err := initXray(input); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(filepath.Join(constant.XrayPath, "config-65431-trojan.json"))
			if err != nil {
				t.Fatal(err)
			}
			var config struct {
				Inbounds []struct {
					Tag            string `json:"tag"`
					StreamSettings struct {
						TLSSettings map[string]interface{} `json:"tlsSettings"`
					} `json:"streamSettings"`
				} `json:"inbounds"`
				Outbounds json.RawMessage `json:"outbounds"`
			}
			if err := json.Unmarshal(data, &config); err != nil {
				t.Fatal(err)
			}
			if len(config.Inbounds) != 2 || config.Inbounds[1].Tag != "tls-inbound" {
				t.Fatal("generated user inbound is missing")
			}
			tls := config.Inbounds[1].StreamSettings.TLSSettings
			if _, exists := tls["allowInsecure"]; exists {
				t.Fatalf("generated server TLS still contains client-only allowInsecure=%v", tls["allowInsecure"])
			}
			if !reflect.DeepEqual(tls["certificates"], []interface{}{map[string]interface{}{"certificateFile": "/cert/fullchain.pem", "keyFile": "/cert/key.pem"}}) || !reflect.DeepEqual(tls["alpn"], []interface{}{"h2", "http/1.1"}) || tls["serverName"] != "node.example.test" || tls["fingerprint"] != "chrome" {
				t.Fatalf("server TLS certificate or other fields changed: %v", tls)
			}
			var outbounds []map[string]interface{}
			if err := json.Unmarshal(config.Outbounds, &outbounds); err != nil {
				t.Fatal(err)
			}
			upstream := outbounds[0]["streamSettings"].(map[string]interface{})["tlsSettings"].(map[string]interface{})
			if upstream["allowInsecure"] != true || upstream["serverName"] != "upstream.example.test" || input.StreamSettings != originalStreamSettings {
				t.Fatal("outbound TLS or caller stream settings changed")
			}
		})
	}
}

func TestInitXraySanitizesTemplateServerTLS(t *testing.T) {
	isolatedXrayDirectory(t)
	input := dto.XrayConfigDto{
		ApiPort: 65434, Port: 1080, Protocol: "socks", Tag: "new-inbound", Settings: `{"auth":"noauth"}`,
		StreamSettings: `{"security":"none"}`,
		Template: `{
  "inbounds":[{"listen":"0.0.0.0","port":443,"protocol":"trojan","tag":"template-inbound","settings":{"clients":[]},"streamSettings":{"network":"tcp","security":"tls","tlsSettings":{"allowInsecure":true,"certificates":[{"certificateFile":"/cert/template.pem","keyFile":"/cert/template.key"}],"alpn":["h2","http/1.1"],"minVersion":"1.2","serverName":"template.example.test"}}}],
  "outbounds":[{"protocol":"trojan","streamSettings":{"security":"tls","tlsSettings":{"allowInsecure":true,"serverName":"upstream.example.test"}}}]
}`,
	}
	if err := initXray(input); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(constant.XrayPath, "config-65434-socks.json"))
	if err != nil {
		t.Fatal(err)
	}
	config := jsonDocument(t, data).(map[string]interface{})
	inbounds := config["inbounds"].([]interface{})
	if len(inbounds) != 3 {
		t.Fatal("template, API or user inbound was lost")
	}
	template := inbounds[0].(map[string]interface{})
	if template["listen"] != "0.0.0.0" || template["port"] != json.Number("443") || template["protocol"] != "trojan" || template["tag"] != "template-inbound" {
		t.Fatal("supported template inbound fields changed")
	}
	wantStream := jsonDocument(t, []byte(`{"network":"tcp","security":"tls","tlsSettings":{"certificates":[{"certificateFile":"/cert/template.pem","keyFile":"/cert/template.key"}],"alpn":["h2","http/1.1"],"minVersion":"1.2","serverName":"template.example.test"}}`))
	if !reflect.DeepEqual(template["streamSettings"], wantStream) {
		t.Fatalf("template server TLS contains allowInsecure or lost supported TLS fields: %v", template["streamSettings"])
	}
	wantOutbound := jsonDocument(t, []byte(`[{"protocol":"trojan","streamSettings":{"security":"tls","tlsSettings":{"allowInsecure":true,"serverName":"upstream.example.test"}}}]`))
	if !reflect.DeepEqual(config["outbounds"], wantOutbound) {
		t.Fatal("template outbound certificate verification changed")
	}
}

// This helper is the external process boundary: it records exactly the config
// passed by StartXray and rejects the server-only incompatibility like Xray does.
func TestXrayCompatibilityProcess(t *testing.T) {
	if os.Getenv("TP_XRAY_COMPATIBILITY_PROCESS") != "1" {
		return
	}
	var path string
	for i, argument := range os.Args {
		if argument == "-c" && i+1 < len(os.Args) {
			path = os.Args[i+1]
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		os.Exit(24)
	}
	if err := os.WriteFile(os.Getenv("TP_XRAY_CONFIG_SEEN"), data, 0600); err != nil {
		os.Exit(24)
	}
	var config struct {
		Inbounds []struct {
			StreamSettings struct {
				TLSSettings map[string]json.RawMessage `json:"tlsSettings"`
			} `json:"streamSettings"`
		} `json:"inbounds"`
	}
	if err := json.Unmarshal(data, &config); err != nil {
		os.Exit(23)
	}
	for _, inbound := range config.Inbounds {
		if _, exists := inbound.StreamSettings.TLSSettings["allowInsecure"]; exists {
			os.Exit(23)
		}
	}
	time.Sleep(30 * time.Second)
	os.Exit(0)
}

func setupXrayCompatibilityProcess(t *testing.T, apiPort uint) string {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	seen := filepath.Join(t.TempDir(), "seen.json")
	t.Setenv("TP_XRAY_COMPATIBILITY_PROCESS", "1")
	t.Setenv("TP_XRAY_TEST_EXECUTABLE", executable)
	t.Setenv("TP_XRAY_CONFIG_SEEN", seen)
	if err := os.WriteFile(filepath.Join(constant.XrayBinPath, "xray"), []byte("#!/bin/sh\nexec \"$TP_XRAY_TEST_EXECUTABLE\" -test.run '^TestXrayCompatibilityProcess$' -- \"$@\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := process.NewXrayProcess().Stop(apiPort, false); err != nil {
			t.Error(err)
		}
	})
	return seen
}

func waitForXrayConfig(t *testing.T, path string) []byte {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil && len(data) != 0 {
			return data
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("Xray process did not receive the configuration")
	return nil
}

func jsonDocument(t *testing.T, data []byte) interface{} {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var document interface{}
	if err := decoder.Decode(&document); err != nil {
		t.Fatal(err)
	}
	return document
}

func TestInitXrayAppMigratesExistingServerTLSBeforeStarting(t *testing.T) {
	isolatedXrayDirectory(t)
	seen := setupXrayCompatibilityProcess(t, 65432)
	path := filepath.Join(constant.XrayPath, "config-65432-trojan.json")
	original := []byte(`{
  "unknownRoot":{"preciseNumber":9007199254740993},
  "inbounds":[
    {"tag":"first","unknownInbound":{"keep":true},"streamSettings":{"security":"tls","unknownStream":[1,2],"tlsSettings":{"allowInsecure":true,"certificates":[{"certificateFile":"/cert/fullchain.pem","keyFile":"/cert/key.pem","unknownCertificate":"keep"}],"alpn":["h2","http/1.1"],"minVersion":"1.2","futureTLS":{"value":9007199254740993}}}},
    {"tag":"second","streamSettings":{"security":"tls","tlsSettings":{"allowInsecure":false,"rejectUnknownSni":true}}},
    {"tag":"plain","streamSettings":{"security":"none","futureTransport":"keep"}}
  ],
  "outbounds":[{"protocol":"trojan","streamSettings":{"security":"tls","tlsSettings":{"allowInsecure":true,"serverName":"upstream.example.test","futureTLS":{"keep":true}}}}]
}`)
	expected := []byte(`{
  "unknownRoot":{"preciseNumber":9007199254740993},
  "inbounds":[
    {"tag":"first","unknownInbound":{"keep":true},"streamSettings":{"security":"tls","unknownStream":[1,2],"tlsSettings":{"certificates":[{"certificateFile":"/cert/fullchain.pem","keyFile":"/cert/key.pem","unknownCertificate":"keep"}],"alpn":["h2","http/1.1"],"minVersion":"1.2","futureTLS":{"value":9007199254740993}}}},
    {"tag":"second","streamSettings":{"security":"tls","tlsSettings":{"rejectUnknownSni":true}}},
    {"tag":"plain","streamSettings":{"security":"none","futureTransport":"keep"}}
  ],
  "outbounds":[{"protocol":"trojan","streamSettings":{"security":"tls","tlsSettings":{"allowInsecure":true,"serverName":"upstream.example.test","futureTLS":{"keep":true}}}}]
}`)
	if err := os.WriteFile(path, original, 0640); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0640); err != nil {
		t.Fatal(err)
	}
	originalInfo, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := InitXrayApp(); err != nil {
		t.Fatal(err)
	}
	started := waitForXrayConfig(t, seen)
	if !reflect.DeepEqual(jsonDocument(t, started), jsonDocument(t, expected)) {
		t.Fatalf("Xray started before a lossless server TLS migration: %s", started)
	}
	migrated, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(migrated, started) {
		t.Fatal("process and persisted configuration differ")
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0640 {
		t.Fatal("migration changed the configuration permissions")
	}
	originalOwner, migratedOwner := originalInfo.Sys().(*syscall.Stat_t), info.Sys().(*syscall.Stat_t)
	if originalOwner.Uid != migratedOwner.Uid || originalOwner.Gid != migratedOwner.Gid {
		t.Fatal("migration changed the configuration owner")
	}
	if err := InitXrayApp(); err != nil {
		t.Fatal(err)
	}
	again, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(again, migrated) {
		t.Fatal("idempotent startup changed the migrated configuration")
	}
	finalInfo, err := os.Stat(path)
	if err != nil || !finalInfo.ModTime().Equal(info.ModTime()) {
		t.Fatal("idempotent startup rewrote the migrated configuration")
	}
}

func TestInitXrayAppPreservesInvalidConfigWithoutStarting(t *testing.T) {
	for _, fixture := range []struct {
		name, content string
		symlink       bool
	}{
		{"malformed_json", `{"inbounds":[`, false},
		{"malformed_tls", `{"inbounds":[{"streamSettings":{"tlsSettings":"invalid"}}]}`, false},
		{"symlink", `{"inbounds":[{"streamSettings":{"tlsSettings":{"allowInsecure":true}}}]}`, true},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			isolatedXrayDirectory(t)
			seen := setupXrayCompatibilityProcess(t, 65433)
			path := filepath.Join(constant.XrayPath, "config-65433-trojan.json")
			contentPath := path
			if fixture.symlink {
				contentPath = filepath.Join(t.TempDir(), "target.json")
			}
			if err := os.WriteFile(contentPath, []byte(fixture.content), 0600); err != nil {
				t.Fatal(err)
			}
			if fixture.symlink {
				if err := os.Symlink(contentPath, path); err != nil {
					t.Fatal(err)
				}
			}
			if err := InitXrayApp(); err == nil {
				t.Fatal("invalid migration input did not prevent Xray startup")
			}
			data, err := os.ReadFile(contentPath)
			if err != nil || string(data) != fixture.content {
				t.Fatal("failed migration damaged the original configuration")
			}
			if _, err := os.Stat(seen); !os.IsNotExist(err) {
				t.Fatal("Xray received a configuration after migration failed")
			}
			entries, err := filepath.Glob(filepath.Join(constant.XrayPath, ".xray-tls-*"))
			if err != nil || len(entries) != 0 {
				t.Fatal("failed migration left temporary configuration files")
			}
		})
	}
}
