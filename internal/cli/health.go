package cli

import (
	"fmt"

	"github.com/231397220/nexus-cli/internal/nexus"
	"github.com/spf13/cobra"
)

// NewHealthCmd builds the `health` command tree.
func NewHealthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "health",
		Short: "Nexus connectivity and API health checks",
	}
	cmd.AddCommand(newHealthCheckCmd())
	return cmd
}

func newHealthCheckCmd() *cobra.Command {
	var cfgPath string
	c := &cobra.Command{
		Use:   "check",
		Short: "Verify Nexus connectivity, auth, and required API endpoints",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(cfgPath)
			if err != nil {
				return err
			}
			client, err := newClient(cfg)
			if err != nil {
				return err
			}

			type check struct {
				name string
				fn   func() error
			}
			checks := []check{
				{"list repositories", func() error { _, err := client.ListRepositories(); return err }},
				{"list privileges", func() error { _, err := client.ListPrivileges(); return err }},
				{"read guest role", func() error { _, err := client.GetRole(cfg.GuestAccess.RoleName); return err }},
			}

			fmt.Printf("Health check against %s\n", cfg.Nexus.BaseURL)
			failed := 0
			for _, ch := range checks {
				if err := ch.fn(); err != nil {
					if nexus.IsNotFound(err) {
						// A 404 on the role check is informative, not fatal.
						fmt.Printf("  WARN  %-22s %s\n", ch.name, err)
					} else {
						fmt.Printf("  FAIL  %-22s %s\n", ch.name, err)
						failed++
					}
					continue
				}
				fmt.Printf("  OK    %-22s\n", ch.name)
			}
			if failed > 0 {
				return fmt.Errorf("%d health check(s) failed", failed)
			}
			fmt.Println("All checks passed.")
			return nil
		},
	}
	c.Flags().StringVar(&cfgPath, "config", "", "config file path (searched if unset: ./, ~/.nexus-cli/, /etc/nexus-cli/)")
	return c
}
