package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/231397220/nexus-cli/internal/audit"
	"github.com/231397220/nexus-cli/internal/config"
	"github.com/231397220/nexus-cli/internal/guest"
	"github.com/231397220/nexus-cli/internal/report"
	"github.com/spf13/cobra"
)

// NewGuestCmd builds the `guest` command tree.
func NewGuestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "guest",
		Short: "Guest / anonymous access governance",
	}
	cmd.AddCommand(newGuestSyncCmd())
	cmd.AddCommand(newGuestCheckCmd())
	return cmd
}

func newGuestSyncCmd() *cobra.Command {
	var (
		cfgPath    string
		dryRun     bool
		reportFile string
	)
	c := &cobra.Command{
		Use:   "sync",
		Short: "Synchronize guest role permissions from config (supports --dry-run)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(cfgPath)
			if err != nil {
				return err
			}
			client, err := newClient(cfg)
			if err != nil {
				return err
			}

			syncer := guest.NewSyncer(cfg)
			plan, rep, err := syncer.PlanAndSync(client, dryRun)
			if err != nil {
				writeAudit(cfg, "guest sync", dryRun, "failed", plan, err)
				report.PrintSync(rep)
				return err
			}
			writeAudit(cfg, "guest sync", dryRun, "success", plan, nil)
			report.PrintSync(rep)

			if reportFile != "" {
				if err := report.WriteFileSync(cfg.Report.OutputDir, reportFile, cfg.Report.Format, rep); err != nil {
					fmt.Fprintf(os.Stderr, "warning: write report: %v\n", err)
				}
			}
			return nil
		},
	}
	c.Flags().StringVar(&cfgPath, "config", "", "config file path (searched if unset: ./, ~/.nexus-cli/, /etc/nexus-cli/)")
	c.Flags().BoolVar(&dryRun, "dry-run", false, "compute and print the plan without applying changes")
	c.Flags().StringVar(&reportFile, "report", "", "write a report file under report.outputDir (e.g. guest-sync-report.txt)")
	return c
}

func newGuestCheckCmd() *cobra.Command {
	var cfgPath string
	c := &cobra.Command{
		Use:   "check",
		Short: "Check whether guest role permissions match config (read-only)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(cfgPath)
			if err != nil {
				return err
			}
			client, err := newClient(cfg)
			if err != nil {
				return err
			}
			checker := guest.NewChecker(cfg)
			rep, err := checker.Check(client)
			if err != nil {
				return err
			}
			report.PrintCheck(rep)
			return nil
		},
	}
	c.Flags().StringVar(&cfgPath, "config", "", "config file path (searched if unset: ./, ~/.nexus-cli/, /etc/nexus-cli/)")
	return c
}

// writeAudit emits one JSONL audit record. Audit write failures are non-fatal
// and surfaced to stderr only (PRD 16).
func writeAudit(cfg *config.Config, command string, dryRun bool, result string, plan *guest.SyncPlan, runErr error) {
	rec := audit.Record{
		Command:      command,
		DryRun:       dryRun,
		Action:       "sync",
		Result:       result,
		Operator:     currentOperator(),
		Timestamp:    time.Now().Format(time.RFC3339),
		NexusBaseURL: cfg.Nexus.BaseURL,
		ErrorMessage: errString(runErr),
	}
	if plan != nil {
		rec.TargetRole = plan.TargetRole
		for _, w := range plan.PrivilegesToCreate {
			rec.CreatedPrivileges = append(rec.CreatedPrivileges, w.Name)
		}
		rec.RemovedPrivileges = append(rec.RemovedPrivileges, plan.RemovedRiskyPrivileges...)
		rec.UpdatedRoles = append(rec.UpdatedRoles, plan.TargetRole)
	}

	logger := audit.New(cfg.Audit.LogPath, cfg.Audit.Enabled, cfg.Audit.MaskSensitive)
	if err := logger.Write(rec); err != nil {
		fmt.Fprintf(os.Stderr, "warning: audit log: %v\n", err)
	}
}

func currentOperator() string {
	if u := os.Getenv("USER"); u != "" {
		return u
	}
	return "unknown"
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
