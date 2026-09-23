package gossiper

import (
	"context"
	"errors"
	"testing"

	"gorm.io/gorm"
	"gorm.io/gorm/utils/tests"
)

// fakeDB is enough for the context plumbing: WithTransaction hands out a
// sentinel *gorm.DB and records whether fn returned an error, which is what
// a real driver would commit or roll back on.
type fakeDB struct {
	Database
	pool       *gorm.DB
	tx         *gorm.DB
	began      int
	rolledBack bool
}

func (f *fakeDB) GetDB() *gorm.DB { return f.pool }

func (f *fakeDB) WithTransaction(fn func(tx *gorm.DB) error) error {
	f.began++
	if err := fn(f.tx); err != nil {
		f.rolledBack = true
		return err
	}
	return nil
}

// gorm's DummyDialector gives real *gorm.DB sessions without a database
// behind them, which is all this plumbing needs.
func newGormSession(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(tests.DummyDialector{}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open dummy gorm session: %v", err)
	}
	return db
}

func newFakeDB(t *testing.T) *fakeDB {
	t.Helper()
	return &fakeDB{pool: newGormSession(t), tx: newGormSession(t)}
}

func TestInTx_RepositoriesSeeTheTransaction(t *testing.T) {
	db := newFakeDB(t)
	var seen *gorm.DB

	err := InTx(context.Background(), db, func(ctx context.Context) error {
		seen = DBFromContext(ctx, db)
		return nil
	})
	if err != nil {
		t.Fatalf("InTx: %v", err)
	}
	if db.began != 1 {
		t.Fatalf("expected one transaction, got %d", db.began)
	}
	if seen == nil || seen.Statement == nil || seen.Statement.Context == nil {
		t.Fatal("DBFromContext must return a handle bound to the request context")
	}
	if db.rolledBack {
		t.Fatal("a successful unit of work must not roll back")
	}
}

func TestDBFromContext_FallsBackToThePool(t *testing.T) {
	db := newFakeDB(t)
	got := DBFromContext(context.Background(), db)
	if got == nil {
		t.Fatal("without a transaction the pool handle is expected")
	}
	if _, ok := TxFromContext(context.Background()); ok {
		t.Fatal("a bare context must not report a transaction")
	}
}

func TestInTx_ErrorRollsBack(t *testing.T) {
	db := newFakeDB(t)
	want := errors.New("insert failed")

	if err := InTx(context.Background(), db, func(context.Context) error { return want }); !errors.Is(err, want) {
		t.Fatalf("expected the original error, got %v", err)
	}
	if !db.rolledBack {
		t.Fatal("a failed unit of work must roll back")
	}
}

// A service calling another service's unit of work must not open a second
// transaction: the inner one would block on the outer one's uncommitted
// rows.
func TestInTx_NestedCallsJoinTheOuterTransaction(t *testing.T) {
	db := newFakeDB(t)
	var outer, inner *gorm.DB

	err := InTx(context.Background(), db, func(ctx context.Context) error {
		outer, _ = TxFromContext(ctx)
		return InTx(ctx, db, func(ctx context.Context) error {
			inner, _ = TxFromContext(ctx)
			return nil
		})
	})
	if err != nil {
		t.Fatalf("InTx: %v", err)
	}
	if db.began != 1 {
		t.Fatalf("expected exactly one transaction, got %d", db.began)
	}
	if outer != inner {
		t.Fatal("the nested call must see the same transaction")
	}
}

func TestInTx_PanicRollsBackAndPropagates(t *testing.T) {
	db := newFakeDB(t)
	defer func() {
		if recover() == nil {
			t.Fatal("the panic must not be swallowed")
		}
	}()
	_ = InTx(context.Background(), db, func(context.Context) error { panic("boom") })
}

func TestInTxSchema_PinsOneTransactionForTheTenant(t *testing.T) {
	session := newGormSession(t)
	db := &schemaDB{tx: session}

	var seen *gorm.DB
	err := InTxSchema(context.Background(), db, "ns_demo", func(ctx context.Context) error {
		seen, _ = TxFromContext(ctx)
		// A nested unit of work must not open a second transaction.
		return InTxSchema(ctx, db, "ns_demo", func(context.Context) error { return nil })
	})
	if err != nil {
		t.Fatalf("InTxSchema: %v", err)
	}
	if db.schemaCalls != 1 {
		t.Fatalf("expected one schema-scoped transaction, got %d", db.schemaCalls)
	}
	if seen != session {
		t.Fatal("repositories must see the transaction the schema was pinned on")
	}
	if db.schema != "ns_demo" {
		t.Fatalf("pinned to %q", db.schema)
	}
}

type schemaDB struct {
	Database
	tx          *gorm.DB
	schema      string
	schemaCalls int
}

func (s *schemaDB) WithSchema(_ context.Context, schema string, fn func(tx *gorm.DB) error) error {
	s.schemaCalls++
	s.schema = schema
	return fn(s.tx)
}
