package guest

import (
	"fmt"
	"sort"

	"github.com/moge/nexus-cli/internal/config"
	"github.com/moge/nexus-cli/internal/naming"
	"github.com/moge/nexus-cli/internal/nexus"
	"github.com/moge/nexus-cli/internal/report"
)

// Checker verifies that the live guest role matches the config (PRD 8.6).
// It never mutates Nexus.
type Checker struct {
	cfg   *config.Config
	namer *naming.Generator
	plan  *Planner
}

// NewChecker returns a Checker.
func NewChecker(cfg *config.Config) *Checker {
	return &Checker{
		cfg:   cfg,
		namer: naming.New(cfg.PrivilegeNaming),
		plan:  NewPlanner(cfg),
	}
}

// Check inspects Nexus and returns a check report.
func (c *Checker) Check(client *nexus.Client) (*report.CheckReport, error) {
	out := &report.CheckReport{TargetRole: c.cfg.GuestAccess.RoleName}

	role, err := client.GetRole(c.cfg.GuestAccess.RoleName)
	if err != nil {
		if nexus.IsNotFound(err) {
			out.Fails = append(out.Fails, fmt.Sprintf("target role %s does not exist", c.cfg.GuestAccess.RoleName))
			return out, nil
		}
		return nil, fmt.Errorf("read guest role: %w", err)
	}

	privSet := make(map[string]struct{}, len(role.Privileges))
	for _, p := range role.Privileges {
		privSet[p] = struct{}{}
	}

	// Forbidden privileges -> FAIL if present.
	for _, f := range c.cfg.GuestAccess.ForbiddenPrivileges {
		if _, ok := privSet[f]; ok {
			out.Fails = append(out.Fails, fmt.Sprintf("%s exists (forbidden)", f))
		} else {
			out.Passes = append(out.Passes, fmt.Sprintf("no %s", f))
		}
	}

	// Warn privileges -> WARN if present.
	for _, w := range c.cfg.GuestAccess.WarnPrivileges {
		if _, ok := privSet[w]; ok {
			out.Warns = append(out.Warns, fmt.Sprintf("%s exists, UI search may expose artifacts", w))
		}
	}

	// Per-repository checks.
	repos, err := client.ListRepositories()
	if err != nil {
		return nil, fmt.Errorf("list repositories: %w", err)
	}
	for _, r := range repos {
		pol := c.plan.PolicyFor(r.Name)
		switch pol {
		case PolicyDeny:
			// ensure no managed privilege grants access
			name := c.namer.PrivilegeName(r.Format, r.Name, c.plan.ActionsFor(PolicyReadOnly))
			if _, ok := privSet[name]; ok {
				out.Fails = append(out.Fails, fmt.Sprintf("%s is denied but has read privilege", r.Name))
			}
		case PolicyReadOnly:
			readName := c.namer.PrivilegeName(r.Format, r.Name, c.plan.ActionsFor(PolicyReadOnly))
			browseName := c.namer.PrivilegeName(r.Format, r.Name, []string{"browse"})
			if _, ok := privSet[readName]; ok {
				out.Passes = append(out.Passes, fmt.Sprintf("%s has read permission", r.Name))
			} else {
				out.Fails = append(out.Fails, fmt.Sprintf("%s missing read permission", r.Name))
			}
			if _, ok := privSet[browseName]; ok {
				out.Fails = append(out.Fails, fmt.Sprintf("%s has browse permission (should not)", r.Name))
			} else {
				out.Passes = append(out.Passes, fmt.Sprintf("%s has no browse permission", r.Name))
			}
		case PolicyBrowseRead:
			name := c.namer.PrivilegeName(r.Format, r.Name, c.plan.ActionsFor(PolicyBrowseRead))
			if _, ok := privSet[name]; ok {
				out.Passes = append(out.Passes, fmt.Sprintf("%s has browse+read permission", r.Name))
			} else {
				out.Warns = append(out.Warns, fmt.Sprintf("%s missing browse+read privilege %s", r.Name, name))
			}
		}
	}

	sort.Strings(out.Passes)
	sort.Strings(out.Warns)
	sort.Strings(out.Fails)
	return out, nil
}
