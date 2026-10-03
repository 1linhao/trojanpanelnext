package service

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"trojan-panel/core"
	"trojan-panel/dao"
	"trojan-panel/model"
)

func TestUninstallAdmissionRejectsActiveContainerBeforeBegin(t *testing.T) {
	server, _ := containerServiceFixture().server(7)
	beginCalls := 0
	for _, status := range []string{"queued", "running"} {
		_, err := beginNodeServerUninstall(7, func(uint) (*model.NodeServer, error) { return server, nil },
			func(context.Context, *model.NodeServer) (*core.HostContainerInventory, error) {
				return &core.HostContainerInventory{Job: &core.HostContainerJob{Status: status}}, nil
			}, func(uint) error { beginCalls++; return nil })
		if !errors.Is(err, core.ErrHostContainerUpdateActive) || beginCalls != 0 || *server.Removing != 0 {
			t.Fatalf("busy uninstall marked removal: %v begin=%d flag=%d", err, beginCalls, *server.Removing)
		}
	}
	if !nodeMaintenanceAdmission.TryLock() {
		t.Fatal("admission lock retained")
	}
	defer nodeMaintenanceAdmission.Unlock()
	if _, err := beginNodeServerUninstall(7, func(uint) (*model.NodeServer, error) { t.Error("competing uninstall read Node"); return server, nil }, nil, func(uint) error { t.Error("competing uninstall marked Node"); return nil }); err == nil {
		t.Fatal("uninstall overlapped another maintenance admission")
	}
}

func TestUninstallPreflightAllowsOldHostAndDetectsEndpointChange(t *testing.T) {
	for _, changed := range []bool{false, true} {
		server, _ := containerServiceFixture().server(7)
		lookups, beginCalls := 0, 0
		_, err := beginNodeServerUninstall(7, func(uint) (*model.NodeServer, error) {
			lookups++
			if changed && lookups == 2 {
				current := *server
				ip := "192.0.2.8"
				current.Ip = &ip
				return &current, nil
			}
			return server, nil
		}, func(context.Context, *model.NodeServer) (*core.HostContainerInventory, error) {
			return nil, core.ErrHostContainerUnsupported
		}, func(uint) error { beginCalls++; return nil })
		if changed {
			if err == nil || beginCalls != 0 {
				t.Fatal("changed uninstall endpoint was marked")
			}
		} else if err != nil || beginCalls != 1 {
			t.Fatalf("old host blocked existing uninstall: %v calls=%d", err, beginCalls)
		}
	}
}

func TestNodeUninstallAcknowledgementBoundary(t *testing.T) {
	id, port := uint(11), uint(8100)
	ip, serverName, mode := "203.0.113.11", "offline.example.test", "mtls"
	server := &model.NodeServer{Id: &id, Ip: &ip, GrpcPort: &port, GrpcTLSServerName: &serverName, GrpcTLSMode: &mode}
	for _, purge := range []bool{false, true} {
		for _, failure := range []string{"begin", "offline", "complete", ""} {
			t.Run(failure+map[bool]string{false: "_keep_data", true: "_purge"}[purge], func(t *testing.T) {
				var calls []string
				fixtureError := errors.New("fixture " + failure)
				operations := nodeUninstallOperations{
					begin: func(selected uint) (*model.NodeServer, error) {
						calls = append(calls, "begin")
						if selected != id {
							t.Fatal("wrong selected server")
						}
						if failure == "begin" {
							return nil, fixtureError
						}
						return server, nil
					},
					remove: func(target string, targetPort uint, transport core.NodeTransport, request core.HostRemoval) (*core.HostRemoval, error) {
						calls = append(calls, "remove")
						if target != ip || targetPort != port+1 || transport.Mode != mode || transport.ServerName != serverName || request.NodeID != id || request.Purge != purge {
							t.Fatal("uninstall targeted the wrong host or mode")
						}
						if failure == "offline" {
							return nil, fixtureError
						}
						return &core.HostRemoval{NodeID: id, Purge: purge, Success: true, Receipt: strings.Repeat("a", 64)}, nil
					},
					complete: func(cleanup dao.RemovalCleanup) error {
						calls = append(calls, "complete")
						if cleanup.NodeID != id || cleanup.Purge != purge || cleanup.Receipt != strings.Repeat("a", 64) {
							t.Fatal("wrong uninstall acknowledgement")
						}
						if failure == "complete" {
							return fixtureError
						}
						return nil
					},
					finalize: func(dao.RemovalCleanup) { calls = append(calls, "finalize") },
				}
				result, err := uninstallNodeServer(id, purge, operations)
				want := []string{"begin"}
				switch failure {
				case "offline":
					want = append(want, "remove")
				case "complete":
					want = append(want, "remove", "complete")
				case "":
					want = append(want, "remove", "complete", "finalize")
				}
				if !reflect.DeepEqual(calls, want) {
					t.Fatalf("calls=%v, want=%v", calls, want)
				}
				if failure != "" {
					if !errors.Is(err, fixtureError) || result != nil {
						t.Fatalf("failed uninstall was reported successful: %#v %v", result, err)
					}
				} else if err != nil || result == nil || !result.CleanupPending {
					t.Fatalf("acknowledged uninstall failed: %#v %v", result, err)
				}
			})
		}
	}
}

func TestOfflineUninstallDoesNotBlockOtherServerDeletion(t *testing.T) {
	id, otherID, port := uint(901), uint(902), uint(8100)
	ip, serverName, mode := "203.0.113.11", "offline.example.test", "mtls"
	server := &model.NodeServer{Id: &id, Ip: &ip, GrpcPort: &port, GrpcTLSServerName: &serverName, GrpcTLSMode: &mode}
	entered, release := make(chan struct{}), make(chan struct{})
	uninstallDone := make(chan error, 1)
	fixtureError := errors.New("target is offline")
	operations := nodeUninstallOperations{
		begin: func(uint) (*model.NodeServer, error) {
			// Match the production begin phase: lock only local DB work.
			nodeLifecycle.Lock()
			defer nodeLifecycle.Unlock()
			return server, nil
		},
		remove: func(string, uint, core.NodeTransport, core.HostRemoval) (*core.HostRemoval, error) {
			close(entered)
			<-release
			return nil, fixtureError
		},
		complete: func(dao.RemovalCleanup) error { t.Error("failed uninstall deleted records"); return nil },
		finalize: func(dao.RemovalCleanup) { t.Error("failed uninstall finalized the host") },
	}
	go func() { _, err := uninstallNodeServer(id, false, operations); uninstallDone <- err }()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("uninstall did not reach the remote boundary")
	}
	var once sync.Once
	defer once.Do(func() { close(release) })
	deleteDone := make(chan error, 1)
	go func() {
		deleteDone <- deleteNodeServer(otherID, func(selected uint) error {
			if selected != otherID {
				return errors.New("wrong local deletion ID")
			}
			return nil
		})
	}()
	select {
	case err := <-deleteDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("offline uninstall blocked another server's Web-only deletion")
	}
	if err := deleteNodeServer(id, func(uint) error { t.Error("busy server records changed"); return nil }); err == nil || !strings.Contains(err.Error(), "in progress") {
		t.Fatalf("same server deletion did not reject a busy uninstall: %v", err)
	}
	if _, err := uninstallNodeServer(id, true, operations); err == nil || !strings.Contains(err.Error(), "in progress") {
		t.Fatalf("same server allowed a second uninstall: %v", err)
	}
	once.Do(func() { close(release) })
	select {
	case err := <-uninstallDone:
		if !errors.Is(err, fixtureError) {
			t.Fatalf("offline failure lost: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("uninstall did not finish")
	}
	if err := deleteNodeServer(id, func(uint) error { return nil }); err != nil {
		t.Fatalf("failed uninstall prevented a later explicit Web-only deletion: %v", err)
	}
}

func TestHostFinalizationDoesNotBlockOtherServerDeletion(t *testing.T) {
	id, otherID, port := uint(903), uint(904), uint(8100)
	ip, serverName, mode := "203.0.113.13", "finalizing.example.test", "mtls"
	server := &model.NodeServer{Id: &id, Ip: &ip, GrpcPort: &port, GrpcTLSServerName: &serverName, GrpcTLSMode: &mode}
	entered, release := make(chan struct{}), make(chan struct{})
	finished := make(chan error, 1)
	var once sync.Once
	defer once.Do(func() { close(release) })
	operations := nodeUninstallOperations{
		begin: func(uint) (*model.NodeServer, error) { return server, nil },
		remove: func(string, uint, core.NodeTransport, core.HostRemoval) (*core.HostRemoval, error) {
			return &core.HostRemoval{NodeID: id, Success: true, Receipt: strings.Repeat("a", 64)}, nil
		},
		complete: func(dao.RemovalCleanup) error {
			nodeLifecycle.Lock()
			defer nodeLifecycle.Unlock()
			return nil
		},
		finalize: func(dao.RemovalCleanup) { close(entered); <-release },
	}
	go func() { _, err := uninstallNodeServer(id, false, operations); finished <- err }()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("uninstall did not reach the finalization boundary")
	}
	deleted := make(chan error, 1)
	go func() { deleted <- deleteNodeServer(otherID, func(uint) error { return nil }) }()
	select {
	case err := <-deleted:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("another host's finalization blocked Web-only deletion")
	}
	once.Do(func() { close(release) })
	select {
	case err := <-finished:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("finalization did not finish")
	}
}
