package project

import (
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	GateIntake           = "intake"
	GateSOT              = "sot"
	GateRoadmap          = "roadmap"
	GatePlan             = "plan"
	GateBackend          = "backend"
	GateImplementation   = "implementation"
	GateReview           = "review"
	GateVerification     = "verification"
	GateDocs             = "docs"
	GateTokenEconomy     = "token-economy"
	GateMultiAgentReview = "multi-agent-review"
	GatePolicyPromotion  = "policy-promotion"
	GateDesignEvidence   = "design-evidence"
	GateFinal            = "final"

	GateStatusPass          = "pass"
	GateStatusFail          = "fail"
	GateStatusBlocked       = "blocked"
	GateStatusNotApplicable = "not_applicable"
)

var gateDefinitions = []GateDefinition{
	{Name: GateIntake, Implemented: true, Description: "run metadata, intake classification, and path/mode eligibility"},
	{Name: GateSOT, Implemented: true, Description: "SOT basis or Path B SOT creation evidence"},
	{Name: GateRoadmap, Implemented: true, Description: "roadmap trace or explicit exception evidence"},
	{Name: GatePlan, Implemented: true, Description: "acceptance criteria, plan.md, and checklist.md"},
	{Name: GateBackend, Implemented: true, Description: "bridge backend evidence artifacts"},
	{Name: GateImplementation, Implemented: true, Description: "implementation evidence artifacts"},
	{Name: GateReview, Implemented: true, Description: "review and red-team evidence artifacts"},
	{Name: GateVerification, Implemented: true, Description: "test-log and verification verdict artifacts"},
	{Name: GateDocs, Implemented: true, Description: "docs-update decision artifacts"},
	{Name: GateTokenEconomy, Implemented: true, Description: "token-001/token-002 token-economy and English-output evidence contract"},
	{Name: GateMultiAgentReview, Implemented: true, Description: "KAS MAR role-first review evidence contract"},
	{Name: GatePolicyPromotion, Implemented: true, Description: "POLPR-007 policy-promotion deterministic evidence shape contract"},
	{Name: GateDesignEvidence, Implemented: true, Description: "DESIGN Teal/UI deterministic evidence gate contract"},
	{Name: GateFinal, Implemented: true, Description: "all required gates pass and final-report.md exists"},
}

var gateRegistry = gateDefinitionMap(gateDefinitions)

type GateDefinition struct {
	Name        string
	Implemented bool
	Description string
}

type GateCheckOptions struct {
	RunID string
	Gate  string
	Now   func() time.Time
}

type GateCheckResult struct {
	RunID           string      `json:"run_id"`
	Gate            string      `json:"gate"`
	Status          string      `json:"status"`
	Checks          []GateCheck `json:"checks"`
	MissingEvidence []string    `json:"missing_evidence"`
	EventID         string      `json:"event_id"`
	ReportPath      string      `json:"report_path,omitempty"`
}

type GateCheck struct {
	Name     string `json:"name"`
	Status   string `json:"status"`
	Path     string `json:"path,omitempty"`
	Message  string `json:"message"`
	Hint     string `json:"hint,omitempty"`
	Field    string `json:"field,omitempty"`
	Expected string `json:"expected,omitempty"`
	Actual   string `json:"actual,omitempty"`
}

func CheckGate(root Root, options GateCheckOptions) (GateCheckResult, error) {
	var result GateCheckResult
	err := withProjectWriteLock(root, "gate check", options.RunID, func() error {
		var err error
		result, err = checkGateUnlocked(root, options)
		return err
	})
	return result, err
}

func checkGateUnlocked(root Root, options GateCheckOptions) (GateCheckResult, error) {
	if options.Now == nil {
		options.Now = func() time.Time { return time.Now().UTC() }
	}
	gate := strings.TrimSpace(options.Gate)
	definition, builtIn := gateRegistry[gate]
	metadata, metadataPath, err := ReadRunMetadata(root, options.RunID)
	if err != nil {
		return GateCheckResult{}, err
	}
	if err := preflightEventCoherence(root); err != nil {
		return GateCheckResult{}, err
	}

	var result GateCheckResult
	if builtIn {
		result, err = checkGateResult(root, metadata, metadataPath.Relative, definition)
	} else {
		result, err = checkWorkflowGate(root, metadata, gate)
	}
	if err != nil {
		return GateCheckResult{}, err
	}

	nextID, err := nextGateEventID(root)
	if err != nil {
		return GateCheckResult{}, err
	}
	result.EventID = nextID
	reportPath, err := gateReportPath(root, result.RunID, result.Gate)
	if err != nil {
		return GateCheckResult{}, err
	}
	result.ReportPath = reportPath.Relative
	appendResult, err := appendEventWithStatusMutation(root, AppendEventOptions{Type: gateEventType(result.Status), RunID: metadata.RunID, Payload: gateEventPayload(result), Now: options.Now}, func(status map[string]any, occurredAt string) error {
		// Write the report before metadata/status so fail-closed state preserves the gate evidence for the appended event.
		if _, err := writeGateReport(root, reportPath, result, occurredAt); err != nil {
			return err
		}
		metadata.GateState[gate] = gateStateSummary(result.Status, nextID, occurredAt, len(result.Checks), len(result.MissingEvidence), reportPath.Relative)
		if err := writeRunMetadataExisting(metadataPath, metadata); err != nil {
			return err
		}
		return updateStatusGateSummary(status, gate, metadata.RunID, result.Status, nextID, occurredAt, reportPath.Relative)
	})
	if err != nil {
		return GateCheckResult{}, err
	}
	result.EventID = appendResult.EventID
	return result, nil
}

func nextGateEventID(root Root) (string, error) {
	eventsPath, err := ResolveRelativePath(root, EventsPath)
	if err != nil {
		return "", err
	}
	last, err := scanLastEventID(eventsPath)
	if err != nil {
		return "", err
	}
	return nextEventID(last, eventsPath.Relative)
}

func checkGateResult(root Root, metadata RunMetadata, metadataRelative string, definition GateDefinition) (GateCheckResult, error) {
	if !definition.Implemented {
		return blockedGateResult(metadata.RunID, definition), nil
	}
	switch definition.Name {
	case GateIntake:
		return checkIntakeGate(root, metadata.RunID)
	case GateSOT:
		return checkSOTGate(root, metadata)
	case GateRoadmap:
		return checkRoadmapGate(root, metadata, metadataRelative)
	case GatePlan:
		return checkPlanGate(root, metadata)
	case GateBackend:
		return checkBackendGate(root, metadata, metadataRelative)
	case GateImplementation:
		return checkImplementationGate(root, metadata)
	case GateReview:
		return checkReviewGate(root, metadata)
	case GateVerification:
		return checkVerificationGate(root, metadata)
	case GateDocs:
		return checkDocsGate(root, metadata)
	case GateTokenEconomy:
		return checkTokenEconomyGate(root, metadata)
	case GateMultiAgentReview:
		return checkMultiAgentReviewGate(root, metadata)
	case GatePolicyPromotion:
		return checkPolicyPromotionGate(root, metadata)
	case GateDesignEvidence:
		return checkDesignEvidenceGate(root, metadata)
	case GateFinal:
		return checkFinalGate(root, metadata, metadataRelative)
	}
	return blockedGateResult(metadata.RunID, definition), nil
}

func checkIntakeGate(root Root, runID string) (GateCheckResult, error) {
	artifactResult, err := ValidateArtifacts(root, ArtifactValidateOptions{RunID: runID, Gate: ArtifactGateIntake})
	if err != nil {
		return GateCheckResult{}, err
	}
	return gateResultFromArtifactValidation(artifactResult), nil
}

func checkSOTGate(root Root, metadata RunMetadata) (GateCheckResult, error) {
	switch metadata.WorkPath {
	case "A_development_execution":
		return gateResultFromChecks(metadata.RunID, GateSOT, []GateCheck{
			checkMarkdownGateArtifact(root, metadata.RunID, markdownGateArtifactRule{Name: "sot_basis", Artifact: "sot-basis.md"}),
		}), nil
	case "B_discovery_shaping":
		return gateResultFromChecks(metadata.RunID, GateSOT, []GateCheck{
			checkMarkdownGateArtifact(root, metadata.RunID, markdownGateArtifactRule{Name: "sot_update", Artifact: "sot-update.md"}),
		}), nil
	default:
		return gateResultFromChecks(metadata.RunID, GateSOT, []GateCheck{{
			Name:     "sot_work_path",
			Status:   GateStatusFail,
			Message:  "work path is not eligible for SOT gate evaluation",
			Hint:     "Repair run-metadata.json to use a supported Kkachi work path.",
			Field:    "work_path",
			Expected: "A_development_execution or B_discovery_shaping",
			Actual:   metadata.WorkPath,
		}}), nil
	}
}

func checkRoadmapGate(root Root, metadata RunMetadata, metadataRelative string) (GateCheckResult, error) {
	if metadata.TaskID != nil && strings.TrimSpace(*metadata.TaskID) != "" {
		return gateResultFromChecks(metadata.RunID, GateRoadmap, []GateCheck{{
			Name:    "roadmap_trace",
			Status:  GateStatusPass,
			Path:    metadataRelative,
			Message: "run metadata records a roadmap task trace",
			Field:   "task_id",
			Actual:  strings.TrimSpace(*metadata.TaskID),
		}}), nil
	}
	return gateResultFromChecks(metadata.RunID, GateRoadmap, []GateCheck{
		checkMarkdownGateArtifact(root, metadata.RunID, markdownGateArtifactRule{Name: "roadmap_trace", Artifact: "roadmap-update.md", AllowNotApplicable: true}),
	}), nil
}

func checkPlanGate(root Root, metadata RunMetadata) (GateCheckResult, error) {
	return gateResultFromChecks(metadata.RunID, GatePlan, []GateCheck{
		checkMarkdownGateArtifact(root, metadata.RunID, markdownGateArtifactRule{Name: "acceptance_criteria", Artifact: "acceptance-criteria.md"}),
		checkMarkdownGateArtifact(root, metadata.RunID, markdownGateArtifactRule{Name: "plan_artifact", Artifact: "plan.md"}),
		checkMarkdownGateArtifact(root, metadata.RunID, markdownGateArtifactRule{Name: "checklist_artifact", Artifact: "checklist.md"}),
	}), nil
}

type markdownGateArtifactRule struct {
	Name               string
	Artifact           string
	AllowNotApplicable bool
}

func checkMarkdownGateArtifact(root Root, runID string, rule markdownGateArtifactRule) GateCheck {
	path, err := artifactPath(root, runID, rule.Artifact)
	if err != nil {
		return GateCheck{Name: rule.Name, Status: GateStatusFail, Message: "gate artifact path is invalid", Hint: "Use artifact init to create canonical artifact paths.", Field: "path", Expected: rule.Artifact, Actual: err.Error()}
	}
	info, err := os.Lstat(path.Absolute)
	if os.IsNotExist(err) {
		return GateCheck{Name: rule.Name, Status: GateStatusFail, Path: path.Relative, Message: "required gate artifact is missing", Hint: "Run artifact init, then record the required gate evidence.", Field: "path", Expected: "existing regular file", Actual: "missing"}
	}
	if err != nil {
		return GateCheck{Name: rule.Name, Status: GateStatusFail, Path: path.Relative, Message: "cannot inspect gate artifact", Hint: "Check run artifact permissions before checking gates.", Field: "path", Expected: "inspectable regular file", Actual: err.Error()}
	}
	if !info.Mode().IsRegular() {
		actual := "non-regular"
		if info.IsDir() {
			actual = "directory"
		}
		return GateCheck{Name: rule.Name, Status: GateStatusFail, Path: path.Relative, Message: "gate artifact must be a regular file", Hint: "Move the conflicting path and re-run artifact init.", Field: "path", Expected: "regular file", Actual: actual}
	}
	if info.Size() == 0 {
		return GateCheck{Name: rule.Name, Status: GateStatusFail, Path: path.Relative, Message: "gate artifact is empty", Hint: "Record the required gate evidence before checking this gate.", Field: "path", Expected: "non-empty file", Actual: "empty"}
	}
	content, err := os.ReadFile(path.Absolute)
	if err != nil {
		return GateCheck{Name: rule.Name, Status: GateStatusFail, Path: path.Relative, Message: "cannot read gate artifact", Hint: "Check run artifact permissions before checking gates.", Field: "path", Expected: "readable file", Actual: err.Error()}
	}
	return validateMarkdownGateArtifact(rule, path.Relative, content)
}

func validateMarkdownGateArtifact(rule markdownGateArtifactRule, relative string, content []byte) GateCheck {
	fields := parseMarkdownFields(string(content))
	status := strings.ToLower(strings.TrimSpace(fields["status"]))
	switch status {
	case "complete":
		return GateCheck{Name: rule.Name, Status: GateStatusPass, Path: relative, Message: "gate artifact is complete"}
	case "not_applicable":
		_, reason := NotApplicableReason(content)
		if rule.AllowNotApplicable && reason != "" {
			return GateCheck{Name: rule.Name, Status: GateStatusPass, Path: relative, Message: "explicit not-applicable reason is recorded", Field: "reason", Actual: reason}
		}
		actual := "not_applicable without reason"
		if !rule.AllowNotApplicable {
			actual = "not_applicable"
		}
		return GateCheck{Name: rule.Name, Status: GateStatusFail, Path: relative, Message: "gate artifact cannot satisfy this gate as not applicable", Hint: "Use Status: complete, or record not_applicable only where the gate explicitly permits it.", Field: "status", Expected: expectedMarkdownGateStatus(rule.AllowNotApplicable), Actual: actual}
	case "pending":
		return GateCheck{Name: rule.Name, Status: GateStatusFail, Path: relative, Message: "gate artifact still has the baseline pending status", Hint: "Replace the baseline with completed gate evidence.", Field: "status", Expected: expectedMarkdownGateStatus(rule.AllowNotApplicable), Actual: status}
	case "":
		return GateCheck{Name: rule.Name, Status: GateStatusFail, Path: relative, Message: "gate artifact status is missing", Hint: "Set Status: complete after recording gate evidence.", Field: "status", Expected: expectedMarkdownGateStatus(rule.AllowNotApplicable), Actual: "missing"}
	default:
		return GateCheck{Name: rule.Name, Status: GateStatusFail, Path: relative, Message: "gate artifact status is invalid", Hint: "Use Status: complete for gate evidence.", Field: "status", Expected: expectedMarkdownGateStatus(rule.AllowNotApplicable), Actual: status}
	}
}

func expectedMarkdownGateStatus(allowNotApplicable bool) string {
	if allowNotApplicable {
		return "complete or not_applicable with Reason"
	}
	return "complete"
}

func gateResultFromChecks(runID string, gate string, checks []GateCheck) GateCheckResult {
	status := GateStatusPass
	missing := []string{}
	for _, check := range checks {
		switch check.Status {
		case GateStatusFail:
			status = GateStatusFail
			if check.Path != "" {
				missing = appendUnique(missing, check.Path)
			}
		case GateStatusBlocked:
			if status != GateStatusFail {
				status = GateStatusBlocked
			}
			if check.Path != "" {
				missing = appendUnique(missing, check.Path)
			}
		}
	}
	return GateCheckResult{RunID: runID, Gate: gate, Status: status, Checks: checks, MissingEvidence: missing}
}

func gateResultFromArtifactValidation(result ArtifactValidateResult) GateCheckResult {
	status := GateStatusPass
	if result.Status == ValidationStatusFail {
		status = GateStatusFail
	}
	checks := make([]GateCheck, 0, len(result.Checks))
	missing := []string{}
	for _, check := range result.Checks {
		gateCheck := GateCheck{Name: check.Name, Status: check.Status, Path: check.Path, Message: check.Message, Hint: check.Hint, Field: check.Field, Expected: check.Expected, Actual: check.Actual}
		checks = append(checks, gateCheck)
		if check.Status == ValidationStatusFail && check.Path != "" {
			missing = appendUnique(missing, check.Path)
		}
	}
	return GateCheckResult{RunID: result.RunID, Gate: result.Gate, Status: status, Checks: checks, MissingEvidence: missing}
}

func blockedGateResult(runID string, definition GateDefinition) GateCheckResult {
	message := fmt.Sprintf("%s gate is declared but not implemented yet", definition.Name)
	check := GateCheck{Name: definition.Name + "_implemented", Status: GateStatusBlocked, Message: message, Hint: "This gate's artifact-specific rules are scheduled for a later gates roadmap task.", Field: "gate", Expected: "implemented gate evaluator", Actual: "blocked"}
	return GateCheckResult{RunID: runID, Gate: definition.Name, Status: GateStatusBlocked, Checks: []GateCheck{check}, MissingEvidence: []string{definition.Description}}
}

func gateEventType(status string) string {
	switch status {
	case GateStatusPass:
		return "gate.passed"
	case GateStatusFail:
		return "gate.failed"
	case GateStatusNotApplicable:
		return "gate.checked"
	default:
		return "gate.checked"
	}
}

func gateEventPayload(result GateCheckResult) map[string]any {
	failed := 0
	blocked := 0
	for _, check := range result.Checks {
		switch check.Status {
		case GateStatusFail:
			failed++
		case GateStatusBlocked:
			blocked++
		}
	}
	payload := map[string]any{"run_id": result.RunID, "gate": result.Gate, "status": result.Status, "checks": len(result.Checks), "failed_checks": failed, "blocked_checks": blocked, "missing_evidence": result.MissingEvidence}
	if result.ReportPath != "" {
		payload["report_path"] = result.ReportPath
	}
	return payload
}

func gateStateSummary(status string, eventID string, checkedAt string, checks int, missingEvidence int, reportPath string) map[string]any {
	return map[string]any{"status": status, "event_id": eventID, "checked_at": checkedAt, "checks": checks, "missing_evidence": missingEvidence, "report_path": reportPath}
}

func updateStatusGateSummary(status map[string]any, gate string, runID string, gateStatus string, eventID string, checkedAt string, reportPath string) error {
	summary, ok := status["gate_summary"].(map[string]any)
	if !ok {
		if status["gate_summary"] == nil {
			summary = map[string]any{}
		} else {
			return &Problem{Code: "status_gate_summary_invalid", Message: "project status gate_summary must be an object", Hint: "Restore status.json from a coherent backup before checking gates.", Path: StatusPath, Field: "gate_summary", Expected: "object", Actual: fmt.Sprintf("%T", status["gate_summary"])}
		}
	}
	summary[gate] = map[string]any{"run_id": runID, "status": gateStatus, "event_id": eventID, "checked_at": checkedAt, "report_path": reportPath}
	status["gate_summary"] = summary
	return nil
}

func KnownGates() []string {
	gates := make([]string, 0, len(gateDefinitions))
	for _, definition := range gateDefinitions {
		gates = append(gates, definition.Name)
	}
	return gates
}

func gateDefinitionMap(definitions []GateDefinition) map[string]GateDefinition {
	registry := make(map[string]GateDefinition, len(definitions))
	for _, definition := range definitions {
		registry[definition.Name] = definition
	}
	return registry
}

func unknownGateProblem(gate string) *Problem {
	return &Problem{Code: "gate_unknown", Message: "gate is not defined", Hint: "Use one of: " + strings.Join(KnownGates(), ", ") + ".", Field: "gate", Expected: strings.Join(KnownGates(), ","), Actual: gate}
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}
