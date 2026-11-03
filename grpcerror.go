package gossiper

import (
	"context"
	"errors"
	"log/slog"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/pieceowater-dev/lotof.lib.gossiper/v2/observability"
)

// InternalError logs err in full -- with the request's trace_id, so it can be
// found again -- and returns a status carrying only msg.
//
// The alternative, status.Errorf(codes.Internal, "failed to X: %v", err), puts
// the underlying failure on the wire: "duplicate key value violates unique
// constraint \"idx_clients_phone\" (SQLSTATE 23505)" names a table, an index
// and a column to whoever made the call. The gateways strip that before it
// reaches a browser (audit D1), but every other caller -- another service, a
// bot, a script -- still sees it, and the detail belongs in the log anyway.
func InternalError(ctx context.Context, msg string, err error) error {
	observability.LoggerFromContext(ctx, nil).Error(msg, slog.Any("error", err))
	return status.Error(codes.Internal, msg)
}

// GRPCInternalErrorMessage is what a caller sees in place of an error that
// carries no gRPC status.
const GRPCInternalErrorMessage = "internal error"

// sanitizeError leaves nil and gRPC status errors as they are -- a handler that
// built a status meant its message to be seen -- and turns anything else
// (a raw driver, ORM or library error) into a bare codes.Internal, logging the
// original with the method.
func sanitizeError(ctx context.Context, method string, err error) error {
	if err == nil {
		return nil
	}
	if _, ok := status.FromError(err); ok {
		return err
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return status.FromContextError(err).Err()
	}
	observability.LoggerFromContext(ctx, nil).Error("rpc failed with a non-status error",
		slog.String("method", method), slog.Any("error", err))
	return status.Error(codes.Internal, GRPCInternalErrorMessage)
}

// ErrorSanitizerUnaryServerInterceptor stops a non-status error returned by a
// handler from reaching the wire: grpc would send its text as an Unknown
// status, naming tables, indexes and columns to whoever made the call.
// Status errors pass through untouched. NewGRPCServer installs it by default.
func ErrorSanitizerUnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		resp, err := handler(ctx, req)
		return resp, sanitizeError(ctx, info.FullMethod, err)
	}
}

// ErrorSanitizerStreamServerInterceptor is the streaming counterpart of
// ErrorSanitizerUnaryServerInterceptor.
func ErrorSanitizerStreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		return sanitizeError(ss.Context(), info.FullMethod, handler(srv, ss))
	}
}
