package tenancy

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// UserMetadata is who is calling and which tenant they are calling for, as
// the gateway put it into the incoming gRPC metadata. Every tenant-scoped
// service used to carry its own copy of this type (generated from
// generic/utils) together with the interceptor that fills it; the field and
// getter names match the generated type so call sites did not change.
type UserMetadata struct {
	UserId     string
	Namespace  string
	DeviceMeta string
}

// GetUserId returns the caller's id, or "" on a nil receiver.
func (m *UserMetadata) GetUserId() string {
	if m == nil {
		return ""
	}
	return m.UserId
}

// GetNamespace returns the tenant slug, or "" on a nil receiver.
func (m *UserMetadata) GetNamespace() string {
	if m == nil {
		return ""
	}
	return m.Namespace
}

// GetDeviceMeta returns the device descriptor, or "" on a nil receiver.
func (m *UserMetadata) GetDeviceMeta() string {
	if m == nil {
		return ""
	}
	return m.DeviceMeta
}

type userMetadataKey struct{}

// ContextWithUserMetadata stores md on the context.
func ContextWithUserMetadata(ctx context.Context, md *UserMetadata) context.Context {
	return context.WithValue(ctx, userMetadataKey{}, md)
}

// UserMetadataFromContext returns what the metadata interceptor (or
// WithUserMetadata) stored.
func UserMetadataFromContext(ctx context.Context) (*UserMetadata, bool) {
	md, ok := ctx.Value(userMetadataKey{}).(*UserMetadata)
	return md, ok
}

// NamespaceFromContext is the tenant slug on the context; empty when no
// metadata is present.
func NamespaceFromContext(ctx context.Context) string {
	md, ok := UserMetadataFromContext(ctx)
	if !ok || md == nil {
		return ""
	}
	return md.Namespace
}

// WithUserMetadata injects metadata for in-process (non-gRPC) code paths --
// background loops and tests that call services directly.
func WithUserMetadata(ctx context.Context, userID, namespace, deviceMeta string) context.Context {
	return ContextWithUserMetadata(ctx, &UserMetadata{UserId: userID, Namespace: namespace, DeviceMeta: deviceMeta})
}

// MetadataFromIncoming reads the caller from incoming gRPC metadata. Both
// key spellings the gateways have used are accepted ("userid"/"user-id",
// "namespace"/"namespace-slug"). Missing keys stay empty.
func MetadataFromIncoming(ctx context.Context) *UserMetadata {
	result := &UserMetadata{}

	incoming, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return result
	}

	if v := incoming.Get("userid"); len(v) > 0 {
		result.UserId = v[0]
	} else if v := incoming.Get("user-id"); len(v) > 0 {
		result.UserId = v[0]
	}

	if v := incoming.Get("namespace"); len(v) > 0 {
		result.Namespace = v[0]
	} else if v := incoming.Get("namespace-slug"); len(v) > 0 {
		result.Namespace = v[0]
	}

	if v := incoming.Get("device_meta"); len(v) > 0 {
		result.DeviceMeta = v[0]
	}

	return result
}

// MetadataInterceptor puts the caller's metadata on the context and refuses
// calls that name no tenant.
type MetadataInterceptor struct {
	excludedMethods map[string]struct{}
	requireUser     bool
}

// NewMetadataInterceptor builds the interceptor; excludedMethods (full gRPC
// method names) pass through untouched -- health checks, tenant provisioning.
func NewMetadataInterceptor(excludedMethods []string) *MetadataInterceptor {
	set := make(map[string]struct{}, len(excludedMethods))
	for _, method := range excludedMethods {
		set[method] = struct{}{}
	}
	return &MetadataInterceptor{excludedMethods: set}
}

// RequireUser makes a missing user id an Unauthenticated error too, for
// services with no public endpoints. By default the user id is optional so a
// public storefront can still resolve its namespace.
func (m *MetadataInterceptor) RequireUser() *MetadataInterceptor {
	m.requireUser = true
	return m
}

// MiddlewareMethod extracts the namespace (and optional user id) from
// incoming gRPC metadata and injects it into the context. The namespace is
// mandatory -- it is required to switch the tenant schema.
func (m *MetadataInterceptor) MiddlewareMethod() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if _, excluded := m.excludedMethods[info.FullMethod]; excluded {
			return handler(ctx, req)
		}

		md := MetadataFromIncoming(ctx)
		// CRITICAL: namespace must be present for multi-tenant schema isolation.
		if md.Namespace == "" {
			return nil, status.Error(codes.PermissionDenied, "namespace is required")
		}
		if m.requireUser && md.UserId == "" {
			return nil, status.Error(codes.Unauthenticated, "missing metadata: user-id")
		}

		return handler(ContextWithUserMetadata(ctx, md), req)
	}
}
