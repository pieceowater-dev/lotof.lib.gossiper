package tenancy

import (
	"context"
	"errors"
	"testing"

	"gorm.io/gorm"
)

type fakeHub struct {
	tenants []EncryptedTenant
	err     error
	calls   int
	ensured []string
}

func (h *fakeHub) Tenants(context.Context) ([]EncryptedTenant, error) {
	h.calls++
	return h.tenants, h.err
}

func (h *fakeHub) EnsureAppTenant(_ context.Context, slug, user string) error {
	h.ensured = append(h.ensured, slug+"/"+user)
	return nil
}

func TestManager_TargetVersion(t *testing.T) {
	models := []any{&modelA{}}
	override := ""
	m := NewManager(ManagerConfig{
		Models:          func() []any { return models },
		VersionOverride: func() string { return override },
	})

	computed := m.TargetVersion()
	if computed != ComputeTargetVersion(models) {
		t.Fatalf("expected the model hash, got %q", computed)
	}
	models = []any{&modelA{}, &modelB{}}
	if m.TargetVersion() == computed {
		t.Fatal("the version must follow the model set, which services fill in after building the config")
	}
	override = "manual-1"
	if m.TargetVersion() != "manual-1" {
		t.Fatal("the emergency override wins")
	}
}

func TestManager_ConfigTenantsToleratesHubFailure(t *testing.T) {
	hub := &fakeHub{err: errors.New("hub down")}
	m := NewManager(ManagerConfig{Hub: hub, OpenDB: func() (Database, error) {
		t.Fatal("no database may be opened when discovery failed")
		return nil, nil
	}})
	m.ConfigTenants()
	if hub.calls != 1 {
		t.Fatalf("hub asked %d times", hub.calls)
	}

	hub.err, hub.tenants = nil, nil
	m.ConfigTenants() // zero namespaces: nothing to open either

	NewManager(ManagerConfig{}).ConfigTenants() // no hub at all
}

func TestManager_EnsureAppTenantNeedsBothIDs(t *testing.T) {
	hub := &fakeHub{}
	m := NewManager(ManagerConfig{Hub: hub})
	if err := m.EnsureAppTenant(context.Background(), "", "u"); err == nil {
		t.Fatal("slug is required")
	}
	if err := m.EnsureAppTenant(context.Background(), "ns", ""); err == nil {
		t.Fatal("user is required")
	}
	if err := m.EnsureAppTenant(context.Background(), "ns", "u"); err != nil || len(hub.ensured) != 1 {
		t.Fatalf("err=%v ensured=%v", err, hub.ensured)
	}
}

func TestManager_SyncTenantRejectsBadSecretWithoutTouchingTheDatabase(t *testing.T) {
	synced := false
	m := NewManager(ManagerConfig{AppBundleSecret: "short", OnSynced: func(string) { synced = true }})
	// A nil database would panic if reached; the secret check comes first.
	if m.SyncTenant(nilDB{}, &EncryptedTenant{Namespace: "ns"}) || synced {
		t.Fatal("a secret that is not 32 bytes must fail the sync")
	}
}

// nilDB satisfies Database for paths that must fail before using it.
type nilDB struct{ Database }

func (nilDB) GetDB() *gorm.DB { return nil }
