package gossiper

import (
	"context"
	"log/slog"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/pieceowater-dev/lotof.lib.gossiper/v2/internal/db"
	"github.com/pieceowater-dev/lotof.lib.gossiper/v2/internal/generic"
	"github.com/pieceowater-dev/lotof.lib.gossiper/v2/internal/observability"
	"github.com/pieceowater-dev/lotof.lib.gossiper/v2/internal/servers"
	grpcServ "github.com/pieceowater-dev/lotof.lib.gossiper/v2/internal/servers/grpc"
	restServ "github.com/pieceowater-dev/lotof.lib.gossiper/v2/internal/servers/http/fiber"
	"github.com/pieceowater-dev/lotof.lib.gossiper/v2/internal/tenant"
	"github.com/pieceowater-dev/lotof.lib.gossiper/v2/internal/transport"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"gorm.io/gorm"
)

// Database aliases the internal database abstraction.
type Database = db.Database

// DBFactory aliases the database factory for creating new database instances.
type DBFactory = db.DatabaseFactory

// DatabaseType represents the type of database being used.
type DatabaseType = db.DatabaseType

// PostgresDB One of Supported database types.
const (
	PostgresDB   DatabaseType = db.PostgresDB
)

// NewDB initializes a new database connection.
// - dbType: The type of database (e.g., PostgresDB).
// - dsn: The data source name for connecting to the database.
// - enableLogs: Whether to enable logging for the database.
// Returns a `Database` interface or an error if initialization fails.
func NewDB(dbType DatabaseType, dsn string, enableLogs bool, autoMigrateModels []any) (Database, error) {
	return db.New(dsn, enableLogs, autoMigrateModels).Create(dbType)
}

// ServerManager aliases the server manager for managing multiple servers.
type ServerManager = servers.ServerManager

// GRPCServer represents a gRPC server instance.
type GRPCServer = grpcServ.Server

// RESTServer represents a REST server instance.
type RESTServer = restServ.Server

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

// NewDefaultGRPCServer creates a grpc.Server with OTel trace propagation pre-wired.
// Use this instead of grpc.NewServer() so incoming trace context is automatically
// received and propagated through the server's handler chain.
func NewDefaultGRPCServer(opts ...grpc.ServerOption) *grpc.Server {
	return grpcServ.NewDefaultServer(opts...)
}

// InitLogger sets the global slog logger to JSON output on stderr.
// Call once at the top of main() so all log output is structured and
// compatible with log aggregators (CloudWatch, Loki, etc.).
func InitLogger() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))
}

// NewRESTServ creates a new REST server.
// - port: The port number for the server.
// - router: The router instance.
// - initRoute: A function to initialize the server's routes.
// Returns a `RESTServer` instance.
func NewRESTServ(port string, router *fiber.App, initRoute func(router *fiber.App)) *RESTServer {
	return restServ.New(port, router, initRoute)
}

type Transport = transport.Transport
type TransportType = transport.Type

const (
	GRPC TransportType = transport.GRPC
)

type TransportFactory = transport.Factory

func NewTransportFactory() *TransportFactory {
	return transport.NewFactory()
}

// Filter [T] is an alias for generic.Filter[T], providing a filter structure.
type Filter[T any] struct {
	generic.Filter[T]
}

// NewFilter creates a new Filter instance with the provided search term, sorting, and pagination.
func NewFilter[T any](search string, sort Sort[T], pagination Pagination) Filter[T] {
	return Filter[T]{
		Filter: generic.NewFilter(search, sort.Sort, pagination.Pagination),
	}
}

// PaginatedResult [T] is an alias for generic.PaginatedResult[T], representing paginated results.
type PaginatedResult[T any] struct {
	generic.PaginatedResult[T]
}

// NewPaginatedResult creates a new PaginatedResult with the given rows and total count.
func NewPaginatedResult[T any](rows []T, count int) PaginatedResult[T] {
	return PaginatedResult[T]{
		PaginatedResult: generic.NewPaginatedResult(rows, count),
	}
}

// Pagination is an alias for generic.Pagination, encapsulating pagination data.
type Pagination struct {
	generic.Pagination
}

// NewPagination creates a new Pagination instance with the specified page and length.
func NewPagination(page, length int) Pagination {
	return Pagination{
		Pagination: generic.NewPagination(page, length),
	}
}

// Sort [T] is an alias for generic.Sort[T], representing sorting data.
type Sort[T any] struct {
	generic.Sort[T]
}

// NewSort creates a new Sort instance for a given field and direction.
func NewSort[T any](field string, direction SortDirection) Sort[T] {
	return Sort[T]{
		Sort: generic.NewSort[T](field, direction),
	}
}

// SortDirection is an alias for generic.SortDirection, representing sort directions (e.g., ascending or descending).
type SortDirection = generic.SortDirection

// IsFieldValid checks if a field is valid in a given model.
func IsFieldValid(model any, field string) bool {
	return generic.IsFieldValid(model, field)
}

// ToSnakeCase converts a string to snake_case format.
func ToSnakeCase(s string) string {
	return generic.ToSnakeCase(s)
}

type ITenantManager = tenant.ITenantManager
type TenantManager = tenant.Manager
type EncryptedTenant = tenant.EncryptedTenant

//type RawTenant struct {
//	database string // Schema name / namespace name
//	username string
//	password string
//}

func NewTenantManager(db *gorm.DB, secret string) (*TenantManager, error) {
	return tenant.NewTenantManager(db, secret)
}

// GenerateRandomString creates a random alphanumeric string of the given length.
func GenerateRandomString(length int) string {
	return generic.GenerateRandomString(length)
}

// EncryptAES256 encrypts plaintext using AES-256 encryption in CTR mode.
func EncryptAES256(key, plaintext string) (string, error) {
	return generic.EncryptAES256(key, plaintext)
}

// DecryptAES256 decrypts a base64-encoded encrypted string using AES-256 in CTR mode.
func DecryptAES256(key, encrypted string) (string, error) {
	return generic.DecryptAES256(key, encrypted)
}

// ObservabilityConfig holds settings for initialising the observability stack.
type ObservabilityConfig = observability.Config

// InitObservability sets up the OTLP tracer provider and a structured JSON logger.
// Returns logger, tracer, shutdown func, and any init error.
// On error fall back to slog.Default() and trace.NewNoopTracerProvider().Tracer("noop").
func InitObservability(ctx context.Context, cfg ObservabilityConfig) (*slog.Logger, trace.Tracer, func(context.Context) error, error) {
	return observability.Init(ctx, cfg)
}

// ObservabilityFiberMiddleware instruments all incoming HTTP requests with trace context
// propagation, request_id generation, span creation, and structured logging.
// Must be registered before route handlers so every request gets a trace.
func ObservabilityFiberMiddleware(logger *slog.Logger, tracer trace.Tracer) func(*fiber.Ctx) error {
	return observability.FiberMiddleware(logger, tracer)
}

// ObservabilityGRPCServerInterceptor adds trace propagation, request_id extraction,
// span creation, and structured logging to every incoming gRPC unary call.
// Use as grpc.UnaryInterceptor on the gRPC server.
func ObservabilityGRPCServerInterceptor(logger *slog.Logger, tracer trace.Tracer) grpc.UnaryServerInterceptor {
	return observability.GRPCServerInterceptor(logger, tracer)
}

// ObservabilityGRPCClientInterceptor propagates request_id and OTel trace headers on
// every outgoing gRPC call and records client spans.
// Use as grpc.WithUnaryInterceptor on gRPC client connections.
func ObservabilityGRPCClientInterceptor(logger *slog.Logger, tracer trace.Tracer) grpc.UnaryClientInterceptor {
	return observability.GRPCClientInterceptor(logger, tracer)
}

// ObservabilityLoggerFromContext enriches logger with request_id, trace_id, span_id,
// tenant, and user fields from the request context.
func ObservabilityLoggerFromContext(ctx context.Context, base *slog.Logger) *slog.Logger {
	return observability.LoggerFromContext(ctx, base)
}

// ObservabilityWithOutgoingMetadata injects request_id and OTel trace headers into the
// outgoing gRPC metadata so downstream services can correlate their logs.
func ObservabilityWithOutgoingMetadata(ctx context.Context) context.Context {
	return observability.WithOutgoingMetadata(ctx)
}

// ObservabilityRequestID extracts the request_id from context.
func ObservabilityRequestID(ctx context.Context) string {
	return observability.RequestID(ctx)
}

// RegisterTransportContextMiddleware sets a function that enriches the context
// before every outgoing gRPC call made by the gossiper transport.
// Call once at startup — typically to inject x-request-id and W3C traceparent
// so all downstream services can correlate their logs with the same trace.
//
// Example:
//
//	gossiper.RegisterTransportContextMiddleware(observability.WithOutgoingMetadata)
func RegisterTransportContextMiddleware(fn func(ctx context.Context) context.Context) {
	transport.RegisterSendContextMiddleware(fn)
}
