package tenant

import (
	"strings"
	"testing"
)

const testSecret = "01234567890123456789012345678901" // 32 bytes

func TestNewTenantManager(t *testing.T) {
	t.Run("valid 32-byte secret succeeds", func(t *testing.T) {
		mgr, err := NewTenantManager(nil, testSecret)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if mgr == nil {
			t.Fatal("expected non-nil manager")
		}
	})

	t.Run("short secret rejected", func(t *testing.T) {
		_, err := NewTenantManager(nil, "short")
		if err == nil {
			t.Error("expected error for short secret, got nil")
		}
	})

	t.Run("long secret rejected", func(t *testing.T) {
		_, err := NewTenantManager(nil, strings.Repeat("x", 33))
		if err == nil {
			t.Error("expected error for long secret, got nil")
		}
	})

	t.Run("empty secret rejected", func(t *testing.T) {
		_, err := NewTenantManager(nil, "")
		if err == nil {
			t.Error("expected error for empty secret, got nil")
		}
	})
}

func TestTenantDataRoundTrip(t *testing.T) {
	tn := tenant{database: "acme", username: "acme_user", password: "s3cr3t"}

	td := tn.toTenantData()
	got, err := td.toTenant("acme")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.database != "acme" || got.username != "acme_user" || got.password != "s3cr3t" {
		t.Errorf("round trip mismatch: got %+v", got)
	}
}

func TestData_ToTenant_InvalidFormat(t *testing.T) {
	tests := []struct {
		name string
		td   data
	}{
		{name: "missing colon", td: data("nocolonhere")},
		{name: "too many parts", td: data("a:b:c")},
		{name: "empty", td: data("")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := tt.td.toTenant("db"); err == nil {
				t.Errorf("expected error for %q, got nil", tt.td)
			}
		})
	}
}

func TestManager_EncryptDecryptTenant_RoundTrip(t *testing.T) {
	mgr, err := NewTenantManager(nil, testSecret)
	if err != nil {
		t.Fatalf("unexpected error creating manager: %v", err)
	}

	tn := tenant{database: "acme", username: "acme_user", password: "s3cr3t"}
	encrypted, err := mgr.EncryptTenant(tn)
	if err != nil {
		t.Fatalf("EncryptTenant unexpected error: %v", err)
	}

	decrypted, err := mgr.decryptTenant(EncryptedTenant{Namespace: "acme", Credentials: encrypted})
	if err != nil {
		t.Fatalf("decryptTenant unexpected error: %v", err)
	}

	if decrypted.database != "acme" || decrypted.username != "acme_user" || decrypted.password != "s3cr3t" {
		t.Errorf("decrypted tenant mismatch: got %+v", decrypted)
	}
}

func TestManager_DecryptTenant_InvalidCiphertext(t *testing.T) {
	mgr, err := NewTenantManager(nil, testSecret)
	if err != nil {
		t.Fatalf("unexpected error creating manager: %v", err)
	}

	_, err = mgr.decryptTenant(EncryptedTenant{Namespace: "acme", Credentials: "not-valid-base64!!!"})
	if err == nil {
		t.Error("expected error for invalid ciphertext, got nil")
	}
}

func TestManager_DecryptTenant_WrongSecretFailsToParse(t *testing.T) {
	mgrA, _ := NewTenantManager(nil, testSecret)
	mgrB, _ := NewTenantManager(nil, "abcdefghijabcdefghijabcdefghijAB") // different 32-byte secret

	tn := tenant{database: "acme", username: "acme_user", password: "s3cr3t"}
	encrypted, err := mgrA.EncryptTenant(tn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Decrypting with the wrong key produces garbage bytes. It may or may not
	// contain a literal colon, so just assert it never reproduces the
	// original plaintext credentials.
	got, err := mgrB.decryptTenant(EncryptedTenant{Namespace: "acme", Credentials: encrypted})
	if err == nil && got.password == "s3cr3t" {
		t.Error("expected decryption with wrong secret to not recover original password")
	}
}

func TestManager_LoadTenants_Success(t *testing.T) {
	mgr, _ := NewTenantManager(nil, testSecret)

	tn := tenant{database: "acme", username: "acme_user", password: "s3cr3t"}
	encrypted, err := mgr.EncryptTenant(tn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	list := []EncryptedTenant{{Namespace: "acme", Credentials: encrypted}}
	if err := mgr.loadTenants(&list); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mgr.tenants) != 1 {
		t.Fatalf("expected 1 loaded tenant, got %d", len(mgr.tenants))
	}
	if mgr.tenants[0].username != "acme_user" {
		t.Errorf("expected username acme_user, got %q", mgr.tenants[0].username)
	}
}

func TestManager_LoadTenants_DecryptFailureStopsLoading(t *testing.T) {
	mgr, _ := NewTenantManager(nil, testSecret)

	list := []EncryptedTenant{{Namespace: "acme", Credentials: "not-valid-base64!!!"}}
	if err := mgr.loadTenants(&list); err == nil {
		t.Error("expected error when a tenant fails to decrypt, got nil")
	}
}

func TestManager_SeedSingleTenant_IncompleteData(t *testing.T) {
	mgr, _ := NewTenantManager(nil, testSecret)

	tests := []struct {
		name string
		tn   tenant
	}{
		{name: "missing database", tn: tenant{username: "u", password: "p"}},
		{name: "missing username", tn: tenant{database: "d", password: "p"}},
		{name: "missing password", tn: tenant{database: "d", username: "u"}},
		{name: "all empty", tn: tenant{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := mgr.seedSingleTenant(tt.tn); err == nil {
				t.Error("expected error for incomplete tenant data, got nil")
			}
		})
	}
}

func TestManager_SeedSingleTenant_InvalidIdentifiers(t *testing.T) {
	mgr, _ := NewTenantManager(nil, testSecret)

	tests := []struct {
		name string
		tn   tenant
	}{
		{name: "invalid schema name", tn: tenant{database: "bad; DROP TABLE users; --", username: "u", password: "p"}},
		{name: "invalid username", tn: tenant{database: "d", username: "bad name", password: "p"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := mgr.seedSingleTenant(tt.tn); err == nil {
				t.Error("expected error for invalid identifier, got nil")
			}
		})
	}
}

func TestManager_SeedTenants_NeverReturnsErrorOnPerTenantFailure(t *testing.T) {
	mgr, _ := NewTenantManager(nil, testSecret)
	// Incomplete tenant fails validation before touching the DB, so this
	// exercises seedTenants' per-tenant error handling without needing a
	// real *gorm.DB.
	mgr.tenants = []tenant{{database: "", username: "", password: ""}}

	if err := mgr.seedTenants(); err != nil {
		t.Errorf("seedTenants should swallow per-tenant errors and return nil, got %v", err)
	}
}
