package mcpkit

import (
	"context"
	"encoding/json"
	"sort"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// DescribeTool is the name of the tool every server gets for free: it tells an AI what this server is for and what
// the key it connected with may do there, so the model can plan before it calls anything.
const DescribeTool = "describe_server"

// DescribedTool is one tool as describe_server tells it.
type DescribedTool struct {
	Name        string `json:"name"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description"`
	// Class is the scope a key needs to call it: "read", "write" or "publish".
	Class string `json:"class"`
	// Allowed says whether this key carries that scope.
	Allowed bool `json:"allowed"`
	// The schemas are JSON Schema objects, only on request.
	InputSchema  map[string]any `json:"input_schema,omitempty"`
	OutputSchema map[string]any `json:"output_schema,omitempty"`
}

// Description is the answer of describe_server.
type Description struct {
	Server       string `json:"server"`
	Version      string `json:"version"`
	Instructions string `json:"instructions,omitempty"`
	Guide        string `json:"guide,omitempty"`
	Access       struct {
		Namespace string   `json:"namespace,omitempty"`
		Role      string   `json:"role,omitempty"`
		Owner     bool     `json:"owner"`
		SiteID    string   `json:"site_id,omitempty"`
		Scopes    []string `json:"scopes"`
	} `json:"access"`
	Classes map[string]string `json:"classes"`
	// Counts is the number of tools per class.
	Counts map[string]int  `json:"counts"`
	Tools  []DescribedTool `json:"tools"`
	Hint   string          `json:"hint,omitempty"`
}

type describeIn struct {
	Tool        string `json:"tool,omitempty" jsonschema:"one tool name: return only that tool with its full input and output schema"`
	WithSchemas bool   `json:"with_schemas,omitempty" jsonschema:"return the input and output schema of every tool (long)"`
}

var classes = map[string]string{
	string(Read):  "only looks at data; safe to call any time",
	string(Write): "changes drafts and settings; nothing the public sees changes at once",
	string(Act):   "has an outside effect (publishing, sending, deleting, money): ask the person first",
}

// addDescribe registers describe_server unless the service already has a tool of that name.
func (c *Config) addDescribe(srv *sdk.Server, p *Principal) {
	AddTool(srv, p, Read, &sdk.Tool{
		Name:  DescribeTool,
		Title: "Describe this server",
		Description: "What this MCP server is for and everything it can do: the usage guide, the permissions of your key and " +
			"every tool with its purpose and permission class (allowed=false: your key cannot call it). Call it first when you " +
			"connect or when unsure which tool fits. Pass tool=<name> for one tool's full input schema, with_schemas=true for all.",
	}, func(ctx context.Context, p *Principal, in describeIn) (*Description, error) {
		return c.Describe(ctx, p, in.Tool, in.WithSchemas || in.Tool != "")
	})
}

// Describe lists what the server offers to one principal, read from the tools it really registers for that key.
func (c *Config) Describe(ctx context.Context, p *Principal, only string, schemas bool) (*Description, error) {
	c.defaults()
	ct, st := sdk.NewInMemoryTransports()
	ss, err := c.NewServer(p).Connect(ctx, st, nil)
	if err != nil {
		return nil, err
	}
	defer ss.Close()
	cs, err := sdk.NewClient(&sdk.Implementation{Name: "describe", Version: "1"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		return nil, err
	}
	defer cs.Close()

	d := &Description{Server: c.Name, Version: c.Version, Instructions: c.Instructions, Guide: c.Guide, Classes: classes,
		Counts: map[string]int{}, Tools: []DescribedTool{}}
	d.Access.Namespace, d.Access.Role, d.Access.Owner, d.Access.SiteID = p.Namespace, p.Role, p.Owner, p.SiteID
	d.Access.Scopes = []string{}
	for s, ok := range p.Scopes {
		if ok {
			d.Access.Scopes = append(d.Access.Scopes, s)
		}
	}
	sort.Strings(d.Access.Scopes)

	params := &sdk.ListToolsParams{}
	for {
		res, err := cs.ListTools(ctx, params)
		if err != nil {
			return nil, err
		}
		for _, t := range res.Tools {
			if t.Name == DescribeTool {
				continue
			}
			class := classOf(t.Annotations)
			d.Counts[class]++
			if only != "" && t.Name != only {
				continue
			}
			dt := DescribedTool{Name: t.Name, Title: t.Title, Description: t.Description, Class: class, Allowed: p.Scopes[class]}
			if schemas {
				dt.InputSchema, dt.OutputSchema = schemaMap(t.InputSchema), schemaMap(t.OutputSchema)
			}
			d.Tools = append(d.Tools, dt)
		}
		if res.NextCursor == "" {
			break
		}
		params.Cursor = res.NextCursor
	}
	sort.SliceStable(d.Tools, func(i, j int) bool { return rank(d.Tools[i].Class) < rank(d.Tools[j].Class) })
	switch {
	case only != "" && len(d.Tools) == 0:
		d.Hint = "no tool named " + strings.TrimSpace(only) + " on this server; call describe_server without tool to see the list"
		d.Instructions, d.Guide = "", ""
	case only != "":
		d.Instructions, d.Guide = "", "" // keep the answer about the one tool
	case !schemas:
		d.Hint = "pass tool=<name> to get the input schema of one tool"
	}
	return d, nil
}

// schemaMap turns a schema as the client got it into a plain JSON object.
func schemaMap(s any) map[string]any {
	if s == nil {
		return nil
	}
	b, err := json.Marshal(s)
	if err != nil {
		return nil
	}
	var m map[string]any
	if json.Unmarshal(b, &m) != nil {
		return nil
	}
	return m
}
