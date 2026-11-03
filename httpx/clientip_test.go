package httpx

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/valyala/fasthttp"
)

func fiberCtx(t *testing.T, peer, xff string) (*fiber.Ctx, func()) {
	t.Helper()
	app := fiber.New()
	req := &fasthttp.Request{}
	req.Header.SetMethod("GET")
	if xff != "" {
		req.Header.Set("X-Forwarded-For", xff)
	}
	fctx := &fasthttp.RequestCtx{}
	fctx.Init(req, &net.TCPAddr{IP: net.ParseIP(peer), Port: 4000}, nil)
	c := app.AcquireCtx(fctx)
	return c, func() { app.ReleaseCtx(c) }
}

func TestClientIP(t *testing.T) {
	cases := []struct{ name, peer, xff, want string }{
		{"proxy appends the real address last", "10.0.5.7", "6.6.6.6, 203.0.113.9", "203.0.113.9"},
		{"loopback proxy", "127.0.0.1", "203.0.113.9", "203.0.113.9"},
		{"public peer cannot spoof", "198.51.100.4", "10.9.9.9", "198.51.100.4"},
		{"no header uses the peer", "10.0.5.7", "", "10.0.5.7"},
		{"garbage last entry falls back to the peer", "10.0.5.7", "1.2.3.4, not-an-ip", "10.0.5.7"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, release := fiberCtx(t, tc.peer, tc.xff)
			defer release()
			if got := ClientIP(c); got != tc.want {
				t.Fatalf("ClientIP = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestClientIPFromRequest(t *testing.T) {
	cases := []struct{ name, remote, xff, want string }{
		{"proxy", "10.1.2.3:55", "6.6.6.6, 203.0.113.9", "203.0.113.9"},
		{"public peer ignores header", "198.51.100.4:55", "10.0.0.1", "198.51.100.4"},
		{"no port", "10.1.2.3", "203.0.113.9", "203.0.113.9"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/", nil)
			r.RemoteAddr = tc.remote
			if tc.xff != "" {
				r.Header.Set("X-Forwarded-For", tc.xff)
			}
			if got := ClientIPFromRequest(r); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestWSCheckOrigin(t *testing.T) {
	mk := func(origin, host, fwd string) *http.Request {
		r := httptest.NewRequest("GET", "/query", nil)
		r.Host = host
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		if fwd != "" {
			r.Header.Set("X-Forwarded-Host", fwd)
		}
		return r
	}
	if !WSCheckOrigin(mk("", "api.lota.tools", "")) {
		t.Fatal("no Origin means a non-browser client")
	}
	if !WSCheckOrigin(mk("https://lota.tools", "lota.tools:8080", "")) {
		t.Fatal("same host must pass")
	}
	if !WSCheckOrigin(mk("https://lota.tools", "pod:8080", "lota.tools, other")) {
		t.Fatal("forwarded host must be honoured")
	}
	if WSCheckOrigin(mk("https://evil.example", "lota.tools", "")) {
		t.Fatal("cross-site origin must be refused")
	}
	if WSCheckOrigin(mk("::bad", "lota.tools", "")) {
		t.Fatal("unparsable origin must be refused")
	}
}

func TestMountHealth(t *testing.T) {
	app := fiber.New()
	MountHealth(app, "svc", true)
	for _, p := range []string{"/", "/health", "/health/live", "/health/ready"} {
		if code, _, _ := do(t, app, "GET", p, nil); code != 200 {
			t.Fatalf("%s: %d", p, code)
		}
	}
	app2 := fiber.New()
	MountHealth(app2, "svc", false)
	if code, _, _ := do(t, app2, "GET", "/", nil); code != 404 {
		t.Fatalf("root must stay free, got %d", code)
	}
}
