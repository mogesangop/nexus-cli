package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/231397220/nexus-cli/internal/nexus"
	"github.com/spf13/cobra"
)

const (
	outputText = "text"
	outputJSON = "json"

	errorConfigInvalid        = "CONFIG_INVALID"
	errorPasswordEnvMissing   = "PASSWORD_ENV_MISSING"
	errorNexusAuthFailed      = "NEXUS_AUTH_FAILED"
	errorNexusAPI             = "NEXUS_API_ERROR"
	errorConfirmationRequired = "CONFIRMATION_REQUIRED"
	errorUnsupportedOutput    = "UNSUPPORTED_OUTPUT"
	errorOperationConflict    = "OPERATION_CONFLICT"
)

type commandResponse struct {
	Command      string           `json:"command"`
	DryRun       bool             `json:"dryRun"`
	Result       string           `json:"result"`
	Data         any              `json:"data,omitempty"`
	Changes      []responseChange `json:"changes"`
	Warnings     []string         `json:"warnings"`
	Errors       []responseError  `json:"errors"`
	AuditLogPath string           `json:"auditLogPath,omitempty"`
}

type responseChange struct {
	ResourceType string `json:"resourceType"`
	Name         string `json:"name"`
	Action       string `json:"action"`
	Details      any    `json:"details,omitempty"`
}

type responseError struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message"`
}

type codedError struct {
	code string
	err  error
}

func (e codedError) Error() string { return e.err.Error() }

func (e codedError) Unwrap() error { return e.err }

func addOutputFlag(cmd *cobra.Command, target *string) {
	cmd.Flags().StringVar(target, "output", outputText, "output format: text or json")
}

func normalizeOutput(format string) string {
	if strings.TrimSpace(format) == "" {
		return outputText
	}
	return strings.ToLower(strings.TrimSpace(format))
}

func validateOutput(format string) error {
	switch normalizeOutput(format) {
	case outputText, outputJSON:
		return nil
	default:
		return codedError{
			code: errorUnsupportedOutput,
			err:  fmt.Errorf("%s: --output must be text or json (got %q)", errorUnsupportedOutput, format),
		}
	}
}

func isJSONOutput(format string) bool {
	return normalizeOutput(format) == outputJSON
}

func writeReadOnlyResponse(cmd *cobra.Command, command, result string, data any, warnings []string) error {
	if warnings == nil {
		warnings = []string{}
	}
	resp := commandResponse{
		Command:  command,
		DryRun:   false,
		Result:   result,
		Data:     data,
		Changes:  []responseChange{},
		Warnings: warnings,
		Errors:   []responseError{},
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(resp)
}

func writeDryRunResponse(cmd *cobra.Command, command string, data any, changes []responseChange, warnings []string) error {
	if changes == nil {
		changes = []responseChange{}
	}
	if warnings == nil {
		warnings = []string{}
	}
	resp := commandResponse{
		Command:  command,
		DryRun:   true,
		Result:   "planned",
		Data:     data,
		Changes:  changes,
		Warnings: warnings,
		Errors:   []responseError{},
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(resp)
}

func validateWriteOutput(format string, dryRun bool) error {
	if err := validateOutput(format); err != nil {
		return err
	}
	if isJSONOutput(format) && !dryRun {
		return codedError{
			code: errorConfirmationRequired,
			err:  fmt.Errorf("%s: --output json for write operations currently requires --dry-run", errorConfirmationRequired),
		}
	}
	return nil
}

func requireWriteConfirmation(command string, dryRun, yes bool) error {
	if dryRun || yes {
		return nil
	}
	return codedError{
		code: errorConfirmationRequired,
		err:  fmt.Errorf("%s: refusing %s without --yes; rerun with --dry-run to preview the plan", errorConfirmationRequired, command),
	}
}

func runWithJSONErrors(cmd *cobra.Command, output, command string, run func() error) error {
	err := run()
	if err == nil {
		return nil
	}
	if isJSONOutput(output) {
		if writeErr := writeErrorResponse(cmd, command, err); writeErr != nil {
			return writeErr
		}
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true
	}
	return err
}

func writeErrorResponse(cmd *cobra.Command, command string, err error) error {
	resp := commandResponse{
		Command:  command,
		DryRun:   false,
		Result:   "failed",
		Changes:  []responseChange{},
		Warnings: []string{},
		Errors: []responseError{
			{Code: classifyError(err), Message: err.Error()},
		},
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(resp)
}

func classifyError(err error) string {
	var ce codedError
	if errors.As(err, &ce) && ce.code != "" {
		return ce.code
	}
	var apiErr *nexus.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.Status {
		case http.StatusUnauthorized, http.StatusForbidden:
			return errorNexusAuthFailed
		default:
			return errorNexusAPI
		}
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "environment variable") && strings.Contains(msg, "not set or empty"):
		return errorPasswordEnvMissing
	case strings.Contains(msg, "confirmation required") ||
		strings.Contains(msg, "without --yes") ||
		strings.Contains(msg, "fencing confirmation required"):
		return errorConfirmationRequired
	case strings.Contains(msg, "exists as") ||
		strings.Contains(msg, "refusing migration") ||
		strings.Contains(msg, "does not match"):
		return errorOperationConflict
	default:
		return errorConfigInvalid
	}
}

func writeIndentedJSON(cmd *cobra.Command, v any) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
