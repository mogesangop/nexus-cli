package nexus

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFileBlobStoreAPI(t *testing.T) {
	var methods []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method+" "+r.URL.Path)
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/service/rest/v1/blobstores":
			_ = json.NewEncoder(w).Encode([]BlobStore{{Name: "default", Type: "file"}})
		case r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(FileBlobStore{Name: "release files", Path: "/nexus-data/blobs/default"})
		case r.Method == http.MethodPost:
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodPut:
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer server.Close()
	client := New(server.URL, "admin", "secret", 5, false)

	stores, err := client.ListBlobStores()
	if err != nil || len(stores) != 1 {
		t.Fatalf("ListBlobStores: stores=%+v err=%v", stores, err)
	}
	store, err := client.GetFileBlobStore("release files")
	if err != nil || store.Name != "release files" {
		t.Fatalf("GetFileBlobStore: store=%+v err=%v", store, err)
	}
	if err := client.CreateFileBlobStore(*store); err != nil {
		t.Fatal(err)
	}
	if err := client.UpdateFileBlobStore(*store); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"GET /service/rest/v1/blobstores",
		"GET /service/rest/v1/blobstores/file/release files",
		"POST /service/rest/v1/blobstores/file",
		"PUT /service/rest/v1/blobstores/file/release files",
	}
	for i := range want {
		if methods[i] != want[i] {
			t.Fatalf("request[%d]=%q want %q", i, methods[i], want[i])
		}
	}
}
