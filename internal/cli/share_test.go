package cli

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/231397220/nexus-cli/internal/config"
	"gopkg.in/yaml.v3"
)

func TestShareGrantHelpDescribesRawExclusivePreflight(t *testing.T) {
	cmd := NewShareCmd()
	grant, _, err := cmd.Find([]string{"grant"})
	if err != nil || grant == nil || grant.Name() != "grant" {
		t.Fatalf("share grant command not found: cmd=%v err=%v", grant, err)
	}
	if !strings.Contains(grant.Long, "raw repository") {
		t.Fatalf("share grant help should mention raw repository scope, got %q", grant.Long)
	}
	if !strings.Contains(grant.Long, "overlapping path access") {
		t.Fatalf("share grant help should mention exclusive preflight, got %q", grant.Long)
	}
}

func TestShareGrantDryRunAuditIncludesTargetsWithoutSecrets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/service/rest/v1/repositories":
			_, _ = io.WriteString(w, `[{"name":"raw-hosted","format":"raw","type":"hosted"}]`)
		case "/service/rest/v1/security/privileges":
			_, _ = io.WriteString(w, `[]`)
		case "/service/rest/v1/security/roles":
			_, _ = io.WriteString(w, `[]`)
		case "/service/rest/v1/security/users":
			_, _ = io.WriteString(w, `[]`)
		case "/service/rest/v1/security/content-selectors":
			_, _ = io.WriteString(w, `[]`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	auditPath := filepath.Join(t.TempDir(), "audit.jsonl")
	cfgPath := writeShareAuditConfig(t, server.URL, auditPath)
	out, err := executeCommandForOutput(newShareGrantCmd(),
		"--config", cfgPath,
		"--repo", "raw-hosted",
		"--path", "/team-a/",
		"--user", "team-a-user",
		"--email", "team-a@example.com",
		"--dry-run",
		"--output", "json",
	)
	if err != nil {
		t.Fatalf("execute: %v\nstdout=%s", err, out)
	}
	data, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("read audit: %v", err)
	}
	text := string(data)
	for _, want := range []string{
		`"command":"share grant"`,
		`"dryRun":true`,
		`"result":"success"`,
		`"targetRepo":"raw-hosted"`,
		`"targetPath":"/team-a/"`,
		`"targetUser":"team-a-user"`,
		`"targetRole":"role_share_`,
		`"createdSelectors":["sel_share_`,
		`"createdPrivileges":["priv_share_`,
		`"createdUsers":["team-a-user"]`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("audit log missing %q: %s", want, text)
		}
	}
	for _, forbidden := range []string{"SHARE_AUDIT_SECRET", "Authorization", "newPassword"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("audit log contains sensitive fragment %q: %s", forbidden, text)
		}
	}
}

func writeShareAuditConfig(t *testing.T, baseURL, auditPath string) string {
	t.Helper()
	cfg := config.Default()
	cfg.Nexus.BaseURL = baseURL
	cfg.Nexus.PasswordEnv = "NEXUS_SHARE_AUDIT_PASSWORD"
	cfg.Audit.Enabled = true
	cfg.Audit.LogPath = auditPath
	cfg.Audit.MaskSensitive = true
	t.Setenv("NEXUS_SHARE_AUDIT_PASSWORD", "SHARE_AUDIT_SECRET")
	data, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}
