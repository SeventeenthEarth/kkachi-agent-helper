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

// commandGroups contains top-level command groups that require repository root
// discovery before dispatch. Project-independent commands such as version and
// capabilities are handled before this map is consulted.
var commandGroups = map[string]struct{}{
	"project":     {},
	"run":         {},
	"artifact":    {},
	"gate":        {},
	"event":       {},
	"schema":      {},
	"lock":        {},
	"diagnostics": {},
	"phase-plan":  {},
	"approval":    {},
	"graph":       {},
}

const (
	// capabilitiesSchemaVersion versions the JSON contract emitted by
	// capabilities --json. Keep bump rules in sync with docs/compatibility.md.
	capabilitiesSchemaVersion = "0.1"
	capabilityStatusSupported = "supported"
	capabilityStatusPlanned   = "planned"
	capabilityStatusOmitted   = "omitted"
)

// BuildInfo is the public version payload returned by the CLI.
type BuildInfo struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"build_date"`
}

type capabilitiesOutput struct {
	Helper                    BuildInfo                 `json:"helper"`
	CapabilitiesSchemaVersion string                    `json:"capabilities_schema_version"`
	ProjectSchemaVersion      string                    `json:"project_schema_version"`
	CommandGroups             []capabilityCommandGroup  `json:"command_groups"`
	CompatibilityFlags        compatibilityFlagsOutput  `json:"compatibility_flags"`
	DeprecatedSurfaces        []capabilitySurfaceOutput `json:"deprecated_surfaces"`
	OmittedSurfaces           []capabilitySurfaceOutput `json:"omitted_surfaces"`
}

type capabilityCommandGroup struct {
	Name        string   `json:"name"`
	Status      string   `json:"status"`
	Subcommands []string `json:"subcommands"`
}

type compatibilityFlagsOutput struct {
	ProjectInit                       bool `json:"project_init"`
	ProjectStatus                     bool `json:"project_status"`
	ProjectDoctor                     bool `json:"project_doctor"`
	RunLifecycle                      bool `json:"run_lifecycle"`
	ArtifactInit                      bool `json:"artifact_init"`
	ArtifactList                      bool `json:"artifact_list"`
	ArtifactValidate                  bool `json:"artifact_validate"`
	ArtifactMutation                  bool `json:"artifact_mutation"`
	Gates                             bool `json:"gates"`
	BackendEvidenceRequirements       bool `json:"backend_evidence_requirements"`
	DiagnosticsExport                 bool `json:"diagnostics_export"`
	PhasePlan                         bool `json:"phase_plan"`
	ApprovalRecords                   bool `json:"approval_records"`
	WorkflowGraphReadonly             bool `json:"workflow_graph_readonly"`
	WorkflowGraphInit                 bool `json:"workflow_graph_init"`
	WorkflowGraphApply                bool `json:"workflow_graph_apply"`
	WorkflowGraphExport               bool `json:"workflow_graph_export"`
	WorkflowGraphDiagnostics          bool `json:"workflow_graph_diagnostics"`
	WorkflowGraphNoDirectYAMLFallback bool `json:"workflow_graph_no_direct_yaml_fallback"`
	InstallCommand                    bool `json:"install_command"`
}

type capabilitySurfaceOutput struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Reason string `json:"reason"`
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
	RootPath            string   `json:"root_path"`
	ProjectID           string   `json:"project_id"`
	ProjectName         string   `json:"project_name"`
	CreatedPaths        []string `json:"created_paths"`
	SchemaPaths         []string `json:"schema_paths"`
	InitialEventID      string   `json:"initial_event_id,omitempty"`
	ReconfiguredEventID string   `json:"reconfigured_event_id,omitempty"`
	Forced              bool     `json:"forced"`
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

type artifactMutationOutput struct {
	RunID        string `json:"run_id"`
	Path         string `json:"path"`
	ArtifactKind string `json:"artifact_kind"`
	Operation    string `json:"operation"`
	Bytes        int64  `json:"bytes"`
	SourcePath   string `json:"source_path,omitempty"`
	Status       string `json:"status,omitempty"`
	Reason       string `json:"reason,omitempty"`
	EventID      string `json:"event_id"`
}

type gateCheckOutput struct {
	RunID           string              `json:"run_id"`
	Gate            string              `json:"gate"`
	Status          string              `json:"status"`
	Checks          []project.GateCheck `json:"checks"`
	MissingEvidence []string            `json:"missing_evidence"`
	EventID         string              `json:"event_id"`
	ReportPath      string              `json:"report_path,omitempty"`
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

type schemaValidateOutput struct {
	Schema   string                `json:"schema"`
	FilePath string                `json:"file_path"`
	Status   string                `json:"status"`
	Checks   []project.SchemaCheck `json:"checks"`
}

type schemaExportOutput struct {
	DryRun     bool     `json:"dry_run"`
	Schemas    []string `json:"schemas"`
	Written    []string `json:"written"`
	Unchanged  []string `json:"unchanged"`
	WouldWrite []string `json:"would_write"`
	EventID    string   `json:"event_id,omitempty"`
}

type schemaMigrationOutput struct {
	DryRun       bool     `json:"dry_run"`
	FromVersion  string   `json:"from_version"`
	ToVersion    string   `json:"to_version"`
	Status       string   `json:"status"`
	Migration    string   `json:"migration"`
	WouldBackup  []string `json:"would_backup"`
	BackedUp     []string `json:"backed_up"`
	BackupPath   string   `json:"backup_path,omitempty"`
	WouldMigrate []string `json:"would_migrate"`
	Migrated     []string `json:"migrated"`
	Unchanged    []string `json:"unchanged"`
	EventID      string   `json:"event_id,omitempty"`
}

type phasePlanInitOutput struct {
	PhasePlan project.PhasePlan `json:"phase_plan"`
	EventID   string            `json:"event_id"`
}

type phasePlanSetOutput struct {
	PhasePlan project.PhasePlan `json:"phase_plan"`
	Phase     project.PhaseRow  `json:"phase"`
	EventID   string            `json:"event_id"`
}

type approvalMutationOutput struct {
	Record  project.ApprovalRecord `json:"record"`
	EventID string                 `json:"event_id"`
}

type helpOutput struct {
	Command      string     `json:"command"`
	Status       string     `json:"status"`
	Usage        string     `json:"usage"`
	Summary      string     `json:"summary"`
	Arguments    []helpItem `json:"arguments,omitempty"`
	Options      []helpItem `json:"options,omitempty"`
	Subcommands  []helpItem `json:"subcommands,omitempty"`
	JSONBehavior string     `json:"json_behavior"`
	Notes        []string   `json:"notes,omitempty"`
}

type helpItem struct {
	Name        string `json:"name"`
	Required    bool   `json:"required,omitempty"`
	Description string `json:"description"`
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
	if isHelpRequest(opts.args) {
		helpTopic := helpPath(opts.args)
		helpTopicText := strings.Join(helpTopic, " ")
		page, ok := lookupHelpPage(helpTopic)
		if !ok {
			writeError(stderr, opts.json, cliError{
				Code:     "unknown_help_topic",
				Message:  fmt.Sprintf("unknown help topic %q", helpTopicText),
				Hint:     usageHint(),
				ExitCode: ExitUsage,
				Field:    "topic",
				Expected: "known command or command group",
				Actual:   helpTopicText,
			})
			return ExitUsage
		}
		writeHelp(stdout, page, opts.json)
		return ExitOK
	}
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
	case "capabilities":
		return runCapabilitiesCommand(opts.args[1:], stdout, stderr, info, opts.json)
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
			if command == "schema" {
				return runSchemaCommand(opts.args[1:], root, stdout, stderr, opts.json)
			}
			if command == "diagnostics" {
				return runDiagnosticsCommand(opts.args[1:], root, stdout, stderr, opts.json)
			}
			if command == "phase-plan" {
				return runPhasePlanCommand(opts.args[1:], root, stdout, stderr, opts.json)
			}
			if command == "approval" {
				return runApprovalCommand(opts.args[1:], root, stdout, stderr, opts.json)
			}
			if command == "graph" {
				return runGraphCommand(opts.args[1:], root, stdout, stderr, opts.json)
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

func runCapabilitiesCommand(args []string, stdout io.Writer, stderr io.Writer, info BuildInfo, jsonMode bool) int {
	if len(args) != 0 {
		writeError(stderr, jsonMode, cliError{
			Code:     "unknown_option",
			Message:  fmt.Sprintf("unknown capabilities option %q", args[0]),
			Hint:     capabilitiesUsageHint(),
			ExitCode: ExitUsage,
			Field:    "option",
			Expected: "optional global --json only",
			Actual:   args[0],
		})
		return ExitUsage
	}
	writeCapabilities(stdout, info, jsonMode)
	return ExitOK
}

func isHelpRequest(args []string) bool {
	if len(args) == 0 {
		return false
	}
	if args[0] == "help" || args[0] == "--help" {
		return true
	}
	for _, arg := range args {
		if arg == "--help" {
			return true
		}
	}
	return false
}

func helpPath(args []string) []string {
	if len(args) == 0 || args[0] == "--help" {
		return nil
	}
	if args[0] == "help" {
		return stripHelpFlags(args[1:])
	}
	return stripHelpFlags(args)
}

func stripHelpFlags(args []string) []string {
	out := make([]string, 0, len(args))
	for _, arg := range args {
		if arg != "--help" {
			out = append(out, arg)
		}
	}
	return out
}

func runDiagnosticsCommand(args []string, root project.Root, stdout io.Writer, stderr io.Writer, jsonMode bool) int {
	options, cliErr := parseDiagnosticsArgs(args)
	if cliErr != nil {
		writeError(stderr, jsonMode, *cliErr)
		return cliErr.ExitCode
	}
	result, err := project.ExportDiagnostics(root, options)
	if err != nil {
		cliErr := errorFromProjectProblem(err)
		writeError(stderr, jsonMode, cliErr)
		return cliErr.ExitCode
	}
	writeDiagnosticsExportResult(stdout, result, jsonMode)
	return ExitOK
}

func parseDiagnosticsArgs(args []string) (project.DiagnosticsExportOptions, *cliError) {
	options := project.DiagnosticsExportOptions{}
	if len(args) == 0 || args[0] != "export" {
		return options, &cliError{Code: "diagnostics_subcommand_required", Message: "diagnostics export subcommand is required", Hint: diagnosticsUsageHint(), ExitCode: ExitUsage}
	}
	seenRun := false
	seenOutput := false
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--run":
			if i+1 >= len(args) {
				return options, &cliError{Code: "missing_option_value", Message: "--run requires a value", Hint: diagnosticsUsageHint(), ExitCode: ExitUsage, Field: "--run", Expected: "run id or unique prefix", Actual: "missing"}
			}
			if seenRun {
				return options, &cliError{Code: "duplicate_option", Message: "duplicate diagnostics export option \"--run\"", Hint: diagnosticsUsageHint(), ExitCode: ExitUsage, Field: "option", Expected: "option appears once", Actual: "--run"}
			}
			seenRun = true
			options.RunID = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return options, &cliError{Code: "missing_option_value", Message: "--output requires a value", Hint: diagnosticsUsageHint(), ExitCode: ExitUsage, Field: "--output", Expected: "repository-relative output path", Actual: "missing"}
			}
			if seenOutput {
				return options, &cliError{Code: "duplicate_option", Message: "duplicate diagnostics export option \"--output\"", Hint: diagnosticsUsageHint(), ExitCode: ExitUsage, Field: "option", Expected: "option appears once", Actual: "--output"}
			}
			seenOutput = true
			options.Output = args[i+1]
			i++
		default:
			return options, &cliError{Code: "unknown_option", Message: fmt.Sprintf("unknown diagnostics export option %q", args[i]), Hint: diagnosticsUsageHint(), ExitCode: ExitUsage, Field: "option", Expected: "--run or --output", Actual: args[i]}
		}
	}
	return options, nil
}

func runPhasePlanCommand(args []string, root project.Root, stdout io.Writer, stderr io.Writer, jsonMode bool) int {
	if len(args) == 0 {
		writeError(stderr, jsonMode, cliError{Code: "phase_plan_subcommand_required", Message: "phase-plan subcommand is required", Hint: phasePlanUsageHint(), ExitCode: ExitUsage})
		return ExitUsage
	}
	switch args[0] {
	case "init":
		if err := requireOnePhasePlanRunID(args); err != nil {
			writeError(stderr, jsonMode, *err)
			return ExitUsage
		}
		result, err := project.InitPhasePlan(root, project.PhasePlanInitOptions{RunID: args[1]})
		if err != nil {
			cliErr := errorFromProjectProblem(err)
			writeError(stderr, jsonMode, cliErr)
			return cliErr.ExitCode
		}
		writePhasePlanInitResult(stdout, result, jsonMode)
		return ExitOK
	case "show":
		if err := requireOnePhasePlanRunID(args); err != nil {
			writeError(stderr, jsonMode, *err)
			return ExitUsage
		}
		result, err := project.ShowPhasePlan(root, args[1])
		if err != nil {
			cliErr := errorFromProjectProblem(err)
			writeError(stderr, jsonMode, cliErr)
			return cliErr.ExitCode
		}
		writePhasePlanShowResult(stdout, result, jsonMode)
		return ExitOK
	case "set":
		options, cliErr := parsePhasePlanSetArgs(args)
		if cliErr != nil {
			writeError(stderr, jsonMode, *cliErr)
			return cliErr.ExitCode
		}
		result, err := project.SetPhasePlanPhase(root, options)
		if err != nil {
			cliErr := errorFromProjectProblem(err)
			writeError(stderr, jsonMode, cliErr)
			return cliErr.ExitCode
		}
		writePhasePlanSetResult(stdout, result, jsonMode)
		return ExitOK
	case "validate":
		options, cliErr := parsePhasePlanValidateArgs(args)
		if cliErr != nil {
			writeError(stderr, jsonMode, *cliErr)
			return cliErr.ExitCode
		}
		result, err := project.ValidatePhasePlan(root, options)
		if err != nil {
			cliErr := errorFromProjectProblem(err)
			writeError(stderr, jsonMode, cliErr)
			return cliErr.ExitCode
		}
		writePhasePlanValidationResult(stdout, result, jsonMode)
		if result.Status == project.PhasePlanStatusPass {
			return ExitOK
		}
		return ExitSafety
	default:
		writeError(stderr, jsonMode, cliError{Code: "phase_plan_subcommand_unknown", Message: "phase-plan subcommand is not supported", Hint: phasePlanUsageHint(), ExitCode: ExitUsage, Field: "subcommand", Expected: "init, show, set, or validate", Actual: args[0]})
		return ExitUsage
	}
}

func runGraphCommand(args []string, root project.Root, stdout io.Writer, stderr io.Writer, jsonMode bool) int {
	if len(args) == 0 {
		writeError(stderr, jsonMode, cliError{Code: "graph_subcommand_required", Message: "graph subcommand is required", Hint: graphUsageHint(), ExitCode: ExitUsage})
		return ExitUsage
	}
	switch args[0] {
	case "init":
		options, cliErr := parseGraphInitArgs(args)
		if cliErr != nil {
			writeError(stderr, jsonMode, *cliErr)
			return cliErr.ExitCode
		}
		result, err := project.InitWorkflowGraph(root, options)
		if err != nil {
			cliErr := errorFromProjectProblem(err)
			writeError(stderr, jsonMode, cliErr)
			return cliErr.ExitCode
		}
		writeGraphInitResult(stdout, result, jsonMode)
		return ExitOK
	case "validate":
		options, cliErr := parseGraphArgs(args, "validate")
		if cliErr != nil {
			writeError(stderr, jsonMode, *cliErr)
			return cliErr.ExitCode
		}
		result := project.ValidateWorkflowGraph(root, options)
		writeGraphValidateResult(stdout, result, jsonMode)
		if result.Status == project.GraphStatusPass {
			return ExitOK
		}
		return ExitSafety
	case "explain":
		options, cliErr := parseGraphArgs(args, "explain")
		if cliErr != nil {
			writeError(stderr, jsonMode, *cliErr)
			return cliErr.ExitCode
		}
		result := project.ExplainWorkflowGraph(root, options)
		writeGraphExplainResult(stdout, result, jsonMode)
		if result.Status == project.GraphStatusPass {
			return ExitOK
		}
		return ExitSafety
	case "diff":
		options, cliErr := parseGraphDiffArgs(args)
		if cliErr != nil {
			writeError(stderr, jsonMode, *cliErr)
			return cliErr.ExitCode
		}
		result := project.DiffWorkflowGraph(root, options)
		writeGraphDiffResult(stdout, result, jsonMode)
		if result.Status == project.GraphStatusPass {
			return ExitOK
		}
		return ExitSafety
	case "propose":
		options, cliErr := parseGraphProposeArgs(args)
		if cliErr != nil {
			writeError(stderr, jsonMode, *cliErr)
			return cliErr.ExitCode
		}
		result, err := project.ProposeWorkflowGraph(root, options)
		if err != nil {
			cliErr := errorFromProjectProblem(err)
			writeError(stderr, jsonMode, cliErr)
			return cliErr.ExitCode
		}
		writeGraphProposalResult(stdout, result, jsonMode)
		return ExitOK
	case "apply":
		options, cliErr := parseGraphApplyArgs(args)
		if cliErr != nil {
			writeError(stderr, jsonMode, *cliErr)
			return cliErr.ExitCode
		}
		result, err := project.ApplyWorkflowGraph(root, options)
		if err != nil {
			cliErr := errorFromProjectProblem(err)
			writeError(stderr, jsonMode, cliErr)
			return cliErr.ExitCode
		}
		writeGraphApplyResult(stdout, result, jsonMode)
		return ExitOK
	case "export":
		options, cliErr := parseGraphExportArgs(args)
		if cliErr != nil {
			writeError(stderr, jsonMode, *cliErr)
			return cliErr.ExitCode
		}
		result, err := project.ExportWorkflowGraph(root, options)
		if err != nil {
			cliErr := errorFromProjectProblem(err)
			writeError(stderr, jsonMode, cliErr)
			return cliErr.ExitCode
		}
		writeGraphExportResult(stdout, result, jsonMode)
		if result.Status == project.GraphStatusPass {
			return ExitOK
		}
		return ExitSafety
	default:
		writeError(stderr, jsonMode, cliError{Code: "graph_subcommand_unknown", Message: "graph subcommand is not supported", Hint: graphUsageHint(), ExitCode: ExitUsage, Field: "subcommand", Expected: "init, validate, explain, diff, propose, apply, or export", Actual: args[0]})
		return ExitUsage
	}
}

func parseGraphInitArgs(args []string) (project.GraphInitOptions, *cliError) {
	options := project.GraphInitOptions{}
	if len(args) == 0 || args[0] != "init" {
		return options, &cliError{Code: "graph_subcommand_required", Message: "graph subcommand is required", Hint: graphUsageHint(), ExitCode: ExitUsage, Field: "subcommand", Expected: "init", Actual: "missing"}
	}
	seenFromTemplate := false
	seenOutput := false
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--from-template", "--output":
			option := args[i]
			if option == "--from-template" && seenFromTemplate || option == "--output" && seenOutput {
				return options, &cliError{Code: "duplicate_option", Message: "duplicate graph option \"" + option + "\"", Hint: graphUsageHint(), ExitCode: ExitUsage, Field: "option", Expected: "option appears once", Actual: option}
			}
			value, next, cliErr := parseGraphValueOption(args, i, option, "non-empty value")
			if cliErr != nil {
				return options, cliErr
			}
			if option == "--from-template" {
				seenFromTemplate = true
				options.FromTemplate = value
			} else {
				seenOutput = true
				options.Output = value
			}
			i = next
		case "--profile":
			return options, &cliError{Code: "unknown_option", Message: "graph init does not support --profile", Hint: graphUsageHint(), ExitCode: ExitUsage, Field: "option", Expected: "--from-template or --output", Actual: "--profile"}
		default:
			return options, &cliError{Code: "unknown_option", Message: fmt.Sprintf("unknown graph init option %q", args[i]), Hint: graphUsageHint(), ExitCode: ExitUsage, Field: "option", Expected: "--from-template or --output", Actual: args[i]}
		}
	}
	if !seenFromTemplate {
		return options, &cliError{Code: "missing_required_option", Message: "graph init requires --from-template", Hint: graphUsageHint(), ExitCode: ExitUsage, Field: "--from-template", Expected: "required graph init option", Actual: "missing"}
	}
	return options, nil
}

func parseGraphArgs(args []string, subcommand string) (project.GraphOptions, *cliError) {
	options := project.GraphOptions{}
	if len(args) == 0 || args[0] != subcommand {
		return options, &cliError{Code: "graph_subcommand_required", Message: "graph subcommand is required", Hint: graphUsageHint(), ExitCode: ExitUsage, Field: "subcommand", Expected: subcommand, Actual: "missing"}
	}
	seenFile := false
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--file":
			if seenFile {
				return options, &cliError{Code: "duplicate_option", Message: "duplicate graph option \"--file\"", Hint: graphUsageHint(), ExitCode: ExitUsage, Field: "option", Expected: "option appears once", Actual: "--file"}
			}
			value, next, cliErr := parseGraphValueOption(args, i, "--file", "repository-relative workflow graph path")
			if cliErr != nil {
				return options, cliErr
			}
			seenFile = true
			options.File = value
			i = next
		default:
			return options, &cliError{Code: "unknown_option", Message: fmt.Sprintf("unknown graph %s option %q", subcommand, args[i]), Hint: graphUsageHint(), ExitCode: ExitUsage, Field: "option", Expected: "--file", Actual: args[i]}
		}
	}
	return options, nil
}

func parseGraphDiffArgs(args []string) (project.GraphDiffOptions, *cliError) {
	options := project.GraphDiffOptions{}
	if len(args) == 0 || args[0] != "diff" {
		return options, &cliError{Code: "graph_subcommand_required", Message: "graph subcommand is required", Hint: graphUsageHint(), ExitCode: ExitUsage, Field: "subcommand", Expected: "diff", Actual: "missing"}
	}
	seen := map[string]bool{}
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--semantic":
			if seen["--semantic"] {
				return options, &cliError{Code: "duplicate_option", Message: "duplicate graph option \"--semantic\"", Hint: graphUsageHint(), ExitCode: ExitUsage, Field: "option", Expected: "option appears once", Actual: "--semantic"}
			}
			seen["--semantic"] = true
		case "--from", "--to":
			option := args[i]
			if seen[option] {
				return options, &cliError{Code: "duplicate_option", Message: "duplicate graph option \"" + option + "\"", Hint: graphUsageHint(), ExitCode: ExitUsage, Field: "option", Expected: "option appears once", Actual: option}
			}
			value, next, cliErr := parseGraphValueOption(args, i, option, "repository-relative workflow graph path")
			if cliErr != nil {
				return options, cliErr
			}
			seen[option] = true
			if option == "--from" {
				options.From = value
			} else {
				options.To = value
			}
			i = next
		default:
			return options, &cliError{Code: "unknown_option", Message: fmt.Sprintf("unknown graph diff option %q", args[i]), Hint: graphUsageHint(), ExitCode: ExitUsage, Field: "option", Expected: "--from, --to, or --semantic", Actual: args[i]}
		}
	}
	for _, required := range []string{"--from", "--to"} {
		if !seen[required] {
			return options, &cliError{Code: "missing_required_option", Message: "graph diff requires " + required, Hint: graphUsageHint(), ExitCode: ExitUsage, Field: required, Expected: "required graph diff option", Actual: "missing"}
		}
	}
	return options, nil
}

func parseGraphProposeArgs(args []string) (project.GraphProposeOptions, *cliError) {
	options := project.GraphProposeOptions{}
	if len(args) == 0 || args[0] != "propose" {
		return options, &cliError{Code: "graph_subcommand_required", Message: "graph subcommand is required", Hint: graphUsageHint(), ExitCode: ExitUsage, Field: "subcommand", Expected: "propose", Actual: "missing"}
	}
	seen := map[string]bool{}
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--patch", "--reason":
			option := args[i]
			if seen[option] {
				return options, &cliError{Code: "duplicate_option", Message: "duplicate graph option \"" + option + "\"", Hint: graphUsageHint(), ExitCode: ExitUsage, Field: "option", Expected: "option appears once", Actual: option}
			}
			value, next, cliErr := parseGraphValueOption(args, i, option, "non-empty value")
			if cliErr != nil {
				return options, cliErr
			}
			seen[option] = true
			if option == "--patch" {
				options.Patch = value
			} else {
				options.Reason = value
			}
			i = next
		default:
			return options, &cliError{Code: "unknown_option", Message: fmt.Sprintf("unknown graph propose option %q", args[i]), Hint: graphUsageHint(), ExitCode: ExitUsage, Field: "option", Expected: "--patch or --reason", Actual: args[i]}
		}
	}
	for _, required := range []string{"--patch", "--reason"} {
		if !seen[required] {
			return options, &cliError{Code: "missing_required_option", Message: "graph propose requires " + required, Hint: graphUsageHint(), ExitCode: ExitUsage, Field: required, Expected: "required graph propose option", Actual: "missing"}
		}
	}
	return options, nil
}

func parseGraphApplyArgs(args []string) (project.GraphApplyOptions, *cliError) {
	options := project.GraphApplyOptions{}
	if len(args) == 0 || args[0] != "apply" {
		return options, &cliError{Code: "graph_subcommand_required", Message: "graph subcommand is required", Hint: graphUsageHint(), ExitCode: ExitUsage, Field: "subcommand", Expected: "apply", Actual: "missing"}
	}
	seen := map[string]bool{}
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--proposal", "--approval":
			option := args[i]
			if seen[option] {
				return options, &cliError{Code: "duplicate_option", Message: "duplicate graph option \"" + option + "\"", Hint: graphUsageHint(), ExitCode: ExitUsage, Field: "option", Expected: "option appears once", Actual: option}
			}
			value, next, cliErr := parseGraphValueOption(args, i, option, "non-empty value")
			if cliErr != nil {
				return options, cliErr
			}
			seen[option] = true
			if option == "--proposal" {
				options.Proposal = value
			} else {
				options.Approval = value
			}
			i = next
		default:
			return options, &cliError{Code: "unknown_option", Message: fmt.Sprintf("unknown graph apply option %q", args[i]), Hint: graphUsageHint(), ExitCode: ExitUsage, Field: "option", Expected: "--proposal or --approval", Actual: args[i]}
		}
	}
	for _, required := range []string{"--proposal", "--approval"} {
		if !seen[required] {
			return options, &cliError{Code: "missing_required_option", Message: "graph apply requires " + required, Hint: graphUsageHint(), ExitCode: ExitUsage, Field: required, Expected: "required graph apply option", Actual: "missing"}
		}
	}
	return options, nil
}

func parseGraphExportArgs(args []string) (project.GraphExportOptions, *cliError) {
	options := project.GraphExportOptions{}
	if len(args) == 0 || args[0] != "export" {
		return options, &cliError{Code: "graph_subcommand_required", Message: "graph subcommand is required", Hint: graphUsageHint(), ExitCode: ExitUsage, Field: "subcommand", Expected: "export", Actual: "missing"}
	}
	seen := map[string]bool{}
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--format", "--output":
			option := args[i]
			if seen[option] {
				return options, &cliError{Code: "duplicate_option", Message: "duplicate graph option \"" + option + "\"", Hint: graphUsageHint(), ExitCode: ExitUsage, Field: "option", Expected: "option appears once", Actual: option}
			}
			expected := "non-empty value"
			if option == "--output" {
				expected = "repository-relative generated diagram path"
			}
			value, next, cliErr := parseGraphValueOption(args, i, option, expected)
			if cliErr != nil {
				return options, cliErr
			}
			seen[option] = true
			if option == "--format" {
				if value != "mermaid" && value != "plantuml" {
					return options, &cliError{Code: "graph_export_format_invalid", Message: "graph export format is not supported", Hint: graphUsageHint(), ExitCode: ExitUsage, Field: "format", Expected: "mermaid or plantuml", Actual: value}
				}
				options.Format = value
			} else {
				options.Output = value
			}
			i = next
		default:
			return options, &cliError{Code: "unknown_option", Message: fmt.Sprintf("unknown graph export option %q", args[i]), Hint: graphUsageHint(), ExitCode: ExitUsage, Field: "option", Expected: "--format or --output", Actual: args[i]}
		}
	}
	if !seen["--format"] {
		return options, &cliError{Code: "missing_required_option", Message: "graph export requires --format", Hint: graphUsageHint(), ExitCode: ExitUsage, Field: "--format", Expected: "required graph export option", Actual: "missing"}
	}
	return options, nil
}

func parseGraphValueOption(args []string, index int, option string, expected string) (string, int, *cliError) {
	if index+1 >= len(args) {
		return "", index, &cliError{Code: "missing_option_value", Message: option + " requires a value", Hint: graphUsageHint(), ExitCode: ExitUsage, Field: option, Expected: expected, Actual: "missing"}
	}
	value := args[index+1]
	if strings.TrimSpace(value) == "" {
		return "", index, &cliError{Code: "missing_option_value", Message: option + " requires a non-empty value", Hint: graphUsageHint(), ExitCode: ExitUsage, Field: option, Expected: expected, Actual: "empty"}
	}
	return value, index + 1, nil
}

func runApprovalCommand(args []string, root project.Root, stdout io.Writer, stderr io.Writer, jsonMode bool) int {
	if len(args) == 0 {
		writeError(stderr, jsonMode, cliError{Code: "approval_subcommand_required", Message: "approval subcommand is required", Hint: approvalUsageHint(), ExitCode: ExitUsage})
		return ExitUsage
	}
	switch args[0] {
	case "request":
		options, cliErr := parseApprovalRequestArgs(args)
		if cliErr != nil {
			writeError(stderr, jsonMode, *cliErr)
			return cliErr.ExitCode
		}
		result, err := project.RequestApproval(root, options)
		if err != nil {
			cliErr := errorFromProjectProblem(err)
			writeError(stderr, jsonMode, cliErr)
			return cliErr.ExitCode
		}
		writeApprovalMutationResult(stdout, result, jsonMode)
		return ExitOK
	case "record":
		options, cliErr := parseApprovalRecordArgs(args)
		if cliErr != nil {
			writeError(stderr, jsonMode, *cliErr)
			return cliErr.ExitCode
		}
		result, err := project.RecordApproval(root, options)
		if err != nil {
			cliErr := errorFromProjectProblem(err)
			writeError(stderr, jsonMode, cliErr)
			return cliErr.ExitCode
		}
		writeApprovalMutationResult(stdout, result, jsonMode)
		return ExitOK
	case "show":
		options, cliErr := parseApprovalShowArgs(args)
		if cliErr != nil {
			writeError(stderr, jsonMode, *cliErr)
			return cliErr.ExitCode
		}
		result, err := project.ShowApprovals(root, options)
		if err != nil {
			cliErr := errorFromProjectProblem(err)
			writeError(stderr, jsonMode, cliErr)
			return cliErr.ExitCode
		}
		writeApprovalShowResult(stdout, result, jsonMode)
		return ExitOK
	default:
		writeError(stderr, jsonMode, cliError{Code: "approval_subcommand_unknown", Message: "approval subcommand is not supported", Hint: approvalUsageHint(), ExitCode: ExitUsage, Field: "subcommand", Expected: "request, record, or show", Actual: args[0]})
		return ExitUsage
	}
}

func parseApprovalRequestArgs(args []string) (project.ApprovalRequestOptions, *cliError) {
	options := project.ApprovalRequestOptions{}
	parsed, cliErr := parseApprovalFlags(args, "request", map[string]approvalFlagSpec{
		"--phase":    {expectedValue: "non-empty value"},
		"--reason":   {expectedValue: "non-empty value"},
		"--evidence": {expectedValue: "non-empty value"},
	}, "--phase, --reason, or --evidence")
	if cliErr != nil {
		return options, cliErr
	}
	options.RunID = parsed.runID
	options.Phase = parsed.values["--phase"]
	options.Reason = parsed.values["--reason"]
	options.Evidence = parsed.values["--evidence"]
	if cliErr := requireApprovalOptions("request", parsed.seen, "--phase", "--reason"); cliErr != nil {
		return options, cliErr
	}
	return options, nil
}

func parseApprovalRecordArgs(args []string) (project.ApprovalRecordOptions, *cliError) {
	options := project.ApprovalRecordOptions{}
	parsed, cliErr := parseApprovalFlags(args, "record", map[string]approvalFlagSpec{
		"--phase":    {expectedValue: "non-empty value"},
		"--decision": {expectedValue: "non-empty value"},
		"--by":       {expectedValue: "non-empty value"},
		"--evidence": {expectedValue: "non-empty value"},
		"--reason":   {expectedValue: "non-empty value"},
	}, "--phase, --decision, --by, --evidence, or --reason")
	if cliErr != nil {
		return options, cliErr
	}
	options.RunID = parsed.runID
	options.Phase = parsed.values["--phase"]
	options.Decision = parsed.values["--decision"]
	options.Approver = parsed.values["--by"]
	options.Evidence = parsed.values["--evidence"]
	options.Reason = parsed.values["--reason"]
	if cliErr := requireApprovalOptions("record", parsed.seen, "--phase", "--decision", "--by", "--evidence"); cliErr != nil {
		return options, cliErr
	}
	return options, nil
}

func parseApprovalShowArgs(args []string) (project.ApprovalShowOptions, *cliError) {
	options := project.ApprovalShowOptions{}
	parsed, cliErr := parseApprovalFlags(args, "show", map[string]approvalFlagSpec{
		"--phase": {expectedValue: "phase id"},
	}, "--phase")
	if cliErr != nil {
		return options, cliErr
	}
	options.RunID = parsed.runID
	options.Phase = parsed.values["--phase"]
	return options, nil
}

type approvalFlagSpec struct {
	expectedValue string
}

type approvalFlagParseResult struct {
	runID  string
	values map[string]string
	seen   map[string]bool
}

func parseApprovalFlags(args []string, subcommand string, specs map[string]approvalFlagSpec, expectedOptions string) (approvalFlagParseResult, *cliError) {
	result := approvalFlagParseResult{values: map[string]string{}, seen: map[string]bool{}}
	if len(args) < 2 || strings.HasPrefix(args[1], "--") {
		return result, &cliError{Code: "run_id_required", Message: fmt.Sprintf("approval %s requires exactly one run id", subcommand), Hint: approvalUsageHint(), ExitCode: ExitUsage, Field: "run_id", Expected: "one run id or unique prefix", Actual: fmt.Sprintf("%d arguments", len(args)-1)}
	}
	result.runID = args[1]
	for i := 2; i < len(args); i++ {
		flag := args[i]
		spec, ok := specs[flag]
		if !ok {
			return result, &cliError{Code: "unknown_option", Message: fmt.Sprintf("unknown approval %s option %q", subcommand, flag), Hint: approvalUsageHint(), ExitCode: ExitUsage, Field: "option", Expected: expectedOptions, Actual: flag}
		}
		if i+1 >= len(args) {
			return result, &cliError{Code: "missing_option_value", Message: flag + " requires a value", Hint: approvalUsageHint(), ExitCode: ExitUsage, Field: flag, Expected: spec.expectedValue, Actual: "missing"}
		}
		if result.seen[flag] {
			return result, &cliError{Code: "duplicate_option", Message: fmt.Sprintf("duplicate approval %s option %q", subcommand, flag), Hint: approvalUsageHint(), ExitCode: ExitUsage, Field: "option", Expected: "option appears once", Actual: flag}
		}
		result.seen[flag] = true
		result.values[flag] = args[i+1]
		i++
	}
	return result, nil
}

func requireApprovalOptions(subcommand string, seen map[string]bool, requiredOptions ...string) *cliError {
	for _, required := range requiredOptions {
		if !seen[required] {
			return &cliError{Code: "missing_required_option", Message: fmt.Sprintf("approval %s requires %s", subcommand, required), Hint: approvalUsageHint(), ExitCode: ExitUsage, Field: required, Expected: "required option", Actual: "missing"}
		}
	}
	return nil
}

func requireOnePhasePlanRunID(args []string) *cliError {
	command := args[0]
	if len(args) == 2 {
		return nil
	}
	if len(args) > 2 && strings.HasPrefix(args[2], "--") {
		return &cliError{Code: "unknown_option", Message: fmt.Sprintf("unknown phase-plan %s option %q", command, args[2]), Hint: phasePlanUsageHint(), ExitCode: ExitUsage, Field: "option", Expected: "no phase-plan " + command + " options", Actual: args[2]}
	}
	return &cliError{Code: "run_id_required", Message: fmt.Sprintf("phase-plan %s requires exactly one run id", command), Hint: phasePlanUsageHint(), ExitCode: ExitUsage, Field: "run_id", Expected: "one run id or unique prefix", Actual: fmt.Sprintf("%d arguments", len(args)-1)}
}

func parsePhasePlanSetArgs(args []string) (project.PhasePlanSetOptions, *cliError) {
	options := project.PhasePlanSetOptions{}
	if len(args) < 3 || strings.HasPrefix(args[1], "--") || strings.HasPrefix(args[2], "--") {
		return options, &cliError{Code: "phase_plan_set_arguments_required", Message: "phase-plan set requires run id and phase id", Hint: phasePlanUsageHint(), ExitCode: ExitUsage, Field: "arguments", Expected: "phase-plan set <run_id> <phase-id>", Actual: fmt.Sprintf("%d arguments", len(args)-1)}
	}
	options.RunID = args[1]
	options.PhaseID = args[2]
	seenStatus := false
	seenEvidence := false
	seenReason := false
	seenApprovalRequired := false
	for i := 3; i < len(args); i++ {
		switch args[i] {
		case "--status":
			if i+1 >= len(args) {
				return options, &cliError{Code: "missing_option_value", Message: "--status requires a value", Hint: phasePlanUsageHint(), ExitCode: ExitUsage, Field: "--status", Expected: "phase status", Actual: "missing"}
			}
			if seenStatus {
				return options, &cliError{Code: "duplicate_option", Message: "duplicate phase-plan set option \"--status\"", Hint: phasePlanUsageHint(), ExitCode: ExitUsage, Field: "option", Expected: "option appears once", Actual: "--status"}
			}
			seenStatus = true
			options.Status = args[i+1]
			i++
		case "--evidence":
			if i+1 >= len(args) {
				return options, &cliError{Code: "missing_option_value", Message: "--evidence requires a value", Hint: phasePlanUsageHint(), ExitCode: ExitUsage, Field: "--evidence", Expected: "evidence reference", Actual: "missing"}
			}
			if seenEvidence {
				return options, &cliError{Code: "duplicate_option", Message: "duplicate phase-plan set option \"--evidence\"", Hint: phasePlanUsageHint(), ExitCode: ExitUsage, Field: "option", Expected: "option appears once", Actual: "--evidence"}
			}
			seenEvidence = true
			options.Evidence = args[i+1]
			i++
		case "--reason":
			if i+1 >= len(args) {
				return options, &cliError{Code: "missing_option_value", Message: "--reason requires a value", Hint: phasePlanUsageHint(), ExitCode: ExitUsage, Field: "--reason", Expected: "reason text", Actual: "missing"}
			}
			if seenReason {
				return options, &cliError{Code: "duplicate_option", Message: "duplicate phase-plan set option \"--reason\"", Hint: phasePlanUsageHint(), ExitCode: ExitUsage, Field: "option", Expected: "option appears once", Actual: "--reason"}
			}
			seenReason = true
			options.Reason = args[i+1]
			i++
		case "--approval-required":
			if i+1 >= len(args) {
				return options, &cliError{Code: "missing_option_value", Message: "--approval-required requires a value", Hint: phasePlanUsageHint(), ExitCode: ExitUsage, Field: "--approval-required", Expected: "true or false", Actual: "missing"}
			}
			if seenApprovalRequired {
				return options, &cliError{Code: "duplicate_option", Message: "duplicate phase-plan set option \"--approval-required\"", Hint: phasePlanUsageHint(), ExitCode: ExitUsage, Field: "option", Expected: "option appears once", Actual: "--approval-required"}
			}
			value := strings.TrimSpace(args[i+1])
			if value != "true" && value != "false" {
				return options, &cliError{Code: "invalid_option_value", Message: "--approval-required must be true or false", Hint: phasePlanUsageHint(), ExitCode: ExitUsage, Field: "--approval-required", Expected: "true or false", Actual: value}
			}
			seenApprovalRequired = true
			options.ApprovalRequiredSet = true
			options.ApprovalRequired = value == "true"
			i++
		default:
			return options, &cliError{Code: "unknown_option", Message: fmt.Sprintf("unknown phase-plan set option %q", args[i]), Hint: phasePlanUsageHint(), ExitCode: ExitUsage, Field: "option", Expected: "--status, --evidence, --reason, or --approval-required", Actual: args[i]}
		}
	}
	if !seenStatus {
		return options, &cliError{Code: "missing_required_option", Message: "phase-plan set requires --status", Hint: phasePlanUsageHint(), ExitCode: ExitUsage, Field: "--status", Expected: "required option", Actual: "missing"}
	}
	return options, nil
}

func parsePhasePlanValidateArgs(args []string) (project.PhasePlanValidationOptions, *cliError) {
	options := project.PhasePlanValidationOptions{}
	if len(args) < 2 || strings.HasPrefix(args[1], "--") {
		return options, &cliError{Code: "run_id_required", Message: "phase-plan validate requires exactly one run id", Hint: phasePlanUsageHint(), ExitCode: ExitUsage, Field: "run_id", Expected: "one run id or unique prefix", Actual: fmt.Sprintf("%d arguments", len(args)-1)}
	}
	options.RunID = args[1]
	for i := 2; i < len(args); i++ {
		switch args[i] {
		case "--final":
			options.Final = true
		default:
			return options, &cliError{Code: "unknown_option", Message: fmt.Sprintf("unknown phase-plan validate option %q", args[i]), Hint: phasePlanUsageHint(), ExitCode: ExitUsage, Field: "option", Expected: "--final", Actual: args[i]}
		}
	}
	return options, nil
}

func runProjectCommand(args []string, root project.Root, stdout io.Writer, stderr io.Writer, jsonMode bool) int {
	if len(args) == 0 {
		writeError(stderr, jsonMode, cliError{Code: "project_subcommand_required", Message: "project subcommand is required", Hint: projectUsageHint(), ExitCode: ExitUsage})
		return ExitUsage
	}
	if args[0] == "init" {
		options, cliErr := parseProjectInitArgs(args[1:])
		if cliErr != nil {
			writeError(stderr, jsonMode, *cliErr)
			return cliErr.ExitCode
		}
		result, err := project.InitProject(root, options)
		if err != nil {
			cliErr := errorFromProjectProblem(err)
			writeError(stderr, jsonMode, cliErr)
			return cliErr.ExitCode
		}
		writeProjectInitResult(stdout, result, jsonMode)
		return ExitOK
	}

	if len(args) != 1 && isImplementedProjectSubcommand(args[0]) {
		writeError(stderr, jsonMode, cliError{Code: "unknown_option", Message: fmt.Sprintf("unknown project %s option %q", args[0], args[1]), Hint: projectUsageHint(), ExitCode: ExitUsage, Field: "option", Expected: "no project status/doctor options", Actual: args[1]})
		return ExitUsage
	}

	if args[0] == "status" {
		result, err := project.InspectProjectStatus(root)
		if err != nil {
			cliErr := errorFromProjectProblem(err)
			writeError(stderr, jsonMode, cliErr)
			return cliErr.ExitCode
		}
		writeProjectStatusResult(stdout, result, jsonMode)
		return exitCodeForHealth(result.Health)
	}

	if args[0] == "doctor" {
		result, err := project.Doctor(root)
		if err != nil {
			cliErr := errorFromProjectProblem(err)
			writeError(stderr, jsonMode, cliErr)
			return cliErr.ExitCode
		}
		writeProjectDoctorResult(stdout, result, jsonMode)
		return exitCodeForHealth(result.Health)
	}

	writeError(stderr, jsonMode, cliError{Code: "not_implemented", Message: "project command is not implemented yet", Hint: projectUsageHint(), ExitCode: ExitUsage})
	return ExitUsage
}

func parseProjectInitArgs(args []string) (project.InitOptions, *cliError) {
	options := project.InitOptions{}
	seen := map[string]bool{}
	setString := func(flag string, value string) *cliError {
		if seen[flag] {
			return &cliError{Code: "duplicate_option", Message: fmt.Sprintf("duplicate project init option %q", flag), Hint: projectInitUsageHint(), ExitCode: ExitUsage, Field: "option", Expected: "option appears once", Actual: flag}
		}
		seen[flag] = true
		switch flag {
		case "--project-name":
			options.Bootstrap.ProjectName = value
		case "--stack":
			options.Bootstrap.Stack = value
		case "--repo-path":
			options.Bootstrap.RepoPath = value
		case "--commander":
			options.Bootstrap.Commander = value
		case "--redteam":
			options.Bootstrap.Redteam = value
		case "--docs-map-roadmap":
			options.Bootstrap.DocsMapRoadmap = value
		case "--docs-map-spec":
			options.Bootstrap.DocsMapSpec = value
		case "--docs-map-architecture":
			options.Bootstrap.DocsMapArchitecture = value
		case "--docs-map-adr-dir":
			options.Bootstrap.DocsMapADRDir = value
		case "--docs-map-todo-dir":
			options.Bootstrap.DocsMapTODODir = value
		case "--docs-map-spec-dir":
			options.Bootstrap.DocsMapSpecDir = value
		case "--test-commands":
			options.Bootstrap.TestCommands = splitCommaSeparated(value)
		case "--backend-policy":
			options.Bootstrap.BackendPolicy = value
		case "--execution-mode":
			options.Bootstrap.ExecutionMode = value
		case "--sot-policy":
			options.Bootstrap.SOTPolicy = value
		}
		return nil
	}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--force":
			if seen["--force"] {
				return options, &cliError{Code: "duplicate_option", Message: "duplicate project init option \"--force\"", Hint: projectInitUsageHint(), ExitCode: ExitUsage, Field: "option", Expected: "option appears once", Actual: "--force"}
			}
			seen["--force"] = true
			options.Force = true
		case "--project-name", "--stack", "--repo-path", "--commander", "--redteam", "--docs-map-roadmap", "--docs-map-spec", "--docs-map-architecture", "--docs-map-adr-dir", "--docs-map-todo-dir", "--docs-map-spec-dir", "--test-commands", "--backend-policy", "--execution-mode", "--sot-policy":
			if i+1 >= len(args) {
				return options, &cliError{Code: "missing_option_value", Message: fmt.Sprintf("%s requires a value", args[i]), Hint: projectInitUsageHint(), ExitCode: ExitUsage, Field: args[i], Expected: "non-empty value", Actual: "missing"}
			}
			if err := setString(args[i], args[i+1]); err != nil {
				return options, err
			}
			i++
		default:
			return options, &cliError{Code: "unknown_option", Message: fmt.Sprintf("unknown project init option %q", args[i]), Hint: projectInitUsageHint(), ExitCode: ExitUsage, Field: "option", Expected: "project init bootstrap option", Actual: args[i]}
		}
	}
	for _, flag := range []string{"--project-name", "--stack", "--repo-path", "--commander", "--redteam", "--docs-map-roadmap", "--docs-map-spec", "--docs-map-architecture", "--docs-map-adr-dir", "--docs-map-todo-dir", "--docs-map-spec-dir", "--test-commands", "--backend-policy", "--execution-mode", "--sot-policy"} {
		if !seen[flag] {
			return options, &cliError{Code: "missing_required_option", Message: "project init requires bootstrap options", Hint: projectInitUsageHint(), ExitCode: ExitUsage, Field: flag, Expected: "required option", Actual: "missing"}
		}
	}
	return options, nil
}

func splitCommaSeparated(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
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
		case "--backend-evidence":
			options.BackendEvidence = value
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
		if err := requireOneRunID(args); err != nil {
			writeError(stderr, jsonMode, *err)
			return ExitUsage
		}
		result, err := project.CheckGate(root, project.GateCheckOptions{RunID: args[1], Gate: project.GateFinal})
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
	case "write", "append":
		runID, artifact, from, cliErr := parseArtifactFromArgs(args)
		if cliErr != nil {
			writeError(stderr, jsonMode, *cliErr)
			return cliErr.ExitCode
		}
		options := project.ArtifactMutateOptions{RunID: runID, Artifact: artifact, From: from}
		var result project.ArtifactMutationResult
		var err error
		if args[0] == "write" {
			result, err = project.WriteArtifact(root, options)
		} else {
			result, err = project.AppendArtifact(root, options)
		}
		if err != nil {
			cliErr := errorFromProjectProblem(err)
			writeError(stderr, jsonMode, cliErr)
			return cliErr.ExitCode
		}
		writeArtifactMutationResult(stdout, result, jsonMode)
		return ExitOK
	case "set-status":
		runID, artifact, status, reason, cliErr := parseArtifactSetStatusArgs(args)
		if cliErr != nil {
			writeError(stderr, jsonMode, *cliErr)
			return cliErr.ExitCode
		}
		result, err := project.SetArtifactStatus(root, project.ArtifactMutateOptions{RunID: runID, Artifact: artifact, Status: status, Reason: reason})
		if err != nil {
			cliErr := errorFromProjectProblem(err)
			writeError(stderr, jsonMode, cliErr)
			return cliErr.ExitCode
		}
		writeArtifactMutationResult(stdout, result, jsonMode)
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

func parseArtifactFromArgs(args []string) (string, string, string, *cliError) {
	command := args[0]
	if len(args) < 3 || strings.HasPrefix(args[1], "--") || strings.HasPrefix(args[2], "--") {
		return "", "", "", &cliError{Code: "artifact_arguments_required", Message: fmt.Sprintf("artifact %s requires a run id and artifact path", command), Hint: fmt.Sprintf("Use artifact %s <run_id> <artifact_path> --from <repo-relative-file>.", command), ExitCode: ExitUsage, Field: "arguments", Expected: fmt.Sprintf("artifact %s <run_id> <artifact_path> --from <file>", command), Actual: fmt.Sprintf("%d arguments", len(args)-1)}
	}
	runID := args[1]
	artifact := args[2]
	from := ""
	for i := 3; i < len(args); i++ {
		switch args[i] {
		case "--from":
			if i+1 >= len(args) {
				err := missingOptionValueError("--from", "repository-relative file", fmt.Sprintf("Use artifact %s <run_id> <artifact_path> --from <file>.", command))
				return "", "", "", &err
			}
			from = args[i+1]
			i++
		default:
			return "", "", "", &cliError{Code: "unknown_option", Message: fmt.Sprintf("unknown artifact %s option %q", command, args[i]), Hint: fmt.Sprintf("Use artifact %s <run_id> <artifact_path> --from <repo-relative-file> with optional global --json.", command), ExitCode: ExitUsage, Field: "option", Expected: "--from", Actual: args[i]}
		}
	}
	if strings.TrimSpace(from) == "" {
		return "", "", "", &cliError{Code: "from_required", Message: fmt.Sprintf("artifact %s requires --from", command), Hint: fmt.Sprintf("Use artifact %s <run_id> <artifact_path> --from <repo-relative-file>.", command), ExitCode: ExitUsage, Field: "from", Expected: "repository-relative source file", Actual: "missing"}
	}
	return runID, artifact, from, nil
}

func parseArtifactSetStatusArgs(args []string) (string, string, string, string, *cliError) {
	if len(args) < 3 || strings.HasPrefix(args[1], "--") || strings.HasPrefix(args[2], "--") {
		return "", "", "", "", &cliError{Code: "artifact_arguments_required", Message: "artifact set-status requires a run id and artifact path", Hint: "Use artifact set-status <run_id> <artifact_path> --status <pending|complete|not_applicable> [--reason <text>] with optional global --json.", ExitCode: ExitUsage, Field: "arguments", Expected: "artifact set-status <run_id> <artifact_path> --status <status>", Actual: fmt.Sprintf("%d arguments", len(args)-1)}
	}
	runID := args[1]
	artifact := args[2]
	status := ""
	reason := ""
	for i := 3; i < len(args); i++ {
		switch args[i] {
		case "--status":
			if i+1 >= len(args) {
				err := missingOptionValueError("--status", "pending, complete, or not_applicable", "Use artifact set-status <run_id> <artifact_path> --status <status>.")
				return "", "", "", "", &err
			}
			status = args[i+1]
			i++
		case "--reason":
			if i+1 >= len(args) {
				err := missingOptionValueError("--reason", "reason text", "Pass --reason with a non-empty explanation.")
				return "", "", "", "", &err
			}
			reason = args[i+1]
			i++
		default:
			return "", "", "", "", &cliError{Code: "unknown_option", Message: fmt.Sprintf("unknown artifact set-status option %q", args[i]), Hint: "Use artifact set-status <run_id> <artifact_path> --status <status> [--reason <text>] with optional global --json.", ExitCode: ExitUsage, Field: "option", Expected: "--status or --reason", Actual: args[i]}
		}
	}
	if strings.TrimSpace(status) == "" {
		return "", "", "", "", &cliError{Code: "status_required", Message: "artifact set-status requires --status", Hint: "Use artifact set-status <run_id> <artifact_path> --status <pending|complete|not_applicable>.", ExitCode: ExitUsage, Field: "status", Expected: "pending, complete, or not_applicable", Actual: "missing"}
	}
	return runID, artifact, status, reason, nil
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

func runSchemaCommand(args []string, root project.Root, stdout io.Writer, stderr io.Writer, jsonMode bool) int {
	if len(args) == 0 {
		writeError(stderr, jsonMode, cliError{Code: "schema_subcommand_required", Message: "schema subcommand is required", Hint: schemaUsageHint(), ExitCode: ExitUsage})
		return ExitUsage
	}
	switch args[0] {
	case "validate":
		file, schemaName, cliErr := parseSchemaValidateArgs(args)
		if cliErr != nil {
			writeError(stderr, jsonMode, *cliErr)
			return cliErr.ExitCode
		}
		result, err := project.ValidateSchemaFile(root, project.SchemaValidateOptions{File: file, Schema: schemaName})
		if err != nil {
			cliErr := errorFromProjectProblem(err)
			writeError(stderr, jsonMode, cliErr)
			return cliErr.ExitCode
		}
		writeSchemaValidateResult(stdout, result, jsonMode)
		if result.Status == "pass" {
			return ExitOK
		}
		return ExitSafety
	case "export":
		options, cliErr := parseSchemaExportArgs(args)
		if cliErr != nil {
			writeError(stderr, jsonMode, *cliErr)
			return cliErr.ExitCode
		}
		result, err := project.ExportSchemas(root, options)
		if err != nil {
			cliErr := errorFromProjectProblem(err)
			writeError(stderr, jsonMode, cliErr)
			return cliErr.ExitCode
		}
		writeSchemaExportResult(stdout, result, jsonMode)
		return ExitOK
	case "migrate":
		options, cliErr := parseSchemaMigrateArgs(args)
		if cliErr != nil {
			writeError(stderr, jsonMode, *cliErr)
			return cliErr.ExitCode
		}
		result, err := project.MigrateSchemaState(root, options)
		if err != nil {
			cliErr := errorFromProjectProblem(err)
			writeError(stderr, jsonMode, cliErr)
			return cliErr.ExitCode
		}
		writeSchemaMigrationResult(stdout, result, jsonMode)
		return ExitOK
	default:
		writeError(stderr, jsonMode, cliError{Code: "not_implemented", Message: "schema command is not implemented yet", Hint: schemaUsageHint(), ExitCode: ExitUsage})
		return ExitUsage
	}
}

func parseSchemaValidateArgs(args []string) (string, string, *cliError) {
	if len(args) < 2 || strings.HasPrefix(args[1], "--") {
		return "", "", &cliError{Code: "schema_file_required", Message: "schema validate requires a file", Hint: "Use schema validate <file> --schema <schema>.", ExitCode: ExitUsage, Field: "file", Expected: "repository-relative file path", Actual: fmt.Sprintf("%d arguments", len(args)-1)}
	}
	file := args[1]
	schemaName := ""
	for i := 2; i < len(args); i++ {
		switch args[i] {
		case "--schema":
			if i+1 >= len(args) {
				return "", "", &cliError{Code: "missing_option_value", Message: "--schema requires a value", Hint: schemaUsageHint(), ExitCode: ExitUsage, Field: "--schema", Expected: "schema name", Actual: "missing"}
			}
			if schemaName != "" {
				return "", "", &cliError{Code: "duplicate_option", Message: "duplicate schema validate option \"--schema\"", Hint: schemaUsageHint(), ExitCode: ExitUsage, Field: "option", Expected: "option appears once", Actual: "--schema"}
			}
			schemaName = args[i+1]
			i++
		default:
			return "", "", &cliError{Code: "unknown_option", Message: fmt.Sprintf("unknown schema validate option %q", args[i]), Hint: schemaUsageHint(), ExitCode: ExitUsage, Field: "option", Expected: "--schema", Actual: args[i]}
		}
	}
	if strings.TrimSpace(schemaName) == "" {
		return "", "", &cliError{Code: "missing_required_option", Message: "schema validate requires --schema", Hint: schemaUsageHint(), ExitCode: ExitUsage, Field: "--schema", Expected: "required option", Actual: "missing"}
	}
	return file, schemaName, nil
}

func parseSchemaMigrateArgs(args []string) (project.SchemaMigrationOptions, *cliError) {
	options := project.SchemaMigrationOptions{}
	seenFrom := false
	seenTo := false
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--from":
			if i+1 >= len(args) {
				return options, &cliError{Code: "missing_option_value", Message: "--from requires a value", Hint: schemaUsageHint(), ExitCode: ExitUsage, Field: "--from", Expected: "source version", Actual: "missing"}
			}
			if seenFrom {
				return options, &cliError{Code: "duplicate_option", Message: "duplicate schema migrate option \"--from\"", Hint: schemaUsageHint(), ExitCode: ExitUsage, Field: "option", Expected: "option appears once", Actual: "--from"}
			}
			seenFrom = true
			options.From = args[i+1]
			i++
		case "--to":
			if i+1 >= len(args) {
				return options, &cliError{Code: "missing_option_value", Message: "--to requires a value", Hint: schemaUsageHint(), ExitCode: ExitUsage, Field: "--to", Expected: "target version", Actual: "missing"}
			}
			if seenTo {
				return options, &cliError{Code: "duplicate_option", Message: "duplicate schema migrate option \"--to\"", Hint: schemaUsageHint(), ExitCode: ExitUsage, Field: "option", Expected: "option appears once", Actual: "--to"}
			}
			seenTo = true
			options.To = args[i+1]
			i++
		case "--dry-run":
			options.DryRun = true
		default:
			return options, &cliError{Code: "unknown_option", Message: fmt.Sprintf("unknown schema migrate option %q", args[i]), Hint: schemaUsageHint(), ExitCode: ExitUsage, Field: "option", Expected: "--from, --to, or --dry-run", Actual: args[i]}
		}
	}
	if !seenFrom {
		return options, &cliError{Code: "missing_required_option", Message: "schema migrate requires --from", Hint: schemaUsageHint(), ExitCode: ExitUsage, Field: "--from", Expected: "required option", Actual: "missing"}
	}
	if !seenTo {
		return options, &cliError{Code: "missing_required_option", Message: "schema migrate requires --to", Hint: schemaUsageHint(), ExitCode: ExitUsage, Field: "--to", Expected: "required option", Actual: "missing"}
	}
	return options, nil
}

func parseSchemaExportArgs(args []string) (project.SchemaExportOptions, *cliError) {
	options := project.SchemaExportOptions{}
	seenSchema := false
	seenAll := false
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--schema":
			if i+1 >= len(args) {
				return options, &cliError{Code: "missing_option_value", Message: "--schema requires a value", Hint: schemaUsageHint(), ExitCode: ExitUsage, Field: "--schema", Expected: "schema name", Actual: "missing"}
			}
			if seenSchema {
				return options, &cliError{Code: "duplicate_option", Message: "duplicate schema export option \"--schema\"", Hint: schemaUsageHint(), ExitCode: ExitUsage, Field: "option", Expected: "option appears once", Actual: "--schema"}
			}
			seenSchema = true
			options.Schema = args[i+1]
			i++
		case "--all":
			if seenAll {
				return options, &cliError{Code: "duplicate_option", Message: "duplicate schema export option \"--all\"", Hint: schemaUsageHint(), ExitCode: ExitUsage, Field: "option", Expected: "option appears once", Actual: "--all"}
			}
			seenAll = true
			options.All = true
		case "--dry-run":
			options.DryRun = true
		default:
			return options, &cliError{Code: "unknown_option", Message: fmt.Sprintf("unknown schema export option %q", args[i]), Hint: schemaUsageHint(), ExitCode: ExitUsage, Field: "option", Expected: "--schema, --all, or --dry-run", Actual: args[i]}
		}
	}
	return options, nil
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

func schemaUsageHint() string {
	return "Use schema validate <file> --schema <config|status|event|run-metadata|selected-cli|bridge-session-snapshot>, schema export [--schema <name>|--all] [--dry-run], or schema migrate --from <version> --to <version> [--dry-run] with optional global --json."
}

func projectUsageHint() string {
	return "Use project init <bootstrap-options> [--force], project status, or project doctor with optional global --json."
}

func projectInitUsageHint() string {
	return "Use project init --project-name <name> --stack <stack> --repo-path <path> --commander <profile> --redteam <profile> --docs-map-roadmap <path> --docs-map-spec <path> --docs-map-architecture <path> --docs-map-adr-dir <path> --docs-map-todo-dir <path> --docs-map-spec-dir <path> --test-commands <comma-separated> --backend-policy <policy> --execution-mode <mode> --sot-policy <policy> [--force]."
}

func diagnosticsUsageHint() string {
	return "Use diagnostics export [--run <run_id>] [--output <repo-relative-path>] with optional global --json."
}

func phasePlanUsageHint() string {
	return "Use phase-plan init <run_id>, phase-plan show <run_id>, phase-plan set <run_id> <phase-id> --status <pending|in_progress|complete|skipped|not_applicable|blocked> [--evidence <path>] [--reason <text>] [--approval-required true|false], or phase-plan validate <run_id> [--final] with optional global --json."
}

func graphUsageHint() string {
	return "Use graph init --from-template <template-id-or-path> [--output .kkachi-workflow.yaml], graph validate [--file .kkachi-workflow.yaml], graph explain [--file .kkachi-workflow.yaml], graph diff --from <graph> --to <graph> [--semantic], graph propose --patch <candidate-graph> --reason <text>, graph apply --proposal <proposal-id> --approval <evidence-ref>, or graph export --format mermaid|plantuml [--output <path>] with optional global --json."
}

func approvalUsageHint() string {
	return "Use approval request <run_id> --phase <phase-id> --reason <reason> [--evidence <ref>], approval record <run_id> --phase <phase-id> --decision <approved|rejected> --by <approver> --evidence <ref> [--reason <reason>], or approval show <run_id> [--phase <phase-id>] with optional global --json."
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
	return "Use artifact init|list|validate|write|append|set-status. Mutation commands accept canonical artifact paths only; use artifact write, not set-status, for schema-owned backend JSON."
}

func runUsageHint() string {
	return "Use run create|activate|close|abort|list|show."
}

func capabilitiesUsageHint() string {
	return "Use capabilities --json for the stable command-surface compatibility report."
}

func runCreateUsageHint() string {
	return "Use run create --title <title> --work-path <A_development_execution|B_discovery_shaping> --work-mode <standard|light> --urgency <normal|urgent|critical> --sot-policy <existing_sot_basis|minimal_sot_before_code|full_sot_before_code> --execution-mode <production_write|adapter_qa|readiness_hardening|research|verification|docs_only> --commander <profile> [--backend-evidence <auto|required|not_applicable>] [--task-id <id>] [--redteam <profile>]."
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

func writeCapabilities(w io.Writer, info BuildInfo, jsonMode bool) {
	payload := capabilitiesPayload(info)
	if jsonMode {
		_ = json.NewEncoder(w).Encode(payload)
		return
	}
	fmt.Fprintf(w, "%s capabilities\n", info.Name)
	fmt.Fprintf(w, "helper_version: %s\n", info.Version)
	fmt.Fprintf(w, "capabilities_schema_version: %s\n", payload.CapabilitiesSchemaVersion)
	fmt.Fprintf(w, "project_schema_version: %s\n", payload.ProjectSchemaVersion)
	fmt.Fprintln(w, "json_contract: use capabilities --json")
}

func writeHelp(w io.Writer, page helpOutput, jsonMode bool) {
	if jsonMode {
		_ = json.NewEncoder(w).Encode(page)
		return
	}
	fmt.Fprintf(w, "%s\n", page.Command)
	fmt.Fprintf(w, "\nUsage:\n  %s\n", page.Usage)
	if page.Status != "" {
		fmt.Fprintf(w, "\nStatus: %s\n", page.Status)
	}
	if page.Summary != "" {
		fmt.Fprintf(w, "\nSummary:\n  %s\n", page.Summary)
	}
	writeHelpItems(w, "Subcommands", page.Subcommands)
	writeHelpItems(w, "Arguments", page.Arguments)
	writeHelpItems(w, "Options", page.Options)
	if page.JSONBehavior != "" {
		fmt.Fprintf(w, "\nJSON behavior:\n  %s\n", page.JSONBehavior)
	}
	writeHelpNotes(w, page.Notes)
}

func writeHelpItems(w io.Writer, title string, items []helpItem) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(w, "\n%s:\n", title)
	for _, item := range items {
		required := ""
		if item.Required {
			required = " (required)"
		}
		fmt.Fprintf(w, "  %s%s - %s\n", item.Name, required, item.Description)
	}
}

func writeHelpNotes(w io.Writer, notes []string) {
	if len(notes) == 0 {
		return
	}
	fmt.Fprintln(w, "\nNotes:")
	for _, note := range notes {
		fmt.Fprintf(w, "  - %s\n", note)
	}
}

func capabilitiesPayload(info BuildInfo) capabilitiesOutput {
	return capabilitiesOutput{
		Helper:                    info,
		CapabilitiesSchemaVersion: capabilitiesSchemaVersion,
		ProjectSchemaVersion:      project.SchemaVersion,
		// Keep this explicit inventory aligned with commandGroups plus the
		// implemented command dispatch switch above. Planned command surfaces
		// are listed here only when KHS compatibility checks need to see them.
		CommandGroups: []capabilityCommandGroup{
			{Name: "project", Status: capabilityStatusSupported, Subcommands: []string{"init", "status", "doctor"}},
			{Name: "run", Status: capabilityStatusSupported, Subcommands: []string{"create", "activate", "close", "abort", "list", "show"}},
			{Name: "artifact", Status: capabilityStatusSupported, Subcommands: []string{"init", "list", "validate", "write", "append", "set-status"}},
			{Name: "gate", Status: capabilityStatusSupported, Subcommands: []string{"check", "final"}},
			{Name: "event", Status: capabilityStatusSupported, Subcommands: []string{"append"}},
			{Name: "schema", Status: capabilityStatusSupported, Subcommands: []string{"validate", "export", "migrate"}},
			{Name: "lock", Status: capabilityStatusSupported, Subcommands: []string{"recover"}},
			{Name: "diagnostics", Status: capabilityStatusSupported, Subcommands: []string{"export"}},
			{Name: "phase-plan", Status: capabilityStatusSupported, Subcommands: []string{"init", "show", "set", "validate"}},
			{Name: "approval", Status: capabilityStatusSupported, Subcommands: []string{"request", "record", "show"}},
			{Name: "graph", Status: capabilityStatusSupported, Subcommands: []string{"init", "validate", "explain", "diff", "propose", "apply", "export"}},
		},
		CompatibilityFlags: compatibilityFlagsOutput{
			ProjectInit:                       true,
			ProjectStatus:                     true,
			ProjectDoctor:                     true,
			RunLifecycle:                      true,
			ArtifactInit:                      true,
			ArtifactList:                      true,
			ArtifactValidate:                  true,
			ArtifactMutation:                  true,
			Gates:                             true,
			BackendEvidenceRequirements:       true,
			DiagnosticsExport:                 true,
			PhasePlan:                         true,
			ApprovalRecords:                   true,
			WorkflowGraphReadonly:             true,
			WorkflowGraphInit:                 true,
			WorkflowGraphApply:                true,
			WorkflowGraphExport:               true,
			WorkflowGraphDiagnostics:          true,
			WorkflowGraphNoDirectYAMLFallback: true,
			InstallCommand:                    false,
		},
		DeprecatedSurfaces: []capabilitySurfaceOutput{},
		OmittedSurfaces: []capabilitySurfaceOutput{
			{Name: "install", Status: capabilityStatusOmitted, Reason: "KAH project bootstrap is handled by project init; Hermes/KHS skill installation belongs to Hermes native tooling."},
		},
	}
}

var helpPages = map[string]helpOutput{
	"": {
		Command:      "kkachi-agent-helper",
		Status:       capabilityStatusSupported,
		Usage:        "kkachi-agent-helper [--json] <command> [subcommand] [options]",
		Summary:      "Deterministic local helper for Kkachi project state, run artifacts, gates, events, schemas, locks, diagnostics, and compatibility discovery.",
		Subcommands:  []helpItem{{Name: "version", Description: "Print helper build version."}, {Name: "capabilities", Description: "Print the command-surface compatibility report."}, {Name: "project", Description: "Initialize and inspect helper project state."}, {Name: "run", Description: "Create, list, show, activate, close, or abort helper runs."}, {Name: "artifact", Description: "Initialize, list, and validate canonical run artifacts."}, {Name: "gate", Description: "Run deterministic gate checks."}, {Name: "event", Description: "Append attributed helper events."}, {Name: "schema", Description: "Validate, export, or migrate helper schemas/state."}, {Name: "lock", Description: "Recover stale helper write locks explicitly."}, {Name: "diagnostics", Description: "Export redacted diagnostics bundles."}, {Name: "phase-plan", Description: "Manage KHS-declared phase-plan state."}, {Name: "approval", Description: "Record and show KHS-declared approval requests/decisions."}, {Name: "graph", Description: "Initialize and inspect project workflow graph state."}, {Name: "help", Description: "Show help for the top level or a command topic."}},
		Options:      []helpItem{{Name: "--json", Description: "Emit machine-readable JSON for commands that support JSON output."}, {Name: "--help", Description: "Show help and exit 0 without requiring helper project state."}, {Name: "--version", Description: "Print helper build version."}},
		JSONBehavior: "Use --json with help for structured help. Use capabilities --json for stable machine compatibility checks; command help is supplemental documentation.",
	},
	"help": {
		Command:      "kkachi-agent-helper help",
		Status:       capabilityStatusSupported,
		Usage:        "kkachi-agent-helper help [command] [subcommand]",
		Summary:      "Show help for the top level or a command topic without requiring helper project state.",
		Arguments:    []helpItem{{Name: "[command] [subcommand]", Description: "Optional help topic such as run create, project init, schema, event, lock, or phase-plan."}},
		JSONBehavior: "Use --json with help for structured help. Use capabilities --json for stable machine compatibility checks; command help is supplemental documentation.",
	},
	"project": {
		Command:      "kkachi-agent-helper project",
		Status:       capabilityStatusSupported,
		Usage:        "kkachi-agent-helper project <init|status|doctor> [options]",
		Summary:      "Manage helper project bootstrap and read-only project health inspection.",
		Subcommands:  []helpItem{{Name: "init", Description: "Create or reconfigure local .kkachi helper state."}, {Name: "status", Description: "Inspect current project health and active run state."}, {Name: "doctor", Description: "Run deterministic project health checks."}},
		Options:      []helpItem{{Name: "--json", Description: "Emit JSON for supported project subcommands."}, {Name: "--help", Description: "Show project help and exit 0."}},
		JSONBehavior: "Project subcommands support global --json for structured output and structured errors. project --help --json emits structured help only.",
	},
	"project init": {
		Command:      "kkachi-agent-helper project init",
		Status:       capabilityStatusSupported,
		Usage:        "kkachi-agent-helper project init --project-name <name> --stack <stack> --repo-path <path> --commander <profile> --redteam <profile> --docs-map-roadmap <path> --docs-map-spec <path> --docs-map-architecture <path> --docs-map-adr-dir <path> --docs-map-todo-dir <path> --docs-map-spec-dir <path> --test-commands <comma-separated> --backend-policy <policy> --execution-mode <mode> --sot-policy <policy> [--force] [--json]",
		Summary:      "Initialize helper-managed project state, local schema copies, event log, project overlay, and docs map.",
		Options:      []helpItem{{Name: "--project-name <name>", Required: true, Description: "Project display name."}, {Name: "--stack <stack>", Required: true, Description: "Project stack label."}, {Name: "--repo-path <path>", Required: true, Description: "Repository path recorded in helper overlay."}, {Name: "--commander <profile>", Required: true, Description: "Commander profile name."}, {Name: "--redteam <profile>", Required: true, Description: "Red-team profile name."}, {Name: "--docs-map-roadmap <path>", Required: true, Description: "Roadmap document path."}, {Name: "--docs-map-spec <path>", Required: true, Description: "Main spec document path."}, {Name: "--docs-map-architecture <path>", Required: true, Description: "Architecture document path."}, {Name: "--docs-map-adr-dir <path>", Required: true, Description: "ADR directory path."}, {Name: "--docs-map-todo-dir <path>", Required: true, Description: "TODO directory path."}, {Name: "--docs-map-spec-dir <path>", Required: true, Description: "Supplemental specs directory path."}, {Name: "--test-commands <comma-separated>", Required: true, Description: "Configured verification commands."}, {Name: "--backend-policy <policy>", Required: true, Description: "Declared backend policy."}, {Name: "--execution-mode <mode>", Required: true, Description: "Declared execution mode."}, {Name: "--sot-policy <policy>", Required: true, Description: "Declared source-of-truth policy."}, {Name: "--force", Description: "Reconfigure bootstrap files while preserving runs, status, and events."}, {Name: "--json", Description: "Emit structured init result."}},
		JSONBehavior: "With --json, successful init emits project paths, project id, created schema paths, event id, and force status. Errors are structured on stderr.",
	},
	"run": {
		Command:      "kkachi-agent-helper run",
		Status:       capabilityStatusSupported,
		Usage:        "kkachi-agent-helper run <create|list|show|activate|close|abort> [arguments] [options]",
		Summary:      "Manage helper run lifecycle metadata and active-run state.",
		Subcommands:  []helpItem{{Name: "create", Description: "Create a run with deterministic classification metadata."}, {Name: "list", Description: "List known runs."}, {Name: "show <run_id>", Description: "Show one run by id or unique prefix."}, {Name: "activate <run_id>", Description: "Mark a run active."}, {Name: "close <run_id>", Description: "Close a run."}, {Name: "abort <run_id>", Description: "Abort a run."}},
		Options:      []helpItem{{Name: "--json", Description: "Emit JSON for supported run subcommands."}, {Name: "--help", Description: "Show run help and exit 0."}},
		JSONBehavior: "Run subcommands support global --json for structured output and structured errors. run --help --json emits structured help only.",
	},
	"run create": {
		Command:      "kkachi-agent-helper run create",
		Status:       capabilityStatusSupported,
		Usage:        "kkachi-agent-helper run create --title <title> --work-path <A_development_execution|B_discovery_shaping> --work-mode <standard|light> --urgency <normal|urgent|critical> --sot-policy <existing_sot_basis|minimal_sot_before_code|full_sot_before_code> --execution-mode <production_write|adapter_qa|readiness_hardening|research|verification|docs_only> --commander <profile> [--backend-evidence <auto|required|not_applicable>] [--task-id <id>] [--redteam <profile>] [--json]",
		Summary:      "Create a helper run and append a run.created event.",
		Options:      []helpItem{{Name: "--title <title>", Required: true, Description: "Human-readable run title."}, {Name: "--work-path <A_development_execution|B_discovery_shaping>", Required: true, Description: "Declared Kkachi work path."}, {Name: "--work-mode <standard|light>", Required: true, Description: "Declared work mode."}, {Name: "--urgency <normal|urgent|critical>", Required: true, Description: "Declared urgency."}, {Name: "--sot-policy <existing_sot_basis|minimal_sot_before_code|full_sot_before_code>", Required: true, Description: "Declared source-of-truth policy."}, {Name: "--execution-mode <production_write|adapter_qa|readiness_hardening|research|verification|docs_only>", Required: true, Description: "Declared execution mode."}, {Name: "--commander <profile>", Required: true, Description: "Commander profile name."}, {Name: "--backend-evidence <auto|required|not_applicable>", Description: "Declare backend evidence requirement independently of execution mode."}, {Name: "--task-id <id>", Description: "Optional roadmap/task id."}, {Name: "--redteam <profile>", Description: "Optional red-team profile override."}, {Name: "--json", Description: "Emit structured run creation result."}},
		JSONBehavior: "With --json, successful create emits run id, state, run path, event id, and metadata. Errors are structured on stderr.",
	},
	"artifact": {
		Command:      "kkachi-agent-helper artifact",
		Status:       capabilityStatusSupported,
		Usage:        "kkachi-agent-helper artifact <init|list|validate|write|append|set-status> <run_id> [artifact_path] [options]",
		Summary:      "Initialize, list, validate, and safely mutate canonical run artifacts.",
		Subcommands:  []helpItem{{Name: "init <run_id>", Description: "Create required artifact placeholders/status for a run."}, {Name: "list <run_id>", Description: "List artifact statuses for a run."}, {Name: "validate <run_id> [--gate intake]", Description: "Validate artifacts for the supported artifact validation gate."}, {Name: "write <run_id> <artifact_path> --from <file>", Description: "Atomically replace a canonical artifact from a repository-relative file."}, {Name: "append <run_id> <artifact_path> --from <file>", Description: "Atomically append a repository-relative file to a canonical artifact."}, {Name: "set-status <run_id> <artifact_path> --status <status> [--reason <text>]", Description: "Update markdown lifecycle status; schema-owned backend JSON artifacts must be rewritten with artifact write."}},
		Options:      []helpItem{{Name: "--from <file>", Description: "Repository-relative source file for write or append."}, {Name: "--status <pending|complete|not_applicable>", Description: "Status for artifact set-status on markdown lifecycle artifacts."}, {Name: "--reason <text>", Description: "Reason required with not_applicable status."}, {Name: "--gate intake", Description: "Select artifact validation gate for artifact validate."}, {Name: "--json", Description: "Emit JSON for supported artifact subcommands."}, {Name: "--help", Description: "Show artifact help and exit 0."}},
		JSONBehavior: "Artifact subcommands support global --json for structured output and structured errors. artifact --help --json emits structured help only.",
		Notes:        []string{"Do not use artifact set-status on selected-cli.json or bridge-session-snapshot.json; use artifact write with valid backend JSON and let gate check backend validate completion."},
	},
	"gate": {
		Command:      "kkachi-agent-helper gate",
		Status:       capabilityStatusSupported,
		Usage:        "kkachi-agent-helper gate check <run_id> <gate> [--json]\n  kkachi-agent-helper gate final <run_id> [--json]",
		Summary:      "Run deterministic gate checks and record gate reports/events.",
		Arguments:    []helpItem{{Name: "<run_id>", Required: true, Description: "Run id or unique prefix."}, {Name: "<gate>", Required: true, Description: "One of: " + strings.Join(project.KnownGates(), ", ") + "."}},
		Subcommands:  []helpItem{{Name: "check <run_id> <gate>", Description: "Check a named gate."}, {Name: "final <run_id>", Description: "Check the final gate."}},
		Options:      []helpItem{{Name: "--json", Description: "Emit structured gate check result."}, {Name: "--help", Description: "Show gate help and exit 0."}},
		JSONBehavior: "With --json, gate checks emit gate status, checks, missing evidence, event id, and report path. Failing gates exit 3 with structured output.",
	},
	"schema": {
		Command: "kkachi-agent-helper schema",
		Status:  capabilityStatusSupported,
		Usage: "kkachi-agent-helper schema validate <file> --schema <schema> [--json]\n" +
			"  kkachi-agent-helper schema export [--schema <name>|--all] [--dry-run] [--json]\n" +
			"  kkachi-agent-helper schema migrate --from <version> --to <version> [--dry-run] [--json]",
		Summary:      "Validate helper state/evidence files against embedded schemas, export schema copies, or run registered migrations.",
		Subcommands:  []helpItem{{Name: "validate <file> --schema <schema>", Description: "Validate a repository file against an embedded schema name or canonical schema path."}, {Name: "export [--schema <name>|--all] [--dry-run]", Description: "Copy embedded schemas into .kkachi/schemas or preview the copy."}, {Name: "migrate --from <version> --to <version> [--dry-run]", Description: "Run registered helper state migrations or preview them."}},
		Options:      []helpItem{{Name: "--schema <schema|name>", Description: "Schema selector for validate/export."}, {Name: "--all", Description: "Export all embedded schemas."}, {Name: "--from <version>", Required: true, Description: "Source schema version for migrate."}, {Name: "--to <version>", Required: true, Description: "Target schema version for migrate."}, {Name: "--dry-run", Description: "Preview export or migration without mutation."}, {Name: "--json", Description: "Emit structured schema command output."}, {Name: "--help", Description: "Show schema help and exit 0."}},
		JSONBehavior: "Schema commands support global --json for structured output and structured errors. schema --help --json emits structured help only.",
	},
	"event": {
		Command:      "kkachi-agent-helper event",
		Status:       capabilityStatusSupported,
		Usage:        "kkachi-agent-helper event append <type> --run <run_id> --payload <json-object> [--json]",
		Summary:      "Append an attributed helper event after validating payload shape and state/event coherence.",
		Subcommands:  []helpItem{{Name: "append <type>", Description: "Append one JSONL event with a compact JSON object payload."}},
		Arguments:    []helpItem{{Name: "<type>", Required: true, Description: "Event type string."}},
		Options:      []helpItem{{Name: "--run <run_id>", Required: true, Description: "Run id for event attribution."}, {Name: "--payload <json-object>", Required: true, Description: "Compact JSON object payload."}, {Name: "--json", Description: "Emit structured append result."}, {Name: "--help", Description: "Show event help and exit 0."}},
		JSONBehavior: "With --json, successful append emits event id, previous id, paths, and timestamp. Errors are structured on stderr.",
	},
	"lock": {
		Command:      "kkachi-agent-helper lock",
		Status:       capabilityStatusSupported,
		Usage:        "kkachi-agent-helper lock recover <active-run|project-write|all> --reason <text> [--run <run_id>] [--json]",
		Summary:      "Explicitly recover stale helper locks without silently removing fresh or malformed locks.",
		Subcommands:  []helpItem{{Name: "recover <active-run|project-write|all>", Description: "Recover one stale lock target and record an event."}},
		Arguments:    []helpItem{{Name: "<active-run|project-write|all>", Required: true, Description: "Lock recovery target."}},
		Options:      []helpItem{{Name: "--reason <text>", Required: true, Description: "Operator reason for recovery."}, {Name: "--run <run_id>", Description: "Run id when recovering active-run locks."}, {Name: "--json", Description: "Emit structured recovery result."}, {Name: "--help", Description: "Show lock help and exit 0."}},
		JSONBehavior: "With --json, successful recovery emits recovered lock metadata. Errors are structured on stderr.",
	},
	"diagnostics": {
		Command:      "kkachi-agent-helper diagnostics",
		Status:       capabilityStatusSupported,
		Usage:        "kkachi-agent-helper diagnostics export [--run <run_id>] [--output <repo-relative-path>] [--json]",
		Summary:      "Export support-safe diagnostics with token-like redaction.",
		Subcommands:  []helpItem{{Name: "export", Description: "Write or print a redacted diagnostics bundle."}},
		Options:      []helpItem{{Name: "--run <run_id>", Description: "Include selected run-local diagnostics."}, {Name: "--output <repo-relative-path>", Description: "Write bundle to a repository-relative path instead of stdout."}, {Name: "--json", Description: "Emit structured export metadata."}, {Name: "--help", Description: "Show diagnostics help and exit 0."}},
		JSONBehavior: "With --json, diagnostics export emits bundle metadata. Without --output, the diagnostics bundle itself is JSON on stdout.",
	},
	"phase-plan": {
		Command:      "kkachi-agent-helper phase-plan",
		Status:       capabilityStatusSupported,
		Usage:        "kkachi-agent-helper phase-plan init <run_id> [--json]\n  kkachi-agent-helper phase-plan show <run_id> [--json]\n  kkachi-agent-helper phase-plan set <run_id> <phase-id> --status <status> [--evidence <path>] [--reason <text>] [--approval-required true|false] [--json]\n  kkachi-agent-helper phase-plan validate <run_id> [--final] [--json]",
		Summary:      "Store, update, show, and deterministically validate KHS-declared phase-plan.yaml state.",
		Subcommands:  []helpItem{{Name: "init <run_id>", Description: "Initialize .kkachi/runs/<run_id>/phase-plan.yaml with required declared phase rows."}, {Name: "show <run_id>", Description: "Show the declared phase plan."}, {Name: "set <run_id> <phase-id>", Description: "Update one declared phase row."}, {Name: "validate <run_id>", Description: "Validate phase-plan structure and optional final completeness."}},
		Options:      []helpItem{{Name: "--status <pending|in_progress|complete|skipped|not_applicable|blocked>", Required: true, Description: "Phase status for phase-plan set."}, {Name: "--evidence <path>", Description: "Evidence link for a completed phase."}, {Name: "--reason <text>", Description: "Required reason for skipped or not-applicable phases."}, {Name: "--approval-required true|false", Description: "Declare whether final validation requires an approved approval record for this phase."}, {Name: "--final", Description: "Require terminal states, evidence, and declared approvals for completed phases."}, {Name: "--json", Description: "Emit structured phase-plan output."}, {Name: "--help", Description: "Show phase-plan help and exit 0."}},
		JSONBehavior: "Phase-plan commands support global --json for structured output and structured errors. Failing validation exits 3 with structured checks.",
		Notes:        []string{"KAH stores and validates declared phase state only; KHS remains responsible for phase applicability and workflow policy."},
	},
	"approval": {
		Command:      "kkachi-agent-helper approval",
		Status:       capabilityStatusSupported,
		Usage:        "kkachi-agent-helper approval request <run_id> --phase <phase-id> --reason <reason> [--evidence <ref>] [--json]\n  kkachi-agent-helper approval record <run_id> --phase <phase-id> --decision <approved|rejected> --by <approver> --evidence <ref> [--reason <reason>] [--json]\n  kkachi-agent-helper approval show <run_id> [--phase <phase-id>] [--json]",
		Summary:      "Record and inspect KHS-declared high-risk phase approval requests and decisions.",
		Subcommands:  []helpItem{{Name: "request <run_id>", Description: "Append an approval.requested event."}, {Name: "record <run_id>", Description: "Append an approval.recorded decision event."}, {Name: "show <run_id>", Description: "Show approval events for a run."}},
		Options:      []helpItem{{Name: "--phase <phase-id>", Required: true, Description: "KHS-declared phase id."}, {Name: "--reason <text>", Description: "Reason for the approval request or decision."}, {Name: "--decision <approved|rejected>", Description: "Approval decision for approval record."}, {Name: "--by <approver>", Description: "Approving principal for approval record."}, {Name: "--evidence <ref>", Description: "Artifact path or message reference; required for approval record."}, {Name: "--json", Description: "Emit structured approval output."}, {Name: "--help", Description: "Show approval help and exit 0."}},
		JSONBehavior: "Approval commands support global --json for structured output and structured errors. KAH records declarations only and does not decide whether approval is required.",
		Notes:        []string{"Use phase-plan set --approval-required true plus phase-plan validate --final to make final validation require an approved decision for a phase."},
	},
	"graph": {
		Command:      "kkachi-agent-helper graph",
		Status:       capabilityStatusSupported,
		Usage:        "kkachi-agent-helper graph init --from-template <template-id-or-path> [--output .kkachi-workflow.yaml] [--json]\n  kkachi-agent-helper graph validate [--file .kkachi-workflow.yaml] [--json]\n  kkachi-agent-helper graph explain [--file .kkachi-workflow.yaml] [--json]\n  kkachi-agent-helper graph diff --from <repo-relative-graph> --to <repo-relative-graph> [--semantic] [--json]\n  kkachi-agent-helper graph propose --patch <repo-relative-candidate-graph> --reason <text> [--json]\n  kkachi-agent-helper graph apply --proposal <proposal-id> --approval <evidence-ref> [--json]\n  kkachi-agent-helper graph export --format mermaid|plantuml [--output <path>] [--json]",
		Summary:      "Initialize, validate, explain, diff, propose, approval-apply, and export project-level .kkachi-workflow.yaml graph state.",
		Subcommands:  []helpItem{{Name: "init", Description: "Create the initial workflow graph from khs-default or an explicit template path."}, {Name: "validate", Description: "Validate workflow graph schema, source authority, and graph references."}, {Name: "explain", Description: "Show the effective graph phases, edges, gates, approvals, and validation summary."}, {Name: "diff", Description: "Compare two valid workflow graph files semantically without writing graph state."}, {Name: "propose", Description: "Record a proposal for a complete candidate workflow graph without applying it."}, {Name: "apply", Description: "Apply an approved proposal after checksum and source-precedence checks."}, {Name: "export", Description: "Render Mermaid or PlantUML generated diagrams without making them graph authority."}},
		Options:      []helpItem{{Name: "--from-template <template-id-or-path>", Description: "Template id khs-default or repository-relative workflow graph YAML template for graph init."}, {Name: "--output .kkachi-workflow.yaml", Description: "Optional graph init output; graph init only supports .kkachi-workflow.yaml."}, {Name: "--file <repo-relative-path>", Description: "Workflow graph file to inspect; defaults to .kkachi-workflow.yaml for validate/explain."}, {Name: "--from <repo-relative-graph>", Description: "Base graph file for graph diff."}, {Name: "--to <repo-relative-graph>", Description: "Candidate graph file for graph diff."}, {Name: "--semantic", Description: "Accepted for graph diff; semantic comparison is always used."}, {Name: "--patch <repo-relative-candidate-graph>", Description: "Complete candidate workflow graph file for graph propose."}, {Name: "--reason <text>", Description: "Required reason recorded in the graph proposal."}, {Name: "--proposal <proposal-id>", Description: "Proposal id returned by graph propose for graph apply."}, {Name: "--approval <evidence-ref>", Description: "Required approval evidence reference for graph apply."}, {Name: "--format mermaid|plantuml", Description: "Required graph export diagram format."}, {Name: "--output <repo-relative-path>", Description: "Optional generated diagram output path for graph export."}, {Name: "--json", Description: "Emit structured graph output."}, {Name: "--help", Description: "Show graph help and exit 0."}},
		JSONBehavior: "Graph init/validate/explain/diff/propose/apply/export support global --json for structured output. Failing graph validation, diff, or export exits 3 while emitting graph result data on stdout.",
		Notes:        []string{"graph init creates only the initial .kkachi-workflow.yaml and fails if one already exists.", "graph propose records .kkachi/graph/proposals evidence and an event; it does not edit .kkachi-workflow.yaml.", "graph apply records required approval evidence by reference but does not decide approval policy.", "graph export writes or prints generated diagrams with authoritative=false; exports are never workflow graph source of truth.", "replacement init and kah alias behavior remain unimplemented.", "KAH validates declared graph state only; KHS remains responsible for workflow policy and templates."},
	},
}

func lookupHelpPage(path []string) (helpOutput, bool) {
	page, ok := helpPages[strings.Join(path, " ")]
	return page, ok
}

func writeProjectInitResult(w io.Writer, result project.InitResult, jsonMode bool) {
	payload := projectInitOutput{
		RootPath:            result.RootPath,
		ProjectID:           result.ProjectID,
		ProjectName:         result.ProjectName,
		CreatedPaths:        result.CreatedPaths,
		SchemaPaths:         result.SchemaPaths,
		InitialEventID:      result.InitialEventID,
		ReconfiguredEventID: result.ReconfiguredEventID,
		Forced:              result.Forced,
	}
	if jsonMode {
		_ = json.NewEncoder(w).Encode(payload)
		return
	}

	if payload.Forced {
		fmt.Fprintf(w, "reconfigured kkachi project: %s\n", payload.RootPath)
	} else {
		fmt.Fprintf(w, "initialized kkachi project: %s\n", payload.RootPath)
	}
	fmt.Fprintf(w, "project_id: %s\n", payload.ProjectID)
	fmt.Fprintf(w, "created:\n")
	for _, path := range payload.CreatedPaths {
		fmt.Fprintf(w, "- %s\n", path)
	}
	for _, path := range payload.SchemaPaths {
		fmt.Fprintf(w, "- %s\n", path)
	}
	if payload.InitialEventID != "" {
		fmt.Fprintf(w, "initial_event_id: %s\n", payload.InitialEventID)
	}
	if payload.ReconfiguredEventID != "" {
		fmt.Fprintf(w, "reconfigured_event_id: %s\n", payload.ReconfiguredEventID)
	}
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

func writeSchemaValidateResult(w io.Writer, result project.SchemaValidateResult, jsonMode bool) {
	payload := schemaValidateOutput{Schema: result.Schema, FilePath: result.FilePath, Status: result.Status, Checks: result.Checks}
	if jsonMode {
		_ = json.NewEncoder(w).Encode(payload)
		return
	}
	fmt.Fprintf(w, "schema validation: %s\n", payload.Status)
	fmt.Fprintf(w, "schema: %s\n", payload.Schema)
	fmt.Fprintf(w, "file_path: %s\n", payload.FilePath)
	for _, check := range payload.Checks {
		fmt.Fprintf(w, "- [%s] %s", check.Status, check.Name)
		if check.Line != 0 {
			fmt.Fprintf(w, " line=%d", check.Line)
		}
		if check.Field != "" {
			fmt.Fprintf(w, " field=%s", check.Field)
		}
		if check.Actual != "" {
			fmt.Fprintf(w, " actual=%s", check.Actual)
		}
		fmt.Fprintf(w, ": %s\n", check.Message)
	}
}

func writeSchemaExportResult(w io.Writer, result project.SchemaExportResult, jsonMode bool) {
	payload := schemaExportOutput{DryRun: result.DryRun, Schemas: result.Schemas, Written: result.Written, Unchanged: result.Unchanged, WouldWrite: result.WouldWrite, EventID: result.EventID}
	if jsonMode {
		_ = json.NewEncoder(w).Encode(payload)
		return
	}
	mode := "exported"
	if payload.DryRun {
		mode = "schema export dry-run"
	}
	fmt.Fprintf(w, "%s: %d schemas\n", mode, len(payload.Schemas))
	if payload.EventID != "" {
		fmt.Fprintf(w, "event_id: %s\n", payload.EventID)
	}
	fmt.Fprintf(w, "written: %d\n", len(payload.Written))
	fmt.Fprintf(w, "unchanged: %d\n", len(payload.Unchanged))
	fmt.Fprintf(w, "would_write: %d\n", len(payload.WouldWrite))
}

func writeSchemaMigrationResult(w io.Writer, result project.SchemaMigrationResult, jsonMode bool) {
	payload := schemaMigrationOutput{DryRun: result.DryRun, FromVersion: result.FromVersion, ToVersion: result.ToVersion, Status: result.Status, Migration: result.Migration, WouldBackup: result.WouldBackup, BackedUp: result.BackedUp, BackupPath: result.BackupPath, WouldMigrate: result.WouldMigrate, Migrated: result.Migrated, Unchanged: result.Unchanged, EventID: result.EventID}
	if jsonMode {
		_ = json.NewEncoder(w).Encode(payload)
		return
	}
	mode := "schema migrated"
	if payload.DryRun {
		mode = "schema migrate dry-run"
	}
	fmt.Fprintf(w, "%s: %s -> %s\n", mode, payload.FromVersion, payload.ToVersion)
	fmt.Fprintf(w, "status: %s\n", payload.Status)
	fmt.Fprintf(w, "migration: %s\n", payload.Migration)
	if payload.BackupPath != "" {
		fmt.Fprintf(w, "backup_path: %s\n", payload.BackupPath)
	}
	if payload.EventID != "" {
		fmt.Fprintf(w, "event_id: %s\n", payload.EventID)
	}
	fmt.Fprintf(w, "would_backup: %d\n", len(payload.WouldBackup))
	fmt.Fprintf(w, "backed_up: %d\n", len(payload.BackedUp))
	fmt.Fprintf(w, "would_migrate: %d\n", len(payload.WouldMigrate))
	fmt.Fprintf(w, "migrated: %d\n", len(payload.Migrated))
	fmt.Fprintf(w, "unchanged: %d\n", len(payload.Unchanged))
}

func writeDiagnosticsExportResult(w io.Writer, result project.DiagnosticsBundle, jsonMode bool) {
	if jsonMode || result.OutputPath == "" {
		_ = json.NewEncoder(w).Encode(result)
		return
	}
	fmt.Fprintf(w, "diagnostics bundle exported: %s\n", result.OutputPath)
	fmt.Fprintf(w, "run_id: %s\n", result.RunID)
	fmt.Fprintf(w, "schema_versions: %d\n", len(result.SchemaVersions))
	fmt.Fprintf(w, "graph_compatibility: %s\n", result.GraphCompatibility.StateStatus)
	fmt.Fprintf(w, "gate_reports: %d\n", len(result.GateReports))
	fmt.Fprintf(w, "selected_artifacts: %d\n", len(result.SelectedArtifacts))
	fmt.Fprintf(w, "approval_records: %d\n", len(result.ApprovalRecords))
}

func writePhasePlanInitResult(w io.Writer, result project.PhasePlanInitResult, jsonMode bool) {
	payload := phasePlanInitOutput{PhasePlan: result.Plan, EventID: result.EventID}
	if jsonMode {
		_ = json.NewEncoder(w).Encode(payload)
		return
	}
	fmt.Fprintf(w, "initialized phase plan for run: %s\n", payload.PhasePlan.RunID)
	fmt.Fprintf(w, "path: %s\n", payload.PhasePlan.Path)
	fmt.Fprintf(w, "event_id: %s\n", payload.EventID)
	fmt.Fprintf(w, "phases: %d\n", len(payload.PhasePlan.Phases))
}

func writePhasePlanShowResult(w io.Writer, plan project.PhasePlan, jsonMode bool) {
	if jsonMode {
		_ = json.NewEncoder(w).Encode(plan)
		return
	}
	fmt.Fprintf(w, "phase plan for run: %s\n", plan.RunID)
	fmt.Fprintf(w, "path: %s\n", plan.Path)
	for _, phase := range plan.Phases {
		fmt.Fprintf(w, "- %s status=%s", phase.ID, phase.Status)
		if phase.Evidence != "" {
			fmt.Fprintf(w, " evidence=%s", phase.Evidence)
		}
		if phase.Reason != "" {
			fmt.Fprintf(w, " reason=%s", phase.Reason)
		}
		if phase.ApprovalRequired {
			fmt.Fprintf(w, " approval_required=true")
		}
		fmt.Fprintln(w)
	}
}

func writePhasePlanSetResult(w io.Writer, result project.PhasePlanSetResult, jsonMode bool) {
	payload := phasePlanSetOutput{PhasePlan: result.Plan, Phase: result.Phase, EventID: result.EventID}
	if jsonMode {
		_ = json.NewEncoder(w).Encode(payload)
		return
	}
	fmt.Fprintf(w, "updated phase plan for run: %s\n", payload.PhasePlan.RunID)
	fmt.Fprintf(w, "path: %s\n", payload.PhasePlan.Path)
	fmt.Fprintf(w, "event_id: %s\n", payload.EventID)
	fmt.Fprintf(w, "phase: %s status=%s\n", payload.Phase.ID, payload.Phase.Status)
	if payload.Phase.ApprovalRequired {
		fmt.Fprintln(w, "approval_required: true")
	}
}

func writeApprovalMutationResult(w io.Writer, result project.ApprovalMutationResult, jsonMode bool) {
	payload := approvalMutationOutput{Record: result.Record, EventID: result.EventID}
	if jsonMode {
		_ = json.NewEncoder(w).Encode(payload)
		return
	}
	fmt.Fprintf(w, "recorded approval event: %s\n", payload.EventID)
	fmt.Fprintf(w, "type: %s\n", payload.Record.Type)
	fmt.Fprintf(w, "phase: %s\n", payload.Record.Phase)
	if payload.Record.Decision != "" {
		fmt.Fprintf(w, "decision: %s\n", payload.Record.Decision)
	}
}

func writeApprovalShowResult(w io.Writer, result project.ApprovalShowResult, jsonMode bool) {
	if jsonMode {
		_ = json.NewEncoder(w).Encode(result)
		return
	}
	fmt.Fprintf(w, "approvals for run: %s\n", result.RunID)
	if result.Phase != "" {
		fmt.Fprintf(w, "phase: %s\n", result.Phase)
	}
	for _, record := range result.Records {
		fmt.Fprintf(w, "- %s %s phase=%s timestamp=%s", record.EventID, record.Type, record.Phase, record.Timestamp)
		if record.Decision != "" {
			fmt.Fprintf(w, " decision=%s", record.Decision)
		}
		if record.Approver != "" {
			fmt.Fprintf(w, " approver=%s", record.Approver)
		}
		if record.Evidence != "" {
			fmt.Fprintf(w, " evidence=%s", record.Evidence)
		}
		fmt.Fprintln(w)
	}
}

func writePhasePlanValidationResult(w io.Writer, result project.PhasePlanValidationResult, jsonMode bool) {
	if jsonMode {
		_ = json.NewEncoder(w).Encode(result)
		return
	}
	fmt.Fprintf(w, "phase-plan validation for run: %s\n", result.RunID)
	fmt.Fprintf(w, "path: %s\n", result.Path)
	fmt.Fprintf(w, "final: %t\n", result.Final)
	fmt.Fprintf(w, "status: %s\n", result.Status)
	for _, check := range result.Checks {
		fmt.Fprintf(w, "- [%s] %s", check.Status, check.Name)
		if check.Field != "" {
			fmt.Fprintf(w, " field=%s", check.Field)
		}
		if check.Actual != "" {
			fmt.Fprintf(w, " actual=%s", check.Actual)
		}
		fmt.Fprintf(w, ": %s\n", check.Message)
	}
}

func writeGraphValidateResult(w io.Writer, result project.GraphValidationResult, jsonMode bool) {
	if jsonMode {
		_ = json.NewEncoder(w).Encode(result)
		return
	}
	fmt.Fprintf(w, "graph validation: %s\n", result.Status)
	fmt.Fprintf(w, "file: %s\n", result.File)
	if result.Checksum != "" {
		fmt.Fprintf(w, "checksum: %s\n", result.Checksum)
	}
	if result.EffectiveSource != "" {
		fmt.Fprintf(w, "effective_source: %s\n", result.EffectiveSource)
	}
	fmt.Fprintf(w, "errors: %d\n", len(result.Errors))
	writeGraphIssues(w, result.Errors)
	fmt.Fprintf(w, "next_action: %s\n", result.NextAction)
}

func writeGraphExplainResult(w io.Writer, result project.GraphExplanationResult, jsonMode bool) {
	if jsonMode {
		_ = json.NewEncoder(w).Encode(result)
		return
	}
	fmt.Fprintf(w, "graph explanation: %s\n", result.Status)
	if result.GraphVersion != "" {
		fmt.Fprintf(w, "graph_version: %s\n", result.GraphVersion)
	}
	if result.EffectiveSource != "" {
		fmt.Fprintf(w, "effective_source: %s\n", result.EffectiveSource)
	}
	fmt.Fprintf(w, "phases: %d\n", len(result.Phases))
	for _, phase := range result.Phases {
		fmt.Fprintf(w, "- phase %s", phase.ID)
		if phase.Title != "" {
			fmt.Fprintf(w, " title=%s", phase.Title)
		}
		if phase.OwnerLayer != "" {
			fmt.Fprintf(w, " owner_layer=%s", phase.OwnerLayer)
		}
		fmt.Fprintf(w, " required=%t\n", phase.Required)
	}
	fmt.Fprintf(w, "edges: %d\n", len(result.Edges))
	for _, edge := range result.Edges {
		fmt.Fprintf(w, "- edge %s -> %s\n", edge.From, edge.To)
	}
	fmt.Fprintf(w, "gates: %d\n", len(result.Gates))
	for _, gate := range result.Gates {
		fmt.Fprintf(w, "- gate %s requires=%s\n", gate.ID, strings.Join(gate.Requires, ","))
	}
	fmt.Fprintf(w, "approval_requirements: %d\n", len(result.ApprovalRequirements))
	for _, approval := range result.ApprovalRequirements {
		fmt.Fprintf(w, "- approval %s role=%s\n", approval.Scope, approval.RequiredRole)
	}
	fmt.Fprintf(w, "pending_proposals: %d\n", len(result.PendingProposals))
	fmt.Fprintf(w, "errors: %d\n", len(result.ValidationSummary.Errors))
	writeGraphIssues(w, result.ValidationSummary.Errors)
	fmt.Fprintf(w, "next_action: %s\n", result.NextAction)
}

func writeGraphInitResult(w io.Writer, result project.GraphInitResult, jsonMode bool) {
	if jsonMode {
		_ = json.NewEncoder(w).Encode(result)
		return
	}
	fmt.Fprintf(w, "graph init: %s\n", result.Status)
	fmt.Fprintf(w, "template_id: %s\n", result.TemplateID)
	fmt.Fprintf(w, "template_source: %s\n", result.TemplateSource)
	fmt.Fprintf(w, "graph_path: %s\n", result.GraphPath)
	fmt.Fprintf(w, "checksum: %s\n", result.Checksum)
	fmt.Fprintf(w, "event_id: %s\n", result.EventID)
	fmt.Fprintf(w, "next_action: %s\n", result.NextAction)
}

func writeGraphDiffResult(w io.Writer, result project.GraphDiffResult, jsonMode bool) {
	if jsonMode {
		_ = json.NewEncoder(w).Encode(result)
		return
	}
	fmt.Fprintf(w, "graph diff: %s\n", result.Status)
	fmt.Fprintf(w, "from: %s\n", result.From.File)
	if result.From.Checksum != "" {
		fmt.Fprintf(w, "from_checksum: %s\n", result.From.Checksum)
	}
	fmt.Fprintf(w, "to: %s\n", result.To.File)
	if result.To.Checksum != "" {
		fmt.Fprintf(w, "to_checksum: %s\n", result.To.Checksum)
	}
	fmt.Fprintf(w, "changed_phases: added=%d removed=%d modified=%d\n", len(result.ChangedPhases.Added), len(result.ChangedPhases.Removed), len(result.ChangedPhases.Modified))
	for _, phase := range result.ChangedPhases.Added {
		fmt.Fprintf(w, "- phase added %s\n", phase.ID)
	}
	for _, phase := range result.ChangedPhases.Removed {
		fmt.Fprintf(w, "- phase removed %s\n", phase.ID)
	}
	for _, change := range result.ChangedPhases.Modified {
		fmt.Fprintf(w, "- phase modified %s\n", change.Key)
	}
	fmt.Fprintf(w, "changed_edges: added=%d removed=%d modified=%d\n", len(result.ChangedEdges.Added), len(result.ChangedEdges.Removed), len(result.ChangedEdges.Modified))
	for _, edge := range result.ChangedEdges.Added {
		fmt.Fprintf(w, "- edge added %s -> %s\n", edge.From, edge.To)
	}
	for _, edge := range result.ChangedEdges.Removed {
		fmt.Fprintf(w, "- edge removed %s -> %s\n", edge.From, edge.To)
	}
	fmt.Fprintf(w, "changed_gates: added=%d removed=%d modified=%d\n", len(result.ChangedGates.Added), len(result.ChangedGates.Removed), len(result.ChangedGates.Modified))
	for _, gate := range result.ChangedGates.Added {
		fmt.Fprintf(w, "- gate added %s\n", gate.ID)
	}
	for _, gate := range result.ChangedGates.Removed {
		fmt.Fprintf(w, "- gate removed %s\n", gate.ID)
	}
	for _, change := range result.ChangedGates.Modified {
		fmt.Fprintf(w, "- gate modified %s\n", change.Key)
	}
	fmt.Fprintf(w, "changed_approvals: added=%d removed=%d modified=%d\n", len(result.ChangedApprovals.Added), len(result.ChangedApprovals.Removed), len(result.ChangedApprovals.Modified))
	for _, approval := range result.ChangedApprovals.Added {
		fmt.Fprintf(w, "- approval added %s\n", approval.Scope)
	}
	for _, approval := range result.ChangedApprovals.Removed {
		fmt.Fprintf(w, "- approval removed %s\n", approval.Scope)
	}
	for _, change := range result.ChangedApprovals.Modified {
		fmt.Fprintf(w, "- approval modified %s\n", change.Key)
	}
	fmt.Fprintf(w, "risk_flags: %s\n", strings.Join(result.RiskFlags, ","))
	fmt.Fprintf(w, "requires_approval: %t\n", result.RequiresApproval)
	fmt.Fprintf(w, "next_action: %s\n", result.NextAction)
}

func writeGraphProposalResult(w io.Writer, result project.GraphProposalResult, jsonMode bool) {
	if jsonMode {
		_ = json.NewEncoder(w).Encode(result)
		return
	}
	fmt.Fprintf(w, "graph proposal: %s\n", result.Status)
	fmt.Fprintf(w, "proposal_id: %s\n", result.ProposalID)
	fmt.Fprintf(w, "proposal_path: %s\n", result.ProposalPath)
	fmt.Fprintf(w, "semantic_diff_ref: %s\n", result.SemanticDiffRef)
	fmt.Fprintf(w, "approval_required: %t\n", result.ApprovalRequired)
	if result.EventID != "" {
		fmt.Fprintf(w, "event_id: %s\n", result.EventID)
	}
	fmt.Fprintf(w, "next_action: %s\n", result.NextAction)
}

func writeGraphApplyResult(w io.Writer, result project.GraphApplyResult, jsonMode bool) {
	if jsonMode {
		_ = json.NewEncoder(w).Encode(result)
		return
	}
	fmt.Fprintf(w, "graph apply: %s\n", result.Status)
	fmt.Fprintf(w, "proposal_id: %s\n", result.ProposalID)
	fmt.Fprintf(w, "approval_ref: %s\n", result.ApprovalRef)
	fmt.Fprintf(w, "graph_path: %s\n", result.GraphPath)
	fmt.Fprintf(w, "new_checksum: %s\n", result.NewChecksum)
	fmt.Fprintf(w, "event_ids: %s\n", strings.Join(result.EventIDs, ","))
	fmt.Fprintf(w, "next_action: %s\n", result.NextAction)
}

func writeGraphExportResult(w io.Writer, result project.GraphExportResult, jsonMode bool) {
	if jsonMode {
		_ = json.NewEncoder(w).Encode(result)
		return
	}
	if result.Status != project.GraphStatusPass {
		fmt.Fprintf(w, "graph export: %s\n", result.Status)
		fmt.Fprintf(w, "source_file: %s\n", result.SourceFile)
		fmt.Fprintf(w, "errors: %d\n", len(result.ValidationSummary.Errors))
		writeGraphIssues(w, result.ValidationSummary.Errors)
		fmt.Fprintf(w, "next_action: %s\n", result.NextAction)
		return
	}
	if result.OutputPath == "" {
		fmt.Fprint(w, result.Diagram)
		return
	}
	fmt.Fprintf(w, "graph export: %s\n", result.Status)
	fmt.Fprintf(w, "format: %s\n", result.Format)
	fmt.Fprintf(w, "output_path: %s\n", result.OutputPath)
	fmt.Fprintf(w, "source_checksum: %s\n", result.SourceChecksum)
	fmt.Fprintf(w, "authoritative: %t\n", result.Authoritative)
	fmt.Fprintf(w, "next_action: %s\n", result.NextAction)
}

func writeGraphIssues(w io.Writer, issues []project.GraphIssue) {
	for _, issue := range issues {
		fmt.Fprintf(w, "- %s", issue.Name)
		if issue.Field != "" {
			fmt.Fprintf(w, " field=%s", issue.Field)
		}
		if issue.Actual != "" {
			fmt.Fprintf(w, " actual=%s", issue.Actual)
		}
		if issue.Line != 0 {
			fmt.Fprintf(w, " line=%d", issue.Line)
		}
		fmt.Fprintf(w, ": %s\n", issue.Message)
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

func writeArtifactMutationResult(w io.Writer, result project.ArtifactMutationResult, jsonMode bool) {
	payload := artifactMutationOutput{RunID: result.RunID, Path: result.Path, ArtifactKind: result.ArtifactKind, Operation: result.Operation, Bytes: result.Bytes, SourcePath: result.SourcePath, Status: result.Status, Reason: result.Reason, EventID: result.EventID}
	if jsonMode {
		_ = json.NewEncoder(w).Encode(payload)
		return
	}
	fmt.Fprintf(w, "mutated artifact for run: %s\n", payload.RunID)
	fmt.Fprintf(w, "operation: %s\n", payload.Operation)
	fmt.Fprintf(w, "path: %s\n", payload.Path)
	fmt.Fprintf(w, "artifact_kind: %s\n", payload.ArtifactKind)
	fmt.Fprintf(w, "bytes: %d\n", payload.Bytes)
	if payload.SourcePath != "" {
		fmt.Fprintf(w, "source_path: %s\n", payload.SourcePath)
	}
	if payload.Status != "" {
		fmt.Fprintf(w, "status: %s\n", payload.Status)
	}
	fmt.Fprintf(w, "event_id: %s\n", payload.EventID)
}

func writeGateCheckResult(w io.Writer, result project.GateCheckResult, jsonMode bool) {
	if jsonMode {
		_ = json.NewEncoder(w).Encode(gateCheckOutput{RunID: result.RunID, Gate: result.Gate, Status: result.Status, Checks: result.Checks, MissingEvidence: result.MissingEvidence, EventID: result.EventID, ReportPath: result.ReportPath})
		return
	}
	fmt.Fprintf(w, "gate check for run: %s\n", result.RunID)
	fmt.Fprintf(w, "gate: %s\n", result.Gate)
	fmt.Fprintf(w, "status: %s\n", result.Status)
	fmt.Fprintf(w, "event_id: %s\n", result.EventID)
	if result.ReportPath != "" {
		fmt.Fprintf(w, "report_path: %s\n", result.ReportPath)
	}
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
	err = redactCLIError(err)
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
	return "Usage: kkachi-agent-helper [--json] <version|capabilities|project|run|artifact|gate|event|schema|lock|diagnostics|phase-plan|approval|graph>"
}

func redactCLIError(err cliError) cliError {
	err.Message = project.RedactString(err.Message)
	err.Hint = project.RedactString(err.Hint)
	err.Path = project.RedactString(err.Path)
	err.Field = project.RedactString(err.Field)
	err.Expected = project.RedactString(err.Expected)
	err.Actual = project.RedactString(err.Actual)
	return err
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
	case "artifact_baseline_encode_failed", "artifact_json_encode_failed", "schema_encode_failed", "graph_proposal_encode_failed":
		return ExitInternal
	case "absolute_path", "empty_path", "path_escape", "repo_root_path", "symlink_escape", "symlink_resolution_failed", "path_inspection_failed", "repo_root_required", "helper_state_exists", "last_event_id_mismatch", "status_invalid_json", "status_last_event_id_invalid", "status_active_run_invalid", "status_project_id_invalid", "project_config_read_failed", "project_config_invalid", "event_log_invalid", "event_log_empty", "event_id_invalid", "event_id_exhausted", "run_metadata_invalid", "run_metadata_invalid_json", "active_run_conflict", "run_transition_invalid", "run_not_found", "run_id_ambiguous", "run_root_read_failed", "run_metadata_read_failed", "run_id_collision", "run_artifact_init_invalid_state", "run_artifact_mutation_invalid_state", "artifact_inspection_failed", "artifact_path_invalid", "artifact_source_missing", "artifact_source_inspection_failed", "artifact_source_invalid", "artifact_source_read_failed", "artifact_read_failed", "artifact_status_invalid", "artifact_reason_required", "artifact_status_unsupported", "artifact_status_not_applicable", "artifact_json_invalid", "status_gate_summary_invalid", "lock_conflict", "lock_stale_recovery_required", "lock_metadata_invalid", "lock_not_found", "lock_identity_mismatch", "lock_release_failed", "schema_validation_read_failed", "schema_reference_invalid", "schema_read_failed", "schema_export_inspection_failed", "schema_export_conflict", "schema_export_read_failed", "schema_migration_path_inspection_failed", "schema_migration_source_version_mismatch", "schema_migration_read_failed", "schema_migration_invalid_json", "schema_migration_invalid_event_log", "schema_migration_version_missing", "schema_migration_backup_failed", "install_manifest_read_failed", "install_manifest_invalid_json", "install_manifest_invalid", "install_manifest_kind_mismatch", "install_source_invalid", "install_source_item_invalid", "install_source_read_failed", "install_checksum_mismatch", "install_duplicate_target", "install_target_inspection_failed", "install_target_read_failed", "install_owner_marker_missing", "install_compatibility_failed", "install_preflight_blocked", "install_apply_failed", "diagnostics_encode_failed", "diagnostics_output_exists", "graph_already_exists", "graph_template_required", "graph_template_unknown", "graph_template_invalid", "graph_output_invalid", "graph_init_invalid", "graph_proposal_invalid", "graph_proposal_inspection_failed", "graph_proposal_id_exhausted", "graph_export_output_invalid", "graph_export_output_inspection_failed", "graph_export_output_exists":
		return ExitSafety
	default:
		return ExitUsage
	}
}
