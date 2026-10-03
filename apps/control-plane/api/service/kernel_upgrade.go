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
	"trojan-panel/model/dto"
	"trojan-panel/model/vo"
)

var kernelNodeLocks sync.Map

const kernelAdmissionTimeout = 55 * time.Second
const kernelAdmissionConcurrency = 16

func GetNodeKernelInventory(token string, nodeServerId uint) (*core.KernelInventoryVo, error) {
	server, err := dao.SelectNodeServer(map[string]interface{}{"id": nodeServerId})
	if err != nil {
		return nil, err
	}
	return core.GetKernelInventory(token, *server.Ip, *server.GrpcPort, nodeTransport(server))
}

func CreateKernelTask(request dto.KernelTaskCreateDto, operator vo.AccountVo, token string) (*model.KernelUpgradeTask, error) {
	if !nodeMaintenanceAdmission.TryLock() {
		return nil, errors.New("another Node maintenance request is being admitted; retry when it finishes")
	}
	defer nodeMaintenanceAdmission.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), kernelAdmissionTimeout)
	defer cancel()
	seenNodes := make(map[uint]bool)
	var servers []*model.NodeServer
	for _, id := range request.NodeServerIds {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if seenNodes[id] {
			continue
		}
		seenNodes[id] = true
		server, err := selectKernelServerSnapshot(ctx, id)
		if err != nil {
			return nil, err
		}
		servers = append(servers, server)
	}
	if err := checkNodeContainersIdle(ctx, servers, fetchNodeContainerInventory); err != nil {
		return nil, err
	}
	if request.CanaryNodeServerId != 0 && !seenNodes[request.CanaryNodeServerId] {
		return nil, errors.New("canary node must be included in nodeServerIds")
	}
	task := &model.KernelUpgradeTask{
		OperatorId: operator.Id, OperatorName: operator.Username,
		CanaryNodeId: request.CanaryNodeServerId, Status: "queued",
	}
	items := make([]model.KernelUpgradeTaskItem, 0, len(servers)*len(request.Targets))
	seenTargets := make(map[string]bool)
	for _, target := range request.Targets {
		if seenTargets[target.Kernel] {
			return nil, fmt.Errorf("kernel %s appears more than once", target.Kernel)
		}
		seenTargets[target.Kernel] = true
		if target.Action != "rollback" && target.Channel == "legacy" {
			return nil, errors.New("legacy versions can only be used for rollback")
		}
	}
	for _, server := range servers {
		for _, target := range request.Targets {
			action := target.Action
			if action == "" {
				action = "install"
			}
			if action == "rollback" && target.Channel != "legacy" && target.Channel != "stable" && target.Channel != "prerelease" {
				return nil, errors.New("invalid rollback channel")
			}
			item := model.KernelUpgradeTaskItem{
				NodeServerId: *server.Id, NodeServerName: *server.Name,
				Kernel: target.Kernel, TargetVersion: target.Version, Channel: target.Channel,
				Stage: "queued", Attempt: 1,
			}
			item.Action = action
			item.IdempotencyKey = fmt.Sprintf("pending-%d-%d-%s-%s", time.Now().UnixNano(), len(items), item.Kernel, item.TargetVersion)
			items = append(items, item)
		}
	}
	nodeLifecycle.Lock()
	defer nodeLifecycle.Unlock()
	if err := recheckKernelServerSnapshots(ctx, servers); err != nil {
		return nil, err
	}
	if err := dao.CreateKernelUpgradeTask(task, items); err != nil {
		return nil, err
	}
	for index := range items {
		items[index].IdempotencyKey = fmt.Sprintf("task-%d-item-%d-attempt-1", task.Id, items[index].Id)
		if err := dao.UpdateKernelTaskItem(items[index]); err != nil {
			return nil, err
		}
	}
	task.Items = items
	go runKernelTask(task.Id, task.CanaryNodeId, items, token)
	return task, nil
}

func SelectKernelTaskPage(pageNum, pageSize uint, status string) ([]model.KernelUpgradeTask, uint, error) {
	return dao.SelectKernelUpgradeTaskPage(pageNum, pageSize, status)
}

func SelectKernelTask(id uint64) (*model.KernelUpgradeTask, error) {
	return dao.SelectKernelUpgradeTask(id)
}

func RetryKernelTask(request dto.KernelTaskRetryDto, token string) error {
	if !nodeMaintenanceAdmission.TryLock() {
		return errors.New("another Node maintenance request is being admitted; retry when it finishes")
	}
	defer nodeMaintenanceAdmission.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), kernelAdmissionTimeout)
	defer cancel()
	nodeLifecycle.RLock()
	task, err := dao.SelectKernelUpgradeTask(request.Id)
	nodeLifecycle.RUnlock()
	if err != nil {
		return err
	}
	selected := make(map[uint64]bool)
	for _, id := range request.ItemIds {
		selected[id] = true
	}
	checked := make(map[uint]bool)
	var servers []*model.NodeServer
	var retryItemIDs []uint64
	for _, item := range task.Items {
		if err := ctx.Err(); err != nil {
			return err
		}
		if item.Stage != "failed" && item.Stage != "rolled_back" || len(selected) > 0 && !selected[item.Id] {
			continue
		}
		retryItemIDs = append(retryItemIDs, item.Id)
		if checked[item.NodeServerId] {
			continue
		}
		server, selectErr := selectKernelServerSnapshot(ctx, item.NodeServerId)
		if selectErr != nil {
			return selectErr
		}
		servers = append(servers, server)
		checked[item.NodeServerId] = true
	}
	if len(retryItemIDs) == 0 {
		return errors.New("no failed task items selected")
	}
	if err = checkNodeContainersIdle(ctx, servers, fetchNodeContainerInventory); err != nil {
		return err
	}
	nodeLifecycle.Lock()
	defer nodeLifecycle.Unlock()
	if err = recheckKernelServerSnapshots(ctx, servers); err != nil {
		return err
	}
	// Only retry items represented by the checked snapshot. An item that fails
	// during the network preflight belongs to a subsequent retry request.
	items, err := dao.ResetKernelTaskItems(request.Id, retryItemIDs)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return errors.New("no failed task items selected")
	}
	go runKernelTask(task.Id, 0, items, token)
	return nil
}

func selectKernelServerSnapshot(ctx context.Context, id uint) (*model.NodeServer, error) {
	nodeLifecycle.RLock()
	defer nodeLifecycle.RUnlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return dao.SelectNodeServer(map[string]interface{}{"id": id})
}

// The caller holds the lifecycle write lock until Create/Reset commits. Server
// edits also use this lifecycle lock, so no endpoint can change after recheck.
func recheckKernelServerSnapshots(ctx context.Context, servers []*model.NodeServer) error {
	for _, snapshot := range servers {
		if err := ctx.Err(); err != nil {
			return err
		}
		current, err := dao.SelectNodeServer(map[string]interface{}{"id": *snapshot.Id})
		if err != nil {
			return err
		}
		if current.Id == nil || current.Ip == nil || current.GrpcPort == nil || *snapshot.Id != *current.Id || *snapshot.Ip != *current.Ip || *snapshot.GrpcPort != *current.GrpcPort || !sameKernelNodeString(snapshot.Name, current.Name) || !sameKernelNodeString(snapshot.GrpcTLSMode, current.GrpcTLSMode) || !sameKernelNodeString(snapshot.GrpcTLSServerName, current.GrpcTLSServerName) || !sameRemovalFlag(snapshot.Removing, current.Removing) {
			return fmt.Errorf("node server %d changed during kernel task preflight; retry", *snapshot.Id)
		}
	}
	return ctx.Err()
}

func sameKernelNodeString(left, right *string) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func checkNodeContainersIdle(ctx context.Context, servers []*model.NodeServer, fetch func(context.Context, *model.NodeServer) (*core.HostContainerInventory, error)) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	jobs := make(chan *model.NodeServer)
	var wait sync.WaitGroup
	var firstError error
	var once sync.Once
	for worker := 0; worker < kernelAdmissionConcurrency && worker < len(servers); worker++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case server, ok := <-jobs:
					if !ok {
						return
					}
					// Waiting for a worker does not consume this Node's own budget.
					nodeCtx, stop := context.WithTimeout(ctx, core.HostContainerTimeout)
					err := checkNodeContainerIdle(nodeCtx, server, fetch)
					stop()
					if err != nil {
						once.Do(func() {
							firstError = fmt.Errorf("node server %d: %w", *server.Id, err)
							cancel()
						})
						return
					}
				}
			}
		}()
	}
enqueue:
	for _, server := range servers {
		select {
		case jobs <- server:
		case <-ctx.Done():
			break enqueue
		}
	}
	close(jobs)
	wait.Wait()
	if firstError != nil {
		return firstError
	}
	return ctx.Err()
}

func ProbeAndEnableNodeServerMTLS(ctx context.Context, nodeServerId uint, serverName string) error {
	server, err := dao.SelectNodeServer(map[string]interface{}{"id": nodeServerId})
	if err != nil {
		return err
	}
	if err = core.ProbeKernelMTLS(ctx, *server.Ip, *server.GrpcPort, serverName); err != nil {
		return err
	}
	return dao.EnableNodeServerMTLS(nodeServerId, serverName)
}

func RecoverKernelTasks() {
	_ = dao.MarkInterruptedKernelItemsFailed()
	items, err := dao.SelectKernelTaskItemsByStage(
		"queued", "downloading", "verifying", "preflight", "switching",
		"restarting", "observing", "rolling_back",
	)
	if err != nil {
		return
	}
	grouped := make(map[uint64][]model.KernelUpgradeTaskItem)
	for _, item := range items {
		grouped[item.TaskId] = append(grouped[item.TaskId], item)
	}
	for taskId, taskItems := range grouped {
		go runKernelTask(taskId, 0, taskItems, "")
	}
}

func CleanupKernelUpgradeAudit() {
	_, _ = dao.CleanupKernelUpgradeAudit(time.Now().AddDate(0, 0, -60))
}

func runKernelTask(taskId uint64, canaryNodeId uint, items []model.KernelUpgradeTaskItem, token string) {
	_ = dao.RefreshKernelTaskStatus(taskId)
	if canaryNodeId != 0 {
		var canary []model.KernelUpgradeTaskItem
		var remaining []model.KernelUpgradeTaskItem
		for _, item := range items {
			if item.NodeServerId == canaryNodeId {
				canary = append(canary, item)
			} else {
				remaining = append(remaining, item)
			}
		}
		for index := range canary {
			if !executeKernelTaskItem(&canary[index], token) {
				for remainingIndex := range remaining {
					remaining[remainingIndex].Stage = "failed"
					remaining[remainingIndex].Result = "failed"
					remaining[remainingIndex].Error = "canary node failed"
					_ = dao.UpdateKernelTaskItem(remaining[remainingIndex])
				}
				_ = dao.RefreshKernelTaskStatus(taskId)
				return
			}
		}
		items = remaining
	}
	sem := make(chan struct{}, 3)
	var wait sync.WaitGroup
	for index := range items {
		wait.Add(1)
		item := &items[index]
		go func() {
			defer wait.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			executeKernelTaskItem(item, token)
		}()
	}
	wait.Wait()
	_ = dao.RefreshKernelTaskStatus(taskId)
}

func executeKernelTaskItem(item *model.KernelUpgradeTaskItem, token string) bool {
	lockValue, _ := kernelNodeLocks.LoadOrStore(item.NodeServerId, &sync.Mutex{})
	nodeLock := lockValue.(*sync.Mutex)
	nodeLock.Lock()
	defer nodeLock.Unlock()

	var inventory *core.KernelInventoryVo
	err := withKernelTaskServer(item.NodeServerId, func(server *model.NodeServer) error {
		var requestErr error
		inventory, requestErr = core.GetKernelInventory(token, *server.Ip, *server.GrpcPort, nodeTransport(server))
		return requestErr
	})
	if err != nil {
		failKernelTaskItem(item, fmt.Errorf("node offline or inventory unavailable: %w", err))
		return false
	}
	kernelEnum, err := kernelProto(item.Kernel)
	if err != nil {
		failKernelTaskItem(item, err)
		return false
	}
	for _, current := range inventory.Kernels {
		if current.Kernel == kernelEnum {
			item.FromVersion = current.CurrentVersion
			break
		}
	}
	action := core.KernelAction_KERNEL_ACTION_INSTALL
	if item.Action == "rollback" {
		action = core.KernelAction_KERNEL_ACTION_ROLLBACK
	}
	var operation *core.KernelOperationVo
	err = withKernelTaskServer(item.NodeServerId, func(server *model.NodeServer) error {
		var requestErr error
		operation, requestErr = core.StartKernelOperation(token, *server.Ip, *server.GrpcPort, nodeTransport(server), &core.KernelOperationRequest{
			IdempotencyKey: item.IdempotencyKey, Kernel: kernelEnum, Version: item.TargetVersion,
			Channel: channelProto(item.Channel), Action: action,
		})
		return requestErr
	})
	if err != nil {
		failKernelTaskItem(item, err)
		return false
	}
	item.CoreOperationId = operation.OperationId
	item.Stage = operation.Stage
	item.SHA256 = operation.Sha256
	_ = dao.UpdateKernelTaskItem(*item)

	deadline := time.Now().Add(15 * time.Minute)
	for time.Now().Before(deadline) {
		time.Sleep(2 * time.Second)
		err = withKernelTaskServer(item.NodeServerId, func(server *model.NodeServer) error {
			var requestErr error
			operation, requestErr = core.GetKernelOperation(token, *server.Ip, *server.GrpcPort, nodeTransport(server), item.CoreOperationId)
			return requestErr
		})
		if err != nil {
			failKernelTaskItem(item, err)
			return false
		}
		item.Stage = operation.Stage
		item.SHA256 = operation.Sha256
		item.Error = operation.Error
		item.RollbackResult = operation.RollbackError
		switch operation.Stage {
		case "succeeded":
			item.Result = "succeeded"
			_ = dao.UpdateKernelTaskItem(*item)
			return true
		case "failed":
			item.Result = "failed"
			_ = dao.UpdateKernelTaskItem(*item)
			return false
		case "rolled_back":
			item.Result = "failed"
			item.RollbackResult = "succeeded"
			_ = dao.UpdateKernelTaskItem(*item)
			return false
		default:
			_ = dao.UpdateKernelTaskItem(*item)
		}
	}
	failKernelTaskItem(item, errors.New("kernel operation timed out"))
	return false
}

// A queued or polling task may outlive a Web-only deletion. Re-read the
// server under the lifecycle lock for every remote call instead of retaining
// its endpoint in a goroutine after its registration has been removed.
func withKernelTaskServer(id uint, request func(*model.NodeServer) error) error {
	nodeLifecycle.RLock()
	defer nodeLifecycle.RUnlock()
	server, err := dao.SelectNodeServer(map[string]interface{}{"id": id})
	if err != nil {
		return err
	}
	if server.Removing != nil && *server.Removing != 0 {
		return errors.New("node server is being removed")
	}
	return request(server)
}

func failKernelTaskItem(item *model.KernelUpgradeTaskItem, err error) {
	item.Stage = "failed"
	item.Result = "failed"
	item.Error = err.Error()
	_ = dao.UpdateKernelTaskItem(*item)
}

func nodeTransport(server *model.NodeServer) core.NodeTransport {
	transport := core.NodeTransport{Mode: "legacy"}
	if server.GrpcTLSMode != nil && *server.GrpcTLSMode != "" {
		transport.Mode = *server.GrpcTLSMode
	}
	if transport.Mode == "mtls" {
		if server.GrpcTLSServerName != nil {
			transport.ServerName = *server.GrpcTLSServerName
		}
	}
	return transport
}

func kernelProto(value string) (core.ManagedKernel, error) {
	switch value {
	case "xray":
		return core.ManagedKernel_MANAGED_KERNEL_XRAY, nil
	case "hysteria2":
		return core.ManagedKernel_MANAGED_KERNEL_HYSTERIA2, nil
	default:
		return core.ManagedKernel_MANAGED_KERNEL_UNSPECIFIED, errors.New("unsupported managed kernel")
	}
}

func channelProto(value string) core.ReleaseChannel {
	switch value {
	case "prerelease":
		return core.ReleaseChannel_RELEASE_CHANNEL_PRERELEASE
	case "legacy":
		return core.ReleaseChannel_RELEASE_CHANNEL_LEGACY
	default:
		return core.ReleaseChannel_RELEASE_CHANNEL_STABLE
	}
}
