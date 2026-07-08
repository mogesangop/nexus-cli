package nexus

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestGetUserFallsBackToListUsersOnMethodNotAllowed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/service/rest/v1/security/users/anonymous":
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		case "/service/rest/v1/security/users":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"userId":"anonymous","source":"default","roles":["nx-anonymous"]}]`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	user, err := New(server.URL, "admin", "secret", 30, false).GetUser("anonymous")
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if user.UserID != "anonymous" || user.Source != "default" || !reflect.DeepEqual(user.Roles, []string{"nx-anonymous"}) {
		t.Fatalf("unexpected user: %#v", user)
	}
}

func TestUpdateUserIncludesSourceAndRoles(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Fatalf("method = %s, want PUT", r.Method)
		}
		if r.URL.Path != "/service/rest/v1/security/users/anonymous" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	err := New(server.URL, "admin", "secret", 30, false).UpdateUser("anonymous", &User{
		UserID:       "anonymous",
		FirstName:    "Anonymous",
		LastName:     "User",
		EmailAddress: "anonymous@example.org",
		Source:       "default",
		Status:       "active",
		Roles:        []string{"role_guest_repository_access"},
	})
	if err != nil {
		t.Fatalf("UpdateUser: %v", err)
	}
	if body["source"] != "default" || body["userId"] != "anonymous" {
		t.Fatalf("missing user identity fields: %#v", body)
	}
	if got, want := stringsFromAny(body["roles"]), []string{"role_guest_repository_access"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("roles = %#v, want %#v", got, want)
	}
}
