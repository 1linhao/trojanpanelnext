package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
	"trojan-panel/core"
	"trojan-panel/model"
	"trojan-panel/model/constant"
	"trojan-panel/model/dto"
	"trojan-panel/model/vo"
)

func TestNodeContainerUpdateCannotBypassCurrentRoleOrActiveKernel(t *testing.T) {
	id, port, removing := uint(7), uint(8100), uint(0)
	ip, mode, name := "192.0.2.7", "mtls", "node.example.test"
	calls := 0
	ops := nodeContainerOperations{
		authorize: func(string) (bool, error) { return false, nil },
		server: func(uint) (*model.NodeServer, error) {
			return &model.NodeServer{Id: &id, Ip: &ip, GrpcPort: &port, Removing: &removing, GrpcTLSMode: &mode, GrpcTLSServerName: &name}, nil
		},
		activeKernel: func(uint) (bool, error) { return true, nil },
		update: func(context.Context, *model.NodeServer, string) (*core.HostContainerJob, error) {
			calls++
			return nil, errors.New("unexpected transport")
		},
	}
	if _, err := startNodeContainerUpdate(context.Background(), "stale-jwt", id, ops); err == nil || calls != 0 {
		t.Fatal("demoted identity reached update")
	}
	ops.authorize = func(string) (bool, error) { return true, nil }
	if _, err := startNodeContainerUpdate(context.Background(), "current-jwt", id, ops); err == nil || calls != 0 {
		t.Fatal("active kernel reached update")
	}
}

func containerServiceFixture() nodeContainerOperations {
	id, port, removing := uint(7), uint(8100), uint(0)
	ip, mode, name := "192.0.2.7", "mtls", "node.example.test"
	return nodeContainerOperations{
		authorize: func(string) (bool, error) { return true, nil },
		server: func(uint) (*model.NodeServer, error) {
			return &model.NodeServer{Id: &id, Ip: &ip, GrpcPort: &port, Removing: &removing, GrpcTLSMode: &mode, GrpcTLSServerName: &name}, nil
		},
		activeKernel: func(uint) (bool, error) { return false, nil },
		inventory: func(context.Context, *model.NodeServer) (*core.HostContainerInventory, error) {
			return &core.HostContainerInventory{NodeID: 7, CurrentVersion: "1.0.2-rc.11", Image: "registered image", UpdateSupported: true}, nil
		},
	}
}

func TestNodeContainerUpdateUsesRegisteredEndpointAndWebTarget(t *testing.T) {
	ops := containerServiceFixture()
	target := strings.TrimPrefix(constant.TrojanPanelVersion, "v")
	ops.update = func(ctx context.Context, server *model.NodeServer, version string) (*core.HostContainerJob, error) {
		if *server.Id != 7 || *server.Ip != "192.0.2.7" || *server.GrpcPort != 8100 || nodeTransport(server).ServerName != "node.example.test" || version != target {
			t.Fatal("update did not use registered identity and Web release")
		}
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > core.HostContainerTimeout {
			t.Fatal("update request has no bounded deadline")
		}
		return &core.HostContainerJob{ID: "existing-job", FromVersion: "1.0.2-rc.11", TargetVersion: version, Status: "queued"}, nil
	}
	job, err := startNodeContainerUpdate(context.Background(), "current-jwt", 7, ops)
	if err != nil || job.TargetVersion != target {
		t.Fatalf("update: %+v %v", job, err)
	}
	inventory, err := nodeContainerInventory(context.Background(), "current-jwt", 7, ops)
	if err != nil || inventory.TargetVersion != target || inventory.NodeID != 7 {
		t.Fatalf("inventory: %+v %v", inventory, err)
	}
	for _, scenario := range []string{"removing", "legacy", "missing", "bad_port"} {
		t.Run(scenario, func(t *testing.T) {
			selected := containerServiceFixture()
			server, _ := selected.server(7)
			switch scenario {
			case "removing":
				value := uint(1)
				server.Removing = &value
			case "legacy":
				value := "legacy"
				server.GrpcTLSMode = &value
			case "missing":
				server = nil
			case "bad_port":
				value := uint(65535)
				server.GrpcPort = &value
			}
			selected.server = func(uint) (*model.NodeServer, error) { return server, nil }
			selected.update = func(context.Context, *model.NodeServer, string) (*core.HostContainerJob, error) {
				t.Fatal("unsafe server reached transport")
				return nil, nil
			}
			if _, err := startNodeContainerUpdate(context.Background(), "current-jwt", 7, selected); err == nil {
				t.Fatal("unsafe server accepted")
			}
			if _, err := nodeContainerInventory(context.Background(), "current-jwt", 7, selected); err == nil {
				t.Fatal("unsafe inventory server accepted")
			}
		})
	}
}

func TestNodeMaintenanceAdmissionCoordinatesKernelAndRemoval(t *testing.T) {
	ops := containerServiceFixture()
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	ops.update = func(context.Context, *model.NodeServer, string) (*core.HostContainerJob, error) {
		close(entered)
		<-release
		return &core.HostContainerJob{ID: "job", Status: "queued"}, nil
	}
	go func() { _, err := startNodeContainerUpdate(context.Background(), "current-jwt", 7, ops); done <- err }()
	<-entered
	if _, err := startNodeContainerUpdate(context.Background(), "current-jwt", 7, ops); err == nil {
		t.Error("concurrent update was admitted")
	}
	if _, err := CreateKernelTask(dto.KernelTaskCreateDto{}, vo.AccountVo{}, ""); err == nil {
		t.Error("kernel create overlapped container submission")
	}
	if err := RetryKernelTask(dto.KernelTaskRetryDto{}, ""); err == nil {
		t.Error("kernel retry overlapped container submission")
	}
	if nodeLifecycle.TryLock() {
		nodeLifecycle.Unlock()
		t.Error("registration removal overlapped update transport")
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestKernelAdmissionChecksContainerJobAndPreservesOldNodes(t *testing.T) {
	ops := containerServiceFixture()
	server, _ := ops.server(7)
	for _, status := range []string{"queued", "running", "succeeded", "failed", ""} {
		fetch := func(context.Context, *model.NodeServer) (*core.HostContainerInventory, error) {
			inventory := &core.HostContainerInventory{}
			if status != "" {
				inventory.Job = &core.HostContainerJob{Status: status}
			}
			return inventory, nil
		}
		err := checkNodeContainerIdle(context.Background(), server, fetch)
		if (err != nil) != (status == "queued" || status == "running") {
			t.Fatalf("job status %q admission: %v", status, err)
		}
	}
	if err := checkNodeContainerIdle(context.Background(), server, func(context.Context, *model.NodeServer) (*core.HostContainerInventory, error) {
		return nil, core.ErrHostContainerUnsupported
	}); err != nil {
		t.Fatal("old host blocks existing kernel feature")
	}
	if err := checkNodeContainerIdle(context.Background(), server, func(context.Context, *model.NodeServer) (*core.HostContainerInventory, error) {
		return nil, errors.New("offline")
	}); err == nil {
		t.Fatal("offline maintenance state was treated as idle")
	}
	mode := "legacy"
	server.GrpcTLSMode = &mode
	if err := checkNodeContainerIdle(context.Background(), server, func(context.Context, *model.NodeServer) (*core.HostContainerInventory, error) {
		t.Fatal("legacy node queried unsupported host endpoint")
		return nil, nil
	}); err != nil {
		t.Fatal(err)
	}
}
