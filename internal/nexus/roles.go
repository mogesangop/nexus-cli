package nexus

import "fmt"

// Role represents a Nexus security role.
type Role struct {
	ID          string   `json:"id"`
	Source      string   `json:"source,omitempty"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Privileges  []string `json:"privileges,omitempty"`
	Roles       []string `json:"roles,omitempty"`
}

// GetRole fetches a role by id. Returns a 404 *APIError (see IsNotFound) if
// the role does not exist. Endpoint: GET /security/roles/{id}.
func (c *Client) GetRole(id string) (*Role, error) {
	var out Role
	if err := c.get("/security/roles/"+id, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreateRole creates a new role. Endpoint: POST /security/roles (PRD 20.2).
func (c *Client) CreateRole(r *Role) (*Role, error) {
	body := map[string]any{
		"id":          r.ID,
		"name":        r.Name,
		"description": r.Description,
		"privileges":  r.Privileges,
		"roles":       r.Roles,
	}
	var out Role
	if err := c.post("/security/roles", body, &out); err != nil {
		return nil, fmt.Errorf("create role %s: %w", r.ID, err)
	}
	return &out, nil
}

// UpdateRole replaces a role's privileges and nested roles.
// Endpoint: PUT /security/roles/{id} (PRD 20.2).
//
// NOTE: Some Nexus versions require the full Role object on PUT and may reject
// unknown fields. Verify against the target Nexus 3.76 Swagger.
func (c *Client) UpdateRole(id string, r *Role) error {
	body := map[string]any{
		"id":          id,
		"name":        r.Name,
		"description": r.Description,
		"privileges":  r.Privileges,
		"roles":       r.Roles,
	}
	if err := c.put("/security/roles/"+id, body); err != nil {
		return fmt.Errorf("update role %s: %w", id, err)
	}
	return nil
}
