package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

const (
	exitOK    = 0
	exitError = 2
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
	Code    string `json:"code"`
	Message string `json:"message"`
	Hint    string `json:"hint"`
}

type errorEnvelope struct {
	Error cliError `json:"error"`
}

type globalOptions struct {
	json bool
	args []string
}

// Run executes the kkachi-agent-helper command and returns the process exit code.
func Run(args []string, stdout io.Writer, stderr io.Writer, info BuildInfo) int {
	opts := parseGlobalOptions(args)
	if len(opts.args) == 0 {
		if opts.json {
			writeJSONError(stderr, cliError{
				Code:    "no_command",
				Message: "no command provided",
				Hint:    usageHint(),
			})
		} else {
			writeHumanError(stderr, cliError{
				Code:    "no_command",
				Message: "no command provided",
				Hint:    usageHint(),
			})
		}
		return exitError
	}

	command := opts.args[0]
	switch command {
	case "--version":
		writeVersion(stdout, info, opts.json)
		return exitOK
	case "version":
		writeVersion(stdout, info, opts.json)
		return exitOK
	default:
		if _, ok := commandGroups[command]; ok {
			writeError(stderr, opts.json, cliError{
				Code:    "not_implemented",
				Message: fmt.Sprintf("command group %q is not implemented yet", command),
				Hint:    "This command group is reserved by docs/specs.md and will be implemented by a later roadmap task.",
			})
			return exitError
		}

		writeError(stderr, opts.json, cliError{
			Code:    "unknown_command",
			Message: fmt.Sprintf("unknown command %q", command),
			Hint:    usageHint(),
		})
		return exitError
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
	fmt.Fprintf(w, "error: %s: %s\nhint: %s\n", err.Code, err.Message, err.Hint)
}

func usageHint() string {
	return "Usage: kkachi-agent-helper [--json] <version|project|run|artifact|gate|event|schema|install>"
}
