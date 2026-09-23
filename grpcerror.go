package gossiper

import (
	"context"
	"log/slog"

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
