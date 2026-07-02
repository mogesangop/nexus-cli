package nexus

import "fmt"

// Privilege represents a Nexus security privilege.
type Privilege struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Type        string `json:"type"`
	Properties  map[string]string `json:"properties,omitempty"`
	ReadOnly    bool   `json:"readOnly,omitempty"`
}

// CreateRepositoryViewPrivilege creates a privilege of type
// "repository-view" granting actions on a repository (format, name).
// Endpoint: POST /security/privileges (PRD 20.2).
//
// NOTE: The exact request body field names ("repository", "format",
// "actions") must match the target Nexus 3.76 Swagger. Verify before relying
// on this in production.
func (c *Client) CreateRepositoryViewPrivilege(name, format, repo string, actions []string) (*Privilege, error) {
	body := map[string]any{
		"name":       name,
		"description": "managed by nexus-cli",
		"type":        "repository-view",
		"properties": map[string]any{
			"repository": repo,
			"format":     format,
			"actions":    joinActions(actions),
		},
	}
	var out Privilege
	if err := c.post("/security/privileges", body, &out); err != nil {
		return nil, fmt.Errorf("create privilege %s: %w", name, err)
	}
	return &out, nil
}

// CreateRepositoryContentSelectorPrivilege creates a privilege of type
// "repository-content-selector" granting actions on a repository (format, name)
// scoped to the paths matched by a named content selector.
// Endpoint: POST /security/privileges.
//
// NOTE: The property key "contentSelector" and its value shape (the selector
// name) must match the target Nexus 3.76 Swagger. Verify before relying on
// this in production.
func (c *Client) CreateRepositoryContentSelectorPrivilege(name, format, repo, selector string, actions []string) (*Privilege, error) {
	body := map[string]any{
		"name":        name,
		"description": "managed by nexus-cli",
		"type":        "repository-content-selector",
		"properties": map[string]any{
			"repository":      repo,
			"format":          format,
			"contentSelector": selector,
			"actions":         joinActions(actions),
		},
	}
	var out Privilege
	if err := c.post("/security/privileges", body, &out); err != nil {
		return nil, fmt.Errorf("create privilege %s: %w", name, err)
	}
	return &out, nil
}

// GetPrivilege fetches a single privilege by name. Returns an *APIError with
// Status 404 (see IsNotFound) when it does not exist.
// Endpoint: GET /security/privileges/{name}.
func (c *Client) GetPrivilege(name string) (*Privilege, error) {
	var out Privilege
	if err := c.get("/security/privileges/"+name, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListPrivileges returns all privileges.
// Endpoint: GET /security/privileges.
func (c *Client) ListPrivileges() ([]Privilege, error) {
	var out []Privilege
	if err := c.get("/security/privileges", &out); err != nil {
		return nil, fmt.Errorf("list privileges: %w", err)
	}
	return out, nil
}

// joinActions renders a Nexus actions property value. Nexus expects a
// comma-separated string for repository-view privilege actions.
func joinActions(actions []string) string {
	out := ""
	for i, a := range actions {
		if i > 0 {
			out += ","
		}
		out += a
	}
	return out
}
