package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/231397220/nexus-cli/internal/audit"
	"github.com/231397220/nexus-cli/internal/config"
	haops "github.com/231397220/nexus-cli/internal/ha"
	"github.com/231397220/nexus-cli/internal/nexus"
	"github.com/spf13/cobra"
)

// NewHACmd builds the `ha` command tree.
func NewHACmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ha",
		Short: "Warm-standby HA status, sync, and manual failover guidance",
		Long: "Warm-standby HA helpers for two independent Nexus OSS nodes. " +
			"F5 switching remains a manual operator action; this command group " +
			"checks health, runs configured sync commands, enforces fencing " +
			"confirmation, and writes audit records.",
	}
	cmd.AddCommand(newHAStatusCmd(), newHAHealthCmd(), newHASyncCmd(), newHAFailoverCmd())
	return cmd
}

func newHAStatusCmd() *cobra.Command {
	var cfgPath string
	c := &cobra.Command{
		Use:   "status",
		Short: "Show both node health and replication lag",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(cfgPath)
			if err != nil {
				return err
			}
			if err := requireHA(cfg); err != nil {
				return err
			}
			state, err := haops.LoadState(cfg.HA.Replication.StateFile)
			if err != nil {
				return err
			}
			fmt.Printf("HA status: enabled=%t localRole=%s mode=%s requireFencing=%t\n",
				cfg.HA.Enabled, cfg.HA.Role, cfg.HA.Failover.Mode, cfg.HA.Failover.RequireFencing)
			fmt.Println("Nodes:")
			failed := printNodeHealth(cfg, false)
			fmt.Printf("Replication state: %s\n", cfg.HA.Replication.StateFile)
			now := time.Now()
			for _, name := range []string{"blob", "metadata"} {
				job := state.Jobs[name]
				lag, ok := haops.Lag(job.LastSuccessAt, now)
				lagText := "unknown"
				if ok {
					lagText = lag.Round(time.Second).String()
				}
				status := "OK"
				if job.LastSuccessAt == "" {
					status = "UNKNOWN"
				}
				if job.LastError != "" {
					status = "ERROR"
				}
				fmt.Printf("  %-8s %-7s method=%s schedule=%q lastSuccess=%s lag=%s\n",
					name, status, fallback(job.Method, "-"), job.Schedule, fallback(job.LastSuccessAt, "-"), lagText)
				if job.LastError != "" {
					fmt.Printf("           lastErrorAt=%s error=%s\n", fallback(job.LastErrorAt, "-"), job.LastError)
				}
			}
			if failed > 0 {
				return fmt.Errorf("%d HA node health check(s) failed", failed)
			}
			return nil
		},
	}
	c.Flags().StringVar(&cfgPath, "config", "", "config file path (searched if unset: ./, ~/.nexus-cli/, /etc/nexus-cli/)")
	return c
}

func newHAHealthCmd() *cobra.Command {
	var cfgPath string
	c := &cobra.Command{
		Use:   "health",
		Short: "Run API health checks against both HA nodes",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(cfgPath)
			if err != nil {
				return err
			}
			if err := requireHA(cfg); err != nil {
				return err
			}
			failed := printNodeHealth(cfg, true)
			if failed > 0 {
				return fmt.Errorf("%d HA node health check(s) failed", failed)
			}
			fmt.Println("All HA node checks passed.")
			return nil
		},
	}
	c.Flags().StringVar(&cfgPath, "config", "", "config file path (searched if unset: ./, ~/.nexus-cli/, /etc/nexus-cli/)")
	return c
}

func newHASyncCmd() *cobra.Command {
	var (
		cfgPath string
		once    bool
		timeout time.Duration
	)
	c := &cobra.Command{
		Use:   "sync --once",
		Short: "Run configured blob and metadata sync commands once",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !once {
				return fmt.Errorf("ha sync currently supports only --once")
			}
			cfg, err := loadConfig(cfgPath)
			if err != nil {
				return err
			}
			if err := requireHA(cfg); err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()
			results, err := haops.Runner{Config: cfg}.SyncOnce(ctx)
			for _, r := range results {
				status := "OK"
				if r.Err != nil {
					status = "FAIL"
				}
				fmt.Printf("%-8s %-4s method=%s duration=%s\n", r.Name, status, r.Method, r.Duration.Round(time.Millisecond))
				if r.Err != nil {
					fmt.Printf("         %s\n", r.Err)
				}
			}
			writeHAAudit(cfg, "ha sync --once", "sync", err)
			if err != nil {
				return err
			}
			fmt.Printf("HA sync state updated: %s\n", cfg.HA.Replication.StateFile)
			return nil
		},
	}
	c.Flags().StringVar(&cfgPath, "config", "", "config file path (searched if unset: ./, ~/.nexus-cli/, /etc/nexus-cli/)")
	c.Flags().BoolVar(&once, "once", false, "execute one immediate blob + metadata sync")
	c.Flags().DurationVar(&timeout, "timeout", 30*time.Minute, "maximum duration for the one-shot sync")
	return c
}

func newHAFailoverCmd() *cobra.Command {
	var (
		cfgPath          string
		from             string
		to               string
		fencingConfirmed bool
		skipSync         bool
		timeout          time.Duration
	)
	c := &cobra.Command{
		Use:   "failover --from primary --to standby --fencing-confirmed",
		Short: "Guide a safe manual failover and write an audit record",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(cfgPath)
			if err != nil {
				return err
			}
			if err := requireHA(cfg); err != nil {
				return err
			}
			src, err := resolveHANode(cfg, from, "primary")
			if err != nil {
				return err
			}
			dst, err := resolveHANode(cfg, to, "standby")
			if err != nil {
				return err
			}
			if src.Name == dst.Name {
				return fmt.Errorf("--from and --to resolve to the same node %q", src.Name)
			}
			if cfg.HA.Failover.RequireFencing && !fencingConfirmed {
				err := fmt.Errorf("fencing confirmation required; stop or isolate %s first, then rerun with --fencing-confirmed", src.Name)
				writeHAAudit(cfg, "ha failover", "failover", err)
				return err
			}

			fmt.Printf("Manual failover plan: %s (%s) -> %s (%s)\n", src.Name, src.BaseURL, dst.Name, dst.BaseURL)
			fmt.Println("Fencing: confirmed old primary is stopped or isolated.")
			if !skipSync {
				ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
				defer cancel()
				fmt.Println("Running final catch-up sync before F5 switch...")
				results, err := haops.Runner{Config: cfg}.SyncOnce(ctx)
				for _, r := range results {
					status := "OK"
					if r.Err != nil {
						status = "FAIL"
					}
					fmt.Printf("  %-8s %-4s duration=%s\n", r.Name, status, r.Duration.Round(time.Millisecond))
					if r.Err != nil {
						fmt.Printf("           %s\n", r.Err)
					}
				}
				if err != nil {
					writeHAAudit(cfg, "ha failover", "failover", err)
					return fmt.Errorf("catch-up sync failed; rerun with --skip-sync only if the old primary is unreachable and RPO loss is accepted: %w", err)
				}
			} else {
				fmt.Println("Catch-up sync: skipped by operator; record the accepted RPO window in the incident.")
			}
			fmt.Println("Next manual steps:")
			fmt.Printf("  1. In F5, remove/disable %s from the active pool.\n", src.Name)
			fmt.Printf("  2. Add/enable %s as the only active Nexus pool member.\n", dst.Name)
			fmt.Println("  3. Verify anonymous download through the F5 virtual service.")
			fmt.Println("  4. Run `nexus-cli guest check` against the active service to verify permissions.")
			writeHAAudit(cfg, "ha failover", "failover", nil)
			return nil
		},
	}
	c.Flags().StringVar(&cfgPath, "config", "", "config file path (searched if unset: ./, ~/.nexus-cli/, /etc/nexus-cli/)")
	c.Flags().StringVar(&from, "from", "primary", "source node name or role to fence")
	c.Flags().StringVar(&to, "to", "standby", "target node name or role to receive traffic")
	c.Flags().BoolVar(&fencingConfirmed, "fencing-confirmed", false, "confirm the source node is stopped or isolated before F5 switch")
	c.Flags().BoolVar(&skipSync, "skip-sync", false, "skip final catch-up sync; use only when the source is unreachable and RPO loss is accepted")
	c.Flags().DurationVar(&timeout, "timeout", 30*time.Minute, "maximum duration for catch-up sync")
	return c
}

func requireHA(cfg *config.Config) error {
	if cfg == nil || !cfg.HA.Enabled {
		return fmt.Errorf("ha.enabled is false or missing; add an enabled ha section to the config")
	}
	return nil
}

func printNodeHealth(cfg *config.Config, verbose bool) int {
	failed := 0
	for _, node := range cfg.HA.Nodes {
		client, err := newHAClient(cfg, node)
		if err != nil {
			fmt.Printf("  FAIL  %-12s role=%-7s url=%s error=%s\n", node.Name, node.Role, node.BaseURL, err)
			failed++
			continue
		}
		checks := []struct {
			name string
			fn   func() error
		}{
			{"list repositories", func() error { _, err := client.ListRepositories(); return err }},
			{"list privileges", func() error { _, err := client.ListPrivileges(); return err }},
			{"read guest role", func() error { _, err := client.GetRole(cfg.GuestAccess.RoleName); return err }},
		}
		nodeFailed := 0
		for _, ch := range checks {
			err := ch.fn()
			if err != nil && !nexus.IsNotFound(err) {
				nodeFailed++
				if verbose {
					fmt.Printf("  FAIL  %-12s %-22s %s\n", node.Name, ch.name, err)
				}
			} else if verbose {
				level := "OK"
				if nexus.IsNotFound(err) {
					level = "WARN"
				}
				fmt.Printf("  %-5s %-12s %-22s %s\n", level, node.Name, ch.name, errString(err))
			}
		}
		if nodeFailed > 0 {
			failed++
			if !verbose {
				fmt.Printf("  FAIL  %-12s role=%-7s url=%s failedChecks=%d\n", node.Name, node.Role, node.BaseURL, nodeFailed)
			}
			continue
		}
		if !verbose {
			fmt.Printf("  OK    %-12s role=%-7s url=%s\n", node.Name, node.Role, node.BaseURL)
		}
	}
	return failed
}

func newHAClient(cfg *config.Config, node config.HANodeConfig) (*nexus.Client, error) {
	pw, err := cfg.HANodePassword(node)
	if err != nil {
		return nil, err
	}
	username := node.Username
	if strings.TrimSpace(username) == "" {
		username = cfg.Nexus.Username
	}
	return nexus.New(node.BaseURL, username, pw, cfg.Nexus.TimeoutSeconds, cfg.Nexus.InsecureSkipTLSVerify), nil
}

func resolveHANode(cfg *config.Config, value, defaultRole string) (config.HANodeConfig, error) {
	if strings.TrimSpace(value) == "" {
		value = defaultRole
	}
	for _, n := range cfg.HA.Nodes {
		if n.Name == value || n.Role == value {
			return n, nil
		}
	}
	return config.HANodeConfig{}, fmt.Errorf("HA node %q not found by name or role", value)
}

func writeHAAudit(cfg *config.Config, command, action string, runErr error) {
	if cfg == nil {
		return
	}
	logger := audit.New(cfg.Audit.LogPath, cfg.Audit.Enabled, cfg.Audit.MaskSensitive)
	rec := audit.Record{
		Command:      command,
		DryRun:       false,
		Action:       action,
		Result:       resultString(runErr),
		Operator:     currentOperator(),
		Timestamp:    time.Now().Format(time.RFC3339),
		NexusBaseURL: cfg.Nexus.BaseURL,
		ErrorMessage: errString(runErr),
	}
	if err := logger.Write(rec); err != nil {
		fmt.Fprintf(os.Stderr, "warning: audit log: %v\n", err)
	}
}

func resultString(err error) string {
	if err == nil {
		return "success"
	}
	return "failed"
}

func fallback(v, d string) string {
	if strings.TrimSpace(v) == "" {
		return d
	}
	return v
}
