// Package share implements the imperative one-shot "grant a named user
// browse+download access to a directory in a repository" flow. It orchestrates
// four Nexus resources — content selector, repository-content-selector
// privilege, role, user — and is fully idempotent on re-run.
//
// Share privileges use a separate `priv_share_` prefix and live on their own
// role, so they are invisible to the guest subsystem (naming.IsManaged checks
// `priv_guest_`). The guest syncer never sees or touches them.
package share

import (
	"crypto/rand"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/231397220/nexus-cli/internal/nexus"
)

// NexusAPI is the subset of the Nexus client the Grantor needs. *nexus.Client
// satisfies it implicitly; tests inject a fake.
type NexusAPI interface {
	ListRepositories() ([]nexus.Repository, error)
	ListContentSelectors() ([]nexus.ContentSelector, error)
	GetContentSelector(name string) (*nexus.ContentSelector, error)
	CreateContentSelector(name, expression string) (*nexus.ContentSelector, error)
	ListPrivileges() ([]nexus.Privilege, error)
	GetPrivilege(name string) (*nexus.Privilege, error)
	CreateRepositoryContentSelectorPrivilege(name, format, repo, selector string, actions []string) (*nexus.Privilege, error)
	ListRoles() ([]nexus.Role, error)
	GetRole(id string) (*nexus.Role, error)
	CreateRole(r *nexus.Role) (*nexus.Role, error)
	UpdateRole(id string, r *nexus.Role) error
	ListUsers() ([]nexus.User, error)
	GetUser(userID string) (*nexus.User, error)
	CreateUser(u *nexus.User) (*nexus.User, error)
	SetPassword(userID, password string) error
}

// Request describes a single grant operation.
type Request struct {
	Repo           string // required
	Path           string // required, e.g. "/team-a/" (leading slash enforced)
	UserID         string // required
	FirstName      string
	LastName       string
	Email          string // required
	Format         string // optional; "" => auto-detect via ListRepositories
	PasswordLength int    // 0 => 24
	DryRun         bool
}

// Result captures what the Grantor did. Password is returned ONLY so the CLI
// can print it once; it never enters audit records or logs.
type Result struct {
	Repo             string
	Format           string
	Path             string
	Selector         string
	Privilege        string
	Role             string
	User             string
	SelectorCreated  bool
	PrivilegeCreated bool
	RoleCreated      bool
	UserCreated      bool
	PasswordSet      bool
	Password         string
	IsolationIssues  []string
}

const (
	privilegePrefix = "priv_share_"
	selectorPrefix  = "sel_share_"
	rolePrefix      = "role_share_"
	defaultActions  = "browse,read"
)

// ErrUserExists is returned when the requested user already exists. The
// Grantor refuses to reset an existing user's password; the caller should pick
// a different user or delete the existing one first.
var ErrUserExists = errors.New("user already exists; refusing to reset password (use a different --user or delete the user first)")

// ErrIsolationViolation is returned when another non-admin user already has
// access that could view or download the requested repository path.
var ErrIsolationViolation = errors.New("exclusive share preflight failed")

// IsolationError carries the concrete reasons that blocked an exclusive share.
type IsolationError struct {
	Issues []string
}

func (e *IsolationError) Error() string {
	if len(e.Issues) == 0 {
		return ErrIsolationViolation.Error()
	}
	return ErrIsolationViolation.Error() + ": " + strings.Join(e.Issues, "; ")
}

func (e *IsolationError) Unwrap() error { return ErrIsolationViolation }

// Grantor orchestrates the four-step grant flow.
type Grantor struct{}

// NewGrantor returns a Grantor.
func NewGrantor() *Grantor { return &Grantor{} }

// Grant performs the grant. It is idempotent: existing selector/privilege/role
// are reused; an existing user is an error. It does NOT roll back partial
// progress on failure — re-running is safe.
func (g *Grantor) Grant(client NexusAPI, req Request) (*Result, error) {
	if err := validate(req); err != nil {
		return nil, err
	}

	res := &Result{
		Repo: req.Repo,
		Path: normalizePath(req.Path),
		User: req.UserID,
	}

	format := req.Format
	if format == "" {
		f, err := detectFormat(client, req.Repo)
		if err != nil {
			return nil, err
		}
		format = f
	}
	res.Format = format
	if format != "raw" {
		return res, fmt.Errorf("user create-readonly supports only raw repositories (repo %q has format %q)", req.Repo, format)
	}

	res.Selector = selectorName(req.Repo, res.Path)
	res.Privilege = privilegeName(format, req.Repo, res.Path)
	res.Role = roleName(req.Repo, res.Path)

	if issues, err := g.checkExclusive(client, req.UserID, req.Repo, res.Path); err != nil {
		return res, err
	} else if len(issues) > 0 {
		res.IsolationIssues = issues
		return res, &IsolationError{Issues: issues}
	}

	if req.DryRun {
		return res, nil
	}

	if _, err := client.GetUser(res.User); err != nil {
		if !nexus.IsNotFound(err) {
			return nil, fmt.Errorf("get user %s: %w", res.User, err)
		}
	} else {
		return nil, fmt.Errorf("%w: %s", ErrUserExists, res.User)
	}

	expr := expression(res.Path)

	// 1. Content selector.
	if _, err := client.GetContentSelector(res.Selector); err != nil {
		if !nexus.IsNotFound(err) {
			return nil, fmt.Errorf("get content selector %s: %w", res.Selector, err)
		}
		if _, err := client.CreateContentSelector(res.Selector, expr); err != nil {
			return nil, fmt.Errorf("create content selector %s: %w", res.Selector, err)
		}
		res.SelectorCreated = true
	}

	// 2. Privilege.
	if _, err := client.GetPrivilege(res.Privilege); err != nil {
		if !nexus.IsNotFound(err) {
			return nil, fmt.Errorf("get privilege %s: %w", res.Privilege, err)
		}
		actions := strings.Split(defaultActions, ",")
		if _, err := client.CreateRepositoryContentSelectorPrivilege(res.Privilege, format, req.Repo, res.Selector, actions); err != nil {
			return nil, fmt.Errorf("create privilege %s: %w", res.Privilege, err)
		}
		res.PrivilegeCreated = true
	}

	// 3. Role.
	if existing, err := client.GetRole(res.Role); err != nil {
		if !nexus.IsNotFound(err) {
			return nil, fmt.Errorf("get role %s: %w", res.Role, err)
		}
		if _, err := client.CreateRole(&nexus.Role{
			ID:          res.Role,
			Name:        res.Role,
			Description: "managed by nexus-cli",
			Privileges:  []string{res.Privilege},
		}); err != nil {
			return nil, fmt.Errorf("create role %s: %w", res.Role, err)
		}
		res.RoleCreated = true
	} else if !contains(existing.Privileges, res.Privilege) {
		updated := append(append([]string{}, existing.Privileges...), res.Privilege)
		if err := client.UpdateRole(res.Role, &nexus.Role{
			ID:          res.Role,
			Name:        existing.Name,
			Description: existing.Description,
			Privileges:  updated,
			Roles:       existing.Roles,
		}); err != nil {
			return nil, fmt.Errorf("update role %s: %w", res.Role, err)
		}
	}

	pw, err := generatePassword(passwordLength(req.PasswordLength))
	if err != nil {
		return nil, fmt.Errorf("generate password: %w", err)
	}

	if _, err := client.CreateUser(&nexus.User{
		UserID:       res.User,
		FirstName:    req.FirstName,
		LastName:     req.LastName,
		EmailAddress: req.Email,
		Status:       "active",
		Roles:        []string{res.Role},
	}); err != nil {
		return nil, fmt.Errorf("create user %s: %w", res.User, err)
	}
	res.UserCreated = true

	if err := client.SetPassword(res.User, pw); err != nil {
		return nil, fmt.Errorf("set password for %s: %w", res.User, err)
	}
	res.PasswordSet = true
	res.Password = pw
	return res, nil
}

func (g *Grantor) checkExclusive(client NexusAPI, targetUser, repo, targetPath string) ([]string, error) {
	privileges, err := client.ListPrivileges()
	if err != nil {
		return nil, err
	}
	roles, err := client.ListRoles()
	if err != nil {
		return nil, err
	}
	users, err := client.ListUsers()
	if err != nil {
		return nil, err
	}
	selectors, err := client.ListContentSelectors()
	if err != nil {
		return nil, err
	}

	privByName := make(map[string]nexus.Privilege, len(privileges))
	for _, p := range privileges {
		privByName[p.Name] = p
	}
	roleByID := make(map[string]nexus.Role, len(roles))
	for _, r := range roles {
		roleByID[r.ID] = r
	}
	selectorByName := make(map[string]nexus.ContentSelector, len(selectors))
	for _, s := range selectors {
		selectorByName[s.Name] = s
	}

	var issues []string
	for _, user := range users {
		if user.UserID == targetUser || isAdminUser(user, roleByID) {
			continue
		}
		for _, privName := range collectPrivileges(user.Roles, roleByID, map[string]bool{}) {
			priv, ok := privByName[privName]
			if !ok {
				continue
			}
			if issue := privilegeConflict(user.UserID, priv, repo, targetPath, selectorByName); issue != "" {
				issues = append(issues, issue)
			}
		}
	}
	return dedupStrings(issues), nil
}

func validate(req Request) error {
	if req.Repo == "" {
		return errors.New("--repo is required")
	}
	if req.Path == "" {
		return errors.New("--path is required")
	}
	if !strings.HasPrefix(req.Path, "/") {
		return fmt.Errorf("--path must start with '/' (got %q)", req.Path)
	}
	if req.UserID == "" {
		return errors.New("--user is required")
	}
	if req.Email == "" {
		return errors.New("--email is required")
	}
	return nil
}

func normalizePath(p string) string {
	if !strings.HasSuffix(p, "/") {
		return p + "/"
	}
	return p
}

func expression(path string) string {
	return fmt.Sprintf("path ^= \"%s\"", path)
}

func detectFormat(client NexusAPI, repo string) (string, error) {
	repos, err := client.ListRepositories()
	if err != nil {
		return "", fmt.Errorf("list repositories: %w", err)
	}
	for _, r := range repos {
		if r.Name == repo {
			return r.Format, nil
		}
	}
	return "", fmt.Errorf("repository %q not found", repo)
}

func selectorName(repo, path string) string {
	return selectorPrefix + sanitize(repo) + "_" + hashPath(path)
}

func privilegeName(format, repo, path string) string {
	return privilegePrefix + sanitize(format) + "_" + sanitize(repo) + "_" + hashPath(path) + "_browse_read"
}

func roleName(repo, path string) string {
	return rolePrefix + sanitize(repo) + "_" + hashPath(path)
}

// sanitize replaces characters Nexus disallows in role/privilege ids with '_'.
func sanitize(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		b := s[i]
		switch {
		case b >= 'a' && b <= 'z', b >= 'A' && b <= 'Z', b >= '0' && b <= '9', b == '_':
			out = append(out, b)
		default:
			out = append(out, '_')
		}
	}
	return string(out)
}

// hashPath derives a short, deterministic suffix from the path so distinct
// paths under the same repo get distinct selectors.
func hashPath(path string) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	var sum [6]byte
	for i := 0; i < len(path); i++ {
		sum[i%6] ^= path[i]
	}
	out := make([]byte, 6)
	for i, b := range sum {
		out[i] = alphabet[int(b)%len(alphabet)]
	}
	return string(out)
}

func contains(slice []string, s string) bool {
	for _, x := range slice {
		if x == s {
			return true
		}
	}
	return false
}

func collectPrivileges(roleIDs []string, roleByID map[string]nexus.Role, seen map[string]bool) []string {
	var out []string
	for _, id := range roleIDs {
		if seen[id] {
			continue
		}
		seen[id] = true
		role, ok := roleByID[id]
		if !ok {
			continue
		}
		out = append(out, role.Privileges...)
		out = append(out, collectPrivileges(role.Roles, roleByID, seen)...)
	}
	return out
}

func isAdminUser(user nexus.User, roleByID map[string]nexus.Role) bool {
	if user.UserID == "admin" {
		return true
	}
	return hasAdminRole(user.Roles, roleByID, map[string]bool{})
}

func hasAdminRole(roleIDs []string, roleByID map[string]nexus.Role, seen map[string]bool) bool {
	for _, id := range roleIDs {
		if isAdminRoleID(id) {
			return true
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		role, ok := roleByID[id]
		if !ok {
			continue
		}
		if isAdminRoleID(role.ID) || isAdminRoleID(role.Name) {
			return true
		}
		for _, p := range role.Privileges {
			if p == "nx-admin" || p == "nx-all" {
				return true
			}
		}
		if hasAdminRole(role.Roles, roleByID, seen) {
			return true
		}
	}
	return false
}

func isAdminRoleID(id string) bool {
	switch strings.ToLower(id) {
	case "admin", "nx-admin":
		return true
	default:
		return false
	}
}

func privilegeConflict(userID string, priv nexus.Privilege, repo, targetPath string, selectors map[string]nexus.ContentSelector) string {
	if priv.Name == "nx-all" || priv.Name == "nx-admin" {
		return fmt.Sprintf("user %s has admin-level privilege %s", userID, priv.Name)
	}
	if !privGrantsAccess(priv) || !privAppliesToRepo(priv, repo) {
		return ""
	}
	switch priv.Type {
	case "repository-view":
		if userID == "anonymous" {
			return fmt.Sprintf("anonymous has access to %s via privilege %s; add %s to guestAccess.protected.repositories and run nexus-cli guest protect", repo, priv.Name, repo)
		}
		return fmt.Sprintf("user %s has repository-wide access via privilege %s; protect the repo with guest protect and remove broad non-admin grants first", userID, priv.Name)
	case "repository-content-selector":
		selectorName := privilegeContentSelector(priv)
		selector, ok := selectors[selectorName]
		if !ok {
			return fmt.Sprintf("user %s has selector privilege %s on repo %s but selector %q cannot be inspected", userID, priv.Name, repo, selectorName)
		}
		prefix, ok := selectorPathPrefix(selector.Expression)
		if !ok {
			return fmt.Sprintf("user %s has selector privilege %s on repo %s with unrecognized selector expression %q", userID, priv.Name, repo, selector.Expression)
		}
		if pathsOverlap(prefix, targetPath) {
			if userID == "anonymous" {
				return fmt.Sprintf("anonymous has access to %s via privilege %s; add %s to guestAccess.protected.repositories and run nexus-cli guest protect", repo, priv.Name, repo)
			}
			return fmt.Sprintf("user %s has overlapping path access via privilege %s (%s)", userID, priv.Name, prefix)
		}
	}
	return ""
}

func privGrantsAccess(priv nexus.Privilege) bool {
	actions := strings.Trim(priv.Properties["actions"], "[]")
	for _, action := range strings.Split(actions, ",") {
		switch strings.TrimSpace(action) {
		case "browse", "read", "*":
			return true
		}
	}
	return false
}

func privAppliesToRepo(priv nexus.Privilege, repo string) bool {
	got := privilegeRepo(priv)
	return got == repo || got == "*"
}

func privilegeRepo(priv nexus.Privilege) string {
	if priv.Properties == nil {
		return ""
	}
	return priv.Properties["repository"]
}

func privilegeContentSelector(priv nexus.Privilege) string {
	if priv.Properties == nil {
		return ""
	}
	if v := priv.Properties["contentSelector"]; v != "" {
		return v
	}
	return priv.Properties["contentSelectorName"]
}

var selectorPathPrefixRE = regexp.MustCompile(`^\s*path\s*\^=\s*"([^"]+)"\s*$`)

func selectorPathPrefix(expr string) (string, bool) {
	m := selectorPathPrefixRE.FindStringSubmatch(expr)
	if len(m) != 2 || !strings.HasPrefix(m[1], "/") {
		return "", false
	}
	return normalizePath(m[1]), true
}

func pathsOverlap(a, b string) bool {
	a = normalizePath(a)
	b = normalizePath(b)
	return strings.HasPrefix(a, b) || strings.HasPrefix(b, a)
}

func dedupStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func passwordLength(n int) int {
	if n <= 0 {
		return 24
	}
	return n
}

// passwordAlphabet excludes visually ambiguous characters (0/O/1/l/I).
const passwordAlphabet = "abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"

func generatePassword(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	for i := range buf {
		buf[i] = passwordAlphabet[int(buf[i])%len(passwordAlphabet)]
	}
	return string(buf), nil
}
