package gossiper

import (
	"context"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	healthgrpc "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

// dialTestServer serves `server` over an in-memory listener and returns a
// health client connected to it.
func dialTestServer(t *testing.T, server *grpc.Server) healthgrpc.HealthClient {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	go func() { _ = server.Serve(lis) }()
	t.Cleanup(server.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return healthgrpc.NewHealthClient(conn)
}

func TestNewGRPCServer_HealthStartsNotServing(t *testing.T) {
	server, healthServer := NewGRPCServer()
	client := dialTestServer(t, server)

	resp, err := client.Check(context.Background(), &healthgrpc.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("health check: %v", err)
	}
	if resp.GetStatus() != healthgrpc.HealthCheckResponse_NOT_SERVING {
		t.Fatalf("health must start NOT_SERVING, got %v", resp.GetStatus())
	}

	healthServer.SetServingStatus("", healthgrpc.HealthCheckResponse_SERVING)
	resp, err = client.Check(context.Background(), &healthgrpc.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("health check: %v", err)
	}
	if resp.GetStatus() != healthgrpc.HealthCheckResponse_SERVING {
		t.Fatalf("health must follow SetServingStatus, got %v", resp.GetStatus())
	}
}

func TestNewGRPCServer_InterceptorOrder(t *testing.T) {
	var calls []string
	record := func(name string) grpc.UnaryServerInterceptor {
		return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
			calls = append(calls, name)
			return handler(ctx, req)
		}
	}
	server, _ := NewGRPCServer(record("first"), record("second"))
	client := dialTestServer(t, server)

	if _, err := client.Check(context.Background(), &healthgrpc.HealthCheckRequest{}); err != nil {
		t.Fatalf("health check: %v", err)
	}
	if len(calls) != 2 || calls[0] != "first" || calls[1] != "second" {
		t.Fatalf("interceptors must run in the given order, got %v", calls)
	}
}

func TestNewGRPCServer_RecoversPanicsFromInterceptors(t *testing.T) {
	panicking := func(context.Context, any, *grpc.UnaryServerInfo, grpc.UnaryHandler) (any, error) {
		panic("boom")
	}
	server, _ := NewGRPCServer(panicking)
	client := dialTestServer(t, server)

	_, err := client.Check(context.Background(), &healthgrpc.HealthCheckRequest{})
	if status.Code(err) != codes.Internal {
		t.Fatalf("a panic in the chain must surface as codes.Internal, got %v", err)
	}
}

func TestNewGRPCServer_RegistersHealthAndReflection(t *testing.T) {
	server, _ := NewGRPCServer()
	info := server.GetServiceInfo()
	for _, name := range []string{"grpc.health.v1.Health", "grpc.reflection.v1.ServerReflection"} {
		if _, ok := info[name]; !ok {
			t.Fatalf("expected %s to be registered, got %v", name, info)
		}
	}
}
