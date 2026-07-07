package nexus

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRawHostedRepositoryAPI(t *testing.T) {
	var methods []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method+" "+r.URL.Path)
		switch {
		case r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(RawHostedRepository{
				Name: "release files", Online: true,
				Storage: RepositoryStorage{BlobStoreName: "default", WritePolicy: "allow_once"},
				Raw:     RawSettings{ContentDisposition: "attachment"},
			})
		case r.Method == http.MethodPost:
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodPut:
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer server.Close()
	client := New(server.URL, "admin", "secret", 5, false)

	repo, err := client.GetRawHostedRepository("release files")
	if err != nil || repo.Name != "release files" {
		t.Fatalf("GetRawHostedRepository: repo=%+v err=%v", repo, err)
	}
	if err := client.CreateRawHostedRepository(*repo); err != nil {
		t.Fatal(err)
	}
	if err := client.UpdateRawHostedRepository(*repo); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"GET /service/rest/v1/repositories/raw/hosted/release files",
		"POST /service/rest/v1/repositories/raw/hosted",
		"PUT /service/rest/v1/repositories/raw/hosted/release files",
	}
	for i := range want {
		if methods[i] != want[i] {
			t.Fatalf("request[%d]=%q want %q", i, methods[i], want[i])
		}
	}
}

func TestGenericRepositoryAPI(t *testing.T) {
	var methods []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method+" "+r.URL.Path)
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{"name": "release files", "online": true})
		case http.MethodPost:
			w.WriteHeader(http.StatusCreated)
		case http.MethodPut:
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer server.Close()
	client := New(server.URL, "admin", "secret", 5, false)

	repo, err := client.GetRepository("npm", "hosted", "release files")
	if err != nil || repo["name"] != "release files" {
		t.Fatalf("GetRepository: repo=%+v err=%v", repo, err)
	}
	body := map[string]any{"name": "release files", "online": true}
	if err := client.CreateRepository("npm", "hosted", body); err != nil {
		t.Fatal(err)
	}
	if err := client.UpdateRepository("npm", "hosted", "release files", body); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"GET /service/rest/v1/repositories/npm/hosted/release files",
		"POST /service/rest/v1/repositories/npm/hosted",
		"PUT /service/rest/v1/repositories/npm/hosted/release files",
	}
	for i := range want {
		if methods[i] != want[i] {
			t.Fatalf("request[%d]=%q want %q", i, methods[i], want[i])
		}
	}
}
