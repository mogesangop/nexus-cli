package nexus

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestComponentsAPI(t *testing.T) {
	var query string
	var deletedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			query = r.URL.RawQuery
			token := "next"
			_ = json.NewEncoder(w).Encode(ComponentPage{
				Items:             []Component{{ID: "component-id", Format: "raw"}},
				ContinuationToken: &token,
			})
		case http.MethodDelete:
			deletedPath = r.URL.Path
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer server.Close()
	client := New(server.URL, "admin", "secret", 5, false)

	page, err := client.ListComponents("raw releases", "token/value")
	if err != nil || len(page.Items) != 1 || *page.ContinuationToken != "next" {
		t.Fatalf("ListComponents: page=%+v err=%v", page, err)
	}
	if query != "continuationToken=token%2Fvalue&repository=raw+releases" {
		t.Fatalf("query=%q", query)
	}
	if err := client.DeleteComponent("id/value"); err != nil {
		t.Fatal(err)
	}
	if deletedPath != "/service/rest/v1/components/id/value" {
		t.Fatalf("delete path=%q", deletedPath)
	}
}
