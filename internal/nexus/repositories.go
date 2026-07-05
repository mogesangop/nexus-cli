package nexus

import (
	"fmt"
	"net/url"
)

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

// RawHostedRepository is the Nexus raw hosted repository request/response.
type RawHostedRepository struct {
	Name      string             `json:"name"`
	Online    bool               `json:"online"`
	Storage   RepositoryStorage  `json:"storage"`
	Cleanup   *CleanupSettings   `json:"cleanup,omitempty"`
	Component *ComponentSettings `json:"component,omitempty"`
	Raw       RawSettings        `json:"raw"`
}

type RepositoryStorage struct {
	BlobStoreName               string `json:"blobStoreName"`
	StrictContentTypeValidation bool   `json:"strictContentTypeValidation"`
	WritePolicy                 string `json:"writePolicy"`
}

type RawSettings struct {
	ContentDisposition string `json:"contentDisposition"`
}

type CleanupSettings struct {
	PolicyNames []string `json:"policyNames"`
}

type ComponentSettings struct {
	ProprietaryComponents bool `json:"proprietaryComponents"`
}

func (c *Client) GetRawHostedRepository(name string) (*RawHostedRepository, error) {
	var out RawHostedRepository
	if err := c.get("/repositories/raw/hosted/"+url.PathEscape(name), &out); err != nil {
		return nil, fmt.Errorf("get raw hosted repository %s: %w", name, err)
	}
	return &out, nil
}

func (c *Client) CreateRawHostedRepository(repo RawHostedRepository) error {
	if err := c.post("/repositories/raw/hosted", repo, nil); err != nil {
		return fmt.Errorf("create raw hosted repository %s: %w", repo.Name, err)
	}
	return nil
}

func (c *Client) UpdateRawHostedRepository(repo RawHostedRepository) error {
	if err := c.put("/repositories/raw/hosted/"+url.PathEscape(repo.Name), repo); err != nil {
		return fmt.Errorf("update raw hosted repository %s: %w", repo.Name, err)
	}
	return nil
}
