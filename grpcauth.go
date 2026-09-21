package gossiper

import (
	"context"
	"log/slog"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// ServiceAuthMetadataKey is the gRPC metadata key carrying the shared
// service-to-service auth token.
const ServiceAuthMetadataKey = "x-service-auth"

// ServiceAuthClientInterceptor returns a client-side unary interceptor that
// attaches `secret` on every outgoing call as ServiceAuthMetadataKey. If
// `secret` is empty it is a no-op, so a not-yet-configured deployment keeps
// working. Register it once via RegisterClientUnaryInterceptor.
func ServiceAuthClientInterceptor(secret string) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		if secret != "" {
			ctx = metadata.AppendToOutgoingContext(ctx, ServiceAuthMetadataKey, secret)
		}
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// Service-auth modes for ServiceAuthServerInterceptorWithMode.
const (
	// ServiceAuthEnforce rejects a call without a valid token with
	// codes.Unauthenticated.
	ServiceAuthEnforce = "enforce"
	// ServiceAuthReport lets such a call through but logs a warning naming the
	// method and the peer. Use it to switch a service on safely: set the
	// secret in report mode, confirm no warnings appear under real traffic
	// (every caller already sends the token), then switch to enforce.
	ServiceAuthReport = "report"
)

// ServiceAuthServerInterceptor returns a server-side unary interceptor that
// rejects calls whose ServiceAuthMetadataKey metadata does not equal
// `secret`. It is ServiceAuthServerInterceptorWithMode in enforce mode.
func ServiceAuthServerInterceptor(secret string, exemptPrefixes ...string) grpc.UnaryServerInterceptor {
	return ServiceAuthServerInterceptorWithMode(secret, ServiceAuthEnforce, exemptPrefixes...)
}

// ServiceAuthServerInterceptorWithMode checks the ServiceAuthMetadataKey
// metadata against `secret`:
//
//   - `secret` empty            -> no-op (allow all); lets the interceptor be
//     deployed before the secret is configured.
//   - method in `exemptPrefixes` -> allowed without a token (health checks,
//     reflection, any genuinely public RPC). Matched by prefix on the full
//     method string, e.g. "/grpc.health.v1.Health/".
//   - otherwise a missing/invalid token is rejected (ServiceAuthEnforce) or
//     logged and allowed (ServiceAuthReport). Any other mode value, including
//     "", means enforce -- a typo must not silently turn the check off.
func ServiceAuthServerInterceptorWithMode(secret, mode string, exemptPrefixes ...string) grpc.UnaryServerInterceptor {
	report := mode == ServiceAuthReport
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if secret == "" {
			return handler(ctx, req)
		}
		for _, p := range exemptPrefixes {
			if p != "" && strings.HasPrefix(info.FullMethod, p) {
				return handler(ctx, req)
			}
		}
		md, _ := metadata.FromIncomingContext(ctx)
		got := ""
		if v := md.Get(ServiceAuthMetadataKey); len(v) > 0 {
			got = v[0]
		}
		if subtleConstEq(got, secret) {
			return handler(ctx, req)
		}
		if report {
			reason := "invalid token"
			if got == "" {
				reason = "missing token"
			}
			caller := ""
			if p, ok := peer.FromContext(ctx); ok && p.Addr != nil {
				caller = p.Addr.String()
			}
			slog.Warn("service auth: call without a valid token (report mode, allowed)",
				slog.String("method", info.FullMethod),
				slog.String("reason", reason),
				slog.String("peer", caller),
			)
			return handler(ctx, req)
		}
		return nil, status.Error(codes.Unauthenticated, "service auth: missing or invalid "+ServiceAuthMetadataKey)
	}
}

// subtleConstEq is a length-independent constant-time-ish string compare that
// avoids importing crypto/subtle's []byte dance at call sites.
func subtleConstEq(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var v byte
	for i := 0; i < len(a); i++ {
		v |= a[i] ^ b[i]
	}
	return v == 0
}
