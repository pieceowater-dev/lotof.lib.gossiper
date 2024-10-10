package ch

import (
	"github.com/pieceowater-dev/lotof.lib.gossiper/internal/conf"
	"github.com/pieceowater-dev/lotof.lib.gossiper/internal/env"
	"gorm.io/driver/clickhouse"
	"gorm.io/gorm"
	"log"
)

type ClickHouseDB struct {
	db          *gorm.DB // Holds the database connection
	autoMigrate bool     // Determines if auto-migration should run
	dsn         string   // DSN environment variable key for database connection
	models      []any    // Models for database migration
	gormConf    *gorm.Config
}

// NewClickHouseDB creates a new instance of ClickHouseDB, configuring it with options
// for automatic migration, the DSN key, and the models to be migrated.
//
// Parameters:
//   - config: conf.DBClickHouseConfig struct containing AutoMigrate flag, the DSN key,
//     and models slice for database migration.
//
// Returns:
//   - A new ClickHouseDB instance.
func NewClickHouseDB(config conf.DBClickHouseConfig) *ClickHouseDB {
	// Provide a default GORM config if none is passed to avoid nil pointer dereference.
	if config.GORMConfig == nil {
		config.GORMConfig = &gorm.Config{}
	}

	return &ClickHouseDB{
		autoMigrate: config.AutoMigrate,
		dsn:         config.EnvClickHouseDBDSN,
		models:      config.Models,
		gormConf:    config.GORMConfig,
	}
}

// InitDB initializes the ClickHouse database connection using GORM and the provided DSN.
// If auto-migration is enabled, it will migrate the provided models automatically.
//
// If auto-migration is disabled, the method will log that manual migration is expected.
//
// Logs an error and terminates the program if the database connection or migration fails.
func (d *ClickHouseDB) InitDB() {
	dsn := d.getClickHouseDSN()
	if dsn == "" {
		log.Fatalf("DSN not found: %s", d.dsn)
	}

	var err error
	d.db, err = gorm.Open(clickhouse.Open(dsn), d.gormConf)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	log.Printf("connected to database")

	if d.autoMigrate {
		err = d.db.AutoMigrate(d.models...)
		if err != nil {
			log.Fatalf("failed to auto-migrate: %v", err)
		}
		log.Printf("auto-migrate complete")
	} else {
		log.Println("Manual migration mode enabled. Skipping auto-migration.")
	}
}

// GetDB returns the active ClickHouse database connection.
//
// Returns:
//   - The *gorm.DB instance representing the database connection.
func (d *ClickHouseDB) GetDB() *gorm.DB {
	return d.db
}

// getClickHouseDSN retrieves the ClickHouse DSN from the environment using the key
// provided in the ClickHouseDB instance. The DSN is required to establish the database connection.
//
// Logs the process of retrieving the connection string, and returns an empty string
// if the retrieval fails.
//
// Returns:
//   - The DSN string for ClickHouse connection.
func (d *ClickHouseDB) getClickHouseDSN() string {
	envInstance := &env.Env{}
	log.Printf("retrieving connection string by " + d.dsn)
	val, err := envInstance.Get(d.dsn)
	if err != nil {
		log.Printf("failed to retrieve DSN: %v", err)
		return ""
	}
	if val == "" {
		log.Printf("DSN is empty for key: %s", d.dsn)
	}
	return val
}
