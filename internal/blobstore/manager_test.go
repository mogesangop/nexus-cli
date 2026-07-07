package blobstore

import (
	"testing"

	"github.com/231397220/nexus-cli/internal/config"
	"github.com/231397220/nexus-cli/internal/nexus"
)

type fakeClient struct {
	stores  []nexus.BlobStore
	current *nexus.FileBlobStore
	created []nexus.FileBlobStore
	updated []nexus.FileBlobStore
}

func (f *fakeClient) ListBlobStores() ([]nexus.BlobStore, error) { return f.stores, nil }
func (f *fakeClient) GetFileBlobStore(string) (*nexus.FileBlobStore, error) {
	copy := *f.current
	return &copy, nil
}
func (f *fakeClient) CreateFileBlobStore(store nexus.FileBlobStore) error {
	f.created = append(f.created, store)
	return nil
}
func (f *fakeClient) UpdateFileBlobStore(store nexus.FileBlobStore) error {
	f.updated = append(f.updated, store)
	return nil
}

func fileDesired() config.FileBlobStore {
	return config.FileBlobStore{Name: "default", Path: "/nexus-data/blobs/default"}
}

func TestEnsureFileCreateAndDryRun(t *testing.T) {
	f := &fakeClient{}
	result, err := New().EnsureFile(f, fileDesired(), true)
	if err != nil {
		t.Fatal(err)
	}
	if result.Action != ActionCreate || len(f.created) != 0 {
		t.Fatalf("result=%+v created=%d", result, len(f.created))
	}
	result, err = New().EnsureFile(f, fileDesired(), false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Action != ActionCreate || len(f.created) != 1 {
		t.Fatalf("result=%+v created=%d", result, len(f.created))
	}
}

func TestEnsureFileUnchangedAndUpdate(t *testing.T) {
	current := nexus.FileBlobStore{Name: "default", Path: "/nexus-data/blobs/default"}
	f := &fakeClient{stores: []nexus.BlobStore{{Name: "default", Type: "file"}}, current: &current}
	result, err := New().EnsureFile(f, fileDesired(), false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Action != ActionUnchanged || len(f.updated) != 0 {
		t.Fatalf("result=%+v updated=%d", result, len(f.updated))
	}

	desired := fileDesired()
	desired.Path = "/data/blobs/default"
	result, err = New().EnsureFile(f, desired, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Action != ActionUpdate || len(f.updated) != 1 {
		t.Fatalf("result=%+v updated=%d", result, len(f.updated))
	}
}

func TestEnsureFileRejectsTypeConflict(t *testing.T) {
	f := &fakeClient{stores: []nexus.BlobStore{{Name: "default", Type: "s3"}}}
	if _, err := New().EnsureFile(f, fileDesired(), false); err == nil {
		t.Fatal("expected conflict")
	}
}
