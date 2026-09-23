package observability

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"go.opentelemetry.io/otel/trace/noop"
	"google.golang.org/grpc"
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
	tracer := noop.NewTracerProvider().Tracer("test")

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

func TestFiberMiddleware_SkipsSuccessfulHealthProbes(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	app := fiber.New()
	app.Use(FiberMiddleware(logger, noop.NewTracerProvider().Tracer("test")))
	app.Get("/health", func(c *fiber.Ctx) error { return c.SendString("ok") })
	app.Get("/health/ready", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusServiceUnavailable) })
	app.Get("/ping", func(c *fiber.Ctx) error { return c.SendString("pong") })

	for _, target := range []string{"/health", "/health/ready", "/ping"} {
		if _, err := app.Test(httptest.NewRequest("GET", target, nil)); err != nil {
			t.Fatalf("GET %s: %v", target, err)
		}
	}

	logged := buf.String()
	if strings.Contains(logged, `"route":"/health"`) {
		t.Fatalf("a successful health probe must not be logged, got %q", logged)
	}
	if !strings.Contains(logged, "health probe failed") || !strings.Contains(logged, `"route":"/health/ready"`) {
		t.Fatalf("a failing health probe must still be logged, got %q", logged)
	}
	if strings.Count(logged, "http request completed") != 1 || !strings.Contains(logged, `"route":"/ping"`) {
		t.Fatalf("ordinary requests must be logged exactly as before, got %q", logged)
	}
}

func TestGRPCServerInterceptor_SkipsSuccessfulHealthChecks(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	icpt := GRPCServerInterceptor(logger, noop.NewTracerProvider().Tracer("test"))
	ok := func(context.Context, any) (any, error) { return "ok", nil }
	fail := func(context.Context, any) (any, error) { return nil, errors.New("db down") }

	_, _ = icpt(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/grpc.health.v1.Health/Check"}, ok)
	if buf.Len() != 0 {
		t.Fatalf("a successful health check must not be logged, got %q", buf.String())
	}
	_, _ = icpt(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/grpc.health.v1.Health/Check"}, fail)
	if !strings.Contains(buf.String(), "health probe failed") {
		t.Fatalf("a failing health check must be logged, got %q", buf.String())
	}
	buf.Reset()
	_, _ = icpt(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/menu.MenuService/List"}, ok)
	if !strings.Contains(buf.String(), "grpc request completed") {
		t.Fatalf("ordinary calls must be logged exactly as before, got %q", buf.String())
	}
}
