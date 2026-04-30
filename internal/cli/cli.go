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
			if _, err := project.DiscoverRoot(options.workingDir); err != nil {
				cliErr := errorFromProjectProblem(err)
				writeError(stderr, opts.json, cliErr)
				return cliErr.ExitCode
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

func exitCodeForProblem(code string) int {
	switch code {
	case "repo_root_not_found":
		return ExitNotFound
	case "absolute_path", "empty_path", "path_escape", "repo_root_path", "symlink_escape", "symlink_resolution_failed", "path_inspection_failed", "repo_root_required":
		return ExitSafety
	default:
		return ExitUsage
	}
}
