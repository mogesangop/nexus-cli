package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/231397220/nexus-cli/internal/audit"
	"github.com/231397220/nexus-cli/internal/config"
	"github.com/231397220/nexus-cli/internal/lifecycle"
	"github.com/231397220/nexus-cli/internal/rawrepo"
	"github.com/spf13/cobra"
)

// NewRepoCmd builds the `repo` command tree.
func NewRepoCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "repo", Short: "Nexus repository operations"}
	cmd.AddCommand(newRepoListCmd(), newRawCmd(), newLifecycleCmd())
	return cmd
}

func newRepoListCmd() *cobra.Command {
	var cfgPath string
	c := &cobra.Command{
		Use: "list", Short: "List all repositories (name, format, type)",
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

func newRawCmd() *cobra.Command {
	c := &cobra.Command{Use: "raw", Short: "Manage raw hosted repositories"}
	c.AddCommand(newRawApplyCmd(), newRawEnsureCmd())
	return c
}

func newRawApplyCmd() *cobra.Command {
	var cfgPath string
	var dryRun bool
	c := &cobra.Command{
		Use: "apply", Short: "Apply raw hosted repositories declared in config",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(cfgPath)
			if err != nil {
				return err
			}
			client, err := newClient(cfg)
			if err != nil {
				return err
			}
			results, err := rawrepo.New().Apply(client, cfg.Repositories.Raw, dryRun)
			for _, result := range results {
				fmt.Printf("%-32s %s\n", result.Name, result.Action)
				writeRepoAudit(cfg, "repo raw apply", result, nil)
			}
			if err != nil {
				writeGeneralAudit(cfg, audit.Record{
					Command: "repo raw apply", DryRun: dryRun, Action: "repository",
					Result: "failed", ErrorMessage: err.Error(),
				})
			}
			return err
		},
	}
	c.Flags().StringVar(&cfgPath, "config", "config.yaml", "config file path")
	c.Flags().BoolVar(&dryRun, "dry-run", false, "show changes without applying them")
	return c
}

func newRawEnsureCmd() *cobra.Command {
	var cfgPath, name, blobStore, writePolicy, disposition string
	var online, strict, dryRun bool
	c := &cobra.Command{
		Use: "ensure", Short: "Create or safely update one raw hosted repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(cfgPath)
			if err != nil {
				return err
			}
			desired := config.RawRepository{
				Name: name, Online: online, ContentDisposition: disposition,
				Storage: config.RawStorage{BlobStoreName: blobStore, StrictContentTypeValidation: strict, WritePolicy: writePolicy},
			}
			probe := config.Default()
			probe.Repositories.Raw = []config.RawRepository{desired}
			if err := probe.Validate(); err != nil {
				return err
			}
			desired = probe.Repositories.Raw[0]
			client, err := newClient(cfg)
			if err != nil {
				return err
			}
			result, err := rawrepo.New().Ensure(client, desired, dryRun)
			if result != nil {
				fmt.Printf("%-32s %s\n", result.Name, result.Action)
				writeRepoAudit(cfg, "repo raw ensure", *result, err)
			} else if err != nil {
				writeGeneralAudit(cfg, audit.Record{
					Command: "repo raw ensure", DryRun: dryRun, Action: "repository",
					Result: "failed", TargetRepo: name, ErrorMessage: err.Error(),
				})
			}
			return err
		},
	}
	f := c.Flags()
	f.StringVar(&cfgPath, "config", "config.yaml", "config file path")
	f.StringVar(&name, "name", "", "repository name (required)")
	f.StringVar(&blobStore, "blob-store", "", "existing Nexus blob store name (required)")
	f.BoolVar(&online, "online", true, "make the repository online")
	f.BoolVar(&strict, "strict-content-type", true, "enable strict content type validation")
	f.StringVar(&writePolicy, "write-policy", "allow_once", "allow, allow_once, or deny")
	f.StringVar(&disposition, "content-disposition", "attachment", "attachment or inline")
	f.BoolVar(&dryRun, "dry-run", false, "show changes without applying them")
	_ = c.MarkFlagRequired("name")
	_ = c.MarkFlagRequired("blob-store")
	return c
}

func newLifecycleCmd() *cobra.Command {
	c := &cobra.Command{Use: "lifecycle", Short: "Preview or run CLI-managed raw retention"}
	c.AddCommand(newLifecycleActionCmd(false), newLifecycleActionCmd(true))
	return c
}

func newLifecycleActionCmd(run bool) *cobra.Command {
	var cfgPath, repository string
	var retentionDays int
	var includes, excludes []string
	var yes bool
	use, short := "preview", "Preview expired raw components without deleting"
	if run {
		use, short = "run", "Delete expired raw components (requires --yes)"
	}
	c := &cobra.Command{
		Use: use, Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			if run && !yes {
				return fmt.Errorf("refusing deletion without --yes; run lifecycle preview first")
			}
			cfg, err := loadConfig(cfgPath)
			if err != nil {
				return err
			}
			policy, err := lifecyclePolicy(cfg, repository)
			if err != nil && !cmd.Flags().Changed("retention-days") {
				return err
			}
			if cmd.Flags().Changed("retention-days") {
				policy.RetentionDays = retentionDays
				policy.Enabled = true
			}
			if cmd.Flags().Changed("include-path") {
				policy.IncludePaths = includes
			}
			if cmd.Flags().Changed("exclude-path") {
				policy.ExcludePaths = excludes
			}
			client, err := newClient(cfg)
			if err != nil {
				return err
			}
			runner := lifecycle.New()
			var rep *lifecycle.Report
			if run {
				rep, err = runner.Run(client, repository, policy)
			} else {
				rep, err = runner.Preview(client, repository, policy)
			}
			if rep != nil {
				printLifecycleReport(rep)
				writeLifecycleAudit(cfg, use, policy, rep, err)
			}
			return err
		},
	}
	f := c.Flags()
	f.StringVar(&cfgPath, "config", "config.yaml", "config file path")
	f.StringVar(&repository, "repo", "", "raw hosted repository name (required)")
	f.IntVar(&retentionDays, "retention-days", 0, "override retention age in days")
	f.StringSliceVar(&includes, "include-path", nil, "RE2 path regex to include (repeatable)")
	f.StringSliceVar(&excludes, "exclude-path", nil, "RE2 path regex to exclude (repeatable)")
	if run {
		f.BoolVar(&yes, "yes", false, "confirm permanent component deletion")
	}
	_ = c.MarkFlagRequired("repo")
	return c
}

func lifecyclePolicy(cfg *config.Config, name string) (config.LifecycleConfig, error) {
	for _, repo := range cfg.Repositories.Raw {
		if repo.Name == name {
			return repo.Lifecycle, nil
		}
	}
	return config.LifecycleConfig{}, fmt.Errorf("repository %q has no lifecycle config; provide --retention-days", name)
}

func printLifecycleReport(r *lifecycle.Report) {
	mode := "PREVIEW"
	if !r.DryRun {
		mode = "RUN"
	}
	fmt.Printf("Raw lifecycle %s: %s\n", mode, r.Repository)
	for _, c := range r.Candidates {
		fmt.Printf("  %-6dd %s  %s\n", c.AgeDays, c.LastModified.Format(time.RFC3339), c.Path)
	}
	for _, warning := range r.Warnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", warning)
	}
	fmt.Printf("Scanned: %d, candidates: %d, deleted: %d, already gone: %d\n", r.Scanned, len(r.Candidates), r.Deleted, r.NotFound)
}

func writeRepoAudit(cfg *config.Config, command string, result rawrepo.Result, runErr error) {
	writeGeneralAudit(cfg, audit.Record{
		Command: command, DryRun: result.DryRun, Action: "repository",
		Result: auditResult(runErr), TargetRepo: result.Name,
		RepositoryAction: string(result.Action), ErrorMessage: errString(runErr),
	})
}

func writeLifecycleAudit(cfg *config.Config, command string, policy config.LifecycleConfig, result *lifecycle.Report, runErr error) {
	writeGeneralAudit(cfg, audit.Record{
		Command: "repo lifecycle " + command, DryRun: result.DryRun, Action: "lifecycle",
		Result: auditResult(runErr), TargetRepo: result.Repository, RetentionDays: policy.RetentionDays,
		IncludePaths: policy.IncludePaths, ExcludePaths: policy.ExcludePaths,
		ScannedComponents: result.Scanned, CandidateComponents: len(result.Candidates),
		DeletedComponents: result.Deleted, SkippedComponents: result.NotFound + len(result.Warnings),
		ErrorMessage: errString(runErr),
	})
}

func writeGeneralAudit(cfg *config.Config, record audit.Record) {
	record.Timestamp = time.Now().Format(time.RFC3339)
	record.Operator = currentOperator()
	record.NexusBaseURL = cfg.Nexus.BaseURL
	logger := audit.New(cfg.Audit.LogPath, cfg.Audit.Enabled, cfg.Audit.MaskSensitive)
	if err := logger.Write(record); err != nil {
		fmt.Fprintf(os.Stderr, "warning: audit log: %v\n", err)
	}
}

func auditResult(err error) string {
	if err != nil {
		return "failed"
	}
	return "success"
}
