package lifecycle

import (
	"fmt"
	"regexp"
	"sort"
	"time"

	"github.com/231397220/nexus-cli/internal/config"
	"github.com/231397220/nexus-cli/internal/nexus"
)

type Client interface {
	ListRepositories() ([]nexus.Repository, error)
	ListComponents(repository, continuationToken string) (*nexus.ComponentPage, error)
	DeleteComponent(id string) error
}

type Candidate struct {
	ComponentID  string
	Path         string
	LastModified time.Time
	AgeDays      int
}

type Report struct {
	Repository string
	DryRun     bool
	Scanned    int
	Candidates []Candidate
	Deleted    int
	NotFound   int
	Warnings   []string
}

type Runner struct {
	now func() time.Time
}

func New() *Runner { return &Runner{now: time.Now} }

func NewAt(now time.Time) *Runner { return &Runner{now: func() time.Time { return now }} }

func (r *Runner) Preview(client Client, repository string, policy config.LifecycleConfig) (*Report, error) {
	repositories, err := client.ListRepositories()
	if err != nil {
		return nil, err
	}
	found := false
	for _, item := range repositories {
		if item.Name != repository {
			continue
		}
		found = true
		if item.Format != "raw" || item.Type != "hosted" {
			return nil, fmt.Errorf("repository %q is %s/%s; lifecycle requires raw/hosted", repository, item.Format, item.Type)
		}
		break
	}
	if !found {
		return nil, fmt.Errorf("repository %q not found", repository)
	}
	includes, excludes, err := compilePolicy(policy)
	if err != nil {
		return nil, err
	}
	if !policy.Enabled {
		return nil, fmt.Errorf("lifecycle is disabled for repository %q", repository)
	}
	report := &Report{Repository: repository, DryRun: true}
	cutoff := r.now().AddDate(0, 0, -policy.RetentionDays)
	token := ""
	for {
		page, err := client.ListComponents(repository, token)
		if err != nil {
			return report, err
		}
		for _, component := range page.Items {
			report.Scanned++
			candidate, warning := evaluate(component, cutoff, r.now(), includes, excludes)
			if warning != "" {
				report.Warnings = append(report.Warnings, warning)
			}
			if candidate != nil {
				report.Candidates = append(report.Candidates, *candidate)
			}
		}
		if page.ContinuationToken == nil || *page.ContinuationToken == "" {
			break
		}
		token = *page.ContinuationToken
	}
	sort.Slice(report.Candidates, func(i, j int) bool {
		return report.Candidates[i].Path < report.Candidates[j].Path
	})
	return report, nil
}

func (r *Runner) Run(client Client, repository string, policy config.LifecycleConfig) (*Report, error) {
	report, err := r.Preview(client, repository, policy)
	if err != nil {
		return report, err
	}
	report.DryRun = false
	for _, candidate := range report.Candidates {
		if err := client.DeleteComponent(candidate.ComponentID); err != nil {
			if nexus.IsNotFound(err) {
				report.NotFound++
				report.Warnings = append(report.Warnings, fmt.Sprintf("%s disappeared before deletion", candidate.Path))
				continue
			}
			return report, err
		}
		report.Deleted++
	}
	return report, nil
}

func compilePolicy(policy config.LifecycleConfig) ([]*regexp.Regexp, []*regexp.Regexp, error) {
	if policy.RetentionDays <= 0 {
		return nil, nil, fmt.Errorf("retentionDays must be greater than zero")
	}
	compile := func(values []string) ([]*regexp.Regexp, error) {
		out := make([]*regexp.Regexp, 0, len(values))
		for _, value := range values {
			re, err := regexp.Compile(value)
			if err != nil {
				return nil, fmt.Errorf("invalid path regex %q: %w", value, err)
			}
			out = append(out, re)
		}
		return out, nil
	}
	includes, err := compile(policy.IncludePaths)
	if err != nil {
		return nil, nil, err
	}
	excludes, err := compile(policy.ExcludePaths)
	return includes, excludes, err
}

func evaluate(component nexus.Component, cutoff, now time.Time, includes, excludes []*regexp.Regexp) (*Candidate, string) {
	if len(component.Assets) == 0 {
		return nil, fmt.Sprintf("component %s has no assets; skipped", component.ID)
	}
	var newest time.Time
	path := ""
	for _, asset := range component.Assets {
		parsed, err := time.Parse(time.RFC3339Nano, asset.LastModified)
		if err != nil {
			return nil, fmt.Sprintf("component %s asset %q has invalid lastModified; skipped", component.ID, asset.Path)
		}
		if path == "" {
			path = asset.Path
		}
		if parsed.After(newest) {
			newest = parsed
		}
	}
	if !matches(path, includes, true) || matches(path, excludes, false) || newest.After(cutoff) {
		return nil, ""
	}
	return &Candidate{
		ComponentID:  component.ID,
		Path:         path,
		LastModified: newest,
		AgeDays:      int(now.Sub(newest).Hours() / 24),
	}, ""
}

func matches(path string, expressions []*regexp.Regexp, emptyDefault bool) bool {
	if len(expressions) == 0 {
		return emptyDefault
	}
	for _, expression := range expressions {
		if expression.MatchString(path) {
			return true
		}
	}
	return false
}
