package rawrepo

import (
	"testing"

	"github.com/231397220/nexus-cli/internal/config"
	"github.com/231397220/nexus-cli/internal/nexus"
)

type fakeClient struct {
	repos   []nexus.Repository
	current *nexus.RawHostedRepository
	created []nexus.RawHostedRepository
	updated []nexus.RawHostedRepository
}

func (f *fakeClient) ListRepositories() ([]nexus.Repository, error) { return f.repos, nil }
func (f *fakeClient) GetRawHostedRepository(string) (*nexus.RawHostedRepository, error) {
	copy := *f.current
	return &copy, nil
}
func (f *fakeClient) CreateRawHostedRepository(r nexus.RawHostedRepository) error {
	f.created = append(f.created, r)
	return nil
}
func (f *fakeClient) UpdateRawHostedRepository(r nexus.RawHostedRepository) error {
	f.updated = append(f.updated, r)
	return nil
}

func desired() config.RawRepository {
	return config.RawRepository{
		Name: "releases", Online: true, ContentDisposition: "attachment",
		Storage: config.RawStorage{
			BlobStoreName: "default", StrictContentTypeValidation: true, WritePolicy: "allow_once",
		},
	}
}

func TestEnsureCreateAndDryRun(t *testing.T) {
	f := &fakeClient{}
	result, err := New().Ensure(f, desired(), true)
	if err != nil {
		t.Fatal(err)
	}
	if result.Action != ActionCreate || len(f.created) != 0 {
		t.Fatalf("result=%+v created=%d", result, len(f.created))
	}
	result, err = New().Ensure(f, desired(), false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Action != ActionCreate || len(f.created) != 1 {
		t.Fatalf("result=%+v created=%d", result, len(f.created))
	}
	if f.created[0].Storage.WritePolicy != "ALLOW_ONCE" || f.created[0].Raw.ContentDisposition != "ATTACHMENT" {
		t.Fatalf("Nexus enums were not normalized: %+v", f.created[0])
	}
}

func TestEnsureUpdatePreservesExtraSettings(t *testing.T) {
	current := nexus.RawHostedRepository{
		Name: "releases", Online: false,
		Storage: nexus.RepositoryStorage{BlobStoreName: "default", WritePolicy: "allow"},
		Raw:     nexus.RawSettings{ContentDisposition: "inline"},
		Cleanup: &nexus.CleanupSettings{PolicyNames: []string{"manual"}},
	}
	f := &fakeClient{
		repos:   []nexus.Repository{{Name: "releases", Format: "raw", Type: "hosted"}},
		current: &current,
	}
	result, err := New().Ensure(f, desired(), false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Action != ActionUpdate || len(f.updated) != 1 {
		t.Fatalf("result=%+v updates=%d", result, len(f.updated))
	}
	if got := f.updated[0].Cleanup.PolicyNames[0]; got != "manual" {
		t.Fatalf("cleanup policy lost: %q", got)
	}
}

func TestEnsureRejectsConflicts(t *testing.T) {
	t.Run("type", func(t *testing.T) {
		f := &fakeClient{repos: []nexus.Repository{{Name: "releases", Format: "maven2", Type: "hosted"}}}
		if _, err := New().Ensure(f, desired(), false); err == nil {
			t.Fatal("expected conflict")
		}
	})
	t.Run("blob store", func(t *testing.T) {
		current := nexus.RawHostedRepository{Storage: nexus.RepositoryStorage{BlobStoreName: "other"}}
		f := &fakeClient{
			repos: []nexus.Repository{{Name: "releases", Format: "raw", Type: "hosted"}}, current: &current,
		}
		if _, err := New().Ensure(f, desired(), false); err == nil {
			t.Fatal("expected conflict")
		}
	})
}
