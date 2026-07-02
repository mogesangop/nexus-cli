package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidate_RequiresBaseURL(t *testing.T) {
	c := Default()
	c.Nexus.BaseURL = ""
	if err := c.Validate(); err == nil {
		t.Fatal("expected error for missing baseUrl")
	}
}

func TestValidate_BadDefaultPolicy(t *testing.T) {
	c := Default()
	c.GuestAccess.DefaultPolicy = "weird"
	if err := c.Validate(); err == nil {
		t.Fatal("expected error for bad defaultPolicy")
	}
}

func TestValidate_OK(t *testing.T) {
	c := Default()
	if err := c.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPassword_FromEnv(t *testing.T) {
	c := Default()
	t.Setenv("NEXUS_ADMIN_PASSWORD", "s3cr3t")
	pw, err := c.Password()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pw != "s3cr3t" {
		t.Fatalf("got %q want s3cr3t", pw)
	}
}

func TestPassword_MissingEnv(t *testing.T) {
	c := Default()
	os.Unsetenv("NEXUS_ADMIN_PASSWORD")
	if _, err := c.Password(); err == nil {
		t.Fatal("expected error when password env is unset")
	}
}

func TestLoad_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "c.yaml")
	c := Default()
	data, err := Marshal(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Nexus.BaseURL != c.Nexus.BaseURL {
		t.Errorf("baseURL mismatch: %q vs %q", loaded.Nexus.BaseURL, c.Nexus.BaseURL)
	}
	if loaded.GuestAccess.RoleName != c.GuestAccess.RoleName {
		t.Errorf("roleName mismatch")
	}
}
