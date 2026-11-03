package httpx

import (
	"net"
	"net/http"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// trustedProxyPeer reports whether the direct peer is a proxy inside the
// network (private, loopback or link-local), the only kind whose
// X-Forwarded-For we believe.
func trustedProxyPeer(peer net.IP) bool {
	return peer != nil && (peer.IsPrivate() || peer.IsLoopback() || peer.IsLinkLocalUnicast())
}

// lastForwardedIP is the last X-Forwarded-For entry as an IP, nil when absent
// or unparsable.
func lastForwardedIP(xff string) net.IP {
	if xff == "" {
		return nil
	}
	parts := strings.Split(xff, ",")
	return net.ParseIP(strings.TrimSpace(parts[len(parts)-1]))
}

// ClientIP is the address of the visitor. The gateway sits behind an AWS load
// balancer, so c.IP() is the balancer's own address and would put every
// visitor into one rate-limit bucket. The balancer appends the address it saw
// as the LAST X-Forwarded-For entry; earlier entries are client-supplied and
// ignored. The header is only honoured when the direct peer is a private,
// loopback or link-local address (i.e. a proxy inside the VPC); otherwise the
// peer address is used.
//
// This deliberately does not use fiber's ProxyHeader: that would trust the
// FIRST entry, which the client controls.
func ClientIP(c *fiber.Ctx) string {
	peer := c.Context().RemoteIP()
	if trustedProxyPeer(peer) {
		if ip := lastForwardedIP(c.Get(fiber.HeaderXForwardedFor)); ip != nil {
			return ip.String()
		}
	}
	if peer != nil {
		return peer.String()
	}
	return c.IP()
}

// ClientIPFromRequest is ClientIP for a net/http request.
func ClientIPFromRequest(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	peer := net.ParseIP(host)
	if trustedProxyPeer(peer) {
		if ip := lastForwardedIP(r.Header.Get("X-Forwarded-For")); ip != nil {
			return ip.String()
		}
	}
	return host
}
