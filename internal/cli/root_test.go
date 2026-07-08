package cli

import (
	"strings"
	"testing"
)

func TestRootLongDescriptionUsesProtectSemantics(t *testing.T) {
	root := NewRoot()
	long := root.cmd.Long
	if !strings.Contains(long, "protected repositories are hidden and non-downloadable") {
		t.Fatalf("root long description should describe protected repos as non-downloadable, got %q", long)
	}
	if strings.Contains(long, "remaining downloadable via exact URL") {
		t.Fatalf("root long description still contains old read-only protected semantics: %q", long)
	}
}
