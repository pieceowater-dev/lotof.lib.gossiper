package oauth

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
)

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func oauthErr(w http.ResponseWriter, code int, e, desc string) {
	writeJSON(w, code, map[string]string{"error": e, "error_description": desc})
}

// Handler serves the discovery documents and the registration, authorization and token endpoints. Paths are
// relative to the gateway root (the ingress adds the /api-pages prefix).
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	meta := func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, 200, map[string]any{
			"issuer": s.base, "authorization_endpoint": s.base + "/oauth/authorize", "token_endpoint": s.base + "/oauth/token",
			"registration_endpoint": s.base + "/oauth/register", "response_types_supported": []string{"code"},
			"grant_types_supported": []string{"authorization_code"}, "code_challenge_methods_supported": []string{"S256"},
			"token_endpoint_auth_methods_supported": []string{"none"}, "scopes_supported": s.Scopes,
		})
	}
	// Clients look these up either under the gateway prefix (/api-pages/.well-known/x) or, per RFC 8414/9728, at the
	// root with the resource path appended (/.well-known/x/api-pages/mcp); the subtree patterns cover the second form.
	for _, p := range []string{"/.well-known/oauth-authorization-server", "/.well-known/openid-configuration"} {
		mux.HandleFunc(p, meta)
		mux.HandleFunc(p+"/", meta)
	}
	resource := func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, 200, map[string]any{
			"resource": s.base + s.MCPPath, "authorization_servers": []string{s.base},
			"bearer_methods_supported": []string{"header"}, "scopes_supported": s.Scopes,
			"resource_name": s.ResourceName,
		})
	}
	mux.HandleFunc("/.well-known/oauth-protected-resource", resource)
	mux.HandleFunc("/.well-known/oauth-protected-resource/", resource)
	mux.HandleFunc("/oauth/register", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			oauthErr(w, 405, "invalid_request", "POST only")
			return
		}
		var in struct {
			Name string   `json:"client_name"`
			URIs []string `json:"redirect_uris"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&in) != nil {
			oauthErr(w, 400, "invalid_client_metadata", "send JSON")
			return
		}
		id, c, err := s.Register(in.Name, in.URIs)
		if err != nil {
			oauthErr(w, 400, "invalid_redirect_uri", err.Error())
			return
		}
		writeJSON(w, 201, map[string]any{
			"client_id": id, "client_name": c.Name, "redirect_uris": c.RedirectURIs, "grant_types": []string{"authorization_code"},
			"response_types": []string{"code"}, "token_endpoint_auth_method": "none",
		})
	})
	mux.HandleFunc("/oauth/authorize", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if _, err := s.CheckRequest(q.Get("client_id"), q.Get("redirect_uri")); err != nil {
			http.Error(w, "invalid authorization request: "+err.Error(), http.StatusBadRequest)
			return
		}
		redirect := q.Get("redirect_uri")
		fail := func(e string) {
			http.Redirect(w, r, ReturnURL(redirect, map[string]string{"error": e, "state": q.Get("state")}), http.StatusFound) // #nosec G710 -- the address is one the client registered, checked by CheckRequest above
		}
		if q.Get("response_type") != "code" {
			fail("unsupported_response_type")
			return
		}
		if q.Get("code_challenge") == "" || q.Get("code_challenge_method") != "S256" {
			fail("invalid_request")
			return
		}
		out := url.Values{}
		for _, k := range []string{"client_id", "redirect_uri", "state", "code_challenge", "code_challenge_method", "scope"} {
			if v := q.Get(k); v != "" {
				out.Set(k, v)
			}
		}
		http.Redirect(w, r, s.ConsentURL(out), http.StatusFound)
	})
	mux.HandleFunc("/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			oauthErr(w, 405, "invalid_request", "POST only")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
		if r.ParseForm() != nil {
			oauthErr(w, 400, "invalid_request", "send a form")
			return
		}
		if r.PostForm.Get("grant_type") != "authorization_code" {
			oauthErr(w, 400, "unsupported_grant_type", "only authorization_code")
			return
		}
		secret, scope, err := s.Exchange(r.PostForm.Get("code"), r.PostForm.Get("client_id"), r.PostForm.Get("redirect_uri"), r.PostForm.Get("code_verifier"))
		if err != nil {
			oauthErr(w, 400, "invalid_grant", "the code is wrong, expired or already used")
			return
		}
		writeJSON(w, 200, map[string]any{"access_token": secret, "token_type": "Bearer", "scope": strings.TrimSpace(scope)})
	})
	// The ingress sends /api-pages/... and the fasthttp adapter keeps that prefix in the request address;
	// the patterns above are written without it.
	prefix := ""
	if u, err := url.Parse(s.base); err == nil {
		prefix = strings.TrimRight(u.Path, "/")
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if prefix != "" && strings.HasPrefix(r.URL.Path, prefix+"/") {
			r2 := r.Clone(r.Context())
			r2.URL.Path = strings.TrimPrefix(r.URL.Path, prefix)
			r = r2
		}
		// ChatGPT also looks next to the MCP address itself: /mcp/.well-known/...
		if strings.HasPrefix(r.URL.Path, s.MCPPath+"/.well-known/") {
			r2 := r.Clone(r.Context())
			r2.URL.Path = strings.TrimPrefix(r.URL.Path, s.MCPPath)
			r = r2
		}
		mux.ServeHTTP(w, r)
	})
}
