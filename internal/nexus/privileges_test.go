package nexus

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestCreateRepositoryViewPrivilegeUsesTypedEndpoint(t *testing.T) {
	var gotPath string
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"priv_guest_raw_repo_read","type":"repository-view"}`))
	}))
	defer server.Close()

	client := New(server.URL, "admin", "secret", 30, false)
	if _, err := client.CreateRepositoryViewPrivilege("priv_guest_raw_repo_read", "raw", "repo", []string{"browse", "read"}); err != nil {
		t.Fatalf("CreateRepositoryViewPrivilege: %v", err)
	}

	if gotPath != "/service/rest/v1/security/privileges/repository-view" {
		t.Fatalf("path = %q, want repository-view endpoint", gotPath)
	}
	if gotBody["type"] != nil || gotBody["properties"] != nil {
		t.Fatalf("body should use typed endpoint schema, got %#v", gotBody)
	}
	if gotBody["format"] != "raw" || gotBody["repository"] != "repo" {
		t.Fatalf("unexpected repository fields: %#v", gotBody)
	}
	if got, want := stringsFromAny(gotBody["actions"]), []string{"browse", "read"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("actions = %#v, want %#v", got, want)
	}
}

func TestCreateRepositoryContentSelectorPrivilegeUsesTypedEndpoint(t *testing.T) {
	var gotPath string
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"priv_share_repo_path","type":"repository-content-selector"}`))
	}))
	defer server.Close()

	client := New(server.URL, "admin", "secret", 30, false)
	if _, err := client.CreateRepositoryContentSelectorPrivilege("priv_share_repo_path", "raw", "repo", "selector", []string{"browse", "read"}); err != nil {
		t.Fatalf("CreateRepositoryContentSelectorPrivilege: %v", err)
	}

	if gotPath != "/service/rest/v1/security/privileges/repository-content-selector" {
		t.Fatalf("path = %q, want repository-content-selector endpoint", gotPath)
	}
	if gotBody["contentSelector"] != "selector" {
		t.Fatalf("contentSelector = %#v, want selector", gotBody["contentSelector"])
	}
}

func stringsFromAny(v any) []string {
	raw, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		s, ok := item.(string)
		if !ok {
			return nil
		}
		out = append(out, s)
	}
	return out
}
