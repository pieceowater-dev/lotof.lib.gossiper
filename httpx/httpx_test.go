package httpx

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func do(t *testing.T, app *fiber.App, method, target string, headers map[string]string) (int, fiber.Map, string) {
	t.Helper()
	req := httptest.NewRequest(method, target, nil)
	for k, v := range headers {
		if k == "Host" {
			req.Host = v
			continue
		}
		req.Header.Set(k, v)
	}
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, target, err)
	}
	body, _ := io.ReadAll(resp.Body)
	h := fiber.Map{}
	for _, k := range []string{"Access-Control-Allow-Origin", "Access-Control-Allow-Headers", "Access-Control-Allow-Credentials"} {
		if v := resp.Header.Get(k); v != "" {
			h[k] = v
		}
	}
	return resp.StatusCode, h, string(body)
}

func TestNormalizeAPIPathPrefix(t *testing.T) {
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error { NormalizeAPIPathPrefix(c); return c.Next() })
	app.All("/*", func(c *fiber.Ctx) error { return c.SendString(c.Path()) })

	for in, want := range map[string]string{
		"/api-menu/query": "/query",
		"/api-menu":       "/",
		"/query":          "/query",
		"/apimenu/query":  "/apimenu/query",
	} {
		if _, _, got := do(t, app, "GET", in, nil); got != want {
			t.Errorf("%s -> %q, want %q", in, got, want)
		}
	}
}

func corsApp() *fiber.App {
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error { CORS(c, "MenuAuthorization"); return c.Next() })
	app.Get("/x", func(c *fiber.Ctx) error { return c.SendString("ok") })
	return app
}

func TestCORS_SameHostOrigin(t *testing.T) {
	_, h, _ := do(t, corsApp(), "GET", "/x", map[string]string{"Host": "lota.tools", "Origin": "https://lota.tools"})
	if h["Access-Control-Allow-Origin"] != "https://lota.tools" {
		t.Fatalf("same-host origin must be allowed, got %v", h)
	}
	if !strings.Contains(h["Access-Control-Allow-Headers"].(string), "MenuAuthorization") {
		t.Fatalf("the gateway's own auth header must be allowed, got %v", h)
	}
	if h["Access-Control-Allow-Credentials"] != "true" {
		t.Fatalf("credentials must be allowed for same-host, got %v", h)
	}
}

func TestCORS_ForwardedHost(t *testing.T) {
	// Behind the ALB the pod sees its internal service name as Host.
	_, h, _ := do(t, corsApp(), "GET", "/x", map[string]string{
		"Host":             "lotof-menu-gtw.lota-main.svc.cluster.local",
		"X-Forwarded-Host": "lota.tools",
		"Origin":           "https://lota.tools",
	})
	if h["Access-Control-Allow-Origin"] != "https://lota.tools" {
		t.Fatalf("X-Forwarded-Host must be used as the public host, got %v", h)
	}
}

func TestCORS_CrossSiteOriginGetsNoHeaders(t *testing.T) {
	_, h, _ := do(t, corsApp(), "GET", "/x", map[string]string{"Host": "lota.tools", "Origin": "https://evil.example"})
	if len(h) != 0 {
		t.Fatalf("a cross-site origin must get no CORS headers, got %v", h)
	}
}

func TestIsSameHostOrigin_NoOrigin(t *testing.T) {
	app := fiber.New()
	app.Get("/x", func(c *fiber.Ctx) error {
		if !IsSameHostOrigin(c) {
			return c.SendStatus(fiber.StatusForbidden)
		}
		return c.SendStatus(fiber.StatusOK)
	})
	if code, _, _ := do(t, app, "GET", "/x", map[string]string{"Host": "lota.tools"}); code != 200 {
		t.Fatalf("a request without Origin must count as same-host, got %d", code)
	}
}

func TestCSRFGuard(t *testing.T) {
	app := fiber.New()
	app.Use(CSRFGuard)
	app.All("/x", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	cases := []struct {
		name    string
		method  string
		headers map[string]string
		want    int
	}{
		{"cross-site POST by fetch metadata", "POST", map[string]string{"Host": "lota.tools", "Sec-Fetch-Site": "cross-site"}, 403},
		{"cross-site POST by Origin", "POST", map[string]string{"Host": "lota.tools", "Origin": "https://evil.example"}, 403},
		{"same-origin POST", "POST", map[string]string{"Host": "lota.tools", "Sec-Fetch-Site": "same-origin"}, 200},
		{"non-browser POST", "POST", map[string]string{"Host": "lota.tools"}, 200},
		{"cross-site GET", "GET", map[string]string{"Host": "lota.tools", "Sec-Fetch-Site": "cross-site"}, 200},
	}
	for _, tc := range cases {
		if code, _, _ := do(t, app, tc.method, "/x", tc.headers); code != tc.want {
			t.Errorf("%s: got %d, want %d", tc.name, code, tc.want)
		}
	}
}

func TestHealthRoutes(t *testing.T) {
	app := fiber.New()
	RegisterHealthRoutes(app, "lotof.test.gtw")
	app.Get("/", HealthHandler("lotof.test.gtw"))
	for _, p := range []string{"/", "/health", "/health/live", "/health/ready"} {
		code, _, body := do(t, app, "GET", p, nil)
		if code != 200 || !strings.Contains(body, `"service":"lotof.test.gtw"`) {
			t.Errorf("GET %s -> %d %s", p, code, body)
		}
	}
}
