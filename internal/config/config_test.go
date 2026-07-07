package config

import (
	"os"
	"path/filepath"
	"strings"
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

func TestValidateRawRepositories(t *testing.T) {
	valid := RawRepository{
		Name: "releases", Online: true,
		Storage:            RawStorage{BlobStoreName: "default", WritePolicy: "allow_once"},
		ContentDisposition: "attachment",
		Lifecycle:          LifecycleConfig{Enabled: true, RetentionDays: 30, IncludePaths: []string{"^releases/"}},
	}
	t.Run("valid", func(t *testing.T) {
		c := Default()
		c.Repositories.Raw = []RawRepository{valid}
		if err := c.Validate(); err != nil {
			t.Fatalf("Validate: %v", err)
		}
	})
	t.Run("missing blob store", func(t *testing.T) {
		c := Default()
		r := valid
		r.Storage.BlobStoreName = ""
		c.Repositories.Raw = []RawRepository{r}
		if err := c.Validate(); err == nil {
			t.Fatal("expected validation error")
		}
	})
	t.Run("invalid retention", func(t *testing.T) {
		c := Default()
		r := valid
		r.Lifecycle.RetentionDays = 0
		c.Repositories.Raw = []RawRepository{r}
		if err := c.Validate(); err == nil {
			t.Fatal("expected validation error")
		}
	})
	t.Run("invalid regex", func(t *testing.T) {
		c := Default()
		r := valid
		r.Lifecycle.IncludePaths = []string{"["}
		c.Repositories.Raw = []RawRepository{r}
		if err := c.Validate(); err == nil {
			t.Fatal("expected validation error")
		}
	})
	t.Run("duplicate", func(t *testing.T) {
		c := Default()
		c.Repositories.Raw = []RawRepository{valid, valid}
		if err := c.Validate(); err == nil {
			t.Fatal("expected validation error")
		}
	})
}

func TestValidateManagedRepositories(t *testing.T) {
	valid := ManagedRepository{
		Name: "npm-hosted", Format: "npm", Type: "hosted",
		Settings: map[string]any{"name": "npm-hosted", "online": true},
	}
	t.Run("valid", func(t *testing.T) {
		c := Default()
		c.Repositories.Managed = []ManagedRepository{valid}
		if err := c.Validate(); err != nil {
			t.Fatalf("Validate: %v", err)
		}
	})
	t.Run("missing format", func(t *testing.T) {
		c := Default()
		r := valid
		r.Format = ""
		c.Repositories.Managed = []ManagedRepository{r}
		if err := c.Validate(); err == nil {
			t.Fatal("expected validation error")
		}
	})
	t.Run("duplicate", func(t *testing.T) {
		c := Default()
		c.Repositories.Managed = []ManagedRepository{valid, valid}
		if err := c.Validate(); err == nil {
			t.Fatal("expected validation error")
		}
	})
	t.Run("settings name mismatch", func(t *testing.T) {
		c := Default()
		r := valid
		r.Settings = map[string]any{"name": "other"}
		c.Repositories.Managed = []ManagedRepository{r}
		if err := c.Validate(); err == nil {
			t.Fatal("expected validation error")
		}
	})
}

func TestValidateFileBlobStores(t *testing.T) {
	valid := FileBlobStore{Name: "default", Path: "/nexus-data/blobs/default"}
	t.Run("valid", func(t *testing.T) {
		c := Default()
		c.BlobStores.File = []FileBlobStore{valid}
		if err := c.Validate(); err != nil {
			t.Fatalf("Validate: %v", err)
		}
	})
	t.Run("missing path", func(t *testing.T) {
		c := Default()
		b := valid
		b.Path = ""
		c.BlobStores.File = []FileBlobStore{b}
		if err := c.Validate(); err == nil {
			t.Fatal("expected validation error")
		}
	})
	t.Run("duplicate", func(t *testing.T) {
		c := Default()
		c.BlobStores.File = []FileBlobStore{valid, valid}
		if err := c.Validate(); err == nil {
			t.Fatal("expected validation error")
		}
	})
	t.Run("bad soft quota", func(t *testing.T) {
		c := Default()
		b := valid
		b.SoftQuota = &SoftQuota{Type: "spaceRemainingQuota", Limit: 0}
		c.BlobStores.File = []FileBlobStore{b}
		if err := c.Validate(); err == nil {
			t.Fatal("expected validation error")
		}
	})
}

func writeConfig(t *testing.T, path string) {
	t.Helper()
	data, err := Marshal(Default())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// chdir changes to dir for the duration of the test. Go 1.22 has no t.Chdir.
func chdir(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
}

func TestResolve_ExplicitReturnedAsIs(t *testing.T) {
	got, err := Resolve("/nonexistent/explicit.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/nonexistent/explicit.yaml" {
		t.Fatalf("got %q want /nonexistent/explicit.yaml", got)
	}
}

func TestResolve_CwdWins(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	writeConfig(t, filepath.Join(tmpHome, ".nexus-cli", "config.yaml"))

	cwd := t.TempDir()
	chdir(t, cwd)
	writeConfig(t, filepath.Join(cwd, "config.yaml"))

	got, err := Resolve("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	abs, _ := filepath.Abs("config.yaml")
	if got != abs {
		t.Fatalf("got %q want cwd config %q", got, abs)
	}
}

func TestResolve_HomeWinsWhenNoCwd(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	homePath := filepath.Join(tmpHome, ".nexus-cli", "config.yaml")
	writeConfig(t, homePath)

	chdir(t, t.TempDir())

	got, err := Resolve("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != homePath {
		t.Fatalf("got %q want %q", got, homePath)
	}
}

func TestResolve_NoHomeSkipsHomeTier(t *testing.T) {
	t.Setenv("HOME", "")
	chdir(t, t.TempDir())

	_, err := Resolve("")
	if err == nil {
		t.Fatal("expected not-found error")
	}
	msg := err.Error()
	if strings.Contains(msg, ".nexus-cli") {
		t.Fatalf("home tier should be absent, error was:\n%s", msg)
	}
	if !strings.Contains(msg, "config.yaml") {
		t.Fatalf("cwd candidate missing, error was:\n%s", msg)
	}
	if !strings.Contains(msg, "/etc/nexus-cli/config.yaml") {
		t.Fatalf("/etc candidate missing, error was:\n%s", msg)
	}
}

func TestResolve_NotFoundListsAllSearched(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	chdir(t, t.TempDir())

	_, err := Resolve("")
	if err == nil {
		t.Fatal("expected not-found error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "no config file found") {
		t.Fatalf("missing header, error was:\n%s", msg)
	}
	if !strings.Contains(msg, "config.yaml") {
		t.Fatalf("cwd candidate missing, error was:\n%s", msg)
	}
	if !strings.Contains(msg, ".nexus-cli") {
		t.Fatalf("home candidate missing, error was:\n%s", msg)
	}
	if !strings.Contains(msg, "/etc/nexus-cli/config.yaml") {
		t.Fatalf("/etc candidate missing, error was:\n%s", msg)
	}
	if !strings.Contains(msg, "config init") {
		t.Fatalf("missing config init hint, error was:\n%s", msg)
	}
}
