package project

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const DiagnosticsVersion = "0.1"

var diagnosticArtifactPaths = []string{
	"run-metadata.json",
	"phase-plan.yaml",
	WorkflowInstanceFile,
	"intake-classification.md",
	"selected-cli.json",
	"capability-check.md",
	"bridge-session-snapshot.json",
	"bridge-events.md",
	"test-log.md",
	"verification.md",
	"docs-update.md",
	"final-report.md",
}

type DiagnosticsExportOptions struct {
	RunID  string
	Output string
	Now    func() time.Time
}

type DiagnosticsBundle struct {
	Version            string                              `json:"version"`
	GeneratedAt        string                              `json:"generated_at"`
	RootPath           string                              `json:"root_path"`
	Redaction          DiagnosticsRedaction                `json:"redaction"`
	Project            DiagnosticsProject                  `json:"project"`
	SchemaVersions     []DiagnosticsSchema                 `json:"schema_versions"`
	GraphCompatibility DiagnosticsGraphCompatibility       `json:"graph_compatibility"`
	WorkflowCatalog    WorkflowCatalogResult               `json:"workflow_catalog"`
	RunID              string                              `json:"run_id,omitempty"`
	WorkflowInstance   *WorkflowInstanceCompletenessResult `json:"workflow_instance,omitempty"`
	GateReports        []DiagnosticsFile                   `json:"gate_reports"`
	SelectedArtifacts  []DiagnosticsFile                   `json:"selected_artifacts"`
	ApprovalRecords    []ApprovalRecord                    `json:"approval_records,omitempty"`
	OutputPath         string                              `json:"output_path,omitempty"`
}

type DiagnosticsRedaction struct {
	Enabled     bool   `json:"enabled"`
	Placeholder string `json:"placeholder"`
}

type DiagnosticsProject struct {
	Config DiagnosticsFile `json:"config"`
	Status DiagnosticsFile `json:"status"`
	Events DiagnosticsFile `json:"events"`
}

type DiagnosticsSchema struct {
	Schema  string `json:"schema"`
	Path    string `json:"path"`
	Status  string `json:"status"`
	Version string `json:"version,omitempty"`
	Error   string `json:"error,omitempty"`
}

type DiagnosticsGraphCompatibility struct {
	SupportStatus            string                               `json:"support_status"`
	StateStatus              string                               `json:"state_status"`
	NoDirectYAMLFallback     bool                                 `json:"no_direct_yaml_fallback"`
	ReasonCodes              []string                             `json:"reason_codes"`
	Validation               GraphValidationResult                `json:"validation"`
	FeedbackIntake           DiagnosticsGraphFeedbackIntake       `json:"feedback_intake"`
	ForbiddenFallbackSources []DiagnosticsForbiddenFallbackSource `json:"forbidden_fallback_sources"`
	NextAction               string                               `json:"next_action"`
}

type DiagnosticsGraphFeedbackIntake struct {
	Status          string                       `json:"status"`
	EffectiveBounds *WorkflowGraphFeedbackIntake `json:"effective_bounds,omitempty"`
	ReasonCodes     []string                     `json:"reason_codes"`
	Issues          []GraphIssue                 `json:"issues,omitempty"`
	NextAction      string                       `json:"next_action"`
}

type DiagnosticsForbiddenFallbackSource struct {
	Source     string `json:"source"`
	Status     string `json:"status"`
	Reason     string `json:"reason"`
	ReasonCode string `json:"reason_code"`
}

type DiagnosticsFile struct {
	Path    string `json:"path"`
	Status  string `json:"status"`
	Bytes   int64  `json:"bytes,omitempty"`
	Content any    `json:"content,omitempty"`
	Error   string `json:"error,omitempty"`
}

func ExportDiagnostics(root Root, options DiagnosticsExportOptions) (DiagnosticsBundle, error) {
	if strings.TrimSpace(root.Path) == "" {
		return DiagnosticsBundle{}, problem("repo_root_required", "repository root is required", "Discover the repository root before exporting diagnostics.")
	}
	if options.Now == nil {
		options.Now = func() time.Time { return time.Now().UTC() }
	}

	runID := ""
	if strings.TrimSpace(options.RunID) != "" {
		resolved, err := ResolveRunID(root, options.RunID)
		if err != nil {
			return DiagnosticsBundle{}, err
		}
		runID = resolved
	} else if active := activeRunIDForDiagnostics(root); active != "" {
		runID = active
	}

	bundle := DiagnosticsBundle{
		Version:     DiagnosticsVersion,
		GeneratedAt: options.Now().UTC().Format(time.RFC3339),
		RootPath:    root.Path,
		Redaction:   DiagnosticsRedaction{Enabled: true, Placeholder: RedactedPlaceholder},
		RunID:       runID,
	}
	bundle.Project = DiagnosticsProject{
		Config: diagnosticTextFile(root, ConfigPath, true),
		Status: diagnosticJSONFile(root, StatusPath),
		Events: diagnosticEventsFile(root),
	}
	bundle.SchemaVersions = diagnosticSchemaVersions(root)
	bundle.GraphCompatibility = diagnosticGraphCompatibility(root)
	bundle.WorkflowCatalog = diagnosticWorkflowCatalog(root)
	if runID != "" {
		workflowInstance, err := CheckWorkflowInstanceCompleteness(root, runID)
		if err != nil {
			return DiagnosticsBundle{}, err
		}
		bundle.WorkflowInstance = &workflowInstance
		bundle.GateReports = diagnosticGateReports(root, runID)
		bundle.SelectedArtifacts = diagnosticSelectedArtifacts(root, runID)
		records, err := ApprovalRecords(root, runID)
		if err != nil {
			return DiagnosticsBundle{}, err
		}
		bundle.ApprovalRecords = redactedApprovalRecords(records)
	} else {
		bundle.GateReports = []DiagnosticsFile{}
		bundle.SelectedArtifacts = []DiagnosticsFile{}
	}

	if strings.TrimSpace(options.Output) != "" {
		path, err := ResolveRelativePath(root, options.Output)
		if err != nil {
			return DiagnosticsBundle{}, err
		}
		if _, err := os.Lstat(path.Absolute); err == nil {
			return DiagnosticsBundle{}, &Problem{Code: "diagnostics_output_exists", Message: "diagnostics output already exists", Hint: "Choose a new repository-relative output path so an older support bundle is not overwritten.", Path: path.Relative, Field: "output", Expected: "absent file path", Actual: "exists"}
		} else if !os.IsNotExist(err) {
			return DiagnosticsBundle{}, &Problem{Code: "path_inspection_failed", Message: "cannot inspect diagnostics output path", Hint: "Check output path permissions before exporting diagnostics.", Path: path.Relative, Field: "output", Expected: "inspectable output path", Actual: err.Error()}
		}
		data, err := json.MarshalIndent(bundle, "", "  ")
		if err != nil {
			return DiagnosticsBundle{}, &Problem{Code: "diagnostics_encode_failed", Message: "cannot encode diagnostics bundle", Hint: "Retry diagnostics export and preserve stderr if the problem repeats.", Field: "diagnostics", Expected: "JSON object", Actual: err.Error()}
		}
		data = append(data, '\n')
		if err := writeNewFileAtomically(path, data); err != nil {
			return DiagnosticsBundle{}, err
		}
		bundle.OutputPath = path.Relative
	}
	return bundle, nil
}

func diagnosticWorkflowCatalog(root Root) WorkflowCatalogResult {
	result, err := ValidateWorkflowCatalog(root, WorkflowCatalogOptions{File: WorkflowCatalogDefaultPath})
	if err != nil {
		return WorkflowCatalogResult{
			SchemaVersion: WorkflowCatalogSchemaVersion,
			Status:        WorkflowCatalogStatusFail,
			OK:            false,
			Reason:        "workflow_catalog_invalid_schema",
			ReasonCodes:   []string{"workflow_catalog_invalid_schema"},
			Path:          workflowCatalogPathForDiagnostics(root),
			Diagnostics: []WorkflowCatalogDiagnostic{{
				Code:     "workflow_catalog_invalid_schema",
				Message:  "workflow catalog diagnostics failed",
				Path:     workflowCatalogPathForDiagnostics(root),
				Field:    "workflow_catalog",
				Expected: "diagnostic workflow catalog validation",
				Actual:   err.Error(),
			}},
			NextAction: "Fail closed for task-DAG catalog use until diagnostics can validate the catalog.",
		}
	}
	return result
}

func diagnosticGraphCompatibility(root Root) DiagnosticsGraphCompatibility {
	validation := redactedGraphValidation(ValidateWorkflowGraph(root, GraphOptions{}))
	stateStatus, nextAction := diagnosticGraphCompatibilityState(validation)
	feedback := diagnosticGraphFeedbackIntake(validation)
	return DiagnosticsGraphCompatibility{
		SupportStatus:        "supported",
		StateStatus:          stateStatus,
		NoDirectYAMLFallback: true,
		ReasonCodes:          diagnosticGraphCompatibilityReasonCodes(validation, feedback),
		Validation:           validation,
		FeedbackIntake:       feedback,
		ForbiddenFallbackSources: []DiagnosticsForbiddenFallbackSource{
			{Source: ".kkachi/config.yaml", Status: "forbidden", Reason: "helper config is never workflow graph authority", ReasonCode: GraphReasonForbiddenFallback},
			{Source: ".kkachi/config/workflows/", Status: "forbidden", Reason: "Kkachi v2 workflow runtime config is outside KAH/KHS graph authority", ReasonCode: GraphReasonForbiddenFallback},
			{Source: "generated diagrams", Status: "forbidden", Reason: "Mermaid and PlantUML exports are non-authoritative visualization artifacts", ReasonCode: GraphReasonForbiddenFallback},
			{Source: "KHS defaults", Status: "forbidden", Reason: "defaults are explicit init/proposal inputs only, never silent graph authority", ReasonCode: GraphReasonForbiddenFallback},
			{Source: "stale .kkachi/ runtime state", Status: "forbidden", Reason: "runtime state is evidence/cache only and cannot replace .kkachi-workflow.yaml", ReasonCode: GraphReasonForbiddenFallback},
		},
		NextAction: nextAction,
	}
}

func diagnosticGraphCompatibilityReasonCodes(validation GraphValidationResult, feedback DiagnosticsGraphFeedbackIntake) []string {
	collector := map[string]bool{}
	for _, code := range validation.ReasonCodes {
		collector[code] = true
	}
	for _, code := range feedback.ReasonCodes {
		collector[code] = true
	}
	if len(collector) == 0 {
		return []string{}
	}
	codes := make([]string, 0, len(collector))
	for code := range collector {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	return codes
}

func diagnosticGraphFeedbackIntake(validation GraphValidationResult) DiagnosticsGraphFeedbackIntake {
	if validation.Status == GraphStatusPass && validation.FeedbackIntake != nil {
		return DiagnosticsGraphFeedbackIntake{
			Status:          GraphStatusPass,
			EffectiveBounds: cleanWorkflowGraphFeedbackIntakePtr(validation.FeedbackIntake),
			ReasonCodes:     []string{},
			Issues:          []GraphIssue{},
			NextAction:      "Configurable EXTERNAL_FEEDBACK_INTAKE bounds are valid; KHS may activate only when capabilities also advertise workflow_graph_configurable_feedback_intake.",
		}
	}
	issues := append([]GraphIssue{}, validation.Errors...)
	issues = append(issues, validation.Conflicts...)
	if validation.Status == GraphStatusPass {
		return DiagnosticsGraphFeedbackIntake{
			Status:      "missing",
			ReasonCodes: []string{GraphReasonFeedbackMissing, GraphReasonRepairSupported},
			Issues:      []GraphIssue{},
			NextAction:  "Fail closed for configurable feedback intake activation; add feedback_intake through graph proposal/apply evidence before use.",
		}
	}
	status := GraphStatusFail
	if graphValidationMissing(validation) {
		status = "missing"
	}
	return DiagnosticsGraphFeedbackIntake{
		Status:      status,
		ReasonCodes: graphFeedbackIntakeReasonCodes(validation, issues),
		Issues:      issues,
		NextAction:  "Fail closed for configurable feedback intake activation; repair stale, missing, or invalid bounds through graph proposal/apply evidence.",
	}
}

func graphFeedbackIntakeReasonCodes(validation GraphValidationResult, issues []GraphIssue) []string {
	collector := map[string]bool{}
	add := func(code string) { collector[code] = true }
	if graphValidationMissing(validation) {
		add(GraphReasonFeedbackMissing)
	}
	if graphValidationOnlyFeedbackStaleBounds(validation) {
		add(GraphReasonRepairSupported)
	} else if validation.Status != GraphStatusPass {
		add(GraphReasonRepairUnsupported)
	}
	for _, issue := range issues {
		for _, code := range graphIssueReasonCodes(issue) {
			if code == GraphReasonFeedbackMissing || code == GraphReasonFeedbackStale || code == GraphReasonFeedbackInvalid {
				add(code)
			}
		}
	}
	if len(collector) == 0 {
		return []string{}
	}
	codes := make([]string, 0, len(collector))
	for code := range collector {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	return codes
}

func diagnosticGraphCompatibilityState(validation GraphValidationResult) (string, string) {
	if validation.Status == GraphStatusPass {
		return GraphStatusPass, "Graph support is available and .kkachi-workflow.yaml validates; KHS may use graph explain --json as read-only projection evidence."
	}
	if graphValidationMissing(validation) {
		return "missing", "Fail closed if workflow graph support is required; create .kkachi-workflow.yaml through graph init --from-template or preserve proposal/apply evidence."
	}
	return GraphStatusFail, "Fail closed if workflow graph support is required; repair .kkachi-workflow.yaml through graph propose/apply evidence before use."
}

func graphValidationMissing(validation GraphValidationResult) bool {
	for _, issue := range validation.Errors {
		if issue.Name == graphIssueGraphFile && issue.Actual == graphIssueActualMissing {
			return true
		}
	}
	return false
}

func redactedGraphValidation(validation GraphValidationResult) GraphValidationResult {
	validation.File = RedactString(validation.File)
	validation.EffectiveSource = RedactString(validation.EffectiveSource)
	validation.NextAction = RedactString(validation.NextAction)
	validation.Errors = redactedGraphIssues(validation.Errors)
	validation.Warnings = redactedGraphIssues(validation.Warnings)
	validation.Conflicts = redactedGraphIssues(validation.Conflicts)
	return validation
}

func redactedGraphIssues(issues []GraphIssue) []GraphIssue {
	if len(issues) == 0 {
		return issues
	}
	redacted := make([]GraphIssue, len(issues))
	for i, issue := range issues {
		redacted[i] = GraphIssue{
			Name:     RedactString(issue.Name),
			Path:     RedactString(issue.Path),
			Message:  RedactString(issue.Message),
			Hint:     RedactString(issue.Hint),
			Field:    RedactString(issue.Field),
			Expected: RedactString(issue.Expected),
			Actual:   RedactString(issue.Actual),
			Line:     issue.Line,
		}
	}
	return redacted
}

func redactedApprovalRecords(records []ApprovalRecord) []ApprovalRecord {
	redacted := make([]ApprovalRecord, len(records))
	for i, record := range records {
		redacted[i] = ApprovalRecord{
			EventID:    RedactString(record.EventID),
			OccurredAt: RedactString(record.OccurredAt),
			Type:       RedactString(record.Type),
			RunID:      RedactString(record.RunID),
			Phase:      RedactString(record.Phase),
			Reason:     RedactString(record.Reason),
			Decision:   RedactString(record.Decision),
			Approver:   RedactString(record.Approver),
			Timestamp:  RedactString(record.Timestamp),
			Evidence:   RedactString(record.Evidence),
		}
	}
	return redacted
}

func activeRunIDForDiagnostics(root Root) string {
	path, err := ResolveRelativePath(root, StatusPath)
	if err != nil {
		return ""
	}
	status, err := readStatus(path)
	if err != nil {
		return ""
	}
	value, _ := optionalString(status, "active_run_id")
	if value == nil {
		return ""
	}
	return *value
}

func diagnosticSchemaVersions(root Root) []DiagnosticsSchema {
	results := make([]DiagnosticsSchema, 0, len(canonicalSchemaNames))
	for _, name := range canonicalSchemaNames {
		relative, err := SchemaPathForName(name)
		if err != nil {
			results = append(results, DiagnosticsSchema{Schema: name, Status: "invalid", Error: RedactString(err.Error())})
			continue
		}
		path, err := ResolveRelativePath(root, relative)
		if err != nil {
			results = append(results, DiagnosticsSchema{Schema: name, Path: relative, Status: "invalid", Error: RedactString(err.Error())})
			continue
		}
		data, err := os.ReadFile(path.Absolute)
		if os.IsNotExist(err) {
			results = append(results, DiagnosticsSchema{Schema: name, Path: path.Relative, Status: "missing", Error: "schema file is missing"})
			continue
		}
		if err != nil {
			results = append(results, DiagnosticsSchema{Schema: name, Path: path.Relative, Status: "unreadable", Error: RedactString(err.Error())})
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal(data, &payload); err != nil {
			results = append(results, DiagnosticsSchema{Schema: name, Path: path.Relative, Status: "invalid", Error: RedactString(err.Error())})
			continue
		}
		version, _ := payload["version"].(string)
		if strings.TrimSpace(version) == "" {
			results = append(results, DiagnosticsSchema{Schema: name, Path: path.Relative, Status: "invalid", Error: "schema version is missing"})
			continue
		}
		// Schema files are project-local input; redact defensively even though
		// canonical schema versions are ordinary short semver-like strings.
		results = append(results, DiagnosticsSchema{Schema: name, Path: path.Relative, Status: "present", Version: RedactString(version)})
	}
	return results
}

func diagnosticGateReports(root Root, runID string) []DiagnosticsFile {
	dir, err := ResolveRelativePath(root, filepath.ToSlash(filepath.Join(RunRootPath, runID, "gate-reports")))
	if err != nil {
		return []DiagnosticsFile{{Path: filepath.ToSlash(filepath.Join(RunRootPath, runID, "gate-reports")), Status: "invalid", Error: RedactString(err.Error())}}
	}
	entries, err := os.ReadDir(dir.Absolute)
	if os.IsNotExist(err) {
		return []DiagnosticsFile{}
	}
	if err != nil {
		return []DiagnosticsFile{{Path: dir.Relative, Status: "unreadable", Error: RedactString(err.Error())}}
	}
	names := []string{}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	files := make([]DiagnosticsFile, 0, len(names))
	for _, name := range names {
		files = append(files, diagnosticJSONFile(root, filepath.ToSlash(filepath.Join(RunRootPath, runID, "gate-reports", name))))
	}
	return files
}

func diagnosticSelectedArtifacts(root Root, runID string) []DiagnosticsFile {
	files := make([]DiagnosticsFile, 0, len(diagnosticArtifactPaths))
	for _, artifact := range diagnosticArtifactPaths {
		relative := filepath.ToSlash(filepath.Join(RunRootPath, runID, artifact))
		if strings.HasSuffix(artifact, ".json") {
			files = append(files, diagnosticJSONFile(root, relative))
			continue
		}
		files = append(files, diagnosticTextFile(root, relative, true))
	}
	return files
}

func diagnosticTextFile(root Root, relative string, redact bool) DiagnosticsFile {
	path, err := ResolveRelativePath(root, relative)
	if err != nil {
		return DiagnosticsFile{Path: relative, Status: "invalid", Error: RedactString(err.Error())}
	}
	data, info, err := readRegularDiagnosticFile(path)
	if err != nil {
		return diagnosticFileError(path.Relative, err)
	}
	content := string(data)
	if redact {
		content = RedactString(content)
	}
	return DiagnosticsFile{Path: path.Relative, Status: "present", Bytes: info.Size(), Content: content}
}

func diagnosticJSONFile(root Root, relative string) DiagnosticsFile {
	path, err := ResolveRelativePath(root, relative)
	if err != nil {
		return DiagnosticsFile{Path: relative, Status: "invalid", Error: RedactString(err.Error())}
	}
	data, info, err := readRegularDiagnosticFile(path)
	if err != nil {
		return diagnosticFileError(path.Relative, err)
	}
	var payload any
	if err := json.Unmarshal(data, &payload); err != nil {
		return DiagnosticsFile{Path: path.Relative, Status: "invalid", Bytes: info.Size(), Error: RedactString(err.Error())}
	}
	return DiagnosticsFile{Path: path.Relative, Status: "present", Bytes: info.Size(), Content: RedactValue(payload)}
}

func diagnosticEventsFile(root Root) DiagnosticsFile {
	path, err := ResolveRelativePath(root, EventsPath)
	if err != nil {
		return DiagnosticsFile{Path: EventsPath, Status: "invalid", Error: RedactString(err.Error())}
	}
	data, info, err := readRegularDiagnosticFile(path)
	if err != nil {
		return diagnosticFileError(path.Relative, err)
	}
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	scanner.Buffer(make([]byte, 0, 64*1024), MaxEventLineBytes)
	var events []any
	line := 0
	for scanner.Scan() {
		line++
		text := strings.TrimSpace(scanner.Text())
		if text == "" {
			return DiagnosticsFile{Path: path.Relative, Status: "invalid", Bytes: info.Size(), Error: fmt.Sprintf("blank event line at %d", line)}
		}
		var payload any
		if err := json.Unmarshal([]byte(text), &payload); err != nil {
			return DiagnosticsFile{Path: path.Relative, Status: "invalid", Bytes: info.Size(), Error: RedactString(fmt.Sprintf("line %d: %v", line, err))}
		}
		events = append(events, RedactValue(payload))
	}
	if err := scanner.Err(); err != nil {
		return DiagnosticsFile{Path: path.Relative, Status: "invalid", Bytes: info.Size(), Error: RedactString(err.Error())}
	}
	return DiagnosticsFile{Path: path.Relative, Status: "present", Bytes: info.Size(), Content: events}
}

func readRegularDiagnosticFile(path SafePath) ([]byte, os.FileInfo, error) {
	info, err := os.Lstat(path.Absolute)
	if err != nil {
		return nil, nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, nil, fmt.Errorf("not a regular file")
	}
	data, err := os.ReadFile(path.Absolute)
	return data, info, err
}

func diagnosticFileError(relative string, err error) DiagnosticsFile {
	status := "unreadable"
	if os.IsNotExist(err) {
		status = "missing"
	}
	return DiagnosticsFile{Path: relative, Status: status, Error: RedactString(err.Error())}
}
