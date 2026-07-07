package nexus

import (
	"fmt"
	"net/url"
)

// BlobStore mirrors the common fields returned by Nexus blob store listings.
type BlobStore struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// SoftQuota is the Nexus blob store soft quota shape.
type SoftQuota struct {
	Type  string `json:"type"`
	Limit int64  `json:"limit"`
}

// FileBlobStore is the request/response body for file blob stores.
type FileBlobStore struct {
	Name      string     `json:"name"`
	Path      string     `json:"path"`
	SoftQuota *SoftQuota `json:"softQuota,omitempty"`
}

func (c *Client) ListBlobStores() ([]BlobStore, error) {
	var stores []BlobStore
	if err := c.get("/blobstores", &stores); err != nil {
		return nil, fmt.Errorf("list blob stores: %w", err)
	}
	return stores, nil
}

func (c *Client) GetFileBlobStore(name string) (*FileBlobStore, error) {
	var out FileBlobStore
	if err := c.get("/blobstores/file/"+url.PathEscape(name), &out); err != nil {
		return nil, fmt.Errorf("get file blob store %s: %w", name, err)
	}
	return &out, nil
}

func (c *Client) CreateFileBlobStore(store FileBlobStore) error {
	if err := c.post("/blobstores/file", store, nil); err != nil {
		return fmt.Errorf("create file blob store %s: %w", store.Name, err)
	}
	return nil
}

func (c *Client) UpdateFileBlobStore(store FileBlobStore) error {
	if err := c.put("/blobstores/file/"+url.PathEscape(store.Name), store); err != nil {
		return fmt.Errorf("update file blob store %s: %w", store.Name, err)
	}
	return nil
}
