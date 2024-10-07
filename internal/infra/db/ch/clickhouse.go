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
}

func NewClickHouseDB(config conf.DBClickHouseConfig) *ClickHouseDB {
	return &ClickHouseDB{
		autoMigrate: config.AutoMigrate,
		dsn:         config.EnvClickHouseDBDSN,
		models:      config.Models,
	}
}

func (d *ClickHouseDB) InitDB() {
	dsn := d.getClickHouseDSN()
	var err error
	d.db, err = gorm.Open(clickhouse.Open(dsn), &gorm.Config{})
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

func (d *ClickHouseDB) GetDB() *gorm.DB {
	return d.db
}

func (d *ClickHouseDB) getClickHouseDSN() string {
	envInstance := &env.Env{}
	log.Printf("retrieving connection string by " + d.dsn)
	val, err := envInstance.Get(d.dsn)
	if err != nil {
		return ""
	}
	return val
}
