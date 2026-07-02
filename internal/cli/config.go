package cli

import (
	"fmt"
	"os"

	"github.com/moge/nexus-cli/internal/config"
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
				output = "config.yaml"
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
	c.Flags().StringVarP(&output, "output", "o", "config.yaml", "output config file path")
	return c
}
