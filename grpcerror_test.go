package gossiper

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	healthgrpc "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

func TestInternalError(t *testing.T) {
	var buf bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	cause := errors.New(`duplicate key value violates unique constraint "idx_clients_phone" (SQLSTATE 23505)`)
	err := InternalError(context.Background(), "failed to create client", cause)

	if status.Code(err) != codes.Internal {
		t.Fatalf("expected Internal, got %v", status.Code(err))
	}
	if msg := status.Convert(err).Message(); msg != "failed to create client" {
		t.Fatalf("the status message must not carry the cause, got %q", msg)
	}
	if strings.Contains(status.Convert(err).Message(), "idx_clients_phone") {
		t.Fatal("the constraint name reached the wire")
	}
	if logged := buf.String(); !strings.Contains(logged, "idx_clients_phone") {
		t.Fatalf("the cause must still be logged in full, got %q", logged)
	}
}

func TestErrorSanitizerUnary(t *testing.T) {
	var buf bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	run := func(err error) error {
		_, got := ErrorSanitizerUnaryServerInterceptor()(context.Background(), nil,
			&grpc.UnaryServerInfo{FullMethod: "/svc.Test/Do"},
			func(context.Context, any) (any, error) { return nil, err })
		return got
	}

	raw := run(errors.New(`relation "clients" does not exist (SQLSTATE 42P01)`))
	if status.Code(raw) != codes.Internal || status.Convert(raw).Message() != GRPCInternalErrorMessage {
		t.Fatalf("raw error must become a bare Internal, got %v", raw)
	}
	logged := buf.String()
	if !strings.Contains(logged, "SQLSTATE 42P01") || !strings.Contains(logged, "/svc.Test/Do") {
		t.Fatalf("the original and the method must be logged, got %q", logged)
	}

	st := status.Error(codes.NotFound, "client not found")
	if got := run(st); got != st {
		t.Fatalf("a status error must pass through unchanged, got %v", got)
	}
	if got := run(nil); got != nil {
		t.Fatalf("nil must stay nil, got %v", got)
	}
	if got := run(context.DeadlineExceeded); status.Code(got) != codes.DeadlineExceeded {
		t.Fatalf("a deadline must keep its code, got %v", got)
	}
}

func TestNewGRPCServer_SanitizesRawErrors(t *testing.T) {
	raw := func(context.Context, any, *grpc.UnaryServerInfo, grpc.UnaryHandler) (any, error) {
		return nil, errors.New("pq: secret detail")
	}
	server, _ := NewGRPCServer(raw)
	client := dialTestServer(t, server)

	_, err := client.Check(context.Background(), &healthgrpc.HealthCheckRequest{})
	if status.Code(err) != codes.Internal || strings.Contains(err.Error(), "secret detail") {
		t.Fatalf("a raw error must reach the caller as a bare Internal, got %v", err)
	}
}

func TestErrorSanitizerStream(t *testing.T) {
	interceptor := ErrorSanitizerStreamServerInterceptor()
	info := &grpc.StreamServerInfo{FullMethod: "/svc.Test/Stream"}
	err := interceptor(nil, fakeStream{}, info, func(any, grpc.ServerStream) error { return errors.New("pq: detail") })
	if status.Code(err) != codes.Internal || strings.Contains(err.Error(), "detail") {
		t.Fatalf("got %v", err)
	}
	st := status.Error(codes.InvalidArgument, "bad")
	if got := interceptor(nil, fakeStream{}, info, func(any, grpc.ServerStream) error { return st }); got != st {
		t.Fatalf("status must pass through, got %v", got)
	}
}

type fakeStream struct{ grpc.ServerStream }

func (fakeStream) Context() context.Context { return context.Background() }
