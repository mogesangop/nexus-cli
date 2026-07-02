package share

import (
	"strings"
	"testing"

	"github.com/moge/nexus-cli/internal/nexus"
)

func TestGrant_AllCreate(t *testing.T) {
	f := newFake()
	g := NewGrantor()
	res, err := g.Grant(f, Request{
		Repo: "my-raw-repo", Path: "/team-a/", UserID: "alice",
		FirstName: "Alice", LastName: "X", Email: "alice@x",
	})
	if err != nil {
		t.Fatalf("Grant: %v", err)
	}
	if !res.SelectorCreated || !res.PrivilegeCreated || !res.RoleCreated || !res.UserCreated || !res.PasswordSet {
		t.Fatalf("expected all created, got %+v", res)
	}
	if res.Password == "" {
		t.Fatal("expected non-empty password")
	}
	if len(res.Password) != 24 {
		t.Fatalf("password length = %d, want 24", len(res.Password))
	}
	if res.Format != "raw" {
		t.Fatalf("format = %q, want raw", res.Format)
	}
	if res.Path != "/team-a/" {
		t.Fatalf("path = %q, want /team-a/", res.Path)
	}
	if got := f.setPasswordCalls["alice"]; got != 1 {
		t.Fatalf("SetPassword calls for alice = %d, want 1", got)
	}
	if !strings.HasPrefix(res.Selector, "sel_share_") {
		t.Fatalf("selector name %q missing prefix", res.Selector)
	}
	if !strings.HasPrefix(res.Privilege, "priv_share_") {
		t.Fatalf("privilege name %q missing prefix", res.Privilege)
	}
	if !strings.HasPrefix(res.Role, "role_share_") {
		t.Fatalf("role name %q missing prefix", res.Role)
	}
}

func TestGrant_ReuseSelectorPrivilegeRole(t *testing.T) {
	f := newFake()
	g := NewGrantor()
	// First run creates everything.
	if _, err := g.Grant(f, Request{
		Repo: "my-raw-repo", Path: "/team-a/", UserID: "alice",
		Email: "alice@x",
	}); err != nil {
		t.Fatalf("first Grant: %v", err)
	}
	// Simulate partial-failure recovery: user deleted, but selector/priv/role
	// remain. Re-run with a fresh user.
	f.users = map[string]*nexus.User{}
	res, err := g.Grant(f, Request{
		Repo: "my-raw-repo", Path: "/team-a/", UserID: "bob",
		Email: "bob@x",
	})
	if err != nil {
		t.Fatalf("second Grant: %v", err)
	}
	if res.SelectorCreated || res.PrivilegeCreated || res.RoleCreated {
		t.Fatalf("expected reuse, got %+v", res)
	}
	if !res.UserCreated {
		t.Fatal("expected user created on second run")
	}
	if got := len(f.selectors); got != 1 {
		t.Fatalf("selectors count = %d, want 1 (no duplicate)", got)
	}
	if got := len(f.privileges); got != 1 {
		t.Fatalf("privileges count = %d, want 1 (no duplicate)", got)
	}
	if got := len(f.roles); got != 1 {
		t.Fatalf("roles count = %d, want 1 (no duplicate)", got)
	}
}

func TestGrant_UserExists_Error(t *testing.T) {
	f := newFake()
	f.users["alice"] = &nexus.User{UserID: "alice"}
	g := NewGrantor()
	_, err := g.Grant(f, Request{
		Repo: "my-raw-repo", Path: "/team-a/", UserID: "alice",
		Email: "alice@x",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errorsIs(err, ErrUserExists) {
		t.Fatalf("expected ErrUserExists, got %v", err)
	}
	if got := f.setPasswordCalls["alice"]; got != 0 {
		t.Fatalf("SetPassword calls = %d, want 0 (must not reset)", got)
	}
	if got := len(f.users); got != 1 {
		t.Fatalf("users count = %d, want 1 (no create)", got)
	}
}

func TestGrant_FormatAutoDetect(t *testing.T) {
	f := newFake()
	f.repos = []nexus.Repository{{Name: "npm-hosted", Format: "npm", Type: "hosted"}}
	g := NewGrantor()
	res, err := g.Grant(f, Request{
		Repo: "npm-hosted", Path: "/pkg/", UserID: "u",
		Email: "u@x",
	})
	if err != nil {
		t.Fatalf("Grant: %v", err)
	}
	if res.Format != "npm" {
		t.Fatalf("format = %q, want npm", res.Format)
	}
}

func TestGrant_DryRun(t *testing.T) {
	f := newFake()
	g := NewGrantor()
	res, err := g.Grant(f, Request{
		Repo: "my-raw-repo", Path: "/team-a/", UserID: "alice",
		Email: "alice@x", DryRun: true,
	})
	if err != nil {
		t.Fatalf("Grant: %v", err)
	}
	if res.SelectorCreated || res.PrivilegeCreated || res.RoleCreated || res.UserCreated || res.PasswordSet {
		t.Fatalf("dry-run must not report creates: %+v", res)
	}
	if res.Password != "" {
		t.Fatalf("dry-run must not generate password, got %q", res.Password)
	}
	if res.Selector == "" || res.Privilege == "" || res.Role == "" {
		t.Fatalf("dry-run should still populate planned names: %+v", res)
	}
	if got := len(f.selectors) + len(f.privileges) + len(f.roles) + len(f.users); got != 0 {
		t.Fatalf("dry-run mutated state: %d resources", got)
	}
}

func TestGrant_PathNormalization(t *testing.T) {
	f := newFake()
	g := NewGrantor()
	res, err := g.Grant(f, Request{
		Repo: "my-raw-repo", Path: "/foo", UserID: "alice",
		Email: "alice@x",
	})
	if err != nil {
		t.Fatalf("Grant: %v", err)
	}
	if res.Path != "/foo/" {
		t.Fatalf("normalized path = %q, want /foo/", res.Path)
	}
	sel := f.selectors[res.Selector]
	if sel == nil {
		t.Fatal("selector not created")
	}
	want := `path ^= "/foo/"`
	if sel.Expression != want {
		t.Fatalf("expression = %q, want %q", sel.Expression, want)
	}
}

// --- fake NexusAPI ---

type fakeAPI struct {
	repos           []nexus.Repository
	selectors       map[string]*nexus.ContentSelector
	privileges      map[string]*nexus.Privilege
	roles           map[string]*nexus.Role
	users           map[string]*nexus.User
	setPasswordCalls map[string]int
}

func newFake() *fakeAPI {
	return &fakeAPI{
		repos:            []nexus.Repository{{Name: "my-raw-repo", Format: "raw", Type: "hosted"}},
		selectors:        map[string]*nexus.ContentSelector{},
		privileges:       map[string]*nexus.Privilege{},
		roles:            map[string]*nexus.Role{},
		users:            map[string]*nexus.User{},
		setPasswordCalls: map[string]int{},
	}
}

func (f *fakeAPI) ListRepositories() ([]nexus.Repository, error) { return f.repos, nil }

func (f *fakeAPI) GetContentSelector(name string) (*nexus.ContentSelector, error) {
	if s, ok := f.selectors[name]; ok {
		return s, nil
	}
	return nil, &nexus.APIError{Status: 404}
}

func (f *fakeAPI) CreateContentSelector(name, expression string) (*nexus.ContentSelector, error) {
	s := &nexus.ContentSelector{Name: name, Type: "csel", Expression: expression}
	f.selectors[name] = s
	return s, nil
}

func (f *fakeAPI) GetPrivilege(name string) (*nexus.Privilege, error) {
	if p, ok := f.privileges[name]; ok {
		return p, nil
	}
	return nil, &nexus.APIError{Status: 404}
}

func (f *fakeAPI) CreateRepositoryContentSelectorPrivilege(name, format, repo, selector string, actions []string) (*nexus.Privilege, error) {
	p := &nexus.Privilege{Name: name, Type: "repository-content-selector"}
	f.privileges[name] = p
	return p, nil
}

func (f *fakeAPI) GetRole(id string) (*nexus.Role, error) {
	if r, ok := f.roles[id]; ok {
		return r, nil
	}
	return nil, &nexus.APIError{Status: 404}
}

func (f *fakeAPI) CreateRole(r *nexus.Role) (*nexus.Role, error) {
	cp := *r
	f.roles[r.ID] = &cp
	return &cp, nil
}

func (f *fakeAPI) UpdateRole(id string, r *nexus.Role) error {
	cp := *r
	f.roles[id] = &cp
	return nil
}

func (f *fakeAPI) GetUser(userID string) (*nexus.User, error) {
	if u, ok := f.users[userID]; ok {
		return u, nil
	}
	return nil, &nexus.APIError{Status: 404}
}

func (f *fakeAPI) CreateUser(u *nexus.User) (*nexus.User, error) {
	cp := *u
	f.users[u.UserID] = &cp
	return &cp, nil
}

func (f *fakeAPI) SetPassword(userID, password string) error {
	f.setPasswordCalls[userID]++
	return nil
}

// errorsIs avoids importing errors in the test file's import list churn; mirrors
// errors.Is semantics against the sentinel chain Grantor produces.
func errorsIs(err, target error) bool {
	return strings.Contains(err.Error(), target.Error())
}
