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

func GetNodeKernelInventory(token string, nodeServerId uint) (*core.KernelInventoryVo, error) {
	server, err := dao.SelectNodeServer(map[string]interface{}{"id": nodeServerId})
	if err != nil {
		return nil, err
	}
	return core.GetKernelInventory(token, *server.Ip, *server.GrpcPort, nodeTransport(server))
}

func CreateKernelTask(request dto.KernelTaskCreateDto, operator vo.AccountVo, token string) (*model.KernelUpgradeTask, error) {
	seenNodes := make(map[uint]bool)
	var servers []*model.NodeServer
	for _, id := range request.NodeServerIds {
		if seenNodes[id] {
			continue
		}
		seenNodes[id] = true
		server, err := dao.SelectNodeServer(map[string]interface{}{"id": id})
		if err != nil {
			return nil, err
		}
		servers = append(servers, server)
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
	task, err := dao.SelectKernelUpgradeTask(request.Id)
	if err != nil {
		return err
	}
	items, err := dao.ResetKernelTaskItems(request.Id, request.ItemIds)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return errors.New("no failed task items selected")
	}
	go runKernelTask(task.Id, 0, items, token)
	return nil
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

	server, err := dao.SelectNodeServer(map[string]interface{}{"id": item.NodeServerId})
	if err != nil {
		failKernelTaskItem(item, err)
		return false
	}
	transport := nodeTransport(server)
	inventory, err := core.GetKernelInventory(token, *server.Ip, *server.GrpcPort, transport)
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
	operation, err := core.StartKernelOperation(token, *server.Ip, *server.GrpcPort, transport, &core.KernelOperationRequest{
		IdempotencyKey: item.IdempotencyKey, Kernel: kernelEnum, Version: item.TargetVersion,
		Channel: channelProto(item.Channel), Action: action,
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
		operation, err = core.GetKernelOperation(token, *server.Ip, *server.GrpcPort, transport, item.CoreOperationId)
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
