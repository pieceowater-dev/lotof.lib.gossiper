package httpx

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
)

// RateLimitConfig tunes RateLimit. The zero value is usable: every field has a default.
//
// The bucket is always chosen by the client address (ClientIP), never by a client-supplied header, so changing a
// header does not yield a fresh bucket. A request that carries credentials (an Authorization header or one of
// CredentialHeaders) only gets the larger CredentialedPerMinute budget: signed-in people share an office address and
// poll more. The credential is NOT verified here, so a junk header buys at most that bounded multiple; the strict
// limit that matters for abuse (anonymous GraphQL mutations) is the one a bot without any header runs into.
type RateLimitConfig struct {
	// AnonymousPerMinute is the budget of a visitor without credentials (default 300).
	AnonymousPerMinute int
	// CredentialedPerMinute is the budget of a request with credentials (default 3000).
	CredentialedPerMinute int
	// MutationsPerMinute is the stricter budget of anonymous GraphQL mutations on GraphQLPaths (default 20).
	MutationsPerMinute int
	// GraphQLPaths are the endpoints whose body is inspected for a mutation (default /query and /graphql).
	GraphQLPaths []string
	// SkipPrefixes are never limited (default: health probes, /media/, /.well-known/).
	SkipPrefixes []string
	// CredentialHeaders besides Authorization that mark a request as carrying credentials.
	CredentialHeaders []string
	// MaxKeys bounds memory: past it the oldest idle buckets are dropped first (default 100000).
	MaxKeys int
	Logger  *slog.Logger

	now func() time.Time // tests
}

func (c *RateLimitConfig) defaults() {
	if c.AnonymousPerMinute <= 0 {
		c.AnonymousPerMinute = 300
	}
	if c.CredentialedPerMinute <= 0 {
		c.CredentialedPerMinute = 3000
	}
	if c.MutationsPerMinute <= 0 {
		c.MutationsPerMinute = 20
	}
	if len(c.GraphQLPaths) == 0 {
		c.GraphQLPaths = []string{"/query", "/graphql"}
	}
	if c.SkipPrefixes == nil {
		c.SkipPrefixes = []string{"/health", "/media/", "/.well-known/"}
	}
	if c.MaxKeys <= 0 {
		c.MaxKeys = 100000
	}
	if c.Logger == nil {
		c.Logger = slog.Default()
	}
	if c.now == nil {
		c.now = time.Now
	}
}

type tokenBucket struct {
	tokens float64
	last   time.Time
}

// RateLimiter is a set of token buckets keyed by string. Capacity is a quarter of the per-minute budget (at least 10),
// so a page that fires a handful of requests at once is fine while a sustained flood is cut at the per-minute rate.
type RateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*tokenBucket
	maxKeys int
	now     func() time.Time
}

func newRateLimiter(maxKeys int, now func() time.Time) *RateLimiter {
	return &RateLimiter{buckets: make(map[string]*tokenBucket), maxKeys: maxKeys, now: now}
}

func capacityFor(perMinute int) float64 {
	return math.Max(10, float64(perMinute)/4)
}

// Allow takes one token from key's bucket (refilled at perMinute per minute). When it is empty the second return
// value is how long until the next token.
func (r *RateLimiter) Allow(key string, perMinute int) (bool, time.Duration) {
	capacity := capacityFor(perMinute)
	rate := float64(perMinute) / 60.0
	now := r.now()

	r.mu.Lock()
	defer r.mu.Unlock()
	b, ok := r.buckets[key]
	if !ok {
		if len(r.buckets) >= r.maxKeys {
			r.evictLocked(now)
		}
		b = &tokenBucket{tokens: capacity, last: now}
		r.buckets[key] = b
	} else {
		b.tokens = math.Min(capacity, b.tokens+now.Sub(b.last).Seconds()*rate)
		b.last = now
	}
	if b.tokens >= 1 {
		b.tokens--
		return true, 0
	}
	wait := time.Duration((1 - b.tokens) / rate * float64(time.Second))
	return false, wait
}

// evictLocked drops idle buckets (untouched for two minutes); when that frees nothing it drops an arbitrary tenth.
func (r *RateLimiter) evictLocked(now time.Time) {
	for k, b := range r.buckets {
		if now.Sub(b.last) > 2*time.Minute {
			delete(r.buckets, k)
		}
	}
	if len(r.buckets) < r.maxKeys {
		return
	}
	n := r.maxKeys / 10
	for k := range r.buckets {
		delete(r.buckets, k)
		if n--; n <= 0 {
			break
		}
	}
}

// IsGraphQLMutation reports whether a GraphQL request body (single operation or a batch) contains a mutation. The
// query string is decoded as JSON first, so escape tricks (mutation) do not hide it; comments and the optional
// operation name are skipped. A body that cannot be parsed counts as a mutation (the strict bucket is the safe side).
func IsGraphQLMutation(body []byte) bool {
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return false
	}
	var ops []struct {
		Query string `json:"query"`
	}
	if body[0] == '[' {
		if json.Unmarshal(body, &ops) != nil {
			return true
		}
	} else {
		var one struct {
			Query string `json:"query"`
		}
		if json.Unmarshal(body, &one) != nil {
			return true
		}
		ops = append(ops, one)
	}
	for _, op := range ops {
		if queryHasMutation(op.Query) {
			return true
		}
	}
	return false
}

// queryHasMutation scans the document for a top-level `mutation` keyword (outside strings, comments and braces).
func queryHasMutation(q string) bool {
	depth := 0
	for i := 0; i < len(q); {
		ch := q[i]
		switch {
		case ch == '#':
			for i < len(q) && q[i] != '\n' {
				i++
			}
		case ch == '"':
			if strings.HasPrefix(q[i:], `"""`) {
				end := strings.Index(q[i+3:], `"""`)
				if end < 0 {
					return false
				}
				i += 3 + end + 3
				continue
			}
			i++
			for i < len(q) && q[i] != '"' {
				if q[i] == '\\' {
					i++
				}
				i++
			}
			i++
		case ch == '{' || ch == '(' || ch == '[':
			depth++
			i++
		case ch == '}' || ch == ')' || ch == ']':
			depth--
			i++
		case depth == 0 && isNameStart(ch):
			j := i
			for j < len(q) && isNameChar(q[j]) {
				j++
			}
			if q[i:j] == "mutation" {
				return true
			}
			i = j
		default:
			i++
		}
	}
	return false
}

func isNameStart(c byte) bool { return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') }
func isNameChar(c byte) bool  { return isNameStart(c) || (c >= '0' && c <= '9') }

// RateLimit is the gateways' shared request limiter (fiber middleware): per-client-IP token buckets, a larger budget
// for requests carrying credentials, a strict one for anonymous GraphQL mutations, and no limit at all on health
// probes, media and well-known paths. Answers 429 with Retry-After when a bucket is empty.
func RateLimit(cfg RateLimitConfig) fiber.Handler {
	cfg.defaults()
	general := newRateLimiter(cfg.MaxKeys, cfg.now)
	mutations := newRateLimiter(cfg.MaxKeys, cfg.now)
	var logMu sync.Mutex
	var lastLog time.Time

	deny := func(c *fiber.Ctx, scope string, wait time.Duration) error {
		logMu.Lock()
		if cfg.now().Sub(lastLog) > time.Second {
			lastLog = cfg.now()
			cfg.Logger.Warn("rate limit exceeded", slog.String("scope", scope), slog.String("ip", ClientIP(c)), slog.String("path", c.Path()))
		}
		logMu.Unlock()
		secs := int(math.Ceil(wait.Seconds()))
		if secs < 1 {
			secs = 1
		}
		c.Set(fiber.HeaderRetryAfter, strconv.Itoa(secs))
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "too many requests, please try again later"})
	}

	return func(c *fiber.Ctx) error {
		if c.Method() == fiber.MethodOptions {
			return c.Next()
		}
		path := c.Path()
		if path == "/" {
			return c.Next()
		}
		for _, p := range cfg.SkipPrefixes {
			if strings.HasPrefix(path, p) {
				return c.Next()
			}
		}
		ip := ClientIP(c)
		creds := hasCredentials(c, cfg.CredentialHeaders)
		// Separate buckets: a bot hammering anonymously must not use up the budget of the signed-in people behind the same address.
		perMinute, key := cfg.AnonymousPerMinute, "anon:"+ip
		if creds {
			perMinute, key = cfg.CredentialedPerMinute, "cred:"+ip
		}
		if ok, wait := general.Allow(key, perMinute); !ok {
			return deny(c, "requests", wait)
		}
		if !creds && c.Method() == fiber.MethodPost && isGraphQLPath(path, cfg.GraphQLPaths) && IsGraphQLMutation(c.Body()) {
			if ok, wait := mutations.Allow("ip:"+ip, cfg.MutationsPerMinute); !ok {
				return deny(c, "anonymous-mutations", wait)
			}
		}
		return c.Next()
	}
}

func isGraphQLPath(path string, paths []string) bool {
	for _, p := range paths {
		if path == p {
			return true
		}
	}
	return false
}

func hasCredentials(c *fiber.Ctx, extra []string) bool {
	if strings.TrimSpace(c.Get(fiber.HeaderAuthorization)) != "" {
		return true
	}
	for _, h := range extra {
		if strings.TrimSpace(c.Get(h)) != "" {
			return true
		}
	}
	return false
}
