package kernelconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

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

func TestSyncKernelCertificatesUpdatesEveryKernelFormat(t *testing.T) {
	basePath := t.TempDir()
	writeKernelConfig(t, basePath, "bin/xray/config/config-40001-vless.json", `{
  "inbounds": [{"protocol":"vless","streamSettings":{"security":"tls","tlsSettings":{"serverName":"node.example.com","certificates":[{"certificateFile":"/old/cert.crt","keyFile":"/old/cert.key"}]}}}],
  "outbounds": [{"protocol":"freedom"}]
}`)
	writeKernelConfig(t, basePath, "bin/naiveproxy/config/config-40002.json", `{
  "apps":{"tls":{"certificates":{"load_files":[{"certificate":"/old/cert.crt","key":"/old/cert.key"}]}}}
}`)
	writeKernelConfig(t, basePath, "bin/hysteria2/config/config-40003.json", `{
  "listen":":10003","tls":{"cert":"/old/cert.crt","key":"/old/cert.key"}
}`)

	crtPath := "/tpdata/trojan-panel-core/cert/fullchain.pem"
	keyPath := "/tpdata/trojan-panel-core/cert/privkey.pem"
	if err := syncKernelCertificates(basePath, crtPath, keyPath); err != nil {
		t.Fatalf("sync certificates: %v", err)
	}

	assertJSONPathString(t, filepath.Join(basePath, "bin/xray/config/config-40001-vless.json"),
		[]any{"inbounds", 0, "streamSettings", "tlsSettings", "certificates", 0, "certificateFile"}, crtPath)
	assertJSONPathString(t, filepath.Join(basePath, "bin/naiveproxy/config/config-40002.json"),
		[]any{"apps", "tls", "certificates", "load_files", 0, "key"}, keyPath)
	assertJSONPathString(t, filepath.Join(basePath, "bin/hysteria2/config/config-40003.json"),
		[]any{"tls", "cert"}, crtPath)
	assertJSONPathString(t, filepath.Join(basePath, "bin/xray/config/config-40001-vless.json"),
		[]any{"outbounds", 0, "protocol"}, "freedom")

	if err := syncKernelCertificates(basePath, crtPath, keyPath); err != nil {
		t.Fatalf("idempotent sync: %v", err)
	}
}

func TestSyncKernelCertificatesDoesNotAddCertificatesToPlainXray(t *testing.T) {
	basePath := t.TempDir()
	path := "bin/xray/config/config-40001-shadowsocks.json"
	want := `{"inbounds":[{"streamSettings":{"security":"none","network":"ws"}}]}`
	writeKernelConfig(t, basePath, path, want)
	if err := syncKernelCertificates(basePath, "new.crt", "new.key"); err != nil {
		t.Fatalf("sync certificates: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(basePath, path))
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != want {
		t.Fatalf("plain config was rewritten: %s", raw)
	}
}

func assertJSONPathString(t *testing.T, path string, parts []any, want string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var current any
	if err := json.Unmarshal(raw, &current); err != nil {
		t.Fatal(err)
	}
	for _, part := range parts {
		switch part := part.(type) {
		case string:
			current = current.(map[string]any)[part]
		case int:
			current = current.([]any)[part]
		}
	}
	if got, _ := current.(string); got != want {
		t.Fatalf("%s path %v = %q, want %q", path, parts, got, want)
	}
}
