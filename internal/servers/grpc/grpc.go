package grpc

import (
	"google.golang.org/grpc"
	"log"
	"net"
)

type Server struct {
	Port      string
	Server    *grpc.Server
	InitRoute func(server *grpc.Server)
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
	log.Print("\033[32m")
	log.Printf("gRPC server running on port %s", g.Port)
	log.Print("\033[0m")
	return g.Server.Serve(listener)
}

func (g *Server) Stop() error {
	g.Server.GracefulStop()
	log.Println("gRPC server stopped")
	return nil
}
