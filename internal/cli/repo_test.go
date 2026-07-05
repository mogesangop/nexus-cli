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

func TestRepoCommandContainsRawAndLifecycle(t *testing.T) {
	cmd := NewRepoCmd()
	found := map[string]bool{}
	for _, child := range cmd.Commands() {
		found[child.Name()] = true
	}
	if !found["raw"] || !found["lifecycle"] || !found["list"] {
		t.Fatalf("commands=%v", found)
	}
}
