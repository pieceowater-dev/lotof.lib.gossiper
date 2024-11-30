# Gossiper

`gossiper` is a modern Go library designed to simplify server management and transport mechanisms for gRPC, REST, and RabbitMQ in distributed systems. The package allows developers to manage servers, routes, and database connections with ease, while supporting a pluggable architecture for extending functionality.

## Installation

```bash
go get github.com/pieceowater-dev/lotof.lib.gossiper/v2
```

Features

- Centralized server management with ServerManager
- Support for gRPC and REST servers
- RabbitMQ server integration
- Transport factory for creating gRPC-based communication channels
- Simplified database connection management
- Modular and extendable design

## Usage Examples

### Server Management

gossiper provides a ServerManager to manage multiple servers, such as gRPC and REST, in one place.

### Example: Managing Multiple Servers

```golang
package main

import (
	"github.com/gin-gonic/gin"
	"github.com/pieceowater-dev/lotof.lib.gossiper/v2"
	"google.golang.org/grpc"
)

func main() {
	serverManager := gossiper.NewServerManager()

	// Initialize gRPC Server
	grpcInitRoute := func(server *grpc.Server) {
		// Define gRPC services here
	}
	serverManager.AddServer(gossiper.NewGRPCServ("50051", grpc.NewServer(), grpcInitRoute))

	// Initialize REST Server
	restInitRoute := func(router *gin.Engine) {
		router.GET("/health", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "ok"})
		})
	}
	serverManager.AddServer(gossiper.NewRESTServ("8080", gin.Default(), restInitRoute))

	// Start all servers
	serverManager.StartAll()
	defer serverManager.StopAll()
}
```

### Transport Factory

The TransportFactory simplifies the creation of transport mechanisms for communication, such as gRPC.

Example: Creating a gRPC Transport
```golang
package main

import (
	"github.com/pieceowater-dev/lotof.lib.gossiper/v2"
)

func main() {
	factory := gossiper.NewTransportFactory()
	grpcTransport := factory.CreateTransport(
		gossiper.GRPC,
		"localhost:50051",
	)

	// Use grpcTransport to send requests or create clients
	_ = grpcTransport
}
```

## Core API

### Server Management

#### ServerManager

Manages the lifecycle of multiple servers (start, stop).
- NewServerManager
Creates a new server manager instance.
- AddServer
Adds a new server (e.g., gRPC, REST) to the manager.
- StartAll / StopAll
Starts or stops all servers managed by the instance.

#### Servers

gRPC Server

- NewGRPCServ
Creates a new gRPC server instance.

REST Server

- NewRESTServ
Creates a new REST server using the Gin framework.

RabbitMQ Server

- NewRMQServ
Creates a new RabbitMQ server instance.

#### Transport

TransportFactory

The TransportFactory provides a unified way to create communication transports.
- Supported Transports:
- GRPC: For gRPC communication.

### Example:
```golang
factory := gossiper.NewTransportFactory()
transport := factory.CreateTransport(gossiper.GRPC, "localhost:50051")
```

#### Database

#### NewDB

Initializes a database connection.
- Supported Database Types:
- PostgresDB: For PostgreSQL connections.

#### Example:
```golang
db, err := gossiper.NewDB(gossiper.PostgresDB, "your-dsn", true)
if err != nil {
    panic(err)
}
```

### Contributing

Contributions are welcome! Feel free to submit issues or pull requests to improve the package or its documentation.

---

With `gossiper`, managing RabbitMQ consumers and environment variables in Go projects becomes more straightforward. Enjoy using it!

## License
This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Author
![PCWT Dev Logo](https://avatars.githubusercontent.com/u/168465239?s=50)
### [PCWT Dev](https://github.com/pieceowater-dev)