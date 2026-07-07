package repoctl

import (
	"testing"

	"github.com/231397220/nexus-cli/internal/config"
	"github.com/231397220/nexus-cli/internal/nexus"
)

type fakeClient struct {
	repos   []nexus.Repository
	current map[string]any
	created []map[string]any
	updated []map[string]any
}

func (f *fakeClient) ListRepositories() ([]nexus.Repository, error) { return f.repos, nil }
func (f *fakeClient) GetRepository(string, string, string) (map[string]any, error) {
	return f.current, nil
}
func (f *fakeClient) CreateRepository(_ string, _ string, body map[string]any) error {
	f.created = append(f.created, body)
	return nil
}
func (f *fakeClient) UpdateRepository(_ string, _ string, _ string, body map[string]any) error {
	f.updated = append(f.updated, body)
	return nil
}

func managedDesired() config.ManagedRepository {
	return config.ManagedRepository{
		Name: "npm-hosted", Format: "npm", Type: "hosted",
		Settings: map[string]any{"online": true, "storage": map[string]any{"blobStoreName": "default"}},
	}
}

func TestEnsureCreateAndDryRun(t *testing.T) {
	f := &fakeClient{}
	result, err := New().Ensure(f, managedDesired(), true)
	if err != nil {
		t.Fatal(err)
	}
	if result.Action != ActionCreate || len(f.created) != 0 {
		t.Fatalf("result=%+v created=%d", result, len(f.created))
	}
	result, err = New().Ensure(f, managedDesired(), false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Action != ActionCreate || len(f.created) != 1 {
		t.Fatalf("result=%+v created=%d", result, len(f.created))
	}
	if f.created[0]["name"] != "npm-hosted" {
		t.Fatalf("name was not injected: %+v", f.created[0])
	}
}

func TestEnsureUnchangedAllowsExtraResponseFields(t *testing.T) {
	f := &fakeClient{
		repos: []nexus.Repository{{Name: "npm-hosted", Format: "npm", Type: "hosted"}},
		current: map[string]any{
			"name": "npm-hosted", "online": true, "url": "http://nexus/repository/npm-hosted",
			"storage": map[string]any{"blobStoreName": "default", "writePolicy": "ALLOW"},
		},
	}
	result, err := New().Ensure(f, managedDesired(), false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Action != ActionUnchanged || len(f.updated) != 0 {
		t.Fatalf("result=%+v updated=%d", result, len(f.updated))
	}
}

func TestEnsureUpdate(t *testing.T) {
	f := &fakeClient{
		repos:   []nexus.Repository{{Name: "npm-hosted", Format: "npm", Type: "hosted"}},
		current: map[string]any{"name": "npm-hosted", "online": false},
	}
	result, err := New().Ensure(f, managedDesired(), false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Action != ActionUpdate || len(f.updated) != 1 {
		t.Fatalf("result=%+v updated=%d", result, len(f.updated))
	}
}

func TestEnsureRejectsFormatTypeConflict(t *testing.T) {
	f := &fakeClient{repos: []nexus.Repository{{Name: "npm-hosted", Format: "raw", Type: "hosted"}}}
	if _, err := New().Ensure(f, managedDesired(), false); err == nil {
		t.Fatal("expected conflict")
	}
}
