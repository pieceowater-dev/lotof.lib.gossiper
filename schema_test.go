package gossiper

import (
	"context"
	"errors"
	"testing"

	"gorm.io/gorm"
)

// nsRecordingDB records which schema WithSchema/WithSchemaReadOnly were asked for.
type nsRecordingDB struct {
	fakeDB
	schema   string
	readOnly bool
}

func (s *nsRecordingDB) WithSchema(_ context.Context, schema string, fn func(tx *gorm.DB) error) error {
	s.schema = schema
	return fn(s.tx)
}

func (s *nsRecordingDB) WithSchemaReadOnly(_ context.Context, schema string, fn func(tx *gorm.DB) error) error {
	s.schema, s.readOnly = schema, true
	return fn(s.tx)
}

func newSchemaDB(t *testing.T) *nsRecordingDB {
	return &nsRecordingDB{fakeDB: *newFakeDB(t)}
}

func TestWithSchema_UsesTheRequestNamespace(t *testing.T) {
	db := newSchemaDB(t)
	ctx := WithUserMetadata(context.Background(), "u", "acme", "")

	var got *gorm.DB
	if err := WithSchema(ctx, db, func(tx *gorm.DB) error { got = tx; return nil }); err != nil {
		t.Fatal(err)
	}
	if db.schema != "acme" || db.readOnly || got != db.tx {
		t.Fatalf("schema=%q readOnly=%v", db.schema, db.readOnly)
	}

	if err := WithSchemaReadOnly(ctx, db, func(*gorm.DB) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if !db.readOnly {
		t.Fatal("the read-only variant must use WithSchemaReadOnly")
	}
}

func TestWithSchema_WithoutNamespaceUsesThePoolOrTheCallersTransaction(t *testing.T) {
	db := newSchemaDB(t)

	var got *gorm.DB
	_ = WithSchema(context.Background(), db, func(tx *gorm.DB) error { got = tx; return nil })
	if db.schema != "" || got == nil {
		t.Fatalf("no namespace must not switch schema, got %q", db.schema)
	}

	_ = InTx(context.Background(), db, func(ctx context.Context) error {
		return WithSchema(ctx, db, func(tx *gorm.DB) error {
			if tx.Statement.Context == nil {
				t.Error("no context on the joined session")
			}
			return nil
		})
	})
	if db.began != 1 {
		t.Fatalf("a repository call inside InTx must join it, began=%d", db.began)
	}
}

func TestWithSchemaStrict_RefusesWithoutNamespace(t *testing.T) {
	db := newSchemaDB(t)
	called := false
	fn := func(*gorm.DB) error { called = true; return nil }
	if err := WithSchemaStrict(context.Background(), db, fn); !errors.Is(err, ErrNoNamespace) || called {
		t.Fatalf("err=%v called=%v", err, called)
	}
	if err := WithSchemaReadOnlyStrict(context.Background(), db, fn); !errors.Is(err, ErrNoNamespace) || called {
		t.Fatalf("err=%v called=%v", err, called)
	}
	ctx := WithUserMetadata(context.Background(), "", "acme", "")
	if err := WithSchemaStrict(ctx, db, fn); err != nil || !called || db.schema != "acme" {
		t.Fatalf("err=%v called=%v schema=%q", err, called, db.schema)
	}
}

func TestTenantRepo(t *testing.T) {
	db := newSchemaDB(t)
	r := NewTenantRepo(db)
	ctx := WithUserMetadata(context.Background(), "", "acme", "")
	_ = r.WithSchema(ctx, func(*gorm.DB) error { return nil })
	_ = r.WithSchemaReadOnly(ctx, func(*gorm.DB) error { return nil })
	if db.schema != "acme" || !db.readOnly {
		t.Fatalf("schema=%q readOnly=%v", db.schema, db.readOnly)
	}
}

func TestPageLimitOffset(t *testing.T) {
	for _, c := range []struct{ page, length, limit, offset int }{
		{1, 20, 20, 0}, {3, 10, 10, 20}, {0, 0, 20, 0}, {-4, 5, 5, 0}, {2, 99999, 1000, 1000},
	} {
		if l, o := PageLimitOffset(c.page, c.length); l != c.limit || o != c.offset {
			t.Errorf("PageLimitOffset(%d,%d) = %d,%d want %d,%d", c.page, c.length, l, o, c.limit, c.offset)
		}
	}
}
