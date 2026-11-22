package httpx

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
)

// IsLocalHostname reports a development host, where cookies cannot carry Secure over plain HTTP.
func IsLocalHostname(hostname string) bool {
	return hostname == "localhost" || hostname == "127.0.0.1"
}

// IsAuthCookieName reports a cookie that carries a platform token: the hub's "token", the patron and owner
// sessions, their refresh tokens and the app tokens ("<app>-token").
func IsAuthCookieName(name string) bool {
	switch name {
	case "token", "refresh_token", "patron_token", "patron_refresh_token", "owner_token", "owner_refresh_token":
		return true
	}
	return strings.HasSuffix(name, "-token")
}

// AuthCookie builds an access-token cookie with the hub's session settings: host-only (no Domain, so sites
// published on sibling subdomains never see it), Path=/, SameSite=Lax, Secure everywhere except localhost, always
// HttpOnly: script never reads a token (GW-12). maxAge is in seconds; a negative one deletes the cookie.
func AuthCookie(c *fiber.Ctx, name, value string, maxAge int) *fiber.Cookie {
	hc := newAuthHTTPCookie(RequestHostname(c), name, value, maxAge)
	return &fiber.Cookie{
		Name:     hc.Name,
		Value:    hc.Value,
		Path:     hc.Path,
		MaxAge:   hc.MaxAge,
		Expires:  hc.Expires,
		Secure:   hc.Secure,
		HTTPOnly: hc.HttpOnly,
		SameSite: fiber.CookieSameSiteLaxMode,
	}
}

// SetAuthCookie sets an access-token cookie on the response (see AuthCookie).
func SetAuthCookie(c *fiber.Ctx, name, value string, maxAge int) {
	c.Cookie(AuthCookie(c, name, value, maxAge))
}

// ClearAuthCookie deletes an access-token cookie set by SetAuthCookie.
func ClearAuthCookie(c *fiber.Ctx, name string) {
	SetAuthCookie(c, name, "", -1)
}

func newAuthHTTPCookie(hostname, name, value string, maxAge int) *http.Cookie {
	cookie := &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		Secure:   !IsLocalHostname(hostname),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
	if maxAge < 0 {
		cookie.Expires = time.Unix(0, 0)
	} else if maxAge > 0 {
		cookie.Expires = time.Now().Add(time.Duration(maxAge) * time.Second)
	}
	return cookie
}

// ErrCrossSiteCookie is returned when a token would come from a cookie on a request that another site may have
// triggered: the browser attaches cookies to those too, so they must not authenticate a state change.
var ErrCrossSiteCookie = errors.New("cross-site request blocked")

// CookieAuthAllowed reports whether a token read from a cookie may authenticate this request. Safe methods pass,
// exactly as with CSRFGuard. A state-changing request (and a websocket upgrade, which is a GET that opens a
// read-write channel) must come from this very host: Sec-Fetch-Site same-origin or none, and an Origin, if any, of
// the same host. This is stricter than CSRFGuard on purpose: "same-site" also covers <slug>.pages.lota.tools, where
// owner-written scripts run, and SameSite=Lax cookies do reach the API from there.
func CookieAuthAllowed(c *fiber.Ctx) bool {
	switch c.Method() {
	case fiber.MethodGet, fiber.MethodHead, fiber.MethodOptions:
		if !strings.EqualFold(c.Get(fiber.HeaderUpgrade), "websocket") {
			return true
		}
	}
	switch c.Get("Sec-Fetch-Site") {
	case "", "same-origin", "none":
	default:
		return false
	}
	return IsSameHostOrigin(c)
}

// BearerToken returns the request's token: the first of headers that is set ("Bearer x" or a bare value), else the
// cookie cookieName. fromCookie tells which. A cookie token on a request CookieAuthAllowed refuses yields
// ErrCrossSiteCookie and no token; a header token is never refused (a header is not attached by the browser on
// its own, so it is no CSRF vector). An empty cookieName disables the fallback.
func BearerToken(c *fiber.Ctx, cookieName string, headers ...string) (token string, fromCookie bool, err error) {
	for _, h := range headers {
		if raw := strings.TrimSpace(c.Get(h)); raw != "" {
			return stripBearer(raw), false, nil
		}
	}
	if cookieName == "" {
		return "", false, nil
	}
	raw := strings.TrimSpace(c.Cookies(cookieName))
	if raw == "" {
		return "", false, nil
	}
	if !CookieAuthAllowed(c) {
		return "", false, ErrCrossSiteCookie
	}
	return raw, true, nil
}

func stripBearer(raw string) string {
	if len(raw) > 7 && strings.EqualFold(raw[:7], "Bearer ") {
		return strings.TrimSpace(raw[7:])
	}
	return raw
}

// PromoteCookieToken copies the cookie cookieName into the request header header as "Bearer <token>" when the
// header is missing, so code further down that only knows the header (GraphQL directives reading the operation's
// headers, REST handlers) authenticates cookie-only browsers unchanged. A header that is set always wins. It
// reports whether it promoted; ErrCrossSiteCookie means a cookie was there but CookieAuthAllowed refused it (the
// header stays unset, so the request goes on as anonymous unless the caller rejects it).
func PromoteCookieToken(c *fiber.Ctx, header, cookieName string) (bool, error) {
	if strings.TrimSpace(c.Get(header)) != "" {
		return false, nil
	}
	token, fromCookie, err := BearerToken(c, cookieName)
	if err != nil || !fromCookie {
		return false, err
	}
	c.Request().Header.Set(header, "Bearer "+token)
	return true, nil
}

// CookieTokenPair maps a request header to the cookie that stands in for it.
type CookieTokenPair struct {
	Header string
	Cookie string
}

// PromoteCookieTokens runs PromoteCookieToken for every pair and answers 403 when a cookie was refused as
// cross-site. It returns false when it has written that answer and the handler must stop.
func PromoteCookieTokens(c *fiber.Ctx, pairs ...CookieTokenPair) bool {
	for _, p := range pairs {
		if _, err := PromoteCookieToken(c, p.Header, p.Cookie); err != nil {
			_ = c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": err.Error()})
			return false
		}
	}
	return true
}

type cookieTokensKey struct{}

// WithCookieTokens records the named cookies of the request in ctx for code that runs after the HTTP request is
// gone, such as the GraphQL websocket InitFunc (its ctx is the upgrade request's context, but it has no access to
// the request). Nothing is recorded when CookieAuthAllowed refuses the request. The values are copied.
func WithCookieTokens(ctx context.Context, c *fiber.Ctx, names ...string) context.Context {
	if !CookieAuthAllowed(c) {
		return ctx
	}
	tokens := map[string]string{}
	for _, n := range names {
		if v := strings.TrimSpace(c.Cookies(n)); v != "" {
			tokens[n] = strings.Clone(v)
		}
	}
	if len(tokens) == 0 {
		return ctx
	}
	return context.WithValue(ctx, cookieTokensKey{}, tokens)
}

// CookieToken is a cookie recorded by WithCookieTokens, "" when absent.
func CookieToken(ctx context.Context, name string) string {
	if ctx == nil {
		return ""
	}
	tokens, _ := ctx.Value(cookieTokensKey{}).(map[string]string)
	return tokens[name]
}

type responseCookies struct {
	mu         sync.Mutex
	w          http.ResponseWriter
	hostname   string
	headerAuth bool
}

type responseCookiesKey struct{}

// WithResponseCookies lets code deep in a net/http handler (a GraphQL resolver) set auth cookies on the response
// w of request r, through SetAuthCookieFromContext. Mount it where the gateway already wraps its context for the
// handler (inside the fasthttpadaptor func).
func WithResponseCookies(ctx context.Context, w http.ResponseWriter, r *http.Request) context.Context {
	rc := &responseCookies{w: w, hostname: httpRequestHostname(r)}
	if r != nil {
		rc.headerAuth = strings.TrimSpace(r.Header.Get("Authorization")) != "" || strings.TrimSpace(r.Header.Get("HubAuthorization")) != ""
	}
	return context.WithValue(ctx, responseCookiesKey{}, rc)
}

// BodyToken is what a response that minted token and set it as a cookie (SetAuthCookieFromContext reported set)
// carries in its body: the token for a caller that authenticated with a header (the web's server render, MCP,
// scripts), "" for a browser on its cookies, whose script must never see a token (GW-12). Without a cookie the body
// is the only way out, so the token stays.
func BodyToken(ctx context.Context, token string, set bool) string {
	if !set || ctx == nil {
		return token
	}
	if rc, ok := ctx.Value(responseCookiesKey{}).(*responseCookies); ok && rc != nil && !rc.headerAuth {
		return ""
	}
	return token
}

// SetAuthCookieFromContext sets an access-token cookie (the AuthCookie settings) on the response recorded by
// WithResponseCookies. It reports false when ctx has no response to write to (websocket, gRPC, tests).
func SetAuthCookieFromContext(ctx context.Context, name, value string, maxAge int) bool {
	if ctx == nil {
		return false
	}
	rc, ok := ctx.Value(responseCookiesKey{}).(*responseCookies)
	if !ok || rc == nil || rc.w == nil {
		return false
	}
	rc.mu.Lock()
	defer rc.mu.Unlock()
	http.SetCookie(rc.w, newAuthHTTPCookie(rc.hostname, name, value, maxAge))
	return true
}

func httpRequestHostname(r *http.Request) string {
	if r == nil {
		return ""
	}
	if forwarded := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Host"), ",")[0]); forwarded != "" {
		return hostnameOf(forwarded)
	}
	return hostnameOf(r.Host)
}

// TokenMaxAge is the cookie MaxAge (seconds) that matches a JWT's lifetime: its exp claim minus now. A token
// without a readable exp gets fallback; an expired one 0. The signature is not checked: the caller has just minted
// or verified the token, this only sizes the cookie.
func TokenMaxAge(token string, fallback int) int {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 3 {
		return fallback
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return fallback
	}
	var claims struct {
		Exp float64 `json:"exp"`
	}
	if json.Unmarshal(raw, &claims) != nil || claims.Exp <= 0 {
		return fallback
	}
	left := int(time.Until(time.Unix(int64(claims.Exp), 0)).Seconds())
	if left < 0 {
		return 0
	}
	return left
}
