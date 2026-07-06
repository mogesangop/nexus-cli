package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/231397220/nexus-cli/internal/audit"
	"github.com/231397220/nexus-cli/internal/config"
	"github.com/231397220/nexus-cli/internal/share"
	"github.com/spf13/cobra"
)

// NewShareCmd builds the `share` command tree.
func NewShareCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "share",
		Short: "Grant a named user browse+download access to a repository directory",
	}
	cmd.AddCommand(newShareGrantCmd())
	return cmd
}

func newShareGrantCmd() *cobra.Command {
	var (
		cfgPath        string
		repo, path     string
		userID         string
		first, last    string
		email          string
		format         string
		passwordLength int
		dryRun         bool
	)
	c := &cobra.Command{
		Use:   "grant",
		Short: "Create a path-scoped browse+read grant for a user (one-shot, idempotent)",
		Long: "Creates a content selector, a repository-content-selector privilege, a " +
			"role, and a user, wiring them together so the user can browse and download " +
			"under the given path in the repository. The password is generated and printed " +
			"once to stdout. Idempotent: existing selector/privilege/role are reused; an " +
			"existing user is an error (the password is never reset). Partial progress is " +
			"NOT rolled back — re-running is safe.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(cfgPath)
			if err != nil {
				return err
			}
			client, err := newClient(cfg)
			if err != nil {
				return err
			}

			req := share.Request{
				Repo:           repo,
				Path:           path,
				UserID:         userID,
				FirstName:      first,
				LastName:       last,
				Email:          email,
				Format:         format,
				PasswordLength: passwordLength,
				DryRun:         dryRun,
			}
			res, err := share.NewGrantor().Grant(client, req)
			if err != nil {
				writeShareAudit(cfg, "share grant", dryRun, "failed", res, err)
				return err
			}
			writeShareAudit(cfg, "share grant", dryRun, "success", res, nil)
			printShareResult(res, dryRun)
			return nil
		},
	}
	c.Flags().StringVar(&cfgPath, "config", "", "config file path (searched if unset: ./, ~/.nexus-cli/, /etc/nexus-cli/)")
	c.Flags().StringVar(&repo, "repo", "", "repository name (required)")
	c.Flags().StringVar(&path, "path", "", "directory path, e.g. /team-a/ (required)")
	c.Flags().StringVar(&userID, "user", "", "user id to create (required)")
	c.Flags().StringVar(&first, "first-name", "", "user first name")
	c.Flags().StringVar(&last, "last-name", "", "user last name")
	c.Flags().StringVar(&email, "email", "", "user email address (required)")
	c.Flags().StringVar(&format, "format", "", "repository format (auto-detected if omitted)")
	c.Flags().IntVar(&passwordLength, "password-length", 24, "generated password length")
	c.Flags().BoolVar(&dryRun, "dry-run", false, "compute and print the plan without applying changes")
	_ = c.MarkFlagRequired("repo")
	_ = c.MarkFlagRequired("path")
	_ = c.MarkFlagRequired("user")
	_ = c.MarkFlagRequired("email")
	return c
}

func printShareResult(res *share.Result, dryRun bool) {
	fmt.Printf("repo:      %s (%s)\n", res.Repo, res.Format)
	fmt.Printf("path:      %s\n", res.Path)
	fmt.Printf("selector:  %s  [%s]\n", res.Selector, createdOrReuse(res.SelectorCreated, dryRun))
	fmt.Printf("privilege: %s  [%s]\n", res.Privilege, createdOrReuse(res.PrivilegeCreated, dryRun))
	fmt.Printf("role:      %s  [%s]\n", res.Role, createdOrReuse(res.RoleCreated, dryRun))
	fmt.Printf("user:      %s  [%s]\n", res.User, createdOrReuse(res.UserCreated, dryRun))
	if dryRun {
		fmt.Println("\n[dry-run] no changes applied; no password generated.")
		return
	}
	if res.Password != "" {
		fmt.Printf("\npassword (print once): %s\n", res.Password)
	}
}

func createdOrReuse(created, dryRun bool) string {
	if dryRun {
		return "would create"
	}
	if created {
		return "created"
	}
	return "reused"
}

// writeShareAudit emits one JSONL audit record for a share grant. The password
// is never recorded (audit invariant #3). Audit write failures are non-fatal.
func writeShareAudit(cfg *config.Config, command string, dryRun bool, result string, res *share.Result, runErr error) {
	rec := audit.Record{
		Command:      command,
		DryRun:       dryRun,
		Action:       "grant",
		Result:       result,
		Operator:     currentOperator(),
		Timestamp:    time.Now().Format(time.RFC3339),
		NexusBaseURL: cfg.Nexus.BaseURL,
		ErrorMessage: errString(runErr),
	}
	if res != nil {
		rec.TargetRepo = res.Repo
		rec.TargetPath = res.Path
		rec.TargetUser = res.User
		rec.TargetRole = res.Role
		if res.SelectorCreated {
			rec.CreatedSelectors = append(rec.CreatedSelectors, res.Selector)
		}
		if res.PrivilegeCreated {
			rec.CreatedPrivileges = append(rec.CreatedPrivileges, res.Privilege)
		}
		if res.RoleCreated {
			rec.UpdatedRoles = append(rec.UpdatedRoles, res.Role)
		}
		if res.UserCreated {
			rec.CreatedUsers = append(rec.CreatedUsers, res.User)
		}
	}

	logger := audit.New(cfg.Audit.LogPath, cfg.Audit.Enabled, cfg.Audit.MaskSensitive)
	if err := logger.Write(rec); err != nil {
		fmt.Fprintf(os.Stderr, "warning: audit log: %v\n", err)
	}
}
