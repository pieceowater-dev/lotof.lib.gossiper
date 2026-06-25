package grpc

import (
	"log/slog"
	"net"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
)

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
