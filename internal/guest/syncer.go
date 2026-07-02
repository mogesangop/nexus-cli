package guest

import (
	"fmt"

	"github.com/moge/nexus-cli/internal/config"
	"github.com/moge/nexus-cli/internal/naming"
	"github.com/moge/nexus-cli/internal/nexus"
	"github.com/moge/nexus-cli/internal/report"
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
			if contains(s.cfg.GuestAccess.ForbiddenPrivileges, p) {
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
		return nil
	}
	updated := *role
	updated.Privileges = desired
	if err := client.UpdateRole(s.cfg.GuestAccess.RoleName, &updated); err != nil {
		return fmt.Errorf("update role: %w", err)
	}
	return nil
}

func toReport(plan *SyncPlan, dryRun bool) *report.SyncReport {
	r := &report.SyncReport{
		DryRun:                 dryRun,
		TargetRole:             plan.TargetRole,
		RepositoriesTotal:      plan.RepositoriesTotal,
		BrowseReadRepositories: plan.BrowseReadRepositories,
		ReadOnlyRepositories:   plan.ReadOnlyRepositories,
		DenyRepositories:       plan.DenyRepositories,
		PrivilegesToSkip:       plan.PrivilegesToSkip,
		PrivilegesToRemove:     plan.PrivilegesToRemove,
		RemovedRiskyPrivileges: plan.RemovedRiskyPrivileges,
		Warnings:               plan.Warnings,
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
