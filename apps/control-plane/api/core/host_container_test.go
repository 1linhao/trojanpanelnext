package core

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestHostContainerTransportRejectsUnsafeRequests(t *testing.T) {
	for _, test := range []struct {
		transport NodeTransport
		port, id  uint
		version   string
	}{
		{NodeTransport{Mode: "legacy"}, 8101, 1, "1.0.2-rc.12"},
		{NodeTransport{Mode: "mtls"}, 8101, 1, "1.0.2-rc.12"},
		{NodeTransport{Mode: "mtls", ServerName: "node.example.test"}, 65536, 1, "1.0.2-rc.12"},
		{NodeTransport{Mode: "mtls", ServerName: "node.example.test"}, 8101, 0, "1.0.2-rc.12"},
		{NodeTransport{Mode: "mtls", ServerName: "node.example.test"}, 8101, 1, "latest;whoami"},
	} {
		if _, err := UpdateHostContainer(context.Background(), "127.0.0.1", test.port, test.transport, test.id, test.version); err == nil {
			t.Fatal("unsafe container update reached transport")
		}
	}
}

func containerMTLSFixture(t *testing.T, handler http.HandlerFunc) (string, uint) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "container test"}, DNSNames: []string{"node.example.test"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	certificate, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(certPEM)
	previous, managed := Config.GrpcConfig, ManagedClientCertificate
	t.Cleanup(func() { Config.GrpcConfig = previous; ManagedClientCertificate = managed })
	dir := t.TempDir()
	for name, data := range map[string][]byte{"client.crt": certPEM, "client.key": keyPEM, "ca.crt": certPEM} {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	Config.GrpcConfig.ClientCertPath = filepath.Join(dir, "client.crt")
	Config.GrpcConfig.ClientKeyPath = filepath.Join(dir, "client.key")
	Config.GrpcConfig.ServerCAPath = filepath.Join(dir, "ca.crt")
	ManagedClientCertificate = nil
	server := httptest.NewUnstartedServer(handler)
	server.TLS = &tls.Config{Certificates: []tls.Certificate{certificate}, ClientCAs: pool, ClientAuth: tls.RequireAndVerifyClientCert, MinVersion: tls.VersionTLS12}
	server.StartTLS()
	t.Cleanup(server.Close)
	host, portText, err := net.SplitHostPort(server.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatal(err)
	}
	return host, uint(port)
}

func TestHostContainerMTLSAndBoundRequest(t *testing.T) {
	host, port := containerMTLSFixture(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || len(r.TLS.PeerCertificates) != 1 {
			t.Error("container request is not authenticated POST")
		}
		var body map[string]any
		if json.NewDecoder(r.Body).Decode(&body) != nil {
			t.Error("invalid body")
		}
		if body["nodeId"] != float64(7) {
			t.Error("wrong registered node identity")
		}
		switch r.URL.Path {
		case "/container/inventory":
			if len(body) != 1 {
				t.Error("inventory sent unintended fields")
			}
			w.Write([]byte(`{"nodeId":7,"currentVersion":"1.0.2-rc.11","image":"ghcr.io/1linhao/trojanpanelnext-node-agent:1.0.2-rc.11","updateSupported":true,"job":null}`))
		case "/container/update":
			if len(body) != 2 || body["version"] != "1.0.2-rc.12" {
				t.Error("unbound container target")
			}
			w.WriteHeader(http.StatusAccepted)
			w.Write([]byte(`{"id":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","fromVersion":"1.0.2-rc.11","targetVersion":"1.0.2-rc.12","status":"queued","error":"","startedAt":"","finishedAt":""}`))
		default:
			t.Error("wrong endpoint")
		}
	})
	transport := NodeTransport{Mode: "mtls", ServerName: "node.example.test"}
	inventory, err := GetHostContainerInventory(context.Background(), host, port, transport, 7)
	if err != nil || inventory.NodeID != 7 {
		t.Fatalf("inventory: %+v %v", inventory, err)
	}
	job, err := UpdateHostContainer(context.Background(), host, port, transport, 7, "1.0.2-rc.12")
	if err != nil || job.ID != strings.Repeat("a", 64) {
		t.Fatalf("update: %+v %v", job, err)
	}
	transport.ServerName = "wrong.example.test"
	if _, err := GetHostContainerInventory(context.Background(), host, port, transport, 7); err == nil {
		t.Fatal("wrong server identity accepted")
	}
}

func TestHostContainerReleaseAndJobContract(t *testing.T) {
	for _, version := range []string{"1.0", "1.0.2", "1.0.2-rc.12"} {
		if !ValidContainerVersion(version) {
			t.Errorf("supported release rejected: %s", version)
		}
	}
	for _, version := range []string{"1.0.2-beta.1", "v1.0.2", "01.0.2", "1.00.2", "1.0.2-rc.0", "latest", strings.Repeat("1", 65) + ".0.0"} {
		if ValidContainerVersion(version) {
			t.Errorf("unsupported release accepted: %s", version)
		}
	}
	job := HostContainerJob{ID: strings.Repeat("a", 64), FromVersion: "1.0", TargetVersion: "1.0.2-rc.12", Status: "queued"}
	if err := ValidateHostContainerJob(&job); err != nil {
		t.Errorf("old release job rejected: %v", err)
	}
	job.FromVersion = "1.0.2-rc.11"
	for _, id := range []string{"job-7", strings.Repeat("A", 64), strings.Repeat("a", 63)} {
		job.ID = id
		if err := ValidateHostContainerJob(&job); err == nil {
			t.Errorf("non-host receipt accepted: %s", id)
		}
	}
	job.ID, job.Status = strings.Repeat("a", 64), "running"
	if err := ValidateHostContainerJob(&job); err == nil {
		t.Error("running job without start accepted")
	}
	job.StartedAt = "2026-10-03T08:00:00Z"
	if err := ValidateHostContainerJob(&job); err != nil {
		t.Error(err)
	}
	job.Status = "succeeded"
	if err := ValidateHostContainerJob(&job); err == nil {
		t.Error("finished job without finish accepted")
	}
	job.FinishedAt = "not-a-time"
	if err := ValidateHostContainerJob(&job); err == nil {
		t.Error("invalid timestamp accepted")
	}
}

func TestHostContainerRejectsURLHostAliasAndNonOfficialImage(t *testing.T) {
	host, port := containerMTLSFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"nodeId":7,"currentVersion":"1.0.2","image":"ghcr.io/1linhao/trojanpanelnext-node-agent:1.0.2","updateSupported":true,"job":null}`))
	})
	transport := NodeTransport{Mode: "mtls", ServerName: "node.example.test"}
	if _, err := GetHostContainerInventory(context.Background(), "ignored@"+host, port, transport, 7); err == nil {
		t.Error("host userinfo changed registered connection identity")
	}
	host, port = containerMTLSFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"nodeId":7,"currentVersion":"1.0.2","image":"untrusted.registry/agent:1.0.2","updateSupported":true,"job":null}`))
	})
	if _, err := GetHostContainerInventory(context.Background(), host, port, transport, 7); err == nil {
		t.Error("non-official image accepted")
	}
}

func TestHostContainerResponseBoundsCompatibilityAndDeadline(t *testing.T) {
	for _, test := range []struct {
		name, body  string
		status      int
		unsupported bool
	}{
		{"old_host", "", 404, true}, {"redirect", "", 307, false},
		{"oversized", strings.Repeat("x", hostContainerResponseLimit+1), 200, false},
		{"wrong_id", `{"nodeId":8,"currentVersion":"1.0.2","image":"image","updateSupported":true,"job":null}`, 200, false},
		{"unknown_field", `{"nodeId":7,"currentVersion":"1.0.2","image":"image","updateSupported":true,"job":null,"secret":"unexpected"}`, 200, false},
		{"trailing", `{"nodeId":7,"currentVersion":"1.0.2","image":"image","updateSupported":true,"job":null}{}`, 200, false},
		{"bad_job", `{"nodeId":7,"currentVersion":"1.0.2","image":"image","updateSupported":true,"job":{"id":"x","fromVersion":"1.0.1","targetVersion":"1.0.2","status":"running","startedAt":""}}`, 200, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			host, port := containerMTLSFixture(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(test.status); w.Write([]byte(test.body)) })
			_, err := GetHostContainerInventory(context.Background(), host, port, NodeTransport{Mode: "mtls", ServerName: "node.example.test"}, 7)
			if err == nil || errors.Is(err, ErrHostContainerUnsupported) != test.unsupported {
				t.Fatalf("response accepted or compatibility error lost: %v", err)
			}
		})
	}
	t.Run("caller_deadline", func(t *testing.T) {
		host, port := containerMTLSFixture(t, func(w http.ResponseWriter, r *http.Request) { time.Sleep(100 * time.Millisecond) })
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		start := time.Now()
		_, err := GetHostContainerInventory(ctx, host, port, NodeTransport{Mode: "mtls", ServerName: "node.example.test"}, 7)
		if err == nil || time.Since(start) > 150*time.Millisecond {
			t.Fatal("container request ignored caller deadline")
		}
	})
}

func TestHostRemovalRecognizesOnlyConfirmedContainerConflict(t *testing.T) {
	for _, scenario := range []struct {
		name, path, body string
		status           int
		confirmed        bool
	}{
		{"confirmed", "/remove", "container_update_active\n", 409, true},
		{"other_conflict", "/remove", "host_removal_pending\n", 409, false},
		{"trailing_data", "/remove", "container_update_active\nunknown", 409, false},
		{"oversized", "/remove", "container_update_active" + strings.Repeat(" ", 200), 409, false},
		{"wrong_status", "/remove", "container_update_active\n", 500, false},
		{"finalize", "/finalize", "container_update_active\n", 409, false},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			host, port := containerMTLSFixture(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(scenario.status)
				w.Write([]byte(scenario.body))
			})
			_, err := hostRemovalCall(host, port, NodeTransport{Mode: "mtls", ServerName: "node.example.test"}, scenario.path, HostRemoval{NodeID: 7})
			if err == nil || errors.Is(err, ErrHostContainerUpdateActive) != scenario.confirmed {
				t.Fatalf("unconfirmed refusal classified as safe reset: %v", err)
			}
		})
	}
}
