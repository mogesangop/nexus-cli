package guest

import (
	"testing"

	"github.com/231397220/nexus-cli/internal/config"
	"github.com/231397220/nexus-cli/internal/nexus"
)

func baseConfig() *config.Config {
	c := config.Default()
	// concrete repos so tests are deterministic
	c.GuestAccess.BrowseRead.ExcludeRepositories = []string{"devops-prod-generic"}
	c.GuestAccess.ReadOnly.Repositories = []string{"devops-prod-generic"}
	c.GuestAccess.Deny.Repositories = []string{"security-prod-generic"}
	return c
}

func repos() []nexus.Repository {
	return []nexus.Repository{
		{Name: "devops-prod-generic", Format: "raw", Type: "hosted"},
		{Name: "maven-public", Format: "maven2", Type: "group"},
		{Name: "npm-public", Format: "npm", Type: "group"},
		{Name: "security-prod-generic", Format: "raw", Type: "hosted"},
	}
}

func TestPolicyFor_PriorityDeny(t *testing.T) {
	p := NewPlanner(baseConfig())
	if got := p.PolicyFor("security-prod-generic"); got != PolicyDeny {
		t.Fatalf("deny repo got %v", got)
	}
}

func TestPolicyFor_ReadOnlyBeatsBrowseRead(t *testing.T) {
	p := NewPlanner(baseConfig())
	// devops-prod-generic is both readOnly and browseRead-excluded; readOnly wins
	if got := p.PolicyFor("devops-prod-generic"); got != PolicyReadOnly {
		t.Fatalf("read-only repo got %v", got)
	}
}

func TestPolicyFor_BrowseRead(t *testing.T) {
	p := NewPlanner(baseConfig())
	if got := p.PolicyFor("maven-public"); got != PolicyBrowseRead {
		t.Fatalf("maven-public got %v want browseRead", got)
	}
}

func TestPolicyFor_DefaultNone(t *testing.T) {
	c := baseConfig()
	c.GuestAccess.DefaultPolicy = "none"
	c.GuestAccess.BrowseRead.IncludeRepositories = []string{} // nothing included explicitly
	p := NewPlanner(c)
	// a repo not in any list with defaultPolicy=none -> none
	if got := p.PolicyFor("unknown-repo"); got != PolicyNone {
		t.Fatalf("unknown repo got %v want none", got)
	}
}

func TestComputeTargets_SkipsDenyAndNone(t *testing.T) {
	c := baseConfig()
	c.GuestAccess.DefaultPolicy = "browseRead"
	p := NewPlanner(c)
	got := p.ComputeTargets(repos())
	names := make(map[string]TargetPermission, len(got))
	for _, t := range got {
		names[t.Repository] = t
	}
	if _, ok := names["security-prod-generic"]; ok {
		t.Error("deny repo must not produce a target")
	}
	if _, ok := names["devops-prod-generic"]; !ok {
		t.Error("read-only repo must produce a target")
	}
	if tp := names["devops-prod-generic"]; !containsString(tp.Actions, "read") || containsString(tp.Actions, "browse") {
		t.Errorf("read-only actions = %v", tp.Actions)
	}
	if tp := names["maven-public"]; !containsString(tp.Actions, "browse") || !containsString(tp.Actions, "read") {
		t.Errorf("browse+read actions = %v", tp.Actions)
	}
}

func TestBuild_PlanShape(t *testing.T) {
	c := baseConfig()
	c.GuestAccess.DefaultPolicy = "browseRead"
	p := NewPlanner(c)

	// existing managed: the read-only privilege already present.
	// existingAll includes a forbidden global browse.
	readOnlyName := p.namer.PrivilegeName("raw", "devops-prod-generic", []string{"read"})
	existingManaged := []string{readOnlyName}
	existingAll := append([]string{}, existingManaged...)
	existingAll = append(existingAll, "nx-repository-view-*-*-browse", "nx-search-read")

	plan := p.Build(repos(), existingManaged, existingAll)

	// Should NOT recreate the read-only privilege.
	foundSkip := false
	for _, s := range plan.PrivilegesToSkip {
		if s == readOnlyName {
			foundSkip = true
		}
	}
	if !foundSkip {
		t.Errorf("expected %s in skip list, got %v", readOnlyName, plan.PrivilegesToSkip)
	}

	// Should create browse+read privileges for the two public repos.
	wantCreate := map[string]bool{
		"priv_guest_maven2_maven_public_browse_read": false,
		"priv_guest_npm_npm_public_browse_read":      false,
	}
	for _, w := range plan.PrivilegesToCreate {
		if _, ok := wantCreate[w.Name]; ok {
			wantCreate[w.Name] = true
		}
	}
	for name, ok := range wantCreate {
		if !ok {
			t.Errorf("expected %s to be created, plan created: %v", name, plan.PrivilegesToCreate)
		}
	}

	// Forbidden global browse must be flagged for removal.
	foundForbidden := false
	for _, r := range plan.RemovedRiskyPrivileges {
		if r == "nx-repository-view-*-*-browse" {
			foundForbidden = true
		}
	}
	if !foundForbidden {
		t.Errorf("expected forbidden nx-repository-view-*-*-browse in removals, got %v", plan.RemovedRiskyPrivileges)
	}

	// nx-search-read is a warn privilege.
	if len(plan.Warnings) == 0 {
		t.Errorf("expected a warning for nx-search-read, got %v", plan.Warnings)
	}
}

func TestBuild_ForbiddenPrivilegesUseGlobMatching(t *testing.T) {
	c := baseConfig()
	c.GuestAccess.ForbiddenPrivileges = []string{"nx-repository-view-*-*-browse"}
	p := NewPlanner(c)
	existingAll := []string{
		"nx-repository-view-raw-devops-prod-generic-browse",
		"nx-repository-view-raw-devops-prod-generic-read",
	}

	plan := p.Build(repos(), nil, existingAll)

	if !containsString(plan.RemovedRiskyPrivileges, "nx-repository-view-raw-devops-prod-generic-browse") {
		t.Fatalf("expected concrete browse privilege to match forbidden glob, got %v", plan.RemovedRiskyPrivileges)
	}
	if containsString(plan.RemovedRiskyPrivileges, "nx-repository-view-raw-devops-prod-generic-read") {
		t.Fatalf("read-only privilege should not match browse glob, got %v", plan.RemovedRiskyPrivileges)
	}
}

func TestBuild_RemovesUnwantedManaged(t *testing.T) {
	c := baseConfig()
	p := NewPlanner(c)
	// a stale managed privilege for a repo that no longer matches any policy
	stale := "priv_guest_raw_old_repo_browse_read"
	existingManaged := []string{stale}
	existingAll := append([]string{}, existingManaged...)
	plan := p.Build(repos(), existingManaged, existingAll)
	found := false
	for _, r := range plan.PrivilegesToRemove {
		if r == stale {
			found = true
		}
	}
	if !found {
		t.Errorf("expected stale managed privilege %s to be removed, got %v", stale, plan.PrivilegesToRemove)
	}
}

func TestBuild_RemovesReadPrivilegeForProtectedRepo(t *testing.T) {
	c := baseConfig()
	p := NewPlanner(c)
	readName := p.namer.PrivilegeName("raw", "security-prod-generic", []string{"read"})

	plan := p.Build(repos(), []string{readName}, []string{readName})

	if !containsString(plan.PrivilegesToRemove, readName) {
		t.Fatalf("expected protected repo read privilege %s to be removed, got %v", readName, plan.PrivilegesToRemove)
	}
	if containsString(plan.PrivilegesToSkip, readName) {
		t.Fatalf("protected repo read privilege should not be skipped, got %v", plan.PrivilegesToSkip)
	}
	if containsString(plan.DenyRepositories, "security-prod-generic") == false {
		t.Fatalf("expected protected repo in deny list, got %v", plan.DenyRepositories)
	}
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
