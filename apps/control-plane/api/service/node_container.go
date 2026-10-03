package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"trojan-panel/core"
	"trojan-panel/dao"
	"trojan-panel/model"
	"trojan-panel/model/constant"
)

// Serialize admission until either the database kernel items or the host job
// have been persisted. Remote workers do not retain this lock.
var nodeMaintenanceAdmission sync.Mutex

type NodeContainerInventoryVo struct {
	*core.HostContainerInventory
	TargetVersion string `json:"targetVersion"`
}

type nodeContainerOperations struct {
	authorize    func(string) (bool, error)
	server       func(uint) (*model.NodeServer, error)
	activeKernel func(uint) (bool, error)
	inventory    func(context.Context, *model.NodeServer) (*core.HostContainerInventory, error)
	update       func(context.Context, *model.NodeServer, string) (*core.HostContainerJob, error)
}

func containerOperations() nodeContainerOperations {
	return nodeContainerOperations{
		authorize: IsCurrentSysAdmin,
		server: func(id uint) (*model.NodeServer, error) {
			return dao.SelectNodeServer(map[string]interface{}{"id": id})
		},
		activeKernel: dao.NodeHasActiveKernelTask,
		inventory:    fetchNodeContainerInventory,
		update: func(ctx context.Context, server *model.NodeServer, version string) (*core.HostContainerJob, error) {
			return core.UpdateHostContainer(ctx, *server.Ip, *server.GrpcPort+1, nodeTransport(server), *server.Id, version)
		},
	}
}

func fetchNodeContainerInventory(ctx context.Context, server *model.NodeServer) (*core.HostContainerInventory, error) {
	return core.GetHostContainerInventory(ctx, *server.Ip, *server.GrpcPort+1, nodeTransport(server), *server.Id)
}

func GetNodeContainerInventory(ctx context.Context, token string, id uint) (*NodeContainerInventoryVo, error) {
	return nodeContainerInventory(ctx, token, id, containerOperations())
}

func nodeContainerInventory(ctx context.Context, token string, id uint, ops nodeContainerOperations) (*NodeContainerInventoryVo, error) {
	if err := authorizeContainerRequest(token, id, ops); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, core.HostContainerTimeout)
	defer cancel()
	nodeLifecycle.RLock()
	defer nodeLifecycle.RUnlock()
	server, err := ops.server(id)
	if err != nil {
		return nil, err
	}
	if err = validateContainerServer(server, id); err != nil {
		return nil, err
	}
	inventory, err := ops.inventory(ctx, server)
	if err != nil {
		return nil, err
	}
	return &NodeContainerInventoryVo{HostContainerInventory: inventory, TargetVersion: strings.TrimPrefix(constant.TrojanPanelVersion, "v")}, nil
}

func StartNodeContainerUpdate(ctx context.Context, token string, id uint) (*core.HostContainerJob, error) {
	return startNodeContainerUpdate(ctx, token, id, containerOperations())
}

func startNodeContainerUpdate(ctx context.Context, token string, id uint, ops nodeContainerOperations) (*core.HostContainerJob, error) {
	if err := authorizeContainerRequest(token, id, ops); err != nil {
		return nil, err
	}
	if !nodeMaintenanceAdmission.TryLock() {
		return nil, errors.New("another Node maintenance request is being admitted; retry when it finishes")
	}
	defer nodeMaintenanceAdmission.Unlock()
	ctx, cancel := context.WithTimeout(ctx, core.HostContainerTimeout)
	defer cancel()
	nodeLifecycle.RLock()
	defer nodeLifecycle.RUnlock()
	server, err := ops.server(id)
	if err != nil {
		return nil, err
	}
	if err = validateContainerServer(server, id); err != nil {
		return nil, err
	}
	active, err := ops.activeKernel(id)
	if err != nil {
		return nil, err
	}
	if active {
		return nil, errors.New("Node has an active kernel task; wait for it to finish before updating its container")
	}
	version := strings.TrimPrefix(constant.TrojanPanelVersion, "v")
	if !core.ValidContainerVersion(version) {
		return nil, errors.New("Web release is not a supported container target")
	}
	return ops.update(ctx, server, version)
}

func authorizeContainerRequest(token string, id uint, ops nodeContainerOperations) error {
	if id == 0 {
		return errors.New(constant.ValidateFailed)
	}
	allowed, err := ops.authorize(token)
	if err != nil {
		return err
	}
	if !allowed {
		return errors.New(constant.ForbiddenError)
	}
	return nil
}

func validateContainerServer(server *model.NodeServer, id uint) error {
	if server == nil || server.Id == nil || *server.Id != id || server.Ip == nil || *server.Ip == "" || server.GrpcPort == nil || *server.GrpcPort == 0 || *server.GrpcPort >= 65535 {
		return errors.New("invalid registered Node maintenance endpoint")
	}
	if server.Removing != nil && *server.Removing != 0 {
		return errors.New("node server is being removed")
	}
	if nodeTransport(server).Mode != "mtls" || server.GrpcTLSServerName == nil || *server.GrpcTLSServerName == "" {
		return core.ErrHostContainerUnsupported
	}
	return nil
}

func checkNodeContainerIdle(ctx context.Context, server *model.NodeServer, fetch func(context.Context, *model.NodeServer) (*core.HostContainerInventory, error)) error {
	if server.Removing != nil && *server.Removing != 0 {
		return errors.New("node server is being removed")
	}
	return checkNodeContainerJobIdle(ctx, server, fetch)
}

func checkNodeContainerJobIdle(ctx context.Context, server *model.NodeServer, fetch func(context.Context, *model.NodeServer) (*core.HostContainerInventory, error)) error {
	// Legacy nodes cannot start a container update through this API.
	if nodeTransport(server).Mode != "mtls" {
		return nil
	}
	inventory, err := fetch(ctx, server)
	if errors.Is(err, core.ErrHostContainerUnsupported) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("cannot verify Node container update state: %w", err)
	}
	if inventory.Job != nil && (inventory.Job.Status == "queued" || inventory.Job.Status == "running") {
		return core.ErrHostContainerUpdateActive
	}
	return nil
}
