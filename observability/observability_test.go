package observability

import (
	"bytes"
	"context"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"go.opentelemetry.io/otel/trace"
)

func TestInit_WithoutEndpoint_StillTraces(t *testing.T) {
	logger, tracer, shutdown, err := Init(context.Background(), Config{
		ServiceName: "test-svc",
		Environment: "test",
		SampleRatio: 1,
		LogLevel:    slog.LevelInfo,
	})
	if err != nil {
		t.Fatalf("Init without an endpoint must succeed, got %v", err)
	}
	t.Cleanup(func() { _ = shutdown(context.Background()) })
	if logger == nil || tracer == nil {
		t.Fatal("Init must return a logger and a tracer")
	}

	_, span := tracer.Start(context.Background(), "op")
	defer span.End()
	if !span.SpanContext().IsValid() {
		t.Fatal("spans must still get valid trace/span IDs without an exporter")
	}
}

func TestInit_WithEndpoint(t *testing.T) {
	_, _, shutdown, err := Init(context.Background(), Config{
		ServiceName:  "test-svc",
		OtlpEndpoint: "http://collector.invalid:4318",
		SampleRatio:  1,
	})
	if err != nil {
		t.Fatalf("Init with an endpoint must succeed (the exporter connects lazily), got %v", err)
	}
	// Shutdown flushes to an unreachable collector; bound it so the test
	// doesn't wait out the exporter's own retry budget.
	ctx, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()
	_ = shutdown(ctx)
}

func TestFiberMiddleware_DoesNotLogAuthorization(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	tracer := trace.NewNoopTracerProvider().Tracer("test")

	app := fiber.New()
	app.Use(FiberMiddleware(logger, tracer))
	app.Get("/ping", func(c *fiber.Ctx) error { return c.SendString("pong") })

	req := httptest.NewRequest("GET", "/ping", nil)
	req.Header.Set("Authorization", "Bearer super-secret-token")
	req.Header.Set("Namespace", "acme")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("unexpected status %d", resp.StatusCode)
	}

	logged := buf.String()
	if !strings.Contains(logged, "http request completed") {
		t.Fatalf("expected a completion log line, got %q", logged)
	}
	if strings.Contains(logged, "super-secret-token") {
		t.Fatalf("the Authorization header must never be logged, got %q", logged)
	}
	if !strings.Contains(logged, `"tenant":"acme"`) {
		t.Fatalf("namespace should still be logged as tenant, got %q", logged)
	}
}
