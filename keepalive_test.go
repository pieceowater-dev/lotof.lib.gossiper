package gossiper

import (
	"context"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthgrpc "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/test/bufconn"

	"github.com/pieceowater-dev/lotof.lib.gossiper/v2/internal/transport"
)

// A client that pings faster than the server's EnforcementPolicy allows is
// answered with GOAWAY "too_many_pings" and its connection is torn down, so
// the two settings have to be read together, not tuned apart.
func TestClientKeepaliveIsSlowerThanTheServerAccepts(t *testing.T) {
	if transport.ClientKeepaliveTime < grpcServerMinClientPingInterval {
		t.Fatalf("client pings every %s but the server only accepts one per %s",
			transport.ClientKeepaliveTime, grpcServerMinClientPingInterval)
	}
	if transport.ClientKeepaliveTimeout >= transport.ClientKeepaliveTime {
		t.Fatalf("a ping timeout of %s never fires with a %s ping interval",
			transport.ClientKeepaliveTimeout, transport.ClientKeepaliveTime)
	}
}

func TestClientKeepaliveConnectsToAGossiperServer(t *testing.T) {
	lis := bufconn.Listen(1024 * 1024)
	server, health := NewGRPCServer()
	health.SetServingStatus("", healthgrpc.HealthCheckResponse_SERVING)
	go func() { _ = server.Serve(lis) }()
	t.Cleanup(server.Stop)

	opts := []grpc.DialOption{
		WithClientInterceptors(),
		ClientKeepalive(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
	}
	conn, err := grpc.NewClient("passthrough:///bufnet", opts...)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := healthgrpc.NewHealthClient(conn).Check(ctx, &healthgrpc.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("health check over a keepalive connection: %v", err)
	}
	if resp.GetStatus() != healthgrpc.HealthCheckResponse_SERVING {
		t.Fatalf("unexpected status %v", resp.GetStatus())
	}
}
