package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/231397220/nexus-cli/internal/config"
	"github.com/231397220/nexus-cli/internal/nexus"
	"github.com/spf13/cobra"
)

func TestReadOnlyCommandsExposeOutputFlag(t *testing.T) {
	tests := []struct {
		name string
		cmd  *cobra.Command
	}{
		{"health check", newHealthCheckCmd()},
		{"repo list", newRepoListCmd()},
		{"repo get", newRepoGetCmd()},
		{"blobstore list", newBlobStoreListCmd()},
		{"blobstore get", newBlobStoreGetCmd()},
		{"guest check", newGuestCheckCmd()},
		{"ha status", newHAStatusCmd()},
		{"ha health", newHAHealthCmd()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.cmd.Flags().Lookup("output") == nil {
				t.Fatalf("%s missing --output flag", tt.name)
			}
		})
	}
}

func TestOutputFlagDefaultsToText(t *testing.T) {
	tests := []struct {
		name string
		cmd  *cobra.Command
	}{
		{"health check", newHealthCheckCmd()},
		{"repo list", newRepoListCmd()},
		{"repo get", newRepoGetCmd()},
		{"blobstore list", newBlobStoreListCmd()},
		{"blobstore get", newBlobStoreGetCmd()},
		{"guest check", newGuestCheckCmd()},
		{"ha status", newHAStatusCmd()},
		{"ha health", newHAHealthCmd()},
		{"repo apply", newRepoApplyCmd()},
		{"repo ensure", newRepoEnsureCmd()},
		{"blobstore apply", newBlobStoreApplyCmd()},
		{"blobstore ensure", newBlobStoreEnsureCmd()},
		{"guest protect", newGuestProtectCmd()},
		{"share grant", newShareGrantCmd()},
		{"repo lifecycle run", newLifecycleActionCmd(true)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := tt.cmd.Flags().Lookup("output")
			if flag == nil {
				t.Fatalf("%s missing --output flag", tt.name)
			}
			if flag.DefValue != outputText {
				t.Fatalf("%s --output default = %q, want %q", tt.name, flag.DefValue, outputText)
			}
		})
	}
}

func TestDryRunWriteCommandsExposeOutputFlag(t *testing.T) {
	tests := []struct {
		name string
		cmd  *cobra.Command
	}{
		{"repo apply", newRepoApplyCmd()},
		{"repo ensure", newRepoEnsureCmd()},
		{"blobstore apply", newBlobStoreApplyCmd()},
		{"blobstore ensure", newBlobStoreEnsureCmd()},
		{"guest protect", newGuestProtectCmd()},
		{"share grant", newShareGrantCmd()},
		{"repo lifecycle run", newLifecycleActionCmd(true)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.cmd.Flags().Lookup("output") == nil {
				t.Fatalf("%s missing --output flag", tt.name)
			}
		})
	}
}

func TestWriteCommandsExposeYesFlag(t *testing.T) {
	tests := []struct {
		name string
		cmd  *cobra.Command
	}{
		{"repo apply", newRepoApplyCmd()},
		{"repo ensure", newRepoEnsureCmd()},
		{"repo raw apply", newRawApplyCmd()},
		{"repo raw ensure", newRawEnsureCmd()},
		{"blobstore apply", newBlobStoreApplyCmd()},
		{"blobstore ensure", newBlobStoreEnsureCmd()},
		{"guest protect", newGuestProtectCmd()},
		{"share grant", newShareGrantCmd()},
		{"repo lifecycle run", newLifecycleActionCmd(true)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.cmd.Flags().Lookup("yes") == nil {
				t.Fatalf("%s missing --yes flag", tt.name)
			}
		})
	}
}

func TestWriteCommandsRequireYesForRealRunsBeforeConfig(t *testing.T) {
	settingsPath := filepath.Join(t.TempDir(), "repo-settings.yaml")
	if err := os.WriteFile(settingsPath, []byte("online: true\n"), 0o644); err != nil {
		t.Fatalf("write settings: %v", err)
	}
	tests := []struct {
		name string
		cmd  *cobra.Command
		args []string
	}{
		{"repo apply", newRepoApplyCmd(), nil},
		{"repo ensure", newRepoEnsureCmd(), []string{"--name", "npm-hosted", "--format", "npm", "--type", "hosted", "--settings", settingsPath}},
		{"repo raw apply", newRawApplyCmd(), nil},
		{"repo raw ensure", newRawEnsureCmd(), []string{"--name", "raw-hosted", "--blob-store", "default"}},
		{"blobstore apply", newBlobStoreApplyCmd(), nil},
		{"blobstore ensure", newBlobStoreEnsureCmd(), []string{"--name", "default", "--path", "/nexus-data/blobs/default"}},
		{"guest protect", newGuestProtectCmd(), nil},
		{"share grant", newShareGrantCmd(), []string{"--repo", "raw-hosted", "--path", "/team-a/", "--user", "team-a-user", "--email", "team-a@example.com"}},
		{"repo lifecycle run", newLifecycleActionCmd(true), []string{"--repo", "raw-hosted"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := executeCommandForOutput(tt.cmd, tt.args...)
			if err == nil {
				t.Fatal("expected confirmation error")
			}
			if !strings.Contains(err.Error(), "CONFIRMATION_REQUIRED") || !strings.Contains(err.Error(), "--yes") {
				t.Fatalf("expected --yes confirmation error, got %v", err)
			}
		})
	}
}

func TestWriteCommandMissingYesJSONErrorOutput(t *testing.T) {
	out, err := executeCommandForOutput(newRepoApplyCmd(), "--output", "json")
	if err == nil {
		t.Fatal("expected confirmation error")
	}
	assertJSONErrorCode(t, out, "repo apply", errorConfirmationRequired)
}

func TestWriteCommandsWithYesBypassConfirmationGate(t *testing.T) {
	settingsPath := filepath.Join(t.TempDir(), "repo-settings.yaml")
	if err := os.WriteFile(settingsPath, []byte("online: true\n"), 0o644); err != nil {
		t.Fatalf("write settings: %v", err)
	}
	tests := []struct {
		name string
		cmd  *cobra.Command
		args []string
	}{
		{"repo apply", newRepoApplyCmd(), []string{"--yes"}},
		{"repo ensure", newRepoEnsureCmd(), []string{"--name", "npm-hosted", "--format", "npm", "--type", "hosted", "--settings", settingsPath, "--yes"}},
		{"repo raw apply", newRawApplyCmd(), []string{"--yes"}},
		{"repo raw ensure", newRawEnsureCmd(), []string{"--name", "raw-hosted", "--blob-store", "default", "--yes"}},
		{"blobstore apply", newBlobStoreApplyCmd(), []string{"--yes"}},
		{"blobstore ensure", newBlobStoreEnsureCmd(), []string{"--name", "default", "--path", "/nexus-data/blobs/default", "--yes"}},
		{"guest protect", newGuestProtectCmd(), []string{"--yes"}},
		{"share grant", newShareGrantCmd(), []string{"--repo", "raw-hosted", "--path", "/team-a/", "--user", "team-a-user", "--email", "team-a@example.com", "--yes"}},
		{"repo lifecycle run", newLifecycleActionCmd(true), []string{"--repo", "raw-hosted", "--yes"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := executeCommandForOutput(tt.cmd, tt.args...)
			if err == nil {
				t.Fatal("expected later validation/config error")
			}
			if strings.Contains(err.Error(), "CONFIRMATION_REQUIRED") || strings.Contains(err.Error(), "without --yes") {
				t.Fatalf("--yes should bypass confirmation gate, got %v", err)
			}
		})
	}
}

func TestWriteCommandsDryRunDoNotRequireYes(t *testing.T) {
	settingsPath := filepath.Join(t.TempDir(), "repo-settings.yaml")
	if err := os.WriteFile(settingsPath, []byte("online: true\n"), 0o644); err != nil {
		t.Fatalf("write settings: %v", err)
	}
	tests := []struct {
		name string
		cmd  *cobra.Command
		args []string
	}{
		{"repo apply", newRepoApplyCmd(), []string{"--dry-run"}},
		{"repo ensure", newRepoEnsureCmd(), []string{"--name", "npm-hosted", "--format", "npm", "--type", "hosted", "--settings", settingsPath, "--dry-run"}},
		{"repo raw apply", newRawApplyCmd(), []string{"--dry-run"}},
		{"repo raw ensure", newRawEnsureCmd(), []string{"--name", "raw-hosted", "--blob-store", "default", "--dry-run"}},
		{"blobstore apply", newBlobStoreApplyCmd(), []string{"--dry-run"}},
		{"blobstore ensure", newBlobStoreEnsureCmd(), []string{"--name", "default", "--path", "/nexus-data/blobs/default", "--dry-run"}},
		{"guest protect", newGuestProtectCmd(), []string{"--dry-run"}},
		{"share grant", newShareGrantCmd(), []string{"--repo", "raw-hosted", "--path", "/team-a/", "--user", "team-a-user", "--email", "team-a@example.com", "--dry-run"}},
		{"repo lifecycle run", newLifecycleActionCmd(true), []string{"--repo", "raw-hosted", "--dry-run"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := executeCommandForOutput(tt.cmd, tt.args...)
			if err == nil {
				t.Fatal("expected later validation/config error")
			}
			if strings.Contains(err.Error(), "--yes") {
				t.Fatalf("dry-run should not require --yes, got %v", err)
			}
		})
	}
}

func TestReadOnlyCommandsRejectUnsupportedOutputBeforeConfig(t *testing.T) {
	cmd := newRepoListCmd()
	cmd.SetArgs([]string{"--output", "xml"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "UNSUPPORTED_OUTPUT") {
		t.Fatalf("expected unsupported output error, got %v", err)
	}
}

func TestReadOnlyCommandsJSONOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/service/rest/v1/repositories":
			_, _ = io.WriteString(w, `[
				{"name":"raw-hosted","format":"raw","type":"hosted"},
				{"name":"npm-hosted","format":"npm","type":"hosted"}
			]`)
		case "/service/rest/v1/repositories/npm/hosted/npm-hosted":
			_, _ = io.WriteString(w, `{"name":"npm-hosted","online":true}`)
		case "/service/rest/v1/blobstores":
			_, _ = io.WriteString(w, `[{"name":"default","type":"file"}]`)
		case "/service/rest/v1/blobstores/file/default":
			_, _ = io.WriteString(w, `{"name":"default","path":"/nexus-data/blobs/default"}`)
		case "/service/rest/v1/security/privileges":
			_, _ = io.WriteString(w, `[]`)
		case "/service/rest/v1/security/roles/role_guest_repository_access":
			_, _ = io.WriteString(w, `{
				"id":"role_guest_repository_access",
				"name":"role_guest_repository_access",
				"privileges":[],
				"roles":[]
			}`)
		case "/service/rest/v1/security/users/anonymous":
			_, _ = io.WriteString(w, `{
				"userId":"anonymous",
				"status":"active",
				"roles":["role_guest_repository_access"]
			}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cfgPath := writeAIOutputConfig(t, server.URL)
	tests := []struct {
		name        string
		cmd         *cobra.Command
		args        []string
		wantCommand string
	}{
		{"health check", newHealthCheckCmd(), []string{"--config", cfgPath, "--output", "json"}, "health check"},
		{"repo list", newRepoListCmd(), []string{"--config", cfgPath, "--output", "json", "--format", "npm"}, "repo list"},
		{"repo get", newRepoGetCmd(), []string{"--config", cfgPath, "--output", "json", "--name", "npm-hosted", "--format", "npm", "--type", "hosted"}, "repo get"},
		{"blobstore list", newBlobStoreListCmd(), []string{"--config", cfgPath, "--output", "json"}, "blobstore list"},
		{"blobstore get", newBlobStoreGetCmd(), []string{"--config", cfgPath, "--output", "json", "--name", "default"}, "blobstore get"},
		{"guest check", newGuestCheckCmd(), []string{"--config", cfgPath, "--output", "json"}, "guest check"},
		{"ha status", newHAStatusCmd(), []string{"--config", cfgPath, "--output", "json"}, "ha status"},
		{"ha health", newHAHealthCmd(), []string{"--config", cfgPath, "--output", "json"}, "ha health"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			tt.cmd.SetOut(&out)
			tt.cmd.SetArgs(tt.args)
			if err := tt.cmd.Execute(); err != nil {
				t.Fatalf("execute: %v\nstdout=%s", err, out.String())
			}
			if !json.Valid(out.Bytes()) {
				t.Fatalf("stdout is not valid json:\n%s", out.String())
			}
			var resp commandResponse
			if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if resp.Command != tt.wantCommand {
				t.Fatalf("command = %q, want %q", resp.Command, tt.wantCommand)
			}
			if resp.Result != "success" {
				t.Fatalf("result = %q, want success; response=%s", resp.Result, out.String())
			}
			if resp.Data == nil {
				t.Fatalf("data should be present: %s", out.String())
			}
			if resp.DryRun {
				t.Fatalf("read-only response dryRun should be false: %s", out.String())
			}
			if resp.Changes == nil {
				t.Fatalf("changes should be present as an empty array: %s", out.String())
			}
			if resp.Warnings == nil {
				t.Fatalf("warnings should be present as an array: %s", out.String())
			}
			if resp.Errors == nil {
				t.Fatalf("errors should be present as an empty array: %s", out.String())
			}
		})
	}
}

func TestClassifyError(t *testing.T) {
	cfg := config.Default()
	cfg.Nexus.PasswordEnv = "NEXUS_OUTPUT_MISSING_PASSWORD"
	_, passwordErr := cfg.Password()

	tests := []struct {
		name string
		err  error
		want string
	}{
		{"unsupported output", validateOutput("xml"), errorUnsupportedOutput},
		{"password env missing", passwordErr, errorPasswordEnvMissing},
		{"auth failed", &nexus.APIError{Status: http.StatusUnauthorized, Body: "bad credentials"}, errorNexusAuthFailed},
		{"api error", &nexus.APIError{Status: http.StatusInternalServerError, Body: "boom"}, errorNexusAPI},
		{"confirmation required", errors.New("refusing deletion without --yes; run lifecycle preview first"), errorConfirmationRequired},
		{"operation conflict", errors.New(`repository "x" exists as npm/hosted; expected raw/hosted`), errorOperationConflict},
		{"config invalid", errors.New("nexus.baseUrl is required"), errorConfigInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifyError(tt.err); got != tt.want {
				t.Fatalf("classifyError() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestReadOnlyCommandJSONErrorOutput(t *testing.T) {
	t.Run("config invalid", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "bad.yaml")
		if err := os.WriteFile(path, []byte("nexus:\n  username: admin\n  passwordEnv: NEXUS_PASSWORD\n"), 0o644); err != nil {
			t.Fatalf("write config: %v", err)
		}
		out, err := executeCommandForOutput(newRepoListCmd(), "--config", path, "--output", "json")
		if err == nil {
			t.Fatal("expected error")
		}
		assertJSONErrorCode(t, out, "repo list", errorConfigInvalid)
	})

	t.Run("password env missing", func(t *testing.T) {
		cfg := config.Default()
		cfg.Nexus.PasswordEnv = "NEXUS_OUTPUT_INTENTIONALLY_MISSING"
		cfg.Audit.Enabled = false
		data, err := config.Marshal(cfg)
		if err != nil {
			t.Fatalf("marshal config: %v", err)
		}
		path := filepath.Join(t.TempDir(), "config.yaml")
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatalf("write config: %v", err)
		}
		out, err := executeCommandForOutput(newRepoListCmd(), "--config", path, "--output", "json")
		if err == nil {
			t.Fatal("expected error")
		}
		assertJSONErrorCode(t, out, "repo list", errorPasswordEnvMissing)
	})

	t.Run("nexus auth failed", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "bad credentials", http.StatusUnauthorized)
		}))
		defer server.Close()
		cfgPath := writeAIOutputConfig(t, server.URL)
		out, err := executeCommandForOutput(newRepoListCmd(), "--config", cfgPath, "--output", "json")
		if err == nil {
			t.Fatal("expected error")
		}
		assertJSONErrorCode(t, out, "repo list", errorNexusAuthFailed)
	})

	t.Run("nexus api error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "boom", http.StatusInternalServerError)
		}))
		defer server.Close()
		cfgPath := writeAIOutputConfig(t, server.URL)
		out, err := executeCommandForOutput(newRepoListCmd(), "--config", cfgPath, "--output", "json")
		if err == nil {
			t.Fatal("expected error")
		}
		assertJSONErrorCode(t, out, "repo list", errorNexusAPI)
	})
}

func TestDryRunWriteCommandsJSONPlanOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/service/rest/v1/repositories":
			_, _ = io.WriteString(w, `[
				{"name":"raw-hosted","format":"raw","type":"hosted"}
			]`)
		case "/service/rest/v1/blobstores":
			_, _ = io.WriteString(w, `[]`)
		case "/service/rest/v1/security/privileges":
			_, _ = io.WriteString(w, `[]`)
		case "/service/rest/v1/security/roles":
			_, _ = io.WriteString(w, `[]`)
		case "/service/rest/v1/security/roles/role_guest_repository_access":
			_, _ = io.WriteString(w, `{
				"id":"role_guest_repository_access",
				"name":"role_guest_repository_access",
				"privileges":[],
				"roles":[]
			}`)
		case "/service/rest/v1/security/users":
			_, _ = io.WriteString(w, `[]`)
		case "/service/rest/v1/security/content-selectors":
			_, _ = io.WriteString(w, `[]`)
		case "/service/rest/v1/components":
			if r.URL.Query().Get("repository") != "raw-hosted" {
				http.NotFound(w, r)
				return
			}
			old := time.Now().AddDate(0, 0, -45).UTC().Format(time.RFC3339Nano)
			_, _ = io.WriteString(w, `{
				"items":[{
					"id":"component-1",
					"repository":"raw-hosted",
					"format":"raw",
					"name":"old.bin",
					"assets":[{"id":"asset-1","path":"releases/old.bin","lastModified":`+strconv.Quote(old)+`}]
				}]
			}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cfgPath := writeAIDryRunConfig(t, server.URL)
	settingsPath := filepath.Join(t.TempDir(), "repo-settings.yaml")
	if err := os.WriteFile(settingsPath, []byte("online: true\n"), 0o644); err != nil {
		t.Fatalf("write settings: %v", err)
	}

	tests := []struct {
		name        string
		cmd         *cobra.Command
		args        []string
		wantCommand string
	}{
		{"repo apply", newRepoApplyCmd(), []string{"--config", cfgPath, "--dry-run", "--output", "json"}, "repo apply"},
		{"repo ensure", newRepoEnsureCmd(), []string{"--config", cfgPath, "--name", "npm-hosted", "--format", "npm", "--type", "hosted", "--settings", settingsPath, "--dry-run", "--output", "json"}, "repo ensure"},
		{"blobstore apply", newBlobStoreApplyCmd(), []string{"--config", cfgPath, "--dry-run", "--output", "json"}, "blobstore apply"},
		{"blobstore ensure", newBlobStoreEnsureCmd(), []string{"--config", cfgPath, "--name", "default", "--path", "/nexus-data/blobs/default", "--dry-run", "--output", "json"}, "blobstore ensure"},
		{"guest protect", newGuestProtectCmd(), []string{"--config", cfgPath, "--dry-run", "--output", "json"}, "guest protect"},
		{"share grant", newShareGrantCmd(), []string{"--config", cfgPath, "--repo", "raw-hosted", "--path", "/team-a/", "--user", "team-a-user", "--email", "team-a@example.com", "--dry-run", "--output", "json"}, "share grant"},
		{"repo lifecycle run", newLifecycleActionCmd(true), []string{"--config", cfgPath, "--repo", "raw-hosted", "--dry-run", "--output", "json"}, "repo lifecycle run"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := executeCommandForOutput(tt.cmd, tt.args...)
			if err != nil {
				t.Fatalf("execute: %v\nstdout=%s", err, out)
			}
			assertJSONDryRunPlan(t, out, tt.wantCommand)
		})
	}
}

func executeCommandForOutput(cmd *cobra.Command, args ...string) (string, error) {
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func assertJSONDryRunPlan(t *testing.T, out, wantCommand string) {
	t.Helper()
	if !json.Valid([]byte(out)) {
		t.Fatalf("stdout is not valid json:\n%s", out)
	}
	var resp commandResponse
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Command != wantCommand {
		t.Fatalf("command = %q, want %q; response=%s", resp.Command, wantCommand, out)
	}
	if !resp.DryRun {
		t.Fatalf("dryRun should be true: %s", out)
	}
	if resp.Result != "planned" {
		t.Fatalf("result = %q, want planned; response=%s", resp.Result, out)
	}
	if len(resp.Changes) == 0 {
		t.Fatalf("changes should contain planned changes: %s", out)
	}
	if len(resp.Errors) != 0 {
		t.Fatalf("errors should be empty: %s", out)
	}
}

func assertJSONErrorCode(t *testing.T, out, wantCommand, wantCode string) {
	t.Helper()
	if !json.Valid([]byte(out)) {
		t.Fatalf("stdout is not valid json:\n%s", out)
	}
	var resp commandResponse
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Command != wantCommand {
		t.Fatalf("command = %q, want %q; response=%s", resp.Command, wantCommand, out)
	}
	if resp.Result != "failed" {
		t.Fatalf("result = %q, want failed; response=%s", resp.Result, out)
	}
	if len(resp.Errors) != 1 {
		t.Fatalf("errors length = %d, want 1; response=%s", len(resp.Errors), out)
	}
	if resp.Errors[0].Code != wantCode {
		t.Fatalf("error code = %q, want %q; response=%s", resp.Errors[0].Code, wantCode, out)
	}
}

func TestCommandResponseJSONShape(t *testing.T) {
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	if err := writeReadOnlyResponse(cmd, "repo list", "success", map[string]any{"total": 0}, nil); err != nil {
		t.Fatalf("write response: %v", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(out.Bytes(), &raw); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	for _, field := range []string{"command", "dryRun", "result", "data", "changes", "warnings", "errors"} {
		if _, ok := raw[field]; !ok {
			t.Fatalf("response missing %q: %s", field, out.String())
		}
	}
	if _, ok := raw["auditLogPath"]; ok {
		t.Fatalf("auditLogPath should be omitted when empty: %s", out.String())
	}
}

func TestJSONOutputEmptyResultShape(t *testing.T) {
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	if err := writeReadOnlyResponse(cmd, "repo list", "success", map[string]any{
		"repositories": []nexus.Repository{},
		"total":        0,
	}, nil); err != nil {
		t.Fatalf("write response: %v", err)
	}
	var resp commandResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Command != "repo list" || resp.Result != "success" || resp.DryRun {
		t.Fatalf("unexpected response envelope: %s", out.String())
	}
	if len(resp.Changes) != 0 {
		t.Fatalf("empty read-only result should have no changes: %s", out.String())
	}
	if len(resp.Warnings) != 0 || len(resp.Errors) != 0 {
		t.Fatalf("warnings/errors should be empty arrays: %s", out.String())
	}
	data, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatalf("data should decode as object: %#v", resp.Data)
	}
	if total, ok := data["total"].(float64); !ok || total != 0 {
		t.Fatalf("total = %#v, want 0; response=%s", data["total"], out.String())
	}
	repos, ok := data["repositories"].([]any)
	if !ok || len(repos) != 0 {
		t.Fatalf("repositories should be empty array: %#v; response=%s", data["repositories"], out.String())
	}
}

func TestJSONDryRunOutputMultiChangeShape(t *testing.T) {
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	changes := []responseChange{
		{
			ResourceType: "repository",
			Name:         "npm-hosted",
			Action:       "create",
			Details: map[string]any{
				"format": "npm",
				"type":   "hosted",
			},
		},
		{
			ResourceType: "repository",
			Name:         "maven-hosted",
			Action:       "update",
			Details: map[string]any{
				"format": "maven2",
				"type":   "hosted",
			},
		},
	}
	if err := writeDryRunResponse(cmd, "repo apply", map[string]any{"total": 2}, changes, []string{"review storage policy"}); err != nil {
		t.Fatalf("write response: %v", err)
	}
	var resp commandResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Command != "repo apply" || resp.Result != "planned" || !resp.DryRun {
		t.Fatalf("unexpected dry-run envelope: %s", out.String())
	}
	if len(resp.Changes) != 2 {
		t.Fatalf("changes length = %d, want 2; response=%s", len(resp.Changes), out.String())
	}
	if resp.Changes[0].Name != "npm-hosted" || resp.Changes[1].Name != "maven-hosted" {
		t.Fatalf("changes did not preserve planned resources: %+v", resp.Changes)
	}
	if len(resp.Warnings) != 1 || resp.Warnings[0] != "review storage policy" {
		t.Fatalf("warnings not preserved: %s", out.String())
	}
	if len(resp.Errors) != 0 {
		t.Fatalf("errors should be empty: %s", out.String())
	}
}

func TestReadOnlyDefaultOutputRemainsText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/service/rest/v1/repositories":
			_, _ = io.WriteString(w, `[{"name":"npm-hosted","format":"npm","type":"hosted"}]`)
		case "/service/rest/v1/blobstores":
			_, _ = io.WriteString(w, `[{"name":"default","type":"file"}]`)
		case "/service/rest/v1/security/privileges":
			_, _ = io.WriteString(w, `[]`)
		case "/service/rest/v1/security/roles/role_guest_repository_access":
			_, _ = io.WriteString(w, `{
				"id":"role_guest_repository_access",
				"name":"role_guest_repository_access",
				"privileges":[],
				"roles":[]
			}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cfgPath := writeAIOutputConfig(t, server.URL)
	tests := []struct {
		name string
		cmd  *cobra.Command
		args []string
		want []string
	}{
		{"repo list", newRepoListCmd(), []string{"--config", cfgPath}, []string{"Repository List", "npm-hosted"}},
		{"blobstore list", newBlobStoreListCmd(), []string{"--config", cfgPath}, []string{"Blob Store List", "default"}},
		{"health check", newHealthCheckCmd(), []string{"--config", cfgPath}, []string{"Health check against", "All checks passed."}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			tt.cmd.SetOut(&out)
			tt.cmd.SetArgs(tt.args)
			if err := tt.cmd.Execute(); err != nil {
				t.Fatalf("execute: %v", err)
			}
			text := out.String()
			if json.Valid(out.Bytes()) {
				t.Fatalf("default output should remain human-readable text, got JSON:\n%s", text)
			}
			for _, want := range tt.want {
				if !strings.Contains(text, want) {
					t.Fatalf("default output missing %q, got:\n%s", want, text)
				}
			}
		})
	}
}

func writeAIOutputConfig(t *testing.T, baseURL string) string {
	t.Helper()
	cfg := config.Default()
	cfg.Nexus.BaseURL = baseURL
	cfg.Nexus.PasswordEnv = "NEXUS_AI_OUTPUT_PASSWORD"
	cfg.Audit.Enabled = false
	cfg.HA.Enabled = true
	cfg.HA.Nodes = []config.HANodeConfig{
		{Name: "primary", Role: "primary", BaseURL: baseURL, PasswordEnv: "NEXUS_AI_OUTPUT_PRIMARY_PASSWORD"},
		{Name: "standby", Role: "standby", BaseURL: baseURL, PasswordEnv: "NEXUS_AI_OUTPUT_STANDBY_PASSWORD"},
	}
	cfg.HA.Replication.StateFile = filepath.Join(t.TempDir(), "ha-state.json")
	t.Setenv("NEXUS_AI_OUTPUT_PASSWORD", "secret")
	t.Setenv("NEXUS_AI_OUTPUT_PRIMARY_PASSWORD", "secret")
	t.Setenv("NEXUS_AI_OUTPUT_STANDBY_PASSWORD", "secret")
	data, err := config.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func writeAIDryRunConfig(t *testing.T, baseURL string) string {
	t.Helper()
	cfg := config.Default()
	cfg.Nexus.BaseURL = baseURL
	cfg.Nexus.PasswordEnv = "NEXUS_AI_DRY_RUN_PASSWORD"
	cfg.Audit.Enabled = false
	cfg.Report.Enabled = false
	cfg.Repositories.Managed = []config.ManagedRepository{
		{
			Name:   "npm-hosted",
			Format: "npm",
			Type:   "hosted",
			Settings: map[string]any{
				"online": true,
			},
		},
	}
	cfg.Repositories.Raw = []config.RawRepository{
		{
			Name:   "raw-hosted",
			Online: true,
			Storage: config.RawStorage{
				BlobStoreName:               "default",
				StrictContentTypeValidation: true,
				WritePolicy:                 "allow_once",
			},
			ContentDisposition: "attachment",
			Lifecycle: config.LifecycleConfig{
				Enabled:       true,
				RetentionDays: 30,
				IncludePaths:  []string{"^releases/.*"},
			},
		},
	}
	cfg.BlobStores.File = []config.FileBlobStore{
		{Name: "default", Path: "/nexus-data/blobs/default"},
	}
	cfg.GuestAccess.BrowseRead.IncludeRepositories = []string{"raw-hosted"}
	cfg.GuestAccess.BrowseRead.ExcludeRepositories = nil
	cfg.GuestAccess.Deny.Repositories = nil
	t.Setenv("NEXUS_AI_DRY_RUN_PASSWORD", "secret")
	data, err := config.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}
