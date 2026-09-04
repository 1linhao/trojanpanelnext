package kernel

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestSelectReleaseAssetUsesOfficialChannelAndArchitecture(t *testing.T) {
	release := releaseMetadata{
		TagName:    "v1.2.3",
		Prerelease: false,
		Assets: []releaseAsset{
			{Name: "Xray-linux-64.zip", URL: "https://github.com/example/xray.zip"},
			{Name: "Xray-linux-64.zip.dgst", URL: "https://github.com/example/xray.zip.dgst"},
			{Name: "Xray-linux-arm64-v8a.zip", URL: "https://github.com/example/xray-arm64.zip"},
		},
	}

	asset, checksum, err := selectReleaseAssets(KernelXray, ChannelStable, "amd64", release)
	if err != nil {
		t.Fatal(err)
	}
	if asset.Name != "Xray-linux-64.zip" || checksum.Name != "Xray-linux-64.zip.dgst" {
		t.Fatalf("unexpected assets: %#v %#v", asset, checksum)
	}

	if _, _, err = selectReleaseAssets(KernelXray, ChannelPrerelease, "amd64", release); err == nil {
		t.Fatal("stable release must not be accepted as prerelease")
	}
	if _, _, err = selectReleaseAssets(KernelXray, ChannelStable, "386", release); !errors.Is(err, ErrUnsupportedArchitecture) {
		t.Fatalf("expected unsupported architecture, got %v", err)
	}
}

func TestChecksumParsesOfficialXrayDigestFormat(t *testing.T) {
	want := strings.Repeat("a", sha256.Size*2)
	got, err := checksumFor(KernelXray, "Xray-linux-64.zip", []byte("MD5= deadbeef\nSHA2-256= "+want+"\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("checksum = %q", got)
	}
}

func TestChecksumParsesOfficialHysteriaHashesFormat(t *testing.T) {
	want := strings.Repeat("b", sha256.Size*2)
	got, err := checksumFor(
		KernelHysteria2,
		"hysteria-linux-amd64",
		[]byte(want+"  build/hysteria-linux-amd64\n"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("checksum = %q", got)
	}
}

func TestExtractXrayRejectsPathTraversal(t *testing.T) {
	var archive bytes.Buffer
	zw := zip.NewWriter(&archive)
	entry, err := zw.Create("../xray")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = entry.Write([]byte("bad")); err != nil {
		t.Fatal(err)
	}
	if err = zw.Close(); err != nil {
		t.Fatal(err)
	}

	if err = extractXray(bytes.NewReader(archive.Bytes()), int64(archive.Len()), t.TempDir()); err == nil {
		t.Fatal("path traversal archive was accepted")
	}
}

func TestManagerRejectsVersionPathTraversal(t *testing.T) {
	m, err := newManager(managerConfig{
		RuntimeDir: t.TempDir(), GOOS: "linux", GOARCH: "amd64",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = m.Start(context.Background(), OperationRequest{
		IdempotencyKey: "unsafe-version",
		Kernel:         KernelXray,
		Version:        "../../outside",
		Channel:        ChannelLegacy,
		Action:         ActionRollback,
	})
	if err == nil {
		t.Fatal("path traversal version was accepted")
	}
}

func TestManagerRecoversInterruptedSwitchToPreviousVersion(t *testing.T) {
	runtimeDir := t.TempDir()
	kernelDir := filepath.Join(runtimeDir, string(KernelXray))
	for _, version := range []string{"v1.0.0", "v1.0.1"} {
		versionDir := filepath.Join(kernelDir, "versions", version)
		if err := os.MkdirAll(versionDir, 0750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(versionDir, "xray"), []byte("binary"), 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink("versions/v1.0.1", filepath.Join(kernelDir, "current")); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(runtimeDir, "operations"), 0700); err != nil {
		t.Fatal(err)
	}
	operation := Operation{
		ID: "interrupted", IdempotencyKey: "interrupted-key", Kernel: KernelXray,
		FromVersion: "v1.0.0", TargetVersion: "v1.0.1", Channel: ChannelStable,
		Action: ActionInstall, Stage: StageObserving, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	data, err := json.Marshal(operation)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(runtimeDir, "operations", "interrupted.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	manager, err := newManager(managerConfig{
		RuntimeDir: runtimeDir, GOOS: "linux", GOARCH: "amd64",
	})
	if err != nil {
		t.Fatal(err)
	}
	recovered, err := manager.Get(context.Background(), operation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.Stage != StageRolledBack {
		t.Fatalf("recovered stage = %s, rollback error = %s", recovered.Stage, recovered.RollbackError)
	}
	current, err := currentVersion(runtimeDir, KernelXray)
	if err != nil {
		t.Fatal(err)
	}
	if current != "v1.0.0" {
		t.Fatalf("recovery selected %q", current)
	}
}

func TestManagerInstallsAtomicallyRetainsThreeAndIsIdempotent(t *testing.T) {
	var releaseRequests atomic.Int32
	server := newReleaseServer(t, &releaseRequests, false)
	defer server.Close()

	runtimeDir := t.TempDir()
	m, err := newManager(managerConfig{
		RuntimeDir: runtimeDir,
		APIBase:    server.URL,
		AllowedURL: func(raw string) bool { return strings.HasPrefix(raw, server.URL) },
		GOOS:       "linux",
		GOARCH:     "amd64",
		Observe:    time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, version := range []string{"v1.0.0", "v1.0.1", "v1.0.2", "v1.0.3"} {
		op, err := m.Start(context.Background(), OperationRequest{
			IdempotencyKey: "install-" + version,
			Kernel:         KernelXray,
			Version:        version,
			Channel:        ChannelStable,
			Action:         ActionInstall,
		})
		if err != nil {
			t.Fatal(err)
		}
		waitOperation(t, m, op.ID, StageSucceeded)
	}

	inventory, err := m.Inventory(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	xray := inventory.Kernels[0]
	if xray.CurrentVersion != "v1.0.3" {
		t.Fatalf("current version = %q", xray.CurrentVersion)
	}
	if len(xray.Versions) != 3 {
		t.Fatalf("expected current plus two successful versions, got %#v", xray.Versions)
	}

	first, err := m.Start(context.Background(), OperationRequest{
		IdempotencyKey: "same-request",
		Kernel:         KernelXray,
		Version:        "v1.0.3",
		Channel:        ChannelStable,
		Action:         ActionInstall,
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := m.Start(context.Background(), OperationRequest{
		IdempotencyKey: "same-request",
		Kernel:         KernelXray,
		Version:        "v1.0.3",
		Channel:        ChannelStable,
		Action:         ActionInstall,
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID {
		t.Fatalf("idempotency returned different operations: %q %q", first.ID, second.ID)
	}
	waitOperation(t, m, first.ID, StageSucceeded)
}

func TestManagerRollsBackWhenRestartFails(t *testing.T) {
	server := newReleaseServer(t, nil, false)
	defer server.Close()
	runtimeDir := t.TempDir()
	m, err := newManager(managerConfig{
		RuntimeDir: runtimeDir,
		APIBase:    server.URL,
		AllowedURL: func(raw string) bool { return strings.HasPrefix(raw, server.URL) },
		GOOS:       "linux",
		GOARCH:     "amd64",
		Observe:    time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	first, err := m.Start(context.Background(), OperationRequest{
		IdempotencyKey: "initial",
		Kernel:         KernelXray, Version: "v1.0.0", Channel: ChannelStable, Action: ActionInstall,
	})
	if err != nil {
		t.Fatal(err)
	}
	waitOperation(t, m, first.ID, StageSucceeded)

	m.lifecycle = &failingLifecycle{}
	second, err := m.Start(context.Background(), OperationRequest{
		IdempotencyKey: "broken",
		Kernel:         KernelXray, Version: "v1.0.1", Channel: ChannelStable, Action: ActionInstall,
	})
	if err != nil {
		t.Fatal(err)
	}
	waitOperation(t, m, second.ID, StageRolledBack)

	current, err := os.Readlink(filepath.Join(runtimeDir, string(KernelXray), "current"))
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(current) != "v1.0.0" {
		t.Fatalf("rollback selected %q", current)
	}
}

func TestManagerSerializesOperationsForTheSameKernel(t *testing.T) {
	server := newReleaseServer(t, nil, false)
	defer server.Close()
	lifecycle := &concurrencyLifecycle{}
	m, err := newManager(managerConfig{
		RuntimeDir: t.TempDir(), APIBase: server.URL,
		AllowedURL: func(raw string) bool { return strings.HasPrefix(raw, server.URL) },
		GOOS:       "linux", GOARCH: "amd64", Observe: time.Millisecond, Lifecycle: lifecycle,
	})
	if err != nil {
		t.Fatal(err)
	}
	first, err := m.Start(context.Background(), OperationRequest{
		IdempotencyKey: "serial-1", Kernel: KernelXray,
		Version: "v2.0.0", Channel: ChannelStable, Action: ActionInstall,
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := m.Start(context.Background(), OperationRequest{
		IdempotencyKey: "serial-2", Kernel: KernelXray,
		Version: "v2.0.1", Channel: ChannelStable, Action: ActionInstall,
	})
	if err != nil {
		t.Fatal(err)
	}
	waitOperation(t, m, first.ID, StageSucceeded)
	waitOperation(t, m, second.ID, StageSucceeded)
	if lifecycle.maximum.Load() != 1 {
		t.Fatalf("same-kernel lifecycle concurrency = %d", lifecycle.maximum.Load())
	}
}

func TestManagerRollsBackWhenObservationFails(t *testing.T) {
	server := newReleaseServer(t, nil, false)
	defer server.Close()
	runtimeDir := t.TempDir()
	m, err := newManager(managerConfig{
		RuntimeDir: runtimeDir, APIBase: server.URL,
		AllowedURL: func(raw string) bool { return strings.HasPrefix(raw, server.URL) },
		GOOS:       "linux", GOARCH: "amd64", Observe: time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	first, err := m.Start(context.Background(), OperationRequest{
		IdempotencyKey: "observe-initial", Kernel: KernelXray,
		Version: "v3.0.0", Channel: ChannelStable, Action: ActionInstall,
	})
	if err != nil {
		t.Fatal(err)
	}
	waitOperation(t, m, first.ID, StageSucceeded)
	m.lifecycle = &observeFailingLifecycle{}
	second, err := m.Start(context.Background(), OperationRequest{
		IdempotencyKey: "observe-failure", Kernel: KernelXray,
		Version: "v3.0.1", Channel: ChannelStable, Action: ActionInstall,
	})
	if err != nil {
		t.Fatal(err)
	}
	waitOperation(t, m, second.ID, StageRolledBack)
	current, err := currentVersion(runtimeDir, KernelXray)
	if err != nil {
		t.Fatal(err)
	}
	if current != "v3.0.0" {
		t.Fatalf("observation rollback selected %q", current)
	}
}

type failingLifecycle struct{ calls atomic.Int32 }

func (*failingLifecycle) Preflight(context.Context, Kernel, string) error { return nil }
func (l *failingLifecycle) Restart(context.Context, Kernel) error {
	if l.calls.Add(1) == 1 {
		return errors.New("restart failed")
	}
	return nil
}
func (*failingLifecycle) Observe(context.Context, Kernel, time.Duration) error { return nil }
func (*failingLifecycle) InUse(context.Context, Kernel) bool                   { return false }

type observeFailingLifecycle struct{ calls atomic.Int32 }

func (*observeFailingLifecycle) Preflight(context.Context, Kernel, string) error { return nil }
func (*observeFailingLifecycle) Restart(context.Context, Kernel) error           { return nil }
func (l *observeFailingLifecycle) Observe(context.Context, Kernel, time.Duration) error {
	if l.calls.Add(1) == 1 {
		return errors.New("observation failed")
	}
	return nil
}
func (*observeFailingLifecycle) InUse(context.Context, Kernel) bool { return false }

type concurrencyLifecycle struct {
	current atomic.Int32
	maximum atomic.Int32
}

func (*concurrencyLifecycle) Preflight(context.Context, Kernel, string) error { return nil }
func (l *concurrencyLifecycle) Restart(context.Context, Kernel) error {
	current := l.current.Add(1)
	for {
		maximum := l.maximum.Load()
		if current <= maximum || l.maximum.CompareAndSwap(maximum, current) {
			break
		}
	}
	time.Sleep(20 * time.Millisecond)
	l.current.Add(-1)
	return nil
}
func (*concurrencyLifecycle) Observe(context.Context, Kernel, time.Duration) error { return nil }
func (*concurrencyLifecycle) InUse(context.Context, Kernel) bool                   { return false }

func waitOperation(t *testing.T, m *Manager, id string, wanted Stage) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		op, err := m.Get(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		if op.Stage == wanted {
			return
		}
		if op.Stage == StageFailed && wanted != StageFailed {
			t.Fatalf("operation failed: %s", op.Error)
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("operation %s did not reach %s", id, wanted)
}

func newReleaseServer(t *testing.T, count *atomic.Int32, prerelease bool) *httptest.Server {
	t.Helper()
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if count != nil {
			count.Add(1)
		}
		switch {
		case strings.Contains(r.URL.Path, "/releases/tags/"):
			version := filepath.Base(r.URL.Path)
			payload := []byte("#!/bin/sh\nexit 0\n")
			archive := xrayArchive(t, payload)
			sum := sha256.Sum256(archive)
			metadata := releaseMetadata{
				TagName: version, Prerelease: prerelease,
				Assets: []releaseAsset{
					{Name: "Xray-linux-64.zip", URL: server.URL + "/asset/" + version},
					{Name: "Xray-linux-64.zip.dgst", URL: server.URL + "/checksum/" + version},
				},
			}
			w.Header().Set("X-Test-Checksum", hex.EncodeToString(sum[:]))
			_ = json.NewEncoder(w).Encode(metadata)
		case strings.HasPrefix(r.URL.Path, "/asset/"):
			_, _ = w.Write(xrayArchive(t, []byte("#!/bin/sh\nexit 0\n")))
		case strings.HasPrefix(r.URL.Path, "/checksum/"):
			archive := xrayArchive(t, []byte("#!/bin/sh\nexit 0\n"))
			sum := sha256.Sum256(archive)
			_, _ = io.WriteString(w, "SHA2-256= "+hex.EncodeToString(sum[:])+"\n")
		default:
			http.NotFound(w, r)
		}
	}))
	return server
}

func xrayArchive(t *testing.T, binary []byte) []byte {
	t.Helper()
	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	header := &zip.FileHeader{Name: "xray", Method: zip.Store}
	header.SetMode(0755)
	entry, err := zw.CreateHeader(header)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = entry.Write(binary); err != nil {
		t.Fatal(err)
	}
	if err = zw.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}
