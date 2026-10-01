package service

import (
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
			nodeLifecycle.Lock()
			defer nodeLifecycle.Unlock()
			server, err := dao.SelectNodeServerForRemoval(id)
			if err != nil {
				return nil, err
			}
			if server.Ip == nil || server.GrpcPort == nil || *server.GrpcPort >= 65535 || server.GrpcTLSServerName == nil {
				return nil, fmt.Errorf("invalid node server uninstall endpoint")
			}
			if err = dao.BeginNodeServerRemoval(id); err != nil {
				return nil, err
			}
			return server, nil
		},
		remove: core.RemoveHost,
		complete: func(cleanup dao.RemovalCleanup) error {
			nodeLifecycle.Lock()
			defer nodeLifecycle.Unlock()
			return dao.CompleteNodeServerRemoval(cleanup)
		},
		finalize: func(cleanup dao.RemovalCleanup) { _ = finishRemovalCleanup(cleanup) },
	})
}

// These operations isolate the remote acknowledgement boundary: local records
// can be removed only after the target reports a successful uninstall.
type nodeUninstallOperations struct {
	begin    func(uint) (*model.NodeServer, error)
	remove   func(string, uint, core.NodeTransport, core.HostRemoval) (*core.HostRemoval, error)
	complete func(dao.RemovalCleanup) error
	finalize func(dao.RemovalCleanup)
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
