package mcpkit

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type in struct {
	N string `json:"n"`
}
type out struct {
	Got string `json:"got"`
}

func testConfig(res Resolver) *Config {
	return &Config{
		Name: "test", Resolver: res, MetadataURL: "https://x.test/.well-known/oauth-protected-resource",
		Register: func(srv *sdk.Server, p *Principal) {
			AddTool(srv, p, Read, &sdk.Tool{Name: "look", Description: "r"}, func(_ context.Context, _ *Principal, i in) (out, error) { return out{Got: i.N}, nil })
			AddTool(srv, p, Write, &sdk.Tool{Name: "edit", Description: "w"}, func(_ context.Context, _ *Principal, _ in) (out, error) { return out{}, nil })
			AddTool(srv, p, Act, &sdk.Tool{Name: "go_live", Description: "a"}, func(_ context.Context, _ *Principal, _ in) (out, error) {
				return out{}, status.Error(codes.FailedPrecondition, "plain message")
			})
		},
	}
}

func callTool(t *testing.T, c *Config, p *Principal, name string) (string, bool) {
	t.Helper()
	ct, st := sdk.NewInMemoryTransports()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	ss, err := c.NewServer(p).Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ss.Close()
	cs, err := sdk.NewClient(&sdk.Implementation{Name: "t", Version: "1"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	res, err := cs.CallTool(ctx, &sdk.CallToolParams{Name: name, Arguments: map[string]any{"n": "v"}})
	if err != nil {
		return err.Error(), true
	}
	msg := ""
	for _, c := range res.Content {
		if tc, ok := c.(*sdk.TextContent); ok {
			msg += tc.Text
		}
	}
	return msg, res.IsError
}

func TestClassesGateTools(t *testing.T) {
	c := testConfig(nil)
	for _, tc := range []struct {
		scopes map[string]bool
		tool   string
		allow  bool
	}{
		{map[string]bool{}, "look", false},
		{map[string]bool{"read": true}, "look", true},
		{map[string]bool{"read": true}, "edit", false},
		{map[string]bool{"write": true}, "edit", true},
		{map[string]bool{"write": true}, "go_live", false},
		{map[string]bool{"publish": true}, "go_live", true}, // allowed, then the gRPC error is shown plain
	} {
		msg, failed := callTool(t, c, &Principal{Scopes: tc.scopes}, tc.tool)
		denied := failed && strings.Contains(msg, "permission")
		if denied == tc.allow {
			t.Errorf("%v %s: allow=%v got %q", tc.scopes, tc.tool, tc.allow, msg)
		}
	}
	if msg, _ := callTool(t, c, &Principal{Scopes: map[string]bool{"publish": true}}, "go_live"); msg != "plain message" {
		t.Errorf("gRPC status must be shown as its message, got %q", msg)
	}
}

func TestAnnotationsFollowClass(t *testing.T) {
	if !Annotations(Read).ReadOnlyHint || *Annotations(Act).DestructiveHint != true || *Annotations(Write).DestructiveHint != false {
		t.Fatal("annotations do not match the classes")
	}
}

func TestSiteBinding(t *testing.T) {
	if _, err := (&Principal{}).Site(""); err == nil {
		t.Fatal("an unbound key needs an id")
	}
	if id, err := (&Principal{SiteID: "a"}).Site(""); err != nil || id != "a" {
		t.Fatal("a bound key defaults to its site")
	}
	if _, err := (&Principal{SiteID: "a"}).Site("b"); err == nil {
		t.Fatal("a bound key must refuse another site")
	}
}

func TestHandlerAuthentication(t *testing.T) {
	good := &Principal{TokenID: "t", Scopes: map[string]bool{"read": true}}
	unavailable := false
	c := testConfig(ResolverFunc(func(_ context.Context, secret string) (*Principal, error) {
		if unavailable {
			return nil, errors.New("down")
		}
		if secret == "lpt_ok" {
			return good, nil
		}
		return nil, nil
	}))
	c.Guard = NewGuard(3, time.Minute)
	h := c.Handler()
	do := func(auth string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/mcp", strings.NewReader(`{}`))
		req.RemoteAddr = "9.9.9.9:1"
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		h.ServeHTTP(rec, req)
		return rec
	}
	if rec := do(""); rec.Code != 401 || !strings.Contains(rec.Header().Get("WWW-Authenticate"), `resource_metadata="https://x.test/.well-known/oauth-protected-resource"`) {
		t.Fatalf("%d %q", rec.Code, rec.Header().Get("WWW-Authenticate"))
	}
	unavailable = true
	if rec := do("Bearer lpt_ok"); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("resolver down must be 503, got %d", rec.Code)
	}
	unavailable = false
	if do("Bearer lpt_ok").Code == 401 {
		t.Fatal("a valid key must pass")
	}
	for i := 0; i < 3; i++ {
		if do("Bearer lpt_bad").Code != 401 {
			t.Fatal("bad key must be 401")
		}
	}
	if rec := do("Bearer lpt_ok"); rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("a guessing address must be blocked, got %d", rec.Code)
	}
}

func TestGuard(t *testing.T) {
	g := NewGuard(5, 5*time.Minute)
	now := time.Now()
	for i := 0; i < 5; i++ {
		g.Fail("1.2.3.4", now)
	}
	if !g.Blocked("1.2.3.4", now) || g.Blocked("5.6.7.8", now) {
		t.Fatal("block must be per address")
	}
	if g.Blocked("1.2.3.4", now.Add(5*time.Minute+time.Second)) {
		t.Fatal("the block must expire")
	}
}

func TestClientAddrTrustsOnlyTheLastHop(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Forwarded-For", "6.6.6.6, 10.0.0.1")
	if ClientAddr(r) != "10.0.0.1" {
		t.Fatal(ClientAddr(r))
	}
}

func TestCatalogListsTheRegisteredTools(t *testing.T) {
	c := testConfig(ResolverFunc(func(context.Context, string) (*Principal, error) { return nil, nil }))
	cat, err := c.Catalog(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, tl := range cat.Tools {
		got[tl.Name] = tl.Class
	}
	if got["look"] != "read" || got["edit"] != "write" || got["go_live"] != "publish" {
		t.Fatalf("the catalog must list every tool with its class, got %v", got)
	}
	if cat.Tools[0].Class != "read" {
		t.Fatalf("read tools come first, got %v", cat.Tools)
	}
}

func TestCatalogHandlerIsPublicJSON(t *testing.T) {
	c := testConfig(ResolverFunc(func(context.Context, string) (*Principal, error) { return nil, nil }))
	rec := httptest.NewRecorder()
	c.CatalogHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/mcp/tools", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"name":"look"`) {
		t.Fatalf("expected the public catalog without a key, got %d %s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	c.CatalogHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/mcp/tools", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("only GET is served, got %d", rec.Code)
	}
}
