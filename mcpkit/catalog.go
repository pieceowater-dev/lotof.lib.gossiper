package mcpkit

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"sync"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// CatalogTool is one tool as the public catalog lists it.
type CatalogTool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	// Class is the scope a key needs to call it: "read", "write" or "publish".
	Class string `json:"class"`
}

// Catalog is what a server can do, listed from the tools it really registers (so a web page, llms.txt or an AI
// can read the live list instead of a copy that drifts).
type Catalog struct {
	Server       string        `json:"server"`
	Instructions string        `json:"instructions,omitempty"`
	Tools        []CatalogTool `json:"tools"`
}

// classOf reads the permission class back from the hints AddTool sets.
func classOf(a *sdk.ToolAnnotations) string {
	switch {
	case a == nil:
		return string(Write)
	case a.ReadOnlyHint:
		return string(Read)
	case a.DestructiveHint != nil && *a.DestructiveHint:
		return string(Act)
	}
	return string(Write)
}

// Catalog lists the tools the server registers for a key that carries every scope. Nothing is called: it only
// asks the server for its tool list over an in-memory connection.
func (c *Config) Catalog(ctx context.Context) (*Catalog, error) {
	all := &Principal{TokenID: "catalog", Owner: true, Role: "OWNER", Scopes: map[string]bool{string(Read): true, string(Write): true, string(Act): true}}
	ct, st := sdk.NewInMemoryTransports()
	ss, err := c.NewServer(all).Connect(ctx, st, nil)
	if err != nil {
		return nil, err
	}
	defer ss.Close()
	cs, err := sdk.NewClient(&sdk.Implementation{Name: "catalog", Version: "1"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		return nil, err
	}
	defer cs.Close()
	out := &Catalog{Server: c.Name, Instructions: c.Instructions, Tools: []CatalogTool{}}
	params := &sdk.ListToolsParams{}
	for {
		res, err := cs.ListTools(ctx, params)
		if err != nil {
			return nil, err
		}
		for _, t := range res.Tools {
			out.Tools = append(out.Tools, CatalogTool{Name: t.Name, Description: t.Description, Class: classOf(t.Annotations)})
		}
		if res.NextCursor == "" {
			break
		}
		params.Cursor = res.NextCursor
	}
	sort.SliceStable(out.Tools, func(i, j int) bool { return rank(out.Tools[i].Class) < rank(out.Tools[j].Class) })
	return out, nil
}

func rank(class string) int {
	switch class {
	case string(Read):
		return 0
	case string(Write):
		return 1
	}
	return 2
}

// CatalogHandler serves the public, unauthenticated tool list as JSON (mount it at <mcp address>/tools). The list
// holds only tool names, descriptions and classes, no data. It is built once and cached for an hour.
func (c *Config) CatalogHandler() http.Handler {
	var (
		mu   sync.Mutex
		body []byte
		at   time.Time
	)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		mu.Lock()
		if body == nil || time.Since(at) > time.Hour {
			ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
			cat, err := c.Catalog(ctx)
			cancel()
			if err != nil {
				mu.Unlock()
				http.Error(w, "catalog unavailable", http.StatusServiceUnavailable)
				return
			}
			body, _ = json.Marshal(cat)
			at = time.Now()
		}
		b := body
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=300")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		_, _ = w.Write(b)
	})
}
