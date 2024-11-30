package main

import (
	"github.com/gin-gonic/gin"
	"github.com/pieceowater-dev/lotof.lib.gossiper/v2"
	"google.golang.org/grpc"
)

// Entry point for the example application demonstrating how to use the gossiper package.
func main() {
	// Create a new server manager to manage multiple servers.
	serverManager := gossiper.NewServerManager()

	// Initialize the gRPC server.
	grpcInitRoute := func(server *grpc.Server) {
		// Example: Add gRPC routes here.
	}
	serverManager.AddServer(gossiper.NewGRPCServ("50051", grpc.NewServer(), grpcInitRoute))

	// Initialize the REST server.
	restInitRoute := func(router *gin.Engine) {
		// Define a health check endpoint.
		router.GET("/health", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "ok"})
		})
	}
	serverManager.AddServer(gossiper.NewRESTServ("8080", gin.Default(), restInitRoute))

	// Start all servers.
	serverManager.StartAll()
	defer serverManager.StopAll()
}
