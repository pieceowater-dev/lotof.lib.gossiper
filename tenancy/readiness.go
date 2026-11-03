package tenancy

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	readyInitialRetryDelay = 200 * time.Millisecond
	readyMaxRetryDelay     = time.Second
	// DefaultReadyMaxWait bounds how long ONE request waits for a tenant that
	// is not ready. The loop used to run until the caller's deadline (60s by
	// default), so a request naming a namespace this service will never have
	// -- public endpoints take the namespace straight from a request header,
	// so anyone can name one -- held a handler for a minute and re-triggered
	// a full tenant warmup on every iteration.
	DefaultReadyMaxWait = 3 * time.Second
)

// ReadinessBackend is what the readiness gate asks about a tenant.
type ReadinessBackend interface {
	// IsReady reports whether the tenant's schema is migrated to the version
	// the current model set implies.
	IsReady(ctx context.Context, namespace string) (bool, error)
	// Info is the last recorded migration status; "" means the service has no
	// record of the namespace at all.
	Info(ctx context.Context, namespace string) (status string, appliedVersion string, err error)
	// TriggerWarmup re-discovers the app's namespaces in the background.
	TriggerWarmup()
}

// AppTenantEnsurer is implemented by a backend that can ask Hub to provision
// a namespace for a caller who is a member of it. Optional: a backend
// without it simply never provisions on first connection.
type AppTenantEnsurer interface {
	EnsureAppTenant(ctx context.Context, namespaceSlug string, userID string) error
}

// Readiness gates requests until the tenant schema for the request's
// namespace has been migrated. If it is not ready within the wait budget,
// the caller is asked to retry instead of being held open. It must run after
// the MetadataInterceptor, which supplies the namespace.
type Readiness struct {
	backend         ReadinessBackend
	excludedMethods map[string]struct{}
	maxWait         time.Duration
}

// NewReadiness builds the gate; excludedMethods pass through ungated
// (provisioning, readiness reports, health checks).
func NewReadiness(backend ReadinessBackend, excludedMethods []string) *Readiness {
	return NewReadinessWithWait(backend, excludedMethods, DefaultReadyMaxWait)
}

// NewReadinessWithWait is NewReadiness with a custom wait budget.
func NewReadinessWithWait(backend ReadinessBackend, excludedMethods []string, maxWait time.Duration) *Readiness {
	excluded := make(map[string]struct{}, len(excludedMethods))
	for _, method := range excludedMethods {
		excluded[method] = struct{}{}
	}
	return &Readiness{backend: backend, excludedMethods: excluded, maxWait: maxWait}
}

// MiddlewareMethod is the gRPC unary interceptor.
func (m *Readiness) MiddlewareMethod() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if _, excluded := m.excludedMethods[info.FullMethod]; excluded {
			return handler(ctx, req)
		}

		namespace := NamespaceFromContext(ctx)
		if namespace == "" {
			return handler(ctx, req)
		}

		giveUpAt := time.Now().Add(m.maxWait)
		delay := readyInitialRetryDelay
		warmupTriggered := false

		for {
			ready, err := m.backend.IsReady(ctx, namespace)
			if err == nil && ready {
				return handler(ctx, req)
			}

			// Once per request, not once per iteration: the warmup re-syncs
			// every namespace of this app, and a caller naming an unknown one
			// used to keep it running back to back.
			if !warmupTriggered {
				warmupTriggered = true
				m.backend.TriggerWarmup()

				// The warmup only rediscovers namespaces already associated
				// with this app (namespace_apps). On a namespace's very first
				// connection that association may not exist yet -- it is
				// written once the billing subscription succeeds -- so warmup
				// alone would leave the namespace stuck. Ask hub directly,
				// scoped to the calling user's own membership.
				if ensurer, ok := m.backend.(AppTenantEnsurer); ok {
					if userMetadata, ok := UserMetadataFromContext(ctx); ok && userMetadata.GetUserId() != "" {
						if ensureErr := ensurer.EnsureAppTenant(ctx, namespace, userMetadata.GetUserId()); ensureErr == nil {
							if ready, readyErr := m.backend.IsReady(ctx, namespace); readyErr == nil && ready {
								return handler(ctx, req)
							}
						}
					}
				}
			}

			// No migration record at all means this service has never
			// provisioned the namespace, and waiting cannot change that --
			// only the warmup triggered above or a call from a member of the
			// namespace can. Say so immediately with a code callers do not
			// retry, so a made-up namespace costs one short call instead of
			// the wait budget times the read-retry count.
			if migrationStatus, _, infoErr := m.backend.Info(ctx, namespace); infoErr == nil && migrationStatus == "" {
				return nil, status.Errorf(codes.FailedPrecondition,
					"namespace %q is not provisioned for this service", namespace)
			}

			remaining := time.Until(giveUpAt)
			if remaining <= 0 {
				return nil, notReadyError(ctx, err)
			}
			if delay > remaining {
				delay = remaining
			}

			select {
			case <-ctx.Done():
				return nil, notReadyError(ctx, err)
			case <-time.After(delay):
			}

			if delay < readyMaxRetryDelay {
				delay *= 2
				if delay > readyMaxRetryDelay {
					delay = readyMaxRetryDelay
				}
			}
		}
	}
}

func notReadyError(ctx context.Context, lastErr error) error {
	_ = grpc.SetHeader(ctx, metadata.Pairs("retry-after", "1"))
	if lastErr != nil {
		return status.Errorf(codes.Unavailable, "tenant readiness check failed: %v", lastErr)
	}
	return status.Error(codes.Unavailable, "tenant migration in progress, retry later")
}
