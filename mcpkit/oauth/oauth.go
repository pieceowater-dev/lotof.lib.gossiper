// Package oauth is the optional sign-in server of mcpkit. It lets AI clients (Claude, ChatGPT, Gemini, anything that
// speaks the MCP authorization flow) connect to a service's MCP endpoint without anyone copying a key: the client
// discovers this server, registers itself, sends the person to the product's consent page, and receives an
// ordinary access key at the end. The key is the same one the console creates by hand (same scopes, same
// resource binding, same revoke button); this package only carries it to the client safely. The product supplies
// the one thing it cannot know, how a key is created, through the KeyIssuer interface (see Approve).
//
// Nothing is stored here. A client id is a signed description of the client, an authorization code is the key
// sealed with AES-GCM together with what the token request must prove (client, redirect address, PKCE challenge,
// expiry). Codes are single use: spent ones are remembered until they expire.
package oauth

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"sync"
	"time"
)

// KeyLifetime is how long a key issued through the flow lasts; the token response states it as expires_in.
const KeyLifetime = 90 * 24 * time.Hour

const (
	codeTTL     = 2 * time.Minute
	maxRedirect = 5
)

var enc = base64.RawURLEncoding

// Server holds the secrets and addresses of the flow.
type Server struct {
	base       string // public address of the gateway, e.g. https://lota.tools/api-pages
	consentURL string // the web page where the person says yes

	// ResourceName is shown in the protected-resource document (default "lota").
	ResourceName string
	// Scopes are advertised as supported (default read, write, publish).
	Scopes []string
	// MCPPath is the MCP endpoint relative to base (default "/mcp").
	MCPPath string
	sigKey  []byte
	codeKey [32]byte

	mu   sync.Mutex
	used map[string]time.Time
}

// New returns nil (flow off) without a secret or a public address.
func New(secret, base, consentURL string) *Server {
	if secret == "" || base == "" || consentURL == "" {
		return nil
	}
	s := &Server{base: strings.TrimRight(base, "/"), consentURL: consentURL, used: map[string]time.Time{},
		ResourceName: "lota", Scopes: []string{"read", "write", "publish"}, MCPPath: "/mcp"}
	s.sigKey = mac([]byte(secret), "oauth-client")
	copy(s.codeKey[:], mac([]byte(secret), "oauth-code"))
	return s
}

func mac(key []byte, label string) []byte {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(label))
	return m.Sum(nil)
}

func (s *Server) Base() string {
	if s == nil {
		return ""
	}
	return s.base
}

// ── clients ──

// Client is what a registration says about an AI client.
type Client struct {
	Name         string   `json:"n"`
	RedirectURIs []string `json:"r"`
	IssuedAt     int64    `json:"t"`
}

func (s *Server) sign(b []byte) string {
	m := hmac.New(sha256.New, s.sigKey)
	m.Write(b)
	return enc.EncodeToString(m.Sum(nil))
}

// RedirectOK accepts what an AI client may use as a return address: https, or http on this computer
// (desktop clients listen on a local port).
func RedirectOK(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.Fragment != "" || len(raw) > 512 || u.User != nil {
		return false
	}
	switch u.Scheme {
	case "https":
		return true
	case "http":
		h := u.Hostname()
		return h == "localhost" || h == "127.0.0.1" || h == "::1"
	}
	return false
}

// Register creates the client id for a new client.
func (s *Server) Register(name string, uris []string) (string, *Client, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "AI client"
	}
	if r := []rune(name); len(r) > 80 {
		name = string(r[:80])
	}
	if len(uris) == 0 || len(uris) > maxRedirect {
		return "", nil, errors.New("redirect_uris: give between 1 and 5 addresses")
	}
	for _, u := range uris {
		if !RedirectOK(u) {
			return "", nil, errors.New("redirect_uris: only https addresses (or http on localhost) are accepted")
		}
	}
	c := &Client{Name: name, RedirectURIs: uris, IssuedAt: time.Now().Unix()}
	raw, _ := json.Marshal(c)
	return enc.EncodeToString(raw) + "." + s.sign(raw), c, nil
}

// ClientOf verifies a client id and returns the client.
func (s *Server) ClientOf(id string) (*Client, bool) {
	body, sig, ok := strings.Cut(id, ".")
	if !ok || len(id) > 4096 {
		return nil, false
	}
	raw, err := enc.DecodeString(body)
	if err != nil || subtle.ConstantTimeCompare([]byte(sig), []byte(s.sign(raw))) != 1 {
		return nil, false
	}
	var c Client
	if json.Unmarshal(raw, &c) != nil || len(c.RedirectURIs) == 0 {
		return nil, false
	}
	return &c, true
}

// CheckRequest validates an authorization request against its client and returns the client.
func (s *Server) CheckRequest(clientID, redirectURI string) (*Client, error) {
	c, ok := s.ClientOf(clientID)
	if !ok {
		return nil, errors.New("unknown client")
	}
	for _, u := range c.RedirectURIs {
		if u == redirectURI {
			return c, nil
		}
	}
	return nil, errors.New("the return address is not registered for this client")
}

// ── authorization codes ──

type codePayload struct {
	Secret    string `json:"s"` // the lota access key (lpt_…)
	ClientID  string `json:"c"`
	Redirect  string `json:"u"`
	Challenge string `json:"h"`
	Scope     string `json:"o"`
	Expires   int64  `json:"e"`
	Nonce     string `json:"n"`
}

func (s *Server) gcm() cipher.AEAD {
	blk, _ := aes.NewCipher(s.codeKey[:])
	g, _ := cipher.NewGCM(blk)
	return g
}

// Seal makes an authorization code carrying the key.
func (s *Server) Seal(secret, clientID, redirect, challenge, scope string) (string, error) {
	n := make([]byte, 12)
	if _, err := rand.Read(n); err != nil {
		return "", err
	}
	raw, _ := json.Marshal(codePayload{secret, clientID, redirect, challenge, scope, time.Now().Add(codeTTL).Unix(), enc.EncodeToString(n)}) // #nosec G117 -- the key is sealed with AES-GCM right below, never sent as plain JSON
	g := s.gcm()
	nonce := make([]byte, g.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	return enc.EncodeToString(append(nonce, g.Seal(nil, nonce, raw, nil)...)), nil
}

var ErrBadGrant = errors.New("invalid_grant")

// Exchange trades a code and its PKCE verifier for the key. A code works once.
func (s *Server) Exchange(code, clientID, redirect, verifier string) (secret, scope string, err error) {
	raw, derr := enc.DecodeString(code)
	g := s.gcm()
	if derr != nil || len(raw) < g.NonceSize()+16 || len(code) > 4096 {
		return "", "", ErrBadGrant
	}
	plain, oerr := g.Open(nil, raw[:g.NonceSize()], raw[g.NonceSize():], nil)
	if oerr != nil {
		return "", "", ErrBadGrant
	}
	var p codePayload
	if json.Unmarshal(plain, &p) != nil || time.Now().Unix() > p.Expires {
		return "", "", ErrBadGrant
	}
	// A code is spent by the first attempt, right or wrong: a guessed verifier must not get a second try.
	s.mu.Lock()
	now := time.Now()
	for k, exp := range s.used {
		if now.After(exp) {
			delete(s.used, k)
		}
	}
	_, spent := s.used[p.Nonce]
	s.used[p.Nonce] = time.Unix(p.Expires, 0).Add(time.Minute)
	s.mu.Unlock()
	if spent {
		return "", "", ErrBadGrant
	}
	sum := sha256.Sum256([]byte(verifier))
	if p.ClientID != clientID || p.Redirect != redirect || verifier == "" || len(verifier) < 43 || len(verifier) > 128 ||
		subtle.ConstantTimeCompare([]byte(enc.EncodeToString(sum[:])), []byte(p.Challenge)) != 1 {
		return "", "", ErrBadGrant
	}
	return p.Secret, p.Scope, nil
}

// ── where the person says yes ──

// ConsentURL is the web page for an authorization request.
func (s *Server) ConsentURL(q url.Values) string {
	u, _ := url.Parse(s.consentURL)
	u.RawQuery = q.Encode()
	return u.String()
}

// ReturnURL builds the address the client is sent back to.
func ReturnURL(redirect string, params map[string]string) string {
	u, _ := url.Parse(redirect)
	q := u.Query()
	for k, v := range params {
		if v != "" {
			q.Set(k, v)
		}
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// ── key issuing ──

// KeyIssuer creates the access key the person agreed to. The implementation must create it inside the rights of
// whoever is asking (the ctx carries them) and must refuse a key beyond their role.
type KeyIssuer interface {
	IssueKey(ctx context.Context, name string, scopes []string, resourceID string, expires time.Time) (secret string, err error)
}

// ApproveRequest is what the consent page sends once the person says yes.
type ApproveRequest struct {
	ClientID, RedirectURI, State, Challenge string
	Scopes                                  []string
	ResourceID                              string // "" = every resource of the namespace
}

// Approve issues the key through the issuer and returns the address to send the person back to, carrying the
// sealed code. Keys last 90 days.
func (s *Server) Approve(ctx context.Context, iss KeyIssuer, in ApproveRequest) (string, error) {
	cl, err := s.CheckRequest(in.ClientID, in.RedirectURI)
	if err != nil {
		return "", err
	}
	if in.Challenge == "" {
		return "", errors.New("code challenge is required")
	}
	secret, err := iss.IssueKey(ctx, "AI: "+cl.Name, in.Scopes, in.ResourceID, time.Now().Add(KeyLifetime))
	if err != nil {
		return "", err
	}
	code, err := s.Seal(secret, in.ClientID, in.RedirectURI, in.Challenge, strings.Join(in.Scopes, " "))
	if err != nil {
		return "", err
	}
	return ReturnURL(in.RedirectURI, map[string]string{"code": code, "state": in.State}), nil
}

// Deny returns the address to send the person back to after they said no.
func (s *Server) Deny(clientID, redirectURI, state string) (string, error) {
	if _, err := s.CheckRequest(clientID, redirectURI); err != nil {
		return "", err
	}
	return ReturnURL(redirectURI, map[string]string{"error": "access_denied", "state": state}), nil
}
