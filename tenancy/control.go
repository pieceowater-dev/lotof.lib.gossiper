package tenancy

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"sync"
	"time"

	"gorm.io/gorm"
)

const (
	retryBaseDelay = 5 * time.Second
	retryMaxDelay  = 5 * time.Minute
)

// DB is the slice of gossiper.Database this package needs. Taking an
// interface keeps tenancy importable from a service's cfg package without
// dragging the whole gossiper root in.
type DB interface {
	GetDB() *gorm.DB
}

// Control is the migration bookkeeping for one service: which tenant schema
// is at which version, who is migrating it right now, and how long to back
// off after a failure. State lives in public.tenant_migration_state, keyed
// by (service, tenant), so every pod of the service sees the same answer.
type Control struct {
	// ServiceName keys this service's rows; two services migrating the same
	// tenant do not block each other.
	ServiceName string
	// TargetVersion is what a tenant has to be at to count as ready --
	// usually ComputeTargetVersion over the AutoMigrate models.
	TargetVersion func() string

	db DB

	stateTableMu    sync.Mutex
	stateTableReady bool
}

func NewControl(db DB, serviceName string, targetVersion func() string) *Control {
	return &Control{ServiceName: serviceName, TargetVersion: targetVersion, db: db}
}

func (c *Control) target() string {
	if c.TargetVersion == nil {
		return ""
	}
	return c.TargetVersion()
}

// EnsureStateTable creates the bookkeeping table once per process.
//
// It used to run on every IsReady call -- that is, on every unary RPC, since
// the readiness gate checks once per request. "CREATE TABLE IF NOT EXISTS"
// is not free: Postgres still parses it and takes catalog locks. The success
// flag is deliberately not a sync.Once, so a transient database outage at
// startup heals on the next call instead of wedging the process for its
// lifetime.
func (c *Control) EnsureStateTable(ctx context.Context) error {
	c.stateTableMu.Lock()
	ready := c.stateTableReady
	c.stateTableMu.Unlock()
	if ready {
		return nil
	}

	if err := c.createStateTable(ctx); err != nil {
		return err
	}

	c.stateTableMu.Lock()
	c.stateTableReady = true
	c.stateTableMu.Unlock()
	return nil
}

func (c *Control) createStateTable(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS public.tenant_migration_state (
			service_name TEXT NOT NULL,
			tenant_namespace TEXT NOT NULL,
			target_version TEXT NOT NULL,
			applied_version TEXT,
			status TEXT NOT NULL,
			failure_count INT NOT NULL DEFAULT 0,
			next_retry_at TIMESTAMPTZ,
			started_at TIMESTAMPTZ,
			finished_at TIMESTAMPTZ,
			error TEXT,
			PRIMARY KEY (service_name, tenant_namespace)
		)`,
		`ALTER TABLE public.tenant_migration_state ADD COLUMN IF NOT EXISTS failure_count INT NOT NULL DEFAULT 0`,
		`ALTER TABLE public.tenant_migration_state ADD COLUMN IF NOT EXISTS next_retry_at TIMESTAMPTZ`,
	}
	for _, q := range statements {
		if err := c.db.GetDB().WithContext(ctx).Exec(q).Error; err != nil {
			return err
		}
	}
	return nil
}

// IsReady reports whether this tenant's schema is migrated to the current
// target version. A tenant with no row at all is not ready -- and never will
// be without a migration, which is what Info lets a caller distinguish.
func (c *Control) IsReady(ctx context.Context, namespace string) (bool, error) {
	if namespace == "" {
		return true, nil
	}
	if err := c.EnsureStateTable(ctx); err != nil {
		return false, err
	}

	var status string
	var applied sql.NullString
	err := c.db.GetDB().WithContext(ctx).
		Raw(`SELECT status, applied_version FROM public.tenant_migration_state WHERE service_name = ? AND tenant_namespace = ?`,
			c.ServiceName, namespace).
		Row().Scan(&status, &applied)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return status == "done" && applied.Valid && applied.String == c.target(), nil
}

// Info returns the tenant's last recorded status and applied version. Empty
// strings mean this service has no record of the tenant at all, which is a
// different thing from "migration in progress" and worth answering
// differently (audit E6).
func (c *Control) Info(ctx context.Context, namespace string) (status string, appliedVersion string, err error) {
	if namespace == "" {
		return "", "", nil
	}
	if err := c.EnsureStateTable(ctx); err != nil {
		return "", "", err
	}

	var applied sql.NullString
	err = c.db.GetDB().WithContext(ctx).
		Raw(`SELECT status, applied_version FROM public.tenant_migration_state WHERE service_name = ? AND tenant_namespace = ?`,
			c.ServiceName, namespace).
		Row().Scan(&status, &applied)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", "", nil
		}
		return "", "", err
	}
	return status, applied.String, nil
}

// Migrate runs steps for one tenant under a Postgres advisory lock, so only
// one pod migrates a given tenant at a time, and records the outcome. A
// failure schedules an exponential backoff: a tenant whose migration keeps
// failing is retried every 5s, then 10s, and so on up to 5 minutes, instead
// of being hammered by every request that arrives.
func (c *Control) Migrate(ctx context.Context, namespace string, steps ...func(context.Context) error) error {
	if namespace == "" {
		return fmt.Errorf("tenant namespace is required")
	}
	if err := c.EnsureStateTable(ctx); err != nil {
		return err
	}

	failureCount, nextRetryAt, err := c.failureState(ctx, namespace)
	if err != nil {
		return err
	}
	if nextRetryAt != nil && time.Now().UTC().Before(*nextRetryAt) {
		return fmt.Errorf("tenant migration backoff active for namespace %q until %s",
			namespace, nextRetryAt.UTC().Format(time.RFC3339))
	}

	lockConn, err := c.tryLock(ctx, namespace)
	if err != nil {
		return err
	}
	if lockConn == nil {
		// Somebody else is migrating this tenant right now. Waiting for them beats failing the
		// caller (a request that just arrived for a new tenant): if they finish the job there is
		// nothing left to do.
		done, waitErr := c.waitForOther(ctx, namespace)
		if waitErr != nil {
			return waitErr
		}
		if done {
			return nil
		}
		return fmt.Errorf("tenant migration is already running for namespace %q", namespace)
	}
	defer c.unlock(ctx, lockConn, namespace)

	started := time.Now().UTC()
	if err := c.record(ctx, namespace, "", "running", failureCount, nil, &started, nil, nil); err != nil {
		return err
	}

	for _, step := range steps {
		if step == nil {
			continue
		}
		if err := step(ctx); err != nil {
			text := err.Error()
			finished := time.Now().UTC()
			retry := finished.Add(retryDelay(failureCount + 1))
			_ = c.record(ctx, namespace, "", "failed", failureCount+1, &retry, &started, &finished, &text)
			return err
		}
	}

	finished := time.Now().UTC()
	return c.record(ctx, namespace, c.target(), "done", 0, nil, &started, &finished, nil)
}

// lockWaitTimeout and lockWaitPoll bound how long Migrate waits for another migration of the same tenant.
const (
	lockWaitTimeout = 25 * time.Second
	lockWaitPoll    = 500 * time.Millisecond
)

// waitForOther polls until the migration running elsewhere has brought the tenant to the target
// version (true), or the wait runs out (false).
func (c *Control) waitForOther(ctx context.Context, namespace string) (bool, error) {
	deadline := time.NewTimer(lockWaitTimeout)
	defer deadline.Stop()
	tick := time.NewTicker(lockWaitPoll)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-deadline.C:
			return false, nil
		case <-tick.C:
			status, applied, err := c.Info(ctx, namespace)
			if err != nil {
				return false, err
			}
			if status == "done" && applied != "" && applied == c.target() {
				return true, nil
			}
		}
	}
}

func (c *Control) failureState(ctx context.Context, namespace string) (int, *time.Time, error) {
	var count int
	var next sql.NullTime
	err := c.db.GetDB().WithContext(ctx).
		Raw(`SELECT failure_count, next_retry_at FROM public.tenant_migration_state WHERE service_name = ? AND tenant_namespace = ?`,
			c.ServiceName, namespace).
		Row().Scan(&count, &next)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, nil, nil
		}
		return 0, nil, err
	}
	if next.Valid {
		t := next.Time
		return count, &t, nil
	}
	return count, nil, nil
}

// tryLock takes the advisory lock on a connection of its own. The lock belongs to the Postgres
// session, so taking it through the pool and releasing it through the pool could hit two
// different connections: the unlock would do nothing and the lock would stay held until that
// connection died, wedging the tenant ("migration is already running") for good.
func (c *Control) tryLock(ctx context.Context, namespace string) (*sql.Conn, error) {
	sqlDB, err := c.db.GetDB().DB()
	if err != nil {
		return nil, err
	}
	conn, err := sqlDB.Conn(ctx)
	if err != nil {
		return nil, err
	}
	var locked bool
	if err := conn.QueryRowContext(ctx, "SELECT pg_try_advisory_lock(hashtext($1), hashtext($2))", c.ServiceName, namespace).Scan(&locked); err != nil {
		_ = conn.Close()
		return nil, err
	}
	if !locked {
		_ = conn.Close()
		return nil, nil
	}
	return conn, nil
}

func (c *Control) unlock(ctx context.Context, conn *sql.Conn, namespace string) {
	// The session goes back to the pool afterwards, so an unlock that failed must not leave the
	// lock behind: closing a connection that cannot unlock discards the session instead.
	if _, err := conn.ExecContext(context.WithoutCancel(ctx), "SELECT pg_advisory_unlock(hashtext($1), hashtext($2))", c.ServiceName, namespace); err != nil {
		_ = conn.Raw(func(any) error { return driver.ErrBadConn })
	}
	_ = conn.Close()
}

func (c *Control) record(ctx context.Context, namespace, appliedVersion, status string, failureCount int, nextRetryAt, startedAt, finishedAt *time.Time, errText *string) error {
	const query = `
INSERT INTO public.tenant_migration_state
(service_name, tenant_namespace, target_version, applied_version, status, failure_count, next_retry_at, started_at, finished_at, error)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (service_name, tenant_namespace)
DO UPDATE SET
	target_version = EXCLUDED.target_version,
	applied_version = EXCLUDED.applied_version,
	status = EXCLUDED.status,
	failure_count = EXCLUDED.failure_count,
	next_retry_at = EXCLUDED.next_retry_at,
	started_at = EXCLUDED.started_at,
	finished_at = EXCLUDED.finished_at,
	error = EXCLUDED.error`

	return c.db.GetDB().WithContext(ctx).Exec(query,
		c.ServiceName, namespace, c.target(), appliedVersion, status,
		failureCount, nextRetryAt, startedAt, finishedAt, errText).Error
}

func retryDelay(failureCount int) time.Duration {
	if failureCount <= 0 {
		return retryBaseDelay
	}
	delay := retryBaseDelay
	for i := 1; i < failureCount; i++ {
		delay *= 2
		if delay >= retryMaxDelay {
			return retryMaxDelay
		}
	}
	if delay > retryMaxDelay {
		return retryMaxDelay
	}
	return delay
}
