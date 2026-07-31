package gossiper

import (
	"context"
	"testing"
)

type facadeTestModel struct {
	Username string
}

func TestNewFilter(t *testing.T) {
	sort := NewSort[string]("username", "ASC")
	pagination := NewPagination(1, 10)
	f := NewFilter[string]("alice", sort, pagination)

	if f.Search != "alice" {
		t.Errorf("expected search %q, got %q", "alice", f.Search)
	}
	if f.Sort.Field != "username" {
		t.Errorf("expected sort field %q, got %q", "username", f.Sort.Field)
	}
	if f.Pagination.Page != 1 || f.Pagination.Length != 10 {
		t.Errorf("expected pagination page=1 length=10, got page=%d length=%d", f.Pagination.Page, f.Pagination.Length)
	}
}

func TestNewPaginatedResult(t *testing.T) {
	rows := []string{"a", "b", "c"}
	result := NewPaginatedResult(rows, 3)

	if len(result.Rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(result.Rows))
	}
	if result.Info.Count != 3 {
		t.Errorf("expected count 3, got %d", result.Info.Count)
	}
}

func TestNewPagination(t *testing.T) {
	p := NewPagination(2, 25)
	if p.Page != 2 || p.Length != 25 {
		t.Errorf("expected page=2 length=25, got page=%d length=%d", p.Page, p.Length)
	}
}

func TestNewSort(t *testing.T) {
	s := NewSort[string]("created_at", "DESC")
	if s.Field != "created_at" {
		t.Errorf("expected field %q, got %q", "created_at", s.Field)
	}
	if s.Direction != "DESC" {
		t.Errorf("expected direction %q, got %q", "DESC", s.Direction)
	}
}

func TestIsFieldValid(t *testing.T) {
	model := &facadeTestModel{}

	if !IsFieldValid(model, "username") {
		t.Error("expected username to be a valid field")
	}
	if IsFieldValid(model, "nonexistent") {
		t.Error("expected nonexistent to be an invalid field")
	}
}

func TestToSnakeCase(t *testing.T) {
	if got := ToSnakeCase("UserName"); got != "user_name" {
		t.Errorf("expected %q, got %q", "user_name", got)
	}
}

func TestGenerateRandomString(t *testing.T) {
	got := GenerateRandomString(16)
	if len(got) != 16 {
		t.Errorf("expected length 16, got %d", len(got))
	}
}

func TestEncryptDecryptAES256(t *testing.T) {
	key := "01234567890123456789012345678901"
	encrypted, err := EncryptAES256(key, "secret data")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	decrypted, err := DecryptAES256(key, encrypted)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decrypted != "secret data" {
		t.Errorf("expected %q, got %q", "secret data", decrypted)
	}
}

func TestNewTenantManager(t *testing.T) {
	if _, err := NewTenantManager(nil, "short"); err == nil {
		t.Error("expected error for invalid secret length, got nil")
	}
	mgr, err := NewTenantManager(nil, "01234567890123456789012345678901")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mgr == nil {
		t.Error("expected non-nil tenant manager")
	}
}

func TestNewTransportFactory(t *testing.T) {
	f := NewTransportFactory()
	if f == nil {
		t.Fatal("expected non-nil transport factory")
	}
	tr := f.CreateTransport(GRPC, "localhost:9000")
	if tr == nil {
		t.Error("expected non-nil transport for GRPC type")
	}
}

func TestRegisterTransportContextMiddleware(t *testing.T) {
	// Registering must not panic and must accept a no-op passthrough hook.
	RegisterTransportContextMiddleware(func(ctx context.Context) context.Context {
		return ctx
	})
}
