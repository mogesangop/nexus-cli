// Package report renders sync/check execution summaries to the console and,
// optionally, to a report file (PRD section 17).
//
// First version supports text and json output. CSV is deferred.
package report

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SyncReport summarizes a guest protection run (dry-run or applied).
type SyncReport struct {
	DryRun                 bool
	TargetRole             string
	RepositoriesTotal      int
	BrowseReadRepositories []string
	ReadOnlyRepositories   []string
	DenyRepositories       []string
	PrivilegesToCreate     []string
	PrivilegesToSkip       []string
	PrivilegesToRemove     []string
	RemovedRiskyPrivileges []string
	Warnings               []string
	Errors                 []string
}

// CheckReport summarizes a guest check.
type CheckReport struct {
	TargetRole string
	Passes     []string
	Warns      []string
	Fails      []string
}

// PrintSync writes a protection report to stdout in the human-readable format from
// PRD 8.4/8.5.
func PrintSync(r *SyncReport) {
	if r.DryRun {
		fmt.Println("Guest Access Protection Plan (dry-run)")
	} else {
		fmt.Println("Guest Access Protection Completed")
	}
	fmt.Println("Target Role:")
	fmt.Printf("  %s\n", r.TargetRole)
	printList("Browse + Read Repositories", r.BrowseReadRepositories)
	printList("Read Only Repositories", r.ReadOnlyRepositories)
	printList("Protected / Denied Repositories", r.DenyRepositories)
	printList("Privileges To Create", r.PrivilegesToCreate)
	printList("Skipped Privileges", r.PrivilegesToSkip)
	printList("Privileges To Remove", r.PrivilegesToRemove)
	printList("Removed Risky Privileges", r.RemovedRiskyPrivileges)
	printList("Warnings", r.Warnings)
	printList("Failures", r.Errors)
	fmt.Printf("Summary:\n  repositories total: %d\n  browse+read: %d\n  read-only: %d\n  denied: %d\n  created privileges: %d\n  skipped privileges: %d\n  removed risky privileges: %d\n",
		r.RepositoriesTotal,
		len(r.BrowseReadRepositories),
		len(r.ReadOnlyRepositories),
		len(r.DenyRepositories),
		len(r.PrivilegesToCreate),
		len(r.PrivilegesToSkip),
		len(r.RemovedRiskyPrivileges),
	)
	if r.DryRun {
		fmt.Println("No changes applied because dry-run is enabled.")
	}
}

// PrintCheck writes a check report to stdout in the format from PRD 8.6.
func PrintCheck(r *CheckReport) {
	fmt.Println("Guest Access Check Result")
	fmt.Println("Role:")
	fmt.Printf("  %s\n", r.TargetRole)
	printList("PASS", r.Passes)
	printList("WARN", r.Warns)
	printList("FAIL", r.Fails)
	if len(r.Fails) > 0 {
		fmt.Println("Suggestion:")
		fmt.Println("  Run: nexus-cli guest protect --config config.yaml --dry-run")
	}
}

// WriteFileSync writes a sync report to dir/name in the requested format.
// format values: "text", "json". Unknown formats default to text.
func WriteFileSync(dir, name string, format string, r *SyncReport) error {
	if dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create report dir: %w", err)
		}
	}
	path := filepath.Join(dir, name)
	var content string
	switch strings.ToLower(format) {
	case "json":
		b, err := json.MarshalIndent(r, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal report: %w", err)
		}
		content = string(b)
	default:
		content = syncReportText(r)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write report %s: %w", path, err)
	}
	return nil
}

// syncReportText returns the same human-readable text printed to stdout.
func syncReportText(r *SyncReport) string {
	var b strings.Builder
	if r.DryRun {
		b.WriteString("Guest Access Protection Plan (dry-run)\n")
	} else {
		b.WriteString("Guest Access Protection Completed\n")
	}
	b.WriteString("Target Role:\n  " + r.TargetRole + "\n")
	writeList(&b, "Browse + Read Repositories", r.BrowseReadRepositories)
	writeList(&b, "Read Only Repositories", r.ReadOnlyRepositories)
	writeList(&b, "Protected / Denied Repositories", r.DenyRepositories)
	writeList(&b, "Privileges To Create", r.PrivilegesToCreate)
	writeList(&b, "Skipped Privileges", r.PrivilegesToSkip)
	writeList(&b, "Privileges To Remove", r.PrivilegesToRemove)
	writeList(&b, "Removed Risky Privileges", r.RemovedRiskyPrivileges)
	writeList(&b, "Warnings", r.Warnings)
	writeList(&b, "Failures", r.Errors)
	fmt.Fprintf(&b, "Summary:\n  repositories total: %d\n  browse+read: %d\n  read-only: %d\n  denied: %d\n  created privileges: %d\n  skipped privileges: %d\n  removed risky privileges: %d\n",
		r.RepositoriesTotal,
		len(r.BrowseReadRepositories),
		len(r.ReadOnlyRepositories),
		len(r.DenyRepositories),
		len(r.PrivilegesToCreate),
		len(r.PrivilegesToSkip),
		len(r.RemovedRiskyPrivileges),
	)
	if r.DryRun {
		b.WriteString("No changes applied because dry-run is enabled.\n")
	}
	return b.String()
}

func printList(label string, items []string) {
	fmt.Printf("%s:\n", label)
	if len(items) == 0 {
		fmt.Println("  (none)")
		return
	}
	for _, it := range items {
		fmt.Printf("  - %s\n", it)
	}
}

func writeList(b *strings.Builder, label string, items []string) {
	fmt.Fprintf(b, "%s:\n", label)
	if len(items) == 0 {
		b.WriteString("  (none)\n")
		return
	}
	for _, it := range items {
		fmt.Fprintf(b, "  - %s\n", it)
	}
}
