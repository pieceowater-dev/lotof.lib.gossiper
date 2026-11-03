package tenancy

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"sync"
	"time"

	"gorm.io/gorm"

	pgdb "github.com/pieceowater-dev/lotof.lib.gossiper/v2/internal/db"
	"github.com/pieceowater-dev/lotof.lib.gossiper/v2/internal/tenant"
)

// Database is the Postgres handle a tenant-scoped service holds
// (gossiper.Database).
type Database = pgdb.Database

// EncryptedTenant is a namespace together with its encrypted schema
// credentials, as Hub hands them out (gossiper.EncryptedTenant).
type EncryptedTenant = tenant.EncryptedTenant

// Hub is the part of the platform's tenant registry a service talks to. The
// gRPC clients for it are generated per service, so the service implements
// this and the provisioning logic stays here.
type Hub interface {
	// Tenants lists the namespaces registered for this app
	// (GatewayNamespacesByApp), credentials still encrypted.
	Tenants(ctx context.Context) ([]EncryptedTenant, error)
	// EnsureAppTenant asks Hub to provision this app's tenant schema for a
	// namespace the given user genuinely belongs to, without recording an app
	// installation.
	EnsureAppTenant(ctx context.Context, namespaceSlug string, userID string) error
}

// ManagerConfig wires a Manager to one service.
type ManagerConfig struct {
	// ServiceName keys this service's rows in tenant_migration_state.
	ServiceName string
	// AppBundleName is only used for log lines.
	AppBundleName string
	// AppBundleSecret is the 32-byte key tenant credentials are encrypted with.
	AppBundleSecret string

	// DatabaseDSN, DebugSQL open the connection the warmup migrates through,
	// unless OpenDB is set.
	DatabaseDSN string
	DebugSQL    bool
	// OpenDB overrides how the warmup connection is opened.
	OpenDB func() (Database, error)

	// Models is the AutoMigrate set. It is a func because services fill the
	// list in after their config is built. The target version is derived
	// from it, so a schema change on any model re-migrates every tenant.
	Models func() []any
	// VersionOverride is an emergency manual target version; "" means use
	// the computed one.
	VersionOverride func() string

	// Hub is the tenant registry; nil disables discovery and provisioning on
	// first connection.
	Hub Hub

	// BeforeModels runs inside the tenant's schema transaction before the
	// models are AutoMigrated (constraint relaxations that AutoMigrate needs).
	BeforeModels func(ctx context.Context, tx *gorm.DB, namespace string) error
	// AfterModels runs inside the same transaction after AutoMigrate: raw
	// SQL migrations, backfills.
	AfterModels func(ctx context.Context, tx *gorm.DB, namespace string) error
	// AfterSchema runs once the schema transaction is committed (search
	// indexes, anything that manages its own connection).
	AfterSchema func(ctx context.Context, db Database, namespace string) error

	// OnSynced is called after a tenant's schema and role were created.
	OnSynced func(namespace string)
}

// Manager is the per-tenant provisioning of one service: discover the
// namespaces Hub knows for this app, create each schema, AutoMigrate it
// under the migration-state bookkeeping, and answer "is this tenant ready".
// It replaces the cfg/tenants.go, tenant_migration_*.go and
// tenant_warmup.go that every tenant-scoped service carried (audit 2).
type Manager struct {
	cfg ManagerConfig

	controlOnce sync.Once
	control     *Control

	warmupOnce sync.Once
	warmup     *Warmup

	dbMu sync.Mutex
	db   Database
}

// NewManager builds the manager for one service.
func NewManager(cfg ManagerConfig) *Manager {
	return &Manager{cfg: cfg}
}

func (m *Manager) models() []any {
	if m.cfg.Models == nil {
		return nil
	}
	return m.cfg.Models()
}

// TargetVersion is the version a tenant has to be at to count as ready: the
// manual override if one is set, otherwise the hash of the model set.
func (m *Manager) TargetVersion() string {
	if m.cfg.VersionOverride != nil {
		if v := m.cfg.VersionOverride(); v != "" {
			return v
		}
	}
	return ComputeTargetVersion(m.models())
}

func (m *Manager) tenantControl(database Database) *Control {
	m.controlOnce.Do(func() {
		m.control = NewControl(database, m.cfg.ServiceName, m.TargetVersion)
	})
	return m.control
}

// IsReady reports whether the tenant's schema is migrated to the target version.
func (m *Manager) IsReady(ctx context.Context, database Database, namespace string) (bool, error) {
	return m.tenantControl(database).IsReady(ctx, namespace)
}

// Info returns the tenant's last recorded status and applied version -- empty
// strings when this service has no record of the tenant at all, which is a
// different answer from "migration in progress".
func (m *Manager) Info(ctx context.Context, database Database, namespace string) (status string, appliedVersion string, err error) {
	return m.tenantControl(database).Info(ctx, namespace)
}

// Status is the live readiness report behind a GetTenantStatus RPC: the
// bookkeeping says the tenant was migrated to the target version, and a cheap
// read confirms the schema is queryable right now.
func (m *Manager) Status(ctx context.Context, database Database, namespace string) (ready bool, appliedVersion, targetVersion string, err error) {
	status, appliedVersion, err := m.Info(ctx, database, namespace)
	if err != nil {
		return false, "", "", err
	}
	targetVersion = m.TargetVersion()

	ready = status == "done" && appliedVersion != "" && appliedVersion == targetVersion
	if ready {
		// Bookkeeping alone only proves the tenant *was* migrated
		// successfully at some point -- a cheap live read closes the gap to
		// "is it actually queryable right now."
		if liveErr := database.WithSchemaReadOnly(ctx, namespace, func(tx *gorm.DB) error {
			return tx.Exec("SELECT 1").Error
		}); liveErr != nil {
			ready = false
		}
	}
	return ready, appliedVersion, targetVersion, nil
}

// SyncTenant creates the PostgreSQL schema (and role) for one tenant from its
// encrypted credentials.
func (m *Manager) SyncTenant(database Database, t *EncryptedTenant) bool {
	log.Printf("[SyncTenant] Starting schema sync for namespace: %s", t.Namespace)

	tm, err := tenant.NewTenantManager(database.GetDB(), m.cfg.AppBundleSecret)
	if err != nil {
		log.Printf("[SyncTenant] Failed to create TenantManager: %v", err)
		return false
	}

	if err := tm.SyncTenants(&[]EncryptedTenant{*t}); err != nil {
		log.Printf("[SyncTenant] Failed to sync tenant '%s': %v", t.Namespace, err)
		return false
	}

	if m.cfg.OnSynced != nil {
		m.cfg.OnSynced(t.Namespace)
	}
	log.Printf("[SyncTenant] Successfully synced schema for namespace: %s", t.Namespace)
	return true
}

// MigrateTenant syncs the tenant's schema and runs migrateFn under an
// advisory lock, recording the outcome so other pods (and the next request)
// can see it.
func (m *Manager) MigrateTenant(ctx context.Context, database Database, t *EncryptedTenant, migrateFn func(context.Context, string) error) error {
	if t == nil || t.Namespace == "" {
		return fmt.Errorf("tenant namespace is required")
	}

	return m.tenantControl(database).Migrate(ctx, t.Namespace,
		func(context.Context) error {
			if !m.SyncTenant(database, t) {
				return fmt.Errorf("sync tenant failed for namespace %q", t.Namespace)
			}
			return nil
		},
		func(ctx context.Context) error {
			if migrateFn == nil {
				return nil
			}
			return migrateFn(ctx, t.Namespace)
		},
	)
}

// MigrateSchema AutoMigrates models inside the namespace's schema, with the
// configured hooks around it. models == nil means the configured set. Wrap
// it in MigrateTenant for the locking and bookkeeping.
func (m *Manager) MigrateSchema(ctx context.Context, database Database, namespace string, models []any, logger *slog.Logger) error {
	if models == nil {
		models = m.models()
	}
	if logger == nil {
		logger = slog.Default()
	}

	err := database.WithSchema(ctx, namespace, func(tx *gorm.DB) error {
		if m.cfg.BeforeModels != nil {
			if err := m.cfg.BeforeModels(ctx, tx, namespace); err != nil {
				return err
			}
		}
		logger.Info("Running AutoMigrate", slog.Int("entity_count", len(models)))
		for i, model := range models {
			logger.Debug("Migrating entity",
				slog.Int("progress", i+1), slog.Int("total", len(models)),
				slog.String("entity_type", fmt.Sprintf("%T", model)))
			if err := tx.AutoMigrate(model); err != nil {
				logger.Error("Failed to auto-migrate entity",
					slog.String("entity_type", fmt.Sprintf("%T", model)), slog.Any("error", err))
				return fmt.Errorf("auto-migrate failed for namespace '%s', model '%T': %w", namespace, model, err)
			}
		}
		if m.cfg.AfterModels != nil {
			return m.cfg.AfterModels(ctx, tx, namespace)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if m.cfg.AfterSchema != nil {
		return m.cfg.AfterSchema(ctx, database, namespace)
	}
	return nil
}

// openDB returns the connection the warmup migrates through, opened once.
// It used to be opened (and its pool never closed) on every warmup scan.
func (m *Manager) openDB() (Database, error) {
	m.dbMu.Lock()
	defer m.dbMu.Unlock()
	if m.db != nil {
		return m.db, nil
	}
	var (
		database Database
		err      error
	)
	if m.cfg.OpenDB != nil {
		database, err = m.cfg.OpenDB()
	} else {
		database, err = pgdb.New(m.cfg.DatabaseDSN, m.cfg.DebugSQL, m.models()).Create(pgdb.PostgresDB)
	}
	if err != nil {
		return nil, err
	}
	m.db = database
	return database, nil
}

// ConfigTenants discovers all namespaces registered for this app from Hub and
// provisions (creates schema + AutoMigrate) each one. A failure is logged,
// not fatal: schemas are also created on demand through NewTenant.
func (m *Manager) ConfigTenants() {
	log.Printf("=== ConfigTenants: Starting namespace discovery and schema initialization ===")

	defer func() {
		if r := recover(); r != nil {
			log.Printf("ConfigTenants recovered from panic: %v", r)
		}
	}()

	if m.cfg.Hub == nil {
		return
	}

	ctx := context.Background()

	log.Printf("[ConfigTenants] Requesting namespaces for app bundle: '%s'", m.cfg.AppBundleName)
	tenants, err := m.cfg.Hub.Tenants(ctx)
	if err != nil {
		log.Printf("[ConfigTenants] Error requesting namespaces: %v", err)
		log.Printf("[ConfigTenants] Not critical -- schemas will be created on-demand via NewTenant gRPC endpoint")
		return
	}

	count := len(tenants)
	log.Printf("[ConfigTenants] Received %d namespace(s) from Hub Gateway", count)
	if count == 0 {
		log.Printf("[ConfigTenants] No existing namespaces for app '%s' -- schemas will be created on first access", m.cfg.AppBundleName)
		return
	}

	database, err := m.openDB()
	if err != nil {
		log.Printf("[ConfigTenants] Failed to create database instance: %v", err)
		return
	}

	for i := range tenants {
		t := tenants[i]
		log.Printf("[ConfigTenants] [%d/%d] Processing namespace: '%s'", i+1, count, t.Namespace)

		err := m.MigrateTenant(ctx, database, &t, func(ctx context.Context, namespace string) error {
			return m.MigrateSchema(ctx, database, namespace, nil, nil)
		})
		if err != nil {
			log.Printf("[ConfigTenants] Migration failed for namespace '%s': %v", t.Namespace, err)
			continue
		}
		log.Printf("[ConfigTenants] AutoMigrate completed for namespace: '%s'", t.Namespace)
	}

	log.Printf("=== ConfigTenants: Warmup finished for %d namespace(s) ===", count)
}

// TriggerWarmupAsync re-discovers this app's namespaces in the background:
// one scan at a time, and not again within the cooldown.
func (m *Manager) TriggerWarmupAsync() {
	m.warmupOnce.Do(func() {
		m.warmup = NewWarmup(m.ConfigTenants)
	})
	m.warmup.TriggerAsync()
}

// DefaultWarmupInterval is how often the warmup loop rescans.
const DefaultWarmupInterval = 10 * time.Minute

// StartWarmupLoop periodically re-discovers the namespaces registered for
// this app and re-provisions any that changed -- a namespace added after
// startup, or a schema change deployed while a tenant's process was already
// up, still gets picked up without a restart. The interval comes from
// TENANT_MIGRATION_POLL_INTERVAL (a Go duration), 10 minutes by default. The
// returned func stops the loop.
func (m *Manager) StartWarmupLoop() (stop func()) {
	interval := DefaultWarmupInterval
	if raw := os.Getenv("TENANT_MIGRATION_POLL_INTERVAL"); raw != "" {
		if parsed, err := time.ParseDuration(raw); err == nil && parsed > 0 {
			interval = parsed
		}
	}

	loopCtx, cancel := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				m.TriggerWarmupAsync()
			case <-loopCtx.Done():
				return
			}
		}
	}()
	return cancel
}

// Readiness returns the request gate for this service, bound to database.
// excludedMethods pass through ungated.
func (m *Manager) Readiness(database Database, excludedMethods []string) *Readiness {
	return NewReadiness(managerBackend{m: m, db: database}, excludedMethods)
}

// managerBackend adapts a Manager and its database to ReadinessBackend.
type managerBackend struct {
	m  *Manager
	db Database
}

func (b managerBackend) IsReady(ctx context.Context, namespace string) (bool, error) {
	return b.m.IsReady(ctx, b.db, namespace)
}

func (b managerBackend) Info(ctx context.Context, namespace string) (string, string, error) {
	return b.m.Info(ctx, b.db, namespace)
}

func (b managerBackend) TriggerWarmup() { b.m.TriggerWarmupAsync() }

func (b managerBackend) EnsureAppTenant(ctx context.Context, namespaceSlug, userID string) error {
	if b.m.cfg.Hub == nil {
		return fmt.Errorf("no hub configured")
	}
	return b.m.cfg.Hub.EnsureAppTenant(ctx, namespaceSlug, userID)
}

// EnsureAppTenant asks Hub to provision this app's tenant for a member of the
// namespace (see Hub).
func (m *Manager) EnsureAppTenant(ctx context.Context, namespaceSlug, userID string) error {
	if namespaceSlug == "" || userID == "" {
		return fmt.Errorf("namespaceSlug and userID are required")
	}
	if m.cfg.Hub == nil {
		return fmt.Errorf("no hub configured")
	}
	return m.cfg.Hub.EnsureAppTenant(ctx, namespaceSlug, userID)
}
