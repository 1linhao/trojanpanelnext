package hostagent

import (
	"bytes"
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
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func identity(t *testing.T, parent *x509.Certificate, parentKey *ecdsa.PrivateKey, client bool) ([]byte, []byte, *x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(time.Now().UnixNano()), Subject: pkix.Name{CommonName: "host-removal-test"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, BasicConstraintsValid: true, DNSNames: []string{"localhost"}}
	if parent == nil {
		template.IsCA = true
		template.KeyUsage |= x509.KeyUsageCertSign
		parent = template
		parentKey = key
	} else if client {
		template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	} else {
		template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
	}
	der, err := x509.CreateCertificate(rand.Reader, template, parent, &key.PublicKey, parentKey)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), parsed, key
}

func setup(t *testing.T) (*Server, *http.Client) {
	t.Helper()
	directory := t.TempDir()
	live := filepath.Join(directory, "live")
	if err := os.Mkdir(live, 0700); err != nil {
		t.Fatal(err)
	}
	ca, _, root, key := identity(t, nil, nil, false)
	cert, private, _, _ := identity(t, root, key, false)
	client, clientKey, _, _ := identity(t, root, key, true)
	config := Config{NodeID: 7, Port: 8101, Certificate: filepath.Join(live, "server.crt"), Key: filepath.Join(live, "server.key"), ClientCA: filepath.Join(live, "client-ca.crt")}
	for path, data := range map[string][]byte{config.Certificate: cert, config.Key: private, config.ClientCA: ca} {
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	server, err := New(config, directory)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	server.workerContext = ctx
	server.callbackInterval = 5 * time.Millisecond
	roots := x509.NewCertPool()
	roots.AppendCertsFromPEM(ca)
	pair, err := tls.X509KeyPair(client, clientKey)
	if err != nil {
		t.Fatal(err)
	}
	return server, &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: "localhost", Certificates: []tls.Certificate{pair}}}}
}

func serve(t *testing.T, s *Server) *httptest.Server {
	t.Helper()
	server := httptest.NewUnstartedServer(s.Handler())
	server.TLS = s.TLSConfig()
	server.StartTLS()
	t.Cleanup(server.Close)
	return server
}
func post(t *testing.T, client *http.Client, url string, request Request) (*Result, int) {
	t.Helper()
	body, _ := json.Marshal(request)
	response, err := client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return nil, response.StatusCode
	}
	var result Result
	if err = json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	return &result, response.StatusCode
}

func TestRemovalAcknowledgesActualCompletionAndCanResume(t *testing.T) {
	s, client := setup(t)
	var executions atomic.Int32
	s.Execute = func(_ context.Context, purge bool) error {
		if !purge {
			return errors.New("expected purge")
		}
		executions.Add(1)
		return os.RemoveAll(filepath.Dir(s.Config.ClientCA))
	}
	server := serve(t, s)
	if _, status := post(t, client, server.URL+"/remove", Request{NodeID: 8, Purge: true}); status != 400 {
		t.Fatal(status)
	}
	first, status := post(t, client, server.URL+"/remove", Request{NodeID: 7, Purge: true})
	if status != 200 || !first.Success || executions.Load() != 1 {
		t.Fatalf("%#v %d", first, status)
	}
	client.CloseIdleConnections()
	resumed, err := New(s.Config, s.Directory)
	if err != nil {
		t.Fatal(err)
	}
	resumed.Execute = func(context.Context, bool) error { t.Fatal("completed job executed twice"); return nil }
	newServer := serve(t, resumed)
	second, status := post(t, client, newServer.URL+"/remove", Request{NodeID: 7, Purge: true})
	if status != 200 || second.Receipt != first.Receipt {
		t.Fatalf("saved result lost: %#v", second)
	}
	if _, status = post(t, client, newServer.URL+"/remove", Request{NodeID: 7}); status != 409 {
		t.Fatal(status)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	resumed.workerContext = ctx
	callback := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	defer callback.Close()
	resumed.callbackClient.Transport = callback.Client().Transport
	var finalized atomic.Int32
	resumed.Finalize = func() error { finalized.Add(1); return nil }
	if _, status = post(t, client, newServer.URL+"/finalize", Request{NodeID: 7, Purge: true, Receipt: "wrong", CallbackURL: callback.URL + callbackPath}); status != 409 || finalized.Load() != 0 {
		t.Fatal(status, finalized.Load())
	}
	if _, status = post(t, client, newServer.URL+"/finalize", Request{NodeID: 7, Purge: true, Receipt: first.Receipt, CallbackURL: callback.URL + callbackPath}); status != 200 {
		t.Fatal(status)
	}
	waitFor(t, func() bool { return finalized.Load() == 1 })
}

func waitFor(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("timed out waiting for removal finalization")
}

func TestFinalizationSurvivesRestartAndLostCallbackResponse(t *testing.T) {
	for _, purge := range []bool{false, true} {
		t.Run(fmt.Sprintf("purge=%t", purge), func(t *testing.T) {
			s, client := setup(t)
			s.Execute = func(context.Context, bool) error { return nil }
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			s.workerContext = ctx
			var callbackAttempts, cleanupAttempts atomic.Int32
			var allowCallback, committed atomic.Bool
			callback := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				callbackAttempts.Add(1)
				var payload struct {
					NodeID  uint   `json:"nodeId"`
					Receipt string `json:"receipt"`
				}
				if r.URL.Path != callbackPath || json.NewDecoder(r.Body).Decode(&payload) != nil || payload.NodeID != 7 || !validReceipt(payload.Receipt) {
					t.Error("invalid completion callback")
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				if !allowCallback.Load() {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				if committed.CompareAndSwap(false, true) {
					// Model a committed Web DELETE followed by a lost HTTP response.
					connection, _, err := w.(http.Hijacker).Hijack()
					if err != nil {
						t.Error(err)
						return
					}
					_ = connection.Close()
					return
				}
				w.WriteHeader(http.StatusNoContent)
			}))
			defer callback.Close()
			s.callbackClient.Transport = callback.Client().Transport
			s.Finalize = func() error { cleanupAttempts.Add(1); return nil }
			server := serve(t, s)
			result, _ := post(t, client, server.URL+"/remove", Request{NodeID: 7, Purge: purge})
			request := Request{NodeID: 7, Purge: purge, Receipt: result.Receipt, CallbackURL: callback.URL + callbackPath}
			if _, status := post(t, client, server.URL+"/finalize", request); status != 200 {
				t.Fatal(status)
			}
			waitFor(t, func() bool { return callbackAttempts.Load() > 0 })
			if cleanupAttempts.Load() != 0 {
				t.Fatal("cleaned up before Web committed its callback")
			}
			info, err := os.Stat(filepath.Join(s.Directory, "finalize.json"))
			if err != nil || info.Mode().Perm() != 0600 {
				t.Fatal("callback job was not saved privately", err)
			}
			cancel()
			waitFor(t, func() bool { s.mu.Lock(); defer s.mu.Unlock(); return !s.workerRunning })
			server.Close()
			resumed, err := New(s.Config, s.Directory)
			if err != nil {
				t.Fatal(err)
			}
			resumed.workerContext, cancel = context.WithCancel(context.Background())
			defer cancel()
			resumed.callbackInterval = 5 * time.Millisecond
			resumed.callbackClient.Transport = callback.Client().Transport
			resumed.Finalize = func() error {
				if cleanupAttempts.Add(1) == 1 {
					return errors.New("test cleanup failure")
				}
				return nil
			}
			allowCallback.Store(true)
			resumed.mu.Lock()
			resumed.startFinalizerLocked()
			resumed.mu.Unlock()
			waitFor(t, func() bool { resumed.mu.Lock(); defer resumed.mu.Unlock(); return resumed.finalized })
			if cleanupAttempts.Load() != 2 || !committed.Load() {
				t.Fatal("callback or cleanup was not retried")
			}
		})
	}
}

func TestCallbackOnlyAcceptsVerifiedHTTPSAndNeverRedirects(t *testing.T) {
	s, client := setup(t)
	s.Execute = func(context.Context, bool) error { return nil }
	server := serve(t, s)
	result, _ := post(t, client, server.URL+"/remove", Request{NodeID: 7})
	for _, raw := range []string{"http://web.example.com" + callbackPath, "https://web.example.com/other", "https://user@web.example.com" + callbackPath, "https://web.example.com" + callbackPath + "?receipt=secret", "https://web.example.com" + callbackPath + "#fragment"} {
		if _, status := post(t, client, server.URL+"/finalize", Request{NodeID: 7, Receipt: result.Receipt, CallbackURL: raw}); status != 409 {
			t.Fatal(raw, status)
		}
	}
	var forwarded atomic.Int32
	target := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { forwarded.Add(1); w.WriteHeader(204) }))
	defer target.Close()
	redirect := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+callbackPath, http.StatusTemporaryRedirect)
	}))
	defer redirect.Close()
	s.callbackClient.Transport = redirect.Client().Transport
	if err := s.sendCallback(context.Background(), Request{NodeID: 7, Receipt: result.Receipt, CallbackURL: redirect.URL + callbackPath}); err == nil || forwarded.Load() != 0 {
		t.Fatal("callback followed a redirect")
	}
	s.callbackClient.Transport = nil
	if err := s.sendCallback(context.Background(), Request{NodeID: 7, Receipt: result.Receipt, CallbackURL: target.URL + callbackPath}); err == nil {
		t.Fatal("untrusted callback certificate accepted")
	}
}

func TestMaintenanceCleanupKeepsRecoveryBinaryUntilSecretsAreGone(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "private")
	library := filepath.Join(t.TempDir(), "library")
	unit := filepath.Join(t.TempDir(), "host.service")
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(library, 0700); err != nil {
		t.Fatal(err)
	}
	for path, data := range map[string]string{filepath.Join(directory, "server.key"): "secret", filepath.Join(library, "tp-host-agent"): "binary", filepath.Join(library, "uninstall.sh"): "script", unit: "unit"} {
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	failed := false
	systemctl := func(args ...string) error {
		if _, err := os.Stat(directory); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("systemd cleanup began before deleting credentials")
		}
		if _, err := os.Stat(filepath.Join(library, "tp-host-agent")); err != nil {
			t.Fatal("recovery executable was removed too soon")
		}
		if !failed {
			failed = true
			return errors.New("test systemd failure")
		}
		return nil
	}
	if err := cleanupMaintenance(directory, library, unit, systemctl); err == nil {
		t.Fatal("cleanup error ignored")
	}
	if _, err := os.Stat(filepath.Join(library, cleanupMarker)); err != nil {
		t.Fatal("cleanup recovery marker lost")
	}
	if err := cleanupMaintenance(directory, library, unit, systemctl); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{directory, library, unit} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("maintenance artifact retained", path)
		}
	}
}

func TestPartialPurgeRetainsAuthenticatedRetryChannel(t *testing.T) {
	s, client := setup(t)
	s.Execute = func(context.Context, bool) error {
		if err := os.Remove(s.Config.Certificate); err != nil {
			return err
		}
		return errors.New("partial purge fixture")
	}
	server := serve(t, s)
	if _, status := post(t, client, server.URL+"/remove", Request{NodeID: 7, Purge: true}); status != 500 {
		t.Fatal(status)
	}
	if _, err := os.Stat(s.Config.ClientCA); err != nil {
		t.Fatal("live CA should still exist", err)
	}
	client.CloseIdleConnections()
	resumed, err := New(s.Config, s.Directory)
	if err != nil {
		t.Fatal(err)
	}
	resumed.Execute = func(context.Context, bool) error { return nil }
	restarted := serve(t, resumed)
	result, status := post(t, client, restarted.URL+"/remove", Request{NodeID: 7, Purge: true})
	if status != 200 || !result.Success {
		t.Fatal("authenticated retry after a partial purge failed", status)
	}
}

func TestFailureAndUnauthenticatedRequestsDoNotReportSuccess(t *testing.T) {
	s, client := setup(t)
	var executed atomic.Int32
	s.Execute = func(context.Context, bool) error { executed.Add(1); return errors.New("test uninstall failed") }
	server := serve(t, s)
	if _, status := post(t, client, server.URL+"/remove", Request{NodeID: 7}); status != 500 {
		t.Fatal(status)
	}
	if _, err := os.Stat(filepath.Join(s.Directory, "result.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failure saved as success: %v", err)
	}
	request := httptest.NewRequest("POST", "/remove", bytes.NewBufferString(`{"nodeId":7}`))
	recorder := httptest.NewRecorder()
	s.Handler().ServeHTTP(recorder, request)
	if recorder.Code != 400 || executed.Load() != 1 {
		t.Fatal(recorder.Code, executed.Load())
	}
	transport := client.Transport.(*http.Transport).Clone()
	transport.TLSClientConfig.Certificates = nil
	unauthenticated := &http.Client{Transport: transport}
	response, err := unauthenticated.Post(server.URL+"/remove", "application/json", bytes.NewBufferString(`{"nodeId":7}`))
	if err == nil {
		response.Body.Close()
		t.Fatal("missing client certificate accepted")
	}
	if executed.Load() != 1 {
		t.Fatal("unauthenticated execution")
	}
}
