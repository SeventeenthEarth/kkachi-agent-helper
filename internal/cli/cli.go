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
	"lock":     {},
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

type runCreateOutput struct {
	RunID    string              `json:"run_id"`
	State    string              `json:"state"`
	RunPath  string              `json:"run_path"`
	EventID  string              `json:"event_id"`
	Metadata project.RunMetadata `json:"metadata"`
}

type runLifecycleOutput struct {
	RunID   string `json:"run_id"`
	State   string `json:"state"`
	EventID string `json:"event_id"`
}

type lockRecoverOutput struct {
	Recovered []project.LockMetadata `json:"recovered"`
}

type artifactInitOutput struct {
	RunID             string                   `json:"run_id"`
	RunPath           string                   `json:"run_path"`
	EventID           string                   `json:"event_id"`
	Created           []project.ArtifactStatus `json:"created"`
	Reinitialized     []project.ArtifactStatus `json:"reinitialized"`
	Preserved         []project.ArtifactStatus `json:"preserved"`
	RequiredArtifacts []string                 `json:"required_artifacts"`
	Artifacts         []project.ArtifactStatus `json:"artifacts"`
}

type artifactListOutput struct {
	RunID     string                   `json:"run_id"`
	Artifacts []project.ArtifactStatus `json:"artifacts"`
}

type artifactValidateOutput struct {
	RunID  string                            `json:"run_id"`
	Gate   string                            `json:"gate"`
	Status string                            `json:"status"`
	Checks []project.ArtifactValidationCheck `json:"checks"`
}

type gateCheckOutput struct {
	RunID           string              `json:"run_id"`
	Gate            string              `json:"gate"`
	Status          string              `json:"status"`
	Checks          []project.GateCheck `json:"checks"`
	MissingEvidence []string            `json:"missing_evidence"`
	EventID         string              `json:"event_id"`
}

type runListOutput struct {
	Runs []project.RunSummary `json:"runs"`
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
			if command == "run" {
				return runRunCommand(opts.args[1:], root, stdout, stderr, opts.json)
			}
			if command == "event" {
				return runEventCommand(opts.args[1:], root, stdout, stderr, opts.json)
			}
			if command == "artifact" {
				return runArtifactCommand(opts.args[1:], root, stdout, stderr, opts.json)
			}
			if command == "lock" {
				return runLockCommand(opts.args[1:], root, stdout, stderr, opts.json)
			}
			if command == "gate" {
				return runGateCommand(opts.args[1:], root, stdout, stderr, opts.json)
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

func runRunCommand(args []string, root project.Root, stdout io.Writer, stderr io.Writer, jsonMode bool) int {
	if len(args) == 0 {
		writeError(stderr, jsonMode, cliError{Code: "run_subcommand_required", Message: "run subcommand is required", Hint: runUsageHint(), ExitCode: ExitUsage})
		return ExitUsage
	}
	switch args[0] {
	case "create":
		options, ok := parseRunCreateOptions(args[1:], stderr, jsonMode)
		if !ok {
			return ExitUsage
		}
		result, err := project.CreateRun(root, options)
		if err != nil {
			cliErr := errorFromProjectProblem(err)
			writeError(stderr, jsonMode, cliErr)
			return cliErr.ExitCode
		}
		writeRunCreateResult(stdout, result, jsonMode)
		return ExitOK
	case "activate", "close", "abort":
		if err := requireOneRunID(args); err != nil {
			writeError(stderr, jsonMode, *err)
			return ExitUsage
		}
		result, err := executeRunLifecycle(args[0], root, args[1])
		if err != nil {
			cliErr := errorFromProjectProblem(err)
			writeError(stderr, jsonMode, cliErr)
			return cliErr.ExitCode
		}
		writeRunLifecycleResult(stdout, args[0], result, jsonMode)
		return ExitOK
	case "list":
		if len(args) != 1 {
			writeError(stderr, jsonMode, cliError{Code: "unknown_option", Message: fmt.Sprintf("unknown run list option %q", args[1]), Hint: "Use run list with optional global --json only.", ExitCode: ExitUsage, Field: "option", Expected: "no run list options", Actual: args[1]})
			return ExitUsage
		}
		runs, err := project.ListRuns(root)
		if err != nil {
			cliErr := errorFromProjectProblem(err)
			writeError(stderr, jsonMode, cliErr)
			return cliErr.ExitCode
		}
		writeRunListResult(stdout, runs, jsonMode)
		return ExitOK
	case "show":
		if err := requireOneRunID(args); err != nil {
			writeError(stderr, jsonMode, *err)
			return ExitUsage
		}
		metadata, err := project.ShowRun(root, args[1])
		if err != nil {
			cliErr := errorFromProjectProblem(err)
			writeError(stderr, jsonMode, cliErr)
			return cliErr.ExitCode
		}
		writeRunShowResult(stdout, metadata, jsonMode)
		return ExitOK
	default:
		writeError(stderr, jsonMode, cliError{Code: "not_implemented", Message: "run command is not implemented yet", Hint: runUsageHint(), ExitCode: ExitUsage})
		return ExitUsage
	}
}

func requireOneRunID(args []string) *cliError {
	command := args[0]
	if len(args) == 2 {
		return nil
	}
	if len(args) > 2 && strings.HasPrefix(args[2], "--") {
		expected := "no run lifecycle options"
		if command == "show" {
			expected = "no run show options"
		}
		return &cliError{
			Code:     "unknown_option",
			Message:  fmt.Sprintf("unknown run %s option %q", command, args[2]),
			Hint:     fmt.Sprintf("Use run %s <run_id> with optional global --json only.", command),
			ExitCode: ExitUsage,
			Field:    "option",
			Expected: expected,
			Actual:   args[2],
		}
	}
	return &cliError{
		Code:     "run_id_required",
		Message:  fmt.Sprintf("run %s requires exactly one run id", command),
		Hint:     fmt.Sprintf("Use run %s <run_id>.", command),
		ExitCode: ExitUsage,
		Field:    "run_id",
		Expected: "one run id or unique prefix",
		Actual:   fmt.Sprintf("%d arguments", len(args)-1),
	}
}

func executeRunLifecycle(command string, root project.Root, runID string) (project.RunLifecycleResult, error) {
	options := project.RunLifecycleOptions{RunID: runID}
	switch command {
	case "activate":
		return project.ActivateRun(root, options)
	case "close":
		return project.CloseRun(root, options)
	case "abort":
		return project.AbortRun(root, options)
	default:
		return project.RunLifecycleResult{}, fmt.Errorf("unsupported lifecycle command %q", command)
	}
}

func parseRunCreateOptions(args []string, stderr io.Writer, jsonMode bool) (project.CreateRunOptions, bool) {
	var options project.CreateRunOptions
	seen := map[string]bool{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "--") {
			writeError(stderr, jsonMode, cliError{Code: "unknown_option", Message: fmt.Sprintf("unknown run create argument %q", arg), Hint: runCreateUsageHint(), ExitCode: ExitUsage, Field: "option", Expected: "run create flag", Actual: arg})
			return options, false
		}
		if i+1 >= len(args) {
			writeError(stderr, jsonMode, missingOptionValueError(arg, "value", runCreateUsageHint()))
			return options, false
		}
		value := args[i+1]
		i++
		if seen[arg] {
			writeError(stderr, jsonMode, cliError{Code: "duplicate_option", Message: fmt.Sprintf("duplicate run create option %q", arg), Hint: runCreateUsageHint(), ExitCode: ExitUsage, Field: "option", Expected: "option appears once", Actual: arg})
			return options, false
		}
		seen[arg] = true
		switch arg {
		case "--title":
			options.Title = value
		case "--work-path":
			options.WorkPath = value
		case "--work-mode":
			options.WorkMode = value
		case "--urgency":
			options.Urgency = value
		case "--sot-policy":
			options.SOTPolicy = value
		case "--execution-mode":
			options.ExecutionMode = value
		case "--commander":
			options.Commander = value
		case "--task-id":
			options.TaskID = value
		case "--redteam":
			options.Redteam = value
		default:
			writeError(stderr, jsonMode, cliError{Code: "unknown_option", Message: fmt.Sprintf("unknown run create option %q", arg), Hint: runCreateUsageHint(), ExitCode: ExitUsage, Field: "option", Expected: "known run create flag", Actual: arg})
			return options, false
		}
	}
	for _, required := range []string{"--title", "--work-path", "--work-mode", "--urgency", "--sot-policy", "--execution-mode", "--commander"} {
		if !seen[required] {
			writeError(stderr, jsonMode, cliError{Code: "missing_required_option", Message: fmt.Sprintf("run create requires %s", required), Hint: runCreateUsageHint(), ExitCode: ExitUsage, Field: required, Expected: "required option", Actual: "missing"})
			return options, false
		}
	}
	return options, true
}

func runGateCommand(args []string, root project.Root, stdout io.Writer, stderr io.Writer, jsonMode bool) int {
	if len(args) == 0 {
		writeError(stderr, jsonMode, cliError{Code: "gate_subcommand_required", Message: "gate subcommand is required", Hint: gateUsageHint(), ExitCode: ExitUsage})
		return ExitUsage
	}
	switch args[0] {
	case "check":
		runID, gate, cliErr := parseGateCheckArgs(args)
		if cliErr != nil {
			writeError(stderr, jsonMode, *cliErr)
			return cliErr.ExitCode
		}
		result, err := project.CheckGate(root, project.GateCheckOptions{RunID: runID, Gate: gate})
		if err != nil {
			cliErr := errorFromProjectProblem(err)
			writeError(stderr, jsonMode, cliErr)
			return cliErr.ExitCode
		}
		writeGateCheckResult(stdout, result, jsonMode)
		if result.Status == project.GateStatusPass {
			return ExitOK
		}
		return ExitSafety
	case "final":
		writeError(stderr, jsonMode, cliError{Code: "not_implemented", Message: "gate final is not implemented yet", Hint: "Use gate check <run_id> <gate>; gate final is reserved for gates-004.", ExitCode: ExitUsage, Field: "subcommand", Expected: "check", Actual: "final"})
		return ExitUsage
	default:
		writeError(stderr, jsonMode, cliError{Code: "not_implemented", Message: "gate command is not implemented yet", Hint: gateUsageHint(), ExitCode: ExitUsage})
		return ExitUsage
	}
}

func parseGateCheckArgs(args []string) (string, string, *cliError) {
	if len(args) != 3 {
		actual := fmt.Sprintf("%d arguments", len(args)-1)
		if len(args) > 1 && strings.HasPrefix(args[len(args)-1], "--") {
			return "", "", &cliError{Code: "unknown_option", Message: fmt.Sprintf("unknown gate check option %q", args[len(args)-1]), Hint: gateUsageHint(), ExitCode: ExitUsage, Field: "option", Expected: "run id and gate name only", Actual: args[len(args)-1]}
		}
		return "", "", &cliError{Code: "gate_check_arguments_required", Message: "gate check requires a run id and gate name", Hint: gateUsageHint(), ExitCode: ExitUsage, Field: "arguments", Expected: "gate check <run_id> <gate>", Actual: actual}
	}
	gate := args[2]
	if !knownGate(gate) {
		return "", "", &cliError{Code: "gate_unknown", Message: "gate is not defined", Hint: "Use one of: " + strings.Join(project.KnownGates(), ", ") + ".", ExitCode: ExitUsage, Field: "gate", Expected: strings.Join(project.KnownGates(), ","), Actual: gate}
	}
	return args[1], gate, nil
}

func knownGate(gate string) bool {
	for _, known := range project.KnownGates() {
		if gate == known {
			return true
		}
	}
	return false
}

func runArtifactCommand(args []string, root project.Root, stdout io.Writer, stderr io.Writer, jsonMode bool) int {
	if len(args) == 0 {
		writeError(stderr, jsonMode, cliError{Code: "artifact_subcommand_required", Message: "artifact subcommand is required", Hint: artifactUsageHint(), ExitCode: ExitUsage})
		return ExitUsage
	}
	switch args[0] {
	case "init":
		if err := requireOneArtifactRunID(args); err != nil {
			writeError(stderr, jsonMode, *err)
			return ExitUsage
		}
		result, err := project.InitArtifacts(root, project.ArtifactInitOptions{RunID: args[1]})
		if err != nil {
			cliErr := errorFromProjectProblem(err)
			writeError(stderr, jsonMode, cliErr)
			return cliErr.ExitCode
		}
		writeArtifactInitResult(stdout, result, jsonMode)
		return ExitOK
	case "list":
		if err := requireOneArtifactRunID(args); err != nil {
			writeError(stderr, jsonMode, *err)
			return ExitUsage
		}
		result, err := project.ListArtifacts(root, args[1])
		if err != nil {
			cliErr := errorFromProjectProblem(err)
			writeError(stderr, jsonMode, cliErr)
			return cliErr.ExitCode
		}
		writeArtifactListResult(stdout, result.RunID, result.Artifacts, jsonMode)
		return ExitOK
	case "validate":
		runID, gate, cliErr := parseArtifactValidateArgs(args)
		if cliErr != nil {
			writeError(stderr, jsonMode, *cliErr)
			return cliErr.ExitCode
		}
		result, err := project.ValidateArtifacts(root, project.ArtifactValidateOptions{RunID: runID, Gate: gate})
		if err != nil {
			cliErr := errorFromProjectProblem(err)
			writeError(stderr, jsonMode, cliErr)
			return cliErr.ExitCode
		}
		writeArtifactValidateResult(stdout, result, jsonMode)
		if result.Status == project.ValidationStatusFail {
			return ExitSafety
		}
		return ExitOK
	default:
		writeError(stderr, jsonMode, cliError{Code: "not_implemented", Message: "artifact command is not implemented yet", Hint: artifactUsageHint(), ExitCode: ExitUsage})
		return ExitUsage
	}
}

func requireOneArtifactRunID(args []string) *cliError {
	command := args[0]
	if len(args) == 2 {
		return nil
	}
	if len(args) > 2 && strings.HasPrefix(args[2], "--") {
		return &cliError{Code: "unknown_option", Message: fmt.Sprintf("unknown artifact %s option %q", command, args[2]), Hint: fmt.Sprintf("Use artifact %s <run_id> with optional global --json only.", command), ExitCode: ExitUsage, Field: "option", Expected: "no artifact command options", Actual: args[2]}
	}
	return &cliError{Code: "run_id_required", Message: fmt.Sprintf("artifact %s requires exactly one run id", command), Hint: fmt.Sprintf("Use artifact %s <run_id>.", command), ExitCode: ExitUsage, Field: "run_id", Expected: "one run id or unique prefix", Actual: fmt.Sprintf("%d arguments", len(args)-1)}
}

func parseArtifactValidateArgs(args []string) (string, string, *cliError) {
	if len(args) < 2 || strings.HasPrefix(args[1], "--") {
		return "", "", &cliError{Code: "run_id_required", Message: "artifact validate requires exactly one run id", Hint: "Use artifact validate <run_id> [--gate intake].", ExitCode: ExitUsage, Field: "run_id", Expected: "one run id or unique prefix", Actual: fmt.Sprintf("%d arguments", len(args)-1)}
	}
	runID := args[1]
	gate := project.ArtifactGateIntake
	for i := 2; i < len(args); i++ {
		switch args[i] {
		case "--gate":
			if i+1 >= len(args) {
				return "", "", &cliError{Code: "missing_option_value", Message: "--gate requires a value", Hint: "Use artifact validate <run_id> --gate intake.", ExitCode: ExitUsage, Field: "--gate", Expected: project.ArtifactGateIntake, Actual: "missing"}
			}
			gate = args[i+1]
			if gate != project.ArtifactGateIntake {
				return "", "", &cliError{Code: "unsupported_gate", Message: "artifact validation gate is not supported", Hint: "Use artifact validate <run_id> --gate intake.", ExitCode: ExitUsage, Field: "gate", Expected: project.ArtifactGateIntake, Actual: gate}
			}
			i++
		default:
			return "", "", &cliError{Code: "unknown_option", Message: fmt.Sprintf("unknown artifact validate option %q", args[i]), Hint: "Use artifact validate <run_id> [--gate intake] with optional global --json.", ExitCode: ExitUsage, Field: "option", Expected: "--gate", Actual: args[i]}
		}
	}
	return runID, gate, nil
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

func runLockCommand(args []string, root project.Root, stdout io.Writer, stderr io.Writer, jsonMode bool) int {
	if len(args) < 2 || args[0] != "recover" {
		writeError(stderr, jsonMode, cliError{Code: "not_implemented", Message: "lock command is not implemented yet", Hint: lockRecoverUsageHint(), ExitCode: ExitUsage})
		return ExitUsage
	}
	options := project.LockRecoveryOptions{Target: args[1]}
	seenReason := false
	for i := 2; i < len(args); i++ {
		switch args[i] {
		case "--reason":
			if i+1 >= len(args) {
				writeError(stderr, jsonMode, missingOptionValueError("--reason", "reason", lockRecoverUsageHint()))
				return ExitUsage
			}
			if seenReason {
				writeError(stderr, jsonMode, cliError{Code: "duplicate_option", Message: "duplicate lock recover option \"--reason\"", Hint: lockRecoverUsageHint(), ExitCode: ExitUsage, Field: "option", Expected: "option appears once", Actual: "--reason"})
				return ExitUsage
			}
			seenReason = true
			options.Reason = args[i+1]
			i++
		case "--run":
			if i+1 >= len(args) {
				writeError(stderr, jsonMode, missingOptionValueError("--run", "run id", lockRecoverUsageHint()))
				return ExitUsage
			}
			options.RunID = args[i+1]
			i++
		default:
			writeError(stderr, jsonMode, cliError{Code: "unknown_option", Message: fmt.Sprintf("unknown lock recover option %q", args[i]), Hint: lockRecoverUsageHint(), ExitCode: ExitUsage, Field: "option", Expected: "--reason or --run", Actual: args[i]})
			return ExitUsage
		}
	}
	if !seenReason {
		writeError(stderr, jsonMode, cliError{Code: "missing_required_option", Message: "lock recover requires --reason", Hint: lockRecoverUsageHint(), ExitCode: ExitUsage, Field: "--reason", Expected: "required option", Actual: "missing"})
		return ExitUsage
	}
	result, err := project.RecoverLocks(root, options)
	if err != nil {
		cliErr := errorFromProjectProblem(err)
		writeError(stderr, jsonMode, cliErr)
		return cliErr.ExitCode
	}
	writeLockRecoverResult(stdout, result, jsonMode)
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

func lockRecoverUsageHint() string {
	return "Use lock recover <active-run|project-write|all> --reason <text> [--run <run_id>] with optional global --json."
}

func gateUsageHint() string {
	return "Use gate check <run_id> <gate> with optional global --json. Known gates: " + strings.Join(project.KnownGates(), ", ") + "."
}

func eventAppendUsageHint() string {
	return "Use event append <type> --run <run_id> --payload <json-object>."
}

func artifactUsageHint() string {
	return "Use artifact init <run_id>, artifact list <run_id>, or artifact validate <run_id> [--gate intake] with optional global --json."
}

func runUsageHint() string {
	return "Use run create|activate|close|abort|list|show."
}

func runCreateUsageHint() string {
	return "Use run create --title <title> --work-path <A_development_execution|B_discovery_shaping> --work-mode <standard|light> --urgency <normal|urgent|critical> --sot-policy <existing_sot_basis|minimal_sot_before_code|full_sot_before_code> --execution-mode <production_write|adapter_qa|readiness_hardening|research|verification|docs_only> --commander <profile> [--task-id <id>] [--redteam <profile>]."
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

func writeLockRecoverResult(w io.Writer, result project.LockRecoveryResult, jsonMode bool) {
	payload := lockRecoverOutput{Recovered: result.Recovered}
	if jsonMode {
		_ = json.NewEncoder(w).Encode(payload)
		return
	}
	fmt.Fprintf(w, "recovered locks: %d\n", len(payload.Recovered))
	for _, lock := range payload.Recovered {
		fmt.Fprintf(w, "- %s pid=%d host=%s command=%s created_at=%s\n", lock.LockName, lock.OwnerPID, lock.Hostname, lock.Command, lock.CreatedAt)
	}
}

func writeArtifactInitResult(w io.Writer, result project.ArtifactInitResult, jsonMode bool) {
	payload := artifactInitOutput{RunID: result.RunID, RunPath: result.RunPath, EventID: result.EventID, Created: result.Created, Reinitialized: result.Reinitialized, Preserved: result.Preserved, RequiredArtifacts: result.RequiredArtifacts, Artifacts: result.Artifacts}
	if jsonMode {
		_ = json.NewEncoder(w).Encode(payload)
		return
	}
	fmt.Fprintf(w, "initialized artifacts for run: %s\n", payload.RunID)
	fmt.Fprintf(w, "run_path: %s\n", payload.RunPath)
	fmt.Fprintf(w, "event_id: %s\n", payload.EventID)
	fmt.Fprintf(w, "created: %d\n", len(payload.Created))
	fmt.Fprintf(w, "reinitialized: %d\n", len(payload.Reinitialized))
	fmt.Fprintf(w, "preserved: %d\n", len(payload.Preserved))
	fmt.Fprintf(w, "required_artifacts: %d\n", len(payload.RequiredArtifacts))
}

func writeArtifactListResult(w io.Writer, runID string, artifacts []project.ArtifactStatus, jsonMode bool) {
	if jsonMode {
		_ = json.NewEncoder(w).Encode(artifactListOutput{RunID: runID, Artifacts: artifacts})
		return
	}
	fmt.Fprintf(w, "artifacts for run: %s\n", runID)
	for _, artifact := range artifacts {
		required := "optional"
		if artifact.Required {
			required = "required"
		}
		state := "missing"
		if artifact.Exists {
			state = "present"
			if artifact.Empty {
				state = "empty"
			}
		}
		fmt.Fprintf(w, "- %s %s state=%s bytes=%d\n", artifact.Path, required, state, artifact.Bytes)
	}
}

func writeGateCheckResult(w io.Writer, result project.GateCheckResult, jsonMode bool) {
	if jsonMode {
		_ = json.NewEncoder(w).Encode(gateCheckOutput{RunID: result.RunID, Gate: result.Gate, Status: result.Status, Checks: result.Checks, MissingEvidence: result.MissingEvidence, EventID: result.EventID})
		return
	}
	fmt.Fprintf(w, "gate check for run: %s\n", result.RunID)
	fmt.Fprintf(w, "gate: %s\n", result.Gate)
	fmt.Fprintf(w, "status: %s\n", result.Status)
	fmt.Fprintf(w, "event_id: %s\n", result.EventID)
	if len(result.MissingEvidence) > 0 {
		fmt.Fprintln(w, "missing_evidence:")
		for _, evidence := range result.MissingEvidence {
			fmt.Fprintf(w, "- %s\n", evidence)
		}
	}
	for _, check := range result.Checks {
		fmt.Fprintf(w, "%s %s", check.Name, check.Status)
		if check.Path != "" {
			fmt.Fprintf(w, " path=%s", check.Path)
		}
		if check.Field != "" {
			fmt.Fprintf(w, " field=%s", check.Field)
		}
		if check.Actual != "" {
			fmt.Fprintf(w, " actual=%s", check.Actual)
		}
		fmt.Fprintf(w, " message=%s\n", check.Message)
	}
}

func writeArtifactValidateResult(w io.Writer, result project.ArtifactValidateResult, jsonMode bool) {
	if jsonMode {
		payload := artifactValidateOutput{RunID: result.RunID, Gate: result.Gate, Status: result.Status, Checks: result.Checks}
		_ = json.NewEncoder(w).Encode(payload)
		return
	}
	fmt.Fprintf(w, "artifact validation for run: %s\n", result.RunID)
	fmt.Fprintf(w, "gate: %s\n", result.Gate)
	fmt.Fprintf(w, "status: %s\n", result.Status)
	for _, check := range result.Checks {
		fmt.Fprintf(w, "- %s %s", check.Name, check.Status)
		if check.Path != "" {
			fmt.Fprintf(w, " path=%s", check.Path)
		}
		if check.Field != "" {
			fmt.Fprintf(w, " field=%s", check.Field)
		}
		fmt.Fprintf(w, " message=%s\n", check.Message)
	}
}

func writeRunCreateResult(w io.Writer, result project.CreateRunResult, jsonMode bool) {
	payload := runCreateOutput{RunID: result.Metadata.RunID, State: result.Metadata.State, RunPath: result.RunPath, EventID: result.EventID, Metadata: result.Metadata}
	if jsonMode {
		_ = json.NewEncoder(w).Encode(payload)
		return
	}
	fmt.Fprintf(w, "created run: %s\n", payload.RunID)
	fmt.Fprintf(w, "state: %s\n", payload.State)
	fmt.Fprintf(w, "run_path: %s\n", payload.RunPath)
	fmt.Fprintf(w, "event_id: %s\n", payload.EventID)
}

func writeRunLifecycleResult(w io.Writer, action string, result project.RunLifecycleResult, jsonMode bool) {
	payload := runLifecycleOutput{RunID: result.Metadata.RunID, State: result.Metadata.State, EventID: result.EventID}
	if jsonMode {
		_ = json.NewEncoder(w).Encode(payload)
		return
	}
	fmt.Fprintf(w, "%s run: %s\n", pastTenseAction(action), payload.RunID)
	fmt.Fprintf(w, "state: %s\n", payload.State)
	fmt.Fprintf(w, "event_id: %s\n", payload.EventID)
}

func pastTenseAction(action string) string {
	switch action {
	case "activate":
		return "activated"
	case "close":
		return "closed"
	case "abort":
		return "aborted"
	default:
		return action
	}
}

func writeRunListResult(w io.Writer, runs []project.RunSummary, jsonMode bool) {
	if jsonMode {
		_ = json.NewEncoder(w).Encode(runListOutput{Runs: runs})
		return
	}
	fmt.Fprintf(w, "runs: %d\n", len(runs))
	for _, run := range runs {
		taskID := "null"
		if run.TaskID != nil {
			taskID = *run.TaskID
		}
		fmt.Fprintf(w, "- %s state=%s task_id=%s created_at=%s title=%s\n", run.RunID, run.State, taskID, run.CreatedAt, run.Title)
	}
}

func writeRunShowResult(w io.Writer, metadata project.RunMetadata, jsonMode bool) {
	if jsonMode {
		_ = json.NewEncoder(w).Encode(metadata)
		return
	}
	fmt.Fprintf(w, "run_id: %s\n", metadata.RunID)
	fmt.Fprintf(w, "title: %s\n", metadata.Title)
	fmt.Fprintf(w, "task_id: %s\n", printableOptional(metadata.TaskID))
	fmt.Fprintf(w, "state: %s\n", metadata.State)
	fmt.Fprintf(w, "work_path: %s\n", metadata.WorkPath)
	fmt.Fprintf(w, "work_mode: %s\n", metadata.WorkMode)
	fmt.Fprintf(w, "urgency: %s\n", metadata.Urgency)
	fmt.Fprintf(w, "sot_policy: %s\n", metadata.SOTPolicy)
	fmt.Fprintf(w, "execution_mode: %s\n", metadata.ExecutionMode)
	fmt.Fprintf(w, "commander: %s\n", metadata.Commander)
	fmt.Fprintf(w, "redteam: %s\n", printableOptional(metadata.Redteam))
	fmt.Fprintf(w, "created_at: %s\n", metadata.CreatedAt)
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
	case "artifact_baseline_encode_failed":
		return ExitInternal
	case "absolute_path", "empty_path", "path_escape", "repo_root_path", "symlink_escape", "symlink_resolution_failed", "path_inspection_failed", "repo_root_required", "helper_state_exists", "last_event_id_mismatch", "status_invalid_json", "status_last_event_id_invalid", "status_active_run_invalid", "event_log_invalid", "event_log_empty", "event_id_invalid", "event_id_exhausted", "run_metadata_invalid", "run_metadata_invalid_json", "active_run_conflict", "run_transition_invalid", "run_not_found", "run_id_ambiguous", "run_root_read_failed", "run_metadata_read_failed", "run_id_collision", "run_artifact_init_invalid_state", "artifact_inspection_failed", "artifact_path_invalid", "status_gate_summary_invalid", "lock_conflict", "lock_stale_recovery_required", "lock_metadata_invalid", "lock_not_found", "lock_identity_mismatch", "lock_release_failed":
		return ExitSafety
	default:
		return ExitUsage
	}
}
