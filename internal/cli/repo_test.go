package cli

import (
	"strings"
	"testing"
)

func TestLifecycleRunRequiresConfirmationBeforeConfigOrNetwork(t *testing.T) {
	cmd := newLifecycleActionCmd(true)
	cmd.SetArgs([]string{"--repo", "releases"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("expected confirmation error, got %v", err)
	}
}

func TestRepoCommandContainsExpectedSubcommands(t *testing.T) {
	cmd := NewRepoCmd()
	found := map[string]bool{}
	for _, child := range cmd.Commands() {
		found[child.Name()] = true
	}
	for _, name := range []string{"list", "get", "apply", "ensure", "raw", "lifecycle"} {
		if !found[name] {
			t.Fatalf("missing %q in commands=%v", name, found)
		}
	}
}

func TestBlobStoreCommandContainsExpectedSubcommands(t *testing.T) {
	cmd := NewBlobStoreCmd()
	found := map[string]bool{}
	for _, child := range cmd.Commands() {
		found[child.Name()] = true
	}
	for _, name := range []string{"list", "get", "apply", "ensure"} {
		if !found[name] {
			t.Fatalf("missing %q in commands=%v", name, found)
		}
	}
}

func TestBlobStoreGetRejectsUnsupportedTypeBeforeConfigOrNetwork(t *testing.T) {
	cmd := newBlobStoreGetCmd()
	cmd.SetArgs([]string{"--name", "default", "--type", "s3"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "supports file") {
		t.Fatalf("expected unsupported type error, got %v", err)
	}
}
