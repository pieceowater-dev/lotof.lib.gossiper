package tenant

import (
	"errors"
	"fmt"
	"github.com/pieceowater-dev/lotof.lib.gossiper/v2/internal/generic"
	"gorm.io/gorm"
	"log"
	"strings"
)

// ITenantManager defines the interface for tenant management
type ITenantManager interface {
	EncryptTenant(tenant tenant) (string, error)
	SyncTenants(encryptedTenants *[]EncryptedTenant) error
	loadTenants(encryptedTenants *[]EncryptedTenant) error
	seedTenants() error
	decryptTenant(et EncryptedTenant) (tenant, error)
}

// Manager implements ITenantManager for managing tenants
type Manager struct {
	db      *gorm.DB
	tenants []tenant
	secret  []byte // 32-byte KEY for AES-256 encryption
}

// EncryptedTenant represents a tenant with encrypted credentials
type EncryptedTenant struct {
	Namespace   string // Database schema name
	Credentials string // AES-256 encrypted string "username:password"
}

// Data TenantData type to hold the data in [username:password] format
type data string

// ToTenantData converts a tenant to TenantData
func (t tenant) toTenantData() data {
	return data(fmt.Sprintf("%s:%s", t.username, t.password))
}

// tenant represents a tenant with plain text credentials
type tenant struct {
	database string // Schema name / namespace name
	username string
	password string
}

// ToTenant converts TenantData back to a tenant
func (td data) toTenant(database string) (tenant, error) {
	parts := strings.Split(string(td), ":")
	if len(parts) != 2 {
		return tenant{}, errors.New("invalid tenant data format")
	}
	return tenant{
		database: database,
		username: parts[0],
		password: parts[1],
	}, nil
}

// NewTenantManager creates a new TenantManager
func NewTenantManager(db *gorm.DB, secret string) (*Manager, error) {
	if len(secret) != 32 {
		return nil, errors.New("secret must be 32 bytes")
	}
	return &Manager{
		db:     db,
		secret: []byte(secret),
	}, nil
}

// LoadTenants loads and decrypts tenants from encrypted data
func (tm *Manager) loadTenants(encryptedTenants *[]EncryptedTenant) error {
	for _, et := range *encryptedTenants {
		t, err := tm.decryptTenant(et)
		if err != nil {
			return err
		}
		tm.tenants = append(tm.tenants, t)
	}
	return nil
}

// SeedTenants creates schemas and users in the database for all loaded tenants
func (tm *Manager) seedTenants() error {
	for _, t := range tm.tenants {
		if err := tm.seedSingleTenant(t); err != nil {
			log.Printf("Failed to seed tenant: %v", err)
		}
	}
	return nil
}

// seedSingleTenant creates schema, user and grants privileges for a single tenant
func (tm *Manager) seedSingleTenant(t tenant) error {
	if t.database == "" || t.username == "" || t.password == "" {
		return errors.New("tenant data is incomplete")
	}

	// Create schema if it doesn't exist
	createSchemaSQL := fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", t.database)
	if err := tm.db.Exec(createSchemaSQL).Error; err != nil {
		return fmt.Errorf("failed to create schema %s: %v", t.database, err)
	}

	// Create user with password
	createUserSQL := fmt.Sprintf(
		`DO $$ 
				BEGIN
					IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = '%s') THEN
						CREATE USER %s WITH PASSWORD '%s';
					END IF;
				END $$;`,
		t.username, t.username, t.password,
	)
	if err := tm.db.Exec(createUserSQL).Error; err != nil {
		return fmt.Errorf("failed to create user %s: %v", t.username, err)
	}

	// Grant all privileges on schema to user
	grantPrivilegesSQL := fmt.Sprintf("GRANT ALL PRIVILEGES ON SCHEMA %s TO %s", t.database, t.username)
	if err := tm.db.Exec(grantPrivilegesSQL).Error; err != nil {
		return fmt.Errorf("failed to grant privileges on schema %s to user %s: %v", t.database, t.username, err)
	}

	log.Printf("Successfully seeded tenant: %s", t.database)
	return nil
}

// EncryptTenant encrypts tenant credentials using AES-256 encryption
func (tm *Manager) EncryptTenant(t tenant) (string, error) {
	// Convert tenant to TenantData string
	data := string(t.toTenantData())

	// Use generic.EncryptAES256 to encrypt the data
	encryptedData, err := generic.EncryptAES256(string(tm.secret), data)
	if err != nil {
		return "", fmt.Errorf("failed to encrypt tenant data: %w", err)
	}

	return encryptedData, nil
}

// decryptTenant decrypts the encrypted tenant credentials
func (tm *Manager) decryptTenant(et EncryptedTenant) (tenant, error) {
	// Use generic.DecryptAES256 to decrypt the credentials
	decryptedData, err := generic.DecryptAES256(string(tm.secret), et.Credentials)
	if err != nil {
		return tenant{}, fmt.Errorf("failed to decrypt tenant data: %w", err)
	}

	// Convert decrypted data back to tenant
	t, err := data(decryptedData).toTenant(et.Namespace)
	if err != nil {
		return tenant{}, fmt.Errorf("error converting decrypted data to tenant: %v", err)
	}

	return t, nil
}

func (tm *Manager) SyncTenants(encryptedTenants *[]EncryptedTenant) error {
	err := tm.loadTenants(encryptedTenants)
	if err != nil {
		return fmt.Errorf("failed to load tenants: %w", err)
	}

	err = tm.seedTenants()
	if err != nil {
		return fmt.Errorf("failed to seed tenants: %w", err)
	}

	return nil
}
