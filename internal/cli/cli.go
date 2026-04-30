package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/SeventeenthEarth/kkachi-agent-helper/internal/project"
)

const (
	ExitOK       = 0
	ExitInternal = 1
	ExitUsage    = 2
	ExitSafety   = 3
	ExitNotFound = 4
)

var commandGroups = map[string]struct{}{
	"project":  {},
	"run":      {},
	"artifact": {},
	"gate":     {},
	"event":    {},
	"schema":   {},
	"install":  {},
}

// BuildInfo is the public version payload returned by the CLI.
type BuildInfo struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"build_date"`
}

type cliError struct {
	Code     string `json:"code"`
	Message  string `json:"message"`
	Hint     string `json:"hint"`
	ExitCode int    `json:"exit_code"`
	Path     string `json:"path,omitempty"`
	Field    string `json:"field,omitempty"`
	Expected string `json:"expected,omitempty"`
	Actual   string `json:"actual,omitempty"`
}

type errorEnvelope struct {
	Error cliError `json:"error"`
}

type projectInitOutput struct {
	RootPath       string   `json:"root_path"`
	ProjectID      string   `json:"project_id"`
	ProjectName    string   `json:"project_name"`
	CreatedPaths   []string `json:"created_paths"`
	SchemaPaths    []string `json:"schema_paths"`
	InitialEventID string   `json:"initial_event_id"`
}

type eventAppendOutput struct {
	EventID    string `json:"event_id"`
	PreviousID string `json:"previous_id"`
	StatusPath string `json:"status_path"`
	EventsPath string `json:"events_path"`
	OccurredAt string `json:"occurred_at"`
}

type projectStatusOutput struct {
	RootPath       string                     `json:"root_path"`
	Health         string                     `json:"health"`
	ProjectID      string                     `json:"project_id"`
	ProjectName    string                     `json:"project_name"`
	ActiveRunID    *string                    `json:"active_run_id"`
	ActiveRunState *string                    `json:"active_run_state"`
	LastEventID    string                     `json:"last_event_id"`
	EventTailID    string                     `json:"event_tail_id"`
	EventCount     int                        `json:"event_count"`
	UpdatedAt      string                     `json:"updated_at"`
	GateSummary    map[string]any             `json:"gate_summary"`
	Issues         []projectDoctorCheckOutput `json:"issues"`
}

type projectDoctorOutput struct {
	RootPath string                     `json:"root_path"`
	Health   string                     `json:"health"`
	Summary  projectDoctorSummaryOutput `json:"summary"`
	Checks   []projectDoctorCheckOutput `json:"checks"`
}

type projectDoctorSummaryOutput struct {
	Passed   int `json:"passed"`
	Warnings int `json:"warnings"`
	Failed   int `json:"failed"`
}

type projectDoctorCheckOutput struct {
	Name     string `json:"name"`
	Status   string `json:"status"`
	Path     string `json:"path"`
	Message  string `json:"message"`
	Hint     string `json:"hint"`
	Field    string `json:"field"`
	Expected string `json:"expected"`
	Actual   string `json:"actual"`
}

type globalOptions struct {
	json bool
	args []string
}

type runOptions struct {
	workingDir string
}

// Run executes the kkachi-agent-helper command and returns the process exit code.
func Run(args []string, stdout io.Writer, stderr io.Writer, info BuildInfo) int {
	jsonMode := parseGlobalOptions(args).json
	wd, err := os.Getwd()
	if err != nil {
		writeError(stderr, jsonMode, cliError{
			Code:     "working_directory_unavailable",
			Message:  "cannot read current working directory",
			Hint:     "Run the command from a readable repository directory.",
			ExitCode: ExitInternal,
			Actual:   err.Error(),
		})
		return ExitInternal
	}
	return runWithOptions(args, stdout, stderr, info, runOptions{workingDir: wd})
}

func runWithOptions(args []string, stdout io.Writer, stderr io.Writer, info BuildInfo, options runOptions) int {
	opts := parseGlobalOptions(args)
	if len(opts.args) == 0 {
		if opts.json {
			writeJSONError(stderr, cliError{
				Code:     "no_command",
				Message:  "no command provided",
				Hint:     usageHint(),
				ExitCode: ExitUsage,
			})
		} else {
			writeHumanError(stderr, cliError{
				Code:     "no_command",
				Message:  "no command provided",
				Hint:     usageHint(),
				ExitCode: ExitUsage,
			})
		}
		return ExitUsage
	}

	command := opts.args[0]
	switch command {
	case "--version":
		writeVersion(stdout, info, opts.json)
		return ExitOK
	case "version":
		writeVersion(stdout, info, opts.json)
		return ExitOK
	default:
		if _, ok := commandGroups[command]; ok {
			root, err := project.DiscoverRoot(options.workingDir)
			if err != nil {
				cliErr := errorFromProjectProblem(err)
				writeError(stderr, opts.json, cliErr)
				return cliErr.ExitCode
			}
			if command == "project" {
				return runProjectCommand(opts.args[1:], root, stdout, stderr, opts.json)
			}
			if command == "event" {
				return runEventCommand(opts.args[1:], root, stdout, stderr, opts.json)
			}
			writeError(stderr, opts.json, cliError{
				Code:     "not_implemented",
				Message:  fmt.Sprintf("command group %q is not implemented yet", command),
				Hint:     "This command group is reserved by docs/specs.md and will be implemented by a later roadmap task.",
				ExitCode: ExitUsage,
			})
			return ExitUsage
		}

		writeError(stderr, opts.json, cliError{
			Code:     "unknown_command",
			Message:  fmt.Sprintf("unknown command %q", command),
			Hint:     usageHint(),
			ExitCode: ExitUsage,
		})
		return ExitUsage
	}
}

func runProjectCommand(args []string, root project.Root, stdout io.Writer, stderr io.Writer, jsonMode bool) int {
	if len(args) > 0 && isImplementedProjectSubcommand(args[0]) && len(args) != 1 {
		writeError(stderr, jsonMode, cliError{
			Code:     "unknown_option",
			Message:  fmt.Sprintf("unknown project %s option %q", args[0], args[1]),
			Hint:     "Use project init, project status, or project doctor without command-specific options; use global --json for JSON output.",
			ExitCode: ExitUsage,
			Field:    "option",
			Expected: "no project subcommand options",
			Actual:   args[1],
		})
		return ExitUsage
	}

	if len(args) == 1 && args[0] == "init" {
		result, err := project.InitProject(root, project.InitOptions{})
		if err != nil {
			cliErr := errorFromProjectProblem(err)
			writeError(stderr, jsonMode, cliErr)
			return cliErr.ExitCode
		}
		writeProjectInitResult(stdout, result, jsonMode)
		return ExitOK
	}

	if len(args) == 1 && args[0] == "status" {
		result, err := project.InspectProjectStatus(root)
		if err != nil {
			cliErr := errorFromProjectProblem(err)
			writeError(stderr, jsonMode, cliErr)
			return cliErr.ExitCode
		}
		writeProjectStatusResult(stdout, result, jsonMode)
		return exitCodeForHealth(result.Health)
	}

	if len(args) == 1 && args[0] == "doctor" {
		result, err := project.Doctor(root)
		if err != nil {
			cliErr := errorFromProjectProblem(err)
			writeError(stderr, jsonMode, cliErr)
			return cliErr.ExitCode
		}
		writeProjectDoctorResult(stdout, result, jsonMode)
		return exitCodeForHealth(result.Health)
	}

	writeError(stderr, jsonMode, cliError{
		Code:     "not_implemented",
		Message:  "project command is not implemented yet",
		Hint:     "Use project init, project status, or project doctor; other project commands are reserved by docs/specs.md.",
		ExitCode: ExitUsage,
	})
	return ExitUsage
}

func runEventCommand(args []string, root project.Root, stdout io.Writer, stderr io.Writer, jsonMode bool) int {
	if len(args) < 2 || args[0] != "append" {
		writeError(stderr, jsonMode, cliError{
			Code:     "not_implemented",
			Message:  "event command is not implemented yet",
			Hint:     eventAppendUsageHint(),
			ExitCode: ExitUsage,
		})
		return ExitUsage
	}

	eventType := args[1]
	runID := ""
	payloadText := ""
	for i := 2; i < len(args); i++ {
		switch args[i] {
		case "--run":
			if i+1 >= len(args) {
				writeError(stderr, jsonMode, missingOptionValueError("--run", "run id", "Pass --run <run_id>."))
				return ExitUsage
			}
			runID = args[i+1]
			i++
		case "--payload":
			if i+1 >= len(args) {
				writeError(stderr, jsonMode, missingOptionValueError("--payload", "JSON object", "Pass --payload '<json-object>'."))
				return ExitUsage
			}
			payloadText = args[i+1]
			i++
		default:
			writeError(stderr, jsonMode, cliError{
				Code:     "unknown_option",
				Message:  fmt.Sprintf("unknown event append option %q", args[i]),
				Hint:     eventAppendUsageHint(),
				ExitCode: ExitUsage,
				Field:    "option",
				Expected: "--run or --payload",
				Actual:   args[i],
			})
			return ExitUsage
		}
	}
	if strings.TrimSpace(runID) == "" {
		writeError(stderr, jsonMode, cliError{
			Code:     "run_id_required",
			Message:  "event append requires --run",
			Hint:     "Pass --run <run_id> so the event remains attributable.",
			ExitCode: ExitUsage,
			Field:    "run_id",
			Expected: "non-empty run id",
			Actual:   "empty",
		})
		return ExitUsage
	}
	if strings.TrimSpace(payloadText) == "" {
		writeError(stderr, jsonMode, cliError{
			Code:     "payload_required",
			Message:  "event append requires --payload",
			Hint:     "Pass --payload '<json-object>' even when the object is empty.",
			ExitCode: ExitUsage,
			Field:    "payload",
			Expected: "JSON object",
			Actual:   "empty",
		})
		return ExitUsage
	}
	if len(payloadText) > project.MaxEventPayloadBytes {
		writeError(stderr, jsonMode, cliError{
			Code:     "payload_too_large",
			Message:  "event payload exceeds the maximum supported size",
			Hint:     "Store large evidence in artifacts and keep event payloads compact.",
			ExitCode: ExitUsage,
			Field:    "payload",
			Expected: fmt.Sprintf("at most %d bytes", project.MaxEventPayloadBytes),
			Actual:   fmt.Sprintf("%d bytes", len(payloadText)),
		})
		return ExitUsage
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(payloadText), &payload); err != nil || payload == nil {
		actual := "not an object"
		if err != nil {
			actual = err.Error()
		}
		writeError(stderr, jsonMode, cliError{
			Code:     "payload_invalid_json",
			Message:  "event payload must be a JSON object",
			Hint:     "Pass --payload with a valid JSON object such as '{\"key\":\"value\"}'.",
			ExitCode: ExitUsage,
			Field:    "payload",
			Expected: "JSON object",
			Actual:   actual,
		})
		return ExitUsage
	}

	result, err := project.AppendEvent(root, project.AppendEventOptions{Type: eventType, RunID: runID, Payload: payload})
	if err != nil {
		cliErr := errorFromProjectProblem(err)
		writeError(stderr, jsonMode, cliErr)
		return cliErr.ExitCode
	}
	writeEventAppendResult(stdout, result, jsonMode)
	return ExitOK
}

func isImplementedProjectSubcommand(command string) bool {
	switch command {
	case "init", "status", "doctor":
		return true
	default:
		return false
	}
}

func eventAppendUsageHint() string {
	return "Use event append <type> --run <run_id> --payload <json-object>."
}

func missingOptionValueError(option string, expected string, hint string) cliError {
	return cliError{
		Code:     "missing_option_value",
		Message:  option + " requires a value",
		Hint:     hint,
		ExitCode: ExitUsage,
		Field:    option,
		Expected: expected,
		Actual:   "missing",
	}
}

func parseGlobalOptions(args []string) globalOptions {
	opts := globalOptions{args: make([]string, 0, len(args))}
	for _, arg := range args {
		if arg == "--json" {
			opts.json = true
			continue
		}
		opts.args = append(opts.args, arg)
	}
	return opts
}

func writeVersion(w io.Writer, info BuildInfo, jsonMode bool) {
	if jsonMode {
		_ = json.NewEncoder(w).Encode(info)
		return
	}

	parts := []string{info.Name, info.Version}
	if hasValue(info.Commit) {
		parts = append(parts, "commit "+info.Commit)
	}
	if hasValue(info.BuildDate) {
		parts = append(parts, "built "+info.BuildDate)
	}
	fmt.Fprintln(w, strings.Join(parts, " "))
}

func writeProjectInitResult(w io.Writer, result project.InitResult, jsonMode bool) {
	payload := projectInitOutput{
		RootPath:       result.RootPath,
		ProjectID:      result.ProjectID,
		ProjectName:    result.ProjectName,
		CreatedPaths:   result.CreatedPaths,
		SchemaPaths:    result.SchemaPaths,
		InitialEventID: result.InitialEventID,
	}
	if jsonMode {
		_ = json.NewEncoder(w).Encode(payload)
		return
	}

	fmt.Fprintf(w, "initialized kkachi project: %s\n", payload.RootPath)
	fmt.Fprintf(w, "project_id: %s\n", payload.ProjectID)
	fmt.Fprintf(w, "created:\n")
	for _, path := range payload.CreatedPaths {
		fmt.Fprintf(w, "- %s\n", path)
	}
	for _, path := range payload.SchemaPaths {
		fmt.Fprintf(w, "- %s\n", path)
	}
	fmt.Fprintf(w, "initial_event_id: %s\n", payload.InitialEventID)
}

func writeEventAppendResult(w io.Writer, result project.AppendEventResult, jsonMode bool) {
	payload := eventAppendOutput{
		EventID:    result.EventID,
		PreviousID: result.PreviousID,
		StatusPath: result.StatusPath,
		EventsPath: result.EventsPath,
		OccurredAt: result.OccurredAt,
	}
	if jsonMode {
		_ = json.NewEncoder(w).Encode(payload)
		return
	}

	fmt.Fprintf(w, "appended event: %s\n", payload.EventID)
	fmt.Fprintf(w, "previous_event_id: %s\n", payload.PreviousID)
	fmt.Fprintf(w, "events_file: %s\n", payload.EventsPath)
	fmt.Fprintf(w, "status_file: %s\n", payload.StatusPath)
}

func writeProjectStatusResult(w io.Writer, result project.ProjectStatus, jsonMode bool) {
	payload := projectStatusPayload(result)
	if jsonMode {
		_ = json.NewEncoder(w).Encode(payload)
		return
	}

	fmt.Fprintf(w, "project status: %s\n", payload.Health)
	fmt.Fprintf(w, "root_path: %s\n", payload.RootPath)
	fmt.Fprintf(w, "project_id: %s\n", payload.ProjectID)
	fmt.Fprintf(w, "project_name: %s\n", payload.ProjectName)
	fmt.Fprintf(w, "active_run_id: %s\n", printableOptional(payload.ActiveRunID))
	fmt.Fprintf(w, "active_run_state: %s\n", printableOptional(payload.ActiveRunState))
	fmt.Fprintf(w, "last_event_id: %s\n", payload.LastEventID)
	fmt.Fprintf(w, "event_tail_id: %s\n", payload.EventTailID)
	fmt.Fprintf(w, "event_count: %d\n", payload.EventCount)
	fmt.Fprintf(w, "updated_at: %s\n", payload.UpdatedAt)
	fmt.Fprintf(w, "issues: %d\n", len(payload.Issues))
	for _, issue := range payload.Issues {
		fmt.Fprintf(w, "- [%s] %s %s: %s\n", issue.Status, issue.Name, issue.Path, issue.Message)
		if issue.Hint != "" {
			fmt.Fprintf(w, "  hint: %s\n", issue.Hint)
		}
	}
}

func writeProjectDoctorResult(w io.Writer, result project.DoctorReport, jsonMode bool) {
	payload := projectDoctorPayload(result)
	if jsonMode {
		_ = json.NewEncoder(w).Encode(payload)
		return
	}

	fmt.Fprintf(w, "project doctor: %s\n", payload.Health)
	fmt.Fprintf(w, "root_path: %s\n", payload.RootPath)
	fmt.Fprintf(w, "summary: %d passed, %d warnings, %d failed\n", payload.Summary.Passed, payload.Summary.Warnings, payload.Summary.Failed)
	for _, check := range payload.Checks {
		fmt.Fprintf(w, "- [%s] %s", check.Status, check.Name)
		if check.Path != "" {
			fmt.Fprintf(w, " %s", check.Path)
		}
		fmt.Fprintf(w, ": %s\n", check.Message)
		if check.Field != "" || check.Expected != "" || check.Actual != "" {
			fmt.Fprintf(w, "  field: %s expected: %s actual: %s\n", check.Field, check.Expected, check.Actual)
		}
		if check.Hint != "" {
			fmt.Fprintf(w, "  hint: %s\n", check.Hint)
		}
	}
}

func projectStatusPayload(result project.ProjectStatus) projectStatusOutput {
	return projectStatusOutput{
		RootPath:       result.RootPath,
		Health:         result.Health,
		ProjectID:      result.ProjectID,
		ProjectName:    result.ProjectName,
		ActiveRunID:    result.ActiveRunID,
		ActiveRunState: result.ActiveRunState,
		LastEventID:    result.LastEventID,
		EventTailID:    result.EventTailID,
		EventCount:     result.EventCount,
		UpdatedAt:      result.UpdatedAt,
		GateSummary:    result.GateSummary,
		Issues:         projectCheckPayloads(result.Issues),
	}
}

func projectDoctorPayload(result project.DoctorReport) projectDoctorOutput {
	return projectDoctorOutput{
		RootPath: result.RootPath,
		Health:   result.Health,
		Summary: projectDoctorSummaryOutput{
			Passed:   result.Summary.Passed,
			Warnings: result.Summary.Warnings,
			Failed:   result.Summary.Failed,
		},
		Checks: projectCheckPayloads(result.Checks),
	}
}

func projectCheckPayloads(checks []project.DiagnosticCheck) []projectDoctorCheckOutput {
	payloads := make([]projectDoctorCheckOutput, 0, len(checks))
	for _, check := range checks {
		payloads = append(payloads, projectDoctorCheckOutput{
			Name:     check.Name,
			Status:   check.Status,
			Path:     check.Path,
			Message:  check.Message,
			Hint:     check.Hint,
			Field:    check.Field,
			Expected: check.Expected,
			Actual:   check.Actual,
		})
	}
	return payloads
}

func printableOptional(value *string) string {
	if value == nil {
		return "null"
	}
	return *value
}

func hasValue(value string) bool {
	return value != "" && value != "unknown"
}

func writeError(w io.Writer, jsonMode bool, err cliError) {
	if jsonMode {
		writeJSONError(w, err)
		return
	}
	writeHumanError(w, err)
}

func writeJSONError(w io.Writer, err cliError) {
	_ = json.NewEncoder(w).Encode(errorEnvelope{Error: err})
}

func writeHumanError(w io.Writer, err cliError) {
	fmt.Fprintf(w, "error: %s: %s\n", err.Code, err.Message)
	if err.Path != "" {
		fmt.Fprintf(w, "path: %s\n", err.Path)
	}
	if err.Field != "" {
		fmt.Fprintf(w, "field: %s\n", err.Field)
	}
	if err.Expected != "" {
		fmt.Fprintf(w, "expected: %s\n", err.Expected)
	}
	if err.Actual != "" {
		fmt.Fprintf(w, "actual: %s\n", err.Actual)
	}
	if err.ExitCode != 0 {
		fmt.Fprintf(w, "exit_code: %d\n", err.ExitCode)
	}
	fmt.Fprintf(w, "hint: %s\n", err.Hint)
}

func usageHint() string {
	return "Usage: kkachi-agent-helper [--json] <version|project|run|artifact|gate|event|schema|install>"
}

func errorFromProjectProblem(err error) cliError {
	var problem *project.Problem
	if errors.As(err, &problem) {
		return cliError{
			Code:     problem.Code,
			Message:  problem.Message,
			Hint:     problem.Hint,
			ExitCode: exitCodeForProblem(problem.Code),
			Path:     problem.Path,
			Field:    problem.Field,
			Expected: problem.Expected,
			Actual:   problem.Actual,
		}
	}

	return cliError{
		Code:     "internal_error",
		Message:  "unexpected helper error",
		Hint:     "Rerun with the same arguments and preserve stderr for diagnosis.",
		ExitCode: ExitInternal,
		Actual:   err.Error(),
	}
}

func exitCodeForHealth(health string) int {
	if health == project.HealthFail {
		return ExitSafety
	}
	return ExitOK
}

func exitCodeForProblem(code string) int {
	switch code {
	case "repo_root_not_found":
		return ExitNotFound
	case "absolute_path", "empty_path", "path_escape", "repo_root_path", "symlink_escape", "symlink_resolution_failed", "path_inspection_failed", "repo_root_required", "helper_state_exists", "last_event_id_mismatch", "status_invalid_json", "status_last_event_id_invalid", "event_log_invalid", "event_log_empty", "event_id_invalid":
		return ExitSafety
	default:
		return ExitUsage
	}
}
