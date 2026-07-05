package lifecycle

import (
	"fmt"
	"testing"
	"time"

	"github.com/231397220/nexus-cli/internal/config"
	"github.com/231397220/nexus-cli/internal/nexus"
)

type fakeClient struct {
	repos     []nexus.Repository
	pages     map[string]*nexus.ComponentPage
	deleted   []string
	deleteErr map[string]error
}

func (f *fakeClient) ListRepositories() ([]nexus.Repository, error) {
	if f.repos == nil {
		return []nexus.Repository{{Name: "raw", Format: "raw", Type: "hosted"}}, nil
	}
	return f.repos, nil
}

func (f *fakeClient) ListComponents(_ string, token string) (*nexus.ComponentPage, error) {
	page, ok := f.pages[token]
	if !ok {
		return nil, fmt.Errorf("unexpected token %q", token)
	}
	return page, nil
}
func (f *fakeClient) DeleteComponent(id string) error {
	if err := f.deleteErr[id]; err != nil {
		return err
	}
	f.deleted = append(f.deleted, id)
	return nil
}

func str(s string) *string { return &s }

func TestPreviewPaginationAgeAndPaths(t *testing.T) {
	now := time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC)
	f := &fakeClient{pages: map[string]*nexus.ComponentPage{
		"": {
			Items: []nexus.Component{
				component("old", "releases/1/app.tar", now.AddDate(0, 0, -91)),
				component("latest", "releases/latest/app.tar", now.AddDate(0, 0, -200)),
			},
			ContinuationToken: str("next"),
		},
		"next": {
			Items: []nexus.Component{
				component("young", "releases/2/app.tar", now.AddDate(0, 0, -89)),
				{ID: "bad", Assets: []nexus.Asset{{Path: "releases/bad", LastModified: "bad"}}},
			},
		},
	}}
	policy := config.LifecycleConfig{
		Enabled: true, RetentionDays: 90,
		IncludePaths: []string{"^releases/"}, ExcludePaths: []string{"^releases/latest/"},
	}
	report, err := NewAt(now).Preview(f, "raw", policy)
	if err != nil {
		t.Fatal(err)
	}
	if report.Scanned != 4 || len(report.Candidates) != 1 || report.Candidates[0].ComponentID != "old" {
		t.Fatalf("unexpected report: %+v", report)
	}
	if len(report.Warnings) != 1 {
		t.Fatalf("warnings=%v", report.Warnings)
	}
	if len(f.deleted) != 0 {
		t.Fatal("preview deleted components")
	}
}

func TestRunDeletionAndNotFound(t *testing.T) {
	now := time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC)
	f := &fakeClient{
		pages: map[string]*nexus.ComponentPage{"": {Items: []nexus.Component{
			component("one", "one", now.AddDate(0, 0, -31)),
			component("gone", "gone", now.AddDate(0, 0, -31)),
		}}},
		deleteErr: map[string]error{"gone": fmt.Errorf("wrapped: %w", &nexus.APIError{Status: 404})},
	}
	report, err := NewAt(now).Run(f, "raw", config.LifecycleConfig{Enabled: true, RetentionDays: 30})
	if err != nil {
		t.Fatal(err)
	}
	if report.Deleted != 1 || report.NotFound != 1 || len(f.deleted) != 1 {
		t.Fatalf("unexpected report: %+v", report)
	}
}

func TestPreviewUsesNewestAssetTime(t *testing.T) {
	now := time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC)
	c := component("multi", "a", now.AddDate(0, 0, -100))
	c.Assets = append(c.Assets, nexus.Asset{Path: "b", LastModified: now.AddDate(0, 0, -10).Format(time.RFC3339Nano)})
	f := &fakeClient{pages: map[string]*nexus.ComponentPage{"": {Items: []nexus.Component{c}}}}
	report, err := NewAt(now).Preview(f, "raw", config.LifecycleConfig{Enabled: true, RetentionDays: 30})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Candidates) != 0 {
		t.Fatalf("new asset should protect component: %+v", report.Candidates)
	}
}

func TestPreviewRejectsNonRawRepository(t *testing.T) {
	f := &fakeClient{repos: []nexus.Repository{{Name: "raw", Format: "maven2", Type: "hosted"}}}
	_, err := New().Preview(f, "raw", config.LifecycleConfig{Enabled: true, RetentionDays: 30})
	if err == nil {
		t.Fatal("expected repository type error")
	}
}

func component(id, path string, modified time.Time) nexus.Component {
	return nexus.Component{
		ID: id, Assets: []nexus.Asset{{Path: path, LastModified: modified.Format(time.RFC3339Nano)}},
	}
}
