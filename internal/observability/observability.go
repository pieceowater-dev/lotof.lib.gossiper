package observability

import (
	"context"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// Config controls how observability is initialised.
type Config struct {
	ServiceName  string
	Environment  string
	OtlpEndpoint string
	SampleRatio  float64
	LogLevel     slog.Level
}

type ctxKey string

const (
	ctxKeyRequestID ctxKey = "request-id"
	ctxKeyTenant    ctxKey = "tenant"
	ctxKeyUser      ctxKey = "user"
)

// Init sets up the OTLP tracer provider and a JSON structured logger.
// Returns logger, tracer, shutdown func, and any init error.
// On error the caller should fall back to slog.Default() and a noop tracer.
func Init(ctx context.Context, cfg Config) (*slog.Logger, trace.Tracer, func(context.Context) error, error) {
	res, err := resource.Merge(
		resource.Default(),
		resource.NewSchemaless(
			semconv.ServiceName(cfg.ServiceName),
			attribute.String("environment", cfg.Environment),
		),
	)
	if err != nil {
		return nil, nil, nil, err
	}

	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(normalizeOTLPEndpoint(cfg.OtlpEndpoint)),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return nil, nil, nil, err
	}

	tp := tracesdk.NewTracerProvider(
		tracesdk.WithBatcher(exporter),
		tracesdk.WithSampler(tracesdk.TraceIDRatioBased(cfg.SampleRatio)),
		tracesdk.WithResource(res),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level:     cfg.LogLevel,
		AddSource: true,
	})).With("service", cfg.ServiceName, "environment", cfg.Environment)

	slog.SetDefault(logger)

	shutdown := func(ctx context.Context) error { return tp.Shutdown(ctx) }
	return logger, tp.Tracer(cfg.ServiceName), shutdown, nil
}

// RequestID extracts the request ID from context.
func RequestID(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKeyRequestID).(string); ok {
		return v
	}
	return ""
}

// WithRequestData attaches request-scoped fields (requestID, tenant, user) to context.
func WithRequestData(ctx context.Context, requestID, tenant, user string) context.Context {
	ctx = context.WithValue(ctx, ctxKeyRequestID, requestID)
	if tenant != "" {
		ctx = context.WithValue(ctx, ctxKeyTenant, tenant)
	}
	if user != "" {
		ctx = context.WithValue(ctx, ctxKeyUser, user)
	}
	return ctx
}

// LoggerFromContext enriches the base logger with request_id, trace_id, span_id, tenant, user
// from context — giving every log line full tracing context.
func LoggerFromContext(ctx context.Context, base *slog.Logger) *slog.Logger {
	if base == nil {
		base = slog.Default()
	}
	var attrs []any
	if id := RequestID(ctx); id != "" {
		attrs = append(attrs, "request_id", id)
	}
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		attrs = append(attrs, "trace_id", sc.TraceID().String(), "span_id", sc.SpanID().String())
	}
	if tenant, _ := ctx.Value(ctxKeyTenant).(string); tenant != "" {
		attrs = append(attrs, "tenant", tenant)
	}
	if user, _ := ctx.Value(ctxKeyUser).(string); user != "" {
		attrs = append(attrs, "user", user)
	}
	if len(attrs) == 0 {
		return base
	}
	return base.With(attrs...)
}

// WithOutgoingMetadata injects request-id and OTel trace headers into outgoing gRPC metadata.
// Call this before every outgoing gRPC call so downstream services can correlate logs.
func WithOutgoingMetadata(ctx context.Context) context.Context {
	reqID := RequestID(ctx)
	if reqID == "" {
		reqID = uuid.NewString()
		ctx = WithRequestData(ctx, reqID, "", "")
	}

	md, _ := metadata.FromOutgoingContext(ctx)
	md = md.Copy()
	md.Set("x-request-id", reqID)

	carrier := metadataCarrier(md)
	otel.GetTextMapPropagator().Inject(ctx, carrier)

	return metadata.NewOutgoingContext(ctx, md)
}

// FiberMiddleware instruments all incoming HTTP requests:
// - extracts or generates request_id
// - propagates OTel trace context from headers
// - creates a server span for the request
// - logs completion (or failure) with full tracing fields
func FiberMiddleware(logger *slog.Logger, tracer trace.Tracer) func(c *fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		ctx := c.UserContext()
		if ctx == nil {
			ctx = context.Background()
		}

		headers := c.GetReqHeaders()
		carrier := make(mapCarrier, len(headers))
		for k, values := range headers {
			if len(values) > 0 {
				carrier[strings.ToLower(k)] = values[0]
			}
		}

		ctx = otel.GetTextMapPropagator().Extract(ctx, carrier)

		reqID := firstNonEmpty(
			carrier.Get("x-request-id"),
			carrier.Get("x-correlation-id"),
		)
		if reqID == "" {
			reqID = uuid.NewString()
		}

		ctx = WithRequestData(ctx, reqID, c.Get("Namespace"), c.Get("Authorization"))
		ctx = WithOutgoingMetadata(ctx)

		ctx, span := tracer.Start(ctx, c.Method()+" "+c.Route().Path,
			trace.WithSpanKind(trace.SpanKindServer),
		)
		defer span.End()

		c.SetUserContext(ctx)
		c.Set("X-Request-ID", reqID)

		err := c.Next()

		log := LoggerFromContext(ctx, logger)
		httpStatus := c.Response().StatusCode()

		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			log.Error("http request failed",
				slog.String("method", c.Method()),
				slog.String("route", c.Route().Path),
				slog.Int("status", httpStatus),
				slog.Duration("latency_ms", time.Since(start)),
				slog.String("remote_ip", c.IP()),
				slog.String("error", err.Error()),
			)
			return err
		}

		span.SetAttributes(
			attribute.String("http.method", c.Method()),
			attribute.String("http.route", c.Route().Path),
			attribute.Int("http.status_code", httpStatus),
		)
		log.Info("http request completed",
			slog.String("method", c.Method()),
			slog.String("route", c.Route().Path),
			slog.Int("status", httpStatus),
			slog.Duration("latency_ms", time.Since(start)),
			slog.String("remote_ip", c.IP()),
		)
		return nil
	}
}

// GRPCServerInterceptor adds tracing, structured logging, and request_id propagation
// to every incoming gRPC unary call. Use as grpc.UnaryInterceptor on the server.
func GRPCServerInterceptor(logger *slog.Logger, tracer trace.Tracer) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		md, _ := metadata.FromIncomingContext(ctx)
		ctx = otel.GetTextMapPropagator().Extract(ctx, metadataCarrier(md))

		reqID := firstFromMD(md, "x-request-id", "x-correlation-id")
		if reqID == "" {
			reqID = uuid.NewString()
		}

		ctx = WithRequestData(ctx, reqID, "", "")
		ctx = WithOutgoingMetadata(ctx)

		ctx, span := tracer.Start(ctx, info.FullMethod, trace.WithSpanKind(trace.SpanKindServer))
		defer span.End()

		resp, err := handler(ctx, req)

		log := LoggerFromContext(ctx, logger)
		grpcCode := status.Code(err)

		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			log.Error("grpc request failed",
				slog.String("method", info.FullMethod),
				slog.String("grpc_code", grpcCode.String()),
				slog.String("error", err.Error()),
			)
			return resp, err
		}

		log.Info("grpc request completed",
			slog.String("method", info.FullMethod),
			slog.String("grpc_code", grpcCode.String()),
		)
		return resp, nil
	}
}

// GRPCClientInterceptor propagates request_id and OTel trace context on every outgoing
// gRPC call and records client spans. Use as grpc.WithUnaryInterceptor on the client.
func GRPCClientInterceptor(logger *slog.Logger, tracer trace.Tracer) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		ctx = WithOutgoingMetadata(ctx)
		ctx, span := tracer.Start(ctx, method, trace.WithSpanKind(trace.SpanKindClient))
		defer span.End()

		err := invoker(ctx, method, req, reply, cc, opts...)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			LoggerFromContext(ctx, logger).Error("grpc client call failed",
				slog.String("method", method),
				slog.String("error", err.Error()),
			)
			return err
		}

		LoggerFromContext(ctx, logger).Info("grpc client call completed",
			slog.String("method", method),
		)
		return nil
	}
}

// mapCarrier adapts a plain string map to an OTel propagation.TextMapCarrier.
type mapCarrier map[string]string

func (c mapCarrier) Get(key string) string        { return c[strings.ToLower(key)] }
func (c mapCarrier) Set(key, value string)        { c[strings.ToLower(key)] = value }
func (c mapCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	return keys
}

// metadataCarrier adapts gRPC metadata to an OTel propagation.TextMapCarrier.
type metadataCarrier metadata.MD

func (m metadataCarrier) Get(key string) string {
	if vals := metadata.MD(m).Get(key); len(vals) > 0 {
		return vals[0]
	}
	return ""
}
func (m metadataCarrier) Set(key, value string) { metadata.MD(m).Set(key, value) }
func (m metadataCarrier) Keys() []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func firstFromMD(md metadata.MD, keys ...string) string {
	for _, k := range keys {
		if v := md.Get(k); len(v) > 0 && v[0] != "" {
			return v[0]
		}
	}
	return ""
}

// normalizeOTLPEndpoint strips scheme prefixes from the endpoint URL
// because the OTLP HTTP exporter expects host:port, not a full URL.
func normalizeOTLPEndpoint(endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return endpoint
	}
	if strings.Contains(endpoint, "://") {
		if parsed, err := url.Parse(endpoint); err == nil && parsed.Host != "" {
			return parsed.Host
		}
	}
	endpoint = strings.TrimPrefix(endpoint, "http://")
	endpoint = strings.TrimPrefix(endpoint, "https://")
	return strings.TrimSuffix(endpoint, "/")
}
