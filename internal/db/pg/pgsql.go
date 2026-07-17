package pg

import (
	"context"
	"fmt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"log"
	"os"
	"reflect"
	"time"

	"github.com/pieceowater-dev/lotof.lib.gossiper/v2/internal/generic"
)

type Postgres struct {
	db *gorm.DB
}

// NewPostgres initializes the Postgres instance with a configurable logger
func NewPostgres(dsn string, enableLogs bool, autoMigrateEntities []any) *Postgres {
	var newLogger logger.Interface
	if enableLogs {
		newLogger = logger.New(
			log.New(os.Stdout, "\r\n", log.LstdFlags),
			logger.Config{
				SlowThreshold:             200 * time.Millisecond,
				LogLevel:                  logger.Info,
				IgnoreRecordNotFoundError: true,
				Colorful:                  true,
			},
		)
	} else {
		newLogger = logger.Default.LogMode(logger.Silent)
	}

	// PreferSimpleProtocol disables server-side prepared statement caching.
	// This connection pool is shared across every tenant, and SwitchSchema
	// below repoints search_path per request on whatever connection gets
	// checked out — a cached prepared plan from one tenant's schema is
	// therefore not safe to reuse against another tenant's identically
	// named table, since Postgres binds a prepared plan's result type to
	// the specific relation OIDs seen at PREPARE time. Reusing it after
	// search_path changes underneath it raises "cached plan must not
	// change result type" (SQLSTATE 0A000). Simple protocol re-sends
	// literal SQL every time, so there's no stale plan to go stale.
	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN:                  dsn,
		PreferSimpleProtocol: true,
	}), &gorm.Config{
		Logger: newLogger,
	})
	if err != nil {
		log.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("Failed to get db instance: %v", err)
	}
	if err := sqlDB.Ping(); err != nil {
		log.Fatalf("Failed to ping PostgreSQL: %v", err)
	}
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	if autoMigrateEntities != nil {
		for _, entity := range autoMigrateEntities {
			if err := db.AutoMigrate(entity); err != nil {
				log.Fatalf("failed to auto-migrate entity: %v", err)
			}
		}
	}

	return &Postgres{db: db}
}

// GetDB returns the GORM database instance
func (p *Postgres) GetDB() *gorm.DB {
	return p.db
}

// WithTransaction executes a function within a transaction
func (p *Postgres) WithTransaction(fn func(tx *gorm.DB) error) error {
	tx := p.db.Begin()
	if tx.Error != nil {
		return tx.Error
	}

	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}

// SeedData populates the database with dynamic initial data
func (p *Postgres) SeedData(data []any) error {
	value := reflect.ValueOf(data)

	for i := 0; i < value.Len(); i++ {
		item := value.Index(i).Interface()

		elemType := reflect.TypeOf(item)
		if elemType.Kind() != reflect.Ptr || elemType.Elem().Kind() != reflect.Struct {
			return fmt.Errorf("invalid data type, expected a pointer to a struct, got %T", item)
		}

		if err := p.db.FirstOrCreate(item).Error; err != nil {
			return fmt.Errorf("failed to seed data: %w", err)
		}
	}
	return nil
}

// SwitchSchema points a checked-out connection's search_path at the given
// tenant schema and immediately releases that connection back to the pool.
//
// Deprecated: this is unsafe to use for anything beyond a diagnostic Exec.
// Postgres's search_path is a per-session (per-connection) setting, while
// *sql.DB pools many physical connections — the very next call against
// GetDB() checks out a connection independently and is NOT guaranteed to
// be the same one this just configured. In practice, under any concurrency
// (or even a tight sequential loop across many schemas, as tenant
// migration does), queries silently run against whatever search_path the
// connection previously had — typically "public" — instead of the
// intended tenant schema, with no error raised anywhere. Use WithSchema
// instead, which pins one connection for the whole operation via a
// transaction. Kept only for source compatibility with existing callers;
// do not use it in new code.
func (p *Postgres) SwitchSchema(schema string) *gorm.DB {
	quoted, err := generic.QuotePGIdentifier(schema)
	if err != nil {
		errDB := p.db.Session(&gorm.Session{})
		errDB.Error = fmt.Errorf("failed to switch schema: %w", err)
		return errDB
	}
	return p.db.Exec(fmt.Sprintf("SET search_path TO %s", quoted))
}

// WithSchema runs fn against a connection pinned to the given tenant
// schema's search_path for the lifetime of the call, then releases it —
// the safe replacement for SwitchSchema()-then-separate-query. A
// transaction is the standard Go database/sql idiom for pinning exactly
// one physical connection across multiple statements (SET search_path,
// then fn's own queries), which is what closes the race described on
// SwitchSchema. Postgres DDL is transactional, so this is equally safe to
// use for AutoMigrate as for ordinary CRUD.
func (p *Postgres) WithSchema(ctx context.Context, schema string, fn func(tx *gorm.DB) error) error {
	quoted, err := generic.QuotePGIdentifier(schema)
	if err != nil {
		return fmt.Errorf("failed to switch schema: %w", err)
	}
	return p.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(fmt.Sprintf("SET search_path TO %s", quoted)).Error; err != nil {
			return fmt.Errorf("failed to set search_path: %w", err)
		}
		return fn(tx)
	})
}

func (p *Postgres) MigrateTenants(schemas []string, autoMigrateEntities []any) error {
	ctx := context.Background()
	for _, schema := range schemas {
		err := p.WithSchema(ctx, schema, func(tx *gorm.DB) error {
			for _, entity := range autoMigrateEntities {
				if err := tx.AutoMigrate(entity); err != nil {
					return fmt.Errorf("failed to auto-migrate entity for schema %s: %w", schema, err)
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
		log.Println("Tenant migrated:", schema)
	}

	return nil
}
