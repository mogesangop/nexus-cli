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
	cmd.AddCommand(newGuestProtectCmd())
	cmd.AddCommand(newGuestSyncCmd())
	cmd.AddCommand(newGuestCheckCmd())
	return cmd
}

func newGuestProtectCmd() *cobra.Command {
	return newGuestApplyCmd("protect", "Protect guest access from config (supports --dry-run)", "guest protect", false)
}

func newGuestSyncCmd() *cobra.Command {
	return newGuestApplyCmd("sync", "Deprecated alias for guest protect", "guest sync", true)
}

func newGuestApplyCmd(use, short, auditCommand string, deprecated bool) *cobra.Command {
	var (
		cfgPath    string
		dryRun     bool
		yes        bool
		reportFile string
		output     string
	)
	c := &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWithJSONErrors(cmd, output, auditCommand, func() error {
				if err := validateWriteOutput(output, dryRun); err != nil {
					return err
				}
				if err := requireWriteConfirmation(auditCommand, dryRun, yes); err != nil {
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

				syncer := guest.NewSyncer(cfg)
				plan, rep, err := syncer.PlanAndSync(client, dryRun)
				if err != nil {
					writeAudit(cfg, auditCommand, dryRun, "failed", plan, err)
					if !(dryRun && isJSONOutput(output)) {
						report.PrintSync(rep)
					}
					return err
				}
				writeAudit(cfg, auditCommand, dryRun, "success", plan, nil)
				if dryRun && isJSONOutput(output) {
					return writeDryRunResponse(cmd, auditCommand, map[string]any{
						"plan":   plan,
						"report": rep,
					}, guestProtectChanges(rep), rep.Warnings)
				}
				report.PrintSync(rep)

				if reportFile != "" {
					if err := report.WriteFileSync(cfg.Report.OutputDir, reportFile, cfg.Report.Format, rep); err != nil {
						fmt.Fprintf(os.Stderr, "warning: write report: %v\n", err)
					}
				}
				return nil
			})
		},
	}
	c.Flags().StringVar(&cfgPath, "config", "", "config file path (searched if unset: ./, ~/.nexus-cli/, /etc/nexus-cli/)")
	c.Flags().BoolVar(&dryRun, "dry-run", false, "compute and print the plan without applying changes")
	c.Flags().BoolVar(&yes, "yes", false, "confirm applying guest access changes")
	c.Flags().StringVar(&reportFile, "report", "", "write a report file under report.outputDir (e.g. guest-protect-report.txt)")
	addOutputFlag(c, &output)
	if deprecated {
		c.Deprecated = "use `guest protect`"
	}
	return c
}

func newGuestCheckCmd() *cobra.Command {
	var cfgPath, output string
	c := &cobra.Command{
		Use:   "check",
		Short: "Check whether guest role permissions match config (read-only)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWithJSONErrors(cmd, output, "guest check", func() error {
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
				checker := guest.NewChecker(cfg)
				rep, err := checker.Check(client)
				if err != nil {
					return err
				}
				if isJSONOutput(output) {
					result := "success"
					if len(rep.Fails) > 0 {
						result = "failed"
					}
					return writeReadOnlyResponse(cmd, "guest check", result, map[string]any{
						"check": rep,
					}, rep.Warns)
				}
				report.PrintCheck(rep)
				return nil
			})
		},
	}
	c.Flags().StringVar(&cfgPath, "config", "", "config file path (searched if unset: ./, ~/.nexus-cli/, /etc/nexus-cli/)")
	addOutputFlag(c, &output)
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

func guestProtectChanges(rep *report.SyncReport) []responseChange {
	if rep == nil {
		return []responseChange{}
	}
	changes := make([]responseChange, 0, len(rep.PrivilegesToCreate)+len(rep.PrivilegesToRemove)+len(rep.RemovedRiskyPrivileges))
	for _, name := range rep.PrivilegesToCreate {
		changes = append(changes, responseChange{ResourceType: "privilege", Name: name, Action: "create"})
	}
	for _, name := range rep.PrivilegesToRemove {
		changes = append(changes, responseChange{ResourceType: "privilege", Name: name, Action: "remove"})
	}
	for _, name := range rep.RemovedRiskyPrivileges {
		changes = append(changes, responseChange{ResourceType: "privilege", Name: name, Action: "remove-risky"})
	}
	return changes
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
