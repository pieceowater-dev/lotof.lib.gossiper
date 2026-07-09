package grpc

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestRecoveryUnaryServerInterceptor_RecoversPanic(t *testing.T) {
	interceptor := RecoveryUnaryServerInterceptor()
	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}

	handler := func(ctx context.Context, req any) (any, error) {
		panic("boom")
	}

	resp, err := interceptor(context.Background(), nil, info, handler)
	if resp != nil {
		t.Errorf("expected nil response after recovered panic, got %v", resp)
	}
	if err == nil {
		t.Fatal("expected an error after recovered panic, got nil")
	}
	if status.Code(err) != codes.Internal {
		t.Errorf("expected codes.Internal, got %v", status.Code(err))
	}
}

func TestRecoveryUnaryServerInterceptor_PassesThroughOnSuccess(t *testing.T) {
	interceptor := RecoveryUnaryServerInterceptor()
	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}

	handler := func(ctx context.Context, req any) (any, error) {
		return "ok", nil
	}

	resp, err := interceptor(context.Background(), nil, info, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != "ok" {
		t.Errorf("expected response 'ok', got %v", resp)
	}
}

func TestRecoveryUnaryServerInterceptor_PassesThroughHandlerError(t *testing.T) {
	interceptor := RecoveryUnaryServerInterceptor()
	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}
	wantErr := errors.New("not found")

	handler := func(ctx context.Context, req any) (any, error) {
		return nil, wantErr
	}

	_, err := interceptor(context.Background(), nil, info, handler)
	if !errors.Is(err, wantErr) {
		t.Errorf("expected handler error to pass through unchanged, got %v", err)
	}
}
