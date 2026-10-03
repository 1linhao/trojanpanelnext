package hostagent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const oldAgentVersion = "1.0.2-rc.11"
const targetAgentVersion = "1.0.2-rc.12"

func containerFixture(t *testing.T) (*Server, *http.Client, *atomic.Int32) {
	t.Helper()
	s, client := setup(t)
	s.Config.OriginalConfig = filepath.Join(s.Directory, "original node.yaml")
	if err := os.WriteFile(s.Config.OriginalConfig, []byte("trojanpanelnext:\n  purpose: node\n"), 0600); err != nil {
		t.Fatal(err)
	}
	s.InspectImage = func(context.Context) (string, error) { return agentRepository + oldAgentVersion, nil }
	s.WorkerActive = func(context.Context, string) (bool, error) { return true, nil }
	count := &atomic.Int32{}
	s.StartUpdate = func(context.Context, string) error { count.Add(1); return nil }
	return s, client, count
}

func containerPost(t *testing.T, client *http.Client, url, body string) (int, []byte) {
	t.Helper()
	response, err := client.Post(url, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var buffer bytes.Buffer
	if _, err = buffer.ReadFrom(response.Body); err != nil {
		t.Fatal(err)
	}
	return response.StatusCode, buffer.Bytes()
}

func jobPost(t *testing.T, client *http.Client, url, version string) *UpdateJob {
	t.Helper()
	status, data := containerPost(t, client, url, `{"nodeId":7,"version":"`+version+`"}`)
	if status != 200 {
		t.Fatalf("update response %d %s", status, data)
	}
	var job UpdateJob
	if err := json.Unmarshal(data, &job); err != nil {
		t.Fatal(err)
	}
	var fields map[string]interface{}
	if err := json.Unmarshal(data, &fields); err != nil || len(fields) != 7 {
		t.Fatal("job exposes unexpected metadata", string(data))
	}
	return &job
}

func inventoryPost(t *testing.T, client *http.Client, url string) *ContainerInventory {
	t.Helper()
	status, data := containerPost(t, client, url, `{"nodeId":7}`)
	if status != 200 {
		t.Fatalf("inventory response %d %s", status, data)
	}
	var inventory ContainerInventory
	if err := json.Unmarshal(data, &inventory); err != nil {
		t.Fatal(err)
	}
	return &inventory
}

func TestContainerInventoryRequiresMTLSAndRegisteredNode(t *testing.T) {
	s, client, count := containerFixture(t)
	server := serve(t, s)
	inventory := inventoryPost(t, client, server.URL+"/container/inventory")
	if inventory.NodeID != 7 || inventory.CurrentVersion != oldAgentVersion || inventory.Image != agentRepository+oldAgentVersion || !inventory.UpdateSupported || inventory.Job != nil {
		t.Fatalf("wrong maintained container %#v", inventory)
	}
	for _, body := range []string{`{"nodeId":8}`, `{"nodeId":7,"command":"docker pull"}`, `{"nodeId":7,"version":""}`, `{"nodeId":7} {}`, `{"nodeId":7,"extra":"` + strings.Repeat("x", 5000) + `"}`} {
		status, _ := containerPost(t, client, server.URL+"/container/inventory", body)
		if status != http.StatusBadRequest {
			t.Fatalf("invalid inventory request accepted: %d", status)
		}
	}
	for _, route := range []string{"/container/inventory", "/container/update"} {
		recorder := httptest.NewRecorder()
		s.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, route, strings.NewReader(`{"nodeId":7,"version":"1.0.2-rc.12"}`)))
		if recorder.Code != 400 {
			t.Fatal("unverified mTLS request accepted", recorder.Code)
		}
		response, err := client.Get(server.URL + route)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != 400 {
			t.Fatal("GET accepted", response.StatusCode)
		}
	}
	transport := client.Transport.(*http.Transport).Clone()
	transport.TLSClientConfig.Certificates = nil
	unauthenticated := &http.Client{Transport: transport}
	response, err := unauthenticated.Post(server.URL+"/container/update", "application/json", strings.NewReader(`{"nodeId":7,"version":"1.0.2-rc.12"}`))
	if err == nil {
		response.Body.Close()
		t.Fatal("missing client certificate accepted")
	}
	if count.Load() != 0 {
		t.Fatal("invalid requests started a worker")
	}
}

func TestContainerUpdateIsDurableConcurrentAndIndependentOfMaintenanceRestart(t *testing.T) {
	s, client, starts := containerFixture(t)
	server := serve(t, s)
	first := jobPost(t, client, server.URL+"/container/update", targetAgentVersion)
	if first.Status != "queued" || first.FromVersion != oldAgentVersion || first.TargetVersion != targetAgentVersion || !validReceipt(first.ID) || first.StartedAt != "" || first.FinishedAt != "" {
		t.Fatalf("wrong queued job %#v", first)
	}
	info, err := os.Stat(filepath.Join(s.Directory, "update.json"))
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("job is not privately persisted", err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			repeat := jobPost(t, client, server.URL+"/container/update", targetAgentVersion)
			if repeat.ID != first.ID {
				t.Error("duplicate update created another job")
			}
		}()
	}
	wg.Wait()
	if starts.Load() != 1 {
		t.Fatal("duplicate workers launched", starts.Load())
	}
	status, _ := containerPost(t, client, server.URL+"/container/update", `{"nodeId":7,"version":"1.0.2-rc.13"}`)
	if status != 409 {
		t.Fatal("conflicting update accepted", status)
	}
	s.Execute = func(context.Context, bool) error { t.Error("remove executed during update"); return nil }
	if _, status := post(t, client, server.URL+"/remove", Request{NodeID: 7}); status != 409 {
		t.Fatal("removal admitted during update", status)
	}
	worker, err := New(s.Config, s.Directory)
	if err != nil {
		t.Fatal(err)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	worker.ExecuteUpdate = func(context.Context, string) error { close(entered); <-release; return nil }
	finished := make(chan error, 1)
	go func() { finished <- worker.RunUpdate(context.Background(), first.ID) }()
	<-entered
	server.Close()
	client.CloseIdleConnections()
	restarted, err := New(s.Config, s.Directory)
	if err != nil {
		t.Fatal(err)
	}
	restarted.InspectImage = s.InspectImage
	restarted.WorkerActive = s.WorkerActive
	restarted.StartUpdate = s.StartUpdate
	if err = restarted.RecoverUpdate(context.Background()); err != nil {
		t.Fatal(err)
	}
	newServer := serve(t, restarted)
	running := inventoryPost(t, client, newServer.URL+"/container/inventory").Job
	if running == nil || running.ID != first.ID || running.Status != "running" || running.StartedAt == "" || running.FinishedAt != "" {
		t.Fatal("running worker lost across maintenance restart", running)
	}
	if err = worker.RunUpdate(context.Background(), first.ID); err == nil {
		t.Fatal("running worker was executed twice")
	}
	close(release)
	if err = <-finished; err != nil {
		t.Fatal(err)
	}
	completed := inventoryPost(t, client, newServer.URL+"/container/inventory").Job
	if completed.Status != "succeeded" || completed.Error != "" {
		t.Fatal("successful worker not recorded", completed)
	}
	if _, err = time.Parse(time.RFC3339, completed.FinishedAt); err != nil {
		t.Fatal(err)
	}
	markerData, err := os.ReadFile(filepath.Join(s.runtimeDirectory(), "container-update.json"))
	if err != nil || !bytes.Contains(markerData, []byte(`"status":"succeeded"`)) {
		t.Fatal("runtime admission marker stayed active", err)
	}
	// A terminal job does not prevent a same-version repair request.
	restarted.InspectImage = func(context.Context) (string, error) { return agentRepository + targetAgentVersion, nil }
	repair := jobPost(t, client, newServer.URL+"/container/update", targetAgentVersion)
	if repair.ID == first.ID || repair.FromVersion != targetAgentVersion || repair.Status != "queued" {
		t.Fatal("same-version repair was not admitted", repair)
	}
}

func TestContainerWorkerFailureRecoveryAndStartFailureClearAdmission(t *testing.T) {
	for _, mode := range []string{"execution_failed", "queued_interrupted", "running_interrupted", "start_failed"} {
		t.Run(mode, func(t *testing.T) {
			s, client, _ := containerFixture(t)
			if mode == "start_failed" {
				s.StartUpdate = func(context.Context, string) error { return errors.New("private systemctl path and password") }
			}
			server := serve(t, s)
			job := jobPost(t, client, server.URL+"/container/update", targetAgentVersion)
			switch mode {
			case "execution_failed":
				s.ExecuteUpdate = func(context.Context, string) error { return errors.New("secret shell output /root/private password") }
				if err := s.RunUpdate(context.Background(), job.ID); err != nil {
					t.Fatal(err)
				}
			case "running_interrupted":
				unlock, err := s.lockMaintenance(context.Background())
				if err != nil {
					t.Fatal(err)
				}
				job.Status, job.StartedAt = "running", time.Now().UTC().Format(time.RFC3339)
				err = s.saveUpdate(job)
				unlock()
				if err != nil {
					t.Fatal(err)
				}
			}
			restarted, err := New(s.Config, s.Directory)
			if err != nil {
				t.Fatal(err)
			}
			restarted.InspectImage = s.InspectImage
			restarted.WorkerActive = func(context.Context, string) (bool, error) { return false, nil }
			if err = restarted.RecoverUpdate(context.Background()); err != nil {
				t.Fatal(err)
			}
			inventory := inventoryPost(t, client, serve(t, restarted).URL+"/container/inventory")
			want := "interrupted"
			if mode == "execution_failed" {
				want = "update_failed"
			}
			if mode == "start_failed" {
				want = "worker_start_failed"
			}
			if inventory.Job == nil || inventory.Job.Status != "failed" || inventory.Job.Error != want || inventory.Job.FinishedAt == "" {
				t.Fatal("failure state lost or unsafe", inventory.Job)
			}
			marker, err := os.ReadFile(filepath.Join(s.runtimeDirectory(), "container-update.json"))
			if err != nil || !bytes.Contains(marker, []byte(`"status":"failed"`)) {
				t.Fatal("failure kept admission locked", err)
			}
		})
	}
}

func TestContainerUpdateRejectsUnsafeVersionsIdentityAndKernelOperations(t *testing.T) {
	s, client, starts := containerFixture(t)
	server := serve(t, s)
	for _, body := range []string{`{"nodeId":8,"version":"1.0.2-rc.12"}`, `{"nodeId":7,"version":"1.0.2-rc.12","force":true}`, `{"nodeId":7,"version":"1.0.2-rc.12"} {}`, `{"nodeId":7,"version":"` + strings.Repeat("1", 5000) + `"}`} {
		status, _ := containerPost(t, client, server.URL+"/container/update", body)
		if status != 400 {
			t.Fatal("invalid update accepted", status)
		}
	}
	for _, version := range []string{"v1.0.2-rc.12", "latest", "1", "1.0.2-beta.1", "1.0.2-rc.01", "1.00.2", "1.0.2;id", "../../v1.0.2", "0.9.0"} {
		status, _ := containerPost(t, client, server.URL+"/container/update", `{"nodeId":7,"version":"`+version+`"}`)
		if status != 400 {
			t.Fatal("invalid version accepted", version, status)
		}
	}
	status, _ := containerPost(t, client, server.URL+"/container/update", `{"nodeId":7,"version":"1.0.2-rc.10"}`)
	if status != 409 {
		t.Fatal("downgrade accepted", status)
	}
	for _, image := range []string{"ghcr.io/attacker/node:1.0.2-rc.11", agentRepository + "latest"} {
		s.InspectImage = func(context.Context) (string, error) { return image, nil }
		status, data := containerPost(t, client, server.URL+"/container/inventory", `{"nodeId":7}`)
		if status != 503 || bytes.Contains(data, []byte(image)) {
			t.Fatal("unmanaged image exposed or accepted", status, string(data))
		}
	}
	s.InspectImage = func(context.Context) (string, error) { return agentRepository + oldAgentVersion, nil }
	if err := os.MkdirAll(filepath.Join(s.runtimeDirectory(), "operations"), 0700); err != nil {
		t.Fatal(err)
	}
	operation := filepath.Join(s.runtimeDirectory(), "operations", "test.json")
	for _, stage := range []string{"queued", "downloading", "verifying", "preflight", "switching", "restarting", "observing", "rolling_back"} {
		if err := os.WriteFile(operation, []byte(`{"stage":"`+stage+`"}`), 0600); err != nil {
			t.Fatal(err)
		}
		status, _ := containerPost(t, client, server.URL+"/container/update", `{"nodeId":7,"version":"1.0.2-rc.12"}`)
		if status != 409 {
			t.Fatal("active kernel admitted container update", stage, status)
		}
	}
	if starts.Load() != 0 {
		t.Fatal("refused update launched worker")
	}
	if err := os.WriteFile(operation, []byte(`{"stage":"rolled_back"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if job := jobPost(t, client, server.URL+"/container/update", targetAgentVersion); job.Status != "queued" {
		t.Fatal(job)
	}
}

func TestContainerReleaseOrdering(t *testing.T) {
	for _, test := range []struct {
		from, target string
		allowed      bool
	}{{"1.0.2-rc.11", "1.0.2-rc.12", true}, {"1.0.2-rc.12", "1.0.2-rc.12", true}, {"1.0.2-rc.12", "1.0.2", true}, {"1.0.2", "1.0.2-rc.13", false}, {"1.0.3", "1.0.2", false}, {"1.0.2", "1.1.0-rc.1", true}} {
		if versionCanUpdate(test.from, test.target) != test.allowed {
			t.Fatalf("wrong release ordering %+v", test)
		}
	}
}

func TestContainerWorkerUsesFixedReleaseTransportAndAllowsTermRollback(t *testing.T) {
	for _, scenario := range []string{"success", "download_failed", "invalid_script", "wrong_version", "probe_failed", "update_failed", "term_rollback", "symlink_config"} {
		t.Run(scenario, func(t *testing.T) {
			s, _, _ := containerFixture(t)
			directory := t.TempDir()
			bin := filepath.Join(directory, "tools")
			temporary := filepath.Join(directory, "downloads with spaces")
			for _, path := range []string{bin, temporary} {
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			}
			entry := filepath.Join(directory, "fixture-entry.sh")
			curlArgs := filepath.Join(directory, "curl-args")
			updateArgs := filepath.Join(directory, "update-args")
			entered := filepath.Join(directory, "entered")
			rolledBack := filepath.Join(directory, "rolled-back")
			write := func(path, body string) {
				t.Helper()
				if err := os.WriteFile(path, []byte(body), 0700); err != nil {
					t.Fatal(err)
				}
			}
			write(filepath.Join(bin, "curl"), `#!/usr/bin/env bash
printf '%s\0' "$@" > "$TEST_CURL_ARGS"
[[ "$TEST_DOWNLOAD_FAIL" == 0 ]] || exit 22
while (($#)); do
  if [[ "$1" == -o ]]; then cp -- "$TEST_ENTRY" "$2"; exit; fi
  shift
done
exit 9
`)
			write(entry, `#!/usr/bin/env bash
if [[ "$1" == --entry-version ]]; then printf '%s\n' "$TEST_ENTRY_VERSION"; exit "$TEST_PROBE_EXIT"; fi
printf '%s\0' "$@" > "$TEST_UPDATE_ARGS"
[[ "$TP_HOST_UPDATE_JOB" == "$TEST_JOB" && "$CORE_CONTAINER" == fixture-agent ]] || exit 9
[[ "$(stat -c %a "$0")" == 600 && "$(stat -c %a "${0%/*}")" == 700 ]] || exit 9
if [[ "$TEST_TERM" == 1 ]]; then
  trap 'printf "restored\n" > "$TEST_ROLLED_BACK"; exit 143' TERM
  printf 'running\n' > "$TEST_ENTERED"
  while :; do sleep 1; done
fi
exit "$TEST_UPDATE_EXIT"
`)
			downloadFail, probeExit, updateExit, term, entryVersion := "0", "0", "0", "0", targetAgentVersion
			switch scenario {
			case "download_failed":
				downloadFail = "1"
			case "invalid_script":
				write(entry, "#!/usr/bin/env bash\nif (; then\n")
			case "wrong_version":
				entryVersion = oldAgentVersion
			case "probe_failed":
				probeExit = "1"
			case "update_failed":
				updateExit = "9"
			case "term_rollback":
				term = "1"
			case "symlink_config":
				target := s.Config.OriginalConfig + ".real"
				if err := os.Rename(s.Config.OriginalConfig, target); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, s.Config.OriginalConfig); err != nil {
					t.Fatal(err)
				}
			}
			jobID := strings.Repeat("a", 64)
			env := map[string]string{"PATH": bin + string(os.PathListSeparator) + os.Getenv("PATH"), "TMPDIR": temporary, "TEST_ENTRY": entry, "TEST_ENTRY_VERSION": entryVersion, "TEST_CURL_ARGS": curlArgs, "TEST_UPDATE_ARGS": updateArgs, "TEST_DOWNLOAD_FAIL": downloadFail, "TEST_PROBE_EXIT": probeExit, "TEST_UPDATE_EXIT": updateExit, "TEST_TERM": term, "TEST_ENTERED": entered, "TEST_ROLLED_BACK": rolledBack, "TEST_JOB": jobID}
			for key, value := range env {
				t.Setenv(key, value)
			}
			s.Config.Environment["CORE_CONTAINER"] = "fixture-agent"
			ctx, cancel := context.WithCancel(context.WithValue(context.Background(), updateContextKey{}, jobID))
			defer cancel()
			result := make(chan error, 1)
			go func() { result <- s.executeUpdate(ctx, targetAgentVersion) }()
			if scenario == "term_rollback" {
				waitFor(t, func() bool { _, err := os.Stat(entered); return err == nil })
				cancel()
			}
			err := <-result
			if scenario == "success" {
				if err != nil {
					t.Fatal(err)
				}
			} else {
				want := scenario
				if scenario == "probe_failed" {
					want = "entry_version_mismatch"
				}
				if scenario == "wrong_version" {
					want = "entry_version_mismatch"
				}
				if scenario == "invalid_script" {
					want = "entry_invalid"
				}
				if scenario == "term_rollback" || scenario == "symlink_config" {
					want = "update_failed"
				}
				if err == nil || err.Error() != want {
					t.Fatalf("unsafe or wrong error: %v want %s", err, want)
				}
			}
			if scenario == "term_rollback" {
				if data, err := os.ReadFile(rolledBack); err != nil || string(data) != "restored\n" {
					t.Fatal("cancellation killed update before rollback", err)
				}
			}
			updateData, _ := os.ReadFile(updateArgs)
			if scenario == "success" || scenario == "update_failed" || scenario == "term_rollback" {
				want := []string{"--version", "v" + targetAgentVersion, "update", "--config", s.Config.OriginalConfig}
				got := strings.Split(strings.TrimSuffix(string(updateData), "\x00"), "\x00")
				if len(got) != len(want) {
					t.Fatal("wrong update arguments", got)
				}
				for i := range want {
					if got[i] != want[i] {
						t.Fatal("worker changed authorization", got)
					}
				}
			} else if len(updateData) != 0 {
				t.Fatal("failed validation executed deployment", string(updateData))
			}
			curlData, _ := os.ReadFile(curlArgs)
			if scenario != "symlink_config" {
				args := strings.Split(strings.TrimSuffix(string(curlData), "\x00"), "\x00")
				want := []string{"--proto", "=https", "--proto-redir", "=https", "--connect-timeout", "10", "--max-time", "60", "--max-filesize", "5242880", "https://raw.githubusercontent.com/1linhao/trojanpanelnext/v" + targetAgentVersion + "/scripts/tp.sh"}
				for _, argument := range want {
					found := false
					for _, got := range args {
						if got == argument {
							found = true
						}
					}
					if !found {
						t.Fatal("missing fixed HTTPS download boundary", argument, args)
					}
				}
			} else if len(curlData) != 0 {
				t.Fatal("unsafe configuration reached download")
			}
			entries, err := os.ReadDir(temporary)
			if err != nil || len(entries) != 0 {
				t.Fatal("worker leaked downloaded entrypoint", entries, err)
			}
		})
	}
}

func TestContainerWorkerIsIndependentSystemdUnitAndDetectsMissingWorker(t *testing.T) {
	s, _, _ := containerFixture(t)
	directory := t.TempDir()
	log := filepath.Join(directory, "unit-args")
	t.Setenv("PATH", directory+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TEST_SYSTEMD_LOG", log)
	for name, body := range map[string]string{
		"systemd-run": "#!/usr/bin/env bash\nprintf '%s\\n' \"$@\" > \"$TEST_SYSTEMD_LOG\"\n",
		"systemctl":   "#!/usr/bin/env bash\nprintf '%s\\n' \"$TEST_SYSTEMD_STATE\"\nexit \"$TEST_SYSTEMD_EXIT\"\n",
	} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(body), 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.startUpdate(context.Background(), "../../some-command"); err == nil {
		t.Fatal("worker accepts command text")
	}
	if _, err := os.Stat(log); !os.IsNotExist(err) {
		t.Fatal("invalid ID reached systemd")
	}
	id := strings.Repeat("b", 64)
	if err := s.startUpdate(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	args := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(args) < 3 || strings.Join(args[len(args)-3:], "\n") != filepath.Join(Library, "tp-host-agent")+"\n--run-update\n"+id {
		t.Fatal("systemd worker accepts paths or shell input", args)
	}
	for _, flag := range []string{"--unit=" + updateUnit(id), "--collect", "--property=Type=exec", "--property=KillMode=control-group", "--property=TimeoutStopSec=180"} {
		if !strings.Contains(string(data), flag+"\n") {
			t.Fatal("missing independent systemd boundary", flag, string(data))
		}
	}
	for _, test := range []struct {
		state, exit   string
		alive, failed bool
	}{{"LoadState=loaded\nActiveState=active", "0", true, false}, {"LoadState=loaded\nActiveState=activating", "0", true, false}, {"LoadState=not-found\nActiveState=inactive", "1", false, false}, {"", "1", false, true}} {
		t.Setenv("TEST_SYSTEMD_STATE", test.state)
		t.Setenv("TEST_SYSTEMD_EXIT", test.exit)
		alive, err := s.workerActive(context.Background(), id)
		if alive != test.alive || (err != nil) != test.failed {
			t.Fatal("worker recovery probe changed", test, alive, err)
		}
	}
}

func TestCLIUpdateMarkerBlocksHostOperationsAndSurvivesStartupRecovery(t *testing.T) {
	for _, savedJob := range []bool{false, true} {
		t.Run(fmt.Sprintf("saved_job_%t", savedJob), func(t *testing.T) {
			s, client, starts := containerFixture(t)
			unlock, err := s.lockMaintenance(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if savedJob {
				job := &UpdateJob{ID: strings.Repeat("d", 64), FromVersion: oldAgentVersion, TargetVersion: targetAgentVersion, Status: "succeeded", StartedAt: time.Now().UTC().Format(time.RFC3339), FinishedAt: time.Now().UTC().Format(time.RFC3339)}
				if err = s.saveUpdate(job); err != nil {
					t.Fatal(err)
				}
			}
			marker := []byte(`{"owner":"cli","token":"AbCd12","nodeId":7,"status":"running"}`)
			if err = atomicWrite(filepath.Join(s.runtimeDirectory(), "container-update.json"), marker); err != nil {
				t.Fatal(err)
			}
			unlock()
			server := serve(t, s)
			status, _ := containerPost(t, client, server.URL+"/container/inventory", `{"nodeId":7}`)
			if status != 503 {
				t.Fatal("CLI update exposed stale inventory", status)
			}
			status, body := containerPost(t, client, server.URL+"/container/update", `{"nodeId":7,"version":"1.0.2-rc.12"}`)
			if status != 409 || string(body) != "container_update_active\n" {
				t.Fatal("CLI marker allowed host update", status, string(body))
			}
			s.Execute = func(context.Context, bool) error { t.Error("CLI marker allowed removal"); return nil }
			status, body = containerPost(t, client, server.URL+"/remove", `{"nodeId":7}`)
			if status != 409 || string(body) != "container_update_active\n" {
				t.Fatal("CLI marker allowed host removal", status, string(body))
			}
			if _, err = os.Stat(filepath.Join(s.Directory, "server.crt")); !os.IsNotExist(err) {
				t.Fatal("busy removal published TLS snapshot")
			}
			if err = s.RecoverUpdate(context.Background()); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(filepath.Join(s.runtimeDirectory(), "container-update.json"))
			if err != nil || !bytes.Equal(data, marker) {
				t.Fatal("host startup erased CLI admission marker", string(data), err)
			}
			if starts.Load() != 0 {
				t.Fatal("busy marker launched another worker")
			}
		})
	}
}

func TestHostRemovalRefusesProductLockBeforeTLSAndPreservesForeignLock(t *testing.T) {
	s, client, _ := containerFixture(t)
	server := serve(t, s)
	if err := os.Mkdir(s.ProductLockDirectory, 0700); err != nil {
		t.Fatal(err)
	}
	var executions atomic.Int32
	s.Execute = func(context.Context, bool) error { executions.Add(1); return nil }
	status, data := containerPost(t, client, server.URL+"/remove", `{"nodeId":7}`)
	if status != 409 || string(data) != "container_update_active\n" || executions.Load() != 0 {
		t.Fatal("CLI publication race was not refused", status, string(data))
	}
	if _, err := os.Stat(filepath.Join(s.Directory, "server.crt")); !os.IsNotExist(err) {
		t.Fatal("busy request published removal snapshot")
	}
	if err := os.Remove(s.ProductLockDirectory); err != nil {
		t.Fatal(err)
	}
	s.Execute = func(context.Context, bool) error {
		if _, err := os.Stat(s.ProductLockDirectory); err != nil {
			t.Fatal("host removal did not reserve CLI product lock", err)
		}
		if err := os.Remove(s.ProductLockDirectory); err != nil {
			t.Fatal(err)
		}
		return os.Mkdir(s.ProductLockDirectory, 0700)
	}
	status, _ = containerPost(t, client, server.URL+"/remove", `{"nodeId":7}`)
	if status != 200 {
		t.Fatal("admitted host removal failed", status)
	}
	if _, err := os.Stat(s.ProductLockDirectory); err != nil {
		t.Fatal("host removal erased a replacement owner's lock", err)
	}
}
