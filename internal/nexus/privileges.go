package nexus

import "fmt"

// Privilege represents a Nexus security privilege.
type Privilege struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Type        string            `json:"type"`
	Properties  map[string]string `json:"properties,omitempty"`
	ReadOnly    bool              `json:"readOnly,omitempty"`
}

// CreateRepositoryViewPrivilege creates a privilege of type
// "repository-view" granting actions on a repository (format, name).
// Endpoint: POST /security/privileges/repository-view.
func (c *Client) CreateRepositoryViewPrivilege(name, format, repo string, actions []string) (*Privilege, error) {
	body := repositoryPrivilegeRequest{
		Name:        name,
		Description: "managed by nexus-cli",
		Actions:     actions,
		Format:      format,
		Repository:  repo,
	}
	var out Privilege
	if err := c.post("/security/privileges/repository-view", body, &out); err != nil {
		return nil, fmt.Errorf("create privilege %s: %w", name, err)
	}
	return &out, nil
}

// CreateRepositoryContentSelectorPrivilege creates a privilege of type
// "repository-content-selector" granting actions on a repository (format, name)
// scoped to the paths matched by a named content selector.
// Endpoint: POST /security/privileges/repository-content-selector.
func (c *Client) CreateRepositoryContentSelectorPrivilege(name, format, repo, selector string, actions []string) (*Privilege, error) {
	body := repositoryContentSelectorPrivilegeRequest{
		repositoryPrivilegeRequest: repositoryPrivilegeRequest{
			Name:        name,
			Description: "managed by nexus-cli",
			Actions:     actions,
			Format:      format,
			Repository:  repo,
		},
		ContentSelector: selector,
	}
	var out Privilege
	if err := c.post("/security/privileges/repository-content-selector", body, &out); err != nil {
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

type repositoryPrivilegeRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Actions     []string `json:"actions"`
	Format      string   `json:"format"`
	Repository  string   `json:"repository"`
}

type repositoryContentSelectorPrivilegeRequest struct {
	repositoryPrivilegeRequest
	ContentSelector string `json:"contentSelector"`
}
