package httpx

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func tokenApp(t *testing.T) *fiber.App {
	t.Helper()
	app := fiber.New()
	h := func(c *fiber.Ctx) error {
		tok, fromCookie, err := BearerToken(c, "menu-token", "MenuAuthorization", "Authorization")
		if err != nil {
			return c.Status(fiber.StatusForbidden).SendString(err.Error())
		}
		src := "header"
		if fromCookie {
			src = "cookie"
		}
		return c.SendString(src + ":" + tok)
	}
	app.Get("/t", h)
	app.Post("/t", h)
	return app
}

func call(t *testing.T, app *fiber.App, method string, headers map[string]string) (int, string) {
	t.Helper()
	req := httptest.NewRequest(method, "http://lota.tools/t", nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func TestBearerToken(t *testing.T) {
	app := tokenApp(t)
	cases := []struct {
		name    string
		method  string
		headers map[string]string
		code    int
		body    string
	}{
		{"header wins over cookie", "POST", map[string]string{"MenuAuthorization": "Bearer h1", "Cookie": "menu-token=c1"}, 200, "header:h1"},
		{"second header", "POST", map[string]string{"Authorization": "Bearer h2"}, 200, "header:h2"},
		{"bare header value", "POST", map[string]string{"Authorization": "raw"}, 200, "header:raw"},
		{"cookie fallback", "POST", map[string]string{"Cookie": "menu-token=c1"}, 200, "cookie:c1"},
		{"cookie same-origin", "POST", map[string]string{"Cookie": "menu-token=c1", "Sec-Fetch-Site": "same-origin", "Origin": "https://lota.tools"}, 200, "cookie:c1"},
		{"cookie cross-site", "POST", map[string]string{"Cookie": "menu-token=c1", "Sec-Fetch-Site": "cross-site"}, 403, ErrCrossSiteCookie.Error()},
		{"cookie same-site subdomain", "POST", map[string]string{"Cookie": "menu-token=c1", "Sec-Fetch-Site": "same-site"}, 403, ErrCrossSiteCookie.Error()},
		{"cookie foreign origin", "POST", map[string]string{"Cookie": "menu-token=c1", "Origin": "https://x.pages.lota.tools"}, 403, ErrCrossSiteCookie.Error()},
		{"cross-site header passes", "POST", map[string]string{"MenuAuthorization": "Bearer h1", "Sec-Fetch-Site": "cross-site"}, 200, "header:h1"},
		{"cookie cross-site GET", "GET", map[string]string{"Cookie": "menu-token=c1", "Sec-Fetch-Site": "cross-site"}, 200, "cookie:c1"},
		{"websocket upgrade cross-site", "GET", map[string]string{"Cookie": "menu-token=c1", "Upgrade": "websocket", "Sec-Fetch-Site": "cross-site"}, 403, ErrCrossSiteCookie.Error()},
		{"nothing", "POST", nil, 200, "header:"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, body := call(t, app, tc.method, tc.headers)
			if code != tc.code || body != tc.body {
				t.Fatalf("got %d %q, want %d %q", code, body, tc.code, tc.body)
			}
		})
	}
}

func TestPromoteCookieTokens(t *testing.T) {
	app := fiber.New()
	app.Post("/t", func(c *fiber.Ctx) error {
		if !PromoteCookieTokens(c, CookieTokenPair{"MenuAuthorization", "menu-token"}, CookieTokenPair{"Authorization", "token"}) {
			return nil
		}
		return c.SendString(c.Get("MenuAuthorization") + "|" + c.Get("Authorization"))
	})
	if code, body := call(t, app, "POST", map[string]string{"Cookie": "menu-token=m; token=h"}); code != 200 || body != "Bearer m|Bearer h" {
		t.Fatalf("promote: %d %q", code, body)
	}
	if code, body := call(t, app, "POST", map[string]string{"Cookie": "menu-token=m; token=h", "Authorization": "Bearer x"}); code != 200 || body != "Bearer m|Bearer x" {
		t.Fatalf("header wins: %d %q", code, body)
	}
	if code, _ := call(t, app, "POST", map[string]string{"Cookie": "token=h", "Sec-Fetch-Site": "cross-site"}); code != 403 {
		t.Fatalf("cross-site: %d", code)
	}
	if code, body := call(t, app, "POST", nil); code != 200 || body != "|" {
		t.Fatalf("none: %d %q", code, body)
	}
}

func setCookieHeader(t *testing.T, app *fiber.App, host string) string {
	t.Helper()
	req := httptest.NewRequest("GET", "http://"+host+"/c", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Join(resp.Header.Values("Set-Cookie"), "\n")
}

func TestSetAuthCookie(t *testing.T) {
	app := fiber.New()
	app.Get("/c", func(c *fiber.Ctx) error {
		SetAuthCookie(c, "token", "abc", 900)
		return nil
	})

	t.Setenv(AuthCookieHTTPOnlyEnv, "")
	sc := setCookieHeader(t, app, "lota.tools")
	for _, want := range []string{"token=abc", "max-age=900", "path=/", "secure", "SameSite=Lax"} {
		if !strings.Contains(strings.ToLower(sc), strings.ToLower(want)) {
			t.Fatalf("missing %q in %q", want, sc)
		}
	}
	if strings.Contains(strings.ToLower(sc), "httponly") || strings.Contains(strings.ToLower(sc), "domain=") {
		t.Fatalf("flag off must be readable and host-only: %q", sc)
	}

	t.Setenv(AuthCookieHTTPOnlyEnv, "true")
	if sc := setCookieHeader(t, app, "lota.tools"); !strings.Contains(strings.ToLower(sc), "httponly") {
		t.Fatalf("flag on must be HttpOnly: %q", sc)
	}
	if sc := setCookieHeader(t, app, "localhost:8080"); strings.Contains(strings.ToLower(sc), "secure") {
		t.Fatalf("localhost must not be Secure: %q", sc)
	}
}

func TestClearAuthCookie(t *testing.T) {
	app := fiber.New()
	app.Get("/c", func(c *fiber.Ctx) error {
		ClearAuthCookie(c, "atrace-token")
		return nil
	})
	sc := strings.ToLower(setCookieHeader(t, app, "lota.tools"))
	if !strings.Contains(sc, "atrace-token=;") || !strings.Contains(sc, "expires=") {
		t.Fatalf("clear: %q", sc)
	}
}

func TestSetAuthCookieFromContext(t *testing.T) {
	if SetAuthCookieFromContext(context.Background(), "x", "y", 1) {
		t.Fatal("no response recorded, must report false")
	}
	t.Setenv(AuthCookieHTTPOnlyEnv, "true")
	r := httptest.NewRequest("POST", "http://pod:8080/query", nil)
	r.Header.Set("X-Forwarded-Host", "lota.tools")
	w := httptest.NewRecorder()
	ctx := WithResponseCookies(r.Context(), w, r)
	if !SetAuthCookieFromContext(ctx, "menu-token", "tok", 3600) {
		t.Fatal("must set")
	}
	c := (&http.Response{Header: w.Header()}).Cookies()
	if len(c) != 1 || c[0].Name != "menu-token" || c[0].Value != "tok" || c[0].MaxAge != 3600 || !c[0].HttpOnly || !c[0].Secure || c[0].Domain != "" || c[0].SameSite != http.SameSiteLaxMode || c[0].Path != "/" {
		t.Fatalf("cookie: %+v", c)
	}
}

func TestWithCookieTokens(t *testing.T) {
	app := fiber.New()
	app.Get("/t", func(c *fiber.Ctx) error {
		ctx := WithCookieTokens(context.Background(), c, "menu-token", "token")
		return c.SendString(CookieToken(ctx, "menu-token") + "|" + CookieToken(ctx, "token"))
	})
	if _, body := call(t, app, "GET", map[string]string{"Cookie": "menu-token=m", "Upgrade": "websocket"}); body != "m|" {
		t.Fatalf("same-origin: %q", body)
	}
	if _, body := call(t, app, "GET", map[string]string{"Cookie": "menu-token=m", "Upgrade": "websocket", "Origin": "https://evil.example"}); body != "|" {
		t.Fatalf("foreign origin: %q", body)
	}
	if CookieToken(context.Background(), "x") != "" {
		t.Fatal("empty ctx")
	}
}

func TestRateLimitCountsAuthCookie(t *testing.T) {
	app := fiber.New()
	app.Get("/t", func(c *fiber.Ctx) error {
		if hasCredentials(c, nil) {
			return c.SendString("cred")
		}
		return c.SendString("anon")
	})
	if _, body := call(t, app, "GET", map[string]string{"Cookie": "pages-token=x"}); body != "cred" {
		t.Fatalf("app cookie: %q", body)
	}
	if _, body := call(t, app, "GET", map[string]string{"Cookie": "theme=dark"}); body != "anon" {
		t.Fatalf("other cookie: %q", body)
	}
}
