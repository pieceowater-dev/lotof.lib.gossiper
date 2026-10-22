package mcpkit

import (
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Guard keeps small in-memory counters of failed sign-ins per address (per replica). It is the only brake in front
// of the tools: there is deliberately no request rate or daily cap on what a valid key may do.
type Guard struct {
	limit  int
	window time.Duration

	mu        sync.Mutex
	fails     map[string][]time.Time
	lastSweep time.Time
}

// NewGuard blocks an address after limit failures within window.
func NewGuard(limit int, window time.Duration) *Guard {
	return &Guard{limit: limit, window: window, fails: map[string][]time.Time{}}
}

// Window is how long a block lasts at most.
func (g *Guard) Window() time.Duration { return g.window }

func (g *Guard) sweep(now time.Time) {
	if now.Sub(g.lastSweep) < time.Minute {
		return
	}
	g.lastSweep = now
	for k, f := range g.fails {
		if len(f) == 0 || now.Sub(f[len(f)-1]) > g.window {
			delete(g.fails, k)
		}
	}
}

// Blocked reports whether the address has failed too often recently.
func (g *Guard) Blocked(addr string, now time.Time) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.sweep(now)
	recent := g.fails[addr][:0]
	for _, t := range g.fails[addr] {
		if now.Sub(t) < g.window {
			recent = append(recent, t)
		}
	}
	g.fails[addr] = recent
	return len(recent) >= g.limit
}

// Fail records a failed sign-in.
func (g *Guard) Fail(addr string, now time.Time) {
	g.mu.Lock()
	g.fails[addr] = append(g.fails[addr], now)
	g.mu.Unlock()
}

// ClientAddr is the caller's address. Behind a load balancer the last X-Forwarded-For entry is the one the
// balancer itself appended; earlier entries are client-supplied and not trusted.
func ClientAddr(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		if ip := strings.TrimSpace(parts[len(parts)-1]); ip != "" {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// TooMany answers an address that is refused because of too many failed sign-ins.
func TooMany(w http.ResponseWriter, wait time.Duration) {
	w.Header().Set("Retry-After", strconv.Itoa(int(wait.Seconds())+1))
	http.Error(w, "too many requests", http.StatusTooManyRequests)
}
