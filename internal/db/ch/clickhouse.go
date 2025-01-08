package ch

import (
	"fmt"
	"gorm.io/driver/clickhouse"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"log"
	"os"
	"reflect"
	"time"
)

type Clickhouse struct {
	db *gorm.DB
}

// NewClickhouse initializes the ClickHouse instance with a configurable logger
func NewClickhouse(dsn string, enableLogs bool, autoMigrateEntities []any) *Clickhouse {
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

	db, err := gorm.Open(clickhouse.Open(dsn), &gorm.Config{
		Logger: newLogger,
	})
	if err != nil {
		log.Fatalf("Failed to connect to ClickHouse: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("Failed to get db instance: %v", err)
	}
	if err := sqlDB.Ping(); err != nil {
		log.Fatalf("Failed to ping ClickHouse: %v", err)
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

	return &Clickhouse{db: db}
}

// GetDB returns the GORM database instance
func (p *Clickhouse) GetDB() *gorm.DB {
	return p.db
}

// WithTransaction executes a function within a transaction
func (p *Clickhouse) WithTransaction(_ func(tx *gorm.DB) error) error {
	return fmt.Errorf("transactions are not supported in ClickHouse")
}

// SeedData populates the database with dynamic initial data
func (p *Clickhouse) SeedData(data []any) error {
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

func (p *Clickhouse) SwitchSchema(schema string) *gorm.DB {
	if err := p.db.Exec(fmt.Sprintf("USE %s", schema)).Error; err != nil {
		panic(fmt.Errorf("failed to switch schema: %w", err))
	}
	return p.db
}

func (p *Clickhouse) MigrateTenants(_ []string, _ []any) error {
	panic(fmt.Errorf("not implemented yet"))
}
