package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/231397220/nexus-cli/internal/audit"
	"github.com/231397220/nexus-cli/internal/config"
	"github.com/231397220/nexus-cli/internal/lifecycle"
	"github.com/231397220/nexus-cli/internal/nexus"
	"github.com/231397220/nexus-cli/internal/rawrepo"
	"github.com/231397220/nexus-cli/internal/repoctl"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// NewRepoCmd builds the `repo` command tree.
func NewRepoCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "repo", Short: "Nexus repository operations"}
	cmd.AddCommand(newRepoListCmd(), newRepoGetCmd(), newRepoApplyCmd(), newRepoEnsureCmd(), newRawCmd(), newLifecycleCmd())
	return cmd
}

func newRepoListCmd() *cobra.Command {
	var cfgPath, formatFilter, typeFilter, output string
	c := &cobra.Command{
		Use: "list", Short: "List all repositories (name, format, type)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWithJSONErrors(cmd, output, "repo list", func() error {
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
				repos, err := client.ListRepositories()
				if err != nil {
					return err
				}
				filtered := make([]nexus.Repository, 0, len(repos))
				for _, r := range repos {
					if formatFilter != "" && r.Format != formatFilter {
						continue
					}
					if typeFilter != "" && r.Type != typeFilter {
						continue
					}
					filtered = append(filtered, r)
				}
				if isJSONOutput(output) {
					return writeReadOnlyResponse(cmd, "repo list", "success", map[string]any{
						"repositories": filtered,
						"total":        len(filtered),
					}, nil)
				}
				out := cmd.OutOrStdout()
				fmt.Fprintln(out, "Repository List")
				fmt.Fprintf(out, "%-32s %-12s %-12s\n", "Name", "Format", "Type")
				for _, r := range filtered {
					fmt.Fprintf(out, "%-32s %-12s %-12s\n", r.Name, r.Format, r.Type)
				}
				fmt.Fprintf(out, "Total: %d\n", len(filtered))
				return nil
			})
		},
	}
	c.Flags().StringVar(&cfgPath, "config", "", "config file path (searched if unset: ./, ~/.nexus-cli/, /etc/nexus-cli/)")
	c.Flags().StringVar(&formatFilter, "format", "", "filter by repository format")
	c.Flags().StringVar(&typeFilter, "type", "", "filter by repository type")
	addOutputFlag(c, &output)
	return c
}

func newRepoGetCmd() *cobra.Command {
	var cfgPath, name, format, typ, output string
	c := &cobra.Command{
		Use: "get", Short: "Get one repository by name, format, and type",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWithJSONErrors(cmd, output, "repo get", func() error {
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
				repo, err := client.GetRepository(format, typ, name)
				if err != nil {
					return err
				}
				if isJSONOutput(output) {
					return writeReadOnlyResponse(cmd, "repo get", "success", map[string]any{
						"repository": repo,
					}, nil)
				}
				return writeIndentedJSON(cmd, repo)
			})
		},
	}
	f := c.Flags()
	f.StringVar(&cfgPath, "config", "", "config file path (searched if unset: ./, ~/.nexus-cli/, /etc/nexus-cli/)")
	f.StringVar(&name, "name", "", "repository name (required)")
	f.StringVar(&format, "format", "", "repository format (required)")
	f.StringVar(&typ, "type", "", "repository type (required)")
	addOutputFlag(c, &output)
	_ = c.MarkFlagRequired("name")
	_ = c.MarkFlagRequired("format")
	_ = c.MarkFlagRequired("type")
	return c
}

func newRepoApplyCmd() *cobra.Command {
	var cfgPath, output string
	var dryRun, yes bool
	c := &cobra.Command{
		Use: "apply", Short: "Apply generic repositories declared in config",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWithJSONErrors(cmd, output, "repo apply", func() error {
				if err := validateWriteOutput(output, dryRun); err != nil {
					return err
				}
				if err := requireWriteConfirmation("repo apply", dryRun, yes); err != nil {
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
				results, err := repoctl.New().Apply(client, cfg.Repositories.Managed, dryRun)
				for _, result := range results {
					writeManagedRepoAudit(cfg, "repo apply", result, nil)
				}
				if err != nil {
					writeGeneralAudit(cfg, audit.Record{
						Command: "repo apply", DryRun: dryRun, Action: "repository",
						Result: "failed", ErrorMessage: err.Error(),
					})
					return err
				}
				if dryRun && isJSONOutput(output) {
					return writeDryRunResponse(cmd, "repo apply", map[string]any{
						"repositories": results,
						"total":        len(results),
					}, managedRepoChanges(results), nil)
				}
				for _, result := range results {
					fmt.Printf("%-32s %-10s %-10s %s\n", result.Name, result.Format, result.Type, result.Action)
				}
				return nil
			})
		},
	}
	c.Flags().StringVar(&cfgPath, "config", "", "config file path (searched if unset: ./, ~/.nexus-cli/, /etc/nexus-cli/)")
	c.Flags().BoolVar(&dryRun, "dry-run", false, "show changes without applying them")
	c.Flags().BoolVar(&yes, "yes", false, "confirm applying repository changes")
	addOutputFlag(c, &output)
	return c
}

func newRepoEnsureCmd() *cobra.Command {
	var cfgPath, name, format, typ, settingsPath, output string
	var dryRun, yes bool
	c := &cobra.Command{
		Use: "ensure", Short: "Create or update one generic repository from settings YAML/JSON",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWithJSONErrors(cmd, output, "repo ensure", func() error {
				if err := validateWriteOutput(output, dryRun); err != nil {
					return err
				}
				if err := requireWriteConfirmation("repo ensure", dryRun, yes); err != nil {
					return err
				}
				settings, err := readSettingsFile(settingsPath)
				if err != nil {
					return err
				}
				desired := config.ManagedRepository{Name: name, Format: format, Type: typ, Settings: settings}
				probe := config.Default()
				probe.Repositories.Managed = []config.ManagedRepository{desired}
				if err := probe.Validate(); err != nil {
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
				result, err := repoctl.New().Ensure(client, probe.Repositories.Managed[0], dryRun)
				if result != nil {
					writeManagedRepoAudit(cfg, "repo ensure", *result, err)
				} else if err != nil {
					writeGeneralAudit(cfg, audit.Record{
						Command: "repo ensure", DryRun: dryRun, Action: "repository",
						Result: "failed", TargetRepo: name, ErrorMessage: err.Error(),
					})
				}
				if err != nil {
					return err
				}
				if result != nil && dryRun && isJSONOutput(output) {
					return writeDryRunResponse(cmd, "repo ensure", map[string]any{
						"repository": result,
					}, managedRepoChanges([]repoctl.Result{*result}), nil)
				}
				if result != nil {
					fmt.Printf("%-32s %-10s %-10s %s\n", result.Name, result.Format, result.Type, result.Action)
				}
				return nil
			})
		},
	}
	f := c.Flags()
	f.StringVar(&cfgPath, "config", "", "config file path (searched if unset: ./, ~/.nexus-cli/, /etc/nexus-cli/)")
	f.StringVar(&name, "name", "", "repository name (required)")
	f.StringVar(&format, "format", "", "repository format (required)")
	f.StringVar(&typ, "type", "", "repository type (required)")
	f.StringVar(&settingsPath, "settings", "", "YAML or JSON file containing repository settings (required)")
	f.BoolVar(&dryRun, "dry-run", false, "show changes without applying them")
	f.BoolVar(&yes, "yes", false, "confirm applying repository changes")
	addOutputFlag(c, &output)
	_ = c.MarkFlagRequired("name")
	_ = c.MarkFlagRequired("format")
	_ = c.MarkFlagRequired("type")
	_ = c.MarkFlagRequired("settings")
	return c
}

func readSettingsFile(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read settings %s: %w", path, err)
	}
	var settings map[string]any
	if err := yaml.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("parse settings %s: %w", path, err)
	}
	if settings == nil {
		settings = map[string]any{}
	}
	return settings, nil
}

func newRawCmd() *cobra.Command {
	c := &cobra.Command{Use: "raw", Short: "Manage raw hosted repositories"}
	c.AddCommand(newRawApplyCmd(), newRawEnsureCmd())
	return c
}

func newRawApplyCmd() *cobra.Command {
	var cfgPath string
	var dryRun, yes bool
	c := &cobra.Command{
		Use: "apply", Short: "Apply raw hosted repositories declared in config",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireWriteConfirmation("repo raw apply", dryRun, yes); err != nil {
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
	c.Flags().StringVar(&cfgPath, "config", "", "config file path (searched if unset: ./, ~/.nexus-cli/, /etc/nexus-cli/)")
	c.Flags().BoolVar(&dryRun, "dry-run", false, "show changes without applying them")
	c.Flags().BoolVar(&yes, "yes", false, "confirm applying raw repository changes")
	return c
}

func newRawEnsureCmd() *cobra.Command {
	var cfgPath, name, blobStore, writePolicy, disposition string
	var online, strict, dryRun, yes bool
	c := &cobra.Command{
		Use: "ensure", Short: "Create or safely update one raw hosted repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireWriteConfirmation("repo raw ensure", dryRun, yes); err != nil {
				return err
			}
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
	f.StringVar(&cfgPath, "config", "", "config file path (searched if unset: ./, ~/.nexus-cli/, /etc/nexus-cli/)")
	f.StringVar(&name, "name", "", "repository name (required)")
	f.StringVar(&blobStore, "blob-store", "", "existing Nexus blob store name (required)")
	f.BoolVar(&online, "online", true, "make the repository online")
	f.BoolVar(&strict, "strict-content-type", true, "enable strict content type validation")
	f.StringVar(&writePolicy, "write-policy", "allow_once", "allow, allow_once, or deny")
	f.StringVar(&disposition, "content-disposition", "attachment", "attachment or inline")
	f.BoolVar(&dryRun, "dry-run", false, "show changes without applying them")
	f.BoolVar(&yes, "yes", false, "confirm applying raw repository changes")
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
	var cfgPath, repository, output string
	var retentionDays int
	var includes, excludes []string
	var yes, dryRun bool
	use, short := "preview", "Preview expired raw components without deleting"
	if run {
		use, short = "run", "Delete expired raw components (requires --yes)"
	}
	c := &cobra.Command{
		Use: use, Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWithJSONErrors(cmd, output, "repo lifecycle "+use, func() error {
				if err := validateWriteOutput(output, !run || dryRun); err != nil {
					return err
				}
				if run {
					if err := requireWriteConfirmation("repo lifecycle run", dryRun, yes); err != nil {
						return err
					}
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
				if run && !dryRun {
					rep, err = runner.Run(client, repository, policy)
				} else {
					rep, err = runner.Preview(client, repository, policy)
				}
				if rep != nil {
					writeLifecycleAudit(cfg, use, policy, rep, err)
				}
				if err != nil {
					return err
				}
				if run && dryRun && isJSONOutput(output) {
					return writeDryRunResponse(cmd, "repo lifecycle run", map[string]any{
						"report": rep,
					}, lifecycleChanges(rep), rep.Warnings)
				}
				if rep != nil {
					printLifecycleReport(rep)
				}
				return nil
			})
		},
	}
	f := c.Flags()
	f.StringVar(&cfgPath, "config", "", "config file path (searched if unset: ./, ~/.nexus-cli/, /etc/nexus-cli/)")
	f.StringVar(&repository, "repo", "", "raw hosted repository name (required)")
	f.IntVar(&retentionDays, "retention-days", 0, "override retention age in days")
	f.StringSliceVar(&includes, "include-path", nil, "RE2 path regex to include (repeatable)")
	f.StringSliceVar(&excludes, "exclude-path", nil, "RE2 path regex to exclude (repeatable)")
	addOutputFlag(c, &output)
	if run {
		f.BoolVar(&yes, "yes", false, "confirm permanent component deletion")
		f.BoolVar(&dryRun, "dry-run", false, "compute and print the deletion plan without applying changes")
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

func writeManagedRepoAudit(cfg *config.Config, command string, result repoctl.Result, runErr error) {
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

func managedRepoChanges(results []repoctl.Result) []responseChange {
	changes := make([]responseChange, 0, len(results))
	for _, result := range results {
		changes = append(changes, responseChange{
			ResourceType: "repository",
			Name:         result.Name,
			Action:       string(result.Action),
			Details: map[string]any{
				"format": result.Format,
				"type":   result.Type,
			},
		})
	}
	return changes
}

func lifecycleChanges(rep *lifecycle.Report) []responseChange {
	if rep == nil {
		return []responseChange{}
	}
	changes := make([]responseChange, 0, len(rep.Candidates))
	for _, candidate := range rep.Candidates {
		changes = append(changes, responseChange{
			ResourceType: "component",
			Name:         candidate.ComponentID,
			Action:       "delete",
			Details: map[string]any{
				"repository":   rep.Repository,
				"path":         candidate.Path,
				"ageDays":      candidate.AgeDays,
				"lastModified": candidate.LastModified.Format(time.RFC3339),
			},
		})
	}
	return changes
}
