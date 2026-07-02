package naming

import (
	"testing"

	"github.com/moge/nexus-cli/internal/config"
)

func cfg() config.PrivilegeNaming {
	return config.PrivilegeNaming{
		Prefix:                "priv_guest",
		Separator:             "_",
		ReplaceDashWithUScore: true,
	}
}

func TestPrivilegeName_ReadOnlyRaw(t *testing.T) {
	g := New(cfg())
	got := g.PrivilegeName("raw", "devops-prod-generic", []string{"read"})
	want := "priv_guest_raw_devops_prod_generic_read"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestPrivilegeName_BrowseReadSorted(t *testing.T) {
	g := New(cfg())
	// actions supplied out of order must normalize to a stable name
	got := g.PrivilegeName("maven2", "maven-public", []string{"read", "browse"})
	want := "priv_guest_maven2_maven_public_browse_read"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestPrivilegeName_DedupActions(t *testing.T) {
	g := New(cfg())
	got := g.PrivilegeName("raw", "raw-public", []string{"read", "read", "browse"})
	want := "priv_guest_raw_raw_public_browse_read"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestSanitizeRepo_DotsAndSlashes(t *testing.T) {
	g := New(cfg())
	cases := map[string]string{
		"devops-prod-generic": "devops_prod_generic",
		"foo.bar":             "foo_bar",
		"a/b/c":               "a_b_c",
		"plain":               "plain",
	}
	for in, want := range cases {
		if got := g.SanitizeRepo(in); got != want {
			t.Errorf("SanitizeRepo(%q)=%q want %q", in, got, want)
		}
	}
}

func TestIsManaged(t *testing.T) {
	g := New(cfg())
	if !g.IsManaged("priv_guest_raw_devops_prod_generic_read") {
		t.Error("expected managed")
	}
	if g.IsManaged("nx-admin") {
		t.Error("nx-admin should not be managed")
	}
	if g.IsManaged("nx-repository-view-*-*-browse") {
		t.Error("global wildcard should not be managed")
	}
}

func TestNoReplaceDash(t *testing.T) {
	c := cfg()
	c.ReplaceDashWithUScore = false
	g := New(c)
	got := g.PrivilegeName("raw", "devops-prod-generic", []string{"read"})
	want := "priv_guest_raw_devops-prod-generic_read"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
