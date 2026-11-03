package httpx

import (
	"net/http"
	"net/url"
	"strings"
)

// WSCheckOrigin is the gorilla websocket Upgrader.CheckOrigin for GraphQL
// subscriptions: the same same-origin policy the REST middleware enforces for
// ordinary requests, honouring X-Forwarded-Host behind a proxy. A request
// without an Origin (non-browser client) is allowed.
func WSCheckOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	parsedOrigin, err := url.Parse(origin)
	if err != nil || parsedOrigin.Hostname() == "" {
		return false
	}
	hostToCompare := r.Host
	if forwarded := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Host"), ",")[0]); forwarded != "" {
		hostToCompare = forwarded
	}
	parsedHost, err := url.Parse("//" + hostToCompare)
	if err != nil || parsedHost.Hostname() == "" {
		return false
	}
	return parsedOrigin.Hostname() == parsedHost.Hostname()
}
