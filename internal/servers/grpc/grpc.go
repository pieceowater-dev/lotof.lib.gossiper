package grpc

import (
	"context"
	"log/slog"
	"net"
	"runtime/debug"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// RecoveryUnaryServerInterceptor recovers from panics in the handler chain
// and turns them into a codes.Internal error instead of letting them escape
// and crash the whole process - a single bad request should not take down
// every tenant sharing this service. Put it first in the interceptor chain
// so it wraps every interceptor after it too.
func RecoveryUnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("recovered from panic in gRPC handler",
					slog.String("method", info.FullMethod),
					slog.Any("panic", r),
					slog.String("stack", string(debug.Stack())),
				)
				err = status.Errorf(codes.Internal, "internal error")
			}
		}()
		return handler(ctx, req)
	}
}

type Server struct {
	Port      string
	Server    *grpc.Server
	InitRoute func(server *grpc.Server)
}

// NewDefaultServer creates a grpc.Server with OTel trace propagation enabled.
// Pass this as the server argument to New / gossiper.NewGRPCServ.
func NewDefaultServer(opts ...grpc.ServerOption) *grpc.Server {
	defaults := []grpc.ServerOption{
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	}
	return grpc.NewServer(append(defaults, opts...)...)
}

func New(port string, server *grpc.Server, initRoute func(server *grpc.Server)) *Server {
	if initRoute != nil {
		initRoute(server)
	}
	return &Server{
		Port:      port,
		Server:    server,
		InitRoute: initRoute,
	}
}

func (g *Server) Start() error {
	listener, err := net.Listen("tcp", ":"+g.Port)
	if err != nil {
		return err
	}
	slog.Info("gRPC server running", "port", g.Port)
	return g.Server.Serve(listener)
}

func (g *Server) Stop() error {
	g.Server.GracefulStop()
	slog.Info("gRPC server stopped")
	return nil
}
