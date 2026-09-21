package gossiper

import (
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthgrpc "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
)

const (
	// grpcServerMaxConnectionAge forces every caller's HTTP/2 connection to
	// gracefully cycle (GOAWAY, then a fresh connection with a fresh HPACK
	// encoder) instead of living indefinitely. golang.org/x/net's HPACK
	// encoder panics ("id <= evictCount") in the *client's* transport
	// goroutine once a single connection has accumulated enough table
	// evictions -- a crash no recover() can catch -- so bounding connection
	// age on the server side is what keeps the calling gateway alive.
	grpcServerMaxConnectionAge = 15 * time.Minute
	// grpcServerMaxConnectionAgeGrace lets in-flight RPCs finish before the
	// aged-out connection is closed. Every LOTOF RPC is unary, so this only
	// has to cover one slow call, not a long-lived stream.
	grpcServerMaxConnectionAgeGrace = 15 * time.Second
	// grpcServerMinClientPingInterval is the fastest keepalive ping rate the
	// server tolerates from a client.
	grpcServerMinClientPingInterval = 10 * time.Second
)

// NewGRPCServer builds an inbound gRPC server with the baseline every LOTOF
// service needs, so it can't drift between repositories:
//
//   - keepalive MaxConnectionAge 15m / grace 15s (see grpcServerMaxConnectionAge),
//     and client pings accepted every 10s (see grpcServerMinClientPingInterval);
//   - RecoveryUnaryServerInterceptor as the outermost interceptor, followed by
//     `interceptors` in the given order -- a panic anywhere in the chain or
//     the handler becomes codes.Internal instead of killing the process;
//   - the standard gRPC health service, starting NOT_SERVING -- the caller
//     flips it to SERVING once the service has finished starting;
//   - server reflection.
//
// Register the service's own handlers on the returned server as usual.
func NewGRPCServer(interceptors ...grpc.UnaryServerInterceptor) (*grpc.Server, *health.Server) {
	chain := append([]grpc.UnaryServerInterceptor{RecoveryUnaryServerInterceptor()}, interceptors...)

	server := grpc.NewServer(
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionAge:      grpcServerMaxConnectionAge,
			MaxConnectionAgeGrace: grpcServerMaxConnectionAgeGrace,
		}),
		// Accept client keepalive pings as often as every 10s, even with no
		// call in flight. The default (at most one per 5 minutes) answers
		// faster pings with GOAWAY "too_many_pings"; clients need them to
		// notice a connection to a pod that is already gone.
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             grpcServerMinClientPingInterval,
			PermitWithoutStream: true,
		}),
		grpc.ChainUnaryInterceptor(chain...),
	)

	healthServer := health.NewServer()
	healthgrpc.RegisterHealthServer(server, healthServer)
	healthServer.SetServingStatus("", healthgrpc.HealthCheckResponse_NOT_SERVING)
	reflection.Register(server)

	return server, healthServer
}
