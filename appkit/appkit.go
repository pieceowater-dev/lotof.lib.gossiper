// Package appkit holds the scaffolding every service's cmd/server/main.go and
// cfg package used to copy: the start/stop-on-signal main loop and the
// environment helpers.
package appkit

import (
	"log/slog"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"syscall"
)

// Application is what a service's internal.NewApp() returns.
type Application interface {
	// Start blocks while the service runs.
	Start()
	// Stop shuts the service down.
	Stop()
}

// Run starts app, waits for SIGINT/SIGTERM and stops it gracefully. It is the
// whole body of a service's main().
func Run(app Application) {
	go app.Start()
	slog.Info("Application started successfully")

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

	<-signalChan
	slog.Info("Received shutdown signal")

	app.Stop()
	slog.Info("Application stopped successfully")
}

// Env is the environment variable's value, or def when it is not set.
func Env(key, def string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return def
}

// EnvFloat is Env parsed as a float; def when unset or unparsable.
func EnvFloat(key string, def float64) float64 {
	if v, ok := os.LookupEnv(key); ok {
		if parsed, err := strconv.ParseFloat(v, 64); err == nil {
			return parsed
		}
	}
	return def
}

// EnvInt is Env parsed as an int; def when unset or unparsable.
func EnvInt(key string, def int) int {
	if v, ok := os.LookupEnv(key); ok {
		if parsed, err := strconv.Atoi(v); err == nil {
			return parsed
		}
	}
	return def
}

// EnvBool is true only for the value "true"; def when unset.
func EnvBool(key string, def bool) bool {
	if v, ok := os.LookupEnv(key); ok {
		return v == "true"
	}
	return def
}

// NormalizePostgresDSN adds the query parameters the shared tenant pool needs
// (no prepared-statement cache, simple protocol) unless the DSN sets them.
func NormalizePostgresDSN(raw string) string {
	if raw == "" {
		return raw
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	query := parsed.Query()
	if query.Get("statement_cache_capacity") == "" {
		query.Set("statement_cache_capacity", "0")
	}
	if query.Get("default_query_exec_mode") == "" {
		query.Set("default_query_exec_mode", "simple_protocol")
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}
