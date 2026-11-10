package httpx

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

func TestTokenBucketRefillsAndLimits(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	r := newRateLimiter(100, func() time.Time { return now })
	// 60/min -> capacity max(10, 15) = 15, one token per second
	for i := 0; i < 15; i++ {
		if ok, _ := r.Allow("k", 60); !ok {
			t.Fatalf("request %d inside the burst was refused", i)
		}
	}
	ok, wait := r.Allow("k", 60)
	if ok || wait <= 0 || wait > 2*time.Second {
		t.Fatalf("16th request must be refused with a short wait, got ok=%v wait=%v", ok, wait)
	}
	now = now.Add(3 * time.Second)
	for i := 0; i < 3; i++ {
		if ok, _ := r.Allow("k", 60); !ok {
			t.Fatalf("token %d should have refilled", i)
		}
	}
	if ok, _ := r.Allow("k", 60); ok {
		t.Fatal("only three tokens refilled")
	}
	if ok, _ := r.Allow("other", 60); !ok {
		t.Fatal("another key has its own bucket")
	}
}

func TestBucketEvictionKeepsMemoryBounded(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	r := newRateLimiter(50, func() time.Time { return now })
	for i := 0; i < 500; i++ {
		r.Allow(strings.Repeat("x", 1)+time.Duration(i).String(), 60)
	}
	if len(r.buckets) > 50 {
		t.Fatalf("buckets grew to %d past the cap", len(r.buckets))
	}
}

func TestIsGraphQLMutation(t *testing.T) {
	cases := map[string]bool{
		`{"query":"mutation { createOrder(a:1){id} }"}`:          true,
		`{"query":"  # note\n mutation Foo($a:Int){ x(a:$a) }"}`: true,
		`{"query":"\u006dutation { x }"}`:                        true, // JSON escape trick
		`{"query":"query { me { id } }"}`:                        false,
		`{"query":"query Q { x(a:\"mutation\") }"}`:              false, // inside a string
		`{"query":"{ x }"}`:                                      false,
		`[{"query":"query { a }"},{"query":"mutation { b }"}]`:   true,
		`{"query":"query { a } # mutation { b }"}`:               false, // comment
		`not json`: true,
		``:         false,
	}
	for body, want := range cases {
		if got := IsGraphQLMutation([]byte(body)); got != want {
			t.Errorf("IsGraphQLMutation(%q) = %v, want %v", body, got, want)
		}
	}
}

func rlApp(cfg RateLimitConfig) *fiber.App {
	app := fiber.New()
	app.Use(RateLimit(cfg))
	app.All("/*", func(c *fiber.Ctx) error { return c.SendString("ok") })
	return app
}

func rlDo(t *testing.T, app *fiber.App, method, path, body string, hdr map[string]string) int {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	return resp.StatusCode
}

func TestRateLimitMiddleware(t *testing.T) {
	cfg := RateLimitConfig{AnonymousPerMinute: 40, CredentialedPerMinute: 400, MutationsPerMinute: 12}
	app := rlApp(cfg)

	// health probes and media are never limited
	for i := 0; i < 200; i++ {
		if c := rlDo(t, app, "GET", "/health/ready", "", nil); c != 200 {
			t.Fatalf("health probe limited at %d", i)
		}
		if c := rlDo(t, app, "GET", "/media/a.png", "", nil); c != 200 {
			t.Fatalf("media limited at %d", i)
		}
	}

	// anonymous mutations: capacity max(10, 12/4) = 10, then 429 with Retry-After
	mut := `{"query":"mutation { x }"}`
	limited := 0
	for i := 0; i < 14; i++ {
		if rlDo(t, app, "POST", "/query", mut, nil) == 429 {
			limited++
		}
	}
	if limited < 3 {
		t.Fatalf("anonymous mutation burst was not limited (limited=%d)", limited)
	}
	// plain anonymous queries are held only to the general bucket (a fresh app: the burst above used some of it)
	q := `{"query":"query { me { id } }"}`
	fresh := rlApp(cfg)
	for i := 0; i < 8; i++ {
		if c := rlDo(t, fresh, "POST", "/query", q, nil); c != 200 {
			t.Fatalf("anonymous query %d refused: %d", i, c)
		}
	}
	// a request with credentials is not held to the anonymous mutation budget
	for i := 0; i < 30; i++ {
		if c := rlDo(t, app, "POST", "/query", mut, map[string]string{"Authorization": "Bearer x"}); c != 200 {
			t.Fatalf("credentialed mutation %d refused: %d", i, c)
		}
	}
}

func TestRateLimitGeneralBucketAndRetryAfter(t *testing.T) {
	app := rlApp(RateLimitConfig{AnonymousPerMinute: 40, MutationsPerMinute: 12})
	var last int
	var retry string
	for i := 0; i < 20; i++ {
		req := httptest.NewRequest("GET", "/something", nil)
		resp, _ := app.Test(req)
		last = resp.StatusCode
		retry = resp.Header.Get("Retry-After")
	}
	if last != 429 || retry == "" {
		t.Fatalf("a sustained anonymous flood must be cut with Retry-After, got %d %q", last, retry)
	}
}
