// Package ha implements the local orchestration pieces for the Nexus
// warm-standby HA workflow. It deliberately does not call F5 APIs; the PRD
// keeps traffic switching as an operator-controlled manual step.
package ha

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/231397220/nexus-cli/internal/config"
)

// SyncState records the last successful one-shot sync result for `ha status`.
type SyncState struct {
	UpdatedAt string         `json:"updatedAt"`
	Jobs      map[string]Job `json:"jobs"`
}

// Job is the persisted result for one replication lane.
type Job struct {
	Method        string `json:"method"`
	Schedule      string `json:"schedule"`
	LastSuccessAt string `json:"lastSuccessAt,omitempty"`
	LastErrorAt   string `json:"lastErrorAt,omitempty"`
	LastError     string `json:"lastError,omitempty"`
}

// LoadState reads a state file. Missing files are treated as an empty state so
// first-run status remains useful.
func LoadState(path string) (SyncState, error) {
	if strings.TrimSpace(path) == "" {
		return SyncState{Jobs: map[string]Job{}}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return SyncState{Jobs: map[string]Job{}}, nil
		}
		return SyncState{}, fmt.Errorf("read HA state %s: %w", path, err)
	}
	var st SyncState
	if err := json.Unmarshal(data, &st); err != nil {
		return SyncState{}, fmt.Errorf("parse HA state %s: %w", path, err)
	}
	if st.Jobs == nil {
		st.Jobs = map[string]Job{}
	}
	return st, nil
}

// SaveState atomically writes a state file.
func SaveState(path string, st SyncState) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	st.UpdatedAt = time.Now().Format(time.RFC3339)
	if st.Jobs == nil {
		st.Jobs = map[string]Job{}
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal HA state: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create HA state dir: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("write HA state temp file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("replace HA state %s: %w", path, err)
	}
	return nil
}

// Runner executes operator-provided sync commands and updates the state file.
type Runner struct {
	Config *config.Config
}

// Result is the outcome of one sync lane.
type Result struct {
	Name     string
	Method   string
	Command  string
	Duration time.Duration
	Err      error
}

// SyncOnce executes blob and metadata sync commands in sequence. Commands are
// intentionally configured by operators because SSH paths, Nexus export/import
// task ids, and environment-specific wrappers differ by installation.
func (r Runner) SyncOnce(ctx context.Context) ([]Result, error) {
	if r.Config == nil {
		return nil, fmt.Errorf("HA config is nil")
	}
	if !r.Config.HA.Enabled {
		return nil, fmt.Errorf("ha.enabled is false")
	}
	jobs := []struct {
		name string
		cfg  config.HASyncConfig
	}{
		{name: "blob", cfg: r.Config.HA.Replication.BlobSync},
		{name: "metadata", cfg: r.Config.HA.Replication.MetadataSync},
	}
	st, err := LoadState(r.Config.HA.Replication.StateFile)
	if err != nil {
		return nil, err
	}
	results := make([]Result, 0, len(jobs))
	var failed bool
	for _, j := range jobs {
		res := Result{Name: j.name, Method: j.cfg.Method, Command: j.cfg.Command}
		start := time.Now()
		err := runShell(ctx, j.cfg.Command)
		res.Duration = time.Since(start)
		res.Err = err
		now := time.Now().Format(time.RFC3339)
		stateJob := Job{Method: j.cfg.Method, Schedule: j.cfg.Schedule}
		if old, ok := st.Jobs[j.name]; ok {
			stateJob = old
			stateJob.Method = j.cfg.Method
			stateJob.Schedule = j.cfg.Schedule
		}
		if err != nil {
			failed = true
			stateJob.LastErrorAt = now
			stateJob.LastError = err.Error()
		} else {
			stateJob.LastSuccessAt = now
			stateJob.LastErrorAt = ""
			stateJob.LastError = ""
		}
		st.Jobs[j.name] = stateJob
		results = append(results, res)
		if err != nil {
			break
		}
	}
	if err := SaveState(r.Config.HA.Replication.StateFile, st); err != nil {
		return results, err
	}
	if failed {
		return results, fmt.Errorf("one or more HA sync jobs failed")
	}
	return results, nil
}

func runShell(ctx context.Context, command string) error {
	if strings.TrimSpace(command) == "" {
		return fmt.Errorf("sync command is empty; set ha.replication.*.command in config")
	}
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", command)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", command)
	}
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("command failed: %w", err)
	}
	return nil
}

// Lag returns a best-effort elapsed duration since an RFC3339 timestamp.
func Lag(ts string, now time.Time) (time.Duration, bool) {
	if ts == "" {
		return 0, false
	}
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return 0, false
	}
	return now.Sub(t), true
}
