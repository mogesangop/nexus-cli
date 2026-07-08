package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
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
	generated, err := config.Load(path)
	if err != nil {
		t.Fatalf("load generated config: %v", err)
	}
	example, err := config.Load(filepath.Join("..", "..", "examples", "config.example.yaml"))
	if err != nil {
		t.Fatalf("load example config: %v", err)
	}
	if !reflect.DeepEqual(generated, example) {
		t.Fatalf("generated config does not match examples/config.example.yaml\n generated=%#v\n example=%#v", generated, example)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read generated config: %v", err)
	}
	text := string(data)
	for _, want := range []string{
		"repositories:",
		"  raw:",
		`    - name: devops-prod-generic`,
		"      storage:",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("generated config missing %q:\n%s", want, text)
		}
	}
}
