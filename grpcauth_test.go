package gossiper

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
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
