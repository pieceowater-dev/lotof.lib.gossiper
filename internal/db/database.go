package db

import (
	"context"
	"fmt"
	postgresql "github.com/pieceowater-dev/lotof.lib.gossiper/v2/internal/db/pg"
	"gorm.io/gorm"
)

const (
	PostgresDB DatabaseType = iota
)

// Database defines the common methods for database operations
type Database interface {
	GetDB() *gorm.DB
	WithTransaction(func(tx *gorm.DB) error) error
	SeedData(data []any) error
	// SwitchSchema is unsafe against a pooled connection — see the
	// implementation's doc comment. Prefer WithSchema.
	SwitchSchema(schema string) *gorm.DB
	// WithSchema runs fn on a connection pinned to schema's search_path for
	// the duration of the call. This is the safe way to do tenant-scoped
	// work against a shared connection pool.
	WithSchema(ctx context.Context, schema string, fn func(tx *gorm.DB) error) error
	// WithSchemaReadOnly is WithSchema without the enclosing transaction —
	// same connection-pinning safety, but skips the BEGIN/COMMIT round trip
	// and the extra Postgres-side transaction bookkeeping. Measured ~25%
	// additional throughput under load once other bottlenecks were fixed
	// (see lotof.menu.gtw's 2026-07-29 stress test). ONLY safe for fn bodies
	// that issue a single read statement (or several reads) — never use
	// this for multi-statement writes that need atomicity, since there is
	// no rollback if a later statement in fn fails.
	WithSchemaReadOnly(ctx context.Context, schema string, fn func(tx *gorm.DB) error) error
	MigrateTenants(schemas []string, autoMigrateEntities []any) error
}

// DatabaseType defines the type of databases supported
type DatabaseType int

// DatabaseFactory is a factory for creating database instances
type DatabaseFactory struct {
	dsn               string
	enableLogs        bool
	autoMigrateModels []any
}

// New initializes a new DatabaseFactory
func New(dsn string, enableLogs bool, autoMigrateModels []any) *DatabaseFactory {
	if autoMigrateModels == nil {
		autoMigrateModels = []any{}
	}
	return &DatabaseFactory{
		dsn:               dsn,
		enableLogs:        enableLogs,
		autoMigrateModels: autoMigrateModels,
	}
}

// Create creates a database instance based on the given type
func (f *DatabaseFactory) Create(dbType DatabaseType) (Database, error) {
	switch dbType {
	case PostgresDB:
		return postgresql.NewPostgres(f.dsn, f.enableLogs, f.autoMigrateModels), nil
	default:
		return nil, fmt.Errorf("unsupported database type: %v", dbType)
	}
}
