package tenancy

import (
	"context"
	"errors"
	"strings"
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
	handled := false
	handler := func(context.Context, any) (any, error) {
		handled = true
		return "ok", nil
	}
	icpt := NewReadinessWithWait(b, []string{"/exempt.Service/Method"}, maxWait).MiddlewareMethod()
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
