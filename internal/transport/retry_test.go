package transport

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func callWith(t *testing.T, ctx context.Context, method string, fail codes.Code, failTimes int) (int, error) {
	t.Helper()
	calls := 0
	invoker := func(context.Context, string, any, any, *grpc.ClientConn, ...grpc.CallOption) error {
		calls++
		if calls <= failTimes {
			return status.Error(fail, "nope")
		}
		return nil
	}
	err := RetryReadsUnaryClientInterceptor()(ctx, method, nil, nil, nil, invoker)
	return calls, err
}

func TestRetryReads_RetriesReadsOnUnavailable(t *testing.T) {
	calls, err := callWith(t, context.Background(), "/menu.MenuService/GetMenuItem", codes.Unavailable, 2)
	if err != nil || calls != 3 {
		t.Fatalf("a read must be retried on Unavailable until it succeeds, got calls=%d err=%v", calls, err)
	}
}

func TestRetryReads_GivesUpAfterThreeAttempts(t *testing.T) {
	calls, err := callWith(t, context.Background(), "/menu.MenuService/ListOrders", codes.Unavailable, 10)
	if status.Code(err) != codes.Unavailable || calls != 3 {
		t.Fatalf("expected 3 attempts then the error, got calls=%d err=%v", calls, err)
	}
}

func TestRetryReads_NeverRetriesWrites(t *testing.T) {
	for _, m := range []string{"/menu.OrderService/CreateOrder", "/sub.SubscriptionService/ActivateSubscription", "/x.Y/Delete", "/x.Y/Getaway"} {
		if calls, _ := callWith(t, context.Background(), m, codes.Unavailable, 10); calls != 1 {
			t.Fatalf("%s must not be retried, got %d calls", m, calls)
		}
	}
}

func TestRetryReads_NeverRetriesSideEffectReads(t *testing.T) {
	for _, m := range []string{"/contacts.Bonus/VerifyPin", "/contacts.Identity/VerifyIdentity", "/hub.Links/ResolveDeepLink"} {
		if calls, _ := callWith(t, context.Background(), m, codes.Unavailable, 10); calls != 1 {
			t.Fatalf("%s has a per-call side effect and must not be retried, got %d calls", m, calls)
		}
	}
}

func TestRetryReads_OnlyUnavailable(t *testing.T) {
	for _, c := range []codes.Code{codes.ResourceExhausted, codes.DeadlineExceeded, codes.Internal, codes.NotFound} {
		if calls, _ := callWith(t, context.Background(), "/x.Y/GetThing", c, 10); calls != 1 {
			t.Fatalf("%v must not be retried, got %d calls", c, calls)
		}
	}
}

func TestRetryReads_StopsAtDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	calls, err := callWith(t, ctx, "/x.Y/GetThing", codes.Unavailable, 10)
	if err == nil || calls != 1 || time.Since(start) > 90*time.Millisecond {
		t.Fatalf("must not wait past the deadline (first backoff is 100ms), got calls=%d err=%v after %v", calls, err, time.Since(start))
	}
}

func TestDefaultDeadline_AddsOnlyWhenMissing(t *testing.T) {
	var got time.Time
	var had bool
	invoker := func(ctx context.Context, _ string, _, _ any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
		got, had = ctx.Deadline()
		return nil
	}
	icpt := DefaultDeadlineUnaryClientInterceptor(time.Minute)

	_ = icpt(context.Background(), "/x.Y/Z", nil, nil, nil, invoker)
	if !had || time.Until(got) > time.Minute || time.Until(got) < 50*time.Second {
		t.Fatalf("a call without a deadline must get one of ~1m, got had=%v in %v", had, time.Until(got))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	want, _ := ctx.Deadline()
	_ = icpt(ctx, "/x.Y/Z", nil, nil, nil, invoker)
	if !got.Equal(want) {
		t.Fatalf("an existing deadline must be kept, got %v want %v", got, want)
	}
}
