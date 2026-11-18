package tenancy

import (
	"context"
	"sync"
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

	// TenantPreparingCode marks the Unavailable answer given while a tenant's
	// schema is being prepared. It is a prefix of the status message, so a
	// gateway can turn it into a machine-readable code and a client can tell
	// "first start, keep waiting" from a real outage.
	TenantPreparingCode = "tenant_preparing"

	// ensureTimeout bounds one provisioning attempt. It runs detached from
	// the request: the request gives up after maxWait, provisioning must not.
	ensureTimeout = 3 * time.Minute
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

	ensureMu sync.Mutex
	ensuring map[string]*ensureCall
}

// ensureCall is one in-flight provisioning of a namespace, shared by every
// request that arrives for it meanwhile.
type ensureCall struct {
	done chan struct{}
	err  error
}

func (c *ensureCall) finished() bool {
	select {
	case <-c.done:
		return true
	default:
		return false
	}
}

// startEnsure provisions the namespace once, whatever number of requests ask.
// A first screen fires many queries at once; each used to start its own
// provisioning on its own request context, so they raced for the same schema
// and were cancelled halfway whenever the browser or gateway gave up -- the
// AutoMigrate transaction rolled back and the tenant never became ready. The
// work now runs detached from every request, once per namespace.
func (m *Readiness) startEnsure(ctx context.Context, ensurer AppTenantEnsurer, namespace, userID string) *ensureCall {
	m.ensureMu.Lock()
	defer m.ensureMu.Unlock()
	if call, ok := m.ensuring[namespace]; ok {
		return call
	}
	if m.ensuring == nil {
		m.ensuring = make(map[string]*ensureCall)
	}
	call := &ensureCall{done: make(chan struct{})}
	m.ensuring[namespace] = call

	detached, cancel := context.WithTimeout(context.WithoutCancel(ctx), ensureTimeout)
	go func() {
		defer cancel()
		call.err = ensurer.EnsureAppTenant(detached, namespace, userID)
		m.ensureMu.Lock()
		delete(m.ensuring, namespace)
		m.ensureMu.Unlock()
		close(call.done)
	}()
	return call
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
		var ensure *ensureCall
		ensureChecked := false

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
						ensure = m.startEnsure(ctx, ensurer, namespace, userMetadata.GetUserId())
					}
				}
			}

			if ensure != nil && !ensure.finished() {
				// Provisioning is still running (it takes seconds on a first start). Wait for it, but
				// never past this request's budget.
				wait := time.NewTimer(time.Until(giveUpAt))
				select {
				case <-ensure.done:
				case <-ctx.Done():
					wait.Stop()
					return nil, notReadyError(ctx, err)
				case <-wait.C:
				}
				wait.Stop()
			}
			if ensure != nil && ensure.finished() && ensure.err == nil && !ensureChecked {
				ensureChecked = true
				if ready, readyErr := m.backend.IsReady(ctx, namespace); readyErr == nil && ready {
					return handler(ctx, req)
				}
			}

			// No migration record at all means this service has never
			// provisioned the namespace, and waiting cannot change that --
			// only the warmup triggered above or a call from a member of the
			// namespace can. Say so immediately with a code callers do not
			// retry, so a made-up namespace costs one short call instead of
			// the wait budget times the read-retry count.
			// Not while Hub is still provisioning it: the record appears when that finishes.
			provisioning := ensure != nil && !ensure.finished()
			if migrationStatus, _, infoErr := m.backend.Info(ctx, namespace); !provisioning && infoErr == nil && migrationStatus == "" {
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
		return status.Errorf(codes.Unavailable, "%s: tenant readiness check failed: %v", TenantPreparingCode, lastErr)
	}
	return status.Error(codes.Unavailable, TenantPreparingCode+": tenant migration in progress, retry later")
}
