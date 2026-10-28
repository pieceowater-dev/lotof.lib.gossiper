# lotof.lib.gossiper

`gossiper` is the shared Go library of the LOTOF platform. Every Go service (gateways `*.gtw`, microservices `*.msvc.*`, Pages edge) imports it for the parts that must not drift between repositories: server baselines, gRPC transport, tenant bookkeeping, observability, secrets hashing, pagination and the MCP endpoint for AI clients.

```bash
go get github.com/pieceowater-dev/lotof.lib.gossiper/v2
```

Module path: `github.com/pieceowater-dev/lotof.lib.gossiper/v2` (Go 1.27). Licence: MIT.

## Versioning

Releases are tagged automatically. Every push to `main` runs `.github/workflows/release.yml`:

1. `checks` job (GitHub-hosted runner, the repository is public): `go mod tidy` must leave `go.mod`/`go.sum` unchanged, `go vet`, `staticcheck`, `gosec`, `govulncheck`, `go test ./...`.
2. `tag` job: takes the highest existing tag and creates the next patch tag (`v2.0.N`). Fails if the tag already exists.

Consequences: a bad commit on `main` that passes the checks is tagged and cannot be un-published, so run the same checks locally before pushing. Services pin a tag in their own `go.mod`; changes to the library are kept additive so old pins keep compiling. Documentation-only commits should carry `[skip ci]` to avoid burning a tag.

## What is inside

| Package / file | Purpose |
|---|---|
| `gossiper.go` | Public facade: `NewServerManager`, `NewGRPCServ`, `NewRESTServ`, `NewDB`, `NewTransportFactory`, filter/sort/pagination helpers, `NewTenantManager`, AES-256 helpers, observability wrappers |
| `grpcserver.go` | `NewGRPCServer(interceptors...)`: baseline inbound gRPC server (see below) |
| `grpcauth.go` | Service-to-service auth: `ServiceAuthClientInterceptor`, `ServiceAuthServerInterceptor[WithMode]` (header `x-service-auth`, modes `enforce` and `report`) |
| `grpcerror.go` | `InternalError`: logs the real error, returns a generic `Internal` status |
| `readiness.go` | `WatchReadiness`, `PingDatabase`: separate `readiness` health service so a database outage removes pods from rotation without restarting them |
| `tx.go` | `InTx`, `InTxSchema`, `ContextWithTx`, `DBFromContext`: transactions carried in the context, per-tenant schema variant |
| `pagination.go` | `ClampPage`, `ClampPageLength`, `ClampOffset` |
| `internal/db` | PostgreSQL (GORM) connection management |
| `internal/servers` | `ServerManager` with gRPC and HTTP (Fiber) servers |
| `internal/transport` | gRPC client factory with keepalive, default 60s deadline, read-only retries on `Unavailable` |
| `internal/tenant` | Encrypted per-tenant connection records |
| `tenancy/` | Per-tenant migration bookkeeping: `Control`, `Warmup`, `ComputeTargetVersion` |
| `observability/` | Structured JSON logs and OpenTelemetry tracing |
| `httpx/` | Fiber helpers: CORS, CSRF guard, health routes, host/scheme detection |
| `secrets/` | bcrypt hashing for PINs and short secrets, with verification of legacy hashes |
| `mcpkit/`, `mcpkit/oauth/` | MCP endpoint and OAuth sign-in for AI clients |
| `cmd/` | `gossiper` CLI entry point and a runnable `example` |

## Servers and transport

```go
sm := gossiper.NewServerManager()

grpcSrv, health := gossiper.NewGRPCServer(myInterceptor) // see baseline below
sm.AddServer(gossiper.NewGRPCServ("50051", grpcSrv, func(s *grpc.Server) { /* register services */ }))

sm.AddServer(gossiper.NewRESTServ("8080", fiber.New(httpx.FiberConfig()), func(app *fiber.App) {
    httpx.RegisterHealthRoutes(app, "my-service")
}))

sm.StartAll()
defer sm.StopAll()
```

`NewGRPCServer` gives every service the same baseline:

- keepalive `MaxConnectionAge` 15 min with a 15 s grace, client pings accepted every 10 s. This is a mitigation for a `golang.org/x/net` HPACK panic that can crash calling gateways on long-lived connections;
- `RecoveryUnaryServerInterceptor` outermost, so a panic becomes `codes.Internal`;
- the standard gRPC health service (starts `NOT_SERVING`, the caller flips it to `SERVING`) and server reflection.

The REST server uses Fiber (not Gin). Clients are created through `NewTransportFactory().CreateTransport(gossiper.GRPC, addr)` or typed clients via `transport.CreateClient`. Chain extra client interceptors (for example the service-auth token) with `RegisterClientUnaryInterceptor` once at startup, and use `WithClientInterceptors()` plus `ClientKeepalive()` on connections dialled by hand.

## Multi-tenancy

Each business gets its own PostgreSQL schema per application. `tenancy.Control` records which tenant is at which schema version, who may migrate it right now, and retries with backoff. `ComputeTargetVersion(models)` hashes the AutoMigrate models, so the migration gate notices schema drift without a hand-bumped constant. `tenancy.Warmup` triggers lazy migration asynchronously, with a panic guard. `InTxSchema` runs a function in a transaction pinned to a tenant schema.

## Observability

`InitObservability(ctx, cfg)` returns a JSON `slog` logger, a tracer and a shutdown function. Spans are exported over OTLP/HTTP only when `Config.OtlpEndpoint` is set; without it trace and span IDs are still assigned, so logs stay correlatable. Plug the Fiber middleware and the gRPC server/client interceptors in to get `request_id`, W3C trace context propagation across services, and `tenant`/`user` fields in log lines.

## MCP for AI clients (`mcpkit`)

`mcpkit` is the shared half of an MCP endpoint (Claude, ChatGPT and other clients) so a service writes only its own tools. It is built on the official `modelcontextprotocol/go-sdk` and is used by the gateways of Pages, Menu, Plans, Contacts, Goods, Issues and Atrace.

- Streamable HTTP, stateless handler with a capped request body.
- Authentication by access key (`Authorization: Bearer lpt_...`) resolved through a `Resolver` interface (in production the keys live in Hub and gateways resolve them there).
- A brake on guessing keys per client address, a `401` carrying `resource_metadata` so a client can start its own sign-in, and one log line per tool call (who, which tool, outcome; arguments are never logged).
- `Principal`: namespace, user, optional site binding, scopes, and an optional `Role` (the key creator's role in the product).
- `AddTool(srv, principal, class, tool, handler)`: tools are registered behind a permission class, `Read`, `Write` or `Act` (scope `publish`), checked before the handler runs, so a tool cannot forget its check.
- gRPC errors are converted into plain messages a model can act on.

The optional `mcpkit/oauth` package adds the sign-in server for AI clients: dynamic client registration, authorize (redirects to your consent page), token endpoint with PKCE, sealed one-time authorization codes (spent by the first exchange attempt, right or wrong), tokens that state `expires_in` (90 days). It needs only a `KeyIssuer` from you.

```go
cfg := &mcpkit.Config{
    Name: "lota-menu", Instructions: "Manage a menu...",
    MetadataURL: "https://<host>/api-menu/.well-known/oauth-protected-resource", // omit without OAuth
    Resolver: mcpkit.ResolverFunc(func(ctx context.Context, secret string) (*mcpkit.Principal, error) {
        // return nil, nil for a bad key (401); return an error when identity is down (503)
    }),
    Register: func(srv *sdk.Server, p *mcpkit.Principal) {
        mcpkit.AddTool(srv, p, mcpkit.Read, &sdk.Tool{Name: "list_items", Description: "List items."}, listItems)
    },
}
mux.Handle("/mcp", cfg.Handler())
```

For OAuth: `oa := oauth.New(secret, publicURL, consentPageURL)`, mount `oa.Handler()` at `/.well-known/*` and `/oauth/*`, and in the consent page backend call `oa.Approve(ctx, issuer, req)` or `oa.Deny(...)`. Tests can build the server for one principal with `cfg.NewServer(p)` and call every tool over `sdk.NewInMemoryTransports()` to prove each tool checks its permission (see `mcpkit/mcpkit_test.go`).

## Secrets

`secrets` hashes PINs and short secrets with bcrypt (per-secret salt, tunable cost). Hashes from the old unsalted-MD5 scheme still verify (`IsLegacyHash`), so callers can re-hash on the first successful use. `GeneratePIN`, `ValidatePINFormat`, `Hash`, `Verify` are the entry points.

## Development

```bash
go build ./...
go vet ./...
go test ./...
```

Run `go mod tidy` before committing; the release job fails on a dirty diff. `go run ./cmd/example` starts a small demo server.

## Contributing

Keep changes additive and covered by tests; every service pins a tag and all of them build against whatever is tagged last. Do not commit secrets or environment-specific values.

## Author

pieceowater-dev
