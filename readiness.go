package gossiper

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/grpc/health"
	healthgrpc "google.golang.org/grpc/health/grpc_health_v1"
)

// ReadinessService is the gRPC health service name that says whether the
// service can do its job right now (its database answers), as opposed to ""
// which only says the process is up. Point readiness probes at it
// (grpc: {port: 50051, service: readiness}) and keep liveness on "": a
// database outage then takes pods out of rotation instead of making the
// kubelet restart them in a loop.
const ReadinessService = "readiness"

// WatchReadiness runs check every interval until ctx is done and publishes the
// result under ReadinessService: SERVING while check returns nil, NOT_SERVING
// otherwise. It starts NOT_SERVING, runs the first check immediately, and
// logs every transition. Run it in its own goroutine.
func WatchReadiness(ctx context.Context, hs *health.Server, interval time.Duration, check func(context.Context) error) {
	hs.SetServingStatus(ReadinessService, healthgrpc.HealthCheckResponse_NOT_SERVING)
	ready := false
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		err := check(ctx)
		switch {
		case err == nil && !ready:
			ready = true
			hs.SetServingStatus(ReadinessService, healthgrpc.HealthCheckResponse_SERVING)
			slog.Info("readiness: serving")
		case err != nil && ready:
			ready = false
			hs.SetServingStatus(ReadinessService, healthgrpc.HealthCheckResponse_NOT_SERVING)
			slog.Warn("readiness: not serving", slog.String("error", err.Error()))
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// PingDatabase returns a readiness check that pings db, bounded to 2 seconds.
func PingDatabase(db Database) func(context.Context) error {
	return func(ctx context.Context) error {
		sqlDB, err := db.GetDB().DB()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		return sqlDB.PingContext(ctx)
	}
}
