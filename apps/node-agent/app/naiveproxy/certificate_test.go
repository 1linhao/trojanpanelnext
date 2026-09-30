package naiveproxy

import (
	"crypto/sha256"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestCertificateProbeReadsCertificateActuallyServed(t *testing.T) {
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(old) })
	if err = os.MkdirAll("bin/naiveproxy/config", 0700); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()
	_, publicPort, _ := net.SplitHostPort(strings.TrimPrefix(server.URL, "https://"))
	config := fmt.Sprintf(`{"apps":{"http":{"servers":{"srv0":{"listen":[":%s"],"tls_connection_policies":[{"match":{"sni":["node.test"]}}]}}}}}`, publicPort)
	if err = os.WriteFile("bin/naiveproxy/config/config-65434.json", []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := servedCertificateFingerprint(65434)
	if err != nil {
		t.Fatal(err)
	}
	if got != sha256.Sum256(server.Certificate().Raw) {
		t.Fatal("probe returned the wrong certificate")
	}
}

func TestCertificateRestartSnapshotsLiveUsersAndKeepsConfigOnError(t *testing.T) {
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(old) })
	if err = os.MkdirAll("bin/naiveproxy/config", 0700); err != nil {
		t.Fatal(err)
	}
	content := `{"apps":{"http":{"users":["runtime-user"]}}}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/config/" {
			t.Errorf("wrong snapshot path %s", r.URL.Path)
		}
		fmt.Fprint(w, content)
	}))
	defer server.Close()
	_, portText, _ := net.SplitHostPort(strings.TrimPrefix(server.URL, "http://"))
	port, _ := strconv.Atoi(portText)
	path := fmt.Sprintf("bin/naiveproxy/config/config-%d.json", port)
	if err = snapshotLiveConfig(uint(port)); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != content {
		t.Fatal("live users were not saved")
	}
	stat, _ := os.Stat(path)
	if stat.Mode().Perm() != 0600 {
		t.Fatal("credentials saved with unsafe permissions")
	}
	content = "invalid-json"
	if err = snapshotLiveConfig(uint(port)); err == nil {
		t.Fatal("invalid snapshot accepted")
	}
	got, _ = os.ReadFile(path)
	if string(got) == content {
		t.Fatal("last valid config overwritten")
	}
}
