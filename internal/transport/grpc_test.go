package transport

import (
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/grpc"
)

type fakeGRPCClient struct{}

func newFakeGRPCClient(_ *grpc.ClientConn) *fakeGRPCClient {
	return &fakeGRPCClient{}
}

func (f *fakeGRPCClient) Echo(_ context.Context, req string) (string, error) {
	return "echo:" + req, nil
}

func (f *fakeGRPCClient) Fail(_ context.Context, _ string) (string, error) {
	return "", errors.New("boom")
}

func (f *fakeGRPCClient) Deadline(ctx context.Context, _ string) (time.Time, error) {
	d, _ := ctx.Deadline()
	return d, nil
}

type ctxKeyType string

const ctxKey ctxKeyType = "test-key"

func (f *fakeGRPCClient) ReadCtxValue(ctx context.Context, _ string) (string, error) {
	v, _ := ctx.Value(ctxKey).(string)
	return v, nil
}

func TestGRPCTransport_CreateClient(t *testing.T) {
	transport := NewGRPCTransport("localhost:9000")

	t.Run("valid constructor succeeds", func(t *testing.T) {
		client, err := transport.CreateClient(newFakeGRPCClient)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, ok := client.(*fakeGRPCClient); !ok {
			t.Fatalf("expected *fakeGRPCClient, got %T", client)
		}
	})

	t.Run("non-function constructor rejected", func(t *testing.T) {
		_, err := transport.CreateClient("not a function")
		if err == nil {
			t.Error("expected error for non-function constructor, got nil")
		}
	})
}

func TestGRPCTransport_Send(t *testing.T) {
	transport := NewGRPCTransport("localhost:9000")
	client := &fakeGRPCClient{}

	t.Run("successful call returns response", func(t *testing.T) {
		resp, err := transport.Send(context.Background(), client, "Echo", "hello")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.(string) != "echo:hello" {
			t.Errorf("expected %q, got %q", "echo:hello", resp)
		}
	})

	t.Run("method returning an error propagates it", func(t *testing.T) {
		_, err := transport.Send(context.Background(), client, "Fail", "hello")
		if err == nil || err.Error() != "boom" {
			t.Errorf("expected error %q, got %v", "boom", err)
		}
	})

	t.Run("unknown method name rejected", func(t *testing.T) {
		_, err := transport.Send(context.Background(), client, "DoesNotExist", "hello")
		if err == nil {
			t.Error("expected error for unknown method, got nil")
		}
	})

	t.Run("nil request rejected", func(t *testing.T) {
		_, err := transport.Send(context.Background(), client, "Echo", nil)
		if err == nil {
			t.Error("expected error for nil request, got nil")
		}
	})

	t.Run("applies default timeout when context has no deadline", func(t *testing.T) {
		resp, err := transport.Send(context.Background(), client, "Deadline", "hello")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		deadline := resp.(time.Time)
		if deadline.IsZero() {
			t.Fatal("expected a deadline to be set, got zero value")
		}
		remaining := time.Until(deadline)
		if remaining <= 0 || remaining > defaultCallTimeout {
			t.Errorf("expected remaining time in (0, %v], got %v", defaultCallTimeout, remaining)
		}
	})

	t.Run("preserves an existing deadline", func(t *testing.T) {
		want := time.Now().Add(100 * time.Millisecond)
		ctx, cancel := context.WithDeadline(context.Background(), want)
		defer cancel()

		resp, err := transport.Send(ctx, client, "Deadline", "hello")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got := resp.(time.Time)
		if !got.Equal(want) {
			t.Errorf("expected deadline to be preserved as %v, got %v", want, got)
		}
	})

	t.Run("send context middleware enriches context", func(t *testing.T) {
		RegisterSendContextMiddleware(func(ctx context.Context) context.Context {
			return context.WithValue(ctx, ctxKey, "injected")
		})
		defer RegisterSendContextMiddleware(nil)

		resp, err := transport.Send(context.Background(), client, "ReadCtxValue", "hello")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.(string) != "injected" {
			t.Errorf("expected middleware-injected value %q, got %q", "injected", resp)
		}
	})
}
