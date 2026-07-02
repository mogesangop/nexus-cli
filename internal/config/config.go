// Package config defines the YAML configuration model for nexus-cli and
// provides loading, default-template generation, and validation.
//
// Field shape follows PRD section 9. Defaults emitted by Default() are a
// generic placeholder template (no environment-specific repository names).
package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config is the root configuration object.
type Config struct {
	Nexus           NexusConfig    `yaml:"nexus"`
	GuestAccess     GuestAccess    `yaml:"guestAccess"`
	PrivilegeNaming PrivilegeNaming `yaml:"privilegeNaming"`
	Audit           AuditConfig    `yaml:"audit"`
	Report          ReportConfig   `yaml:"report"`
}

// NexusConfig holds connection settings for the target Nexus instance.
type NexusConfig struct {
	BaseURL                string `yaml:"baseUrl"`
	Username               string `yaml:"username"`
	PasswordEnv            string `yaml:"passwordEnv"`
	TimeoutSeconds         int    `yaml:"timeoutSeconds"`
	InsecureSkipTLSVerify  bool   `yaml:"insecureSkipTLSVerify"`
}

// GuestAccess configures the guest/anonymous role permission sync.
type GuestAccess struct {
	Enabled              bool             `yaml:"enabled"`
	RoleName             string           `yaml:"roleName"`
	AnonymousUserID      string           `yaml:"anonymousUserId"`
	DefaultPolicy        string           `yaml:"defaultPolicy"`
	BrowseRead           BrowseReadRule   `yaml:"browseRead"`
	ReadOnly             NameList         `yaml:"readOnly"`
	Deny                 NameList         `yaml:"deny"`
	Actions              ActionsConfig    `yaml:"actions"`
	ForbiddenPrivileges  []string         `yaml:"forbiddenPrivileges"`
	WarnPrivileges       []string         `yaml:"warnPrivileges"`
}

// BrowseReadRule selects repositories eligible for browse+read.
type BrowseReadRule struct {
	IncludeRepositories []string `yaml:"includeRepositories"`
	ExcludeRepositories []string `yaml:"excludeRepositories"`
}

// NameList wraps a repository name list under a "repositories" key.
type NameList struct {
	Repositories []string `yaml:"repositories"`
}

// ActionsConfig maps policy names to the Nexus actions they grant.
type ActionsConfig struct {
	BrowseRead []string `yaml:"browseRead"`
	ReadOnly   []string `yaml:"readOnly"`
}

// PrivilegeNaming controls generated privilege name formatting.
type PrivilegeNaming struct {
	Prefix               string `yaml:"prefix"`
	Separator            string `yaml:"separator"`
	ReplaceDashWithUScore bool   `yaml:"replaceDashWithUnderscore"`
}

// AuditConfig controls audit log output.
type AuditConfig struct {
	Enabled       bool   `yaml:"enabled"`
	LogPath       string `yaml:"logPath"`
	MaskSensitive bool   `yaml:"maskSensitive"`
}

// ReportConfig controls execution report output.
type ReportConfig struct {
	Enabled   bool   `yaml:"enabled"`
	OutputDir string `yaml:"outputDir"`
	Format    string `yaml:"format"`
}

// Default returns a generic placeholder configuration template suitable for
// `config init`. It contains no environment-specific repository names; the
// operator must fill in real values before running sync.
func Default() *Config {
	return &Config{
		Nexus: NexusConfig{
			BaseURL:                "http://nexus.example.com",
			Username:               "admin",
			PasswordEnv:            "NEXUS_ADMIN_PASSWORD",
			TimeoutSeconds:         30,
			InsecureSkipTLSVerify:  false,
		},
		GuestAccess: GuestAccess{
			Enabled:         true,
			RoleName:        "role_guest_repository_access",
			AnonymousUserID: "anonymous",
			DefaultPolicy:   "browseRead",
			BrowseRead: BrowseReadRule{
				IncludeRepositories: []string{"*"},
				ExcludeRepositories: []string{"REPLACE_WITH_READ_ONLY_REPO"},
			},
			ReadOnly: NameList{
				Repositories: []string{"REPLACE_WITH_READ_ONLY_REPO"},
			},
			Deny: NameList{
				Repositories: []string{},
			},
			Actions: ActionsConfig{
				BrowseRead: []string{"browse", "read"},
				ReadOnly:   []string{"read"},
			},
			ForbiddenPrivileges: []string{
				"nx-repository-view-*-*-browse",
				"nx-repository-view-*-*-*",
				"nx-all",
				"nx-admin",
			},
			WarnPrivileges: []string{"nx-search-read"},
		},
		PrivilegeNaming: PrivilegeNaming{
			Prefix:                "priv_guest",
			Separator:             "_",
			ReplaceDashWithUScore: true,
		},
		Audit: AuditConfig{
			Enabled:       true,
			LogPath:       "./logs/nexus-cli-audit.log",
			MaskSensitive: true,
		},
		Report: ReportConfig{
			Enabled:   true,
			OutputDir: "./reports",
			Format:    "text",
		},
	}
}

// Load reads and parses a YAML config file from path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return &c, nil
}

// Validate checks required fields and policy consistency.
func (c *Config) Validate() error {
	if c.Nexus.BaseURL == "" {
		return fmt.Errorf("nexus.baseUrl is required")
	}
	if c.Nexus.Username == "" {
		return fmt.Errorf("nexus.username is required")
	}
	if c.Nexus.PasswordEnv == "" {
		return fmt.Errorf("nexus.passwordEnv is required")
	}
	if c.Nexus.TimeoutSeconds <= 0 {
		c.Nexus.TimeoutSeconds = 30
	}
	if c.GuestAccess.RoleName == "" {
		return fmt.Errorf("guestAccess.roleName is required")
	}
	switch c.GuestAccess.DefaultPolicy {
	case "browseRead", "none":
	default:
		return fmt.Errorf("guestAccess.defaultPolicy must be browseRead or none, got %q", c.GuestAccess.DefaultPolicy)
	}
	if c.PrivilegeNaming.Prefix == "" {
		return fmt.Errorf("privilegeNaming.prefix is required")
	}
	if c.PrivilegeNaming.Separator == "" {
		c.PrivilegeNaming.Separator = "_"
	}
	return nil
}

// Marshal renders the config as YAML with a leading header comment.
func Marshal(c *Config) ([]byte, error) {
	out, err := yaml.Marshal(c)
	if err != nil {
		return nil, err
	}
	header := []byte("# nexus-cli configuration (generated by `config init`). Fill in real values.\n")
	return append(header, out...), nil
}

// Password resolves the Nexus admin password from the configured environment
// variable. Returns an error if the variable is unset or empty.
func (c *Config) Password() (string, error) {
	if c.Nexus.PasswordEnv == "" {
		return "", fmt.Errorf("nexus.passwordEnv is not set in config")
	}
	v := os.Getenv(c.Nexus.PasswordEnv)
	if strings.TrimSpace(v) == "" {
		return "", fmt.Errorf("environment variable %s is not set or empty", c.Nexus.PasswordEnv)
	}
	return v, nil
}
