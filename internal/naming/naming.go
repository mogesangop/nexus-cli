// Package naming generates and recognizes managed Repository View privilege
// names according to PRD section 11.
//
// Privilege name grammar:
//
//	{prefix}_{format}_{sanitizedRepo}_{actionPart}
//
// where actionPart is the sorted granted actions joined by the separator
// (e.g. "browse_read" or "read"). Dashes, dots and slashes in the repository
// name are replaced with underscores when ReplaceDashWithUnderscore is set.
package naming

import (
	"sort"
	"strings"

	"github.com/231397220/nexus-cli/internal/config"
)

// Generator builds privilege names from a PrivilegeNaming config.
type Generator struct {
	cfg config.PrivilegeNaming
}

// New returns a Generator bound to the given naming config.
func New(cfg config.PrivilegeNaming) *Generator {
	return &Generator{cfg: cfg}
}

// ManagedPrefix returns the configured prefix followed by the separator,
// e.g. "priv_guest_". Privileges whose names start with this string are
// considered managed by nexus-cli.
func (g *Generator) ManagedPrefix() string {
	return g.cfg.Prefix + g.cfg.Separator
}

// SanitizeRepo converts a repository name into the token used inside a
// privilege name. Dashes, dots and slashes are replaced with the separator
// when ReplaceDashWithUnderscore is enabled; otherwise the name is used as-is.
func (g *Generator) SanitizeRepo(repo string) string {
	if !g.cfg.ReplaceDashWithUScore {
		return repo
	}
	r := strings.NewReplacer("-", g.cfg.Separator, ".", g.cfg.Separator, "/", g.cfg.Separator)
	return r.Replace(repo)
}

// PrivilegeName builds a privilege name for the given format, repository and
// actions. Actions are deduplicated and sorted before joining so the same
// logical permission always yields the same name (idempotency).
func (g *Generator) PrivilegeName(format, repo string, actions []string) string {
	actions = normalizeActions(actions)
	return strings.Join([]string{
		g.cfg.Prefix,
		g.sanitizeFormat(format),
		g.SanitizeRepo(repo),
		strings.Join(actions, g.cfg.Separator),
	}, g.cfg.Separator)
}

// IsManaged reports whether a privilege name belongs to nexus-cli (starts with
// the configured managed prefix).
func (g *Generator) IsManaged(privName string) bool {
	return strings.HasPrefix(privName, g.ManagedPrefix())
}

func (g *Generator) sanitizeFormat(format string) string {
	if !g.cfg.ReplaceDashWithUScore {
		return format
	}
	r := strings.NewReplacer("-", g.cfg.Separator, ".", g.cfg.Separator, "/", g.cfg.Separator)
	return r.Replace(format)
}

func normalizeActions(actions []string) []string {
	seen := make(map[string]struct{}, len(actions))
	out := make([]string, 0, len(actions))
	for _, a := range actions {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		if _, ok := seen[a]; ok {
			continue
		}
		seen[a] = struct{}{}
		out = append(out, a)
	}
	sort.Strings(out)
	return out
}
