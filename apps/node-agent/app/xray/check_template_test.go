//go:build linux && (amd64 || arm64)

package xray

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"trojan-panel-core/core"
)

func TestInstalledKernelTemplateCheck(t *testing.T) {
	binary := os.Getenv("TP_TEST_XRAY")
	if binary == "" {
		t.Skip("set TP_TEST_XRAY to an installed native Xray binary")
	}
	runtimeDir := t.TempDir()
	t.Setenv("TP_KERNEL_RUNTIME", runtimeDir)
	current := filepath.Join(runtimeDir, "xray", "current")
	if err := os.MkdirAll(current, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(binary, filepath.Join(current, "xray")); err != nil {
		t.Fatal(err)
	}
	oldCert := core.Config.CertConfig
	t.Cleanup(func() { core.Config.CertConfig = oldCert })
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	identity := x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour)}
	der, err := x509.CreateCertificate(rand.Reader, &identity, &identity, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	material := func(name, kind string, der []byte) string {
		path := filepath.Join(runtimeDir, name)
		if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: kind, Bytes: der}), 0600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	cert := material("node.crt", "CERTIFICATE", der)
	private := material("node.key", "EC PRIVATE KEY", keyDER)
	core.Config.CertConfig.CrtPath, core.Config.CertConfig.KeyPath = cert, private
	tlsTemplate := func(path string) string {
		certificate, _ := json.Marshal(map[string]string{"certificateFile": path, "keyFile": private})
		return `{"inbounds":[{"protocol":"socks","port":42001,"settings":{},"streamSettings":{"security":"tls","tlsSettings":{"certificates":[` + string(certificate) + `]}}}]}`
	}
	for _, tc := range []struct {
		name, input string
		valid       bool
	}{
		{"valid", `{"outbounds":[{"protocol":"freedom"}]}`, true},
		{"type", `{"log":"warning"}`, false},
		{"protocol", `{"outbounds":[{"protocol":"made-up-protocol"}]}`, false},
		{"node_tls", tlsTemplate(cert), true},
		{"missing_tls", tlsTemplate(filepath.Join(runtimeDir, "missing.crt")), false},
		{"other_local_tls", tlsTemplate(material("other.crt", "CERTIFICATE", der)), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := CheckTemplates(context.Background(), []string{tc.input})
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v err=%v", tc.valid, err)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := CheckTemplates(ctx, []string{"{}"}); err == nil {
		t.Fatal("accepted canceled check")
	}
}

func TestCheckOutputBound(t *testing.T) {
	var b checkOutput
	input := strings.Repeat("x", 16384)
	if n, err := b.Write([]byte(input)); err != nil || n != len(input) || len(b.String()) != 4096 {
		t.Fatal(n, err, len(b.String()))
	}
}
