package api

import (
	"context"
	"errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"time"
	"trojan-panel-core/app/xray"
)

func (s *KernelApiServer) CheckXrayTemplates(ctx context.Context, request *XrayTemplatesRequest) (*Response, error) {
	if err := authRequest(ctx); err != nil {
		return &Response{Msg: err.Error()}, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	if err := xray.CheckTemplates(ctx, request.GetContents()); err != nil {
		if errors.Is(err, xray.ErrTemplateCheckUnavailable) {
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		}
		return &Response{Msg: err.Error()}, nil
	}
	return &Response{Success: true}, nil
}
