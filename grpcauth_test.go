package gossiper

import (
	"bytes"
	"context"
	"log/slog"
	"net"
	"strings"
	"testing"

	"github.com/pieceowater-dev/lotof.lib.gossiper/v2/internal/transport"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	healthgrpc "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func srvIntercept(t *testing.T, secret, sent, method string, exempt ...string) error {
	t.Helper()
	i := ServiceAuthServerInterceptor(secret, exempt...)
	ctx := context.Background()
	if sent != "" {
		ctx = metadata.NewIncomingContext(ctx, metadata.Pairs(ServiceAuthMetadataKey, sent))
	}
	_, err := i(ctx, nil, &grpc.UnaryServerInfo{FullMethod: method}, func(context.Context, any) (any, error) { return nil, nil })
	return err
}

func TestServiceAuthServer_NoSecret_AllowsAll(t *testing.T) {
	if err := srvIntercept(t, "", "", "/x.Y/Z"); err != nil {
		t.Fatalf("no-secret must be a no-op, got %v", err)
	}
}

func TestServiceAuthServer_Enforces(t *testing.T) {
	if err := srvIntercept(t, "topsecret", "topsecret", "/x.Y/Z"); err != nil {
		t.Fatalf("correct token must pass, got %v", err)
	}
	if err := srvIntercept(t, "topsecret", "wrong", "/x.Y/Z"); err == nil {
		t.Fatal("wrong token must be rejected")
	}
	if err := srvIntercept(t, "topsecret", "", "/x.Y/Z"); err == nil {
		t.Fatal("missing token must be rejected")
	}
}

func TestServiceAuthServer_ExemptPrefix(t *testing.T) {
	if err := srvIntercept(t, "topsecret", "", "/grpc.health.v1.Health/Check", "/grpc.health.v1.Health/"); err != nil {
		t.Fatalf("exempt method must pass without a token, got %v", err)
	}
}

func TestServiceAuthClient_AppendsWhenSet(t *testing.T) {
	got := ""
	inv := func(ctx context.Context, _ string, _, _ any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
		md, _ := metadata.FromOutgoingContext(ctx)
		if v := md.Get(ServiceAuthMetadataKey); len(v) > 0 {
			got = v[0]
		}
		return nil
	}
	_ = ServiceAuthClientInterceptor("abc")(context.Background(), "/x/y", nil, nil, nil, inv)
	if got != "abc" {
		t.Fatalf("client interceptor did not attach the token, got %q", got)
	}
	got = ""
	_ = ServiceAuthClientInterceptor("")(context.Background(), "/x/y", nil, nil, nil, inv)
	if got != "" {
		t.Fatalf("empty secret must not attach anything, got %q", got)
	}
}

func srvInterceptMode(t *testing.T, secret, mode, sent string) (called bool, err error) {
	t.Helper()
	i := ServiceAuthServerInterceptorWithMode(secret, mode, "/grpc.health.v1.Health/")
	ctx := context.Background()
	if sent != "" {
		ctx = metadata.NewIncomingContext(ctx, metadata.Pairs(ServiceAuthMetadataKey, sent))
	}
	_, err = i(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/x.Y/Z"}, func(context.Context, any) (any, error) {
		called = true
		return nil, nil
	})
	return called, err
}

func TestServiceAuthServer_ReportModeAllowsButLogs(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	called, err := srvInterceptMode(t, "topsecret", ServiceAuthReport, "")
	if err != nil || !called {
		t.Fatalf("report mode must let a call without a token through, got called=%v err=%v", called, err)
	}
	if !strings.Contains(buf.String(), "report mode") || !strings.Contains(buf.String(), "missing token") {
		t.Fatalf("report mode must log the unauthenticated call, got %q", buf.String())
	}

	buf.Reset()
	if called, err := srvInterceptMode(t, "topsecret", ServiceAuthReport, "topsecret"); err != nil || !called {
		t.Fatalf("a valid token must pass in report mode, got called=%v err=%v", called, err)
	}
	if buf.Len() != 0 {
		t.Fatalf("a valid token must not be logged, got %q", buf.String())
	}
}

func TestServiceAuthServer_UnknownModeEnforces(t *testing.T) {
	for _, mode := range []string{"", "enforce", "Report", "off"} {
		if called, err := srvInterceptMode(t, "topsecret", mode, ""); err == nil || called {
			t.Fatalf("mode %q must enforce (fail closed), got called=%v err=%v", mode, called, err)
		}
	}
}

func TestWithClientInterceptors_AppliesRegisteredOnes(t *testing.T) {
	saved := transport.ClientUnaryInterceptors()
	t.Cleanup(func() { transport.ResetClientUnaryInterceptorsForTest(saved) })
	transport.ResetClientUnaryInterceptorsForTest(nil)
	RegisterClientUnaryInterceptor(ServiceAuthClientInterceptor("abc"))

	lis := bufconn.Listen(1 << 20)
	srv, _ := NewGRPCServer(ServiceAuthServerInterceptor("abc"))
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	dial := func(opts ...grpc.DialOption) error {
		conn, err := grpc.NewClient("passthrough:///bufnet", append([]grpc.DialOption{
			grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		}, opts...)...)
		if err != nil {
			return err
		}
		defer conn.Close()
		// A registered, non-exempt method (unknown methods are answered with
		// Unimplemented by grpc-go before any interceptor runs).
		_, err = healthgrpc.NewHealthClient(conn).Check(context.Background(), &healthgrpc.HealthCheckRequest{})
		return err
	}
	if code := status.Code(dial()); code != codes.Unauthenticated {
		t.Fatalf("a plain dial must go out without the token, got %v", code)
	}
	if err := dial(WithClientInterceptors()); err != nil {
		t.Fatalf("WithClientInterceptors must attach the registered token, got %v", err)
	}
}
