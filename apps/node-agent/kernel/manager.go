package kernel

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	DefaultRuntimeDir = "/tpdata/trojan-panel-core/runtime"
	maxDownloadSize   = 200 << 20
	maxExtractedSize  = 300 << 20
)

var (
	ErrUnsupportedArchitecture = errors.New("unsupported architecture: only linux amd64 and arm64 are supported")
	ErrInvalidKernel           = errors.New("unsupported managed kernel")
	ErrInvalidChannel          = errors.New("release channel does not match official prerelease metadata")
	ErrOperationNotFound       = errors.New("kernel operation not found")
	safeVersion                = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]{0,63}$`)
)

type Kernel string

const (
	KernelXray      Kernel = "xray"
	KernelHysteria2 Kernel = "hysteria2"
)

type Channel string

const (
	ChannelStable     Channel = "stable"
	ChannelPrerelease Channel = "prerelease"
	ChannelLegacy     Channel = "legacy"
)

type Action string

const (
	ActionInstall  Action = "install"
	ActionRollback Action = "rollback"
)

type Stage string

const (
	StageQueued      Stage = "queued"
	StageDownloading Stage = "downloading"
	StageVerifying   Stage = "verifying"
	StagePreflight   Stage = "preflight"
	StageSwitching   Stage = "switching"
	StageRestarting  Stage = "restarting"
	StageObserving   Stage = "observing"
	StageSucceeded   Stage = "succeeded"
	StageRollingBack Stage = "rolling_back"
	StageRolledBack  Stage = "rolled_back"
	StageFailed      Stage = "failed"
)

type Version struct {
	Version    string    `json:"version"`
	Channel    Channel   `json:"channel"`
	SHA256     string    `json:"sha256"`
	Installed  time.Time `json:"installedAt"`
	Legacy     bool      `json:"legacy"`
	Successful bool      `json:"successful"`
}

type InventoryItem struct {
	Kernel         Kernel    `json:"kernel"`
	Supported      bool      `json:"supported"`
	CurrentVersion string    `json:"currentVersion"`
	CurrentSHA256  string    `json:"currentSha256"`
	Channel        Channel   `json:"channel"`
	InUse          bool      `json:"inUse"`
	Versions       []Version `json:"versions"`
	Error          string    `json:"error,omitempty"`
}

type Inventory struct {
	OS      string          `json:"os"`
	Arch    string          `json:"arch"`
	Kernels []InventoryItem `json:"kernels"`
}

type OperationRequest struct {
	IdempotencyKey string  `json:"idempotencyKey"`
	Kernel         Kernel  `json:"kernel"`
	Version        string  `json:"version"`
	Channel        Channel `json:"channel"`
	Action         Action  `json:"action"`
}

type Operation struct {
	ID             string    `json:"id"`
	IdempotencyKey string    `json:"idempotencyKey"`
	Kernel         Kernel    `json:"kernel"`
	FromVersion    string    `json:"fromVersion"`
	TargetVersion  string    `json:"targetVersion"`
	Channel        Channel   `json:"channel"`
	Action         Action    `json:"action"`
	SHA256         string    `json:"sha256,omitempty"`
	Stage          Stage     `json:"stage"`
	Error          string    `json:"error,omitempty"`
	RollbackError  string    `json:"rollbackError,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// Lifecycle is the seam between version management and the existing proxy
// process supervisor. Preflight must not alter live processes. Restart only
// replaces the selected kernel's children; Observe verifies every instance.
type Lifecycle interface {
	Preflight(ctx context.Context, kernel Kernel, candidateBinary string) error
	Restart(ctx context.Context, kernel Kernel) error
	Observe(ctx context.Context, kernel Kernel, duration time.Duration) error
	InUse(ctx context.Context, kernel Kernel) bool
}

type noopLifecycle struct{}

func (noopLifecycle) Preflight(context.Context, Kernel, string) error      { return nil }
func (noopLifecycle) Restart(context.Context, Kernel) error                { return nil }
func (noopLifecycle) Observe(context.Context, Kernel, time.Duration) error { return nil }
func (noopLifecycle) InUse(context.Context, Kernel) bool                   { return false }

type managerConfig struct {
	RuntimeDir string
	APIBase    string
	AllowedURL func(string) bool
	HTTPClient *http.Client
	GOOS       string
	GOARCH     string
	Observe    time.Duration
	Lifecycle  Lifecycle
}

type Manager struct {
	runtimeDir string
	apiBase    string
	allowedURL func(string) bool
	client     *http.Client
	goos       string
	goarch     string
	observe    time.Duration
	lifecycle  Lifecycle

	mu          sync.RWMutex
	operations  map[string]*Operation
	idempotency map[string]string
	kernelLocks map[Kernel]*sync.Mutex
}

func New(runtimeDir string, lifecycle Lifecycle) (*Manager, error) {
	if runtimeDir == "" {
		runtimeDir = DefaultRuntimeDir
	}
	return newManager(managerConfig{
		RuntimeDir: runtimeDir,
		APIBase:    "https://api.github.com",
		AllowedURL: officialDownloadURL,
		HTTPClient: &http.Client{Timeout: 2 * time.Minute},
		GOOS:       runtime.GOOS,
		GOARCH:     runtime.GOARCH,
		Observe:    30 * time.Second,
		Lifecycle:  lifecycle,
	})
}

func newManager(cfg managerConfig) (*Manager, error) {
	if cfg.RuntimeDir == "" {
		return nil, errors.New("runtime directory is required")
	}
	if cfg.APIBase == "" {
		cfg.APIBase = "https://api.github.com"
	}
	if cfg.AllowedURL == nil {
		cfg.AllowedURL = officialDownloadURL
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 2 * time.Minute}
	}
	if cfg.GOOS == "" {
		cfg.GOOS = runtime.GOOS
	}
	if cfg.GOARCH == "" {
		cfg.GOARCH = runtime.GOARCH
	}
	if cfg.Observe <= 0 {
		cfg.Observe = 30 * time.Second
	}
	if cfg.Lifecycle == nil {
		cfg.Lifecycle = noopLifecycle{}
	}
	m := &Manager{
		runtimeDir:  cfg.RuntimeDir,
		apiBase:     strings.TrimRight(cfg.APIBase, "/"),
		allowedURL:  cfg.AllowedURL,
		client:      cfg.HTTPClient,
		goos:        cfg.GOOS,
		goarch:      cfg.GOARCH,
		observe:     cfg.Observe,
		lifecycle:   cfg.Lifecycle,
		operations:  make(map[string]*Operation),
		idempotency: make(map[string]string),
		kernelLocks: map[Kernel]*sync.Mutex{
			KernelXray:      {},
			KernelHysteria2: {},
		},
	}
	if err := os.MkdirAll(filepath.Join(cfg.RuntimeDir, "operations"), 0700); err != nil {
		return nil, err
	}
	if err := m.loadOperations(); err != nil {
		return nil, err
	}
	return m, nil
}

func (m *Manager) Inventory(ctx context.Context) (Inventory, error) {
	inventory := Inventory{OS: m.goos, Arch: m.goarch}
	supported := m.goos == "linux" && (m.goarch == "amd64" || m.goarch == "arm64")
	for _, name := range []Kernel{KernelXray, KernelHysteria2} {
		item := InventoryItem{Kernel: name, Supported: supported}
		versions, err := m.readVersions(name)
		if err != nil {
			item.Error = err.Error()
		}
		item.Versions = versions
		current, err := currentVersion(m.runtimeDir, name)
		if err == nil {
			item.CurrentVersion = current
			item.InUse = m.lifecycle.InUse(ctx, name)
			for _, version := range versions {
				if version.Version == current {
					item.CurrentSHA256 = version.SHA256
					item.Channel = version.Channel
					break
				}
			}
		}
		inventory.Kernels = append(inventory.Kernels, item)
	}
	return inventory, nil
}

func (m *Manager) Start(ctx context.Context, request OperationRequest) (*Operation, error) {
	if err := m.validateRequest(request); err != nil {
		return nil, err
	}
	m.mu.Lock()
	if id, ok := m.idempotency[request.IdempotencyKey]; ok {
		op := cloneOperation(m.operations[id])
		m.mu.Unlock()
		return op, nil
	}
	id, err := randomID()
	if err != nil {
		m.mu.Unlock()
		return nil, err
	}
	from, _ := currentVersion(m.runtimeDir, request.Kernel)
	now := time.Now().UTC()
	op := &Operation{
		ID: id, IdempotencyKey: request.IdempotencyKey, Kernel: request.Kernel,
		FromVersion: from, TargetVersion: request.Version, Channel: request.Channel,
		Action: request.Action, Stage: StageQueued, CreatedAt: now, UpdatedAt: now,
	}
	m.operations[id] = op
	m.idempotency[request.IdempotencyKey] = id
	if err = m.persistLocked(op); err != nil {
		delete(m.operations, id)
		delete(m.idempotency, request.IdempotencyKey)
		m.mu.Unlock()
		return nil, err
	}
	result := cloneOperation(op)
	m.mu.Unlock()

	go m.run(context.WithoutCancel(ctx), id)
	return result, nil
}

func (m *Manager) Get(_ context.Context, id string) (*Operation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	op, ok := m.operations[id]
	if !ok {
		return nil, ErrOperationNotFound
	}
	return cloneOperation(op), nil
}

func (m *Manager) validateRequest(request OperationRequest) error {
	if request.Kernel != KernelXray && request.Kernel != KernelHysteria2 {
		return ErrInvalidKernel
	}
	if m.goos != "linux" || (m.goarch != "amd64" && m.goarch != "arm64") {
		return ErrUnsupportedArchitecture
	}
	if request.IdempotencyKey == "" || request.Version == "" {
		return errors.New("idempotency key and version are required")
	}
	if !safeVersion.MatchString(request.Version) {
		return errors.New("version contains unsupported characters")
	}
	if request.Channel != ChannelStable && request.Channel != ChannelPrerelease && request.Channel != ChannelLegacy {
		return ErrInvalidChannel
	}
	if request.Action != ActionInstall && request.Action != ActionRollback {
		return errors.New("unsupported kernel action")
	}
	if request.Action == ActionInstall && request.Channel == ChannelLegacy {
		return errors.New("legacy versions can only be restored from local history")
	}
	return nil
}

func (m *Manager) run(ctx context.Context, id string) {
	op, err := m.Get(ctx, id)
	if err != nil {
		return
	}
	lock := m.kernelLocks[op.Kernel]
	lock.Lock()
	defer lock.Unlock()

	var candidate string
	if op.Action == ActionInstall {
		candidate, err = m.downloadAndInstall(ctx, op)
	} else {
		var checksum string
		candidate, checksum, err = validateInstalledVersion(
			m.runtimeDir, op.Kernel, op.TargetVersion, op.Channel, true,
		)
		if err != nil {
			err = fmt.Errorf("rollback version is unavailable: %w", err)
		} else {
			m.setSHA256(id, checksum)
		}
	}
	if err != nil {
		m.finish(id, StageFailed, err, nil)
		return
	}

	m.setStage(id, StagePreflight)
	if err = m.lifecycle.Preflight(ctx, op.Kernel, candidate); err != nil {
		m.finish(id, StageFailed, err, nil)
		return
	}
	m.setStage(id, StageSwitching)
	if err = switchCurrent(m.runtimeDir, op.Kernel, op.TargetVersion); err != nil {
		m.finish(id, StageFailed, err, nil)
		return
	}
	m.setStage(id, StageRestarting)
	liveCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	if err = m.lifecycle.Restart(liveCtx, op.Kernel); err != nil {
		m.setStage(id, StageRollingBack)
		rollbackErr := m.rollback(ctx, op)
		m.finish(id, StageRolledBack, err, rollbackErr)
		return
	}
	m.setStage(id, StageObserving)
	if err = m.lifecycle.Observe(liveCtx, op.Kernel, m.observe); err != nil {
		m.setStage(id, StageRollingBack)
		rollbackErr := m.rollback(ctx, op)
		m.finish(id, StageRolledBack, err, rollbackErr)
		return
	}
	if err = markSuccessful(m.runtimeDir, op.Kernel, op.TargetVersion); err != nil {
		m.setStage(id, StageRollingBack)
		rollbackErr := m.rollback(ctx, op)
		m.finish(id, StageRolledBack, err, rollbackErr)
		return
	}
	if err = m.prune(op.Kernel); err != nil {
		// Retention is best-effort after the new kernel has passed observation.
		// A cleanup error must not report a healthy live switch as failed.
	}
	m.finish(id, StageSucceeded, nil, nil)
}

func (m *Manager) rollback(ctx context.Context, op *Operation) error {
	if op.FromVersion == "" {
		return errors.New("no previous version is available")
	}
	if err := switchCurrent(m.runtimeDir, op.Kernel, op.FromVersion); err != nil {
		return err
	}
	rollbackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 60*time.Second)
	defer cancel()
	if err := m.lifecycle.Restart(rollbackCtx, op.Kernel); err != nil {
		return err
	}
	return m.lifecycle.Observe(rollbackCtx, op.Kernel, m.observe)
}

func (m *Manager) downloadAndInstall(ctx context.Context, op *Operation) (string, error) {
	m.setStage(op.ID, StageDownloading)
	release, err := m.getRelease(ctx, op.Kernel, op.TargetVersion)
	if err != nil {
		return "", err
	}
	asset, checksumAsset, err := selectReleaseAssets(op.Kernel, op.Channel, m.goarch, release)
	if err != nil {
		return "", err
	}
	if !m.allowedURL(asset.URL) || !m.allowedURL(checksumAsset.URL) {
		return "", errors.New("release asset URL is not on the allowlist")
	}
	payload, err := m.download(ctx, asset.URL)
	if err != nil {
		return "", err
	}
	checksums, err := m.download(ctx, checksumAsset.URL)
	if err != nil {
		return "", err
	}
	m.setStage(op.ID, StageVerifying)
	want, err := checksumFor(op.Kernel, asset.Name, checksums)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	got := hex.EncodeToString(sum[:])
	if !strings.EqualFold(want, got) {
		return "", fmt.Errorf("SHA-256 mismatch for %s", asset.Name)
	}

	kernelDir := filepath.Join(m.runtimeDir, string(op.Kernel))
	if err = os.MkdirAll(filepath.Join(kernelDir, "versions"), 0750); err != nil {
		return "", err
	}
	staging, err := os.MkdirTemp(kernelDir, ".install-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(staging)
	if op.Kernel == KernelXray {
		err = extractXray(bytes.NewReader(payload), int64(len(payload)), staging)
	} else {
		err = os.WriteFile(filepath.Join(staging, binaryName(op.Kernel)), payload, 0755)
	}
	if err != nil {
		return "", err
	}
	binaryChecksum, err := hashKernelBinary(filepath.Join(staging, binaryName(op.Kernel)))
	if err != nil {
		return "", err
	}
	version := Version{
		Version: op.TargetVersion, Channel: op.Channel, SHA256: binaryChecksum,
		Installed: time.Now().UTC(), Successful: false,
	}
	metadata, err := json.MarshalIndent(version, "", "  ")
	if err != nil {
		return "", err
	}
	if err = os.WriteFile(filepath.Join(staging, "metadata.json"), metadata, 0640); err != nil {
		return "", err
	}
	target := filepath.Join(kernelDir, "versions", op.TargetVersion)
	if _, statErr := os.Stat(target); statErr == nil {
		var installedSHA string
		_, installedSHA, err = validateInstalledVersion(
			m.runtimeDir, op.Kernel, op.TargetVersion, op.Channel, false,
		)
		if err != nil {
			return "", fmt.Errorf("existing version directory failed integrity validation: %w", err)
		}
		if !strings.EqualFold(installedSHA, binaryChecksum) {
			return "", errors.New("existing version checksum differs from the official release")
		}
	} else if !os.IsNotExist(statErr) {
		return "", statErr
	} else if err = os.Rename(staging, target); err != nil {
		return "", err
	}
	m.setSHA256(op.ID, binaryChecksum)
	return filepath.Join(target, binaryName(op.Kernel)), nil
}

func (m *Manager) getRelease(ctx context.Context, kernel Kernel, version string) (releaseMetadata, error) {
	repository := "XTLS/Xray-core"
	releaseTag := version
	if kernel == KernelHysteria2 {
		repository = "apernet/hysteria"
		releaseTag = "app/" + version
	}
	endpoint := fmt.Sprintf("%s/repos/%s/releases/tags/%s", m.apiBase, repository, url.PathEscape(releaseTag))
	var release releaseMetadata
	err := m.getJSON(ctx, endpoint, &release)
	if err != nil {
		return release, err
	}
	if release.Draft || release.TagName != releaseTag {
		return release, errors.New("release is draft or tag does not match")
	}
	return release, nil
}

func (m *Manager) getJSON(ctx context.Context, endpoint string, target any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "trojan-panel-core")
	response, err := m.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("GitHub release request failed: %s", response.Status)
	}
	return json.NewDecoder(io.LimitReader(response.Body, 4<<20)).Decode(target)
}

func (m *Manager) download(ctx context.Context, endpoint string) ([]byte, error) {
	if !m.allowedURL(endpoint) {
		return nil, errors.New("release asset URL is not on the allowlist")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", "trojan-panel-core")
	response, err := m.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if !m.allowedURL(response.Request.URL.String()) {
		return nil, errors.New("release redirect URL is not on the allowlist")
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("asset download failed: %s", response.Status)
	}
	reader := io.LimitReader(response.Body, maxDownloadSize+1)
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 || len(data) > maxDownloadSize {
		return nil, errors.New("release asset is empty or too large")
	}
	return data, nil
}

func (m *Manager) setStage(id string, stage Stage) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if op := m.operations[id]; op != nil {
		op.Stage = stage
		op.UpdatedAt = time.Now().UTC()
		_ = m.persistLocked(op)
	}
}

func (m *Manager) setSHA256(id, checksum string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if op := m.operations[id]; op != nil {
		op.SHA256 = checksum
		op.UpdatedAt = time.Now().UTC()
		_ = m.persistLocked(op)
	}
}

func (m *Manager) finish(id string, stage Stage, operationErr, rollbackErr error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if op := m.operations[id]; op != nil {
		op.Stage = stage
		op.UpdatedAt = time.Now().UTC()
		if operationErr != nil {
			op.Error = operationErr.Error()
		}
		if rollbackErr != nil {
			op.RollbackError = rollbackErr.Error()
			op.Stage = StageFailed
		}
		_ = m.persistLocked(op)
	}
}

func (m *Manager) persistLocked(op *Operation) error {
	data, err := json.MarshalIndent(op, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(m.runtimeDir, "operations", op.ID+".json")
	tmp := path + ".tmp"
	if err = os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (m *Manager) loadOperations() error {
	files, err := filepath.Glob(filepath.Join(m.runtimeDir, "operations", "*.json"))
	if err != nil {
		return err
	}
	for _, file := range files {
		data, readErr := os.ReadFile(file)
		if readErr != nil {
			return readErr
		}
		var op Operation
		if readErr = json.Unmarshal(data, &op); readErr != nil {
			return readErr
		}
		if op.Stage != StageSucceeded && op.Stage != StageFailed && op.Stage != StageRolledBack {
			op.Error = "operation interrupted by core restart"
			switch op.Stage {
			case StageSwitching, StageRestarting, StageObserving, StageRollingBack:
				current, currentErr := currentVersion(m.runtimeDir, op.Kernel)
				if currentErr == nil && current == op.FromVersion {
					op.Stage = StageRolledBack
				} else if op.FromVersion != "" {
					if rollbackErr := switchCurrent(m.runtimeDir, op.Kernel, op.FromVersion); rollbackErr == nil {
						op.Stage = StageRolledBack
					} else {
						op.Stage = StageFailed
						op.RollbackError = rollbackErr.Error()
					}
				} else {
					op.Stage = StageFailed
					op.RollbackError = "no previous version is available"
				}
			default:
				op.Stage = StageFailed
			}
			op.UpdatedAt = time.Now().UTC()
		}
		copy := op
		m.operations[op.ID] = &copy
		m.idempotency[op.IdempotencyKey] = op.ID
		if err = m.persistLocked(&copy); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) readVersions(kernel Kernel) ([]Version, error) {
	entries, err := os.ReadDir(filepath.Join(m.runtimeDir, string(kernel), "versions"))
	if os.IsNotExist(err) {
		return []Version{}, nil
	}
	if err != nil {
		return nil, err
	}
	versions := make([]Version, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(m.runtimeDir, string(kernel), "versions", entry.Name(), "metadata.json"))
		if readErr != nil {
			continue
		}
		var version Version
		if json.Unmarshal(data, &version) == nil {
			versions = append(versions, version)
		}
	}
	sort.Slice(versions, func(i, j int) bool { return versions[i].Installed.After(versions[j].Installed) })
	return versions, nil
}

func (m *Manager) prune(kernel Kernel) error {
	current, _ := currentVersion(m.runtimeDir, kernel)
	versions, err := m.readVersions(kernel)
	if err != nil {
		return err
	}
	keep := make(map[string]bool)
	if current != "" {
		keep[current] = true
	}
	for _, version := range versions {
		if version.Successful && len(keep) < 3 {
			keep[version.Version] = true
		}
	}
	for _, version := range versions {
		if !keep[version.Version] {
			if err = os.RemoveAll(filepath.Join(m.runtimeDir, string(kernel), "versions", version.Version)); err != nil {
				return err
			}
		}
	}
	return nil
}

type releaseMetadata struct {
	TagName    string         `json:"tag_name"`
	Draft      bool           `json:"draft"`
	Prerelease bool           `json:"prerelease"`
	Assets     []releaseAsset `json:"assets"`
}

type releaseAsset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

func selectReleaseAssets(kernel Kernel, channel Channel, arch string, release releaseMetadata) (releaseAsset, releaseAsset, error) {
	if arch != "amd64" && arch != "arm64" {
		return releaseAsset{}, releaseAsset{}, ErrUnsupportedArchitecture
	}
	if (channel == ChannelPrerelease) != release.Prerelease {
		return releaseAsset{}, releaseAsset{}, ErrInvalidChannel
	}
	var wanted, checksumName string
	if kernel == KernelXray {
		if arch == "amd64" {
			wanted = "Xray-linux-64.zip"
		} else {
			wanted = "Xray-linux-arm64-v8a.zip"
		}
		checksumName = wanted + ".dgst"
	} else if kernel == KernelHysteria2 {
		wanted = "hysteria-linux-" + arch
		checksumName = "hashes.txt"
	} else {
		return releaseAsset{}, releaseAsset{}, ErrInvalidKernel
	}
	var asset, checksum releaseAsset
	for _, candidate := range release.Assets {
		switch candidate.Name {
		case wanted:
			asset = candidate
		case checksumName:
			checksum = candidate
		}
	}
	if asset.Name == "" || checksum.Name == "" || asset.URL == "" || checksum.URL == "" {
		return releaseAsset{}, releaseAsset{}, errors.New("required release asset or official checksum is missing")
	}
	return asset, checksum, nil
}

func checksumFor(kernel Kernel, assetName string, contents []byte) (string, error) {
	scanner := bufio.NewScanner(bytes.NewReader(contents))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if kernel == KernelXray && len(fields) == 2 && fields[0] == "SHA2-256=" {
			if len(fields[1]) == sha256.Size*2 {
				if _, err := hex.DecodeString(fields[1]); err == nil {
					return strings.ToLower(fields[1]), nil
				}
			}
		}
		for i, field := range fields {
			if filepath.Base(strings.TrimPrefix(field, "*")) == assetName {
				for _, candidate := range fields[:i] {
					candidate = strings.TrimSpace(candidate)
					if len(candidate) == sha256.Size*2 {
						if _, err := hex.DecodeString(candidate); err == nil {
							return strings.ToLower(candidate), nil
						}
					}
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", errors.New("official SHA-256 is missing")
}

func extractXray(reader io.ReaderAt, size int64, target string) error {
	archive, err := zip.NewReader(reader, size)
	if err != nil {
		return err
	}
	var extracted int64
	found := false
	for _, file := range archive.File {
		clean := filepath.Clean(file.Name)
		if filepath.IsAbs(file.Name) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return errors.New("archive contains path traversal")
		}
		if file.Mode()&os.ModeSymlink != 0 {
			return errors.New("archive contains a symlink")
		}
		extracted += int64(file.UncompressedSize64)
		if extracted > maxExtractedSize {
			return errors.New("archive expands beyond the size limit")
		}
		if filepath.Base(clean) != "xray" || file.FileInfo().IsDir() {
			continue
		}
		source, openErr := file.Open()
		if openErr != nil {
			return openErr
		}
		destination := filepath.Join(target, "xray")
		output, openErr := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0755)
		if openErr != nil {
			source.Close()
			return openErr
		}
		_, copyErr := io.Copy(output, io.LimitReader(source, maxExtractedSize+1))
		closeErr := output.Close()
		source.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		found = true
	}
	if !found {
		return errors.New("xray binary is missing from archive")
	}
	return nil
}

func currentVersion(runtimeDir string, kernel Kernel) (string, error) {
	target, err := os.Readlink(filepath.Join(runtimeDir, string(kernel), "current"))
	if err != nil {
		return "", err
	}
	return filepath.Base(filepath.Clean(target)), nil
}

func validateInstalledVersion(runtimeDir string, kernel Kernel, versionName string, channel Channel, requireSuccessful bool) (string, string, error) {
	versionDir := filepath.Join(runtimeDir, string(kernel), "versions", versionName)
	data, err := os.ReadFile(filepath.Join(versionDir, "metadata.json"))
	if err != nil {
		return "", "", err
	}
	var version Version
	if err = json.Unmarshal(data, &version); err != nil {
		return "", "", err
	}
	if version.Version != versionName || version.Channel != channel {
		return "", "", errors.New("local metadata does not match requested version and channel")
	}
	if requireSuccessful && !version.Successful {
		return "", "", errors.New("local version has never completed a successful observation")
	}
	if len(version.SHA256) != sha256.Size*2 {
		return "", "", errors.New("local metadata has no valid SHA-256")
	}
	binary := filepath.Join(versionDir, binaryName(kernel))
	checksum, err := hashKernelBinary(binary)
	if err != nil {
		return "", "", err
	}
	if !strings.EqualFold(checksum, version.SHA256) {
		return "", "", errors.New("local kernel binary SHA-256 does not match metadata")
	}
	return binary, checksum, nil
}

func hashKernelBinary(binary string) (string, error) {
	file, err := os.Open(binary)
	if err != nil {
		return "", err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Size() == 0 {
		return "", errors.New("local kernel binary is empty or not a regular file")
	}
	hash := sha256.New()
	if _, err = io.Copy(hash, io.LimitReader(file, maxDownloadSize+1)); err != nil {
		return "", err
	}
	if info.Size() > maxDownloadSize {
		return "", errors.New("local kernel binary is too large")
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func switchCurrent(runtimeDir string, kernel Kernel, version string) error {
	kernelDir := filepath.Join(runtimeDir, string(kernel))
	target := filepath.Join("versions", version)
	if _, err := os.Stat(filepath.Join(kernelDir, target, binaryName(kernel))); err != nil {
		return err
	}
	if err := os.MkdirAll(kernelDir, 0750); err != nil {
		return err
	}
	tmp := filepath.Join(kernelDir, ".current-"+version)
	_ = os.Remove(tmp)
	if err := os.Symlink(target, tmp); err != nil {
		return err
	}
	if err := os.Rename(tmp, filepath.Join(kernelDir, "current")); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func markSuccessful(runtimeDir string, kernel Kernel, versionName string) error {
	path := filepath.Join(runtimeDir, string(kernel), "versions", versionName, "metadata.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var version Version
	if err = json.Unmarshal(data, &version); err != nil {
		return err
	}
	version.Successful = true
	data, err = json.MarshalIndent(version, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err = os.WriteFile(tmp, data, 0640); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func binaryName(kernel Kernel) string {
	if kernel == KernelXray {
		return "xray"
	}
	return "hysteria2"
}

func officialDownloadURL(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" {
		return false
	}
	switch strings.ToLower(parsed.Hostname()) {
	case "github.com", "objects.githubusercontent.com", "release-assets.githubusercontent.com":
		return true
	default:
		return false
	}
}

func randomID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(value[:]), nil
}

func cloneOperation(op *Operation) *Operation {
	if op == nil {
		return nil
	}
	copy := *op
	return &copy
}
