package ha

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/231397220/nexus-cli/internal/config"
)

func TestLoadStateMissingFile(t *testing.T) {
	st, err := LoadState(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if len(st.Jobs) != 0 {
		t.Fatalf("jobs=%v want empty", st.Jobs)
	}
}

func TestRunnerSyncOnceUpdatesState(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell command shape is platform-specific")
	}
	dir := t.TempDir()
	blobMarker := filepath.Join(dir, "blob.ok")
	metaMarker := filepath.Join(dir, "metadata.ok")
	cfg := validHAConfigForTest(filepath.Join(dir, "state.json"))
	cfg.HA.Replication.BlobSync.Command = "touch " + shellQuote(blobMarker)
	cfg.HA.Replication.MetadataSync.Command = "touch " + shellQuote(metaMarker)

	results, err := Runner{Config: cfg}.SyncOnce(context.Background())
	if err != nil {
		t.Fatalf("SyncOnce: %v results=%+v", err, results)
	}
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
	for _, p := range []string{blobMarker, metaMarker} {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("marker %s not created: %v", p, err)
		}
	}
	st, err := LoadState(cfg.HA.Replication.StateFile)
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	for _, name := range []string{"blob", "metadata"} {
		if st.Jobs[name].LastSuccessAt == "" {
			t.Fatalf("%s LastSuccessAt not recorded: %+v", name, st.Jobs[name])
		}
	}
}

func TestRunnerSyncOnceStopsAfterFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell command shape is platform-specific")
	}
	cfg := validHAConfigForTest(filepath.Join(t.TempDir(), "state.json"))
	cfg.HA.Replication.BlobSync.Command = "exit 7"
	cfg.HA.Replication.MetadataSync.Command = "touch should-not-run"

	results, err := Runner{Config: cfg}.SyncOnce(context.Background())
	if err == nil {
		t.Fatal("expected sync failure")
	}
	if len(results) != 1 || results[0].Err == nil {
		t.Fatalf("results=%+v want one failed blob result", results)
	}
	st, err := LoadState(cfg.HA.Replication.StateFile)
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if st.Jobs["blob"].LastError == "" {
		t.Fatalf("blob error not recorded: %+v", st.Jobs["blob"])
	}
	if _, ok := st.Jobs["metadata"]; ok {
		t.Fatalf("metadata should not have run: %+v", st.Jobs["metadata"])
	}
}

func TestLag(t *testing.T) {
	now := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	got, ok := Lag(now.Add(-5*time.Minute).Format(time.RFC3339), now)
	if !ok || got != 5*time.Minute {
		t.Fatalf("Lag=%s ok=%t", got, ok)
	}
}

func validHAConfigForTest(stateFile string) *config.Config {
	cfg := config.Default()
	cfg.HA.Enabled = true
	cfg.HA.Role = "primary"
	cfg.HA.Nodes = []config.HANodeConfig{
		{Name: "primary", Role: "primary", BaseURL: "http://nexus-a.example.com", PasswordEnv: "NEXUS_PRIMARY_PASSWORD"},
		{Name: "standby", Role: "standby", BaseURL: "http://nexus-b.example.com", PasswordEnv: "NEXUS_STANDBY_PASSWORD"},
	}
	cfg.HA.Replication.StateFile = stateFile
	return cfg
}

func shellQuote(s string) string {
	return "'" + s + "'"
}
