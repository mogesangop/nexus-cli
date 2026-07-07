package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/231397220/nexus-cli/internal/config"
)

func TestHACommandContainsExpectedSubcommands(t *testing.T) {
	cmd := NewHACmd()
	found := map[string]bool{}
	for _, child := range cmd.Commands() {
		found[child.Name()] = true
	}
	for _, name := range []string{"status", "health", "sync", "failover"} {
		if !found[name] {
			t.Fatalf("missing %q in commands=%v", name, found)
		}
	}
}

func TestHAFailoverRequiresFencingBeforeSync(t *testing.T) {
	path := writeHAConfig(t)
	cmd := newHAFailoverCmd()
	cmd.SetArgs([]string{"--config", path})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "fencing confirmation required") {
		t.Fatalf("expected fencing error, got %v", err)
	}
}

func TestHASyncRequiresOnceBeforeConfigOrCommands(t *testing.T) {
	cmd := newHASyncCmd()
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--once") {
		t.Fatalf("expected --once error, got %v", err)
	}
}

func writeHAConfig(t *testing.T) string {
	t.Helper()
	cfg := config.Default()
	cfg.HA.Enabled = true
	cfg.HA.Role = "primary"
	cfg.HA.Nodes = []config.HANodeConfig{
		{Name: "primary", Role: "primary", BaseURL: "http://nexus-a.example.com", PasswordEnv: "NEXUS_PRIMARY_PASSWORD"},
		{Name: "standby", Role: "standby", BaseURL: "http://nexus-b.example.com", PasswordEnv: "NEXUS_STANDBY_PASSWORD"},
	}
	cfg.HA.Replication.StateFile = filepath.Join(t.TempDir(), "ha-state.json")
	cfg.Audit.Enabled = false
	data, err := config.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}
