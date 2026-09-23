// Package dbtx carries an open transaction on a context. It exists as its
// own package so both the database drivers and the public gossiper API can
// use the same key without importing each other.
package dbtx

import (
	"context"

	"gorm.io/gorm"
)

type key struct{}

// With returns a context carrying tx.
func With(ctx context.Context, tx *gorm.DB) context.Context {
	return context.WithValue(ctx, key{}, tx)
}

// From returns the transaction on ctx, if any.
func From(ctx context.Context) (*gorm.DB, bool) {
	tx, ok := ctx.Value(key{}).(*gorm.DB)
	return tx, ok && tx != nil
}
