package core

import (
	"context"
	"errors"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"time"
)

func GetKernelInventory(token, ip string, grpcPort uint, transport NodeTransport) (*KernelInventoryVo, error) {
	conn, ctx, closeConnection, err := newGrpcInstance(token, ip, grpcPort, 8*time.Second, transport)
	defer closeConnection()
	if err != nil {
		return nil, err
	}
	response, err := NewApiKernelServiceClient(conn).GetKernelInventory(ctx, &KernelInventoryRequest{})
	if err != nil {
		return nil, err
	}
	if !response.Success {
		return nil, errors.New(response.Msg)
	}
	var inventory KernelInventoryVo
	if err = anypb.UnmarshalTo(response.Data, &inventory, proto.UnmarshalOptions{}); err != nil {
		return nil, err
	}
	return &inventory, nil
}

func StartKernelOperation(token, ip string, grpcPort uint, transport NodeTransport, request *KernelOperationRequest) (*KernelOperationVo, error) {
	conn, ctx, closeConnection, err := newGrpcInstance(token, ip, grpcPort, 8*time.Second, transport)
	defer closeConnection()
	if err != nil {
		return nil, err
	}
	response, err := NewApiKernelServiceClient(conn).StartKernelOperation(ctx, request)
	if err != nil {
		return nil, err
	}
	if !response.Success {
		return nil, errors.New(response.Msg)
	}
	var operation KernelOperationVo
	if err = anypb.UnmarshalTo(response.Data, &operation, proto.UnmarshalOptions{}); err != nil {
		return nil, err
	}
	return &operation, nil
}

func GetKernelOperation(token, ip string, grpcPort uint, transport NodeTransport, id string) (*KernelOperationVo, error) {
	conn, ctx, closeConnection, err := newGrpcInstance(token, ip, grpcPort, 8*time.Second, transport)
	defer closeConnection()
	if err != nil {
		return nil, err
	}
	response, err := NewApiKernelServiceClient(conn).GetKernelOperation(ctx, &KernelOperationStateRequest{OperationId: id})
	if err != nil {
		return nil, err
	}
	if !response.Success {
		return nil, errors.New(response.Msg)
	}
	var operation KernelOperationVo
	if err = anypb.UnmarshalTo(response.Data, &operation, proto.UnmarshalOptions{}); err != nil {
		return nil, err
	}
	return &operation, nil
}

func ProbeKernelMTLS(ctx context.Context, ip string, grpcPort uint, serverName string) error {
	conn, callContext, closeConnection, err := newGrpcInstance("", ip, grpcPort, 8*time.Second, NodeTransport{
		Mode: "mtls", ServerName: serverName,
	})
	defer closeConnection()
	if err != nil {
		return err
	}
	if deadline, ok := ctx.Deadline(); ok {
		var cancel context.CancelFunc
		callContext, cancel = context.WithDeadline(callContext, deadline)
		defer cancel()
	}
	response, err := NewApiKernelServiceClient(conn).GetKernelInventory(callContext, &KernelInventoryRequest{})
	if err != nil {
		return err
	}
	if !response.Success {
		return errors.New(response.Msg)
	}
	return nil
}
