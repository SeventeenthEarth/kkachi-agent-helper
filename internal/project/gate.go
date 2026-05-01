package project

import (
	"fmt"
	"strings"
	"time"
)

const (
	GateIntake         = "intake"
	GateSOT            = "sot"
	GateRoadmap        = "roadmap"
	GatePlan           = "plan"
	GateBackend        = "backend"
	GateImplementation = "implementation"
	GateReview         = "review"
	GateVerification   = "verification"
	GateDocs           = "docs"
	GateFinal          = "final"

	GateStatusPass    = "pass"
	GateStatusFail    = "fail"
	GateStatusBlocked = "blocked"
)

var gateDefinitions = []GateDefinition{
	{Name: GateIntake, Implemented: true, Description: "run metadata, intake classification, and path/mode eligibility"},
	{Name: GateSOT, Description: "SOT basis or Path B SOT creation evidence"},
	{Name: GateRoadmap, Description: "roadmap trace or explicit exception evidence"},
	{Name: GatePlan, Description: "acceptance criteria, plan.md, and checklist.md"},
	{Name: GateBackend, Description: "bridge backend evidence artifacts"},
	{Name: GateImplementation, Description: "implementation evidence artifacts"},
	{Name: GateReview, Description: "review and red-team evidence artifacts"},
	{Name: GateVerification, Description: "test-log and verification verdict artifacts"},
	{Name: GateDocs, Description: "docs-update decision artifacts"},
	{Name: GateFinal, Description: "all required gates pass and final-report.md exists"},
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
	definition, ok := gateRegistry[gate]
	if !ok {
		return GateCheckResult{}, unknownGateProblem(gate)
	}
	metadata, metadataPath, err := ReadRunMetadata(root, options.RunID)
	if err != nil {
		return GateCheckResult{}, err
	}
	if err := preflightEventCoherence(root); err != nil {
		return GateCheckResult{}, err
	}

	result, err := checkGateResult(root, metadata.RunID, definition)
	if err != nil {
		return GateCheckResult{}, err
	}

	nextID, err := nextGateEventID(root)
	if err != nil {
		return GateCheckResult{}, err
	}
	result.EventID = nextID
	appendResult, err := appendEventWithStatusMutation(root, AppendEventOptions{Type: gateEventType(result.Status), RunID: metadata.RunID, Payload: gateEventPayload(result), Now: options.Now}, func(status map[string]any, occurredAt string) error {
		metadata.GateState[gate] = gateStateSummary(result.Status, nextID, occurredAt, len(result.Checks), len(result.MissingEvidence))
		if err := writeRunMetadataExisting(metadataPath, metadata); err != nil {
			return err
		}
		return updateStatusGateSummary(status, gate, metadata.RunID, result.Status, nextID, occurredAt)
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

func checkGateResult(root Root, runID string, definition GateDefinition) (GateCheckResult, error) {
	if !definition.Implemented {
		return blockedGateResult(runID, definition), nil
	}
	artifactResult, err := ValidateArtifacts(root, ArtifactValidateOptions{RunID: runID, Gate: ArtifactGateIntake})
	if err != nil {
		return GateCheckResult{}, err
	}
	return gateResultFromArtifactValidation(artifactResult), nil
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
	return map[string]any{"run_id": result.RunID, "gate": result.Gate, "status": result.Status, "checks": len(result.Checks), "failed_checks": failed, "blocked_checks": blocked, "missing_evidence": result.MissingEvidence}
}

func gateStateSummary(status string, eventID string, checkedAt string, checks int, missingEvidence int) map[string]any {
	return map[string]any{"status": status, "event_id": eventID, "checked_at": checkedAt, "checks": checks, "missing_evidence": missingEvidence}
}

func updateStatusGateSummary(status map[string]any, gate string, runID string, gateStatus string, eventID string, checkedAt string) error {
	summary, ok := status["gate_summary"].(map[string]any)
	if !ok {
		if status["gate_summary"] == nil {
			summary = map[string]any{}
		} else {
			return &Problem{Code: "status_gate_summary_invalid", Message: "project status gate_summary must be an object", Hint: "Restore status.json from a coherent backup before checking gates.", Path: StatusPath, Field: "gate_summary", Expected: "object", Actual: fmt.Sprintf("%T", status["gate_summary"])}
		}
	}
	summary[gate] = map[string]any{"run_id": runID, "status": gateStatus, "event_id": eventID, "checked_at": checkedAt}
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
