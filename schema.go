package gossiper

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/pieceowater-dev/lotof.lib.gossiper/v2/tenancy"
)

// ErrNoNamespace is returned by the strict schema helpers when the request
// context carries no tenant.
var ErrNoNamespace = errors.New("no namespace in context")

// WithSchema runs fn against a connection pinned to the current request's
// tenant schema (the namespace the metadata interceptor put on ctx), inside a
// transaction. With no namespace on the context it runs fn directly against
// the caller's transaction or the default connection -- the behaviour of the
// per-repository withSchema helpers this replaces (78 copies, audit 2).
//
// Prefer WithSchemaStrict where running against the default schema would be
// a bug.
func WithSchema(ctx context.Context, db Database, fn func(tx *gorm.DB) error) error {
	if ns := tenancy.NamespaceFromContext(ctx); ns != "" {
		return db.WithSchema(ctx, ns, fn)
	}
	return fn(DBFromContext(ctx, db))
}

// WithSchemaReadOnly is WithSchema without the enclosing transaction -- only
// safe for fn bodies issuing read statements (see Database.WithSchemaReadOnly).
func WithSchemaReadOnly(ctx context.Context, db Database, fn func(tx *gorm.DB) error) error {
	if ns := tenancy.NamespaceFromContext(ctx); ns != "" {
		return db.WithSchemaReadOnly(ctx, ns, fn)
	}
	return fn(DBFromContext(ctx, db))
}

// WithSchemaStrict is WithSchema that refuses to run without a namespace:
// ErrNoNamespace instead of silently touching the default schema.
func WithSchemaStrict(ctx context.Context, db Database, fn func(tx *gorm.DB) error) error {
	ns := tenancy.NamespaceFromContext(ctx)
	if ns == "" {
		return ErrNoNamespace
	}
	return db.WithSchema(ctx, ns, fn)
}

// WithSchemaReadOnlyStrict is WithSchemaReadOnly that refuses to run without
// a namespace.
func WithSchemaReadOnlyStrict(ctx context.Context, db Database, fn func(tx *gorm.DB) error) error {
	ns := tenancy.NamespaceFromContext(ctx)
	if ns == "" {
		return ErrNoNamespace
	}
	return db.WithSchemaReadOnly(ctx, ns, fn)
}

// TenantRepo is a repository base: embed it (or hold one) and call
// WithSchema/WithSchemaReadOnly instead of declaring the helper pair per
// repository.
type TenantRepo struct {
	DB Database
}

// NewTenantRepo wraps db.
func NewTenantRepo(db Database) TenantRepo { return TenantRepo{DB: db} }

// WithSchema is gossiper.WithSchema on the repository's database.
func (r TenantRepo) WithSchema(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return WithSchema(ctx, r.DB, fn)
}

// WithSchemaReadOnly is gossiper.WithSchemaReadOnly on the repository's database.
func (r TenantRepo) WithSchemaReadOnly(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return WithSchemaReadOnly(ctx, r.DB, fn)
}

// Tenant context, re-exported so services need one import.
type (
	// UserMetadata is the caller and tenant of a request.
	UserMetadata = tenancy.UserMetadata
)

// NamespaceFromContext is the request's tenant slug, "" when absent.
func NamespaceFromContext(ctx context.Context) string { return tenancy.NamespaceFromContext(ctx) }

// UserMetadataFromContext is the request's caller and tenant.
func UserMetadataFromContext(ctx context.Context) (*UserMetadata, bool) {
	return tenancy.UserMetadataFromContext(ctx)
}

// WithUserMetadata puts a caller and tenant on ctx for in-process code paths.
func WithUserMetadata(ctx context.Context, userID, namespace, deviceMeta string) context.Context {
	return tenancy.WithUserMetadata(ctx, userID, namespace, deviceMeta)
}
