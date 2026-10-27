package mcpkit

import (
	"errors"
	"fmt"
)

// Class is a permission class of a tool. The value is the name of the scope an access key must carry.
type Class string

const (
	// Read tools only look at data.
	Read Class = "read"
	// Write tools change drafts and settings, nothing the public sees at once.
	Write Class = "write"
	// Act tools have an outside effect (publishing, domains, sending): the scope is called "publish" in lota keys.
	Act Class = "publish"
)

// Principal is who an access key belongs to.
type Principal struct {
	TokenID   string
	Namespace string
	UserID    string
	Scopes    map[string]bool
	SiteID    string // "" = any site (resource) of the namespace; otherwise the key is confined to this one
	Owner     bool   // the key was issued by the namespace owner
	Role      string // optional: the creator's role in the product (e.g. a staff role), set by the gateway that resolves the key
}

// Need returns an error unless the key carries the scope.
func (p *Principal) Need(scope string) error {
	if !p.Scopes[scope] {
		return fmt.Errorf("this access token has no %q permission", scope)
	}
	return nil
}

// Site resolves the effective site (resource) id: a key bound to one site is confined to it.
func (p *Principal) Site(id string) (string, error) {
	if p.SiteID == "" {
		if id == "" {
			return "", errors.New("site_id is required")
		}
		return id, nil
	}
	if id != "" && id != p.SiteID {
		return "", errors.New("this access token is limited to another site")
	}
	return p.SiteID, nil
}
