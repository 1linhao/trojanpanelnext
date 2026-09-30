// Package hostagent implements the host-side, mTLS-authenticated removal
// endpoint. It accepts only removal modes, never command text or file paths.
package hostagent

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

const Directory = "/etc/trojanpanelnext-host"
const Library = "/usr/local/lib/trojanpanelnext-host"
const Unit = "/etc/systemd/system/trojanpanelnext-host.service"
const cleanupMarker = "cleanup-ready"
const callbackPath = "/api/nodeServer/completeHostRemoval"

type Config struct {
	NodeID         uint              `json:"nodeId"`
	Port           uint              `json:"port"`
	Certificate    string            `json:"certificate"`
	Key            string            `json:"key"`
	ClientCA       string            `json:"clientCA"`
	OriginalConfig string            `json:"originalConfig"`
	Environment    map[string]string `json:"environment"`
}

type Request struct {
	NodeID      uint   `json:"nodeId"`
	Purge       bool   `json:"purge"`
	Receipt     string `json:"receipt,omitempty"`
	CallbackURL string `json:"callbackUrl,omitempty"`
}

type Result struct {
	NodeID  uint   `json:"nodeId"`
	Purge   bool   `json:"purge"`
	Receipt string `json:"receipt"`
	Success bool   `json:"success"`
}

type Server struct {
	Config           Config
	Directory        string
	Execute          func(context.Context, bool) error
	Finalize         func() error
	mu               sync.Mutex
	result           *Result
	finalized        bool
	callback         *Request
	workerRunning    bool
	callbackClient   *http.Client
	callbackInterval time.Duration
	workerContext    context.Context
}

func New(config Config, directory string) (*Server, error) {
	if config.NodeID == 0 || config.Port == 0 || config.Port > 65535 {
		return nil, errors.New("host removal requires a registered node ID and a valid port")
	}
	s := &Server{Config: config, Directory: directory, callbackInterval: 30 * time.Second, workerContext: context.Background()}
	s.callbackClient = &http.Client{Timeout: 15 * time.Second, Transport: &http.Transport{Proxy: nil, TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}, TLSHandshakeTimeout: 10 * time.Second}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	data, err := os.ReadFile(filepath.Join(directory, "result.json"))
	if err == nil {
		var result Result
		if err = json.Unmarshal(data, &result); err != nil || result.NodeID != config.NodeID || !result.Success || !validReceipt(result.Receipt) {
			return nil, errors.New("invalid saved removal result")
		}
		s.result = &result
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	data, err = os.ReadFile(filepath.Join(directory, "finalize.json"))
	if err == nil {
		var request Request
		if err = json.Unmarshal(data, &request); err != nil || !s.matchesResult(request) || validateCallbackURL(request.CallbackURL) != nil {
			return nil, errors.New("invalid saved finalization request")
		}
		s.callback = &request
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	s.Execute = s.execute
	return s, nil
}

func (s *Server) execute(ctx context.Context, purge bool) error {
	args := []string{filepath.Join(Library, "uninstall.sh"), "--config", filepath.Join(s.Directory, "node.yaml"), "--keep-data"}
	if purge {
		args[len(args)-1] = "--purge"
	}
	command := exec.CommandContext(ctx, "/bin/bash", args...)
	command.Env = append(os.Environ(), "TP_DEFER_HOST_CLEANUP=1", "TP_ORIGINAL_CONFIG_FILE="+s.Config.OriginalConfig)
	for key, value := range s.Config.Environment {
		command.Env = append(command.Env, key+"="+value)
	}
	// Script output stays on the host. No credentials, paths or shell output are
	// returned to the browser; failure leaves the Web registration intact.
	command.Stdout, command.Stderr = os.Stdout, os.Stderr
	return command.Run()
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/remove", s.remove)
	mux.HandleFunc("/finalize", s.finalize)
	return mux
}

func decode(w http.ResponseWriter, r *http.Request) (Request, error) {
	if r.Method != http.MethodPost {
		return Request{}, errors.New("POST is required")
	}
	if r.TLS == nil || len(r.TLS.VerifiedChains) == 0 {
		return Request{}, errors.New("verified mTLS client is required")
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	var request Request
	if err := d.Decode(&request); err != nil {
		return request, err
	}
	if err := d.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return request, errors.New("unexpected trailing JSON")
	}
	return request, nil
}

func (s *Server) remove(w http.ResponseWriter, r *http.Request) {
	request, err := decode(w, r)
	if err != nil || request.NodeID != s.Config.NodeID || request.Receipt != "" || request.CallbackURL != "" {
		http.Error(w, "invalid removal request", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.finalized {
		http.Error(w, "removal already finalized", http.StatusConflict)
		return
	}
	if s.result != nil {
		if request.Purge != s.result.Purge {
			http.Error(w, "removal mode conflicts with saved result", http.StatusConflict)
			return
		}
		writeResult(w, s.result)
		return
	}
	// Preserve the authenticated endpoint until Web commits its transaction and
	// acknowledges the result. These copies are deleted by /finalize.
	if err = s.snapshotTLS(); err != nil {
		http.Error(w, "could not preserve removal channel", 500)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	if err = s.Execute(ctx, request.Purge); err != nil {
		http.Error(w, "host uninstall failed; registration retained", 500)
		return
	}
	var token [32]byte
	if _, err = rand.Read(token[:]); err != nil {
		http.Error(w, "could not create result receipt", 500)
		return
	}
	result := &Result{NodeID: request.NodeID, Purge: request.Purge, Receipt: hex.EncodeToString(token[:]), Success: true}
	data, _ := json.Marshal(result)
	if err = atomicWrite(filepath.Join(s.Directory, "result.json"), data); err != nil {
		http.Error(w, "could not persist result", 500)
		return
	}
	s.result = result
	writeResult(w, result)
}

func (s *Server) finalize(w http.ResponseWriter, r *http.Request) {
	request, err := decode(w, r)
	if err != nil {
		http.Error(w, "invalid finalization request", 400)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.matchesResult(request) || validateCallbackURL(request.CallbackURL) != nil {
		http.Error(w, "invalid removal receipt", 409)
		return
	}
	// The same authenticated Web identity may repair its deployment URL while
	// keeping the original receipt. Publish that change durably before ACKing.
	if s.callback == nil || s.callback.CallbackURL != request.CallbackURL {
		data, _ := json.Marshal(request)
		if err = atomicWrite(filepath.Join(s.Directory, "finalize.json"), data); err != nil {
			http.Error(w, "could not persist finalization request", 500)
			return
		}
		s.callback = &request
	}
	s.startFinalizerLocked()
	writeResult(w, s.result)
}

func validReceipt(receipt string) bool {
	decoded, err := hex.DecodeString(receipt)
	return err == nil && len(decoded) == 32 && receipt == hex.EncodeToString(decoded)
}

func (s *Server) matchesResult(request Request) bool {
	return s.result != nil && request.NodeID == s.result.NodeID && request.Receipt == s.result.Receipt && request.Purge == s.result.Purge
}

func validateCallbackURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || len(raw) > 2048 || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Path != callbackPath || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("a fixed HTTPS Web removal callback is required")
	}
	return nil
}

func (s *Server) startFinalizerLocked() {
	if s.callback == nil || s.workerRunning || s.finalized {
		return
	}
	s.workerRunning = true
	go s.finalizationLoop()
}

func (s *Server) finalizationLoop() {
	defer func() { s.mu.Lock(); s.workerRunning = false; s.mu.Unlock() }()
	for {
		if s.workerContext.Err() != nil {
			return
		}
		s.mu.Lock()
		request := *s.callback
		s.mu.Unlock()
		if err := s.sendCallback(s.workerContext, request); err == nil {
			s.mu.Lock()
			if s.Finalize != nil && s.Finalize() == nil {
				s.finalized = true
				s.mu.Unlock()
				return
			}
			s.mu.Unlock()
		}
		timer := time.NewTimer(s.callbackInterval)
		select {
		case <-s.workerContext.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func (s *Server) sendCallback(ctx context.Context, request Request) error {
	body, _ := json.Marshal(struct {
		NodeID  uint   `json:"nodeId"`
		Receipt string `json:"receipt"`
	}{request.NodeID, request.Receipt})
	r, err := http.NewRequestWithContext(ctx, http.MethodPost, request.CallbackURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	r.Header.Set("Content-Type", "application/json")
	response, err := s.callbackClient.Do(r)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	// Other middleware may return HTTP 200 for application errors. Only the
	// dedicated callback's 204 confirms that the Web outbox was cleared.
	if response.StatusCode != http.StatusNoContent {
		return fmt.Errorf("Web cleanup callback returned HTTP %d", response.StatusCode)
	}
	return nil
}

func writeResult(w http.ResponseWriter, result *Result) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

func atomicWrite(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".host-removal-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(data)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err = os.Rename(f.Name(), path); err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func (s *Server) snapshotTLS() error {
	for name, source := range map[string]string{"server.crt": s.Config.Certificate, "server.key": s.Config.Key, "client-ca.crt": s.Config.ClientCA} {
		data, err := os.ReadFile(source)
		if errors.Is(err, os.ErrNotExist) {
			data, err = os.ReadFile(filepath.Join(s.Directory, name))
		}
		if err != nil {
			return err
		}
		if err = atomicWrite(filepath.Join(s.Directory, name), data); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) TLSConfig() *tls.Config {
	config := &tls.Config{MinVersion: tls.VersionTLS12, ClientAuth: tls.RequireAndVerifyClientCert}
	config.GetConfigForClient = func(*tls.ClientHelloInfo) (*tls.Config, error) {
		// Read the live trust bundle on every handshake, so control CA rotation
		// also covers this host endpoint without another distribution channel.
		ca, err := os.ReadFile(s.Config.ClientCA)
		if err != nil {
			ca, err = os.ReadFile(filepath.Join(s.Directory, "client-ca.crt"))
		}
		if err != nil {
			return nil, err
		}
		roots := x509.NewCertPool()
		if !roots.AppendCertsFromPEM(ca) {
			return nil, errors.New("invalid control CA bundle")
		}
		certificate, err := tls.LoadX509KeyPair(s.Config.Certificate, s.Config.Key)
		if err != nil {
			// A partially failed purge may erase Caddy before the live CA. Keep
			// the saved channel usable so the same operation can be retried.
			certificate, err = tls.LoadX509KeyPair(filepath.Join(s.Directory, "server.crt"), filepath.Join(s.Directory, "server.key"))
		}
		if err != nil {
			return nil, err
		}
		return &tls.Config{MinVersion: tls.VersionTLS12, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: roots, Certificates: []tls.Certificate{certificate}}, nil
	}
	return config
}

func Run(config Config) error {
	s, err := New(config, Directory)
	if err != nil {
		return err
	}
	httpServer := &http.Server{Addr: fmt.Sprintf(":%d", config.Port), Handler: s.Handler(), TLSConfig: s.TLSConfig(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.workerContext = ctx
	s.Finalize = func() error {
		if err := Cleanup(); err != nil {
			return err
		}
		go func() { time.Sleep(time.Second); _ = httpServer.Shutdown(context.Background()) }()
		return nil
	}
	s.mu.Lock()
	s.startFinalizerLocked()
	s.mu.Unlock()
	err = httpServer.ListenAndServeTLS("", "")
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// CleanupRequired survives a crash while deleting Directory, including one
// that removes result.json before config.json. It contains no credentials.
func CleanupRequired() (bool, error) {
	_, err := os.Stat(filepath.Join(Library, cleanupMarker))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

// Cleanup only removes the fixed maintenance installation. It never runs the
// business uninstaller, so startup recovery cannot delete retained node data.
func Cleanup() error {
	return cleanupMaintenance(Directory, Library, Unit, func(args ...string) error {
		return exec.Command("systemctl", args...).Run()
	})
}

func cleanupMaintenance(directory, library, unit string, systemctl func(...string) error) error {
	if err := os.MkdirAll(library, 0700); err != nil {
		return err
	}
	if err := atomicWrite(filepath.Join(library, cleanupMarker), []byte("1\n")); err != nil {
		return err
	}
	// Keep the executable and unit until every credential and persisted request
	// has been erased. A restart observes the marker and resumes fixed cleanup.
	if err := os.RemoveAll(directory); err != nil {
		return err
	}
	entries, err := os.ReadDir(library)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Name() == "tp-host-agent" || entry.Name() == cleanupMarker {
			continue
		}
		if err = os.RemoveAll(filepath.Join(library, entry.Name())); err != nil {
			return err
		}
	}
	if _, err = os.Stat(unit); err == nil {
		if err = systemctl("disable", "trojanpanelnext-host.service"); err != nil {
			return err
		}
		if err = os.Remove(unit); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err = systemctl("daemon-reload"); err != nil {
		return err
	}
	if err = os.Remove(filepath.Join(library, cleanupMarker)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err = os.Remove(filepath.Join(library, "tp-host-agent")); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.Remove(library)
}
