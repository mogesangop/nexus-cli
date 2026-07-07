package blobstore

import (
	"fmt"

	"github.com/231397220/nexus-cli/internal/config"
	"github.com/231397220/nexus-cli/internal/nexus"
)

type Client interface {
	ListBlobStores() ([]nexus.BlobStore, error)
	GetFileBlobStore(name string) (*nexus.FileBlobStore, error)
	CreateFileBlobStore(store nexus.FileBlobStore) error
	UpdateFileBlobStore(store nexus.FileBlobStore) error
}

type Action string

const (
	ActionCreate    Action = "create"
	ActionUpdate    Action = "update"
	ActionUnchanged Action = "unchanged"
)

type Result struct {
	Name   string
	Type   string
	Action Action
	DryRun bool
}

type Manager struct{}

func New() *Manager { return &Manager{} }

func (m *Manager) EnsureFile(client Client, desired config.FileBlobStore, dryRun bool) (*Result, error) {
	stores, err := client.ListBlobStores()
	if err != nil {
		return nil, err
	}
	for _, store := range stores {
		if store.Name == desired.Name {
			if store.Type != "" && store.Type != "file" {
				return nil, fmt.Errorf("blob store %q exists as %s; expected file", desired.Name, store.Type)
			}
			current, err := client.GetFileBlobStore(desired.Name)
			if err != nil {
				return nil, err
			}
			target := toNexus(desired)
			if equalFile(*current, target) {
				return &Result{Name: desired.Name, Type: "file", Action: ActionUnchanged, DryRun: dryRun}, nil
			}
			if !dryRun {
				if err := client.UpdateFileBlobStore(target); err != nil {
					return nil, err
				}
			}
			return &Result{Name: desired.Name, Type: "file", Action: ActionUpdate, DryRun: dryRun}, nil
		}
	}
	target := toNexus(desired)
	if !dryRun {
		if err := client.CreateFileBlobStore(target); err != nil {
			return nil, err
		}
	}
	return &Result{Name: desired.Name, Type: "file", Action: ActionCreate, DryRun: dryRun}, nil
}

func (m *Manager) ApplyFile(client Client, desired []config.FileBlobStore, dryRun bool) ([]Result, error) {
	results := make([]Result, 0, len(desired))
	for _, store := range desired {
		result, err := m.EnsureFile(client, store, dryRun)
		if err != nil {
			return results, err
		}
		results = append(results, *result)
	}
	return results, nil
}

func toNexus(store config.FileBlobStore) nexus.FileBlobStore {
	out := nexus.FileBlobStore{Name: store.Name, Path: store.Path}
	if store.SoftQuota != nil {
		out.SoftQuota = &nexus.SoftQuota{Type: store.SoftQuota.Type, Limit: store.SoftQuota.Limit}
	}
	return out
}

func equalFile(a, b nexus.FileBlobStore) bool {
	if a.Name != b.Name || a.Path != b.Path {
		return false
	}
	if a.SoftQuota == nil || b.SoftQuota == nil {
		return a.SoftQuota == nil && b.SoftQuota == nil
	}
	return a.SoftQuota.Type == b.SoftQuota.Type && a.SoftQuota.Limit == b.SoftQuota.Limit
}
