package db

import (
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
	// Add more cases for other database types
	default:
		return nil, fmt.Errorf("unsupported database type: %v", dbType)
	}
}
