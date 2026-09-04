package api

import (
	"context"
	"errors"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"os"
	"trojan-panel-core/kernel"
)

var kernelManager *kernel.Manager

func initKernelManager() error {
	runtimeDir := os.Getenv("TP_KERNEL_RUNTIME")
	var err error
	kernelManager, err = kernel.New(runtimeDir, kernel.ProcessLifecycle{})
	return err
}

type KernelApiServer struct {
	UnimplementedApiKernelServiceServer
}

func (s *KernelApiServer) GetKernelInventory(ctx context.Context, _ *KernelInventoryRequest) (*Response, error) {
	if err := authRequest(ctx); err != nil {
		return &Response{Success: false, Msg: err.Error()}, nil
	}
	inventory, err := kernelManager.Inventory(ctx)
	if err != nil {
		return &Response{Success: false, Msg: err.Error()}, nil
	}
	result := &KernelInventoryVo{Os: inventory.OS, Arch: inventory.Arch}
	for _, item := range inventory.Kernels {
		entry := &KernelInventoryItemVo{
			Kernel: kernelToProto(item.Kernel), Supported: item.Supported,
			CurrentVersion: item.CurrentVersion, CurrentSha256: item.CurrentSHA256,
			Channel: channelToProto(item.Channel), InUse: item.InUse, Error: item.Error,
		}
		for _, version := range item.Versions {
			entry.Versions = append(entry.Versions, &KernelVersionVo{
				Version: version.Version, Channel: channelToProto(version.Channel),
				Sha256: version.SHA256, InstalledAt: version.Installed.Unix(),
				Legacy: version.Legacy, Successful: version.Successful,
			})
		}
		result.Kernels = append(result.Kernels, entry)
	}
	return messageResponse(result)
}

func (s *KernelApiServer) StartKernelOperation(ctx context.Context, request *KernelOperationRequest) (*Response, error) {
	if err := authRequest(ctx); err != nil {
		return &Response{Success: false, Msg: err.Error()}, nil
	}
	managedKernel, err := kernelFromProto(request.Kernel)
	if err != nil {
		return &Response{Success: false, Msg: err.Error()}, nil
	}
	channel, err := channelFromProto(request.Channel)
	if err != nil {
		return &Response{Success: false, Msg: err.Error()}, nil
	}
	action, err := actionFromProto(request.Action)
	if err != nil {
		return &Response{Success: false, Msg: err.Error()}, nil
	}
	operation, err := kernelManager.Start(ctx, kernel.OperationRequest{
		IdempotencyKey: request.IdempotencyKey,
		Kernel:         managedKernel, Version: request.Version, Channel: channel, Action: action,
	})
	if err != nil {
		return &Response{Success: false, Msg: err.Error()}, nil
	}
	return messageResponse(operationToProto(operation))
}

func (s *KernelApiServer) GetKernelOperation(ctx context.Context, request *KernelOperationStateRequest) (*Response, error) {
	if err := authRequest(ctx); err != nil {
		return &Response{Success: false, Msg: err.Error()}, nil
	}
	operation, err := kernelManager.Get(ctx, request.OperationId)
	if err != nil {
		return &Response{Success: false, Msg: err.Error()}, nil
	}
	return messageResponse(operationToProto(operation))
}

func messageResponse(message proto.Message) (*Response, error) {
	data, err := anypb.New(message)
	if err != nil {
		return &Response{Success: false, Msg: err.Error()}, nil
	}
	return &Response{Success: true, Data: data}, nil
}

func operationToProto(operation *kernel.Operation) *KernelOperationVo {
	return &KernelOperationVo{
		OperationId: operation.ID, IdempotencyKey: operation.IdempotencyKey,
		Kernel: kernelToProto(operation.Kernel), FromVersion: operation.FromVersion,
		TargetVersion: operation.TargetVersion, Channel: channelToProto(operation.Channel),
		Action: actionToProto(operation.Action), Sha256: operation.SHA256,
		Stage: string(operation.Stage), Error: operation.Error, RollbackError: operation.RollbackError,
		CreatedAt: operation.CreatedAt.Unix(), UpdatedAt: operation.UpdatedAt.Unix(),
	}
}

func kernelFromProto(value ManagedKernel) (kernel.Kernel, error) {
	switch value {
	case ManagedKernel_MANAGED_KERNEL_XRAY:
		return kernel.KernelXray, nil
	case ManagedKernel_MANAGED_KERNEL_HYSTERIA2:
		return kernel.KernelHysteria2, nil
	default:
		return "", errors.New("unsupported managed kernel")
	}
}

func kernelToProto(value kernel.Kernel) ManagedKernel {
	if value == kernel.KernelHysteria2 {
		return ManagedKernel_MANAGED_KERNEL_HYSTERIA2
	}
	return ManagedKernel_MANAGED_KERNEL_XRAY
}

func channelFromProto(value ReleaseChannel) (kernel.Channel, error) {
	switch value {
	case ReleaseChannel_RELEASE_CHANNEL_STABLE:
		return kernel.ChannelStable, nil
	case ReleaseChannel_RELEASE_CHANNEL_PRERELEASE:
		return kernel.ChannelPrerelease, nil
	case ReleaseChannel_RELEASE_CHANNEL_LEGACY:
		return kernel.ChannelLegacy, nil
	default:
		return "", errors.New("unsupported release channel")
	}
}

func channelToProto(value kernel.Channel) ReleaseChannel {
	switch value {
	case kernel.ChannelPrerelease:
		return ReleaseChannel_RELEASE_CHANNEL_PRERELEASE
	case kernel.ChannelLegacy:
		return ReleaseChannel_RELEASE_CHANNEL_LEGACY
	default:
		return ReleaseChannel_RELEASE_CHANNEL_STABLE
	}
}

func actionFromProto(value KernelAction) (kernel.Action, error) {
	switch value {
	case KernelAction_KERNEL_ACTION_INSTALL:
		return kernel.ActionInstall, nil
	case KernelAction_KERNEL_ACTION_ROLLBACK:
		return kernel.ActionRollback, nil
	default:
		return "", errors.New("unsupported kernel action")
	}
}

func actionToProto(value kernel.Action) KernelAction {
	if value == kernel.ActionRollback {
		return KernelAction_KERNEL_ACTION_ROLLBACK
	}
	return KernelAction_KERNEL_ACTION_INSTALL
}
