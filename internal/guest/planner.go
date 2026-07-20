// Package guest implements the guest/anonymous permission sync engine.
//
// It computes a SyncPlan from the config + live repository list, applies it
// idempotently (syncer), and verifies drift (checker).
package guest

import (
	"sort"

	"github.com/231397220/nexus-cli/internal/config"
	"github.com/231397220/nexus-cli/internal/naming"
	"github.com/231397220/nexus-cli/internal/nexus"
)

// Policy is the resolved access policy for a single repository.
type Policy string

const (
	PolicyProtected    Policy = "protected"
	PolicyDownloadOnly Policy = "downloadOnly"
	PolicyPublic       Policy = "public"
	PolicyNone         Policy = "none" // legacy defaultPolicy: none
)

// TargetPermission is the desired permission for one repository.
type TargetPermission struct {
	Repository string
	Format     string
	Actions    []string
	Policy     Policy
}

// TargetPrivilege is a privilege the guest role should have after sync.
type TargetPrivilege struct {
	Name       string
	Format     string
	Repository string
	Actions    []string
}

// SyncPlan is the computed set of changes between config and Nexus state.
type SyncPlan struct {
	TargetRole               string
	RepositoriesTotal        int
	PublicRepositories       []string
	DownloadOnlyRepositories []string
	ProtectedRepositories    []string
	PrivilegesToCreate       []TargetPrivilege
	PrivilegesToSkip         []string
	PrivilegesToRemove       []string
	RemovedRiskyPrivileges   []string
	Warnings                 []string
}

// Planner computes target permissions and a SyncPlan.
type Planner struct {
	cfg   *config.Config
	namer *naming.Generator
}

// NewPlanner returns a Planner.
func NewPlanner(cfg *config.Config) *Planner {
	return &Planner{
		cfg:   cfg,
		namer: naming.New(cfg.PrivilegeNaming),
	}
}

// PolicyFor resolves the policy for a single repository. Protected always wins
// so an accidental duplicate entry can never grant guest access.
func (p *Planner) PolicyFor(repo string) Policy {
	if p.usesLegacyPolicies() {
		return p.legacyPolicyFor(repo)
	}
	if matchesAny(p.cfg.GuestAccess.Protected.Repositories, repo) {
		return PolicyProtected
	}
	if matchesAny(p.cfg.GuestAccess.DownloadOnly.Repositories, repo) {
		return PolicyDownloadOnly
	}
	if matchesAny(p.cfg.GuestAccess.Public.Repositories, repo) {
		return PolicyPublic
	}
	switch p.cfg.GuestAccess.DefaultPolicy {
	case "protected":
		return PolicyProtected
	default:
		return PolicyPublic
	}
}

func (p *Planner) usesLegacyPolicies() bool {
	g := p.cfg.GuestAccess
	return len(g.BrowseRead.IncludeRepositories) > 0 || len(g.BrowseRead.ExcludeRepositories) > 0 ||
		len(g.ReadOnly.Repositories) > 0 || len(g.Deny.Repositories) > 0 ||
		g.DefaultPolicy == "browseRead" || g.DefaultPolicy == "none"
}

func (p *Planner) legacyPolicyFor(repo string) Policy {
	if contains(p.cfg.GuestAccess.Deny.Repositories, repo) {
		return PolicyProtected
	}
	if contains(p.cfg.GuestAccess.ReadOnly.Repositories, repo) {
		return PolicyDownloadOnly
	}
	excluded := contains(p.cfg.GuestAccess.BrowseRead.ExcludeRepositories, repo)
	included := matchInclude(p.cfg.GuestAccess.BrowseRead.IncludeRepositories, repo)
	if included && !excluded {
		return PolicyPublic
	}
	switch p.cfg.GuestAccess.DefaultPolicy {
	case "none":
		return PolicyNone
	default:
		return PolicyPublic
	}
}

// ActionsFor returns the Nexus actions granted for a policy.
func (p *Planner) ActionsFor(pol Policy) []string {
	switch pol {
	case PolicyPublic:
		return []string{"browse", "read"}
	case PolicyDownloadOnly:
		return []string{"read"}
	default:
		return nil
	}
}

// ComputeTargets maps repositories to target permissions, skipping deny/none.
func (p *Planner) ComputeTargets(repos []nexus.Repository) []TargetPermission {
	out := make([]TargetPermission, 0, len(repos))
	for _, r := range repos {
		pol := p.PolicyFor(r.Name)
		if pol == PolicyProtected || pol == PolicyNone {
			continue
		}
		out = append(out, TargetPermission{
			Repository: r.Name,
			Format:     r.Format,
			Actions:    p.ActionsFor(pol),
			Policy:     pol,
		})
	}
	return out
}

// TargetPrivileges builds the desired privilege set for the given targets.
func (p *Planner) TargetPrivileges(targets []TargetPermission) []TargetPrivilege {
	out := make([]TargetPrivilege, 0, len(targets))
	for _, t := range targets {
		out = append(out, TargetPrivilege{
			Name:       p.namer.PrivilegeName(t.Format, t.Repository, t.Actions),
			Format:     t.Format,
			Repository: t.Repository,
			Actions:    t.Actions,
		})
	}
	return out
}

// Build computes a SyncPlan given the live role privileges. ExistingManaged is
// the set of privilege names currently on the guest role that match the
// managed prefix. ExistingAll is every privilege name on the role (used to
// detect forbidden privileges to remove).
func (p *Planner) Build(repos []nexus.Repository, existingManaged, existingAll []string) *SyncPlan {
	targets := p.ComputeTargets(repos)
	want := p.TargetPrivileges(targets)

	wantByName := make(map[string]TargetPrivilege, len(want))
	for _, w := range want {
		wantByName[w.Name] = w
	}

	existingSet := make(map[string]struct{}, len(existingManaged))
	for _, e := range existingManaged {
		existingSet[e] = struct{}{}
	}

	var toCreate []TargetPrivilege
	var toSkip []string
	for _, w := range want {
		if _, ok := existingSet[w.Name]; ok {
			toSkip = append(toSkip, w.Name)
		} else {
			toCreate = append(toCreate, w)
		}
	}

	// Managed privileges on the role that are no longer wanted -> remove.
	var toRemove []string
	for _, e := range existingManaged {
		if _, ok := wantByName[e]; !ok {
			toRemove = append(toRemove, e)
		}
	}

	// Forbidden privileges (managed or not) -> remove. PRD 12.3 + 13.
	allSet := make(map[string]struct{}, len(existingAll))
	for _, e := range existingAll {
		allSet[e] = struct{}{}
	}
	var risky []string
	for e := range allSet {
		if matchesAny(p.cfg.GuestAccess.ForbiddenPrivileges, e) {
			risky = append(risky, e)
		}
	}

	plan := &SyncPlan{
		TargetRole:             p.cfg.GuestAccess.RoleName,
		RepositoriesTotal:      len(repos),
		PrivilegesToCreate:     toCreate,
		PrivilegesToSkip:       toSkip,
		PrivilegesToRemove:     toRemove,
		RemovedRiskyPrivileges: risky,
	}
	for _, r := range repos {
		switch p.PolicyFor(r.Name) {
		case PolicyPublic:
			plan.PublicRepositories = append(plan.PublicRepositories, r.Name)
		case PolicyDownloadOnly:
			plan.DownloadOnlyRepositories = append(plan.DownloadOnlyRepositories, r.Name)
		case PolicyProtected:
			plan.ProtectedRepositories = append(plan.ProtectedRepositories, r.Name)
		}
	}
	for _, w := range p.cfg.GuestAccess.WarnPrivileges {
		for e := range allSet {
			if !matchesAny([]string{w}, e) {
				continue
			}
			plan.Warnings = append(plan.Warnings, w+" exists; UI search may expose artifacts")
			break
		}
	}
	sort.Strings(plan.PublicRepositories)
	sort.Strings(plan.DownloadOnlyRepositories)
	sort.Strings(plan.ProtectedRepositories)
	return plan
}

// matchInclude reports whether repo matches any include pattern. A single
// element "*" matches all. Otherwise exact match is required.
func matchInclude(patterns []string, repo string) bool {
	if len(patterns) == 0 {
		return false
	}
	for _, p := range patterns {
		if p == "*" {
			return true
		}
		if p == repo {
			return true
		}
	}
	return false
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func cloneStrings(in []string) []string {
	if in == nil {
		return nil
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}
