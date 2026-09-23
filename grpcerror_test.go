package gossiper

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestInternalError(t *testing.T) {
	var buf bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	cause := errors.New(`duplicate key value violates unique constraint "idx_clients_phone" (SQLSTATE 23505)`)
	err := InternalError(context.Background(), "failed to create client", cause)

	if status.Code(err) != codes.Internal {
		t.Fatalf("expected Internal, got %v", status.Code(err))
	}
	if msg := status.Convert(err).Message(); msg != "failed to create client" {
		t.Fatalf("the status message must not carry the cause, got %q", msg)
	}
	if strings.Contains(status.Convert(err).Message(), "idx_clients_phone") {
		t.Fatal("the constraint name reached the wire")
	}
	if logged := buf.String(); !strings.Contains(logged, "idx_clients_phone") {
		t.Fatalf("the cause must still be logged in full, got %q", logged)
	}
}
