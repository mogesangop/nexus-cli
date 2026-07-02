// Package cli wires nexus-cli's cobra commands. Each command reads a --config
// file, resolves the admin password from the environment, and delegates to the
// guest/nexus packages.
package cli

import (
	"fmt"
	"os"

	"github.com/moge/nexus-cli/internal/config"
	"github.com/moge/nexus-cli/internal/nexus"
	"github.com/spf13/cobra"
)

// Root is the top-level nexus-cli command. It is a thin wrapper around cobra
// so main.go stays minimal.
type Root struct {
	cmd *cobra.Command
}

// NewRoot builds the root command with all subcommands attached.
func NewRoot() *Root {
	root := &cobra.Command{
		Use:   "nexus-cli",
		Short: "Nexus Repository 3 guest access governance CLI",
		Long: "nexus-cli synchronizes Nexus Repository 3.76 guest/anonymous " +
			"role permissions so a repository can be hidden from the UI while " +
			"remaining downloadable via exact URL. See doc/ for the PRD.",
	}
	root.AddCommand(NewConfigCmd(), NewRepoCmd(), NewGuestCmd(), NewHealthCmd())
	return &Root{cmd: root}
}

// Execute runs the root command.
func (r *Root) Execute() error { return r.cmd.Execute() }

// commonFlags holds shared per-command state.
type commonFlags struct {
	configPath string
}

// loadConfig reads and validates the config file.
func loadConfig(path string) (*config.Config, error) {
	return config.Load(path)
}

// newClient builds a Nexus client from config, resolving the password from the
// environment (PRD 19.1: password never lives in the config file).
func newClient(cfg *config.Config) (*nexus.Client, error) {
	pw, err := cfg.Password()
	if err != nil {
		return nil, err
	}
	return nexus.New(cfg.Nexus.BaseURL, cfg.Nexus.Username, pw, cfg.Nexus.TimeoutSeconds, cfg.Nexus.InsecureSkipTLSVerify), nil
}

// fail prints a message to stderr and returns a non-zero exit code.
func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
