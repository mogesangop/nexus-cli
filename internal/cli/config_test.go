package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/231397220/nexus-cli/internal/config"
)

func TestConfigInit_CreatesDirAndWritesDefault(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	cmd := newConfigInitCmd()
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	path := filepath.Join(tmpHome, ".nexus-cli", "config.yaml")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("config not written at %s: %v", path, err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Fatalf("file perm = %o, want 0o644", info.Mode().Perm())
	}
	dirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("dir stat: %v", err)
	}
	if dirInfo.Mode().Perm() != 0o700 {
		t.Fatalf("dir perm = %o, want 0o700", dirInfo.Mode().Perm())
	}
	if _, err := config.Load(path); err != nil {
		t.Fatalf("written config does not load: %v", err)
	}
}
