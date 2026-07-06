package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/231397220/nexus-cli/internal/config"
	"github.com/spf13/cobra"
)

// marshalYAML is a thin wrapper over config.Marshal kept in the cli package so
// command files read top-down without importing yaml.v3 directly.
func marshalYAML(c *config.Config) ([]byte, error) { return config.Marshal(c) }

// NewConfigCmd builds the `config` command tree.
func NewConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Configuration management",
	}
	cmd.AddCommand(newConfigInitCmd())
	return cmd
}

func newConfigInitCmd() *cobra.Command {
	var output string
	c := &cobra.Command{
		Use:   "init",
		Short: "Generate a default config.yaml template",
		RunE: func(cmd *cobra.Command, args []string) error {
			if output == "" {
				home, err := os.UserHomeDir()
				if err != nil {
					return fmt.Errorf("resolve home directory for default output: %w", err)
				}
				output = filepath.Join(home, ".nexus-cli", "config.yaml")
			}
			if err := os.MkdirAll(filepath.Dir(output), 0o700); err != nil {
				return fmt.Errorf("create config directory: %w", err)
			}
			cfg := config.Default()
			data, err := marshalYAML(cfg)
			if err != nil {
				return err
			}
			if err := os.WriteFile(output, data, 0o644); err != nil {
				return fmt.Errorf("write %s: %w", output, err)
			}
			fmt.Printf("Generated %s\n", output)
			fmt.Println("Edit the file (especially baseUrl, passwordEnv, roleName, repository lists) before running sync.")
			return nil
		},
	}
	c.Flags().StringVarP(&output, "output", "o", "", "output config file path (default: ~/.nexus-cli/config.yaml)")
	return c
}
