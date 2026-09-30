package service

import (
	"fmt"
	"sync"
	"time"
	"trojan-panel/core"
	"trojan-panel/dao"
)

var nodeLifecycle sync.RWMutex
var removalFinalizer sync.Mutex

type NodeRemovalResult struct {
	CleanupPending bool `json:"cleanupPending"`
}

func DeleteNodeServerById(id *uint, purge bool) (*NodeRemovalResult, error) {
	nodeLifecycle.Lock()
	defer nodeLifecycle.Unlock()
	if _, err := core.HostRemovalCallbackURL(); err != nil {
		return nil, err
	}
	server, err := dao.SelectNodeServerForRemoval(*id)
	if err != nil {
		return nil, err
	}
	if server.GrpcPort == nil || *server.GrpcPort >= 65535 {
		return nil, fmt.Errorf("invalid node server gRPC port")
	}
	if err = dao.BeginNodeServerRemoval(*id); err != nil {
		return nil, err
	}
	result, err := core.RemoveHost(*server.Ip, *server.GrpcPort+1, nodeTransport(server), core.HostRemoval{NodeID: *id, Purge: purge})
	if err != nil {
		return nil, err
	}
	cleanup := dao.RemovalCleanup{NodeID: *id, IP: *server.Ip, Port: *server.GrpcPort + 1, ServerName: *server.GrpcTLSServerName, Purge: purge, Receipt: result.Receipt}
	if err = dao.CompleteNodeServerRemoval(cleanup); err != nil {
		return nil, err
	}
	// The outbox was committed together with local deletion. If the final ACK
	// fails, the background worker retries without losing the endpoint details.
	removalFinalizer.Lock()
	_ = finishRemovalCleanup(cleanup)
	removalFinalizer.Unlock()
	// Finalization is durable on the host after this ACK. Its callback clears the
	// outbox asynchronously, so the maintenance cleanup remains pending here.
	return &NodeRemovalResult{CleanupPending: true}, nil
}

func finishRemovalCleanup(item dao.RemovalCleanup) error {
	callbackURL, err := core.HostRemovalCallbackURL()
	if err != nil {
		return err
	}
	return core.FinalizeHostRemoval(item.IP, item.Port, core.NodeTransport{Mode: "mtls", ServerName: item.ServerName}, core.HostRemoval{NodeID: item.NodeID, Purge: item.Purge, Receipt: item.Receipt, CallbackURL: callbackURL})
}

func InitNodeRemovalFinalizer() {
	go func() {
		for {
			items, err := dao.PendingRemovalCleanups()
			if err == nil {
				for _, item := range items {
					_ = finishRemovalCleanup(item)
				}
			}
			time.Sleep(30 * time.Second)
		}
	}()
}
