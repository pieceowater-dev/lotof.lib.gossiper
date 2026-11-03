package tenancy

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func incoming(kv ...string) context.Context {
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs(kv...))
}

func TestMetadataFromIncoming_AcceptsBothKeySpellings(t *testing.T) {
	md := MetadataFromIncoming(incoming("user-id", "u1", "namespace-slug", "acme", "device_meta", "ios"))
	if md.UserId != "u1" || md.Namespace != "acme" || md.DeviceMeta != "ios" {
		t.Fatalf("got %+v", md)
	}
	md = MetadataFromIncoming(incoming("userid", "u2", "namespace", "beta"))
	if md.UserId != "u2" || md.Namespace != "beta" {
		t.Fatalf("got %+v", md)
	}
	if md := MetadataFromIncoming(context.Background()); md.Namespace != "" || md.UserId != "" {
		t.Fatalf("no metadata must give an empty value, got %+v", md)
	}
}

func TestNilUserMetadataGetters(t *testing.T) {
	var md *UserMetadata
	if md.GetUserId() != "" || md.GetNamespace() != "" || md.GetDeviceMeta() != "" {
		t.Fatal("getters on nil must be empty")
	}
	if NamespaceFromContext(context.Background()) != "" {
		t.Fatal("no metadata means no namespace")
	}
}

func runInterceptor(icpt grpc.UnaryServerInterceptor, ctx context.Context, method string) (context.Context, error) {
	var seen context.Context
	_, err := icpt(ctx, nil, &grpc.UnaryServerInfo{FullMethod: method}, func(c context.Context, _ any) (any, error) {
		seen = c
		return nil, nil
	})
	return seen, err
}

func TestMetadataInterceptor(t *testing.T) {
	icpt := NewMetadataInterceptor([]string{"/health/Check"}).MiddlewareMethod()

	seen, err := runInterceptor(icpt, incoming("namespace", "acme"), "/svc/Method")
	if err != nil || NamespaceFromContext(seen) != "acme" {
		t.Fatalf("err=%v ns=%q", err, NamespaceFromContext(seen))
	}

	if _, err := runInterceptor(icpt, incoming("user-id", "u1"), "/svc/Method"); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("a call without a namespace must be refused, got %v", err)
	}

	if _, err := runInterceptor(icpt, context.Background(), "/health/Check"); err != nil {
		t.Fatalf("excluded methods pass untouched, got %v", err)
	}
}

func TestMetadataInterceptor_RequireUser(t *testing.T) {
	icpt := NewMetadataInterceptor(nil).RequireUser().MiddlewareMethod()
	if _, err := runInterceptor(icpt, incoming("namespace", "acme"), "/svc/Method"); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v", err)
	}
	if _, err := runInterceptor(icpt, incoming("namespace", "acme", "user-id", "u"), "/svc/Method"); err != nil {
		t.Fatal(err)
	}
}
