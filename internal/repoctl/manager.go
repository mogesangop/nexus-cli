package repoctl

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/231397220/nexus-cli/internal/config"
	"github.com/231397220/nexus-cli/internal/nexus"
)

type Client interface {
	ListRepositories() ([]nexus.Repository, error)
	GetRepository(format, typ, name string) (map[string]any, error)
	CreateRepository(format, typ string, body map[string]any) error
	UpdateRepository(format, typ, name string, body map[string]any) error
}

type Action string

const (
	ActionCreate    Action = "create"
	ActionUpdate    Action = "update"
	ActionUnchanged Action = "unchanged"
)

type Result struct {
	Name   string
	Format string
	Type   string
	Action Action
	DryRun bool
}

type Manager struct{}

func New() *Manager { return &Manager{} }

func (m *Manager) Ensure(client Client, desired config.ManagedRepository, dryRun bool) (*Result, error) {
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
	body, err := requestBody(desired)
	if err != nil {
		return nil, err
	}
	if summary == nil {
		if !dryRun {
			if err := client.CreateRepository(desired.Format, desired.Type, body); err != nil {
				return nil, err
			}
		}
		return &Result{Name: desired.Name, Format: desired.Format, Type: desired.Type, Action: ActionCreate, DryRun: dryRun}, nil
	}
	if summary.Format != desired.Format || summary.Type != desired.Type {
		return nil, fmt.Errorf("repository %q exists as %s/%s; expected %s/%s", desired.Name, summary.Format, summary.Type, desired.Format, desired.Type)
	}
	current, err := client.GetRepository(desired.Format, desired.Type, desired.Name)
	if err != nil {
		return nil, err
	}
	if containsJSON(current, body) {
		return &Result{Name: desired.Name, Format: desired.Format, Type: desired.Type, Action: ActionUnchanged, DryRun: dryRun}, nil
	}
	if !dryRun {
		if err := client.UpdateRepository(desired.Format, desired.Type, desired.Name, body); err != nil {
			return nil, err
		}
	}
	return &Result{Name: desired.Name, Format: desired.Format, Type: desired.Type, Action: ActionUpdate, DryRun: dryRun}, nil
}

func (m *Manager) Apply(client Client, desired []config.ManagedRepository, dryRun bool) ([]Result, error) {
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

func requestBody(desired config.ManagedRepository) (map[string]any, error) {
	body := cloneMap(desired.Settings)
	if body == nil {
		body = map[string]any{}
	}
	if v, ok := body["name"].(string); ok && v != "" && v != desired.Name {
		return nil, fmt.Errorf("settings.name %q does not match repository name %q", v, desired.Name)
	}
	body["name"] = desired.Name
	return body, nil
}

func cloneMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func containsJSON(current, desired map[string]any) bool {
	na, errA := normalize(current)
	nb, errB := normalize(desired)
	if errA != nil || errB != nil {
		return containsValue(current, desired)
	}
	return containsValue(na, nb)
}

func normalize(v any) (any, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var out any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func containsValue(current, desired any) bool {
	desiredMap, ok := desired.(map[string]any)
	if !ok {
		return reflect.DeepEqual(current, desired)
	}
	currentMap, ok := current.(map[string]any)
	if !ok {
		return false
	}
	for k, desiredValue := range desiredMap {
		currentValue, exists := currentMap[k]
		if !exists || !containsValue(currentValue, desiredValue) {
			return false
		}
	}
	return true
}
