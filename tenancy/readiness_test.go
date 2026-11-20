package tenancy

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// readyAfter says how many readiness checks report "not ready" before the
// tenant becomes ready; a negative value means never.
type stubBackend struct {
	readyAfter int
	// migrationStatus is what Info reports; "" means this service has no
	// record of the namespace at all.
	migrationStatus string
	checks          int
	checkErr        error
	warmups         int
	ensures         int
	ensureErr       error
	readyOnEnsure   bool
}

func (s *stubBackend) IsReady(context.Context, string) (bool, error) {
	s.checks++
	if s.checkErr != nil {
		return false, s.checkErr
	}
	if s.readyAfter < 0 {
		return false, nil
	}
	return s.checks > s.readyAfter, nil
}

func (s *stubBackend) Info(context.Context, string) (string, string, error) {
	return s.migrationStatus, "", nil
}

func (s *stubBackend) TriggerWarmup() { s.warmups++ }

func (s *stubBackend) EnsureAppTenant(context.Context, string, string) error {
	s.ensures++
	if s.ensureErr == nil && s.readyOnEnsure {
		s.readyAfter = 0
	}
	return s.ensureErr
}

func callReadiness(t *testing.T, b ReadinessBackend, ctx context.Context, maxWait time.Duration) (bool, error) {
	t.Helper()
	return callGate(t, NewReadinessWithWait(b, []string{"/exempt.Service/Method"}, maxWait), ctx)
}

// callGate sends one request through an existing gate, as a service's single
// gate serves every request.
func callGate(t *testing.T, gate *Readiness, ctx context.Context) (bool, error) {
	t.Helper()
	handled := false
	handler := func(context.Context, any) (any, error) {
		handled = true
		return "ok", nil
	}
	icpt := gate.MiddlewareMethod()
	_, err := icpt(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/plans.Service/List"}, handler)
	return handled, err
}

func TestReadiness_ReadyTenantPassesStraightThrough(t *testing.T) {
	b := &stubBackend{readyAfter: 0}
	handled, err := callReadiness(t, b, WithUserMetadata(context.Background(), "user-1", "ns", ""), time.Second)
	if err != nil || !handled {
		t.Fatalf("expected the handler to run, got handled=%v err=%v", handled, err)
	}
	if b.warmups != 0 || b.ensures != 0 {
		t.Fatalf("a ready tenant must not trigger warmup or provisioning, got %d/%d", b.warmups, b.ensures)
	}
}

func TestReadiness_NoNamespaceIsNotGated(t *testing.T) {
	b := &stubBackend{readyAfter: -1}
	handled, err := callReadiness(t, b, context.Background(), time.Second)
	if err != nil || !handled || b.checks != 0 {
		t.Fatalf("handled=%v err=%v checks=%d", handled, err, b.checks)
	}
}

func TestReadiness_UnknownTenantGivesUpWithinBudget(t *testing.T) {
	b := &stubBackend{readyAfter: -1, migrationStatus: "running"}
	start := time.Now()
	handled, err := callReadiness(t, b, WithUserMetadata(context.Background(), "", "ns-nobody-has", ""), 300*time.Millisecond)
	elapsed := time.Since(start)

	if handled {
		t.Fatal("the handler must not run for a tenant that never becomes ready")
	}
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("expected Unavailable, got %v", err)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("waited %s, which is far past the 300ms budget", elapsed)
	}
	if b.warmups != 1 {
		t.Fatalf("warmup must be triggered exactly once per request, got %d", b.warmups)
	}
}

func TestReadiness_RetriesWhileMigrationFinishes(t *testing.T) {
	b := &stubBackend{readyAfter: 2, migrationStatus: "running"}
	handled, err := callReadiness(t, b, WithUserMetadata(context.Background(), "", "ns", ""), 2*time.Second)
	if err != nil || !handled {
		t.Fatalf("expected the handler to run after the tenant became ready, got handled=%v err=%v", handled, err)
	}
	if b.checks < 3 {
		t.Fatalf("expected the readiness check to be retried, got %d checks", b.checks)
	}
}

func TestReadiness_FirstConnectionProvisionsThroughHub(t *testing.T) {
	b := &stubBackend{readyAfter: -1, readyOnEnsure: true, migrationStatus: "running"}
	handled, err := callReadiness(t, b, WithUserMetadata(context.Background(), "user-1", "ns-new", ""), time.Second)
	if err != nil || !handled {
		t.Fatalf("expected provisioning to unblock the request, got handled=%v err=%v", handled, err)
	}
	if b.ensures != 1 {
		t.Fatalf("expected exactly one provisioning attempt, got %d", b.ensures)
	}
}

func TestReadiness_AnonymousCallerIsNotProvisioned(t *testing.T) {
	b := &stubBackend{readyAfter: -1, readyOnEnsure: true, migrationStatus: "running"}
	if _, err := callReadiness(t, b, WithUserMetadata(context.Background(), "", "ns-new", ""), 300*time.Millisecond); status.Code(err) != codes.Unavailable {
		t.Fatalf("expected Unavailable, got %v", err)
	}
	if b.ensures != 0 {
		t.Fatal("provisioning must be scoped to a caller's own membership, never done for an anonymous call")
	}
}

func TestReadiness_CheckErrorIsReported(t *testing.T) {
	b := &stubBackend{checkErr: errors.New("db down"), migrationStatus: "running"}
	_, err := callReadiness(t, b, WithUserMetadata(context.Background(), "", "ns", ""), 200*time.Millisecond)
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("expected Unavailable, got %v", err)
	}
	if got := status.Convert(err).Message(); !strings.Contains(got, "db down") {
		t.Fatalf("the underlying error should stay visible, got %q", got)
	}
}

func TestReadiness_UnprovisionedNamespaceFailsFast(t *testing.T) {
	// No migration record: waiting cannot help, and FailedPrecondition is a
	// code the gateway's read retries leave alone.
	b := &stubBackend{readyAfter: -1, migrationStatus: ""}
	start := time.Now()
	handled, err := callReadiness(t, b, WithUserMetadata(context.Background(), "", "ns-made-up", ""), 3*time.Second)
	elapsed := time.Since(start)

	if handled {
		t.Fatal("the handler must not run for a namespace this service does not have")
	}
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("expected FailedPrecondition, got %v", err)
	}
	if elapsed > time.Second {
		t.Fatalf("an unprovisioned namespace must not consume the wait budget, took %s", elapsed)
	}
	if b.warmups != 1 {
		t.Fatalf("warmup should still be triggered once, got %d", b.warmups)
	}
}

func TestReadiness_ExemptMethodSkipsTheGate(t *testing.T) {
	b := &stubBackend{readyAfter: -1}
	handler := func(context.Context, any) (any, error) { return "ok", nil }
	icpt := NewReadinessWithWait(b, []string{"/exempt.Service/Method"}, time.Second).MiddlewareMethod()
	ctx := WithUserMetadata(context.Background(), "", "ns", "")
	if _, err := icpt(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/exempt.Service/Method"}, handler); err != nil {
		t.Fatalf("an exempt method must not be gated, got %v", err)
	}
	if b.checks != 0 {
		t.Fatalf("an exempt method must not be checked, got %d checks", b.checks)
	}
}

// slowEnsurer provisions after a delay, on whatever context it is handed, and
// reports whether that context was cancelled before it finished.
type slowEnsurer struct {
	mu        sync.Mutex
	calls     int
	delay     time.Duration
	cancelled bool
	finished  chan struct{}
	once      sync.Once
	ready     bool
}

func (s *slowEnsurer) IsReady(context.Context, string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ready, nil
}

func (s *slowEnsurer) TriggerWarmup() {}

func (s *slowEnsurer) Info(context.Context, string) (string, string, error) {
	return "", "", nil // no record yet: the very first connection
}

func (s *slowEnsurer) EnsureAppTenant(ctx context.Context, _, _ string) error {
	s.mu.Lock()
	s.calls++
	s.mu.Unlock()
	select {
	case <-time.After(s.delay):
	case <-ctx.Done():
		s.mu.Lock()
		s.cancelled = true
		s.mu.Unlock()
		s.once.Do(func() { close(s.finished) })
		return ctx.Err()
	}
	s.mu.Lock()
	s.ready = true
	s.mu.Unlock()
	s.once.Do(func() { close(s.finished) })
	return nil
}

func TestReadiness_SlowProvisioningAnswersTenantPreparingNotNotProvisioned(t *testing.T) {
	b := &slowEnsurer{delay: 400 * time.Millisecond, finished: make(chan struct{})}
	_, err := callReadiness(t, b, WithUserMetadata(context.Background(), "user-1", "ns-new", ""), 100*time.Millisecond)
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("a tenant still being provisioned must answer Unavailable, got %v", err)
	}
	if !strings.HasPrefix(status.Convert(err).Message(), TenantPreparingCode) {
		t.Fatalf("the answer must carry the %q code, got %q", TenantPreparingCode, status.Convert(err).Message())
	}
	<-b.finished // the provisioning goes on after the request has given up
	if b.cancelled {
		t.Fatal("provisioning was cancelled together with the request that started it")
	}

	handled, err := callReadiness(t, b, WithUserMetadata(context.Background(), "user-1", "ns-new", ""), time.Second)
	if err != nil || !handled {
		t.Fatalf("the retry must pass once provisioning finished, handled=%v err=%v", handled, err)
	}
}

func TestReadiness_ConcurrentRequestsShareOneProvisioning(t *testing.T) {
	b := &slowEnsurer{delay: 300 * time.Millisecond, finished: make(chan struct{})}
	gate := NewReadinessWithWait(b, nil, 2*time.Second)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			handled, err := callGate(t, gate, WithUserMetadata(context.Background(), "user-1", "ns-new", ""))
			if err != nil || !handled {
				t.Errorf("handled=%v err=%v", handled, err)
			}
		}()
	}
	wg.Wait()
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.calls != 1 {
		t.Fatalf("eight simultaneous first requests started %d provisionings, want 1", b.calls)
	}
}

func TestReadiness_CancelledRequestDoesNotStopProvisioning(t *testing.T) {
	b := &slowEnsurer{delay: 200 * time.Millisecond, finished: make(chan struct{})}
	ctx, cancel := context.WithCancel(WithUserMetadata(context.Background(), "user-1", "ns-new", ""))
	go func() { time.Sleep(30 * time.Millisecond); cancel() }()
	if _, err := callReadiness(t, b, ctx, 2*time.Second); status.Code(err) != codes.Unavailable {
		t.Fatalf("expected Unavailable for the cancelled request, got %v", err)
	}
	<-b.finished
	if b.cancelled {
		t.Fatal("cancelling the request cancelled the provisioning")
	}
}
