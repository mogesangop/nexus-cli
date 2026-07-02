package nexus

import "fmt"

// Repository mirrors the subset of Nexus repository fields nexus-cli needs.
type Repository struct {
	Name   string `json:"name"`
	Format string `json:"format"`
	Type   string `json:"type"`
}

// ListRepositories returns all repositories visible to the authenticated user.
// Endpoint: GET /repositories (PRD 20.2).
//
// NOTE: Field names should be verified against the target Nexus 3.76 Swagger
// (UI → Settings → System → API). Different minor versions may emit
// additional fields; this struct captures only what nexus-cli consumes.
func (c *Client) ListRepositories() ([]Repository, error) {
	var repos []Repository
	if err := c.get("/repositories", &repos); err != nil {
		return nil, fmt.Errorf("list repositories: %w", err)
	}
	return repos, nil
}
