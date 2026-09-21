// Package httpx holds the HTTP edge pieces every LOTOF gateway needs in front
// of its Fiber routes: ingress path normalisation, same-host CORS, a CSRF
// guard for cookie-authenticated endpoints, and the health endpoints.
package httpx

import (
	"net/url"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// baseAllowHeaders are the request headers every gateway accepts from the
// browser; CORS appends the gateway's own auth header (e.g. MenuAuthorization).
var baseAllowHeaders = []string{"Content-Type", "Authorization", "Namespace", "DeviceId", "Fingerprint", "X-Requested-With"}

// NormalizeAPIPathPrefix strips a leading "/api-<service>" segment (e.g.
// "/api-menu/query" -> "/query") so routes registered as plain "/query"
// still match when the gateway sits behind an ALB ingress path-prefix rule
// that does not rewrite the path.
func NormalizeAPIPathPrefix(c *fiber.Ctx) {
	trimmedPath := strings.TrimPrefix(c.Path(), "/")
	prefix, remainder, hasRemainder := strings.Cut(trimmedPath, "/")
	if !strings.HasPrefix(prefix, "api-") {
		return
	}

	normalizedPath := "/"
	if hasRemainder {
		normalizedPath += remainder
	}
	c.Path(normalizedPath)
}

func hostnameOf(rawHost string) string {
	parsed, err := url.Parse("//" + rawHost)
	if err != nil {
		return ""
	}
	return parsed.Hostname()
}

// RequestHostname is the public-facing host of the request, honouring the
// ALB's X-Forwarded-Host (the pod's own Host header is the internal service name).
func RequestHostname(c *fiber.Ctx) string {
	if forwarded := strings.TrimSpace(strings.Split(c.Get("X-Forwarded-Host"), ",")[0]); forwarded != "" {
		return hostnameOf(forwarded)
	}
	return hostnameOf(c.Hostname())
}

func isSameHostOrigin(c *fiber.Ctx, origin string) bool {
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Hostname() == "" {
		return false
	}
	return parsed.Hostname() == RequestHostname(c)
}

// IsSameHostOrigin reports whether the request's Origin header (if any) is the
// same host it is being served from. Requests without an Origin (non-browser
// callers, same-origin GETs) count as same-host.
func IsSameHostOrigin(c *fiber.Ctx) bool {
	return isSameHostOrigin(c, c.Get("Origin"))
}

// CORS sets the CORS response headers for a same-host Origin. A cross-host
// Origin gets NO Access-Control-Allow-* headers, so the browser blocks the
// response -- reflecting an arbitrary Origin together with
// Allow-Credentials: true would let any website make credentialed calls to
// the API. extraAllowHeaders are appended to the standard request headers,
// typically the gateway's own auth header.
func CORS(c *fiber.Ctx, extraAllowHeaders ...string) {
	origin := c.Get("Origin")
	if origin != "" && !isSameHostOrigin(c, origin) {
		return
	}
	if origin != "" {
		c.Set("Access-Control-Allow-Origin", origin)
	}
	c.Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
	c.Set("Access-Control-Allow-Headers", strings.Join(append(append([]string{}, baseAllowHeaders...), extraAllowHeaders...), ", "))
	c.Set("Access-Control-Allow-Credentials", "true")
	c.Vary("Origin")
}

// CSRFGuard rejects state-changing requests that a browser sent from another
// site. Mount it on endpoints authenticated purely by cookies, where any page
// could otherwise POST with the victim's cookies attached. Non-browser callers
// (no Origin, no Sec-Fetch-Site) and genuine same-site requests pass through.
func CSRFGuard(c *fiber.Ctx) error {
	switch c.Method() {
	case fiber.MethodGet, fiber.MethodHead, fiber.MethodOptions:
		return c.Next()
	}

	if site := c.Get("Sec-Fetch-Site"); site != "" {
		if site == "cross-site" {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "cross-site request blocked"})
		}
		return c.Next()
	}

	if origin := c.Get("Origin"); origin != "" && !isSameHostOrigin(c, origin) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "cross-site request blocked"})
	}
	return c.Next()
}

// HealthHandler answers with a static "ok" for serviceName. Besides the
// /health* routes, gateways mount it on "/" because that is the load
// balancer's default health-check path: without it the ALB sees 404 there,
// marks every target unhealthy and falls back to routing blindly.
func HealthHandler(serviceName string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok", "service": serviceName})
	}
}

// RegisterHealthRoutes wires GET /health, /health/live and /health/ready --
// the endpoints the k8s probes and uptime checks use.
func RegisterHealthRoutes(app *fiber.App, serviceName string) {
	app.Get("/health", HealthHandler(serviceName))
	app.Get("/health/live", func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok", "service": serviceName, "check": "liveness"})
	})
	app.Get("/health/ready", func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok", "service": serviceName, "check": "readiness"})
	})
}
