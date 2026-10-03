package kernel

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestKernelStartRefusesActiveContainerUpdate(t *testing.T) {
	for _, status := range []string{"queued", "running"} {
		for _, action := range []Action{ActionInstall, ActionRollback} {
			t.Run(status+"/"+string(action), func(t *testing.T) {
				directory := t.TempDir()
				manager, err := newManager(managerConfig{RuntimeDir: directory, GOOS: "linux", GOARCH: "amd64"})
				if err != nil {
					t.Fatal(err)
				}
				if err = os.WriteFile(filepath.Join(directory, "container-update.json"), []byte(`{"status":"`+status+`"}`), 0600); err != nil {
					t.Fatal(err)
				}
				_, err = manager.Start(context.Background(), OperationRequest{IdempotencyKey: "test", Kernel: KernelXray, Version: "v1.2.3", Channel: ChannelStable, Action: action})
				if err == nil || !strings.Contains(err.Error(), "container update is active") {
					t.Fatalf("expected update conflict, got %v", err)
				}
				files, err := os.ReadDir(filepath.Join(directory, "operations"))
				if err != nil || len(files) != 0 {
					t.Fatalf("rejected operation was persisted: %v, %v", files, err)
				}
			})
		}
	}
}

func TestKernelAdmissionRespectsHostLockAndCancellation(t *testing.T) {
	directory := t.TempDir()
	manager, err := newManager(managerConfig{RuntimeDir: directory})
	if err != nil {
		t.Fatal(err)
	}
	lock, err := os.OpenFile(filepath.Join(directory, "maintenance.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		t.Fatal(err)
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err = manager.lockMaintenance(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected cancelled host lock wait, got %v", err)
	}
}

func TestKernelAdmissionRecoversAfterCompletedContainerUpdate(t *testing.T) {
	for _, status := range []string{"succeeded", "failed"} {
		t.Run(status, func(t *testing.T) {
			directory := t.TempDir()
			manager, err := newManager(managerConfig{RuntimeDir: directory})
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(filepath.Join(directory, "container-update.json"), []byte(`{"status":"`+status+`"}`), 0600); err != nil {
				t.Fatal(err)
			}
			unlock, err := manager.lockMaintenance(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			unlock()
		})
	}
}

func TestKernelAdmissionFailsClosedOnInvalidUpdateState(t *testing.T) {
	for _, data := range []string{`broken`, `{"status":"unknown"}`, `{"status":"succeeded"}{}`} {
		directory := t.TempDir()
		manager, err := newManager(managerConfig{RuntimeDir: directory})
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(directory, "container-update.json"), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		unlock, err := manager.lockMaintenance(context.Background())
		if err == nil {
			unlock()
			t.Fatalf("invalid update state accepted: %s", data)
		}
	}
}
