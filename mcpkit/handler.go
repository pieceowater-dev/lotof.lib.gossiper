package mcpkit

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Resolver turns the secret of an access key into the principal it belongs to.
//
// Return (nil, nil) for a key that does not exist, is revoked or expired: the caller counts it as a failed
// sign-in. Return an error only when the answer cannot be given (the identity service is down): the client gets
// 503 and the failure is not held against its address.
type Resolver interface {
	Resolve(ctx context.Context, secret string) (*Principal, error)
}

// ResolverFunc adapts a function to Resolver.
type ResolverFunc func(ctx context.Context, secret string) (*Principal, error)

func (f ResolverFunc) Resolve(ctx context.Context, secret string) (*Principal, error) {
	return f(ctx, secret)
}

// Config describes one MCP endpoint.
type Config struct {
	Name         string // server name shown to the client, e.g. "lota-pages"
	Version      string // default "1.0.0"
	Instructions string // usage notes the client shows to the model
	Resolver     Resolver

	// Register adds the tools for one authenticated request. Use AddTool inside it.
	Register func(srv *sdk.Server, p *Principal)

	// MetadataURL, when set, is advertised as resource_metadata on every 401 so an AI client can start its
	// own sign-in flow (the address of /.well-known/oauth-protected-resource).
	MetadataURL string
	// Realm is the WWW-Authenticate realm, default Name.
	Realm string
	// Guard is the brake on guessing keys; default 30 failures per address per 5 minutes.
	Guard *Guard
	// MaxBody caps a request body, default 2 MiB.
	MaxBody int64
	// Logger receives one line per tool call, default slog.Default().
	Logger *slog.Logger
}

func (c *Config) defaults() {
	if c.Version == "" {
		c.Version = "1.0.0"
	}
	if c.Realm == "" {
		c.Realm = c.Name
	}
	if c.Guard == nil {
		c.Guard = NewGuard(30, 5*time.Minute)
	}
	if c.MaxBody <= 0 {
		c.MaxBody = 2 << 20
	}
	if c.Logger == nil {
		c.Logger = slog.Default()
	}
}

// NewServer builds the sdk server for one principal: the tools from Register plus the call log. A nil
// principal gets a server without tools. The handler uses it for every request; tests may call it directly.
func (c *Config) NewServer(p *Principal) *sdk.Server {
	c.defaults()
	srv := sdk.NewServer(&sdk.Implementation{Name: c.Name, Version: c.Version}, &sdk.ServerOptions{Instructions: c.Instructions})
	if p == nil {
		return srv
	}
	logCalls(srv, c.Logger, p)
	if c.Register != nil {
		c.Register(srv, p)
	}
	return srv
}

type principalKey struct{}

// Handler returns the http.Handler to mount at the MCP address.
func (c *Config) Handler() http.Handler {
	c.defaults()
	inner := sdk.NewStreamableHTTPHandler(func(r *http.Request) *sdk.Server {
		p, _ := r.Context().Value(principalKey{}).(*Principal)
		return c.NewServer(p)
	}, &sdk.StreamableHTTPOptions{Stateless: true})

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, c.MaxBody)
		secret := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
		addr := ClientAddr(r)
		now := time.Now()
		if c.Guard.Blocked(addr, now) {
			TooMany(w, c.Guard.Window())
			return
		}
		if secret == "" {
			Unauthorized(w, c.Realm, c.MetadataURL)
			return
		}
		p, err := c.Resolver.Resolve(r.Context(), secret)
		if err != nil {
			http.Error(w, "authentication is unavailable", http.StatusServiceUnavailable)
			return
		}
		if p == nil {
			c.Guard.Fail(addr, now)
			Unauthorized(w, c.Realm, c.MetadataURL)
			return
		}
		if p.Scopes == nil {
			p.Scopes = map[string]bool{}
		}
		inner.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), principalKey{}, p)))
	})
}

// Unauthorized writes the 401 with a WWW-Authenticate header pointing at the sign-in metadata.
func Unauthorized(w http.ResponseWriter, realm, metadataURL string) {
	h := `Bearer realm="` + realm + `"`
	if metadataURL != "" {
		h += `, resource_metadata="` + metadataURL + `"`
	}
	w.Header().Set("WWW-Authenticate", h)
	http.Error(w, "unauthorized: missing or invalid access token", http.StatusUnauthorized)
}

// logCalls writes one log line per tool call: who, which tool, outcome, duration; never the arguments.
func logCalls(srv *sdk.Server, log *slog.Logger, p *Principal) {
	srv.AddReceivingMiddleware(func(next sdk.MethodHandler) sdk.MethodHandler {
		return func(ctx context.Context, method string, req sdk.Request) (sdk.Result, error) {
			if method != "tools/call" {
				return next(ctx, method, req)
			}
			name := ""
			if cr, ok := req.(*sdk.CallToolRequest); ok && cr.Params != nil {
				name = cr.Params.Name
			}
			start := time.Now()
			res, err := next(ctx, method, req)
			failed := err != nil
			if cr, ok := res.(*sdk.CallToolResult); ok && cr != nil && cr.IsError {
				failed = true
			}
			log.Info("mcp tool", slog.String("token", p.TokenID), slog.String("namespace", p.Namespace), slog.String("user", p.UserID),
				slog.String("tool", name), slog.Bool("failed", failed), slog.Int64("ms", time.Since(start).Milliseconds()))
			return res, err
		}
	})
}
