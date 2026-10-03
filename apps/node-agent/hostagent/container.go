package hostagent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"
)

const agentRepository = "ghcr.io/1linhao/trojanpanelnext-node-agent:"
const defaultRuntime = "/tpdata/trojan-panel-core/runtime"

var releasePattern = regexp.MustCompile(`^[1-9][0-9]*\.(0|[1-9][0-9]*)(\.(0|[1-9][0-9]*))?(-rc\.[1-9][0-9]*)?$`)
var containerPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,127}$`)
var cliTokenPattern = regexp.MustCompile(`^[A-Za-z0-9]{6,64}$`)
var errCLIUpdateActive = errors.New("container_update_active")

type maintenanceMarker struct {
	Status string `json:"status"`
	JobID  string `json:"jobId,omitempty"`
	Owner  string `json:"owner,omitempty"`
	Token  string `json:"token,omitempty"`
	NodeID uint   `json:"nodeId,omitempty"`
}

type UpdateJob struct {
	ID            string `json:"id"`
	FromVersion   string `json:"fromVersion"`
	TargetVersion string `json:"targetVersion"`
	Status        string `json:"status"`
	Error         string `json:"error"`
	StartedAt     string `json:"startedAt"`
	FinishedAt    string `json:"finishedAt"`
}

type ContainerInventory struct {
	NodeID          uint       `json:"nodeId"`
	CurrentVersion  string     `json:"currentVersion"`
	Image           string     `json:"image"`
	UpdateSupported bool       `json:"updateSupported"`
	Job             *UpdateJob `json:"job"`
}

type containerRequest struct {
	NodeID  uint   `json:"nodeId"`
	Version string `json:"version,omitempty"`
}

type updateRecord struct {
	NodeID uint      `json:"nodeId"`
	Job    UpdateJob `json:"job"`
}

func validRelease(version string) bool {
	return len(version) <= 64 && releasePattern.MatchString(version)
}
func activeUpdate(job *UpdateJob) bool {
	return job != nil && (job.Status == "queued" || job.Status == "running")
}
func updateUnit(id string) string { return "trojanpanelnext-update-" + id + ".service" }

func versionCanUpdate(from, target string) bool {
	if !validRelease(from) || !validRelease(target) {
		return false
	}
	split := func(v string) ([]string, string) {
		parts := strings.SplitN(v, "-rc.", 2)
		numbers := strings.Split(parts[0], ".")
		if len(numbers) == 2 {
			numbers = append(numbers, "0")
		}
		rc := ""
		if len(parts) == 2 {
			rc = parts[1]
		}
		return numbers, rc
	}
	a, aRC := split(from)
	b, bRC := split(target)
	compare := func(x, y string) int {
		first, _ := new(big.Int).SetString(x, 10)
		second, _ := new(big.Int).SetString(y, 10)
		return second.Cmp(first)
	}
	for i := range a {
		if cmp := compare(a[i], b[i]); cmp != 0 {
			return cmp > 0
		}
	}
	if bRC == "" {
		return true
	}
	if aRC == "" {
		return false
	}
	return compare(aRC, bRC) >= 0
}

func decodeContainer(w http.ResponseWriter, r *http.Request, inventory bool) (containerRequest, error) {
	var request containerRequest
	if r.Method != http.MethodPost || r.TLS == nil || len(r.TLS.VerifiedChains) == 0 {
		return request, errors.New("verified mTLS POST required")
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if inventory {
		var identity struct {
			NodeID uint `json:"nodeId"`
		}
		if err := decoder.Decode(&identity); err != nil {
			return request, err
		}
		request.NodeID = identity.NodeID
	} else {
		if err := decoder.Decode(&request); err != nil {
			return request, err
		}
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return request, errors.New("unexpected trailing JSON")
	}
	return request, nil
}

func writeContainerJSON(w http.ResponseWriter, value interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(value)
}

func (s *Server) runtimeDirectory() string {
	if path := s.Config.Environment["KERNEL_RUNTIME_PATH"]; path != "" {
		return path
	}
	return defaultRuntime
}

// The same lock is used by the container's kernel manager before admitting an
// operation. Its lifetime is deliberately shorter than the update worker.
func (s *Server) lockMaintenance(ctx context.Context) (func(), error) {
	directory := s.runtimeDirectory()
	if !filepath.IsAbs(directory) {
		return nil, errors.New("invalid runtime directory")
	}
	if err := os.MkdirAll(directory, 0700); err != nil {
		return nil, err
	}
	info, err := os.Lstat(directory)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("invalid runtime directory")
	}
	if err = os.Chmod(directory, 0700); err != nil {
		return nil, err
	}
	fd, err := syscall.Open(filepath.Join(directory, "maintenance.lock"), syscall.O_CREAT|syscall.O_RDWR|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), "maintenance.lock")
	if err = file.Chmod(0600); err != nil {
		_ = file.Close()
		return nil, err
	}
	for {
		if err = syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB); err == nil {
			return func() { _ = syscall.Flock(fd, syscall.LOCK_UN); _ = file.Close() }, nil
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) && !errors.Is(err, syscall.EAGAIN) {
			_ = file.Close()
			return nil, err
		}
		select {
		case <-ctx.Done():
			_ = file.Close()
			return nil, ctx.Err()
		case <-time.After(20 * time.Millisecond):
		}
	}
}

func readPrivateJSON(path string, target interface{}) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() > 65536 {
		return errors.New("invalid maintenance state file")
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	decoder := json.NewDecoder(io.LimitReader(file, 65537))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(target); err != nil {
		return err
	}
	if err = decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("invalid maintenance state JSON")
	}
	return nil
}

func (s *Server) readUpdate() (*UpdateJob, error) {
	var record updateRecord
	if err := readPrivateJSON(filepath.Join(s.Directory, "update.json"), &record); errors.Is(err, os.ErrNotExist) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	job := record.Job
	if record.NodeID != s.Config.NodeID || !validReceipt(job.ID) || !validRelease(job.FromVersion) || !validRelease(job.TargetVersion) || !versionCanUpdate(job.FromVersion, job.TargetVersion) {
		return nil, errors.New("invalid update identity")
	}
	switch job.Status {
	case "queued", "running", "succeeded", "failed":
	default:
		return nil, errors.New("invalid update state")
	}
	for _, stamp := range []string{job.StartedAt, job.FinishedAt} {
		if stamp != "" {
			if _, err := time.Parse(time.RFC3339, stamp); err != nil {
				return nil, errors.New("invalid update timestamp")
			}
		}
	}
	// Never expose arbitrary messages left by another host program or old state.
	switch job.Error {
	case "", "interrupted", "worker_start_failed", "download_failed", "entry_invalid", "entry_version_mismatch", "update_failed", "inventory_failed", "kernel_operation_active":
	default:
		return nil, errors.New("invalid update error")
	}
	return &job, nil
}

func (s *Server) cliUpdateActive() (bool, error) {
	var marker maintenanceMarker
	err := readPrivateJSON(filepath.Join(s.runtimeDirectory(), "container-update.json"), &marker)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	switch marker.Status {
	case "queued", "running", "succeeded", "failed":
	default:
		return false, errors.New("invalid maintenance marker")
	}
	if marker.Owner == "cli" {
		if !cliTokenPattern.MatchString(marker.Token) || marker.NodeID != s.Config.NodeID || marker.JobID != "" {
			return false, errors.New("invalid CLI maintenance marker")
		}
		return marker.Status == "queued" || marker.Status == "running", nil
	}
	if marker.Owner != "" || marker.Token != "" || marker.NodeID != 0 || !validReceipt(marker.JobID) {
		return false, errors.New("invalid worker maintenance marker")
	}
	return false, nil
}

func (s *Server) saveUpdate(job *UpdateJob) error {
	busy, err := s.cliUpdateActive()
	if err != nil {
		return err
	}
	if busy {
		return errCLIUpdateActive
	}
	data, err := json.Marshal(updateRecord{NodeID: s.Config.NodeID, Job: *job})
	if err != nil {
		return err
	}
	if err = atomicWrite(filepath.Join(s.Directory, "update.json"), data); err != nil {
		return err
	}
	marker, _ := json.Marshal(maintenanceMarker{Status: job.Status, JobID: job.ID})
	return atomicWrite(filepath.Join(s.runtimeDirectory(), "container-update.json"), marker)
}

func (s *Server) reconcileUpdate(ctx context.Context) (*UpdateJob, error) {
	busy, err := s.cliUpdateActive()
	if err != nil {
		return nil, err
	}
	job, err := s.readUpdate()
	if err != nil {
		return nil, err
	}
	if busy {
		return job, nil
	}
	if activeUpdate(job) {
		alive, err := s.WorkerActive(ctx, job.ID)
		if err != nil {
			return nil, err
		}
		if !alive {
			job.Status, job.Error, job.FinishedAt = "failed", "interrupted", time.Now().UTC().Format(time.RFC3339)
			if err := s.saveUpdate(job); err != nil {
				return nil, err
			}
		}
	}
	return job, nil
}

func (s *Server) inspectImage(ctx context.Context) (string, error) {
	name := s.Config.Environment["CORE_CONTAINER"]
	if name == "" {
		name = "trojan-panel-core"
	}
	if !containerPattern.MatchString(name) {
		return "", errors.New("invalid maintained container")
	}
	data, err := exec.CommandContext(ctx, "docker", "inspect", "--format", "{{.Config.Image}}", "--", name).Output()
	if err != nil || len(data) > 512 {
		return "", errors.New("container inventory unavailable")
	}
	return strings.TrimSpace(string(data)), nil
}

func (s *Server) inventory(ctx context.Context) (*ContainerInventory, error) {
	busy, err := s.cliUpdateActive()
	if err != nil {
		return nil, err
	}
	if busy {
		return nil, errCLIUpdateActive
	}
	image, err := s.InspectImage(ctx)
	if err != nil {
		return nil, err
	}
	version := strings.TrimPrefix(image, agentRepository)
	if !strings.HasPrefix(image, agentRepository) || !validRelease(version) {
		return nil, errors.New("unsupported agent image")
	}
	job, err := s.reconcileUpdate(ctx)
	if err != nil {
		return nil, err
	}
	return &ContainerInventory{NodeID: s.Config.NodeID, CurrentVersion: version, Image: image, UpdateSupported: true, Job: job}, nil
}

func (s *Server) containerInventory(w http.ResponseWriter, r *http.Request) {
	request, err := decodeContainer(w, r, true)
	if err != nil || request.NodeID != s.Config.NodeID {
		http.Error(w, "invalid container inventory request", 400)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	unlock, err := s.lockMaintenance(ctx)
	if err != nil {
		http.Error(w, "maintenance_state_unavailable", 503)
		return
	}
	defer unlock()
	inventory, err := s.inventory(ctx)
	if err != nil {
		http.Error(w, "container_inventory_unavailable", 503)
		return
	}
	writeContainerJSON(w, inventory)
}

func (s *Server) kernelOperationActive() (bool, error) {
	entries, err := os.ReadDir(filepath.Join(s.runtimeDirectory(), "operations"))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		info, err := os.Lstat(filepath.Join(s.runtimeDirectory(), "operations", entry.Name()))
		if err != nil || !info.Mode().IsRegular() || info.Size() > 65536 {
			return false, errors.New("invalid kernel operation")
		}
		file, err := os.Open(filepath.Join(s.runtimeDirectory(), "operations", entry.Name()))
		if err != nil {
			return false, err
		}
		var operation struct {
			Stage string `json:"stage"`
		}
		err = json.NewDecoder(io.LimitReader(file, 65537)).Decode(&operation)
		_ = file.Close()
		if err != nil {
			return false, err
		}
		switch operation.Stage {
		case "succeeded", "failed", "rolled_back":
		default:
			return true, nil
		}
	}
	return false, nil
}

func (s *Server) containerUpdate(w http.ResponseWriter, r *http.Request) {
	request, err := decodeContainer(w, r, false)
	if err != nil || request.NodeID != s.Config.NodeID || !validRelease(request.Version) {
		http.Error(w, "invalid container update request", 400)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.result != nil || s.finalized {
		http.Error(w, "host_removal_pending", 409)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	unlock, err := s.lockMaintenance(ctx)
	if err != nil {
		http.Error(w, "maintenance_state_unavailable", 503)
		return
	}
	defer unlock()
	busy, markerErr := s.cliUpdateActive()
	if markerErr != nil {
		http.Error(w, "maintenance_state_unavailable", 503)
		return
	}
	if busy {
		http.Error(w, "container_update_active", 409)
		return
	}
	inventory, err := s.inventory(ctx)
	if err != nil {
		http.Error(w, "container_inventory_unavailable", 503)
		return
	}
	if activeUpdate(inventory.Job) {
		if inventory.Job.TargetVersion != request.Version {
			http.Error(w, "container_update_active", 409)
			return
		}
		writeContainerJSON(w, inventory.Job)
		return
	}
	if !versionCanUpdate(inventory.CurrentVersion, request.Version) {
		http.Error(w, "downgrade_not_supported", 409)
		return
	}
	active, err := s.kernelOperationActive()
	if err != nil {
		http.Error(w, "maintenance_state_unavailable", 503)
		return
	}
	if active {
		http.Error(w, "kernel_operation_active", 409)
		return
	}
	var token [32]byte
	if _, err := rand.Read(token[:]); err != nil {
		http.Error(w, "maintenance_state_unavailable", 503)
		return
	}
	job := &UpdateJob{ID: hex.EncodeToString(token[:]), FromVersion: inventory.CurrentVersion, TargetVersion: request.Version, Status: "queued"}
	if err = s.saveUpdate(job); err != nil {
		http.Error(w, "maintenance_state_unavailable", 503)
		return
	}
	if err = s.StartUpdate(ctx, job.ID); err != nil {
		job.Status, job.Error, job.FinishedAt = "failed", "worker_start_failed", time.Now().UTC().Format(time.RFC3339)
		if err = s.saveUpdate(job); err != nil {
			http.Error(w, "maintenance_state_unavailable", 503)
			return
		}
	}
	writeContainerJSON(w, job)
}

func (s *Server) startUpdate(ctx context.Context, id string) error {
	if !validReceipt(id) {
		return errors.New("invalid update job")
	}
	command := exec.CommandContext(ctx, "systemd-run", "--unit="+updateUnit(id), "--collect", "--property=Type=exec", "--property=UMask=0077", "--property=RuntimeMaxSec=3600", "--property=TimeoutStopSec=180", "--property=KillMode=control-group", filepath.Join(Library, "tp-host-agent"), "--run-update", id)
	return command.Run()
}

func (s *Server) workerActive(ctx context.Context, id string) (bool, error) {
	if !validReceipt(id) {
		return false, errors.New("invalid update job")
	}
	command := exec.CommandContext(ctx, "systemctl", "show", "--property=LoadState", "--property=ActiveState", updateUnit(id))
	data, err := command.Output()
	load, state := "", ""
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "LoadState=") {
			load = strings.TrimPrefix(line, "LoadState=")
		}
		if strings.HasPrefix(line, "ActiveState=") {
			state = strings.TrimPrefix(line, "ActiveState=")
		}
	}
	if load == "not-found" {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if load != "loaded" {
		return false, errors.New("invalid worker load state")
	}
	switch state {
	case "active", "activating", "reloading", "deactivating":
		return true, nil
	case "inactive", "failed", "":
		return false, nil
	default:
		return false, errors.New("invalid worker state")
	}
}

// RecoverUpdate runs before opening the HTTPS listener. It also repairs a
// stale runtime marker after a crash between the two atomic state writes.
func (s *Server) RecoverUpdate(ctx context.Context) error {
	unlock, err := s.lockMaintenance(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	busy, err := s.cliUpdateActive()
	if err != nil {
		return err
	}
	if busy {
		return nil
	}
	job, err := s.reconcileUpdate(ctx)
	if err != nil {
		return err
	}
	if job != nil {
		return s.saveUpdate(job)
	}
	return nil
}

// RunUpdate runs only a previously admitted job. A systemd transient unit owns
// this process, so replacing/restarting the HTTPS maintenance service is safe.
func (s *Server) RunUpdate(ctx context.Context, id string) error {
	if !validReceipt(id) {
		return errors.New("invalid update job")
	}
	unlock, err := s.lockMaintenance(ctx)
	if err != nil {
		return err
	}
	busy, markerErr := s.cliUpdateActive()
	if markerErr != nil || busy {
		unlock()
		return errors.New("container maintenance is active")
	}
	job, err := s.readUpdate()
	if err != nil || job == nil || job.ID != id || job.Status != "queued" || s.result != nil {
		unlock()
		return errors.New("update job is not queued")
	}
	active, err := s.kernelOperationActive()
	if err != nil || active {
		job.Status, job.Error, job.FinishedAt = "failed", "kernel_operation_active", time.Now().UTC().Format(time.RFC3339)
		saved := s.saveUpdate(job)
		unlock()
		return saved
	}
	job.Status, job.StartedAt = "running", time.Now().UTC().Format(time.RFC3339)
	if err = s.saveUpdate(job); err != nil {
		unlock()
		return err
	}
	unlock()
	executeErr := s.ExecuteUpdate(context.WithValue(ctx, updateContextKey{}, job.ID), job.TargetVersion)
	// Final persistence must remain possible after the execution deadline expires.
	finishCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	unlock, err = s.lockMaintenance(finishCtx)
	if err != nil {
		return err
	}
	defer unlock()
	current, err := s.readUpdate()
	if err != nil || current == nil || current.ID != id || current.Status != "running" {
		return errors.New("update state changed")
	}
	current.Status, current.Error, current.FinishedAt = "succeeded", "", time.Now().UTC().Format(time.RFC3339)
	if executeErr != nil {
		current.Status, current.Error = "failed", "update_failed"
		var safe *updateFailure
		if errors.As(executeErr, &safe) {
			current.Error = safe.code
		}
	}
	return s.saveUpdate(current)
}

type updateFailure struct{ code string }
type updateContextKey struct{}

func (e *updateFailure) Error() string { return e.code }

func (s *Server) executeUpdate(ctx context.Context, version string) error {
	if !validRelease(version) {
		return &updateFailure{"entry_invalid"}
	}
	path := s.Config.OriginalConfig
	info, err := os.Lstat(path)
	if !filepath.IsAbs(path) || err != nil || !info.Mode().IsRegular() {
		return &updateFailure{"update_failed"}
	}
	temporary, err := os.MkdirTemp("", "tpnext-host-update-")
	if err != nil {
		return &updateFailure{"download_failed"}
	}
	defer os.RemoveAll(temporary)
	entry := filepath.Join(temporary, "tp.sh")
	url := "https://raw.githubusercontent.com/1linhao/trojanpanelnext/v" + version + "/scripts/tp.sh"
	command := exec.CommandContext(ctx, "curl", "--fail", "--location", "--silent", "--show-error", "--proto", "=https", "--proto-redir", "=https", "--retry", "2", "--retry-max-time", "180", "--connect-timeout", "10", "--max-time", "60", "--max-filesize", "5242880", url, "-o", entry)
	if err = command.Run(); err != nil {
		return &updateFailure{"download_failed"}
	}
	if err = os.Chmod(entry, 0600); err != nil {
		return &updateFailure{"entry_invalid"}
	}
	if err = exec.CommandContext(ctx, "/bin/bash", "-n", entry).Run(); err != nil {
		return &updateFailure{"entry_invalid"}
	}
	data, err := exec.CommandContext(ctx, "/bin/bash", entry, "--entry-version").Output()
	if err != nil || strings.TrimRight(string(data), "\n") != version {
		return &updateFailure{"entry_version_mismatch"}
	}
	command = exec.CommandContext(ctx, "/bin/bash", entry, "--version", "v"+version, "update", "--config", path)
	// TERM lets update.sh's EXIT trap restore containers and configuration.
	// Killing the process group first also interrupts any blocked Docker client.
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.Cancel = func() error {
		err := syscall.Kill(-command.Process.Pid, syscall.SIGTERM)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	command.WaitDelay = 120 * time.Second
	jobID, _ := ctx.Value(updateContextKey{}).(string)
	if !validReceipt(jobID) {
		return &updateFailure{"update_failed"}
	}
	command.Env = append(os.Environ(), "TP_HOST_UPDATE_JOB="+jobID)
	for key, value := range s.Config.Environment {
		command.Env = append(command.Env, key+"="+value)
	}
	command.Stdout, command.Stderr = os.Stdout, os.Stderr
	if err = command.Run(); err != nil {
		return &updateFailure{"update_failed"}
	}
	return nil
}
