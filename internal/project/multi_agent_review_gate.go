package project

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

const (
	multiAgentReviewArtifact      = "multi-agent-review/status.json"
	multiAgentReviewSchemaVersion = "mar-evidence.v1"
)

var multiAgentReviewStatuses = []string{"PASS", "PASS_WITH_FINDINGS", "REQUEST_CHANGES", "BLOCKED", "DEGRADED", "FAILED"}
var multiAgentReviewCoveredStatuses = []string{"PASS", "PASS_WITH_FINDINGS"}
var multiAgentReviewProviderCandidates = []string{"primary", "secondary", "alternate", "retry", "premium"}

type multiAgentReviewEvidence struct {
	SchemaVersion        string                    `json:"schema_version"`
	RunID                string                    `json:"run_id"`
	TaskID               string                    `json:"task_id"`
	Status               string                    `json:"status"`
	Reason               string                    `json:"reason"`
	Coverage             *multiAgentReviewCoverage `json:"coverage"`
	ProviderAttempts     []marProviderAttempt      `json:"provider_attempts"`
	BlueDispositionRef   *marEvidenceRef           `json:"blue_disposition_ref"`
	RedAdjudicationRef   *marEvidenceRef           `json:"red_adjudication_ref"`
	AlternateApprovalRef *marEvidenceRef           `json:"alternate_approval_ref"`
	WaiverRef            *marEvidenceRef           `json:"waiver_ref"`
	PremiumReviewUsed    bool                      `json:"premium_review_used"`
	PremiumApprovalRef   *marEvidenceRef           `json:"premium_approval_ref"`
	BlueReason           string                    `json:"blue_reason"`
}

type multiAgentReviewCoverage struct {
	RequiredRoles           []string                        `json:"required_roles"`
	ObservedRoles           []string                        `json:"observed_roles"`
	CoveredRoles            []string                        `json:"covered_roles"`
	MinimumMet              bool                            `json:"minimum_met"`
	UnresolvedRequiredRoles []string                        `json:"unresolved_required_roles"`
	ByRole                  map[string]multiAgentReviewRole `json:"by_role"`
	RedTriggerSummary       *multiAgentReviewRedTrigger     `json:"red_trigger_summary"`
	BlueMatrixInputs        map[string]any                  `json:"blue_matrix_inputs"`
	OperatorReportText      string                          `json:"operator_report_text"`
}

type multiAgentReviewRole struct {
	RoleID                 string   `json:"role_id"`
	State                  string   `json:"state"`
	Resolution             string   `json:"resolution"`
	Reason                 string   `json:"reason"`
	AttemptID              string   `json:"attempt_id"`
	ProviderID             string   `json:"provider_id"`
	ProviderCandidate      string   `json:"provider_candidate"`
	ProviderFailureReasons []string `json:"provider_failure_reasons"`
}

type multiAgentReviewRedTrigger struct {
	RedAdjudicationRequired bool     `json:"red_adjudication_required"`
	Triggers                []string `json:"triggers"`
}

type marProviderAttempt struct {
	SchemaVersion         string         `json:"schema_version"`
	RunID                 string         `json:"run_id"`
	TaskID                string         `json:"task_id"`
	AttemptID             string         `json:"attempt_id"`
	RoleID                string         `json:"role_id"`
	ProviderID            string         `json:"provider_id"`
	ProviderCandidate     string         `json:"provider_candidate"`
	CommandLane           string         `json:"command_lane"`
	SelectedModel         any            `json:"selected_model"`
	TerminalStatus        string         `json:"terminal_status"`
	ParserStatus          string         `json:"parser_status"`
	ProviderFailureReason string         `json:"provider_failure_reason"`
	RawOutputPath         string         `json:"raw_output_path"`
	ParsedFindingPath     string         `json:"parsed_finding_path"`
	MutationCheck         map[string]any `json:"mutation_check"`
}

type marEvidenceRef struct {
	Path     string   `json:"path"`
	Checksum string   `json:"checksum,omitempty"`
	Markers  []string `json:"markers,omitempty"`
}

func checkMultiAgentReviewGate(root Root, metadata RunMetadata) (GateCheckResult, error) {
	path, err := artifactPath(root, metadata.RunID, multiAgentReviewArtifact)
	if err != nil {
		check := GateCheck{Name: "mar_artifact", Status: GateStatusFail, Message: "MAR evidence artifact path is invalid", Hint: "Use a repository-confined multi-agent-review/status.json artifact.", Field: "path", Expected: multiAgentReviewArtifact, Actual: err.Error()}
		return gateResultFromChecks(metadata.RunID, GateMultiAgentReview, []GateCheck{check}), nil
	}
	info, err := os.Lstat(path.Absolute)
	if os.IsNotExist(err) {
		if !multiAgentReviewGateRequired(root, metadata) {
			check := GateCheck{Name: "mar_task_scope", Status: GateStatusNotApplicable, Path: path.Relative, Message: "MAR evidence is not required for this task without a MAR artifact", Field: "task_id", Expected: "MAR task id or MAR evidence artifact", Actual: metadataTaskID(metadata)}
			return GateCheckResult{RunID: metadata.RunID, Gate: GateMultiAgentReview, Status: GateStatusNotApplicable, Checks: []GateCheck{check}}, nil
		}
		check := GateCheck{Name: "mar_artifact", Status: GateStatusFail, Path: path.Relative, Message: "required MAR evidence artifact is missing", Hint: "Write multi-agent-review/status.json before checking the MAR gate.", Field: "path", Expected: "existing regular file", Actual: "missing"}
		return gateResultFromChecks(metadata.RunID, GateMultiAgentReview, []GateCheck{check}), nil
	}
	if err != nil {
		check := GateCheck{Name: "mar_artifact", Status: GateStatusFail, Path: path.Relative, Message: "cannot inspect MAR evidence artifact", Hint: "Check run artifact permissions before checking the MAR gate.", Field: "path", Expected: "inspectable regular file", Actual: err.Error()}
		return gateResultFromChecks(metadata.RunID, GateMultiAgentReview, []GateCheck{check}), nil
	}
	if !info.Mode().IsRegular() {
		check := GateCheck{Name: "mar_artifact", Status: GateStatusFail, Path: path.Relative, Message: "MAR evidence artifact must be a regular file", Hint: "Move the conflicting path and rewrite the MAR evidence artifact.", Field: "path", Expected: "regular file", Actual: fileKind(info)}
		return gateResultFromChecks(metadata.RunID, GateMultiAgentReview, []GateCheck{check}), nil
	}
	content, err := os.ReadFile(path.Absolute)
	if err != nil {
		check := GateCheck{Name: "mar_artifact", Status: GateStatusFail, Path: path.Relative, Message: "cannot read MAR evidence artifact", Hint: "Check run artifact permissions before checking the MAR gate.", Field: "path", Expected: "readable file", Actual: err.Error()}
		return gateResultFromChecks(metadata.RunID, GateMultiAgentReview, []GateCheck{check}), nil
	}
	if !multiAgentReviewGateRequired(root, metadata) && multiAgentReviewBaselineArtifact(content) {
		check := GateCheck{Name: "mar_task_scope", Status: GateStatusNotApplicable, Path: path.Relative, Message: "MAR evidence is not required for this task", Field: "task_id", Expected: "MAR task id or explicit MAR evidence requirement", Actual: metadataTaskID(metadata)}
		return GateCheckResult{RunID: metadata.RunID, Gate: GateMultiAgentReview, Status: GateStatusNotApplicable, Checks: []GateCheck{check}}, nil
	}
	return multiAgentReviewResultFromChecks(metadata.RunID, validateMultiAgentReviewEvidence(root, metadata, path.Relative, content)), nil
}

func multiAgentReviewBaselineArtifact(content []byte) bool {
	var baseline struct {
		Version  string `json:"version"`
		Status   string `json:"status"`
		Artifact string `json:"artifact"`
	}
	if err := json.Unmarshal(content, &baseline); err != nil {
		return false
	}
	return baseline.Version == "0.1" && baseline.Status == "pending" && baseline.Artifact == multiAgentReviewArtifact
}

func validateMultiAgentReviewEvidence(root Root, metadata RunMetadata, relative string, content []byte) []GateCheck {
	checks := []GateCheck{{Name: "mar_artifact", Status: GateStatusPass, Path: relative, Message: "MAR evidence artifact is present"}}
	var raw map[string]any
	if err := json.Unmarshal(content, &raw); err != nil || raw == nil {
		actual := "not an object"
		if err != nil {
			actual = err.Error()
		}
		return append(checks, GateCheck{Name: "mar_json", Status: GateStatusFail, Path: relative, Message: "MAR evidence is malformed JSON", Hint: "Write multi-agent-review/status.json as one JSON object.", Field: "json", Expected: "JSON object", Actual: actual})
	}
	var evidence multiAgentReviewEvidence
	if err := json.Unmarshal(content, &evidence); err != nil {
		return append(checks, GateCheck{Name: "mar_json", Status: GateStatusFail, Path: relative, Message: "MAR evidence cannot be decoded", Hint: "Use the mar-evidence.v1 evidence schema.", Field: "json", Expected: "mar-evidence.v1 object", Actual: err.Error()})
	}
	checks = append(checks, validateMultiAgentReviewRequiredFields(relative, raw, evidence)...)
	checks = append(checks, validateMultiAgentReviewIdentity(metadata, relative, evidence)...)
	checks = append(checks, validateMultiAgentReviewStatus(relative, evidence)...)
	checks = append(checks, validateMultiAgentReviewCoverage(root, metadata, relative, evidence)...)
	checks = append(checks, validateMultiAgentReviewDispositionRefs(root, relative, evidence)...)
	return checks
}

func validateMultiAgentReviewRequiredFields(relative string, raw map[string]any, evidence multiAgentReviewEvidence) []GateCheck {
	checks := []GateCheck{}
	for _, field := range []string{"schema_version", "run_id", "task_id", "status", "reason", "coverage", "provider_attempts", "blue_disposition_ref"} {
		if _, ok := raw[field]; !ok {
			checks = append(checks, GateCheck{Name: "mar_required_field", Status: GateStatusFail, Path: relative, Message: "MAR evidence is missing a required field", Hint: "Write all mar-evidence.v1 required fields before checking the gate.", Field: field, Expected: "present", Actual: "missing"})
		}
	}
	if evidence.SchemaVersion == multiAgentReviewSchemaVersion {
		checks = append(checks, GateCheck{Name: "schema_version", Status: GateStatusPass, Path: relative, Message: "MAR evidence schema version is supported", Field: "schema_version", Actual: evidence.SchemaVersion})
	} else {
		actual := evidence.SchemaVersion
		if strings.TrimSpace(actual) == "" {
			actual = "missing"
		}
		checks = append(checks, GateCheck{Name: "schema_version", Status: GateStatusFail, Path: relative, Message: "MAR evidence schema version is unsupported", Hint: "Use schema_version mar-evidence.v1.", Field: "schema_version", Expected: multiAgentReviewSchemaVersion, Actual: actual})
	}
	if strings.TrimSpace(evidence.Reason) == "" {
		checks = append(checks, GateCheck{Name: "reason", Status: GateStatusFail, Path: relative, Message: "MAR evidence reason is missing", Hint: "Record a concise mechanical reason for the MAR status.", Field: "reason", Expected: "non-empty string", Actual: "missing"})
	}
	return checks
}
func validateMultiAgentReviewIdentity(metadata RunMetadata, relative string, evidence multiAgentReviewEvidence) []GateCheck {
	checks := []GateCheck{}
	if evidence.RunID == metadata.RunID {
		checks = append(checks, GateCheck{Name: "run_id", Status: GateStatusPass, Path: relative, Message: "run_id matches run metadata", Field: "run_id", Actual: evidence.RunID})
	} else {
		checks = append(checks, GateCheck{Name: "run_id", Status: GateStatusFail, Path: relative, Message: "run_id does not match run metadata", Hint: "Record the current run id in MAR evidence.", Field: "run_id", Expected: metadata.RunID, Actual: evidence.RunID})
	}
	if evidence.TaskID == metadataTaskID(metadata) {
		checks = append(checks, GateCheck{Name: "task_id", Status: GateStatusPass, Path: relative, Message: "task_id matches run metadata", Field: "task_id", Actual: evidence.TaskID})
	} else {
		checks = append(checks, GateCheck{Name: "task_id", Status: GateStatusFail, Path: relative, Message: "task_id does not match run metadata", Hint: "Record the current run task id in MAR evidence.", Field: "task_id", Expected: metadataTaskID(metadata), Actual: evidence.TaskID})
	}
	return checks
}

func validateMultiAgentReviewStatus(relative string, evidence multiAgentReviewEvidence) []GateCheck {
	checks := []GateCheck{}
	if allowed(evidence.Status, multiAgentReviewStatuses...) {
		checks = append(checks, GateCheck{Name: "mar_status", Status: GateStatusPass, Path: relative, Message: "MAR status vocabulary is supported", Field: "status", Actual: evidence.Status})
		if !allowed(evidence.Status, multiAgentReviewCoveredStatuses...) {
			checks = append(checks, GateCheck{Name: "mar_status_passable", Status: GateStatusFail, Path: relative, Message: "non-pass MAR status cannot satisfy the MAR gate", Hint: "Resolve MAR findings or record a passable PASS/PASS_WITH_FINDINGS disposition before final gate.", Field: "status", Expected: strings.Join(multiAgentReviewCoveredStatuses, ","), Actual: evidence.Status})
		}
	} else {
		actual := evidence.Status
		if strings.TrimSpace(actual) == "" {
			actual = "missing"
		}
		checks = append(checks, GateCheck{Name: "mar_status", Status: GateStatusFail, Path: relative, Message: "MAR status vocabulary is unsupported", Hint: "Use PASS, PASS_WITH_FINDINGS, REQUEST_CHANGES, BLOCKED, DEGRADED, or FAILED.", Field: "status", Expected: strings.Join(multiAgentReviewStatuses, ","), Actual: actual})
	}
	if (evidence.Status == "DEGRADED" || evidence.Status == "BLOCKED" || evidence.Status == "FAILED") && strings.TrimSpace(evidence.BlueReason) == "" {
		checks = append(checks, GateCheck{Name: "blue_reason", Status: GateStatusFail, Path: relative, Message: "non-clean MAR status requires a Blue reason", Hint: "Record blue_reason for degraded, blocked, or failed MAR coverage.", Field: "blue_reason", Expected: "non-empty Blue reason", Actual: "missing"})
	}
	return checks
}

func validateMultiAgentReviewCoverage(root Root, metadata RunMetadata, relative string, evidence multiAgentReviewEvidence) []GateCheck {
	if evidence.Coverage == nil {
		return []GateCheck{{Name: "coverage", Status: GateStatusFail, Path: relative, Message: "MAR role coverage is missing", Hint: "Record role-first coverage under coverage.", Field: "coverage", Expected: "object", Actual: "missing"}}
	}
	checks := []GateCheck{}
	coverage := evidence.Coverage
	if len(coverage.RequiredRoles) == 0 {
		checks = append(checks, GateCheck{Name: "required_roles", Status: GateStatusFail, Path: relative, Message: "MAR evidence lacks required roles", Hint: "Record the KAS-declared required review roles.", Field: "coverage.required_roles", Expected: "one or more roles", Actual: "empty"})
	}
	attemptsByID := map[string]marProviderAttempt{}
	for _, attempt := range evidence.ProviderAttempts {
		if strings.TrimSpace(attempt.AttemptID) != "" {
			attemptsByID[attempt.AttemptID] = attempt
		}
		checks = append(checks, validateMultiAgentReviewAttempt(root, metadata, relative, attempt)...)
	}
	for _, roleID := range coverage.RequiredRoles {
		roleID = strings.TrimSpace(roleID)
		role, ok := coverage.ByRole[roleID]
		if !ok {
			checks = append(checks, GateCheck{Name: "required_role:" + roleID, Status: GateStatusFail, Path: relative, Message: "required MAR role lacks coverage evidence", Hint: "Record a by_role entry for every required MAR role.", Field: "coverage.by_role." + roleID, Expected: "role coverage entry", Actual: "missing"})
			continue
		}
		checks = append(checks, validateMultiAgentReviewRole(relative, roleID, role, attemptsByID)...)
	}
	if evidence.Status == "PASS" && (!coverage.MinimumMet || len(coverage.UnresolvedRequiredRoles) > 0) {
		checks = append(checks, GateCheck{Name: "minimum_role_coverage", Status: GateStatusFail, Path: relative, Message: "PASS cannot include unresolved required MAR roles", Hint: "Refresh MAR coverage or record a non-clean status with Blue disposition.", Field: "coverage.minimum_met", Expected: "true with no unresolved_required_roles", Actual: fmt.Sprintf("minimum_met=%t unresolved=%d", coverage.MinimumMet, len(coverage.UnresolvedRequiredRoles))})
	}
	return checks
}

func validateMultiAgentReviewRole(relative string, roleID string, role multiAgentReviewRole, attemptsByID map[string]marProviderAttempt) []GateCheck {
	name := "required_role:" + roleID
	if role.RoleID != roleID {
		return []GateCheck{{Name: name, Status: GateStatusFail, Path: relative, Message: "required MAR role id is inconsistent", Hint: "Make coverage.by_role keys and role_id fields match.", Field: "role_id", Expected: roleID, Actual: role.RoleID}}
	}
	switch role.State {
	case "covered":
		attempt, ok := attemptsByID[role.AttemptID]
		if !ok {
			return []GateCheck{{Name: name, Status: GateStatusFail, Path: relative, Message: "covered MAR role lacks a matching provider attempt", Hint: "Preserve the successful provider attempt referenced by role coverage.", Field: "attempt_id", Expected: role.AttemptID, Actual: "missing"}}
		}
		if attempt.RoleID != roleID || attempt.ProviderID != role.ProviderID || attempt.ProviderCandidate != role.ProviderCandidate || !allowed(attempt.TerminalStatus, multiAgentReviewCoveredStatuses...) {
			return []GateCheck{{Name: name, Status: GateStatusFail, Path: relative, Message: "covered MAR role does not match a successful provider attempt", Hint: "Link covered roles only to PASS or PASS_WITH_FINDINGS attempts for the same role/provider candidate.", Field: "coverage.by_role." + roleID, Expected: "matching successful attempt", Actual: fmt.Sprintf("attempt_role=%s provider=%s candidate=%s status=%s", attempt.RoleID, attempt.ProviderID, attempt.ProviderCandidate, attempt.TerminalStatus)}}
		}
		return []GateCheck{{Name: name, Status: GateStatusPass, Path: relative, Message: "required MAR role is covered by matching provider evidence", Field: "coverage.by_role." + roleID, Actual: role.Resolution}}
	case "unresolved":
		if role.Reason == "missing_role_attempt" || len(role.ProviderFailureReasons) > 0 {
			return []GateCheck{{Name: name, Status: GateStatusFail, Path: relative, Message: "required MAR role remains unresolved", Hint: "KAH fails closed until KAS records covered role evidence or explicit waiver/disposition evidence.", Field: "coverage.by_role." + roleID + ".state", Expected: "covered", Actual: "unresolved"}}
		}
		return []GateCheck{{Name: name, Status: GateStatusFail, Path: relative, Message: "unresolved MAR role lacks deterministic failure reason evidence", Hint: "Record provider_failure_reasons or missing_role_attempt for unresolved roles.", Field: "coverage.by_role." + roleID + ".provider_failure_reasons", Expected: "one or more reason codes", Actual: "missing"}}
	case "waived":
		if strings.TrimSpace(role.Reason) == "" && len(role.ProviderFailureReasons) == 0 {
			return []GateCheck{{Name: name, Status: GateStatusFail, Path: relative, Message: "waived MAR role lacks deterministic reason evidence", Hint: "Record waiver reason and provider failure reason evidence for waived required roles.", Field: "coverage.by_role." + roleID + ".reason", Expected: "non-empty waiver/failure reason", Actual: "missing"}}
		}
		attempt, ok := attemptsByID[role.AttemptID]
		if !ok {
			return []GateCheck{{Name: name, Status: GateStatusFail, Path: relative, Message: "waived MAR role lacks a linked failed provider attempt", Hint: "Link waived roles to a failed, blocked, degraded, or request-changes attempt for the same role.", Field: "attempt_id", Expected: "matching non-clean provider attempt", Actual: "missing"}}
		}
		if attempt.RoleID != roleID || allowed(attempt.TerminalStatus, multiAgentReviewCoveredStatuses...) {
			return []GateCheck{{Name: name, Status: GateStatusFail, Path: relative, Message: "waived MAR role is not linked to non-clean provider attempt evidence", Hint: "Preserve the failed/unavailable attempt evidence that justified the waiver.", Field: "coverage.by_role." + roleID, Expected: "matching non-clean provider attempt", Actual: fmt.Sprintf("attempt_role=%s status=%s", attempt.RoleID, attempt.TerminalStatus)}}
		}
		if role.ProviderID != "" && attempt.ProviderID != role.ProviderID {
			return []GateCheck{{Name: name, Status: GateStatusFail, Path: relative, Message: "waived MAR role provider id does not match linked provider attempt", Hint: "Keep waived role coverage and attempt evidence identity-consistent.", Field: "provider_id", Expected: role.ProviderID, Actual: attempt.ProviderID}}
		}
		if role.ProviderCandidate != "" && attempt.ProviderCandidate != role.ProviderCandidate {
			return []GateCheck{{Name: name, Status: GateStatusFail, Path: relative, Message: "waived MAR role provider candidate does not match linked provider attempt", Hint: "Keep waived role coverage and attempt evidence identity-consistent.", Field: "provider_candidate", Expected: role.ProviderCandidate, Actual: attempt.ProviderCandidate}}
		}
		return []GateCheck{{Name: name, Status: GateStatusPass, Path: relative, Message: "required MAR role is explicitly waived with linked failed provider evidence", Field: "coverage.by_role." + roleID + ".state", Actual: "waived"}}
	default:
		return []GateCheck{{Name: name, Status: GateStatusFail, Path: relative, Message: "required MAR role state is unsupported", Hint: "Use covered, waived, or unresolved role states.", Field: "coverage.by_role." + roleID + ".state", Expected: "covered, waived, or unresolved", Actual: role.State}}
	}
}

func validateMultiAgentReviewAttempt(root Root, metadata RunMetadata, artifactRelative string, attempt marProviderAttempt) []GateCheck {
	name := "provider_attempt:" + strings.TrimSpace(attempt.AttemptID)
	if name == "provider_attempt:" {
		name = "provider_attempt"
	}
	checks := []GateCheck{}
	if attempt.SchemaVersion != "mar.provider_attempt.v1" {
		checks = append(checks, GateCheck{Name: name, Status: GateStatusFail, Path: artifactRelative, Message: "provider attempt schema version is unsupported", Hint: "Use KAS mar.provider_attempt.v1 attempt evidence.", Field: "schema_version", Expected: "mar.provider_attempt.v1", Actual: attempt.SchemaVersion})
	}
	for _, field := range []struct{ name, value string }{{"run_id", attempt.RunID}, {"task_id", attempt.TaskID}, {"attempt_id", attempt.AttemptID}, {"role_id", attempt.RoleID}, {"provider_id", attempt.ProviderID}, {"provider_candidate", attempt.ProviderCandidate}, {"terminal_status", attempt.TerminalStatus}} {
		if strings.TrimSpace(field.value) == "" {
			checks = append(checks, GateCheck{Name: name, Status: GateStatusFail, Path: artifactRelative, Message: "provider attempt lacks a required field", Hint: "Record complete provider attempt identity and status evidence.", Field: field.name, Expected: "non-empty string", Actual: "missing"})
		}
	}
	if strings.TrimSpace(attempt.RunID) != "" && attempt.RunID != metadata.RunID {
		checks = append(checks, GateCheck{Name: name, Status: GateStatusFail, Path: artifactRelative, Message: "provider attempt run_id does not match run metadata", Hint: "Do not count stale or cross-run MAR provider attempts.", Field: "run_id", Expected: metadata.RunID, Actual: attempt.RunID})
	}
	if strings.TrimSpace(attempt.TaskID) != "" && attempt.TaskID != metadataTaskID(metadata) {
		checks = append(checks, GateCheck{Name: name, Status: GateStatusFail, Path: artifactRelative, Message: "provider attempt task_id does not match run metadata", Hint: "Do not count stale or cross-task MAR provider attempts.", Field: "task_id", Expected: metadataTaskID(metadata), Actual: attempt.TaskID})
	}
	if strings.TrimSpace(attempt.ProviderCandidate) != "" && !allowed(attempt.ProviderCandidate, multiAgentReviewProviderCandidates...) {
		checks = append(checks, GateCheck{Name: name, Status: GateStatusFail, Path: artifactRelative, Message: "provider candidate vocabulary is unsupported", Hint: "Use primary, secondary, alternate, retry, or premium.", Field: "provider_candidate", Expected: strings.Join(multiAgentReviewProviderCandidates, ","), Actual: attempt.ProviderCandidate})
	}
	if strings.TrimSpace(attempt.TerminalStatus) != "" && !allowed(attempt.TerminalStatus, multiAgentReviewStatuses...) {
		checks = append(checks, GateCheck{Name: name, Status: GateStatusFail, Path: artifactRelative, Message: "provider attempt terminal status is unsupported", Hint: "Use the MAR status vocabulary.", Field: "terminal_status", Expected: strings.Join(multiAgentReviewStatuses, ","), Actual: attempt.TerminalStatus})
	}
	if allowed(attempt.TerminalStatus, multiAgentReviewCoveredStatuses...) {
		if checked, _ := attempt.MutationCheck["checked"].(bool); !checked {
			checks = append(checks, GateCheck{Name: name, Status: GateStatusFail, Path: artifactRelative, Message: "successful provider attempt lacks mutation guard evidence", Hint: "Record mutation_check.checked=true for read-only MAR attempts.", Field: "mutation_check.checked", Expected: "true", Actual: fmt.Sprintf("%v", attempt.MutationCheck["checked"])})
		}
		if detected, _ := attempt.MutationCheck["detected"].(bool); detected {
			checks = append(checks, GateCheck{Name: name, Status: GateStatusFail, Path: artifactRelative, Message: "successful provider attempt reports a mutation", Hint: "Do not count mutated reviewer attempts as clean MAR coverage.", Field: "mutation_check.detected", Expected: "false", Actual: "true"})
		}
	}
	if !allowed(attempt.TerminalStatus, multiAgentReviewCoveredStatuses...) && strings.TrimSpace(attempt.ProviderFailureReason) == "" {
		checks = append(checks, GateCheck{Name: name, Status: GateStatusFail, Path: artifactRelative, Message: "non-clean provider attempt lacks failure reason", Hint: "Record a deterministic provider_failure_reason for non-clean MAR attempts.", Field: "provider_failure_reason", Expected: "non-empty reason code", Actual: "missing"})
	}
	checks = append(checks, validateMultiAgentReviewPathRef(root, artifactRelative, name+".raw_output_path", attempt.RawOutputPath, allowed(attempt.TerminalStatus, multiAgentReviewCoveredStatuses...))...)
	if allowed(attempt.TerminalStatus, multiAgentReviewCoveredStatuses...) {
		checks = append(checks, validateMultiAgentReviewPathRef(root, artifactRelative, name+".parsed_finding_path", attempt.ParsedFindingPath, true)...)
	}
	if !gateChecksContainStatus(checks, GateStatusFail) {
		checks = append(checks, GateCheck{Name: name, Status: GateStatusPass, Path: artifactRelative, Message: "provider attempt evidence is mechanically valid", Field: "attempt_id", Actual: attempt.AttemptID})
	}
	return checks
}

func gateChecksContainStatus(checks []GateCheck, status string) bool {
	for _, check := range checks {
		if check.Status == status {
			return true
		}
	}
	return false
}

func validateMultiAgentReviewDispositionRefs(root Root, relative string, evidence multiAgentReviewEvidence) []GateCheck {
	checks := []GateCheck{}
	if evidence.BlueDispositionRef == nil {
		checks = append(checks, GateCheck{Name: "blue_disposition_ref", Status: GateStatusFail, Path: relative, Message: "MAR evidence lacks Blue disposition reference", Hint: "Record blue_disposition_ref before the MAR gate can pass.", Field: "blue_disposition_ref", Expected: "evidence ref", Actual: "missing"})
	} else {
		checks = append(checks, validateMultiAgentReviewEvidenceRef(root, relative, "blue_disposition_ref", *evidence.BlueDispositionRef)...)
	}
	redRequired := evidence.Coverage != nil && evidence.Coverage.RedTriggerSummary != nil && evidence.Coverage.RedTriggerSummary.RedAdjudicationRequired
	if redRequired {
		if evidence.RedAdjudicationRef == nil {
			checks = append(checks, GateCheck{Name: "red_adjudication_ref", Status: GateStatusFail, Path: relative, Message: "MAR evidence lacks required Red adjudication reference", Hint: "Record red_adjudication_ref when MAR red_trigger_summary requires adjudication.", Field: "red_adjudication_ref", Expected: "evidence ref", Actual: "missing"})
		} else {
			checks = append(checks, validateMultiAgentReviewEvidenceRef(root, relative, "red_adjudication_ref", *evidence.RedAdjudicationRef)...)
		}
	}
	if multiAgentReviewAlternateApprovalRequired(evidence) {
		if evidence.AlternateApprovalRef == nil {
			checks = append(checks, GateCheck{Name: "alternate_approval_ref", Status: GateStatusFail, Path: relative, Message: "alternate or secondary MAR provider evidence lacks approval reference", Hint: "Record alternate_approval_ref when secondary or alternate provider attempts are counted as coverage.", Field: "alternate_approval_ref", Expected: "evidence ref", Actual: "missing"})
		} else {
			checks = append(checks, validateMultiAgentReviewEvidenceRef(root, relative, "alternate_approval_ref", *evidence.AlternateApprovalRef)...)
		}
	}
	if multiAgentReviewWaiverRequired(evidence) {
		if evidence.WaiverRef == nil {
			checks = append(checks, GateCheck{Name: "waiver_ref", Status: GateStatusFail, Path: relative, Message: "waived MAR role lacks waiver evidence reference", Hint: "Record waiver_ref when required MAR role coverage is accepted by explicit 주군 waiver.", Field: "waiver_ref", Expected: "evidence ref", Actual: "missing"})
		} else {
			checks = append(checks, validateMultiAgentReviewEvidenceRef(root, relative, "waiver_ref", *evidence.WaiverRef)...)
		}
	}
	if evidence.PremiumReviewUsed || multiAgentReviewProviderCandidateUsed(evidence, "premium") {
		if evidence.PremiumApprovalRef == nil {
			checks = append(checks, GateCheck{Name: "premium_approval_ref", Status: GateStatusFail, Path: relative, Message: "premium MAR evidence lacks approval reference", Hint: "Record premium_approval_ref when premium review was used.", Field: "premium_approval_ref", Expected: "evidence ref", Actual: "missing"})
		} else {
			checks = append(checks, validateMultiAgentReviewEvidenceRef(root, relative, "premium_approval_ref", *evidence.PremiumApprovalRef)...)
		}
	}
	return checks
}

func multiAgentReviewAlternateApprovalRequired(evidence multiAgentReviewEvidence) bool {
	for _, attempt := range evidence.ProviderAttempts {
		if !allowed(attempt.TerminalStatus, multiAgentReviewCoveredStatuses...) {
			continue
		}
		if attempt.ProviderCandidate == "secondary" || attempt.ProviderCandidate == "alternate" {
			return true
		}
	}
	return false
}

func multiAgentReviewProviderCandidateUsed(evidence multiAgentReviewEvidence, candidate string) bool {
	for _, attempt := range evidence.ProviderAttempts {
		if attempt.ProviderCandidate == candidate {
			return true
		}
	}
	return false
}

func multiAgentReviewWaiverRequired(evidence multiAgentReviewEvidence) bool {
	if evidence.Coverage == nil {
		return false
	}
	for _, role := range evidence.Coverage.ByRole {
		if role.State == "waived" || role.Resolution == "waived" {
			return true
		}
	}
	return false
}

func validateMultiAgentReviewEvidenceRef(root Root, artifactRelative string, field string, ref marEvidenceRef) []GateCheck {
	checks := []GateCheck{}
	if strings.TrimSpace(ref.Path) == "" {
		return []GateCheck{{Name: field, Status: GateStatusFail, Path: artifactRelative, Message: "MAR evidence ref path is missing", Hint: "Record repository-confined MAR evidence refs.", Field: field + ".path", Expected: "non-empty path", Actual: "missing"}}
	}
	path, err := ResolveRelativePath(root, ref.Path)
	if err != nil {
		return []GateCheck{{Name: field, Status: GateStatusFail, Path: artifactRelative, Message: "MAR evidence ref path is unsafe", Hint: "Use repository-relative paths without absolute paths or parent traversal.", Field: field + ".path", Expected: "repository-confined relative path", Actual: err.Error()}}
	}
	content, okCheck := readRegularEvidenceRef(root, artifactRelative, field, path)
	if okCheck != nil {
		return append(checks, *okCheck)
	}
	checks = append(checks, validateMultiAgentReviewChecksumAndMarkers(field, path.Relative, content, ref.Checksum, ref.Markers)...)
	if len(checks) == 0 {
		checks = append(checks, GateCheck{Name: field, Status: GateStatusPass, Path: path.Relative, Message: "MAR evidence ref path is safe and present", Field: field + ".path", Actual: path.Relative})
	}
	return checks
}

func validateMultiAgentReviewPathRef(root Root, artifactRelative string, field string, ref string, required bool) []GateCheck {
	if strings.TrimSpace(ref) == "" {
		if required {
			return []GateCheck{{Name: field, Status: GateStatusFail, Path: artifactRelative, Message: "MAR provider attempt path reference is missing", Hint: "Record repository-confined raw/parsed path references for provider attempts.", Field: field, Expected: "non-empty path", Actual: "missing"}}
		}
		return nil
	}
	path, err := ResolveRelativePath(root, ref)
	if err != nil {
		return []GateCheck{{Name: field, Status: GateStatusFail, Path: artifactRelative, Message: "MAR provider attempt path reference is unsafe", Hint: "Use repository-relative paths without absolute paths or parent traversal.", Field: field, Expected: "repository-confined relative path", Actual: err.Error()}}
	}
	_, okCheck := readRegularEvidenceRef(root, artifactRelative, field, path)
	if okCheck != nil {
		return []GateCheck{*okCheck}
	}
	return []GateCheck{{Name: field, Status: GateStatusPass, Path: path.Relative, Message: "MAR provider attempt path reference is safe and present", Field: field, Actual: path.Relative}}
}

func readRegularEvidenceRef(root Root, artifactRelative string, field string, path SafePath) ([]byte, *GateCheck) {
	info, err := os.Lstat(path.Absolute)
	if err != nil {
		actual := err.Error()
		if os.IsNotExist(err) {
			actual = "missing"
		}
		check := GateCheck{Name: field, Status: GateStatusFail, Path: path.Relative, Message: "MAR evidence ref path is missing or unreadable", Hint: "Preserve referenced MAR evidence artifacts before checking the gate.", Field: field, Expected: "existing regular file", Actual: actual}
		return nil, &check
	}
	if !info.Mode().IsRegular() {
		check := GateCheck{Name: field, Status: GateStatusFail, Path: path.Relative, Message: "MAR evidence ref path must be a regular file", Hint: "Reference regular evidence files only.", Field: field, Expected: "regular file", Actual: fileKind(info)}
		return nil, &check
	}
	content, err := os.ReadFile(path.Absolute)
	if err != nil {
		check := GateCheck{Name: field, Status: GateStatusFail, Path: path.Relative, Message: "MAR evidence ref path cannot be read", Hint: "Check evidence file permissions before rerunning the gate.", Field: field, Expected: "readable file", Actual: err.Error()}
		return nil, &check
	}
	return content, nil
}

func validateMultiAgentReviewChecksumAndMarkers(field string, relative string, content []byte, checksum string, markers []string) []GateCheck {
	checks := []GateCheck{}
	if strings.TrimSpace(checksum) != "" {
		if !tokenEconomyChecksumPattern.MatchString(checksum) {
			checks = append(checks, GateCheck{Name: field + ".checksum", Status: GateStatusFail, Path: relative, Message: "MAR evidence checksum has an unsupported format", Hint: "Use sha256:<64 hex characters>.", Field: field + ".checksum", Expected: "sha256:<64hex>", Actual: checksum})
		} else {
			sum := sha256.Sum256(content)
			actual := "sha256:" + hex.EncodeToString(sum[:])
			if !strings.EqualFold(actual, checksum) {
				checks = append(checks, GateCheck{Name: field + ".checksum", Status: GateStatusFail, Path: relative, Message: "MAR evidence checksum does not match current file content", Hint: "Refresh the MAR evidence ref checksum or restore the referenced artifact.", Field: field + ".checksum", Expected: checksum, Actual: actual})
			} else {
				checks = append(checks, GateCheck{Name: field + ".checksum", Status: GateStatusPass, Path: relative, Message: "MAR evidence checksum matches", Field: field + ".checksum", Actual: actual})
			}
		}
	}
	for i, marker := range markers {
		if strings.TrimSpace(marker) == "" {
			checks = append(checks, GateCheck{Name: field + ".marker", Status: GateStatusFail, Path: relative, Message: "MAR evidence marker must be non-empty", Hint: "Remove empty markers or record the exact marker text to check.", Field: fmt.Sprintf("%s.markers[%d]", field, i), Expected: "non-empty marker", Actual: "empty"})
			continue
		}
		if strings.Contains(string(content), marker) {
			checks = append(checks, GateCheck{Name: field + ".marker", Status: GateStatusPass, Path: relative, Message: "MAR evidence marker is present", Field: fmt.Sprintf("%s.markers[%d]", field, i), Actual: marker})
		} else {
			checks = append(checks, GateCheck{Name: field + ".marker", Status: GateStatusFail, Path: relative, Message: "MAR evidence marker is missing", Hint: "Record the required marker in the referenced MAR evidence artifact or update the evidence ref.", Field: fmt.Sprintf("%s.markers[%d]", field, i), Expected: marker, Actual: "missing"})
		}
	}
	return checks
}

func validateMultiAgentReviewEvidenceSchema(relative string, content []byte) []SchemaCheck {
	var raw map[string]any
	if err := json.Unmarshal(content, &raw); err != nil || raw == nil {
		actual := "not an object"
		if err != nil {
			actual = err.Error()
		}
		return []SchemaCheck{schemaFail("json", relative, "file is not valid MAR evidence JSON", "Fix the file so it contains one JSON object before validating again.", "json", "JSON object", actual)}
	}
	checks := []SchemaCheck{}
	for _, field := range []string{"schema_version", "run_id", "task_id", "status", "reason", "coverage", "provider_attempts", "blue_disposition_ref"} {
		if _, ok := raw[field]; !ok {
			checks = append(checks, schemaFail(field, relative, "MAR evidence required field is missing", "Use the mar-evidence.v1 schema.", field, "present", "missing"))
		} else {
			checks = append(checks, schemaPass(field, relative, "MAR evidence required field is present"))
		}
	}
	if version, _ := raw["schema_version"].(string); version != multiAgentReviewSchemaVersion {
		actual := version
		if strings.TrimSpace(actual) == "" {
			actual = "missing"
		}
		checks = append(checks, schemaFail("schema_version", relative, "MAR evidence schema version is unsupported", "Use schema_version mar-evidence.v1.", "schema_version", multiAgentReviewSchemaVersion, actual))
	}
	if status, _ := raw["status"].(string); !allowed(status, multiAgentReviewStatuses...) {
		actual := status
		if strings.TrimSpace(actual) == "" {
			actual = "missing"
		}
		checks = append(checks, schemaFail("status", relative, "MAR status vocabulary is unsupported", "Use PASS, PASS_WITH_FINDINGS, REQUEST_CHANGES, BLOCKED, DEGRADED, or FAILED.", "status", strings.Join(multiAgentReviewStatuses, ","), actual))
	} else {
		checks = append(checks, schemaPass("status", relative, "MAR status vocabulary is valid"))
	}
	coverage, _ := raw["coverage"].(map[string]any)
	if coverage == nil {
		checks = append(checks, schemaFail("coverage", relative, "MAR coverage must be an object", "Record role-first coverage under coverage.", "coverage", "object", fmt.Sprintf("%T", raw["coverage"])))
	} else {
		checks = append(checks, requireStringArrayField(relative, coverage, "required_roles"))
		checks = append(checks, requireStringArrayField(relative, coverage, "covered_roles"))
		checks = append(checks, requireObjectField(relative, coverage, "by_role"))
	}
	attempts, ok := raw["provider_attempts"].([]any)
	if !ok {
		checks = append(checks, schemaFail("provider_attempts", relative, "provider_attempts must be an array", "Record KAS MAR provider attempts as an array.", "provider_attempts", "array", fmt.Sprintf("%T", raw["provider_attempts"])))
	} else {
		for i, attempt := range attempts {
			attemptMap, ok := attempt.(map[string]any)
			if !ok {
				checks = append(checks, schemaFail("provider_attempts", relative, "provider attempt must be an object", "Record provider attempt entries as objects.", fmt.Sprintf("provider_attempts[%d]", i), "object", fmt.Sprintf("%T", attempt)))
				continue
			}
			for _, field := range []string{"schema_version", "run_id", "task_id", "attempt_id", "role_id", "provider_id", "provider_candidate", "terminal_status"} {
				value, _ := attemptMap[field].(string)
				if strings.TrimSpace(value) == "" {
					checks = append(checks, schemaFail("provider_attempts", relative, "provider attempt required string field is missing", "Record complete provider attempt identity and status evidence.", fmt.Sprintf("provider_attempts[%d].%s", i, field), "non-empty string", fmt.Sprintf("%v", attemptMap[field])))
				} else {
					checks = append(checks, schemaPass(fmt.Sprintf("provider_attempts[%d].%s", i, field), relative, "provider attempt required string field is present"))
				}
			}
		}
	}
	return checks
}

func multiAgentReviewResultFromChecks(runID string, checks []GateCheck) GateCheckResult {
	status := GateStatusPass
	missing := []string{}
	sawPass := false
	for _, check := range checks {
		switch check.Status {
		case GateStatusFail:
			status = GateStatusFail
			if check.Path != "" {
				missing = appendUnique(missing, check.Path)
			}
		case GateStatusPass:
			sawPass = true
		}
	}
	if status != GateStatusFail && !sawPass {
		status = GateStatusNotApplicable
	}
	return GateCheckResult{RunID: runID, Gate: GateMultiAgentReview, Status: status, Checks: checks, MissingEvidence: missing}
}

func multiAgentReviewTaskID(taskID string) bool {
	id := strings.ToLower(strings.TrimSpace(taskID))
	return strings.HasPrefix(id, "mar-")
}

func multiAgentReviewGateRequired(root Root, metadata RunMetadata) bool {
	_ = root
	if multiAgentReviewTaskID(metadataTaskID(metadata)) {
		return true
	}
	for _, artifact := range metadata.RequiredArtifacts {
		if artifact == multiAgentReviewArtifact {
			return true
		}
	}
	return false
}
