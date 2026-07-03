package transport

import (
	"context"
	"errors"
	"log/slog"
	"reflect"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// defaultCallTimeout is applied to Send calls that arrive without a deadline.
const defaultCallTimeout = 10 * time.Second

// sendContextMiddleware is an optional hook called on every outgoing gRPC Send.
// Register it once at startup via RegisterSendContextMiddleware to inject
// x-request-id, W3C traceparent, and any other cross-cutting metadata.
var sendContextMiddleware func(ctx context.Context) context.Context

// RegisterSendContextMiddleware sets a function that enriches the context before
// every outgoing gRPC call made by this transport. Calling it again overwrites the
// previous registration. Safe to call before any transport is created.
func RegisterSendContextMiddleware(fn func(ctx context.Context) context.Context) {
	sendContextMiddleware = fn
}

// retryServiceConfig enables transparent retries on transient failures.
const retryServiceConfig = `{
	"methodConfig": [{
		"name": [{"service": ""}],
		"retryPolicy": {
			"maxAttempts": 3,
			"initialBackoff": "0.1s",
			"maxBackoff": "1s",
			"backoffMultiplier": 2.0,
			"retryableStatusCodes": ["UNAVAILABLE", "RESOURCE_EXHAUSTED"]
		}
	}]
}`

// GRPCTransport handles client-side gRPC transport.
type GRPCTransport struct {
	address string
}

func NewGRPCTransport(address string) *GRPCTransport {
	return &GRPCTransport{address: address}
}

// CreateClient creates a gRPC client using the passed constructor.
// The underlying connection includes OTel trace propagation, retry policy,
// and a stats handler for observability.
func (g *GRPCTransport) CreateClient(clientConstructor any) (any, error) {
	conn, err := grpc.NewClient(
		g.address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultServiceConfig(retryServiceConfig),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		return nil, errors.New("failed to connect to gRPC server: " + err.Error())
	}

	constructorValue := reflect.ValueOf(clientConstructor)
	if constructorValue.Kind() != reflect.Func {
		return nil, errors.New("clientConstructor must be a function")
	}

	clientValues := constructorValue.Call([]reflect.Value{reflect.ValueOf(conn)})
	if len(clientValues) > 0 {
		return clientValues[0].Interface(), nil
	}
	return nil, errors.New("failed to create gRPC client")
}

// Send invokes a gRPC method by name via reflection.
// If the context has no deadline, a default timeout is applied.
func (g *GRPCTransport) Send(ctx context.Context, client any, serviceMethod string, request any) (any, error) {
	clientValue := reflect.ValueOf(client)
	method := clientValue.MethodByName(serviceMethod)
	if !method.IsValid() {
		return nil, errors.New("invalid service method: " + serviceMethod)
	}

	if sendContextMiddleware != nil {
		ctx = sendContextMiddleware(ctx)
	}

	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, defaultCallTimeout)
		defer cancel()
	}

	reqValue := reflect.ValueOf(request)
	if !reqValue.IsValid() {
		return nil, errors.New("invalid request for method: " + serviceMethod)
	}

	returnValues := method.Call([]reflect.Value{reflect.ValueOf(ctx), reqValue})
	if len(returnValues) > 1 && returnValues[1].Interface() != nil {
		err := returnValues[1].Interface().(error)
		slog.Error("gRPC call failed", "method", serviceMethod, "target", g.address, "error", err)
		return nil, err
	}
	return returnValues[0].Interface(), nil
}
