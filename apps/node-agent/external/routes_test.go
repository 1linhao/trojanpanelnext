package external

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"trojan-panel-core/model/constant"
)

// writeKernelConfig creates one kernel configuration file below basePath.
func writeKernelConfig(t *testing.T, basePath, relativePath, content string) {
	t.Helper()
	path := filepath.Join(basePath, relativePath)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("create kernel config dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write kernel config %s: %v", relativePath, err)
	}
}

func TestBuildManifestListsEveryKernelInbound(t *testing.T) {
	basePath := t.TempDir()

	// VLESS over TCP with TLS terminates TLS in the kernel and falls back to the
	// camouflage site on port 80.
	writeKernelConfig(t, basePath, "bin/xray/config/config-40001-vless.json", `{
  "inbounds": [
    {
      "listen": "127.0.0.1",
      "port": 40001,
      "protocol": "dokodemo-door",
      "settings": {"address": "127.0.0.1"},
      "tag": "api"
    },
    {
      "listen": "0.0.0.0",
      "port": 10001,
      "protocol": "vless",
      "settings": {"fallbacks": [{"dest": 80, "xver": 0}]},
      "streamSettings": {
        "network": "tcp",
        "security": "tls",
        "tlsSettings": {"serverName": "node.example.com"}
      },
      "tag": "vless"
    }
  ]
}`)

	// Shadowsocks over WebSocket without TLS remains a direct kernel listener;
	// the manifest only describes it for observation.
	writeKernelConfig(t, basePath, "bin/xray/config/config-40002-shadowsocks.json", `{
  "inbounds": [
    {
      "listen": "0.0.0.0",
      "port": 10002,
      "protocol": "shadowsocks",
      "settings": {},
      "streamSettings": {
        "network": "ws",
        "security": "none",
        "wsSettings": {"path": "/trojan-panel-websocket-path"}
      }
    }
  ]
}`)

	// NaiveProxy terminates TLS inside the kernel and serves the camouflage site.
	writeKernelConfig(t, basePath, "bin/naiveproxy/config/config-40003.json", `{
  "apps": {
    "http": {
      "servers": {
        "srv0": {
          "listen": [":10003"],
          "routes": []
        }
      }
    }
  }
}`)

	// Hysteria2 is QUIC only and remains a direct UDP listener.
	writeKernelConfig(t, basePath, "bin/hysteria2/config/config-40004.json", `{
  "listen": ":10004",
  "tls": {"cert": "/tpdata/trojan-panel-core/cert/fullchain.pem"}
}`)

	manifest, err := buildManifest(basePath, "node.example.com",
		"/tpdata/trojan-panel-core/cert/fullchain.pem",
		"/tpdata/trojan-panel-core/cert/privkey.pem")
	if err != nil {
		t.Fatalf("build manifest: %v", err)
	}

	if manifest.SchemaVersion != manifestVersion {
		t.Fatalf("schema version = %d, want %d", manifest.SchemaVersion, manifestVersion)
	}
	if manifest.APIPortOffset != 30000 {
		t.Fatalf("api port offset = %d, want 30000", manifest.APIPortOffset)
	}
	if manifest.Certificates.KeyPath != "/tpdata/trojan-panel-core/cert/privkey.pem" {
		t.Fatalf("key path = %q", manifest.Certificates.KeyPath)
	}
	if len(manifest.Routes) != 4 {
		t.Fatalf("routes = %d, want 4: %+v", len(manifest.Routes), manifest.Routes)
	}

	// Routes are sorted by port so that the manifest is stable between runs.
	wantPorts := []uint{10001, 10002, 10003, 10004}
	for index, want := range wantPorts {
		if manifest.Routes[index].Port != want {
			t.Fatalf("route %d port = %d, want %d", index, manifest.Routes[index].Port, want)
		}
	}

	vless := manifest.Routes[0]
	switch {
	case vless.Kernel != "xray":
		t.Errorf("vless kernel = %q", vless.Kernel)
	case vless.Protocol != "vless":
		t.Errorf("vless protocol = %q", vless.Protocol)
	case vless.APIPort != 40001:
		t.Errorf("vless api port = %d", vless.APIPort)
	case vless.Network != "tcp" || vless.Security != "tls":
		t.Errorf("vless transport = %s/%s", vless.Network, vless.Security)
	case vless.SNI != "node.example.com":
		t.Errorf("vless sni = %q", vless.SNI)
	case vless.FallbackPort != 80:
		t.Errorf("vless fallback port = %d", vless.FallbackPort)
	case !vless.TLSManagedByKernel:
		t.Error("vless must keep TLS in the kernel")
	case !vless.ExternalFallbackRequired:
		t.Error("vless tcp+tls must serve the camouflage site on fallback")
	}

	websocket := manifest.Routes[1]
	switch {
	case websocket.Network != "ws":
		t.Errorf("websocket network = %q", websocket.Network)
	case websocket.WSPath != "/trojan-panel-websocket-path":
		t.Errorf("websocket path = %q", websocket.WSPath)
	case websocket.Security != "none":
		t.Errorf("websocket security = %q", websocket.Security)
	case websocket.SNI != "node.example.com":
		t.Errorf("websocket sni = %q", websocket.SNI)
	case websocket.ExternalFallbackRequired:
		t.Error("plain websocket must not claim the camouflage site")
	}

	if manifest.Routes[2].ExternalFallbackRequired || !manifest.Routes[2].TLSManagedByKernel {
		t.Errorf("naiveproxy = %+v", manifest.Routes[2])
	}
	if manifest.Routes[3].Network != "udp" {
		t.Errorf("hysteria2 network = %q", manifest.Routes[3].Network)
	}
}

// A malformed kernel configuration must be skipped without losing the others.
func TestBuildManifestSkipsBrokenConfigurations(t *testing.T) {
	basePath := t.TempDir()
	writeKernelConfig(t, basePath, "bin/xray/config/config-40001-vless.json", `{"inbounds": [`)
	writeKernelConfig(t, basePath, "bin/xray/config/notes.txt", `not a configuration`)
	writeKernelConfig(t, basePath, "bin/hysteria2/config/config-40002.json", `{
  "listen": "0.0.0.0:10002"
}`)

	manifest, err := buildManifest(basePath, "", "", "")
	if err != nil {
		t.Fatalf("build manifest: %v", err)
	}
	if len(manifest.Routes) != 1 {
		t.Fatalf("routes = %d, want 1: %+v", len(manifest.Routes), manifest.Routes)
	}
	if manifest.Routes[0].Port != 10002 {
		t.Fatalf("hysteria2 port = %d, want 10002", manifest.Routes[0].Port)
	}
}

func TestBuildManifestWithoutKernels(t *testing.T) {
	manifest, err := buildManifest(t.TempDir(), "", "", "")
	if err != nil {
		t.Fatalf("build manifest: %v", err)
	}
	if len(manifest.Routes) != 0 {
		t.Fatalf("routes = %d, want 0", len(manifest.Routes))
	}
	if manifest.Routes == nil {
		t.Fatal("routes must be an empty array, not null")
	}
}

// TP_EXTERNAL_DIR moves the contract, which is what the container image sets to
// keep the manifest next to the installer managed directory.
func TestRoutesPathHonoursEnvironment(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TP_EXTERNAL_DIR", dir)
	if got := RoutesPath(); got != filepath.Join(dir, "routes.json") {
		t.Fatalf("RoutesPath() = %q, want %q", got, filepath.Join(dir, "routes.json"))
	}
	t.Setenv("TP_EXTERNAL_DIR", "")
	if got := RoutesPath(); got != constant.ExternalRoutesFile {
		t.Fatalf("RoutesPath() = %q, want %q", got, constant.ExternalRoutesFile)
	}
}

func TestPortFromListenAcceptsContainerFormats(t *testing.T) {
	cases := map[string]uint{
		":443":         443,
		"0.0.0.0:443":  443,
		"[::]:8443":    8443,
		"127.0.0.1:80": 80,
		"":             0,
		":":            0,
		":notaport":    0,
	}
	for listen, want := range cases {
		if got := portFromListen(listen); got != want {
			t.Errorf("portFromListen(%q) = %d, want %d", listen, got, want)
		}
	}
}

// The written file is the contract EntryController parses, so it must be valid
// JSON with the documented fields and must never be half written.
func TestWriteRoutesFileReplacesAtomically(t *testing.T) {
	basePath := t.TempDir()
	writeKernelConfig(t, basePath, "bin/xray/config/config-40001-vless.json", `{
  "inbounds": [
    {
      "listen": "0.0.0.0",
      "port": 10001,
      "protocol": "vless",
      "settings": {"fallbacks": [{"dest": 80}]},
      "streamSettings": {
        "network": "tcp",
        "security": "tls",
        "tlsSettings": {"serverName": "node.example.com"}
      }
    }
  ]
}`)

	manifest, err := buildManifest(basePath, "node.example.com", "crt.pem", "key.pem")
	if err != nil {
		t.Fatalf("build manifest: %v", err)
	}

	path := filepath.Join(t.TempDir(), "nested", "routes.json")
	if err := writeRoutesFile(path, manifest); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if matches, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".atomic-write.*.tmp")); err != nil || len(matches) != 0 {
		t.Errorf("temporary file was left behind: %v, matches=%v", err, matches)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var decoded Manifest
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("manifest is not valid JSON: %v", err)
	}
	if decoded.SchemaVersion != manifestVersion {
		t.Errorf("schema version = %d, want %d", decoded.SchemaVersion, manifestVersion)
	}
	if len(decoded.Routes) != 1 {
		t.Fatalf("routes = %d, want 1", len(decoded.Routes))
	}
	if decoded.Routes[0].SNI != "node.example.com" {
		t.Errorf("sni = %q", decoded.Routes[0].SNI)
	}
	if decoded.Routes[0].FallbackPort != 80 {
		t.Errorf("fallback port = %d", decoded.Routes[0].FallbackPort)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat manifest: %v", err)
	}
	if info.Mode().Perm() != 0644 {
		t.Errorf("manifest mode = %v, want 0644", info.Mode().Perm())
	}
}
