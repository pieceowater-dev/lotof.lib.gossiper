package gossiper

import (
	"context"

	"gorm.io/gorm"

	"github.com/pieceowater-dev/lotof.lib.gossiper/v2/internal/dbtx"
)

// ContextWithTx carries an open transaction on the context so repositories
// below join it instead of opening their own connection. Prefer InTx, which
// does this and the commit/rollback for you.
func ContextWithTx(ctx context.Context, tx *gorm.DB) context.Context {
	return dbtx.With(ctx, tx)
}

// TxFromContext returns the transaction InTx put on the context, if any.
func TxFromContext(ctx context.Context) (*gorm.DB, bool) {
	return dbtx.From(ctx)
}

// DBFromContext is what a repository method should use instead of
// db.GetDB(): the caller's transaction when there is one, the pool
// otherwise. That is what lets a service span several repository calls in
// one transaction without every repository growing a *gorm.DB parameter.
func DBFromContext(ctx context.Context, db Database) *gorm.DB {
	if tx, ok := TxFromContext(ctx); ok {
		return tx.WithContext(ctx)
	}
	return db.GetDB().WithContext(ctx)
}

// InTx runs fn inside one transaction, passing it a context that every
// repository underneath will pick the transaction up from. fn returning an
// error rolls everything back; a panic does too, and is then re-raised.
//
// Nested calls join the outer transaction rather than opening a second one,
// so a service can call another service's unit of work without deadlocking
// itself against its own uncommitted writes.
//
// Keep outbound calls to other services out of fn: they hold the
// transaction open for a network round trip, and they cannot be rolled back
// when a later statement fails.
func InTx(ctx context.Context, db Database, fn func(ctx context.Context) error) error {
	if _, already := TxFromContext(ctx); already {
		return fn(ctx)
	}
	return db.WithTransaction(func(tx *gorm.DB) error {
		return fn(ContextWithTx(ctx, tx))
	})
}

// InTxSchema is InTx for a tenant-scoped service: one transaction pinned to
// schema's search_path, which every repository call inside fn joins --
// including the ones that go through WithSchema, since that now joins a
// transaction already on the context instead of opening its own.
//
// Use it where one request writes through several repositories of the same
// tenant (a sale and its stock movements, a booking and its lines); without
// it each repository call is its own transaction and a failure part-way
// through leaves the tenant's data half-written.
func InTxSchema(ctx context.Context, db Database, schema string, fn func(ctx context.Context) error) error {
	if _, already := TxFromContext(ctx); already {
		return fn(ctx)
	}
	return db.WithSchema(ctx, schema, func(tx *gorm.DB) error {
		return fn(ContextWithTx(ctx, tx))
	})
}
