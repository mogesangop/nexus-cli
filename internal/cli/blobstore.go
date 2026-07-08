package cli

import (
	"fmt"

	"github.com/231397220/nexus-cli/internal/audit"
	"github.com/231397220/nexus-cli/internal/blobstore"
	"github.com/231397220/nexus-cli/internal/config"
	"github.com/spf13/cobra"
)

func NewBlobStoreCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "blobstore", Short: "Nexus blob store operations"}
	cmd.AddCommand(newBlobStoreListCmd(), newBlobStoreGetCmd(), newBlobStoreApplyCmd(), newBlobStoreEnsureCmd())
	return cmd
}

func newBlobStoreListCmd() *cobra.Command {
	var cfgPath, output string
	c := &cobra.Command{
		Use: "list", Short: "List blob stores",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWithJSONErrors(cmd, output, "blobstore list", func() error {
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
				stores, err := client.ListBlobStores()
				if err != nil {
					return err
				}
				if isJSONOutput(output) {
					return writeReadOnlyResponse(cmd, "blobstore list", "success", map[string]any{
						"blobStores": stores,
						"total":      len(stores),
					}, nil)
				}
				out := cmd.OutOrStdout()
				fmt.Fprintln(out, "Blob Store List")
				fmt.Fprintf(out, "%-32s %-12s\n", "Name", "Type")
				for _, store := range stores {
					fmt.Fprintf(out, "%-32s %-12s\n", store.Name, store.Type)
				}
				fmt.Fprintf(out, "Total: %d\n", len(stores))
				return nil
			})
		},
	}
	c.Flags().StringVar(&cfgPath, "config", "", "config file path (searched if unset: ./, ~/.nexus-cli/, /etc/nexus-cli/)")
	addOutputFlag(c, &output)
	return c
}

func newBlobStoreGetCmd() *cobra.Command {
	var cfgPath, name, typ, output string
	c := &cobra.Command{
		Use: "get", Short: "Get one blob store",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWithJSONErrors(cmd, output, "blobstore get", func() error {
				if err := validateOutput(output); err != nil {
					return err
				}
				if typ != "file" {
					return fmt.Errorf("unsupported blob store type %q; v1 supports file", typ)
				}
				cfg, err := loadConfig(cfgPath)
				if err != nil {
					return err
				}
				client, err := newClient(cfg)
				if err != nil {
					return err
				}
				store, err := client.GetFileBlobStore(name)
				if err != nil {
					return err
				}
				if isJSONOutput(output) {
					return writeReadOnlyResponse(cmd, "blobstore get", "success", map[string]any{
						"blobStore": store,
					}, nil)
				}
				return writeIndentedJSON(cmd, store)
			})
		},
	}
	f := c.Flags()
	f.StringVar(&cfgPath, "config", "", "config file path (searched if unset: ./, ~/.nexus-cli/, /etc/nexus-cli/)")
	f.StringVar(&name, "name", "", "blob store name (required)")
	f.StringVar(&typ, "type", "file", "blob store type")
	addOutputFlag(c, &output)
	_ = c.MarkFlagRequired("name")
	return c
}

func newBlobStoreApplyCmd() *cobra.Command {
	var cfgPath, output string
	var dryRun, yes bool
	c := &cobra.Command{
		Use: "apply", Short: "Apply file blob stores declared in config",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWithJSONErrors(cmd, output, "blobstore apply", func() error {
				if err := validateWriteOutput(output, dryRun); err != nil {
					return err
				}
				if err := requireWriteConfirmation("blobstore apply", dryRun, yes); err != nil {
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
				results, err := blobstore.New().ApplyFile(client, cfg.BlobStores.File, dryRun)
				for _, result := range results {
					writeBlobStoreAudit(cfg, "blobstore apply", result, nil)
				}
				if err != nil {
					writeGeneralAudit(cfg, audit.Record{
						Command: "blobstore apply", DryRun: dryRun, Action: "blobstore",
						Result: "failed", ErrorMessage: err.Error(),
					})
					return err
				}
				if dryRun && isJSONOutput(output) {
					return writeDryRunResponse(cmd, "blobstore apply", map[string]any{
						"blobStores": results,
						"total":      len(results),
					}, blobStoreChanges(results), nil)
				}
				for _, result := range results {
					fmt.Printf("%-32s %-10s %s\n", result.Name, result.Type, result.Action)
				}
				return nil
			})
		},
	}
	c.Flags().StringVar(&cfgPath, "config", "", "config file path (searched if unset: ./, ~/.nexus-cli/, /etc/nexus-cli/)")
	c.Flags().BoolVar(&dryRun, "dry-run", false, "show changes without applying them")
	c.Flags().BoolVar(&yes, "yes", false, "confirm applying blob store changes")
	addOutputFlag(c, &output)
	return c
}

func newBlobStoreEnsureCmd() *cobra.Command {
	var cfgPath, name, path, quotaType, output string
	var quotaLimit int64
	var dryRun, yes bool
	c := &cobra.Command{
		Use: "ensure", Short: "Create or update one file blob store",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWithJSONErrors(cmd, output, "blobstore ensure", func() error {
				if err := validateWriteOutput(output, dryRun); err != nil {
					return err
				}
				if err := requireWriteConfirmation("blobstore ensure", dryRun, yes); err != nil {
					return err
				}
				desired := config.FileBlobStore{Name: name, Path: path}
				quotaTypeChanged := cmd.Flags().Changed("soft-quota-type")
				quotaLimitChanged := cmd.Flags().Changed("soft-quota-limit")
				if quotaTypeChanged != quotaLimitChanged {
					return fmt.Errorf("soft quota requires both --soft-quota-type and --soft-quota-limit")
				}
				if quotaTypeChanged {
					desired.SoftQuota = &config.SoftQuota{Type: quotaType, Limit: quotaLimit}
				}
				probe := config.Default()
				probe.BlobStores.File = []config.FileBlobStore{desired}
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
				result, err := blobstore.New().EnsureFile(client, probe.BlobStores.File[0], dryRun)
				if result != nil {
					writeBlobStoreAudit(cfg, "blobstore ensure", *result, err)
				} else if err != nil {
					writeGeneralAudit(cfg, audit.Record{
						Command: "blobstore ensure", DryRun: dryRun, Action: "blobstore",
						Result: "failed", TargetBlobStore: name, ErrorMessage: err.Error(),
					})
				}
				if err != nil {
					return err
				}
				if result != nil && dryRun && isJSONOutput(output) {
					return writeDryRunResponse(cmd, "blobstore ensure", map[string]any{
						"blobStore": result,
					}, blobStoreChanges([]blobstore.Result{*result}), nil)
				}
				if result != nil {
					fmt.Printf("%-32s %-10s %s\n", result.Name, result.Type, result.Action)
				}
				return nil
			})
		},
	}
	f := c.Flags()
	f.StringVar(&cfgPath, "config", "", "config file path (searched if unset: ./, ~/.nexus-cli/, /etc/nexus-cli/)")
	f.StringVar(&name, "name", "", "blob store name (required)")
	f.StringVar(&path, "path", "", "file blob store path (required)")
	f.StringVar(&quotaType, "soft-quota-type", "", "soft quota type")
	f.Int64Var(&quotaLimit, "soft-quota-limit", 0, "soft quota limit")
	f.BoolVar(&dryRun, "dry-run", false, "show changes without applying them")
	f.BoolVar(&yes, "yes", false, "confirm applying blob store changes")
	addOutputFlag(c, &output)
	_ = c.MarkFlagRequired("name")
	_ = c.MarkFlagRequired("path")
	return c
}

func writeBlobStoreAudit(cfg *config.Config, command string, result blobstore.Result, runErr error) {
	writeGeneralAudit(cfg, audit.Record{
		Command: command, DryRun: result.DryRun, Action: "blobstore",
		Result: auditResult(runErr), TargetBlobStore: result.Name,
		BlobStoreAction: string(result.Action), ErrorMessage: errString(runErr),
	})
}

func blobStoreChanges(results []blobstore.Result) []responseChange {
	changes := make([]responseChange, 0, len(results))
	for _, result := range results {
		changes = append(changes, responseChange{
			ResourceType: "blobstore",
			Name:         result.Name,
			Action:       string(result.Action),
			Details: map[string]any{
				"type": result.Type,
			},
		})
	}
	return changes
}
