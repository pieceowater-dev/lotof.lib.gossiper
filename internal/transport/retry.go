package transport

import (
	"context"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	retryMaxAttempts    = 3
	retryInitialBackoff = 100 * time.Millisecond
	retryMaxBackoff     = time.Second
)

// readPrefixes mark RPCs that only read: repeating one can't create a
// duplicate or change state. The platform's RPC names follow this convention
// (checked against every rpc in the lotof.*.proto repositories).
var readPrefixes = []string{"Get", "List", "Find", "Search", "Count", "Check", "Is", "Has", "Exists", "Lookup", "Read", "Resolve", "Validate", "Verify"}

// notRetryable are read-named RPCs that still have a side effect per call
// (attempt counters, codes sent, click tracking) -- never repeated.
var notRetryable = map[string]bool{"VerifyPin": true, "VerifyIdentity": true, "ResolveDeepLink": true}

// isRetryableMethod reports whether fullMethod ("/pkg.Service/Method") is a
// read-only RPC that is safe to repeat.
func isRetryableMethod(fullMethod string) bool {
	name := fullMethod[strings.LastIndex(fullMethod, "/")+1:]
	if notRetryable[name] {
		return false
	}
	for _, p := range readPrefixes {
		if len(name) > len(p) && strings.HasPrefix(name, p) && name[len(p)] >= 'A' && name[len(p)] <= 'Z' {
			return true
		}
	}
	return false
}

// RetryReadsUnaryClientInterceptor retries read-only RPCs (see
// isRetryableMethod) that fail with codes.Unavailable, up to 3 attempts with
// exponential backoff, and never past the call's deadline. Writes are never
// repeated: an Unavailable after the server already applied a write (the
// connection dropped before the response) would otherwise duplicate it.
// Requests that never left the client are still retried transparently by
// grpc-go itself, for every method.
func RetryReadsUnaryClientInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		if !isRetryableMethod(method) {
			return invoker(ctx, method, req, reply, cc, opts...)
		}
		backoff := retryInitialBackoff
		var err error
		for attempt := 1; ; attempt++ {
			err = invoker(ctx, method, req, reply, cc, opts...)
			if err == nil || status.Code(err) != codes.Unavailable || attempt >= retryMaxAttempts {
				return err
			}
			select {
			case <-ctx.Done():
				return err
			case <-time.After(backoff):
			}
			if backoff *= 2; backoff > retryMaxBackoff {
				backoff = retryMaxBackoff
			}
		}
	}
}
