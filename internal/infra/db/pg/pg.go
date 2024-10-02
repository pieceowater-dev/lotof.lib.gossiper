package pg

import (
	"github.com/pieceowater-dev/lotof.lib.gossiper/internal/conf"
	"github.com/pieceowater-dev/lotof.lib.gossiper/internal/env"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"log"
)

// PGDB is a wrapper struct for managing PostgreSQL database connections
// and handling migrations using GORM.
type PGDB struct {
	db          *gorm.DB // Holds the database connection
	autoMigrate bool     // Determines if auto-migration should run
	dsn         string   // DSN environment variable key for database connection
	models      []any    // Models for database migration
}

// NewPGDB creates a new instance of PGDB, configuring it with options
// for automatic migration, the DSN key, and the models to be migrated.
//
// Parameters:
//   - config: conf.DBPGConfig struct containing AutoMigrate flag, the DSN key,
//     and models slice for database migration.
//
// Returns:
//   - A new PGDB instance.
func NewPGDB(config conf.DBPGConfig) *PGDB {
	return &PGDB{
		autoMigrate: config.AutoMigrate,
		dsn:         config.EnvPostgresDBDSN,
		models:      config.Models,
	}
}

// InitDB initializes the PostgreSQL database connection using GORM and the provided DSN.
// If auto-migration is enabled, it will migrate the provided models automatically.
//
// If auto-migration is disabled, the method will log that manual migration is expected.
//
// Logs an error and terminates the program if the database connection or migration fails.
func (d *PGDB) InitDB() {
	dsn := d.getPostgresDSN()
	var err error
	d.db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
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

// GetDB returns the active PostgreSQL database connection.
//
// Returns:
//   - The *gorm.DB instance representing the database connection.
func (d *PGDB) GetDB() *gorm.DB {
	return d.db
}

// getPostgresDSN retrieves the PostgreSQL DSN from the environment using the key
// provided in the PGDB instance. The DSN is required to establish the database connection.
//
// Logs the process of retrieving the connection string, and returns an empty string
// if the retrieval fails.
//
// Returns:
//   - The DSN string for PostgreSQL connection.
func (d *PGDB) getPostgresDSN() string {
	envInstance := &env.Env{}
	log.Printf("retrieving connection string by " + d.dsn)
	val, err := envInstance.Get(d.dsn)
	if err != nil {
		return ""
	}
	return val
}
