package gossiper

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/grpc/health"
	healthgrpc "google.golang.org/grpc/health/grpc_health_v1"
)

func readinessStatus(t *testing.T, hs *health.Server, service string) healthgrpc.HealthCheckResponse_ServingStatus {
	t.Helper()
	resp, err := hs.Check(context.Background(), &healthgrpc.HealthCheckRequest{Service: service})
	if err != nil {
		return healthgrpc.HealthCheckResponse_SERVICE_UNKNOWN // not published yet
	}
	return resp.GetStatus()
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition not reached in time")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestWatchReadiness_FollowsCheckAndLeavesLivenessAlone(t *testing.T) {
	hs := health.NewServer()
	hs.SetServingStatus("", healthgrpc.HealthCheckResponse_SERVING)
	var failing atomic.Bool
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go WatchReadiness(ctx, hs, 10*time.Millisecond, func(context.Context) error {
		if failing.Load() {
			return errors.New("db down")
		}
		return nil
	})

	waitFor(t, func() bool { return readinessStatus(t, hs, ReadinessService) == healthgrpc.HealthCheckResponse_SERVING })

	failing.Store(true)
	waitFor(t, func() bool {
		return readinessStatus(t, hs, ReadinessService) == healthgrpc.HealthCheckResponse_NOT_SERVING
	})
	if got := readinessStatus(t, hs, ""); got != healthgrpc.HealthCheckResponse_SERVING {
		t.Fatalf("liveness (\"\") must not follow the dependency check, got %v", got)
	}

	failing.Store(false)
	waitFor(t, func() bool { return readinessStatus(t, hs, ReadinessService) == healthgrpc.HealthCheckResponse_SERVING })
}

func TestWatchReadiness_StartsNotServing(t *testing.T) {
	hs := health.NewServer()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	block := make(chan struct{})
	defer close(block)
	go WatchReadiness(ctx, hs, time.Hour, func(context.Context) error { <-block; return nil })
	waitFor(t, func() bool {
		resp, err := hs.Check(context.Background(), &healthgrpc.HealthCheckRequest{Service: ReadinessService})
		return err == nil && resp.GetStatus() == healthgrpc.HealthCheckResponse_NOT_SERVING
	})
}
