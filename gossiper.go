package gossiper

import (
	"github.com/gin-gonic/gin"
	"github.com/pieceowater-dev/lotof.lib.gossiper/internal/core/db"
	"github.com/pieceowater-dev/lotof.lib.gossiper/internal/core/servers"
	grpcServ "github.com/pieceowater-dev/lotof.lib.gossiper/internal/core/servers/grpc"
	"github.com/pieceowater-dev/lotof.lib.gossiper/internal/core/servers/rabbitmq"
	rmqServ "github.com/pieceowater-dev/lotof.lib.gossiper/internal/core/servers/rabbitmq"
	"github.com/pieceowater-dev/lotof.lib.gossiper/internal/core/servers/rest"
	restServ "github.com/pieceowater-dev/lotof.lib.gossiper/internal/core/servers/rest"
	"google.golang.org/grpc"
)

// Database aliases the internal database abstraction.
type Database = db.Database

// DBFactory aliases the database factory for creating new database instances.
type DBFactory = db.DatabaseFactory

// DatabaseType represents the type of database being used.
type DatabaseType = db.DatabaseType

// PostgresDB One of Supported database types.
const (
	PostgresDB DatabaseType = db.PostgresDB
)

// NewDB initializes a new database connection.
// - dbType: The type of database (e.g., PostgresDB).
// - dsn: The data source name for connecting to the database.
// - enableLogs: Whether to enable logging for the database.
// Returns a `Database` interface or an error if initialization fails.
func NewDB(dbType DatabaseType, dsn string, enableLogs bool) (Database, error) {
	return db.New(dsn, enableLogs).Create(dbType)
}

// ServerManager aliases the server manager for managing multiple servers.
type ServerManager = servers.ServerManager

// GRPCServer represents a gRPC server instance.
type GRPCServer = grpcServ.Server

// RESTServer represents a REST server instance.
type RESTServer = rest.Server

// RMQServer represents a RabbitMQ server instance.
type RMQServer = rabbitmq.Server

// NewServerManager creates a new instance of the server manager.
// The server manager is responsible for starting and stopping multiple server instances.
func NewServerManager() *ServerManager {
	return servers.NewServerManager()
}

// NewGRPCServ creates a new gRPC server.
// - port: The port number for the server.
// - server: The gRPC server instance.
// - initRoute: A function to initialize the server's routes.
// Returns a `GRPCServer` instance.
func NewGRPCServ(port string, server *grpc.Server, initRoute func(server *grpc.Server)) *GRPCServer {
	return grpcServ.New(port, server, initRoute)
}

// NewRESTServ creates a new REST server.
// - port: The port number for the server.
// - router: The Gin router instance.
// - initRoute: A function to initialize the server's routes.
// Returns a `RESTServer` instance.
func NewRESTServ(port string, router *gin.Engine, initRoute func(router *gin.Engine)) *RESTServer {
	return restServ.New(port, router, initRoute)
}

// NewRMQServ creates a new RabbitMQ server.
// Returns an `RMQServer` instance.
func NewRMQServ() *RMQServer {
	return rmqServ.New()
}
