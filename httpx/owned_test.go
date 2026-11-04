package httpx

import (
	"net/http"
	"testing"

	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttpadaptor"
)

// The request fasthttpadaptor builds reads from fasthttp's buffer; once OwnRequest has run, a later request that
// reuses the buffer must not change what the handler (and gRPC's header table) already holds.
func TestOwnedSurvivesBufferReuse(t *testing.T) {
	const token = "Bearer eyJhbGciOiJIUzI1NiJ9.first-user-token-0000000000000000000000000000.signature"
	var held, heldPath, heldQuery string

	handler := fasthttpadaptor.NewFastHTTPHandler(Owned(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		held = r.Header.Get("MenuAuthorization")
		heldPath = r.URL.Path
		heldQuery = r.URL.RawQuery
	})))

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/query?op=first")
	ctx.Request.Header.SetMethod("POST")
	ctx.Request.Header.Set("MenuAuthorization", token)
	handler(ctx)

	// the connection serves another user: same buffer, different bytes
	ctx.Request.Reset()
	ctx.Request.SetRequestURI("/other?op=secnd")
	ctx.Request.Header.Set("MenuAuthorization", "Bearer SECOND-USER-TOKEN-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx")

	if held != token {
		t.Fatalf("header value changed after the buffer was reused: %q", held)
	}
	if heldPath != "/query" || heldQuery != "op=first" {
		t.Fatalf("URL changed after the buffer was reused: %q ? %q", heldPath, heldQuery)
	}
}

func TestOwnRequestKeepsEverythingTheHandlerSees(t *testing.T) {
	r, _ := http.NewRequest(http.MethodPost, "https://user:pw@example.com/a/b?x=1#frag", nil)
	r.Header.Add("X-Multi", "a")
	r.Header.Add("X-Multi", "b")
	r.RemoteAddr = "10.0.0.1:1234"
	OwnRequest(r)
	if got := r.Header.Values("X-Multi"); len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("multi-value header lost: %v", got)
	}
	if r.URL.Path != "/a/b" || r.URL.RawQuery != "x=1" || r.URL.Fragment != "frag" || r.URL.User.Username() != "user" || r.RemoteAddr != "10.0.0.1:1234" || r.Host != "example.com" {
		t.Fatalf("request changed: %+v / %+v", r, r.URL)
	}
}
