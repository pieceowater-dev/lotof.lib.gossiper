package gossiper

import (
	"context"

	"gorm.io/gorm"
)

type txContextKey struct{}

// ContextWithTx carries an open transaction on the context so repositories
// below join it instead of opening their own connection. Prefer InTx, which
// does this and the commit/rollback for you.
func ContextWithTx(ctx context.Context, tx *gorm.DB) context.Context {
	return context.WithValue(ctx, txContextKey{}, tx)
}

// TxFromContext returns the transaction InTx put on the context, if any.
func TxFromContext(ctx context.Context) (*gorm.DB, bool) {
	tx, ok := ctx.Value(txContextKey{}).(*gorm.DB)
	return tx, ok && tx != nil
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
