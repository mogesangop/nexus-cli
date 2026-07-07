// Package config defines the YAML configuration model for nexus-cli and
// provides loading, default-template generation, and validation.
//
// Field shape follows PRD section 9. Defaults emitted by Default() are a
// generic placeholder template (no environment-specific repository names).
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config is the root configuration object.
type Config struct {
	Nexus           NexusConfig        `yaml:"nexus"`
	HA              HAConfig           `yaml:"ha,omitempty"`
	Repositories    RepositoriesConfig `yaml:"repositories,omitempty"`
	BlobStores      BlobStoresConfig   `yaml:"blobStores,omitempty"`
	GuestAccess     GuestAccess        `yaml:"guestAccess"`
	PrivilegeNaming PrivilegeNaming    `yaml:"privilegeNaming"`
	Audit           AuditConfig        `yaml:"audit"`
	Report          ReportConfig       `yaml:"report"`
}

// RepositoriesConfig contains repositories managed by nexus-cli.
type RepositoriesConfig struct {
	Raw     []RawRepository     `yaml:"raw,omitempty"`
	Managed []ManagedRepository `yaml:"managed,omitempty"`
}

// ManagedRepository is a generic Nexus repository desired state.
type ManagedRepository struct {
	Name     string         `yaml:"name"`
	Format   string         `yaml:"format"`
	Type     string         `yaml:"type"`
	Settings map[string]any `yaml:"settings"`
}

// RawRepository is the desired configuration of a raw hosted repository.
type RawRepository struct {
	Name               string          `yaml:"name"`
	Online             bool            `yaml:"online"`
	Storage            RawStorage      `yaml:"storage"`
	ContentDisposition string          `yaml:"contentDisposition"`
	Lifecycle          LifecycleConfig `yaml:"lifecycle,omitempty"`
}

type RawStorage struct {
	BlobStoreName               string `yaml:"blobStoreName"`
	StrictContentTypeValidation bool   `yaml:"strictContentTypeValidation"`
	WritePolicy                 string `yaml:"writePolicy"`
}

// LifecycleConfig defines CLI-managed retention for raw components.
type LifecycleConfig struct {
	Enabled       bool     `yaml:"enabled"`
	RetentionDays int      `yaml:"retentionDays"`
	IncludePaths  []string `yaml:"includePaths,omitempty"`
	ExcludePaths  []string `yaml:"excludePaths,omitempty"`
}

// BlobStoresConfig contains blob stores managed by nexus-cli.
type BlobStoresConfig struct {
	File []FileBlobStore `yaml:"file,omitempty"`
}

// FileBlobStore is the desired configuration of a file blob store.
type FileBlobStore struct {
	Name      string     `yaml:"name"`
	Path      string     `yaml:"path"`
	SoftQuota *SoftQuota `yaml:"softQuota,omitempty"`
}

type SoftQuota struct {
	Type  string `yaml:"type"`
	Limit int64  `yaml:"limit"`
}

// NexusConfig holds connection settings for the target Nexus instance.
type NexusConfig struct {
	BaseURL               string `yaml:"baseUrl"`
	Username              string `yaml:"username"`
	PasswordEnv           string `yaml:"passwordEnv"`
	TimeoutSeconds        int    `yaml:"timeoutSeconds"`
	InsecureSkipTLSVerify bool   `yaml:"insecureSkipTLSVerify"`
}

// HAConfig holds the optional warm-standby HA settings. When Enabled is false
// or the section is omitted, nexus-cli keeps its single-node behavior.
type HAConfig struct {
	Enabled     bool                `yaml:"enabled"`
	Role        string              `yaml:"role"`
	Nodes       []HANodeConfig      `yaml:"nodes"`
	Replication HAReplicationConfig `yaml:"replication"`
	Failover    HAFailoverConfig    `yaml:"failover"`
}

// HANodeConfig describes one Nexus node participating in the warm-standby pair.
type HANodeConfig struct {
	Name        string `yaml:"name"`
	Role        string `yaml:"role"`
	BaseURL     string `yaml:"baseUrl"`
	Username    string `yaml:"username,omitempty"`
	PasswordEnv string `yaml:"passwordEnv"`
}

// HAReplicationConfig configures operator-provided one-shot sync commands and
// the state file used by `ha status`.
type HAReplicationConfig struct {
	StateFile    string       `yaml:"stateFile"`
	BlobSync     HASyncConfig `yaml:"blobSync"`
	MetadataSync HASyncConfig `yaml:"metadataSync"`
}

// HASyncConfig describes one replication lane.
type HASyncConfig struct {
	Method   string `yaml:"method"`
	Schedule string `yaml:"schedule"`
	Command  string `yaml:"command,omitempty"`
}

// HAFailoverConfig controls the safety gates for manual failover guidance.
type HAFailoverConfig struct {
	Mode           string `yaml:"mode"`
	RequireFencing bool   `yaml:"requireFencing"`
}

// UnmarshalYAML gives requireFencing a safe default even when the failover
// section is present but the field is omitted.
func (h *HAFailoverConfig) UnmarshalYAML(value *yaml.Node) error {
	type raw HAFailoverConfig
	defaulted := raw{Mode: "manual", RequireFencing: true}
	if err := value.Decode(&defaulted); err != nil {
		return err
	}
	*h = HAFailoverConfig(defaulted)
	return nil
}

// GuestAccess configures the guest/anonymous role permission sync.
type GuestAccess struct {
	Enabled             bool           `yaml:"enabled"`
	RoleName            string         `yaml:"roleName"`
	AnonymousUserID     string         `yaml:"anonymousUserId"`
	DefaultPolicy       string         `yaml:"defaultPolicy"`
	BrowseRead          BrowseReadRule `yaml:"browseRead"`
	ReadOnly            NameList       `yaml:"readOnly"`
	Deny                NameList       `yaml:"deny"`
	Actions             ActionsConfig  `yaml:"actions"`
	ForbiddenPrivileges []string       `yaml:"forbiddenPrivileges"`
	WarnPrivileges      []string       `yaml:"warnPrivileges"`
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
	Prefix                string `yaml:"prefix"`
	Separator             string `yaml:"separator"`
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
			BaseURL:               "http://nexus.example.com",
			Username:              "admin",
			PasswordEnv:           "NEXUS_ADMIN_PASSWORD",
			TimeoutSeconds:        30,
			InsecureSkipTLSVerify: false,
		},
		HA: HAConfig{
			Enabled: false,
			Role:    "primary",
			Replication: HAReplicationConfig{
				StateFile: "./logs/nexus-cli-ha-state.json",
				BlobSync: HASyncConfig{
					Method:   "rsync",
					Schedule: "*/5 * * * *",
				},
				MetadataSync: HASyncConfig{
					Method:   "export-import",
					Schedule: "*/15 * * * *",
				},
			},
			Failover: HAFailoverConfig{
				Mode:           "manual",
				RequireFencing: true,
			},
		},
		Repositories: RepositoriesConfig{Raw: []RawRepository{}, Managed: []ManagedRepository{}},
		BlobStores:   BlobStoresConfig{File: []FileBlobStore{}},
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
	if err := c.validateHA(); err != nil {
		return err
	}
	seenRepos := make(map[string]struct{}, len(c.Repositories.Raw))
	for i := range c.Repositories.Raw {
		r := &c.Repositories.Raw[i]
		if strings.TrimSpace(r.Name) == "" {
			return fmt.Errorf("repositories.raw[%d].name is required", i)
		}
		if _, exists := seenRepos[r.Name]; exists {
			return fmt.Errorf("repositories.raw contains duplicate name %q", r.Name)
		}
		seenRepos[r.Name] = struct{}{}
		if strings.TrimSpace(r.Storage.BlobStoreName) == "" {
			return fmt.Errorf("repositories.raw[%d].storage.blobStoreName is required", i)
		}
		if r.Storage.WritePolicy == "" {
			r.Storage.WritePolicy = "allow_once"
		}
		switch r.Storage.WritePolicy {
		case "allow", "allow_once", "deny":
		default:
			return fmt.Errorf("repositories.raw[%d].storage.writePolicy must be allow, allow_once, or deny", i)
		}
		if r.ContentDisposition == "" {
			r.ContentDisposition = "attachment"
		}
		switch r.ContentDisposition {
		case "attachment", "inline":
		default:
			return fmt.Errorf("repositories.raw[%d].contentDisposition must be attachment or inline", i)
		}
		if r.Lifecycle.Enabled && r.Lifecycle.RetentionDays <= 0 {
			return fmt.Errorf("repositories.raw[%d].lifecycle.retentionDays must be greater than zero", i)
		}
		for _, expression := range append(append([]string{}, r.Lifecycle.IncludePaths...), r.Lifecycle.ExcludePaths...) {
			if _, err := regexp.Compile(expression); err != nil {
				return fmt.Errorf("repositories.raw[%d].lifecycle invalid path regex %q: %w", i, expression, err)
			}
		}
	}
	seenManagedRepos := make(map[string]struct{}, len(c.Repositories.Managed))
	for i := range c.Repositories.Managed {
		r := &c.Repositories.Managed[i]
		if strings.TrimSpace(r.Name) == "" {
			return fmt.Errorf("repositories.managed[%d].name is required", i)
		}
		if strings.TrimSpace(r.Format) == "" {
			return fmt.Errorf("repositories.managed[%d].format is required", i)
		}
		if strings.TrimSpace(r.Type) == "" {
			return fmt.Errorf("repositories.managed[%d].type is required", i)
		}
		if _, exists := seenManagedRepos[r.Name]; exists {
			return fmt.Errorf("repositories.managed contains duplicate name %q", r.Name)
		}
		seenManagedRepos[r.Name] = struct{}{}
		if r.Settings == nil {
			r.Settings = map[string]any{}
		}
		if v, ok := r.Settings["name"].(string); ok && v != "" && v != r.Name {
			return fmt.Errorf("repositories.managed[%d].settings.name must match name %q", i, r.Name)
		}
	}
	seenFileBlobStores := make(map[string]struct{}, len(c.BlobStores.File))
	for i := range c.BlobStores.File {
		b := &c.BlobStores.File[i]
		if strings.TrimSpace(b.Name) == "" {
			return fmt.Errorf("blobStores.file[%d].name is required", i)
		}
		if strings.TrimSpace(b.Path) == "" {
			return fmt.Errorf("blobStores.file[%d].path is required", i)
		}
		if _, exists := seenFileBlobStores[b.Name]; exists {
			return fmt.Errorf("blobStores.file contains duplicate name %q", b.Name)
		}
		seenFileBlobStores[b.Name] = struct{}{}
		if b.SoftQuota != nil {
			if strings.TrimSpace(b.SoftQuota.Type) == "" {
				return fmt.Errorf("blobStores.file[%d].softQuota.type is required", i)
			}
			if b.SoftQuota.Limit <= 0 {
				return fmt.Errorf("blobStores.file[%d].softQuota.limit must be greater than zero", i)
			}
		}
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

func (c *Config) validateHA() error {
	if c.HA.Failover.Mode == "" {
		c.HA.Failover.Mode = "manual"
		c.HA.Failover.RequireFencing = true
	}
	if c.HA.Replication.StateFile == "" {
		c.HA.Replication.StateFile = "./logs/nexus-cli-ha-state.json"
	}
	if c.HA.Replication.BlobSync.Method == "" {
		c.HA.Replication.BlobSync.Method = "rsync"
	}
	if c.HA.Replication.BlobSync.Schedule == "" {
		c.HA.Replication.BlobSync.Schedule = "*/5 * * * *"
	}
	if c.HA.Replication.MetadataSync.Method == "" {
		c.HA.Replication.MetadataSync.Method = "export-import"
	}
	if c.HA.Replication.MetadataSync.Schedule == "" {
		c.HA.Replication.MetadataSync.Schedule = "*/15 * * * *"
	}
	if !c.HA.Enabled {
		return nil
	}
	switch c.HA.Role {
	case "primary", "standby":
	default:
		return fmt.Errorf("ha.role must be primary or standby")
	}
	if c.HA.Failover.Mode != "manual" {
		return fmt.Errorf("ha.failover.mode must be manual")
	}
	switch c.HA.Replication.BlobSync.Method {
	case "rsync", "rclone":
	default:
		return fmt.Errorf("ha.replication.blobSync.method must be rsync or rclone")
	}
	if c.HA.Replication.BlobSync.Schedule == "" {
		return fmt.Errorf("ha.replication.blobSync.schedule is required")
	}
	if c.HA.Replication.MetadataSync.Method != "export-import" {
		return fmt.Errorf("ha.replication.metadataSync.method must be export-import")
	}
	if c.HA.Replication.MetadataSync.Schedule == "" {
		return fmt.Errorf("ha.replication.metadataSync.schedule is required")
	}
	if len(c.HA.Nodes) != 2 {
		return fmt.Errorf("ha.nodes must contain exactly one primary and one standby node")
	}
	seenNames := map[string]struct{}{}
	roleCounts := map[string]int{}
	for i := range c.HA.Nodes {
		n := &c.HA.Nodes[i]
		if strings.TrimSpace(n.Name) == "" {
			return fmt.Errorf("ha.nodes[%d].name is required", i)
		}
		if _, exists := seenNames[n.Name]; exists {
			return fmt.Errorf("ha.nodes contains duplicate name %q", n.Name)
		}
		seenNames[n.Name] = struct{}{}
		switch n.Role {
		case "primary", "standby":
			roleCounts[n.Role]++
		default:
			return fmt.Errorf("ha.nodes[%d].role must be primary or standby", i)
		}
		if strings.TrimSpace(n.BaseURL) == "" {
			return fmt.Errorf("ha.nodes[%d].baseUrl is required", i)
		}
		if strings.TrimSpace(n.PasswordEnv) == "" {
			return fmt.Errorf("ha.nodes[%d].passwordEnv is required", i)
		}
		if strings.TrimSpace(n.Username) == "" {
			n.Username = c.Nexus.Username
		}
	}
	if roleCounts["primary"] != 1 || roleCounts["standby"] != 1 {
		return fmt.Errorf("ha.nodes must contain exactly one primary and one standby node")
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

// HANodePassword resolves the password for an HA node by name.
func (c *Config) HANodePassword(node HANodeConfig) (string, error) {
	if node.PasswordEnv == "" {
		return "", fmt.Errorf("ha.nodes[%s].passwordEnv is not set in config", node.Name)
	}
	v := os.Getenv(node.PasswordEnv)
	if strings.TrimSpace(v) == "" {
		return "", fmt.Errorf("environment variable %s is not set or empty", node.PasswordEnv)
	}
	return v, nil
}

// Resolve finds the config file path. If explicit is non-empty it is used
// as-is (highest priority, no existence check — a typo surfaces as a Load
// read error rather than silently falling through to search). Otherwise the
// search order is:
//
//	./config.yaml, ~/.nexus-cli/config.yaml, /etc/nexus-cli/config.yaml
//
// First existing file wins. When $HOME is unset the home tier is skipped so
// the CLI stays usable in headless containers. Returns an error listing all
// searched paths when none exist.
func Resolve(explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	var candidates []string
	if cwdPath, err := filepath.Abs("config.yaml"); err == nil {
		candidates = append(candidates, cwdPath)
	} else {
		candidates = append(candidates, "config.yaml")
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		candidates = append(candidates, filepath.Join(home, ".nexus-cli", "config.yaml"))
	}
	candidates = append(candidates, "/etc/nexus-cli/config.yaml")

	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	var b strings.Builder
	b.WriteString("no config file found. Searched:")
	for _, p := range candidates {
		b.WriteString("\n  - ")
		b.WriteString(p)
	}
	b.WriteString("\nCreate one with `nexus-cli config init`, or pass --config <path>.")
	return "", fmt.Errorf("%s", b.String())
}
