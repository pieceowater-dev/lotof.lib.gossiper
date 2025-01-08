package pg

import (
	"fmt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"log"
	"os"
	"reflect"
	"time"
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

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
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

func (p *Postgres) SwitchSchema(schema string) *gorm.DB {
	if err := p.db.Exec(fmt.Sprintf("SET search_path TO %s", schema)).Error; err != nil {
		panic(fmt.Errorf("failed to switch schema: %w", err))
	}
	return p.db
}

func (p *Postgres) MigrateTenants(schemas []string, autoMigrateEntities []any) error {
	for _, schema := range schemas {
		if err := p.SwitchSchema(schema).Error; err != nil {
			return fmt.Errorf("failed to switch to schema %s: %w", schema, err)
		}

		for _, entity := range autoMigrateEntities {
			if err := p.db.AutoMigrate(entity); err != nil {
				return fmt.Errorf("failed to auto-migrate entity for schema %s: %w", schema, err)
			}
		}
		log.Println("Tenant migrated:", schema)
	}

	return nil
}
