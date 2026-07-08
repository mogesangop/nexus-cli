package nexus

import "fmt"

// ContentSelector represents a Nexus content selector (type "csel"), which
// scopes repository paths via a CSEL expression such as `path ^= "/dir/"`.
type ContentSelector struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Type        string `json:"type"`
	Expression  string `json:"expression"`
}

// CreateContentSelector creates a content selector.
// Endpoint: POST /security/content-selectors.
//
// NOTE: The expression dialect is Nexus CSEL (JEXL-ish). `path ^= "..."` is a
// prefix match. Verify against the target Nexus 3.76 Swagger before relying on
// this in production.
func (c *Client) CreateContentSelector(name, expression string) (*ContentSelector, error) {
	body := map[string]any{
		"name":        name,
		"description": "managed by nexus-cli",
		"type":        "csel",
		"expression":  expression,
	}
	var out ContentSelector
	if err := c.post("/security/content-selectors", body, &out); err != nil {
		return nil, fmt.Errorf("create content selector %s: %w", name, err)
	}
	return &out, nil
}

// ListContentSelectors returns all Nexus content selectors.
// Endpoint: GET /security/content-selectors.
func (c *Client) ListContentSelectors() ([]ContentSelector, error) {
	var out []ContentSelector
	if err := c.get("/security/content-selectors", &out); err != nil {
		return nil, fmt.Errorf("list content selectors: %w", err)
	}
	return out, nil
}

// GetContentSelector fetches a content selector by name. Returns an *APIError
// with Status 404 (see IsNotFound) when it does not exist.
// Endpoint: GET /security/content-selectors/{name}.
//
// NOTE: Verify the path parameter is the selector name (not a numeric id) on
// the target Nexus 3.76 Swagger.
func (c *Client) GetContentSelector(name string) (*ContentSelector, error) {
	var out ContentSelector
	if err := c.get("/security/content-selectors/"+name, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
