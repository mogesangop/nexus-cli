package cli

import "testing"

func TestGuestCommandIncludesProtectAndDeprecatedSync(t *testing.T) {
	cmd := NewGuestCmd()

	protect, _, err := cmd.Find([]string{"protect"})
	if err != nil || protect == nil || protect.Name() != "protect" {
		t.Fatalf("guest protect command not found: cmd=%v err=%v", protect, err)
	}

	sync, _, err := cmd.Find([]string{"sync"})
	if err != nil || sync == nil || sync.Name() != "sync" {
		t.Fatalf("guest sync command not found: cmd=%v err=%v", sync, err)
	}
	if sync.Deprecated == "" {
		t.Fatal("guest sync should be marked deprecated")
	}
}
