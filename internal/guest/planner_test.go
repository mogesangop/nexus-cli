package guest

import (
	"testing"

	"github.com/231397220/nexus-cli/internal/config"
	"github.com/231397220/nexus-cli/internal/nexus"
)

func baseConfig() *config.Config {
	c := config.Default()
	// concrete repos so tests are deterministic
	c.GuestAccess.DownloadOnly.Repositories = []string{"protected-repo-example"}
	c.GuestAccess.Protected.Repositories = []string{"security-prod-generic"}
	return c
}

func repos() []nexus.Repository {
	return []nexus.Repository{
		{Name: "protected-repo-example", Format: "raw", Type: "hosted"},
		{Name: "maven-public", Format: "maven2", Type: "group"},
		{Name: "npm-public", Format: "npm", Type: "group"},
		{Name: "security-prod-generic", Format: "raw", Type: "hosted"},
	}
}

func TestPolicyFor_ProtectedWins(t *testing.T) {
	p := NewPlanner(baseConfig())
	if got := p.PolicyFor("security-prod-generic"); got != PolicyProtected {
		t.Fatalf("protected repo got %v", got)
	}
}

func TestPolicyFor_ProtectedWinsOverEveryOtherList(t *testing.T) {
	c := baseConfig()
	c.GuestAccess.Public.Repositories = []string{"*"}
	c.GuestAccess.DownloadOnly.Repositories = []string{"security-prod-generic"}
	p := NewPlanner(c)
	if got := p.PolicyFor("security-prod-generic"); got != PolicyProtected {
		t.Fatalf("protected repo got %v, want protected", got)
	}
}

func TestPolicyFor_DownloadOnly(t *testing.T) {
	p := NewPlanner(baseConfig())
	if got := p.PolicyFor("protected-repo-example"); got != PolicyDownloadOnly {
		t.Fatalf("download-only repo got %v", got)
	}
}

func TestPolicyFor_Public(t *testing.T) {
	p := NewPlanner(baseConfig())
	if got := p.PolicyFor("maven-public"); got != PolicyPublic {
		t.Fatalf("maven-public got %v want public", got)
	}
}

func TestPolicyFor_DefaultProtected(t *testing.T) {
	c := baseConfig()
	c.GuestAccess.DefaultPolicy = "protected"
	c.GuestAccess.Public.Repositories = []string{} // nothing explicitly public
	p := NewPlanner(c)
	if got := p.PolicyFor("unknown-repo"); got != PolicyProtected {
		t.Fatalf("unknown repo got %v want protected", got)
	}
}

func TestComputeTargets_SkipsDenyAndNone(t *testing.T) {
	c := baseConfig()
	c.GuestAccess.DefaultPolicy = "public"
	p := NewPlanner(c)
	got := p.ComputeTargets(repos())
	names := make(map[string]TargetPermission, len(got))
	for _, t := range got {
		names[t.Repository] = t
	}
	if _, ok := names["security-prod-generic"]; ok {
		t.Error("deny repo must not produce a target")
	}
	if _, ok := names["protected-repo-example"]; !ok {
		t.Error("read-only repo must produce a target")
	}
	if tp := names["protected-repo-example"]; !containsString(tp.Actions, "read") || containsString(tp.Actions, "browse") {
		t.Errorf("read-only actions = %v", tp.Actions)
	}
	if tp := names["maven-public"]; !containsString(tp.Actions, "browse") || !containsString(tp.Actions, "read") {
		t.Errorf("browse+read actions = %v", tp.Actions)
	}
}

func TestBuild_PlanShape(t *testing.T) {
	c := baseConfig()
	c.GuestAccess.DefaultPolicy = "public"
	p := NewPlanner(c)

	// existing managed: the read-only privilege already present.
	// existingAll includes a forbidden global browse.
	readOnlyName := p.namer.PrivilegeName("raw", "protected-repo-example", []string{"read"})
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
		"nx-repository-view-raw-protected-repo-example-browse",
		"nx-repository-view-raw-protected-repo-example-read",
	}

	plan := p.Build(repos(), nil, existingAll)

	if !containsString(plan.RemovedRiskyPrivileges, "nx-repository-view-raw-protected-repo-example-browse") {
		t.Fatalf("expected concrete browse privilege to match forbidden glob, got %v", plan.RemovedRiskyPrivileges)
	}
	if containsString(plan.RemovedRiskyPrivileges, "nx-repository-view-raw-protected-repo-example-read") {
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
	if containsString(plan.ProtectedRepositories, "security-prod-generic") == false {
		t.Fatalf("expected protected repo in protected list, got %v", plan.ProtectedRepositories)
	}
}

func TestPolicyFor_LegacyConfiguration(t *testing.T) {
	c := config.Default()
	c.GuestAccess.Public.Repositories = nil
	c.GuestAccess.Protected.Repositories = nil
	c.GuestAccess.DefaultPolicy = "browseRead"
	c.GuestAccess.BrowseRead.IncludeRepositories = []string{"*"}
	c.GuestAccess.Deny.Repositories = []string{"security-prod-generic"}
	p := NewPlanner(c)
	if got := p.PolicyFor("security-prod-generic"); got != PolicyProtected {
		t.Fatalf("legacy protected repo got %v", got)
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
