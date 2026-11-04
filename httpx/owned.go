package httpx

import (
	"net/http"
	"net/url"
	"strings"
)

// OwnRequest gives a request built by fasthttpadaptor strings of its own.
//
// fasthttpadaptor fills the net/http request with strings that point straight into fasthttp's request buffer (it
// converts the bytes without copying), and fasthttp reuses that buffer for the next request on the connection. A
// gateway forwards headers such as an Authorization token to gRPC as metadata, and gRPC's HTTP/2 encoder keeps the
// value in its dynamic header table and uses it as a map key, long after the request is finished. Once the buffer is
// overwritten the key changes under the table: the encoder panics ("id <= evictCount") inside gRPC's transport
// goroutine, which no recover can catch and which takes the whole gateway down. With a reused buffer the stored value
// can also become another user's token.
//
// Cloning every string the handler can see removes the aliasing for the price of a few small copies per request.
// Call it first thing in any handler that sits behind fasthttpadaptor (or wrap the handler with Owned).
func OwnRequest(r *http.Request) {
	headers := make(http.Header, len(r.Header))
	for name, values := range r.Header {
		owned := make([]string, len(values))
		for i, v := range values {
			owned[i] = strings.Clone(v)
		}
		headers[strings.Clone(name)] = owned
	}
	r.Header = headers

	r.Method = strings.Clone(r.Method)
	r.Host = strings.Clone(r.Host)
	r.RequestURI = strings.Clone(r.RequestURI)
	r.Proto = strings.Clone(r.Proto)
	r.RemoteAddr = strings.Clone(r.RemoteAddr)
	if r.URL != nil {
		u := *r.URL
		u.Scheme = strings.Clone(u.Scheme)
		u.Opaque = strings.Clone(u.Opaque)
		u.Host = strings.Clone(u.Host)
		u.Path = strings.Clone(u.Path)
		u.RawPath = strings.Clone(u.RawPath)
		u.RawQuery = strings.Clone(u.RawQuery)
		u.Fragment = strings.Clone(u.Fragment)
		u.RawFragment = strings.Clone(u.RawFragment)
		if u.User != nil {
			password, _ := u.User.Password()
			u.User = url.UserPassword(strings.Clone(u.User.Username()), strings.Clone(password))
		}
		r.URL = &u
	}
}

// Owned wraps h so that it always sees a request with strings of its own (see OwnRequest):
// fasthttpadaptor.NewFastHTTPHandler(httpx.Owned(handler)).
func Owned(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		OwnRequest(r)
		h.ServeHTTP(w, r)
	})
}
