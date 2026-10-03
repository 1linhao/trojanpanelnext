package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
	"trojan-panel/core"
	"trojan-panel/dao"
	"trojan-panel/model"
)

var nodeLifecycle sync.RWMutex
var nodeRemovalOperations sync.Map

type NodeRemovalResult struct {
	CleanupPending bool `json:"cleanupPending"`
}

// Keep lock identities stable even while a failed uninstall is retried. These
// entries contain no server endpoints, receipts or connection information.
func nodeRemovalMutex(id uint) *sync.Mutex {
	lock, _ := nodeRemovalOperations.LoadOrStore(id, &sync.Mutex{})
	return lock.(*sync.Mutex)
}

func withNodeRemovalOperation(id uint, operation func() error) error {
	lock := nodeRemovalMutex(id)
	if !lock.TryLock() {
		return fmt.Errorf("node server %d has a deletion or uninstall in progress; retry when it finishes", id)
	}
	defer lock.Unlock()
	return operation()
}

// DeleteNodeServerById is a Web-only action. No Node endpoint, certificate or
// callback configuration is read, so an offline Node can be forgotten.
func DeleteNodeServerById(id *uint) error {
	return deleteNodeServer(*id, dao.DeleteNodeServerRecords)
}

func deleteNodeServer(id uint, deleteRecords func(uint) error) error {
	return withNodeRemovalOperation(id, func() error {
		nodeLifecycle.Lock()
		defer nodeLifecycle.Unlock()
		// Redis has no per-Node connection cache. Its JWT, system, account
		// limits, blacklists and global operation locks must survive deletion.
		return deleteRecords(id)
	})
}

func UninstallNodeServerById(id *uint, purge bool) (*NodeRemovalResult, error) {
	return uninstallNodeServer(*id, purge, nodeUninstallOperations{
		begin: func(id uint) (*model.NodeServer, error) {
			if _, err := core.HostRemovalCallbackURL(); err != nil {
				return nil, err
			}
			return beginNodeServerUninstall(id, dao.SelectNodeServerForRemoval, fetchNodeContainerInventory, dao.BeginNodeServerRemoval)
		},
		remove: core.RemoveHost,
		cancelUnstarted: func(id uint) error {
			nodeLifecycle.Lock()
			defer nodeLifecycle.Unlock()
			return dao.CancelUnstartedNodeServerRemoval(id)
		},
		complete: func(cleanup dao.RemovalCleanup) error {
			nodeLifecycle.Lock()
			defer nodeLifecycle.Unlock()
			return dao.CompleteNodeServerRemoval(cleanup)
		},
		finalize: func(cleanup dao.RemovalCleanup) { _ = finishRemovalCleanup(cleanup) },
	})
}

func beginNodeServerUninstall(id uint, selectServer func(uint) (*model.NodeServer, error), fetch func(context.Context, *model.NodeServer) (*core.HostContainerInventory, error), begin func(uint) error) (*model.NodeServer, error) {
	if !nodeMaintenanceAdmission.TryLock() {
		return nil, errors.New("another Node maintenance request is being admitted; retry when it finishes")
	}
	defer nodeMaintenanceAdmission.Unlock()
	nodeLifecycle.RLock()
	server, err := selectServer(id)
	nodeLifecycle.RUnlock()
	if err != nil {
		return nil, err
	}
	if !validUninstallEndpoint(server, id) {
		return nil, errors.New("invalid node server uninstall endpoint")
	}
	ctx, cancel := context.WithTimeout(context.Background(), core.HostContainerTimeout)
	defer cancel()
	// A previous attempt may already have removed the container. Its saved
	// /remove receipt must remain reachable even when inventory is unavailable.
	if server.Removing == nil || *server.Removing == 0 {
		if err = checkNodeContainerJobIdle(ctx, server, fetch); err != nil {
			return nil, err
		}
	}
	// Recheck the snapshot after the remote preflight. Network IO does not hold
	// the lifecycle write lock, so another server can still be forgotten.
	nodeLifecycle.Lock()
	defer nodeLifecycle.Unlock()
	current, err := selectServer(id)
	if err != nil {
		return nil, err
	}
	if !validUninstallEndpoint(current, id) || *server.Ip != *current.Ip || *server.GrpcPort != *current.GrpcPort || *server.GrpcTLSServerName != *current.GrpcTLSServerName || nodeTransport(server).Mode != nodeTransport(current).Mode || !sameRemovalFlag(server.Removing, current.Removing) {
		return nil, errors.New("node server changed during uninstall preflight; retry")
	}
	// Preserve the state before this attempt marked removal, including when a
	// caller supplies a shared model instance at the DAO boundary.
	snapshot := *current
	if current.Removing != nil {
		flag := *current.Removing
		snapshot.Removing = &flag
	}
	if err = begin(id); err != nil {
		return nil, err
	}
	return &snapshot, nil
}

func validUninstallEndpoint(server *model.NodeServer, id uint) bool {
	return server != nil && server.Id != nil && *server.Id == id && server.Ip != nil && *server.Ip != "" && server.GrpcPort != nil && *server.GrpcPort > 0 && *server.GrpcPort < 65535 && server.GrpcTLSServerName != nil
}

func sameRemovalFlag(left, right *uint) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

// These operations isolate the remote acknowledgement boundary: local records
// can be removed only after the target reports a successful uninstall.
type nodeUninstallOperations struct {
	begin           func(uint) (*model.NodeServer, error)
	remove          func(string, uint, core.NodeTransport, core.HostRemoval) (*core.HostRemoval, error)
	cancelUnstarted func(uint) error
	complete        func(dao.RemovalCleanup) error
	finalize        func(dao.RemovalCleanup)
}

func uninstallNodeServer(id uint, purge bool, operations nodeUninstallOperations) (*NodeRemovalResult, error) {
	var result *NodeRemovalResult
	err := withNodeRemovalOperation(id, func() error {
		server, err := operations.begin(id)
		if err != nil {
			return err
		}
		// Only this server's operation mutex is held during the potentially
		// slow RPC. An offline target never blocks deleting a different Node.
		acknowledgement, err := operations.remove(*server.Ip, *server.GrpcPort+1, nodeTransport(server), core.HostRemoval{NodeID: id, Purge: purge})
		if err != nil {
			if errors.Is(err, core.ErrHostContainerUpdateActive) && server.Removing != nil && *server.Removing == 0 && operations.cancelUnstarted != nil {
				if resetErr := operations.cancelUnstarted(id); resetErr != nil {
					return fmt.Errorf("%w; removal marker was not reset: %v", err, resetErr)
				}
			}
			return err
		}
		cleanup := dao.RemovalCleanup{NodeID: id, IP: *server.Ip, Port: *server.GrpcPort + 1, ServerName: *server.GrpcTLSServerName, Purge: purge, Receipt: acknowledgement.Receipt}
		if err = operations.complete(cleanup); err != nil {
			return err
		}
		// The outbox is committed together with local deletion. A failed final
		// ACK is retried from its durable details; it is not a failed uninstall.
		operations.finalize(cleanup)
		result = &NodeRemovalResult{CleanupPending: true}
		return nil
	})
	return result, err
}

func finishRemovalCleanup(item dao.RemovalCleanup) error {
	// A worker may have read a receipt before a Web-only deletion discarded
	// it. Check again under this Node's operation mutex before contacting it.
	exists, err := dao.RemovalCleanupExists(item.NodeID, item.Receipt)
	if err != nil || !exists {
		return err
	}
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
					lock := nodeRemovalMutex(item.NodeID)
					if lock.TryLock() {
						_ = finishRemovalCleanup(item)
						lock.Unlock()
					}
				}
			}
			time.Sleep(30 * time.Second)
		}
	}()
}
