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
	var cfgPath, output string
	c := &cobra.Command{
		Use:   "check",
		Short: "Verify Nexus connectivity, auth, and required API endpoints",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWithJSONErrors(cmd, output, "health check", func() error {
				if err := validateOutput(output); err != nil {
					return err
				}
				cfg, err := loadConfig(cfgPath)
				if err != nil {
					return err
				}
				client, err := newClient(cfg)
				if err != nil {
					return err
				}

				checks := []healthCheck{
					{"list repositories", func() error { _, err := client.ListRepositories(); return err }},
					{"list privileges", func() error { _, err := client.ListPrivileges(); return err }},
					{"read guest role", func() error { _, err := client.GetRole(cfg.GuestAccess.RoleName); return err }},
				}

				results, warnings, failed, firstErr := runHealthChecks(checks)
				if isJSONOutput(output) {
					if failed > 0 {
						return firstErr
					}
					return writeReadOnlyResponse(cmd, "health check", "success", map[string]any{
						"baseUrl": cfg.Nexus.BaseURL,
						"checks":  results,
						"failed":  failed,
					}, warnings)
				}

				out := cmd.OutOrStdout()
				fmt.Fprintf(out, "Health check against %s\n", cfg.Nexus.BaseURL)
				for _, result := range results {
					switch result.Status {
					case "warn":
						fmt.Fprintf(out, "  WARN  %-22s %s\n", result.Name, result.Error)
					case "fail":
						fmt.Fprintf(out, "  FAIL  %-22s %s\n", result.Name, result.Error)
					default:
						fmt.Fprintf(out, "  OK    %-22s\n", result.Name)
					}
				}
				if failed > 0 {
					return fmt.Errorf("%d health check(s) failed", failed)
				}
				fmt.Fprintln(out, "All checks passed.")
				return nil
			})
		},
	}
	c.Flags().StringVar(&cfgPath, "config", "", "config file path (searched if unset: ./, ~/.nexus-cli/, /etc/nexus-cli/)")
	addOutputFlag(c, &output)
	return c
}

type healthCheck struct {
	name string
	fn   func() error
}

type healthCheckResult struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

func runHealthChecks(checks []healthCheck) ([]healthCheckResult, []string, int, error) {
	results := make([]healthCheckResult, 0, len(checks))
	var warnings []string
	failed := 0
	var firstErr error
	for _, ch := range checks {
		if err := ch.fn(); err != nil {
			item := healthCheckResult{Name: ch.name, Error: err.Error()}
			if nexus.IsNotFound(err) {
				item.Status = "warn"
				warnings = append(warnings, fmt.Sprintf("%s: %s", ch.name, err))
			} else {
				item.Status = "fail"
				failed++
				if firstErr == nil {
					firstErr = err
				}
			}
			results = append(results, item)
			continue
		}
		results = append(results, healthCheckResult{Name: ch.name, Status: "ok"})
	}
	if firstErr == nil && failed > 0 {
		firstErr = fmt.Errorf("%d health check(s) failed", failed)
	}
	return results, warnings, failed, firstErr
}
