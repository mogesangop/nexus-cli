package rawrepo

import (
	"fmt"
	"strings"

	"github.com/231397220/nexus-cli/internal/config"
	"github.com/231397220/nexus-cli/internal/nexus"
)

type Client interface {
	ListRepositories() ([]nexus.Repository, error)
	GetRawHostedRepository(name string) (*nexus.RawHostedRepository, error)
	CreateRawHostedRepository(repo nexus.RawHostedRepository) error
	UpdateRawHostedRepository(repo nexus.RawHostedRepository) error
}

type Action string

const (
	ActionCreate    Action = "create"
	ActionUpdate    Action = "update"
	ActionUnchanged Action = "unchanged"
)

type Result struct {
	Name   string
	Action Action
	DryRun bool
}

type Manager struct{}

func New() *Manager { return &Manager{} }

// Ensure creates or safely updates one raw hosted repository.
func (m *Manager) Ensure(client Client, desired config.RawRepository, dryRun bool) (*Result, error) {
	repos, err := client.ListRepositories()
	if err != nil {
		return nil, err
	}
	var summary *nexus.Repository
	for i := range repos {
		if repos[i].Name == desired.Name {
			summary = &repos[i]
			break
		}
	}
	target := toNexus(desired)
	if summary == nil {
		if !dryRun {
			if err := client.CreateRawHostedRepository(target); err != nil {
				return nil, err
			}
		}
		return &Result{Name: desired.Name, Action: ActionCreate, DryRun: dryRun}, nil
	}
	if summary.Format != "raw" || summary.Type != "hosted" {
		return nil, fmt.Errorf("repository %q exists as %s/%s; expected raw/hosted", desired.Name, summary.Format, summary.Type)
	}
	current, err := client.GetRawHostedRepository(desired.Name)
	if err != nil {
		return nil, err
	}
	if current.Storage.BlobStoreName != desired.Storage.BlobStoreName {
		return nil, fmt.Errorf("repository %q uses blob store %q; refusing migration to %q", desired.Name, current.Storage.BlobStoreName, desired.Storage.BlobStoreName)
	}
	if equalMutable(*current, target) {
		return &Result{Name: desired.Name, Action: ActionUnchanged, DryRun: dryRun}, nil
	}
	// PUT requires a complete request. Preserve the server's name and blob store,
	// while replacing only fields declared safe by this feature.
	current.Online = target.Online
	current.Storage.StrictContentTypeValidation = target.Storage.StrictContentTypeValidation
	current.Storage.WritePolicy = target.Storage.WritePolicy
	current.Raw.ContentDisposition = target.Raw.ContentDisposition
	if !dryRun {
		if err := client.UpdateRawHostedRepository(*current); err != nil {
			return nil, err
		}
	}
	return &Result{Name: desired.Name, Action: ActionUpdate, DryRun: dryRun}, nil
}

func (m *Manager) Apply(client Client, desired []config.RawRepository, dryRun bool) ([]Result, error) {
	results := make([]Result, 0, len(desired))
	for _, repo := range desired {
		result, err := m.Ensure(client, repo, dryRun)
		if err != nil {
			return results, err
		}
		results = append(results, *result)
	}
	return results, nil
}

func toNexus(r config.RawRepository) nexus.RawHostedRepository {
	return nexus.RawHostedRepository{
		Name:   r.Name,
		Online: r.Online,
		Storage: nexus.RepositoryStorage{
			BlobStoreName:               r.Storage.BlobStoreName,
			StrictContentTypeValidation: r.Storage.StrictContentTypeValidation,
			WritePolicy:                 strings.ToUpper(r.Storage.WritePolicy),
		},
		Raw: nexus.RawSettings{ContentDisposition: strings.ToUpper(r.ContentDisposition)},
	}
}

func equalMutable(a, b nexus.RawHostedRepository) bool {
	return a.Online == b.Online &&
		a.Storage.StrictContentTypeValidation == b.Storage.StrictContentTypeValidation &&
		strings.EqualFold(a.Storage.WritePolicy, b.Storage.WritePolicy) &&
		strings.EqualFold(a.Raw.ContentDisposition, b.Raw.ContentDisposition)
}
