package oauth

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func pkce(v string) string { s := sha256.Sum256([]byte(v)); return enc.EncodeToString(s[:]) }

const verifier = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk" // 43 chars, the RFC 7636 example

func TestFlow(t *testing.T) {
	s := New("secret", "https://lota.tools/api-pages", "https://lota.tools/mcp/authorize")
	h := s.Handler()

	// discovery
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/.well-known/oauth-protected-resource", nil))
	if !strings.Contains(rec.Body.String(), `"authorization_servers":["https://lota.tools/api-pages"]`) {
		t.Fatal(rec.Body.String())
	}

	// behind the ingress the request still carries the gateway prefix
	pr := httptest.NewRecorder()
	h.ServeHTTP(pr, httptest.NewRequest("GET", "/api-pages/.well-known/oauth-protected-resource", nil))
	if pr.Code != 200 {
		t.Fatalf("with the gateway prefix: %d", pr.Code)
	}

	for _, p := range []string{"/api-pages/mcp/.well-known/oauth-protected-resource", "/mcp/.well-known/oauth-authorization-server", "/api-pages/mcp/.well-known/openid-configuration"} {
		r := httptest.NewRecorder()
		h.ServeHTTP(r, httptest.NewRequest("GET", p, nil))
		if r.Code != 200 {
			t.Fatalf("next to the MCP address, %s: %d", p, r.Code)
		}
	}

	// the root form with the resource path appended (what ChatGPT asks for)
	for _, p := range []string{"/.well-known/oauth-protected-resource/api-pages/mcp", "/.well-known/oauth-authorization-server/api-pages", "/.well-known/openid-configuration/api-pages"} {
		r := httptest.NewRecorder()
		h.ServeHTTP(r, httptest.NewRequest("GET", p, nil))
		if r.Code != 200 || !strings.Contains(r.Body.String(), "lota.tools/api-pages") {
			t.Fatalf("%s: %d %s", p, r.Code, r.Body)
		}
	}

	// registration refuses a hostile return address, accepts the real ones
	post := func(body string) *httptest.ResponseRecorder {
		r := httptest.NewRecorder()
		h.ServeHTTP(r, httptest.NewRequest("POST", "/oauth/register", strings.NewReader(body)))
		return r
	}
	if r := post(`{"client_name":"x","redirect_uris":["javascript:alert(1)"]}`); r.Code != 400 {
		t.Fatalf("javascript: redirect accepted: %d", r.Code)
	}
	if r := post(`{"client_name":"x","redirect_uris":["http://evil.example/cb"]}`); r.Code != 400 {
		t.Fatalf("plain http to a stranger accepted: %d", r.Code)
	}
	r := post(`{"client_name":"Claude","redirect_uris":["https://claude.ai/api/mcp/auth_callback","http://localhost:33418/callback"]}`)
	if r.Code != 201 {
		t.Fatalf("register: %d %s", r.Code, r.Body)
	}
	var reg struct {
		ClientID string `json:"client_id"`
	}
	_ = json.Unmarshal(r.Body.Bytes(), &reg)

	// authorize: a wrong return address is not redirected to, a good request goes to the consent page
	bad := httptest.NewRecorder()
	h.ServeHTTP(bad, httptest.NewRequest("GET", "/oauth/authorize?client_id="+reg.ClientID+"&redirect_uri=https://evil.example/&response_type=code&code_challenge=x&code_challenge_method=S256", nil))
	if bad.Code != 400 {
		t.Fatalf("unregistered redirect: %d", bad.Code)
	}
	redir := "https://claude.ai/api/mcp/auth_callback"
	q := url.Values{"client_id": {reg.ClientID}, "redirect_uri": {redir}, "response_type": {"code"}, "state": {"st1"}, "code_challenge": {pkce(verifier)}, "code_challenge_method": {"S256"}, "scope": {"read write"}}
	ok := httptest.NewRecorder()
	h.ServeHTTP(ok, httptest.NewRequest("GET", "/oauth/authorize?"+q.Encode(), nil))
	if ok.Code != http.StatusFound || !strings.HasPrefix(ok.Header().Get("Location"), "https://lota.tools/mcp/authorize?") {
		t.Fatalf("authorize: %d %s", ok.Code, ok.Header().Get("Location"))
	}

	// the consent page asks for a code; the client trades it
	code, err := s.Seal("lpt_secretkey", reg.ClientID, redir, pkce(verifier), "read write")
	if err != nil {
		t.Fatal(err)
	}
	token := func(code, v, ru string) *httptest.ResponseRecorder {
		f := url.Values{"grant_type": {"authorization_code"}, "code": {code}, "client_id": {reg.ClientID}, "redirect_uri": {ru}, "code_verifier": {v}}
		rq := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(f.Encode()))
		rq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, rq)
		return w
	}
	if w := token(code, "wrong-verifier-wrong-verifier-wrong-verifier-1", redir); w.Code != 400 {
		t.Fatalf("a wrong verifier must fail: %d", w.Code)
	}
	if w := token(code, verifier, "https://claude.ai/other"); w.Code != 400 {
		t.Fatalf("a different redirect must fail: %d", w.Code)
	}
	w := token(code, verifier, redir)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"access_token":"lpt_secretkey"`) {
		t.Fatalf("exchange: %d %s", w.Code, w.Body)
	}
	if w := token(code, verifier, redir); w.Code != 400 {
		t.Fatalf("a code works once: %d", w.Code)
	}
	if New("", "b", "c") != nil {
		t.Fatal("no secret, no flow")
	}
}

type fakeIssuer struct {
	scopes []string
	res    string
}

func (f *fakeIssuer) IssueKey(_ context.Context, name string, scopes []string, res string, exp time.Time) (string, error) {
	f.scopes, f.res = scopes, res
	if !strings.HasPrefix(name, "AI: ") || time.Until(exp) < 89*24*time.Hour {
		return "", errors.New("bad key request")
	}
	return "lpt_issued", nil
}

func TestApproveSealsTheIssuedKey(t *testing.T) {
	s := New("secret", "https://x.test/api", "https://x.test/consent")
	id, _, err := s.Register("Claude", []string{"https://claude.ai/cb"})
	if err != nil {
		t.Fatal(err)
	}
	iss := &fakeIssuer{}
	back, err := s.Approve(context.Background(), iss, ApproveRequest{ClientID: id, RedirectURI: "https://claude.ai/cb", State: "st", Challenge: pkce(verifier), Scopes: []string{"read"}, ResourceID: "site1"})
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(back)
	if u.Query().Get("state") != "st" || iss.res != "site1" {
		t.Fatal(back)
	}
	secret, scope, err := s.Exchange(u.Query().Get("code"), id, "https://claude.ai/cb", verifier)
	if err != nil || secret != "lpt_issued" || scope != "read" {
		t.Fatalf("%q %q %v", secret, scope, err)
	}
	if _, err := s.Approve(context.Background(), iss, ApproveRequest{ClientID: id, RedirectURI: "https://evil.test/cb", Challenge: "x"}); err == nil {
		t.Fatal("an unregistered return address must be refused")
	}
	d, err := s.Deny(id, "https://claude.ai/cb", "st")
	if err != nil || !strings.Contains(d, "access_denied") {
		t.Fatal(d, err)
	}
}
