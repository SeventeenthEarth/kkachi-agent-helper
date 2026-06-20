package project

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	PhasePlanVersion = "0.1"
	PhasePlanPath    = "phase-plan.yaml"

	PhaseStatusPending       = "pending"
	PhaseStatusInProgress    = "in_progress"
	PhaseStatusComplete      = "complete"
	PhaseStatusSkipped       = "skipped"
	PhaseStatusNotApplicable = "not_applicable"
	PhaseStatusBlocked       = "blocked"

	PhasePlanStatusPass = "pass"
	PhasePlanStatusFail = "fail"
)

var (
	defaultPhaseIDs = []string{
		"intake",
		"sot",
		"roadmap",
		"task-classification",
		"plan",
		"vet",
		"ask",
		"implement",
		"enhance-test",
		"ai-slop-cleaner",
		"optimize",
		"docs",
		"verify",
		"review",
		"request-feedback-1",
		"handle-feedback-1",
		"mar-review",
		"second-color-review",
		"final",
	}
	phaseStatuses = []string{
		PhaseStatusPending,
		PhaseStatusInProgress,
		PhaseStatusComplete,
		PhaseStatusSkipped,
		PhaseStatusNotApplicable,
		PhaseStatusBlocked,
	}
	feedbackPhasePattern = regexp.MustCompile(`^(request-feedback|handle-feedback)-([0-9]+)$`)
)

type PhasePlan struct {
	Version string     `json:"version"`
	RunID   string     `json:"run_id"`
	Phases  []PhaseRow `json:"phases"`
	Path    string     `json:"path,omitempty"`
}

type PhaseRow struct {
	ID               string `json:"id"`
	Status           string `json:"status"`
	Evidence         string `json:"evidence,omitempty"`
	Reason           string `json:"reason,omitempty"`
	ApprovalRequired bool   `json:"approval_required,omitempty"`
}

type PhasePlanInitOptions struct {
	RunID string
	Now   func() time.Time
}

type PhasePlanInitResult struct {
	Plan    PhasePlan `json:"phase_plan"`
	EventID string    `json:"event_id"`
}

type PhasePlanSetOptions struct {
	RunID               string
	PhaseID             string
	Status              string
	Evidence            string
	Reason              string
	ApprovalRequired    bool
	ApprovalRequiredSet bool
	Now                 func() time.Time
}

type PhasePlanSetResult struct {
	Plan    PhasePlan `json:"phase_plan"`
	Phase   PhaseRow  `json:"phase"`
	EventID string    `json:"event_id"`
}

type PhasePlanValidationOptions struct {
	RunID string
	Final bool
}

type PhasePlanValidationResult struct {
	RunID  string           `json:"run_id"`
	Path   string           `json:"path"`
	Final  bool             `json:"final"`
	Status string           `json:"status"`
	Checks []PhasePlanCheck `json:"checks"`
	Plan   PhasePlan        `json:"phase_plan"`
}

type PhasePlanCheck struct {
	Name     string `json:"name"`
	Status   string `json:"status"`
	Path     string `json:"path,omitempty"`
	Message  string `json:"message"`
	Hint     string `json:"hint,omitempty"`
	Field    string `json:"field,omitempty"`
	Expected string `json:"expected,omitempty"`
	Actual   string `json:"actual,omitempty"`
}

type phasePlanFeedbackPolicy struct {
	requiredRounds map[int]bool
	allowedRounds  map[int]bool
	sourceCheck    *PhasePlanCheck
}

func InitPhasePlan(root Root, options PhasePlanInitOptions) (PhasePlanInitResult, error) {
	var result PhasePlanInitResult
	err := withProjectWriteLock(root, "phase-plan init", options.RunID, func() error {
		var err error
		result, err = initPhasePlanUnlocked(root, options)
		return err
	})
	return result, err
}

func initPhasePlanUnlocked(root Root, options PhasePlanInitOptions) (PhasePlanInitResult, error) {
	if options.Now == nil {
		options.Now = func() time.Time { return time.Now().UTC() }
	}
	metadata, _, err := ReadRunMetadata(root, options.RunID)
	if err != nil {
		return PhasePlanInitResult{}, err
	}
	if err := preflightEventCoherence(root); err != nil {
		return PhasePlanInitResult{}, err
	}
	path, err := phasePlanPath(root, metadata.RunID)
	if err != nil {
		return PhasePlanInitResult{}, err
	}
	if _, err := os.Lstat(path.Absolute); err == nil {
		return PhasePlanInitResult{}, &Problem{Code: "phase_plan_exists", Message: "phase plan already exists", Hint: "Use phase-plan show, phase-plan validate, or phase-plan set to inspect or update existing phase state.", Path: path.Relative, Field: "path", Expected: "absent phase-plan.yaml", Actual: "exists"}
	} else if !os.IsNotExist(err) {
		return PhasePlanInitResult{}, &Problem{Code: "path_inspection_failed", Message: "cannot inspect phase plan path", Hint: "Check run directory permissions before initializing the phase plan.", Path: path.Relative, Field: "path", Expected: "inspectable path", Actual: err.Error()}
	}
	plan := defaultPhasePlan(metadata.RunID, path.Relative, phasePlanRequiredPhaseIDs(root))
	content := encodePhasePlan(plan)
	appendResult, err := appendEventWithStatusMutation(root, AppendEventOptions{Type: "phase_plan.initialized", RunID: metadata.RunID, Payload: map[string]any{"path": path.Relative, "phase_count": len(plan.Phases)}, Now: options.Now}, func(map[string]any, string) error {
		return writeNewFileAtomically(path, content)
	})
	if err != nil {
		return PhasePlanInitResult{}, err
	}
	return PhasePlanInitResult{Plan: plan, EventID: appendResult.EventID}, nil
}

func ShowPhasePlan(root Root, runID string) (PhasePlan, error) {
	metadata, _, err := ReadRunMetadata(root, runID)
	if err != nil {
		return PhasePlan{}, err
	}
	return readPhasePlan(root, metadata.RunID)
}

func SetPhasePlanPhase(root Root, options PhasePlanSetOptions) (PhasePlanSetResult, error) {
	var result PhasePlanSetResult
	err := withProjectWriteLock(root, "phase-plan set", options.RunID, func() error {
		var err error
		result, err = setPhasePlanPhaseUnlocked(root, options)
		return err
	})
	return result, err
}

func setPhasePlanPhaseUnlocked(root Root, options PhasePlanSetOptions) (PhasePlanSetResult, error) {
	if options.Now == nil {
		options.Now = func() time.Time { return time.Now().UTC() }
	}
	metadata, _, err := ReadRunMetadata(root, options.RunID)
	if err != nil {
		return PhasePlanSetResult{}, err
	}
	if err := preflightEventCoherence(root); err != nil {
		return PhasePlanSetResult{}, err
	}
	plan, err := readPhasePlan(root, metadata.RunID)
	if err != nil {
		return PhasePlanSetResult{}, err
	}
	phaseID := strings.TrimSpace(options.PhaseID)
	if phaseID == "" {
		return PhasePlanSetResult{}, &Problem{Code: "phase_id_required", Message: "phase id is required", Hint: "Pass a phase id from phase-plan show.", Field: "phase_id", Expected: "non-empty phase id", Actual: "empty"}
	}
	status := strings.TrimSpace(options.Status)
	if !knownPhaseStatus(status) {
		return PhasePlanSetResult{}, &Problem{Code: "phase_status_invalid", Message: "phase status is not supported", Hint: "Use pending, in_progress, complete, skipped, not_applicable, or blocked.", Field: "status", Expected: strings.Join(phaseStatuses, ","), Actual: status}
	}
	reason := strings.TrimSpace(options.Reason)
	if (status == PhaseStatusSkipped || status == PhaseStatusNotApplicable) && reason == "" {
		return PhasePlanSetResult{}, &Problem{Code: "phase_reason_required", Message: "skipped or not-applicable phase requires a reason", Hint: "Record KHS's explicit reason; KAH does not infer phase applicability.", Field: "reason", Expected: "non-empty reason", Actual: "missing"}
	}
	index := -1
	for i, phase := range plan.Phases {
		if phase.ID == phaseID {
			index = i
			break
		}
	}
	if index == -1 {
		return PhasePlanSetResult{}, &Problem{Code: "phase_id_unknown", Message: "phase id is not declared in phase plan", Hint: "KAH stores declared phase rows only; initialize or rewrite the phase plan with the required row before updating it.", Path: plan.Path, Field: "phase_id", Expected: "declared phase id", Actual: phaseID}
	}
	updated := PhaseRow{ID: phaseID, Status: status, Evidence: strings.TrimSpace(options.Evidence), Reason: reason, ApprovalRequired: plan.Phases[index].ApprovalRequired}
	if options.ApprovalRequiredSet {
		updated.ApprovalRequired = options.ApprovalRequired
	}
	plan.Phases[index] = updated
	if _, err := validatePhasePlanShape(plan); err != nil {
		return PhasePlanSetResult{}, err
	}
	content := encodePhasePlan(plan)
	path, err := phasePlanPath(root, metadata.RunID)
	if err != nil {
		return PhasePlanSetResult{}, err
	}
	payload := map[string]any{"path": path.Relative, "phase_id": phaseID, "status": status}
	if options.ApprovalRequiredSet {
		payload["approval_required"] = options.ApprovalRequired
	}
	appendResult, err := appendEventWithStatusMutation(root, AppendEventOptions{Type: "phase_plan.updated", RunID: metadata.RunID, Payload: payload, Now: options.Now}, func(map[string]any, string) error {
		return writeExistingFileAtomically(path, content)
	})
	if err != nil {
		return PhasePlanSetResult{}, err
	}
	return PhasePlanSetResult{Plan: plan, Phase: updated, EventID: appendResult.EventID}, nil
}

func ValidatePhasePlan(root Root, options PhasePlanValidationOptions) (PhasePlanValidationResult, error) {
	metadata, _, err := ReadRunMetadata(root, options.RunID)
	if err != nil {
		return PhasePlanValidationResult{}, err
	}
	plan, err := readPhasePlan(root, metadata.RunID)
	if err != nil {
		return PhasePlanValidationResult{}, err
	}
	policy := resolvePhasePlanFeedbackPolicy(root, plan.Path)
	requiredPhaseIDs := phasePlanRequiredPhaseIDs(root)
	checks := validatePhasePlanChecksWithApprovals(root, metadata, plan, options.Final, &policy, requiredPhaseIDs)
	status := PhasePlanStatusPass
	for _, check := range checks {
		if check.Status == PhasePlanStatusFail {
			status = PhasePlanStatusFail
			break
		}
	}
	return PhasePlanValidationResult{RunID: metadata.RunID, Path: plan.Path, Final: options.Final, Status: status, Checks: checks, Plan: plan}, nil
}

func defaultPhasePlan(runID string, path string, requiredPhaseIDs []string) PhasePlan {
	phases := make([]PhaseRow, 0, len(requiredPhaseIDs))
	for _, id := range requiredPhaseIDs {
		phases = append(phases, PhaseRow{ID: id, Status: PhaseStatusPending})
	}
	return PhasePlan{Version: PhasePlanVersion, RunID: runID, Path: path, Phases: phases}
}

func phasePlanRequiredPhaseIDs(root Root) []string {
	ids := append([]string{}, defaultPhaseIDs...)
	seen := map[string]bool{}
	for _, id := range ids {
		seen[id] = true
	}
	loaded := loadWorkflowGraph(root, GraphOptions{File: WorkflowGraphDefaultPath})
	if loaded.validation.Status != GraphStatusPass {
		return ids
	}
	for _, phase := range loaded.graph.Phases {
		id := strings.TrimSpace(phase.ID)
		if id == "" || !phase.Required || seen[id] {
			continue
		}
		ids = append(ids, id)
		seen[id] = true
	}
	return ids
}

func phasePlanPath(root Root, runID string) (SafePath, error) {
	if !runIDPattern.MatchString(runID) {
		return SafePath{}, invalidRunField("", "run_id", "run-YYYYMMDDTHHMMSSZ-<12hex>", runID)
	}
	return ResolveRelativePath(root, filepath.ToSlash(filepath.Join(RunRootPath, runID, PhasePlanPath)))
}

func readPhasePlan(root Root, runID string) (PhasePlan, error) {
	path, err := phasePlanPath(root, runID)
	if err != nil {
		return PhasePlan{}, err
	}
	data, err := os.ReadFile(path.Absolute)
	if os.IsNotExist(err) {
		return PhasePlan{}, &Problem{Code: "phase_plan_missing", Message: "phase plan is missing", Hint: "Run phase-plan init for this run or write a valid phase-plan.yaml before validation.", Path: path.Relative, Field: "path", Expected: "existing phase-plan.yaml", Actual: "missing"}
	}
	if err != nil {
		return PhasePlan{}, &Problem{Code: "phase_plan_read_failed", Message: "cannot read phase plan", Hint: "Check run directory permissions before reading the phase plan.", Path: path.Relative, Field: "path", Expected: "readable phase-plan.yaml", Actual: err.Error()}
	}
	plan, err := parsePhasePlan(data, path.Relative)
	if err != nil {
		return PhasePlan{}, err
	}
	if plan.RunID != runID {
		return PhasePlan{}, &Problem{Code: "phase_plan_run_id_mismatch", Message: "phase plan run id does not match its run directory", Hint: "Keep phase-plan.yaml scoped to its helper run directory.", Path: path.Relative, Field: "run_id", Expected: runID, Actual: plan.RunID}
	}
	return plan, nil
}

func validatePhasePlanShape(plan PhasePlan) (PhasePlanValidationResult, error) {
	checks := validatePhasePlanChecks(plan, false, nil, defaultPhaseIDs)
	for _, check := range checks {
		if check.Status == PhasePlanStatusFail && (check.Name == "version" || check.Name == "run_id" || check.Name == "phase_id" || check.Name == "phase_status" || check.Name == "duplicate_phase") {
			return PhasePlanValidationResult{}, &Problem{Code: "phase_plan_invalid", Message: check.Message, Hint: check.Hint, Path: check.Path, Field: check.Field, Expected: check.Expected, Actual: check.Actual}
		}
	}
	return PhasePlanValidationResult{RunID: plan.RunID, Path: plan.Path, Status: PhasePlanStatusPass, Checks: checks, Plan: plan}, nil
}

func validatePhasePlanChecks(plan PhasePlan, final bool, feedbackPolicy *phasePlanFeedbackPolicy, requiredPhaseIDs []string) []PhasePlanCheck {
	checks := []PhasePlanCheck{}
	add := func(check PhasePlanCheck) {
		if check.Status == "" {
			check.Status = PhasePlanStatusPass
		}
		if check.Path == "" {
			check.Path = plan.Path
		}
		checks = append(checks, check)
	}
	if plan.Version == PhasePlanVersion {
		add(PhasePlanCheck{Name: "version", Message: "phase plan version is supported", Field: "version", Actual: plan.Version})
	} else {
		actual := plan.Version
		if actual == "" {
			actual = "missing"
		}
		add(PhasePlanCheck{Name: "version", Status: PhasePlanStatusFail, Message: "phase plan version is unsupported", Hint: "Use the current phase-plan.yaml format emitted by phase-plan init.", Field: "version", Expected: PhasePlanVersion, Actual: actual})
	}
	if runIDPattern.MatchString(plan.RunID) {
		add(PhasePlanCheck{Name: "run_id", Message: "phase plan run id is valid", Field: "run_id", Actual: plan.RunID})
	} else {
		actual := plan.RunID
		if actual == "" {
			actual = "missing"
		}
		add(PhasePlanCheck{Name: "run_id", Status: PhasePlanStatusFail, Message: "phase plan run id is invalid", Hint: "Keep the helper-generated run id in phase-plan.yaml.", Field: "run_id", Expected: "run-YYYYMMDDTHHMMSSZ-<12hex>", Actual: actual})
	}

	seen := map[string]PhaseRow{}
	duplicates := []string{}
	for _, phase := range plan.Phases {
		if strings.TrimSpace(phase.ID) == "" {
			add(PhasePlanCheck{Name: "phase_id", Status: PhasePlanStatusFail, Message: "phase id is missing", Hint: "Every declared phase row must include id.", Field: "phases[].id", Expected: "non-empty phase id", Actual: "missing"})
			continue
		}
		if _, ok := seen[phase.ID]; ok {
			duplicates = append(duplicates, phase.ID)
		}
		seen[phase.ID] = phase
		if !knownPhaseStatus(phase.Status) {
			actual := phase.Status
			if actual == "" {
				actual = "missing"
			}
			add(PhasePlanCheck{Name: "phase_status", Status: PhasePlanStatusFail, Message: "phase status is invalid", Hint: "Use a supported deterministic phase status.", Field: phase.ID + ".status", Expected: strings.Join(phaseStatuses, ","), Actual: actual})
		}
		if (phase.Status == PhaseStatusSkipped || phase.Status == PhaseStatusNotApplicable) && strings.TrimSpace(phase.Reason) == "" {
			add(PhasePlanCheck{Name: "skip_reason", Status: PhasePlanStatusFail, Message: "skipped or not-applicable phase requires a reason", Hint: "Record KHS's explicit reason; KAH does not infer applicability.", Field: phase.ID + ".reason", Expected: "non-empty reason", Actual: "missing"})
		}
	}
	if len(duplicates) == 0 {
		add(PhasePlanCheck{Name: "duplicate_phase", Message: "phase ids are unique"})
	} else {
		sort.Strings(duplicates)
		add(PhasePlanCheck{Name: "duplicate_phase", Status: PhasePlanStatusFail, Message: "phase ids must be unique", Hint: "Remove duplicate declared phase rows.", Field: "phases[].id", Expected: "unique phase ids", Actual: strings.Join(duplicates, ",")})
	}

	missing := []string{}
	for _, id := range requiredPhaseIDs {
		if _, ok := seen[id]; !ok {
			missing = append(missing, id)
		}
	}
	if len(missing) == 0 {
		add(PhasePlanCheck{Name: "required_phases", Message: "required phase rows are present"})
	} else {
		add(PhasePlanCheck{Name: "required_phases", Status: PhasePlanStatusFail, Message: "required phase rows are missing", Hint: "Initialize or update phase-plan.yaml with all KHS-declared required rows.", Field: "phases[].id", Expected: strings.Join(requiredPhaseIDs, ","), Actual: strings.Join(missing, ",")})
	}

	feedbackChecks := validateFeedbackPhases(seen, plan.Path, feedbackPolicy)
	checks = append(checks, feedbackChecks...)
	if final {
		checks = append(checks, validateFinalPhaseState(plan, seen, requiredPhaseIDs)...)
	}
	return checks
}

func validatePhasePlanChecksWithApprovals(root Root, metadata RunMetadata, plan PhasePlan, final bool, feedbackPolicy *phasePlanFeedbackPolicy, requiredPhaseIDs []string) []PhasePlanCheck {
	checks := validatePhasePlanChecks(plan, final, feedbackPolicy, requiredPhaseIDs)
	checks = append(checks, validateWorkflowPhaseProjection(root, metadata, plan)...)
	if final {
		checks = append(checks, validateFinalApprovalState(root, plan)...)
	}
	return checks
}

func validateFeedbackPhases(seen map[string]PhaseRow, path string, policy *phasePlanFeedbackPolicy) []PhasePlanCheck {
	checks := []PhasePlanCheck{}
	requests := map[int]bool{}
	handles := map[int]bool{}
	invalid := []string{}
	hasFeedbackPhases := false
	for id := range seen {
		matches := feedbackPhasePattern.FindStringSubmatch(id)
		if matches == nil {
			continue
		}
		hasFeedbackPhases = true
		round, err := strconv.Atoi(matches[2])
		if err != nil || !feedbackRoundAllowed(round, policy) {
			invalid = append(invalid, id)
			continue
		}
		if matches[1] == "request-feedback" {
			requests[round] = true
		} else {
			handles[round] = true
		}
	}
	if hasFeedbackPhases && policy != nil && policy.sourceCheck != nil {
		checks = append(checks, *policy.sourceCheck)
	}
	if len(invalid) > 0 {
		sort.Strings(invalid)
		checks = append(checks, PhasePlanCheck{Name: "feedback_round_range", Status: PhasePlanStatusFail, Path: path, Message: "feedback phase round is outside the graph-declared supported range", Hint: "Use feedback rounds declared by .kkachi-workflow.yaml feedback_intake policy.", Field: "phases[].id", Expected: feedbackRoundExpected(policy), Actual: strings.Join(invalid, ",")})
	} else {
		checks = append(checks, PhasePlanCheck{Name: "feedback_round_range", Status: PhasePlanStatusPass, Path: path, Message: "feedback phase rounds are within the graph-declared supported range"})
	}
	if policy != nil && policy.sourceCheck == nil {
		missingRequired := []string{}
		for round := range policy.requiredRounds {
			if !requests[round] {
				missingRequired = append(missingRequired, fmt.Sprintf("request-feedback-%d", round))
			}
			if !handles[round] {
				missingRequired = append(missingRequired, fmt.Sprintf("handle-feedback-%d", round))
			}
		}
		if len(missingRequired) > 0 {
			sort.Strings(missingRequired)
			checks = append(checks, PhasePlanCheck{Name: "feedback_required_rounds", Status: PhasePlanStatusFail, Path: path, Message: "required feedback rounds are missing", Hint: "Declare the graph-required feedback request and handling rows in phase-plan.yaml.", Field: "phases[].id", Expected: "request-feedback-1 and handle-feedback-1", Actual: strings.Join(missingRequired, ",")})
		} else {
			checks = append(checks, PhasePlanCheck{Name: "feedback_required_rounds", Status: PhasePlanStatusPass, Path: path, Message: "required feedback rounds are declared"})
		}
	}
	unpaired := []string{}
	for round := range requests {
		if !handles[round] {
			unpaired = append(unpaired, fmt.Sprintf("handle-feedback-%d", round))
		}
	}
	for round := range handles {
		if !requests[round] {
			unpaired = append(unpaired, fmt.Sprintf("request-feedback-%d", round))
		}
	}
	if len(unpaired) > 0 {
		sort.Strings(unpaired)
		checks = append(checks, PhasePlanCheck{Name: "feedback_pairs", Status: PhasePlanStatusFail, Path: path, Message: "feedback request and handling phases must be paired", Hint: "Declare both request-feedback-N and handle-feedback-N for each feedback round.", Field: "phases[].id", Expected: "paired feedback phases", Actual: strings.Join(unpaired, ",")})
	} else {
		checks = append(checks, PhasePlanCheck{Name: "feedback_pairs", Status: PhasePlanStatusPass, Path: path, Message: "feedback request and handling phases are paired"})
	}
	return checks
}

func resolvePhasePlanFeedbackPolicy(root Root, path string) phasePlanFeedbackPolicy {
	loaded := loadWorkflowGraph(root, GraphOptions{File: WorkflowGraphDefaultPath})
	if loaded.validation.Status != GraphStatusPass {
		return phasePlanFeedbackPolicy{
			sourceCheck: &PhasePlanCheck{
				Name:     "feedback_policy_source",
				Status:   PhasePlanStatusFail,
				Path:     path,
				Message:  "workflow graph feedback policy cannot be used",
				Hint:     "Create or repair .kkachi-workflow.yaml with a valid feedback_intake policy before validating feedback phases.",
				Field:    WorkflowGraphDefaultPath,
				Expected: "valid workflow graph with feedback_intake",
				Actual:   graphValidationIssueSummary(loaded.validation.Errors),
			},
		}
	}
	feedback := loaded.validation.FeedbackIntake
	if feedback == nil {
		return phasePlanFeedbackPolicy{
			sourceCheck: &PhasePlanCheck{
				Name:     "feedback_policy_missing",
				Status:   PhasePlanStatusFail,
				Path:     path,
				Message:  "workflow graph feedback policy is missing",
				Hint:     "Declare feedback_intake in .kkachi-workflow.yaml before validating feedback phases.",
				Field:    "feedback_intake",
				Expected: graphFeedbackIntakePolicy,
				Actual:   graphIssueActualMissing,
			},
		}
	}
	required := map[int]bool{}
	allowed := map[int]bool{}
	for _, round := range feedback.RequiredRounds {
		required[round] = true
		allowed[round] = true
	}
	for _, round := range feedback.OptionalRounds {
		allowed[round] = true
	}
	return phasePlanFeedbackPolicy{requiredRounds: required, allowedRounds: allowed}
}

func feedbackRoundAllowed(round int, policy *phasePlanFeedbackPolicy) bool {
	if round < 1 {
		return false
	}
	if policy == nil || policy.sourceCheck != nil {
		return round <= 5
	}
	return policy.allowedRounds[round]
}

func feedbackRoundExpected(policy *phasePlanFeedbackPolicy) string {
	if policy == nil || policy.sourceCheck != nil {
		return "request-feedback-1..5 or handle-feedback-1..5"
	}
	rounds := make([]int, 0, len(policy.allowedRounds))
	for round := range policy.allowedRounds {
		rounds = append(rounds, round)
	}
	sort.Ints(rounds)
	parts := make([]string, 0, len(rounds)*2)
	for _, round := range rounds {
		parts = append(parts, fmt.Sprintf("request-feedback-%d", round), fmt.Sprintf("handle-feedback-%d", round))
	}
	return strings.Join(parts, ",")
}

func graphValidationIssueSummary(issues []GraphIssue) string {
	if len(issues) == 0 {
		return "invalid"
	}
	parts := make([]string, 0, len(issues))
	for _, issue := range issues {
		if issue.Name == "" {
			continue
		}
		parts = append(parts, issue.Name)
	}
	if len(parts) == 0 {
		return "invalid"
	}
	sort.Strings(parts)
	return strings.Join(parts, ",")
}

func validateFinalPhaseState(plan PhasePlan, seen map[string]PhaseRow, requiredPhaseIDs []string) []PhasePlanCheck {
	checks := []PhasePlanCheck{}
	nonTerminal := []string{}
	missingEvidence := []string{}
	for _, id := range finalPhaseIDs(seen, requiredPhaseIDs) {
		phase, ok := seen[id]
		if !ok {
			continue
		}
		if !terminalPhaseStatus(phase.Status) {
			nonTerminal = append(nonTerminal, id+"="+phase.Status)
		}
		if phase.Status == PhaseStatusComplete && strings.TrimSpace(phase.Evidence) == "" {
			missingEvidence = append(missingEvidence, id)
		}
	}
	if len(nonTerminal) == 0 {
		checks = append(checks, PhasePlanCheck{Name: "final_terminal_states", Status: PhasePlanStatusPass, Path: plan.Path, Message: "required phase rows have terminal states"})
	} else {
		checks = append(checks, PhasePlanCheck{Name: "final_terminal_states", Status: PhasePlanStatusFail, Path: plan.Path, Message: "final phase validation requires terminal states", Hint: "Mark each required phase complete, skipped, or not_applicable before final validation.", Field: "phases[].status", Expected: "complete, skipped, or not_applicable", Actual: strings.Join(nonTerminal, ",")})
	}
	if len(missingEvidence) == 0 {
		checks = append(checks, PhasePlanCheck{Name: "final_evidence_links", Status: PhasePlanStatusPass, Path: plan.Path, Message: "completed phase rows include evidence links"})
	} else {
		checks = append(checks, PhasePlanCheck{Name: "final_evidence_links", Status: PhasePlanStatusFail, Path: plan.Path, Message: "completed phase rows require evidence links", Hint: "Record a run artifact path or evidence reference for each completed phase.", Field: "phases[].evidence", Expected: "non-empty evidence for complete phases", Actual: strings.Join(missingEvidence, ",")})
	}
	return checks
}

func finalPhaseIDs(seen map[string]PhaseRow, requiredPhaseIDs []string) []string {
	result := append([]string{}, requiredPhaseIDs...)
	included := map[string]bool{}
	for _, id := range result {
		included[id] = true
	}
	extraFeedback := []string{}
	for id := range seen {
		if included[id] || feedbackPhasePattern.FindStringSubmatch(id) == nil {
			continue
		}
		extraFeedback = append(extraFeedback, id)
	}
	sort.Strings(extraFeedback)
	return append(result, extraFeedback...)
}

func validateFinalApprovalState(root Root, plan PhasePlan) []PhasePlanCheck {
	required := []string{}
	for _, phase := range plan.Phases {
		if phase.ApprovalRequired {
			required = append(required, phase.ID)
		}
	}
	if len(required) == 0 {
		return []PhasePlanCheck{{Name: "final_approval_records", Status: PhasePlanStatusPass, Path: plan.Path, Message: "no required phase approvals are declared"}}
	}
	latest, err := ApprovalLatestDecisions(root, plan.RunID)
	if err != nil {
		return []PhasePlanCheck{{Name: "final_approval_records", Status: PhasePlanStatusFail, Path: plan.Path, Message: "required phase approvals cannot be checked", Hint: "Repair approval event records or restore a coherent event log.", Field: "approval_required", Expected: "readable approval records", Actual: err.Error()}}
	}
	missing := []string{}
	rejected := []string{}
	for _, phase := range required {
		decision, ok := latest[phase]
		if !ok {
			missing = append(missing, phase)
			continue
		}
		if decision.Decision != ApprovalDecisionApproved {
			rejected = append(rejected, phase+"="+decision.Decision)
		}
	}
	if len(missing) == 0 && len(rejected) == 0 {
		return []PhasePlanCheck{{Name: "final_approval_records", Status: PhasePlanStatusPass, Path: plan.Path, Message: "required phase approvals are approved"}}
	}
	actual := []string{}
	if len(missing) > 0 {
		actual = append(actual, "missing:"+strings.Join(missing, ","))
	}
	if len(rejected) > 0 {
		actual = append(actual, "not_approved:"+strings.Join(rejected, ","))
	}
	return []PhasePlanCheck{{Name: "final_approval_records", Status: PhasePlanStatusFail, Path: plan.Path, Message: "required phase approvals are missing or not approved", Hint: "Use approval record with --decision approved for each approval-required phase before final validation.", Field: "approval_required", Expected: "approved approval records", Actual: strings.Join(actual, ";")}}
}

func knownPhaseStatus(status string) bool {
	for _, known := range phaseStatuses {
		if status == known {
			return true
		}
	}
	return false
}

func terminalPhaseStatus(status string) bool {
	return status == PhaseStatusComplete || status == PhaseStatusSkipped || status == PhaseStatusNotApplicable
}

func encodePhasePlan(plan PhasePlan) []byte {
	var builder strings.Builder
	builder.WriteString("version: ")
	builder.WriteString(yamlQuotedScalar(plan.Version))
	builder.WriteString("\nrun_id: ")
	builder.WriteString(yamlQuotedScalar(plan.RunID))
	builder.WriteString("\nphases:\n")
	for _, phase := range plan.Phases {
		builder.WriteString("  - id: ")
		builder.WriteString(yamlQuotedScalar(phase.ID))
		builder.WriteString("\n    status: ")
		builder.WriteString(yamlQuotedScalar(phase.Status))
		builder.WriteByte('\n')
		if strings.TrimSpace(phase.Evidence) != "" {
			builder.WriteString("    evidence: ")
			builder.WriteString(yamlQuotedScalar(phase.Evidence))
			builder.WriteByte('\n')
		}
		if strings.TrimSpace(phase.Reason) != "" {
			builder.WriteString("    reason: ")
			builder.WriteString(yamlQuotedScalar(phase.Reason))
			builder.WriteByte('\n')
		}
		if phase.ApprovalRequired {
			builder.WriteString("    approval_required: true\n")
		}
	}
	return []byte(builder.String())
}

func yamlQuotedScalar(value string) string {
	return strconv.Quote(value)
}

func parsePhasePlan(data []byte, path string) (PhasePlan, error) {
	plan := PhasePlan{Path: path}
	var current *PhaseRow
	flush := func() {
		if current != nil {
			plan.Phases = append(plan.Phases, *current)
		}
		current = nil
	}
	for lineNumber, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") || line == "phases:" {
			continue
		}
		if strings.HasPrefix(line, "- ") {
			flush()
			current = &PhaseRow{}
			line = strings.TrimSpace(strings.TrimPrefix(line, "- "))
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			return PhasePlan{}, &Problem{Code: "phase_plan_invalid_yaml", Message: "phase plan contains an unsupported YAML line", Hint: "Use the constrained phase-plan.yaml format emitted by phase-plan init.", Path: path, Field: "yaml", Expected: "key: value line", Actual: fmt.Sprintf("line %d", lineNumber+1)}
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		parsed, err := parseYAMLScalar(value)
		if err != nil {
			return PhasePlan{}, &Problem{Code: "phase_plan_invalid_yaml", Message: "phase plan scalar is invalid", Hint: "Use quoted string scalars or plain strings without control characters.", Path: path, Field: key, Expected: "string scalar", Actual: fmt.Sprintf("line %d: %s", lineNumber+1, err.Error())}
		}
		switch key {
		case "version":
			if current != nil {
				return PhasePlan{}, &Problem{Code: "phase_plan_invalid_yaml", Message: "top-level phase plan field appears inside a phase row", Hint: "Place version before the phases list.", Path: path, Field: key, Expected: "top-level field", Actual: fmt.Sprintf("line %d", lineNumber+1)}
			}
			plan.Version = parsed
		case "run_id":
			if current != nil {
				return PhasePlan{}, &Problem{Code: "phase_plan_invalid_yaml", Message: "top-level phase plan field appears inside a phase row", Hint: "Place run_id before the phases list.", Path: path, Field: key, Expected: "top-level field", Actual: fmt.Sprintf("line %d", lineNumber+1)}
			}
			plan.RunID = parsed
		case "id", "status", "evidence", "reason", "approval_required":
			if current == nil {
				return PhasePlan{}, &Problem{Code: "phase_plan_invalid_yaml", Message: "phase field appears outside a phase row", Hint: "Place phase fields under phases: list rows.", Path: path, Field: key, Expected: "field below phases list item", Actual: fmt.Sprintf("line %d", lineNumber+1)}
			}
			if key == "approval_required" && parsed != "true" && parsed != "false" {
				return PhasePlan{}, &Problem{Code: "phase_plan_invalid_yaml", Message: "phase approval_required field is invalid", Hint: "Use approval_required: true or approval_required: false.", Path: path, Field: key, Expected: "true or false", Actual: parsed}
			}
			setPhaseField(current, key, parsed)
		default:
			return PhasePlan{}, &Problem{Code: "phase_plan_invalid_yaml", Message: "phase plan contains an unsupported field", Hint: "Use version, run_id, phases, id, status, evidence, reason, and approval_required only.", Path: path, Field: key, Expected: "supported phase-plan field", Actual: key}
		}
	}
	flush()
	if _, err := validatePhasePlanShape(plan); err != nil {
		return PhasePlan{}, err
	}
	return plan, nil
}

func setPhaseField(phase *PhaseRow, key string, value string) {
	switch key {
	case "id":
		phase.ID = value
	case "status":
		phase.Status = value
	case "evidence":
		phase.Evidence = value
	case "reason":
		phase.Reason = value
	case "approval_required":
		phase.ApprovalRequired = value == "true"
	}
}

func parseYAMLScalar(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if strings.HasPrefix(value, `"`) {
		if !strings.HasSuffix(value, `"`) || len(value) == 1 {
			return "", fmt.Errorf("unterminated quoted scalar")
		}
		unquoted, err := strconv.Unquote(value)
		if err != nil {
			return "", err
		}
		return unquoted, nil
	}
	if strings.ContainsAny(value, "\r\n\t") {
		return "", fmt.Errorf("control character")
	}
	return value, nil
}
