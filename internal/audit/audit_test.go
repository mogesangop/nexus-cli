package audit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoggerMasksSensitiveErrorFragments(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	logger := New(path, true, true)
	err := logger.Write(Record{
		Command:      "user create-readonly",
		DryRun:       false,
		Action:       "grant",
		Result:       "failed",
		ErrorMessage: `upstream rejected Authorization: Basic abc123 password=plain {"newPassword":"generated"}`,
	})
	if err != nil {
		t.Fatalf("write audit: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read audit: %v", err)
	}
	text := string(data)
	for _, forbidden := range []string{"abc123", "plain", "generated", "Authorization: Basic"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("audit log contains sensitive fragment %q: %s", forbidden, text)
		}
	}
	for _, want := range []string{"auth=[REDACTED]", "password=[REDACTED]", `"newPassword\":\"[REDACTED]\"`} {
		if !strings.Contains(text, want) {
			t.Fatalf("audit log missing redaction marker %q: %s", want, text)
		}
	}
}
