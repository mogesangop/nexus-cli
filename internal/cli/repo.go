package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// NewRepoCmd builds the `repo` command tree.
func NewRepoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repo",
		Short: "Nexus repository operations",
	}
	cmd.AddCommand(newRepoListCmd())
	return cmd
}

func newRepoListCmd() *cobra.Command {
	var cfgPath string
	c := &cobra.Command{
		Use:   "list",
		Short: "List all repositories (name, format, type)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(cfgPath)
			if err != nil {
				return err
			}
			client, err := newClient(cfg)
			if err != nil {
				return err
			}
			repos, err := client.ListRepositories()
			if err != nil {
				return err
			}
			fmt.Println("Repository List")
			fmt.Printf("%-32s %-12s %-12s\n", "Name", "Format", "Type")
			for _, r := range repos {
				fmt.Printf("%-32s %-12s %-12s\n", r.Name, r.Format, r.Type)
			}
			fmt.Printf("Total: %d\n", len(repos))
			return nil
		},
	}
	c.Flags().StringVar(&cfgPath, "config", "config.yaml", "config file path")
	return c
}
