package guest

import (
	"fmt"

	"github.com/231397220/nexus-cli/internal/config"
	"github.com/231397220/nexus-cli/internal/naming"
	"github.com/231397220/nexus-cli/internal/nexus"
	"github.com/231397220/nexus-cli/internal/report"
)

// Syncer applies a computed plan to Nexus. It is idempotent (PRD 14): a second
// run with unchanged state creates nothing and removes nothing.
type Syncer struct {
	cfg   *config.Config
	namer *naming.Generator
	plan  *Planner
}

// NewSyncer returns a Syncer.
func NewSyncer(cfg *config.Config) *Syncer {
	return &Syncer{
		cfg:   cfg,
		namer: naming.New(cfg.PrivilegeNaming),
		plan:  NewPlanner(cfg),
	}
}

// PlanAndSync reads Nexus state, computes a plan, and applies it unless dryRun
// is true. It returns the plan (for reporting/audit) and an error on failure.
//
// Per PRD 18.2 the first version aborts on the first privilege-create error
// rather than continuing. A subsequent --continue-on-error flag may be added
// in a later version.
func (s *Syncer) PlanAndSync(client *nexus.Client, dryRun bool) (*SyncPlan, *report.SyncReport, error) {
	repos, err := client.ListRepositories()
	if err != nil {
		return nil, nil, err
	}

	role, err := client.GetRole(s.cfg.GuestAccess.RoleName)
	if err != nil {
		if !nexus.IsNotFound(err) {
			return nil, nil, fmt.Errorf("read guest role: %w", err)
		}
		role = nil
	}

	var existingAll []string
	var existingManaged []string
	if role != nil {
		existingAll = append([]string(nil), role.Privileges...)
		for _, p := range role.Privileges {
			if s.namer.IsManaged(p) {
				existingManaged = append(existingManaged, p)
			}
		}
	}

	plan := s.plan.Build(repos, existingManaged, existingAll)

	if !dryRun {
		if err := s.apply(client, plan, role); err != nil {
			return plan, toReport(plan, dryRun), err
		}
	}
	return plan, toReport(plan, dryRun), nil
}

// apply creates missing privileges, reconciles the role's privilege set, and
// removes forbidden privileges. Managed-but-unwanted privileges are removed;
// non-managed, non-forbidden privileges are preserved (PRD 12.3).
func (s *Syncer) apply(client *nexus.Client, plan *SyncPlan, role *nexus.Role) error {
	// 1. Create missing privileges.
	created := make(map[string]struct{}, len(plan.PrivilegesToCreate))
	for _, w := range plan.PrivilegesToCreate {
		if _, err := client.CreateRepositoryViewPrivilege(w.Name, w.Format, w.Repository, w.Actions); err != nil {
			return fmt.Errorf("create privilege %s: %w", w.Name, err)
		}
		created[w.Name] = struct{}{}
	}

	// 2. Compute the target privilege set for the role.
	//    Start from existing managed privileges, drop removed ones, add created
	//    ones, and drop forbidden ones (managed or not).
	var desired []string
	existingManaged := make(map[string]struct{})
	for _, p := range plan.PrivilegesToSkip {
		desired = append(desired, p)
		existingManaged[p] = struct{}{}
	}
	for _, p := range plan.PrivilegesToRemove {
		// these are managed + unwanted; do not carry forward.
		_ = p
	}
	for _, w := range plan.PrivilegesToCreate {
		desired = append(desired, w.Name)
	}
	// Preserve non-managed, non-forbidden existing privileges.
	if role != nil {
		for _, p := range role.Privileges {
			if s.namer.IsManaged(p) {
				continue // managed set already reconciled above
			}
			if matchesAny(s.cfg.GuestAccess.ForbiddenPrivileges, p) {
				continue // forbidden -> drop
			}
			desired = append(desired, p)
		}
	}
	desired = dedup(desired)

	// 3. Create or update the role.
	if role == nil {
		newRole := &nexus.Role{
			ID:          s.cfg.GuestAccess.RoleName,
			Name:        s.cfg.GuestAccess.RoleName,
			Description: "managed by nexus-cli",
			Privileges:  desired,
		}
		if _, err := client.CreateRole(newRole); err != nil {
			return fmt.Errorf("create role: %w", err)
		}
	} else {
		updated := *role
		updated.Privileges = desired
		if err := client.UpdateRole(s.cfg.GuestAccess.RoleName, &updated); err != nil {
			return fmt.Errorf("update role: %w", err)
		}
	}

	// 4. Ensure the anonymous user is actually governed by the target role.
	if err := s.reconcileAnonymousUser(client); err != nil {
		return err
	}
	return nil
}

func (s *Syncer) reconcileAnonymousUser(client *nexus.Client) error {
	userID := s.cfg.GuestAccess.AnonymousUserID
	if userID == "" {
		return nil
	}
	user, err := client.GetUser(userID)
	if err != nil {
		return fmt.Errorf("read anonymous user %s: %w", userID, err)
	}

	roles := make([]string, 0, len(user.Roles)+1)
	for _, roleID := range user.Roles {
		if roleID == s.cfg.GuestAccess.RoleName {
			roles = append(roles, roleID)
			continue
		}
		risky, err := s.roleHasForbiddenPrivilege(client, roleID, map[string]bool{})
		if err != nil {
			return fmt.Errorf("inspect anonymous role %s: %w", roleID, err)
		}
		if risky {
			continue
		}
		roles = append(roles, roleID)
	}
	if !contains(roles, s.cfg.GuestAccess.RoleName) {
		roles = append(roles, s.cfg.GuestAccess.RoleName)
	}
	roles = dedup(roles)
	if sameStringSet(user.Roles, roles) {
		return nil
	}

	updated := *user
	updated.Roles = roles
	if err := client.UpdateUser(userID, &updated); err != nil {
		return fmt.Errorf("update anonymous user %s roles: %w", userID, err)
	}
	return nil
}

func (s *Syncer) roleHasForbiddenPrivilege(client *nexus.Client, roleID string, seen map[string]bool) (bool, error) {
	if seen[roleID] {
		return false, nil
	}
	seen[roleID] = true

	role, err := client.GetRole(roleID)
	if err != nil {
		return false, err
	}
	for _, privilege := range role.Privileges {
		if matchesAny(s.cfg.GuestAccess.ForbiddenPrivileges, privilege) {
			return true, nil
		}
	}
	for _, nested := range role.Roles {
		risky, err := s.roleHasForbiddenPrivilege(client, nested, seen)
		if err != nil {
			return false, err
		}
		if risky {
			return true, nil
		}
	}
	return false, nil
}

func toReport(plan *SyncPlan, dryRun bool) *report.SyncReport {
	r := &report.SyncReport{
		DryRun:                   dryRun,
		TargetRole:               plan.TargetRole,
		RepositoriesTotal:        plan.RepositoriesTotal,
		PublicRepositories:       plan.PublicRepositories,
		DownloadOnlyRepositories: plan.DownloadOnlyRepositories,
		ProtectedRepositories:    plan.ProtectedRepositories,
		PrivilegesToSkip:         plan.PrivilegesToSkip,
		PrivilegesToRemove:       plan.PrivilegesToRemove,
		RemovedRiskyPrivileges:   plan.RemovedRiskyPrivileges,
		Warnings:                 plan.Warnings,
	}
	for _, w := range plan.PrivilegesToCreate {
		r.PrivilegesToCreate = append(r.PrivilegesToCreate, w.Name)
	}
	return r
}

func dedup(in []string) []string {
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

func sameStringSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := make(map[string]int, len(a))
	for _, v := range a {
		seen[v]++
	}
	for _, v := range b {
		seen[v]--
		if seen[v] < 0 {
			return false
		}
	}
	return true
}
