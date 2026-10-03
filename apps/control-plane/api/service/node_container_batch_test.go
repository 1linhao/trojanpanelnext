package service

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
	"trojan-panel/core"
	"trojan-panel/model"
)

func TestKernelContainerPreflightGivesSlowHealthyNodesIndependentBudgets(t *testing.T) {
	var servers []*model.NodeServer
	for index := uint(7); index < 10; index++ {
		server, _ := containerServiceFixture().server(7)
		id := index
		server.Id = &id
		servers = append(servers, server)
	}
	var completed atomic.Int32
	fetch := func(ctx context.Context, server *model.NodeServer) (*core.HostContainerInventory, error) {
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > core.HostContainerTimeout+50*time.Millisecond {
			t.Error("individual Node query was not limited to 5 seconds")
		}
		timer := time.NewTimer(2 * time.Second)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timer.C:
			completed.Add(1)
			return &core.HostContainerInventory{NodeID: *server.Id}, nil
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), kernelAdmissionTimeout)
	defer cancel()
	if err := checkNodeContainersIdle(ctx, servers, fetch); err != nil {
		t.Fatalf("three healthy Nodes taking 2 seconds each rejected: %v", err)
	}
	if completed.Load() != 3 {
		t.Fatal("not every healthy Node was checked")
	}
}

func TestKernelContainerPreflightBoundsConcurrencyAndCancelsAllWorkers(t *testing.T) {
	var servers []*model.NodeServer
	for id := uint(1); id <= 20; id++ {
		server, _ := containerServiceFixture().server(7)
		current := id
		server.Id = &current
		servers = append(servers, server)
	}
	var active, peak atomic.Int32
	fetch := func(ctx context.Context, server *model.NodeServer) (*core.HostContainerInventory, error) {
		count := active.Add(1)
		defer active.Add(-1)
		for previous := peak.Load(); count > previous; previous = peak.Load() {
			if peak.CompareAndSwap(previous, count) {
				break
			}
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(20 * time.Millisecond):
			return &core.HostContainerInventory{}, nil
		}
	}
	if err := checkNodeContainersIdle(context.Background(), servers, fetch); err != nil {
		t.Fatal(err)
	}
	if peak.Load() < 9 || peak.Load() > 16 {
		t.Fatalf("unbounded or sequential queries: peak=%d", peak.Load())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := checkNodeContainersIdle(ctx, servers, func(ctx context.Context, server *model.NodeServer) (*core.HostContainerInventory, error) {
		active.Add(1)
		defer active.Add(-1)
		<-ctx.Done()
		return nil, ctx.Err()
	}); err == nil || active.Load() != 0 {
		t.Fatalf("cancelled preflight kept workers or accepted a task: %v active=%d", err, active.Load())
	}
}
