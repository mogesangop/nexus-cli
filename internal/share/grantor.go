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
	"strings"

	"github.com/moge/nexus-cli/internal/nexus"
)

// NexusAPI is the subset of the Nexus client the Grantor needs. *nexus.Client
// satisfies it implicitly; tests inject a fake.
type NexusAPI interface {
	ListRepositories() ([]nexus.Repository, error)
	GetContentSelector(name string) (*nexus.ContentSelector, error)
	CreateContentSelector(name, expression string) (*nexus.ContentSelector, error)
	GetPrivilege(name string) (*nexus.Privilege, error)
	CreateRepositoryContentSelectorPrivilege(name, format, repo, selector string, actions []string) (*nexus.Privilege, error)
	GetRole(id string) (*nexus.Role, error)
	CreateRole(r *nexus.Role) (*nexus.Role, error)
	UpdateRole(id string, r *nexus.Role) error
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
	Repo              string
	Format            string
	Path              string
	Selector          string
	Privilege         string
	Role              string
	User              string
	SelectorCreated   bool
	PrivilegeCreated  bool
	RoleCreated       bool
	UserCreated       bool
	PasswordSet       bool
	Password          string
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
		Repo:  req.Repo,
		Path:  normalizePath(req.Path),
		User:  req.UserID,
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

	res.Selector = selectorName(req.Repo, res.Path)
	res.Privilege = privilegeName(format, req.Repo)
	res.Role = roleName(req.Repo, res.Path)

	if req.DryRun {
		return res, nil
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

	// 4. User.
	if _, err := client.GetUser(res.User); err != nil {
		if !nexus.IsNotFound(err) {
			return nil, fmt.Errorf("get user %s: %w", res.User, err)
		}
	} else {
		return nil, fmt.Errorf("%w: %s", ErrUserExists, res.User)
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

func privilegeName(format, repo string) string {
	return privilegePrefix + sanitize(format) + "_" + sanitize(repo) + "_browse_read"
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
