// Package audit writes append-only JSONL audit records (PRD section 16).
// Records never contain the admin password or Authorization header.
package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Record is a single audit log entry.
type Record struct {
	Timestamp          string   `json:"timestamp"`
	Operator           string   `json:"operator"`
	Command            string   `json:"command"`
	NexusBaseURL       string   `json:"nexusBaseUrl"`
	TargetRole         string   `json:"targetRole,omitempty"`
	DryRun             bool     `json:"dryRun"`
	Action             string   `json:"action"`
	Result             string   `json:"result"`
	CreatedPrivileges  []string `json:"createdPrivileges,omitempty"`
	UpdatedRoles       []string `json:"updatedRoles,omitempty"`
	RemovedPrivileges  []string `json:"removedPrivileges,omitempty"`
	ErrorMessage       string   `json:"errorMessage,omitempty"`
	TargetUser         string   `json:"targetUser,omitempty"`
	TargetPath         string   `json:"targetPath,omitempty"`
	TargetRepo         string   `json:"targetRepo,omitempty"`
	CreatedSelectors   []string `json:"createdSelectors,omitempty"`
	CreatedUsers       []string `json:"createdUsers,omitempty"`
}

// Logger writes JSONL records to a file. It is safe for sequential use by a
// single CLI invocation; concurrent writes from multiple goroutines should be
// guarded by the caller.
type Logger struct {
	path          string
	enabled       bool
	maskSensitive bool
}

// New returns a Logger. If enabled is false, Write is a no-op. The parent
// directory of path is created on the first Write if it does not exist.
func New(path string, enabled, maskSensitive bool) *Logger {
	return &Logger{path: path, enabled: enabled, maskSensitive: maskSensitive}
}

// Write appends a record as one JSON line. It never returns an error that
// would fail the CLI: audit failures are reported to stderr but do not abort
// the audited operation.
func (l *Logger) Write(r Record) error {
	if !l.enabled {
		return nil
	}
	if r.Timestamp == "" {
		r.Timestamp = time.Now().Format(time.RFC3339)
	}
	if l.maskSensitive {
		r = mask(r)
	}
	if dir := filepath.Dir(l.path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create audit dir %s: %w", dir, err)
		}
	}
	f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open audit log %s: %w", l.path, err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(r); err != nil {
		return fmt.Errorf("write audit record: %w", err)
	}
	return nil
}

// mask is a placeholder for sensitive-field redaction. By design the Record
// type never carries secrets; this hook exists so future fields can be
// scrubbed without changing call sites. Currently a pass-through.
func mask(r Record) Record {
	return r
}
