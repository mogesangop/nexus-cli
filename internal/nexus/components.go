package nexus

import (
	"fmt"
	"net/url"
)

type Asset struct {
	ID           string `json:"id"`
	Path         string `json:"path"`
	LastModified string `json:"lastModified"`
}

type Component struct {
	ID         string  `json:"id"`
	Repository string  `json:"repository"`
	Format     string  `json:"format"`
	Name       string  `json:"name"`
	Assets     []Asset `json:"assets"`
}

type ComponentPage struct {
	Items             []Component `json:"items"`
	ContinuationToken *string     `json:"continuationToken"`
}

// ListComponents returns one page from the official Components API.
func (c *Client) ListComponents(repository, continuationToken string) (*ComponentPage, error) {
	q := url.Values{"repository": []string{repository}}
	if continuationToken != "" {
		q.Set("continuationToken", continuationToken)
	}
	var out ComponentPage
	if err := c.get("/components?"+q.Encode(), &out); err != nil {
		return nil, fmt.Errorf("list components for %s: %w", repository, err)
	}
	return &out, nil
}

func (c *Client) DeleteComponent(id string) error {
	if err := c.delete("/components/" + url.PathEscape(id)); err != nil {
		return fmt.Errorf("delete component %s: %w", id, err)
	}
	return nil
}
