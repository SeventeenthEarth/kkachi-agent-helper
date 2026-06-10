package project

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	token002ApplicabilityRequired      = "required"
	token002ApplicabilityConditional   = "conditional"
	token002ApplicabilityOptional      = "optional"
	token002ApplicabilityNotApplicable = "not_applicable"
)

var token002RequiredFields = []string{
	"schema_version",
	"run_id",
	"task_id",
	"task_class",
	"scope",
	"verification_profile_evidence",
	"phase_packet_evidence",
	"review_bundle_evidence",
	"watcher_evidence",
	"change_verification_matrix_evidence",
	"mutation_approval_evidence",
}

type token002Evidence struct {
	SchemaVersion                    string                              `json:"schema_version"`
	RunID                            string                              `json:"run_id"`
	TaskID                           string                              `json:"task_id"`
	TaskClass                        string                              `json:"task_class"`
	Scope                            *tokenEvidenceSection               `json:"scope"`
	VerificationProfileEvidence      *token002VerificationEvidence       `json:"verification_profile_evidence"`
	PhasePacketEvidence              *token002PhasePacketEvidence        `json:"phase_packet_evidence"`
	ReviewBundleEvidence             *token002ReviewBundleEvidence       `json:"review_bundle_evidence"`
	WatcherEvidence                  *token002WatcherEvidence            `json:"watcher_evidence"`
	ChangeVerificationMatrixEvidence *token002ChangeVerificationEvidence `json:"change_verification_matrix_evidence"`
	MutationApprovalEvidence         *tokenMutationEvidence              `json:"mutation_approval_evidence"`
}

type token002VerificationEvidence struct {
	Status        string                 `json:"status"`
	Reason        string                 `json:"reason,omitempty"`
	SelectedGates []token002SelectedGate `json:"selected_gates,omitempty"`
}

type token002SelectedGate struct {
	SelectedProfileID   string                `json:"selected_profile_id"`
	SelectedGateID      string                `json:"selected_gate_id"`
	GateKind            string                `json:"gate_kind"`
	Command             string                `json:"command"`
	Timeout             string                `json:"timeout"`
	Applicability       string                `json:"applicability"`
	Status              string                `json:"status"`
	NotApplicableReason string                `json:"not_applicable_reason,omitempty"`
	RunnerResult        *token002RunnerResult `json:"runner_result,omitempty"`
}

type token002RunnerResult struct {
	Command                string                    `json:"command"`
	Timeout                string                    `json:"timeout"`
	Applicability          string                    `json:"applicability"`
	Status                 string                    `json:"status"`
	ExitCode               *int                      `json:"exit_code,omitempty"`
	DurationMS             *int                      `json:"duration_ms,omitempty"`
	LogRef                 *tokenEvidenceRef         `json:"log_ref,omitempty"`
	FailureExcerpt         *token002FailureExcerpt   `json:"failure_excerpt,omitempty"`
	DeterministicExtractor *token002ExtractorPosture `json:"deterministic_extractor,omitempty"`
	LikelyFailingTarget    string                    `json:"likely_failing_target,omitempty"`
}

type token002FailureExcerpt struct {
	Text      string `json:"text"`
	LineStart *int   `json:"line_start,omitempty"`
	LineEnd   *int   `json:"line_end,omitempty"`
	Truncated *bool  `json:"truncated,omitempty"`
}

type token002ExtractorPosture struct {
	Extractor string `json:"extractor"`
	Result    string `json:"result"`
}

type token002PhasePacketEvidence struct {
	Status       string                      `json:"status"`
	Reason       string                      `json:"reason,omitempty"`
	PhasePackets []token002PhasePacketRecord `json:"phase_packets,omitempty"`
	RunSummary   *token002RunSummaryRecord   `json:"run_summary,omitempty"`
}

type token002PhasePacketRecord struct {
	PacketVersion         string                         `json:"packet_version"`
	SummaryID             string                         `json:"summary_id"`
	RunID                 string                         `json:"run_id"`
	TaskID                string                         `json:"task_id"`
	PhaseID               string                         `json:"phase_id"`
	Status                string                         `json:"status"`
	PacketValidity        string                         `json:"packet_validity"`
	SourceArtifact        *tokenEvidenceRef              `json:"source_artifact"`
	EvidenceClass         string                         `json:"evidence_class"`
	CompressionPolicy     string                         `json:"compression_policy"`
	ChangedPaths          []string                       `json:"changed_paths,omitempty"`
	VerificationSummary   string                         `json:"verification_summary"`
	BlockerList           []string                       `json:"blocker_list,omitempty"`
	CriticalReferences    []token002CriticalReference    `json:"critical_references"`
	RetrievalInstructions *token002RetrievalInstructions `json:"retrieval_instructions"`
	InvalidationBehavior  *token002InvalidationBehavior  `json:"invalidation_behavior"`
	NextAction            string                         `json:"next_action"`
}

type token002RunSummaryRecord struct {
	SummaryVersion        string                         `json:"summary_version"`
	SummaryID             string                         `json:"summary_id"`
	RunID                 string                         `json:"run_id"`
	TaskID                string                         `json:"task_id"`
	Status                string                         `json:"status"`
	PacketValidity        string                         `json:"packet_validity"`
	PhasePackets          []token002PhasePacketPointer   `json:"phase_packets"`
	ChangedPaths          []string                       `json:"changed_paths,omitempty"`
	VerificationSummary   string                         `json:"verification_summary"`
	BlockerList           []string                       `json:"blocker_list,omitempty"`
	CriticalReferences    []token002CriticalReference    `json:"critical_references"`
	RetrievalInstructions *token002RetrievalInstructions `json:"retrieval_instructions"`
	InvalidationBehavior  *token002InvalidationBehavior  `json:"invalidation_behavior"`
	NextAction            string                         `json:"next_action"`
}

type token002PhasePacketPointer struct {
	PhaseID        string `json:"phase_id"`
	Status         string `json:"status"`
	PacketPath     string `json:"packet_path"`
	PacketChecksum string `json:"packet_checksum"`
}

type token002CriticalReference struct {
	Class                string `json:"class"`
	Path                 string `json:"path"`
	Checksum             string `json:"checksum"`
	RetrievalInstruction string `json:"retrieval_instruction"`
}

type token002RetrievalInstructions struct {
	Default      string   `json:"default"`
	RequiredWhen []string `json:"required_when"`
}

type token002InvalidationBehavior struct {
	InvalidIf []string `json:"invalid_if"`
	OnInvalid string   `json:"on_invalid"`
}

type token002ReviewBundleEvidence struct {
	Status                     string                              `json:"status"`
	Reason                     string                              `json:"reason,omitempty"`
	ReviewBundleVersion        string                              `json:"review_bundle_version"`
	TaskID                     string                              `json:"task_id"`
	RunID                      string                              `json:"run_id"`
	AcceptanceReference        *tokenEvidenceRef                   `json:"acceptance_reference"`
	DiffArtifact               *tokenEvidenceRef                   `json:"diff_artifact"`
	DiffChecksum               string                              `json:"diff_checksum"`
	ChangedPaths               []string                            `json:"changed_paths"`
	VerificationSummaries      []token002ReviewVerificationSummary `json:"verification_summaries"`
	ForbiddenScope             []string                            `json:"forbidden_scope"`
	RequestedVerdictVocabulary []string                            `json:"requested_verdict_vocabulary"`
	RoleVerdicts               []token002RoleVerdict               `json:"role_verdicts"`
	FindingDispositions        []token002FindingDisposition        `json:"finding_dispositions"`
	ArtifactPointers           []tokenEvidenceRef                  `json:"artifact_pointers"`
	BlueSynthesisInputs        map[string]string                   `json:"blue_synthesis_inputs"`
	RetrievalInstructions      *token002RetrievalInstructions      `json:"retrieval_instructions"`
	InvalidationBehavior       *token002InvalidationBehavior       `json:"invalidation_behavior"`
}

type token002ReviewVerificationSummary struct {
	Phase            string `json:"phase"`
	Status           string `json:"status"`
	ArtifactPath     string `json:"artifact_path"`
	ArtifactChecksum string `json:"artifact_checksum"`
}

type token002RoleVerdict struct {
	Role             string `json:"role"`
	Verdict          string `json:"verdict"`
	ArtifactPath     string `json:"artifact_path"`
	ArtifactChecksum string `json:"artifact_checksum"`
}

type token002FindingDisposition struct {
	FindingID        string `json:"finding_id"`
	Disposition      string `json:"disposition"`
	ArtifactPath     string `json:"artifact_path"`
	ArtifactChecksum string `json:"artifact_checksum"`
}

type token002WatcherEvidence struct {
	Status             string             `json:"status"`
	Reason             string             `json:"reason,omitempty"`
	WatcherVersion     string             `json:"watcher_version"`
	TerminalOnly       *bool              `json:"terminal_only"`
	TerminalStatus     string             `json:"terminal_status"`
	MechanicalScope    []string           `json:"mechanical_scope"`
	ArtifactPointers   []tokenEvidenceRef `json:"artifact_pointers"`
	NoReplacementClaim string             `json:"no_replacement_claim"`
}

type token002ChangeVerificationEvidence struct {
	Status                           string                      `json:"status"`
	Reason                           string                      `json:"reason,omitempty"`
	MatrixVersion                    string                      `json:"matrix_version"`
	TaskID                           string                      `json:"task_id"`
	RunID                            string                      `json:"run_id"`
	PolicyOwner                      string                      `json:"policy_owner"`
	VerificationSelectionPolicyOwner string                      `json:"verification_selection_policy_owner"`
	KAHValidationRole                string                      `json:"kah_validation_role"`
	KAHForbiddenDecisions            []string                    `json:"kah_forbidden_decisions"`
	ChangedPathSource                *token002ChangedPathSource  `json:"changed_path_source"`
	ChangedPaths                     []token002ChangedPathRecord `json:"changed_paths"`
	Rules                            []token002ChangeRule        `json:"rules"`
	BoundaryNotes                    []string                    `json:"boundary_notes"`
}

type token002ChangedPathSource struct {
	SourceType     string `json:"source_type"`
	SourceRef      string `json:"source_ref"`
	SourceChecksum string `json:"source_checksum"`
}

type token002ChangedPathRecord struct {
	Path                      string             `json:"path"`
	ChangeClass               string             `json:"change_class"`
	DeterministicEvidenceRefs []tokenEvidenceRef `json:"deterministic_evidence_refs"`
}

type token002ChangeRule struct {
	SelectedRuleID               string                              `json:"selected_rule_id"`
	ChangeClass                  string                              `json:"change_class"`
	ChangedPathSetClasses        []string                            `json:"changed_path_set_classes"`
	SelectedVerificationCommands []token002MatrixVerificationCommand `json:"selected_verification_commands"`
	ScopedVerification           []token002MatrixScopedVerification  `json:"scoped_verification"`
	SkippedGates                 []token002MatrixSkippedGate         `json:"skipped_gates"`
	NoSkippedGatesReason         string                              `json:"no_skipped_gates_reason"`
	FinalAggregatePreservation   *token002FinalAggregatePreservation `json:"final_aggregate_preservation"`
}

type token002MatrixVerificationCommand struct {
	SelectedProfileID string            `json:"selected_profile_id"`
	SelectedGateID    string            `json:"selected_gate_id"`
	Command           string            `json:"command"`
	Timeout           string            `json:"timeout"`
	Applicability     string            `json:"applicability"`
	Status            string            `json:"status"`
	EvidenceRef       *tokenEvidenceRef `json:"evidence_ref"`
}

type token002MatrixScopedVerification struct {
	Command     string            `json:"command"`
	ScopeReason string            `json:"scope_reason"`
	Status      string            `json:"status"`
	EvidenceRef *tokenEvidenceRef `json:"evidence_ref"`
}

type token002MatrixSkippedGate struct {
	SelectedProfileID         string             `json:"selected_profile_id"`
	SelectedGateID            string             `json:"selected_gate_id"`
	SkipReason                string             `json:"skip_reason"`
	DeterministicEvidenceRefs []tokenEvidenceRef `json:"deterministic_evidence_refs"`
}

type token002FinalAggregatePreservation struct {
	Status                    string             `json:"status"`
	NotApplicableReason       string             `json:"not_applicable_reason"`
	DeterministicEvidenceRefs []tokenEvidenceRef `json:"deterministic_evidence_refs"`
}

func validateToken002EconomyEvidence(root Root, metadata RunMetadata, relative string, content []byte, raw map[string]any) []GateCheck {
	checks := []GateCheck{}
	var evidence token002Evidence
	if err := jsonUnmarshalToken002(content, &evidence); err != nil {
		return append(checks, GateCheck{Name: "token_economy_json", Status: GateStatusFail, Path: relative, Message: "token-002 evidence cannot be decoded", Hint: "Use the token002.v1 evidence schema.", Field: "json", Expected: "token002.v1 object", Actual: err.Error()})
	}
	checks = append(checks, validateToken002SchemaFields(relative, raw, evidence)...)
	checks = append(checks, validateToken002Identity(metadata, relative, evidence)...)
	checks = append(checks, validateTokenSection(root, relative, "scope", evidence.Scope, false)...)
	checks = append(checks, validateToken002VerificationEvidence(root, relative, evidence.VerificationProfileEvidence)...)
	checks = append(checks, validateToken002PhasePacketEvidence(root, relative, evidence.PhasePacketEvidence)...)
	checks = append(checks, validateToken002ReviewBundleEvidence(root, relative, evidence.ReviewBundleEvidence)...)
	checks = append(checks, validateToken002WatcherEvidence(root, relative, evidence.WatcherEvidence)...)
	checks = append(checks, validateToken002ChangeVerificationEvidence(root, relative, evidence.ChangeVerificationMatrixEvidence)...)
	checks = append(checks, validateTokenMutationEvidence(root, relative, evidence.MutationApprovalEvidence)...)
	return checks
}

func jsonUnmarshalToken002(content []byte, target any) error {
	return json.Unmarshal(content, target)
}

func validateToken002SchemaFields(relative string, raw map[string]any, evidence token002Evidence) []GateCheck {
	checks := []GateCheck{}
	for _, field := range token002RequiredFields {
		if _, ok := raw[field]; !ok {
			checks = append(checks, GateCheck{Name: "token_required_field", Status: GateStatusFail, Path: relative, Message: "token-002 evidence is missing a required field", Hint: "Write all token002.v1 required fields before checking the gate.", Field: field, Expected: "present", Actual: "missing"})
		}
	}
	if evidence.SchemaVersion == tokenEconomyToken002SchemaVersion {
		checks = append(checks, GateCheck{Name: "schema_version", Status: GateStatusPass, Path: relative, Message: "token-economy schema version is supported", Field: "schema_version", Actual: evidence.SchemaVersion})
	} else {
		actual := evidence.SchemaVersion
		if strings.TrimSpace(actual) == "" {
			actual = "missing"
		}
		checks = append(checks, GateCheck{Name: "schema_version", Status: GateStatusFail, Path: relative, Message: "token-economy schema version is unsupported", Hint: "Use schema_version token002.v1 for token-002 evidence.", Field: "schema_version", Expected: tokenEconomyToken002SchemaVersion, Actual: actual})
	}
	if strings.TrimSpace(evidence.TaskClass) == "" {
		checks = append(checks, GateCheck{Name: "task_class", Status: GateStatusFail, Path: relative, Message: "token-002 evidence is missing task_class", Hint: "Record the deterministic KAS task class in task_class.", Field: "task_class", Expected: "non-empty string", Actual: "missing"})
	} else {
		checks = append(checks, GateCheck{Name: "task_class", Status: GateStatusPass, Path: relative, Message: "task_class is recorded", Field: "task_class", Actual: evidence.TaskClass})
	}
	return checks
}

func validateToken002Identity(metadata RunMetadata, relative string, evidence token002Evidence) []GateCheck {
	checks := []GateCheck{}
	if evidence.RunID == metadata.RunID {
		checks = append(checks, GateCheck{Name: "run_id", Status: GateStatusPass, Path: relative, Message: "run_id matches run metadata", Field: "run_id", Actual: evidence.RunID})
	} else {
		checks = append(checks, GateCheck{Name: "run_id", Status: GateStatusFail, Path: relative, Message: "run_id does not match run metadata", Hint: "Record the current run id in token-economy-evidence.json.", Field: "run_id", Expected: metadata.RunID, Actual: evidence.RunID})
	}
	if evidence.TaskID == tokenEconomyToken002TaskID && metadataTaskID(metadata) == tokenEconomyToken002TaskID {
		checks = append(checks, GateCheck{Name: "task_id", Status: GateStatusPass, Path: relative, Message: "task_id is token-002", Field: "task_id", Actual: evidence.TaskID})
	} else {
		actual := evidence.TaskID
		if strings.TrimSpace(actual) == "" {
			actual = "missing"
		}
		checks = append(checks, GateCheck{Name: "task_id", Status: GateStatusFail, Path: relative, Message: "task_id is not token-002", Hint: "The token-002 evidence schema is valid only for token-002 runs.", Field: "task_id", Expected: tokenEconomyToken002TaskID, Actual: actual})
	}
	return checks
}

func validateToken002VerificationEvidence(root Root, relative string, evidence *token002VerificationEvidence) []GateCheck {
	if evidence == nil {
		return missingToken002Section(relative, "verification_profile_evidence")
	}
	checks := []GateCheck{validateToken002Status(relative, "verification_profile_evidence", evidence.Status, evidence.Reason)}
	if token002SectionNotApplicableOrInvalid(evidence.Status) {
		return checks
	}
	if len(evidence.SelectedGates) == 0 {
		checks = append(checks, token002Fail(relative, "verification_profile_evidence.selected_gates", "verification profile evidence lacks selected gate records", "Record one or more KAS-selected gate records.", "verification_profile_evidence.selected_gates", "one or more records", "missing"))
	}
	for i, gate := range evidence.SelectedGates {
		field := fmt.Sprintf("verification_profile_evidence.selected_gates[%d]", i)
		checks = append(checks, requireToken002String(relative, field+".selected_profile_id", gate.SelectedProfileID)...)
		checks = append(checks, requireToken002String(relative, field+".selected_gate_id", gate.SelectedGateID)...)
		checks = append(checks, requireToken002String(relative, field+".gate_kind", gate.GateKind)...)
		checks = append(checks, validateToken002Applicability(relative, field+".applicability", gate.Applicability)...)
		checks = append(checks, validateToken002ResultVocabulary(relative, field+".status", gate.Status)...)
		if gate.Applicability == token002ApplicabilityNotApplicable || gate.Status == GateStatusNotApplicable {
			if strings.TrimSpace(gate.NotApplicableReason) == "" {
				checks = append(checks, token002Fail(relative, field+".not_applicable_reason", "not_applicable selected gate requires a reason", "Record the deterministic KAS reason instead of fake runner fields.", field+".not_applicable_reason", "non-empty reason", "missing"))
			}
			if gate.RunnerResult != nil || strings.TrimSpace(gate.Command) != "" || strings.TrimSpace(gate.Timeout) != "" {
				checks = append(checks, token002Fail(relative, field, "not_applicable selected gate must not fake runner fields", "Leave command, timeout, and runner_result empty when the gate is not applicable.", field, "selected ids plus reason only", "execution fields present"))
			}
			continue
		}
		checks = append(checks, requireToken002String(relative, field+".command", gate.Command)...)
		checks = append(checks, requireToken002String(relative, field+".timeout", gate.Timeout)...)
		if gate.RunnerResult == nil {
			checks = append(checks, token002Fail(relative, field+".runner_result", "selected gate lacks no-agent runner result", "Record compact runner result fields plus full log artifact pointer.", field+".runner_result", "object", "missing"))
			continue
		}
		checks = append(checks, validateToken002RunnerResult(root, relative, field+".runner_result", *gate.RunnerResult, gate.Status)...)
	}
	return checks
}

func validateToken002RunnerResult(root Root, relative string, field string, result token002RunnerResult, selectedStatus string) []GateCheck {
	checks := []GateCheck{}
	checks = append(checks, requireToken002String(relative, field+".command", result.Command)...)
	checks = append(checks, requireToken002String(relative, field+".timeout", result.Timeout)...)
	checks = append(checks, validateToken002Applicability(relative, field+".applicability", result.Applicability)...)
	checks = append(checks, validateToken002ResultVocabulary(relative, field+".status", result.Status)...)
	if result.Status != selectedStatus {
		checks = append(checks, token002Fail(relative, field+".status", "runner result status does not match selected gate status", "Keep selected gate and no-agent runner status coherent.", field+".status", selectedStatus, result.Status))
	}
	if result.Status == GateStatusNotApplicable {
		return checks
	}
	if result.ExitCode == nil {
		checks = append(checks, token002Fail(relative, field+".exit_code", "runner result lacks exit code", "Record the no-agent runner process exit code.", field+".exit_code", "integer", "missing"))
	}
	if result.DurationMS == nil || result.DurationMS != nil && *result.DurationMS < 0 {
		actual := "missing"
		if result.DurationMS != nil {
			actual = fmt.Sprintf("%d", *result.DurationMS)
		}
		checks = append(checks, token002Fail(relative, field+".duration_ms", "runner result duration is missing or invalid", "Record non-negative no-agent runner duration_ms.", field+".duration_ms", "non-negative integer", actual))
	}
	checks = append(checks, validateRequiredToken002Ref(root, relative, field+".log_ref", result.LogRef)...)
	if result.Status == GateStatusFail {
		if result.FailureExcerpt == nil {
			checks = append(checks, token002Fail(relative, field+".failure_excerpt", "failed runner result lacks bounded failure excerpt", "Record a bounded model-visible excerpt while preserving the full log pointer.", field+".failure_excerpt", "object", "missing"))
		} else {
			checks = append(checks, validateToken002FailureExcerpt(relative, field+".failure_excerpt", *result.FailureExcerpt)...)
		}
		if result.DeterministicExtractor == nil {
			checks = append(checks, token002Fail(relative, field+".deterministic_extractor", "failed runner result lacks deterministic extractor posture", "Record which deterministic extractor ran and its result.", field+".deterministic_extractor", "object", "missing"))
		} else {
			checks = append(checks, requireToken002String(relative, field+".deterministic_extractor.extractor", result.DeterministicExtractor.Extractor)...)
			if !allowed(result.DeterministicExtractor.Result, "extracted", "not_found", "not_applicable") {
				checks = append(checks, token002Fail(relative, field+".deterministic_extractor.result", "deterministic extractor result vocabulary is unsupported", "Use extracted, not_found, or not_applicable.", field+".deterministic_extractor.result", "extracted,not_found,not_applicable", result.DeterministicExtractor.Result))
			}
		}
	}
	return checks
}

func validateToken002FailureExcerpt(relative string, field string, excerpt token002FailureExcerpt) []GateCheck {
	checks := []GateCheck{}
	text := strings.TrimSpace(excerpt.Text)
	if text == "" || len(text) > 4000 {
		actual := "missing"
		if text != "" {
			actual = fmt.Sprintf("%d bytes", len(text))
		}
		checks = append(checks, token002Fail(relative, field+".text", "failure excerpt is missing or unbounded", "Record a non-empty bounded excerpt no larger than 4000 bytes.", field+".text", "1..4000 bytes", actual))
	}
	if excerpt.LineStart == nil || excerpt.LineEnd == nil || excerpt.LineStart != nil && excerpt.LineEnd != nil && *excerpt.LineStart > *excerpt.LineEnd {
		checks = append(checks, token002Fail(relative, field, "failure excerpt lacks valid line bounds", "Record line_start and line_end with line_start <= line_end.", field, "valid line_start/line_end", "missing or invalid"))
	}
	if excerpt.Truncated == nil {
		checks = append(checks, token002Fail(relative, field+".truncated", "failure excerpt lacks truncation posture", "Record whether the excerpt was truncated.", field+".truncated", "boolean", "missing"))
	}
	return checks
}

func validateToken002PhasePacketEvidence(root Root, relative string, evidence *token002PhasePacketEvidence) []GateCheck {
	if evidence == nil {
		return missingToken002Section(relative, "phase_packet_evidence")
	}
	checks := []GateCheck{validateToken002Status(relative, "phase_packet_evidence", evidence.Status, evidence.Reason)}
	if token002SectionNotApplicableOrInvalid(evidence.Status) {
		return checks
	}
	if len(evidence.PhasePackets) == 0 {
		checks = append(checks, token002Fail(relative, "phase_packet_evidence.phase_packets", "phase packet evidence lacks packet records", "Record one or more normalized token008 phase packet records.", "phase_packet_evidence.phase_packets", "one or more records", "missing"))
	}
	for i, packet := range evidence.PhasePackets {
		checks = append(checks, validateToken002PhasePacketRecord(root, relative, fmt.Sprintf("phase_packet_evidence.phase_packets[%d]", i), packet)...)
	}
	if evidence.RunSummary == nil {
		checks = append(checks, token002Fail(relative, "phase_packet_evidence.run_summary", "phase packet evidence lacks run summary record", "Record normalized token008 run-summary evidence.", "phase_packet_evidence.run_summary", "object", "missing"))
	} else {
		checks = append(checks, validateToken002RunSummary(root, relative, "phase_packet_evidence.run_summary", *evidence.RunSummary)...)
	}
	return checks
}

func validateToken002PhasePacketRecord(root Root, relative string, field string, packet token002PhasePacketRecord) []GateCheck {
	checks := []GateCheck{}
	checks = append(checks, requireExactToken002String(relative, field+".packet_version", packet.PacketVersion, "token008.v1")...)
	checks = append(checks, requireToken002String(relative, field+".summary_id", packet.SummaryID)...)
	checks = append(checks, requireToken002String(relative, field+".run_id", packet.RunID)...)
	checks = append(checks, requireExactToken002String(relative, field+".task_id", packet.TaskID, tokenEconomyToken002TaskID)...)
	checks = append(checks, requireToken002String(relative, field+".phase_id", packet.PhaseID)...)
	checks = append(checks, validateToken002PhaseStatus(relative, field+".status", packet.Status)...)
	checks = append(checks, validateToken002CurrentValidity(relative, field+".packet_validity", packet.PacketValidity)...)
	checks = append(checks, validateRequiredToken002Ref(root, relative, field+".source_artifact", packet.SourceArtifact)...)
	checks = append(checks, validateToken002EvidenceClass(relative, field+".evidence_class", packet.EvidenceClass)...)
	checks = append(checks, validateToken002CompressionPolicy(relative, field+".compression_policy", packet.CompressionPolicy)...)
	checks = append(checks, requireToken002String(relative, field+".verification_summary", packet.VerificationSummary)...)
	checks = append(checks, validateToken002CriticalReferences(root, relative, field+".critical_references", packet.CriticalReferences)...)
	checks = append(checks, validateToken002Retrieval(relative, field+".retrieval_instructions", packet.RetrievalInstructions)...)
	checks = append(checks, validateToken002Invalidation(relative, field+".invalidation_behavior", packet.InvalidationBehavior)...)
	checks = append(checks, requireToken002String(relative, field+".next_action", packet.NextAction)...)
	return checks
}

func validateToken002RunSummary(root Root, relative string, field string, summary token002RunSummaryRecord) []GateCheck {
	checks := []GateCheck{}
	checks = append(checks, requireExactToken002String(relative, field+".summary_version", summary.SummaryVersion, "token008.v1")...)
	checks = append(checks, requireToken002String(relative, field+".summary_id", summary.SummaryID)...)
	checks = append(checks, requireToken002String(relative, field+".run_id", summary.RunID)...)
	checks = append(checks, requireExactToken002String(relative, field+".task_id", summary.TaskID, tokenEconomyToken002TaskID)...)
	checks = append(checks, validateToken002PhaseStatus(relative, field+".status", summary.Status)...)
	checks = append(checks, validateToken002CurrentValidity(relative, field+".packet_validity", summary.PacketValidity)...)
	if len(summary.PhasePackets) == 0 {
		checks = append(checks, token002Fail(relative, field+".phase_packets", "run summary lacks phase packet pointers", "Record phase packet path/checksum pointers.", field+".phase_packets", "one or more pointers", "missing"))
	}
	for i, packet := range summary.PhasePackets {
		item := fmt.Sprintf("%s.phase_packets[%d]", field, i)
		checks = append(checks, requireToken002String(relative, item+".phase_id", packet.PhaseID)...)
		checks = append(checks, validateToken002PhaseStatus(relative, item+".status", packet.Status)...)
		checks = append(checks, validateToken002RefParts(root, relative, item, packet.PacketPath, packet.PacketChecksum)...)
	}
	checks = append(checks, requireToken002String(relative, field+".verification_summary", summary.VerificationSummary)...)
	checks = append(checks, validateToken002CriticalReferences(root, relative, field+".critical_references", summary.CriticalReferences)...)
	checks = append(checks, validateToken002Retrieval(relative, field+".retrieval_instructions", summary.RetrievalInstructions)...)
	checks = append(checks, validateToken002Invalidation(relative, field+".invalidation_behavior", summary.InvalidationBehavior)...)
	checks = append(checks, requireToken002String(relative, field+".next_action", summary.NextAction)...)
	return checks
}

func validateToken002ReviewBundleEvidence(root Root, relative string, evidence *token002ReviewBundleEvidence) []GateCheck {
	if evidence == nil {
		return missingToken002Section(relative, "review_bundle_evidence")
	}
	checks := []GateCheck{validateToken002Status(relative, "review_bundle_evidence", evidence.Status, evidence.Reason)}
	if token002SectionNotApplicableOrInvalid(evidence.Status) {
		return checks
	}
	checks = append(checks, requireExactToken002String(relative, "review_bundle_evidence.review_bundle_version", evidence.ReviewBundleVersion, "token009.v1")...)
	checks = append(checks, requireExactToken002String(relative, "review_bundle_evidence.task_id", evidence.TaskID, tokenEconomyToken002TaskID)...)
	checks = append(checks, requireToken002String(relative, "review_bundle_evidence.run_id", evidence.RunID)...)
	checks = append(checks, validateRequiredToken002Ref(root, relative, "review_bundle_evidence.acceptance_reference", evidence.AcceptanceReference)...)
	checks = append(checks, validateRequiredToken002Ref(root, relative, "review_bundle_evidence.diff_artifact", evidence.DiffArtifact)...)
	checks = append(checks, validateToken002Checksum(relative, "review_bundle_evidence.diff_checksum", evidence.DiffChecksum)...)
	checks = append(checks, validateToken002ChangedPaths(root, relative, "review_bundle_evidence.changed_paths", evidence.ChangedPaths)...)
	if len(evidence.VerificationSummaries) == 0 {
		checks = append(checks, token002Fail(relative, "review_bundle_evidence.verification_summaries", "review bundle lacks verification summaries", "Record compact verification summary refs.", "review_bundle_evidence.verification_summaries", "one or more records", "missing"))
	}
	for i, summary := range evidence.VerificationSummaries {
		field := fmt.Sprintf("review_bundle_evidence.verification_summaries[%d]", i)
		checks = append(checks, requireToken002String(relative, field+".phase", summary.Phase)...)
		checks = append(checks, validateToken002PhaseStatus(relative, field+".status", summary.Status)...)
		checks = append(checks, validateToken002RefParts(root, relative, field, summary.ArtifactPath, summary.ArtifactChecksum)...)
	}
	checks = append(checks, requireToken002StringSlice(relative, "review_bundle_evidence.forbidden_scope", evidence.ForbiddenScope)...)
	checks = append(checks, validateToken002RequestedVerdicts(relative, evidence.RequestedVerdictVocabulary)...)
	if len(evidence.RoleVerdicts) == 0 {
		checks = append(checks, token002Fail(relative, "review_bundle_evidence.role_verdicts", "review bundle lacks role verdicts", "Record requested role verdict refs.", "review_bundle_evidence.role_verdicts", "one or more records", "missing"))
	}
	for i, verdict := range evidence.RoleVerdicts {
		field := fmt.Sprintf("review_bundle_evidence.role_verdicts[%d]", i)
		checks = append(checks, requireToken002String(relative, field+".role", verdict.Role)...)
		if !allowed(verdict.Verdict, "accepted", "accepted_with_required_rework", "rejected", "blocked") {
			checks = append(checks, token002Fail(relative, field+".verdict", "role verdict vocabulary is unsupported", "Use the requested review verdict vocabulary.", field+".verdict", "accepted,accepted_with_required_rework,rejected,blocked", verdict.Verdict))
		}
		checks = append(checks, validateToken002RefParts(root, relative, field, verdict.ArtifactPath, verdict.ArtifactChecksum)...)
	}
	if len(evidence.FindingDispositions) == 0 {
		checks = append(checks, token002Fail(relative, "review_bundle_evidence.finding_dispositions", "review bundle lacks finding dispositions", "Record finding disposition refs.", "review_bundle_evidence.finding_dispositions", "one or more records", "missing"))
	}
	for i, disposition := range evidence.FindingDispositions {
		field := fmt.Sprintf("review_bundle_evidence.finding_dispositions[%d]", i)
		checks = append(checks, requireToken002String(relative, field+".finding_id", disposition.FindingID)...)
		if !allowed(disposition.Disposition, "accepted", "rejected", "blocked", "deferred_with_owner") {
			checks = append(checks, token002Fail(relative, field+".disposition", "finding disposition vocabulary is unsupported", "Use accepted, rejected, blocked, or deferred_with_owner.", field+".disposition", "accepted,rejected,blocked,deferred_with_owner", disposition.Disposition))
		}
		checks = append(checks, validateToken002RefParts(root, relative, field, disposition.ArtifactPath, disposition.ArtifactChecksum)...)
	}
	checks = append(checks, validateToken002RefList(root, relative, "review_bundle_evidence.artifact_pointers", evidence.ArtifactPointers)...)
	if len(evidence.BlueSynthesisInputs) == 0 {
		checks = append(checks, token002Fail(relative, "review_bundle_evidence.blue_synthesis_inputs", "review bundle lacks blue synthesis inputs", "Record compact lane verdict/disposition/pointer inputs.", "review_bundle_evidence.blue_synthesis_inputs", "object", "missing"))
	}
	checks = append(checks, validateToken002Retrieval(relative, "review_bundle_evidence.retrieval_instructions", evidence.RetrievalInstructions)...)
	checks = append(checks, validateToken002Invalidation(relative, "review_bundle_evidence.invalidation_behavior", evidence.InvalidationBehavior)...)
	return checks
}

func validateToken002WatcherEvidence(root Root, relative string, evidence *token002WatcherEvidence) []GateCheck {
	if evidence == nil {
		return missingToken002Section(relative, "watcher_evidence")
	}
	checks := []GateCheck{validateToken002Status(relative, "watcher_evidence", evidence.Status, evidence.Reason)}
	if token002SectionNotApplicableOrInvalid(evidence.Status) {
		return checks
	}
	checks = append(checks, requireExactToken002String(relative, "watcher_evidence.watcher_version", evidence.WatcherVersion, "token009.v1")...)
	if evidence.TerminalOnly == nil || !*evidence.TerminalOnly {
		actual := "missing"
		if evidence.TerminalOnly != nil {
			actual = "false"
		}
		checks = append(checks, token002Fail(relative, "watcher_evidence.terminal_only", "watcher report must be terminal-only", "Record terminal_only true; KAH does not operate watcher policy.", "watcher_evidence.terminal_only", "true", actual))
	}
	if !allowed(evidence.TerminalStatus, "complete", "failed", "blocked", "not_applicable") {
		checks = append(checks, token002Fail(relative, "watcher_evidence.terminal_status", "watcher terminal status vocabulary is unsupported", "Use complete, failed, blocked, or not_applicable.", "watcher_evidence.terminal_status", "complete,failed,blocked,not_applicable", evidence.TerminalStatus))
	}
	if len(evidence.MechanicalScope) == 0 {
		checks = append(checks, token002Fail(relative, "watcher_evidence.mechanical_scope", "watcher evidence lacks mechanical scope", "Record mechanically checkable watcher scopes only.", "watcher_evidence.mechanical_scope", "one or more scopes", "missing"))
	}
	for i, scope := range evidence.MechanicalScope {
		if !allowed(scope, "artifact_existence", "artifact_checksum", "role_verdict_presence", "finding_disposition_presence") {
			checks = append(checks, token002Fail(relative, fmt.Sprintf("watcher_evidence.mechanical_scope[%d]", i), "watcher mechanical scope vocabulary is unsupported", "Use KAS TOKEN-009 mechanical scopes only.", fmt.Sprintf("watcher_evidence.mechanical_scope[%d]", i), "artifact_existence,artifact_checksum,role_verdict_presence,finding_disposition_presence", scope))
		}
	}
	checks = append(checks, validateToken002RefList(root, relative, "watcher_evidence.artifact_pointers", evidence.ArtifactPointers)...)
	claim := strings.ToLower(evidence.NoReplacementClaim)
	for _, required := range []string{"kanban", "kah evidence", "review cards", "blue synthesis"} {
		if !strings.Contains(claim, required) {
			checks = append(checks, token002Fail(relative, "watcher_evidence.no_replacement_claim", "watcher evidence lacks explicit no-replacement claim", "State that watcher reports do not replace Kanban, KAH evidence, review cards, or Blue synthesis.", "watcher_evidence.no_replacement_claim", "mentions Kanban, KAH evidence, review cards, Blue synthesis", evidence.NoReplacementClaim))
			break
		}
	}
	return checks
}

func validateToken002ChangeVerificationEvidence(root Root, relative string, evidence *token002ChangeVerificationEvidence) []GateCheck {
	if evidence == nil {
		return missingToken002Section(relative, "change_verification_matrix_evidence")
	}
	checks := []GateCheck{validateToken002Status(relative, "change_verification_matrix_evidence", evidence.Status, evidence.Reason)}
	if token002SectionNotApplicableOrInvalid(evidence.Status) {
		return checks
	}
	checks = append(checks, requireExactToken002String(relative, "change_verification_matrix_evidence.matrix_version", evidence.MatrixVersion, "token010.v1")...)
	checks = append(checks, requireExactToken002String(relative, "change_verification_matrix_evidence.task_id", evidence.TaskID, tokenEconomyToken002TaskID)...)
	checks = append(checks, requireToken002String(relative, "change_verification_matrix_evidence.run_id", evidence.RunID)...)
	checks = append(checks, requireExactToken002String(relative, "change_verification_matrix_evidence.policy_owner", evidence.PolicyOwner, "KAS")...)
	checks = append(checks, requireExactToken002String(relative, "change_verification_matrix_evidence.verification_selection_policy_owner", evidence.VerificationSelectionPolicyOwner, "KAS")...)
	checks = append(checks, requireExactToken002String(relative, "change_verification_matrix_evidence.kah_validation_role", evidence.KAHValidationRole, "mechanical_recorded_evidence_only")...)
	checks = append(checks, requireContainsAll(relative, "change_verification_matrix_evidence.kah_forbidden_decisions", evidence.KAHForbiddenDecisions, []string{"decide skip policy", "choose tests to skip", "infer skips from file extensions"})...)
	if evidence.ChangedPathSource == nil {
		checks = append(checks, token002Fail(relative, "change_verification_matrix_evidence.changed_path_source", "change matrix lacks changed path source", "Record source type, ref, and checksum.", "change_verification_matrix_evidence.changed_path_source", "object", "missing"))
	} else {
		if !allowed(evidence.ChangedPathSource.SourceType, "git diff", "git status", "artifact manifest", "review artifact", "not_applicable") {
			checks = append(checks, token002Fail(relative, "change_verification_matrix_evidence.changed_path_source.source_type", "changed path source vocabulary is unsupported", "Use the TOKEN-010 source_type vocabulary.", "change_verification_matrix_evidence.changed_path_source.source_type", "git diff,git status,artifact manifest,review artifact,not_applicable", evidence.ChangedPathSource.SourceType))
		}
		checks = append(checks, validateToken002RefParts(root, relative, "change_verification_matrix_evidence.changed_path_source", evidence.ChangedPathSource.SourceRef, evidence.ChangedPathSource.SourceChecksum)...)
	}
	if len(evidence.ChangedPaths) == 0 {
		checks = append(checks, token002Fail(relative, "change_verification_matrix_evidence.changed_paths", "change matrix lacks changed path records", "Record changed path classifications, including no-change/not_applicable records when applicable.", "change_verification_matrix_evidence.changed_paths", "one or more records", "missing"))
	}
	for i, changed := range evidence.ChangedPaths {
		field := fmt.Sprintf("change_verification_matrix_evidence.changed_paths[%d]", i)
		checks = append(checks, validateToken002SafePath(root, relative, field+".path", changed.Path, true)...)
		checks = append(checks, validateToken002ChangeClass(relative, field+".change_class", changed.ChangeClass, false)...)
		checks = append(checks, validateToken002RefList(root, relative, field+".deterministic_evidence_refs", changed.DeterministicEvidenceRefs)...)
	}
	if len(evidence.Rules) == 0 {
		checks = append(checks, token002Fail(relative, "change_verification_matrix_evidence.rules", "change matrix lacks verification rules", "Record KAS-selected verification matrix rules.", "change_verification_matrix_evidence.rules", "one or more rules", "missing"))
	}
	for i, rule := range evidence.Rules {
		checks = append(checks, validateToken002ChangeRule(root, relative, fmt.Sprintf("change_verification_matrix_evidence.rules[%d]", i), rule)...)
	}
	checks = append(checks, requireContainsAll(relative, "change_verification_matrix_evidence.boundary_notes", evidence.BoundaryNotes, []string{"KAS owns verification-selection policy.", "KAH validates recorded deterministic evidence only.", "KAH must not decide skip policy or choose tests to skip."})...)
	return checks
}

func validateToken002ChangeRule(root Root, relative string, field string, rule token002ChangeRule) []GateCheck {
	checks := []GateCheck{}
	checks = append(checks, requireToken002String(relative, field+".selected_rule_id", rule.SelectedRuleID)...)
	checks = append(checks, validateToken002ChangeClass(relative, field+".change_class", rule.ChangeClass, true)...)
	if len(rule.ChangedPathSetClasses) == 0 {
		checks = append(checks, token002Fail(relative, field+".changed_path_set_classes", "change rule lacks changed path set classes", "Record every changed path class covered by the selected rule.", field+".changed_path_set_classes", "one or more classes", "missing"))
	}
	for i, class := range rule.ChangedPathSetClasses {
		checks = append(checks, validateToken002ChangeClass(relative, fmt.Sprintf("%s.changed_path_set_classes[%d]", field, i), class, false)...)
	}
	if len(rule.SelectedVerificationCommands) == 0 {
		checks = append(checks, token002Fail(relative, field+".selected_verification_commands", "change rule lacks selected verification commands", "Record selected/scoped/not_applicable KAS verification commands.", field+".selected_verification_commands", "one or more records", "missing"))
	}
	for i, command := range rule.SelectedVerificationCommands {
		item := fmt.Sprintf("%s.selected_verification_commands[%d]", field, i)
		checks = append(checks, requireToken002String(relative, item+".selected_profile_id", command.SelectedProfileID)...)
		checks = append(checks, requireToken002String(relative, item+".selected_gate_id", command.SelectedGateID)...)
		checks = append(checks, validateToken002Applicability(relative, item+".applicability", command.Applicability)...)
		checks = append(checks, validateToken002ResultVocabulary(relative, item+".status", command.Status)...)
		if command.Status == GateStatusNotApplicable || command.Applicability == token002ApplicabilityNotApplicable {
			continue
		}
		checks = append(checks, requireToken002String(relative, item+".command", command.Command)...)
		checks = append(checks, requireToken002String(relative, item+".timeout", command.Timeout)...)
		checks = append(checks, validateRequiredToken002Ref(root, relative, item+".evidence_ref", command.EvidenceRef)...)
	}
	if len(rule.ScopedVerification) == 0 {
		checks = append(checks, token002Fail(relative, field+".scoped_verification", "change rule lacks scoped verification records", "Record scoped verification or explicit not_applicable records.", field+".scoped_verification", "one or more records", "missing"))
	}
	for i, scoped := range rule.ScopedVerification {
		item := fmt.Sprintf("%s.scoped_verification[%d]", field, i)
		checks = append(checks, validateToken002ResultVocabulary(relative, item+".status", scoped.Status)...)
		if scoped.Status == GateStatusNotApplicable {
			checks = append(checks, requireToken002String(relative, item+".scope_reason", scoped.ScopeReason)...)
			continue
		}
		checks = append(checks, requireToken002String(relative, item+".command", scoped.Command)...)
		checks = append(checks, requireToken002String(relative, item+".scope_reason", scoped.ScopeReason)...)
		checks = append(checks, validateRequiredToken002Ref(root, relative, item+".evidence_ref", scoped.EvidenceRef)...)
	}
	if len(rule.SkippedGates) == 0 {
		checks = append(checks, requireToken002String(relative, field+".no_skipped_gates_reason", rule.NoSkippedGatesReason)...)
	} else {
		for i, skipped := range rule.SkippedGates {
			item := fmt.Sprintf("%s.skipped_gates[%d]", field, i)
			checks = append(checks, requireToken002String(relative, item+".selected_profile_id", skipped.SelectedProfileID)...)
			checks = append(checks, requireToken002String(relative, item+".selected_gate_id", skipped.SelectedGateID)...)
			checks = append(checks, requireToken002String(relative, item+".skip_reason", skipped.SkipReason)...)
			checks = append(checks, validateToken002RefList(root, relative, item+".deterministic_evidence_refs", skipped.DeterministicEvidenceRefs)...)
		}
	}
	if rule.FinalAggregatePreservation == nil {
		checks = append(checks, token002Fail(relative, field+".final_aggregate_preservation", "change rule lacks final aggregate preservation evidence", "Record final aggregate preservation status and evidence refs.", field+".final_aggregate_preservation", "object", "missing"))
	} else {
		checks = append(checks, validateToken002FinalAggregate(root, relative, field+".final_aggregate_preservation", *rule.FinalAggregatePreservation)...)
	}
	return checks
}

func validateToken002FinalAggregate(root Root, relative string, field string, final token002FinalAggregatePreservation) []GateCheck {
	checks := []GateCheck{}
	if !allowed(final.Status, "preserved", "required", "not_applicable") {
		checks = append(checks, token002Fail(relative, field+".status", "final aggregate preservation status is unsupported", "Use preserved, required, or not_applicable.", field+".status", "preserved,required,not_applicable", final.Status))
	}
	if final.Status == GateStatusNotApplicable && strings.TrimSpace(final.NotApplicableReason) == "" {
		checks = append(checks, token002Fail(relative, field+".not_applicable_reason", "not_applicable final aggregate preservation requires a reason", "Record task contract or approval-backed deterministic reason.", field+".not_applicable_reason", "non-empty reason", "missing"))
	}
	checks = append(checks, validateToken002RefList(root, relative, field+".deterministic_evidence_refs", final.DeterministicEvidenceRefs)...)
	return checks
}

func validateToken002Status(relative, name, status, reason string) GateCheck {
	status = strings.TrimSpace(status)
	switch status {
	case GateStatusPass:
		return GateCheck{Name: name, Status: GateStatusPass, Path: relative, Message: "token-002 evidence section status is pass", Field: name + ".status", Actual: status}
	case GateStatusFail:
		return GateCheck{Name: name, Status: GateStatusFail, Message: "token-002 evidence section records fail", Field: name + ".status", Actual: status}
	case GateStatusNotApplicable:
		if strings.TrimSpace(reason) == "" {
			return GateCheck{Name: name, Status: GateStatusFail, Path: relative, Message: "not_applicable token-002 evidence requires a reason", Hint: "Record a deterministic reason for every not_applicable token-002 section.", Field: name + ".reason", Expected: "non-empty reason", Actual: "missing"}
		}
		return GateCheck{Name: name, Status: GateStatusNotApplicable, Path: relative, Message: "token-002 evidence section is not applicable with a reason", Field: name + ".reason", Actual: strings.TrimSpace(reason)}
	default:
		actual := status
		if actual == "" {
			actual = "missing"
		}
		return GateCheck{Name: name, Status: GateStatusFail, Path: relative, Message: "token-002 evidence status vocabulary is unsupported", Hint: "Use pass, fail, or not_applicable for token-002 evidence.", Field: name + ".status", Expected: "pass,fail,not_applicable", Actual: actual}
	}
}

func token002SectionNotApplicableOrInvalid(status string) bool {
	return status == GateStatusNotApplicable || !allowed(status, GateStatusPass, GateStatusFail)
}

func validateToken002ResultVocabulary(relative, field, status string) []GateCheck {
	if allowed(status, GateStatusPass, GateStatusFail, GateStatusNotApplicable) {
		return []GateCheck{{Name: field, Status: GateStatusPass, Path: relative, Message: "token-002 result status vocabulary is supported", Field: field, Actual: status}}
	}
	actual := status
	if strings.TrimSpace(actual) == "" {
		actual = "missing"
	}
	return []GateCheck{token002Fail(relative, field, "token-002 result status vocabulary is unsupported", "Use pass, fail, or not_applicable. Warning-only statuses are not supported.", field, "pass,fail,not_applicable", actual)}
}

func validateToken002Applicability(relative, field, value string) []GateCheck {
	if allowed(value, token002ApplicabilityRequired, token002ApplicabilityConditional, token002ApplicabilityOptional, token002ApplicabilityNotApplicable) {
		return []GateCheck{{Name: field, Status: GateStatusPass, Path: relative, Message: "token-002 applicability vocabulary is supported", Field: field, Actual: value}}
	}
	actual := value
	if strings.TrimSpace(actual) == "" {
		actual = "missing"
	}
	return []GateCheck{token002Fail(relative, field, "token-002 applicability vocabulary is unsupported", "Use required, conditional, optional, or not_applicable.", field, "required,conditional,optional,not_applicable", actual)}
}

func validateToken002PhaseStatus(relative, field, status string) []GateCheck {
	if allowed(status, GateStatusPass, GateStatusFail, "blocked", "in_progress", GateStatusNotApplicable) {
		return []GateCheck{{Name: field, Status: GateStatusPass, Path: relative, Message: "token-002 phase status vocabulary is supported", Field: field, Actual: status}}
	}
	actual := status
	if strings.TrimSpace(actual) == "" {
		actual = "missing"
	}
	return []GateCheck{token002Fail(relative, field, "token-002 phase status vocabulary is unsupported", "Use pass, fail, blocked, in_progress, or not_applicable.", field, "pass,fail,blocked,in_progress,not_applicable", actual)}
}

func validateToken002CurrentValidity(relative, field, validity string) []GateCheck {
	if validity == "current" {
		return []GateCheck{{Name: field, Status: GateStatusPass, Path: relative, Message: "token-002 packet validity is current", Field: field, Actual: validity}}
	}
	actual := validity
	if strings.TrimSpace(actual) == "" {
		actual = "missing"
	}
	return []GateCheck{token002Fail(relative, field, "token-002 evidence validity is not current", "Regenerate stale, invalid, or superseded evidence before using it for the gate.", field, "current", actual)}
}

func validateToken002EvidenceClass(relative, field, value string) []GateCheck {
	if allowed(value, "acceptance_criteria", "approval", "forbidden_scope", "failure", "review_finding", "gate_failure", "verification_log", "status_readback") {
		return []GateCheck{{Name: field, Status: GateStatusPass, Path: relative, Message: "token-002 evidence class vocabulary is supported", Field: field, Actual: value}}
	}
	return []GateCheck{token002Fail(relative, field, "token-002 evidence class vocabulary is unsupported", "Use the KAS-declared token008 evidence class vocabulary.", field, "known token008 evidence class", value)}
}

func validateToken002CompressionPolicy(relative, field, value string) []GateCheck {
	if allowed(value, "no_compression", "direct_reference_only", "summary_with_pointer", "summary_safe") {
		return []GateCheck{{Name: field, Status: GateStatusPass, Path: relative, Message: "token-002 compression policy vocabulary is supported", Field: field, Actual: value}}
	}
	return []GateCheck{token002Fail(relative, field, "token-002 compression policy vocabulary is unsupported", "Use no_compression, direct_reference_only, summary_with_pointer, or summary_safe.", field, "known token008 compression policy", value)}
}

func validateToken002ChangeClass(relative, field, value string, allowNotApplicable bool) []GateCheck {
	allowedValues := []string{"no-change", "docs-only", "source-code", "template", "schema", "test-only", "artifact-only", "review-comment-only"}
	if allowNotApplicable {
		allowedValues = append(allowedValues, GateStatusNotApplicable)
	}
	if allowed(value, allowedValues...) {
		return []GateCheck{{Name: field, Status: GateStatusPass, Path: relative, Message: "token-002 change class vocabulary is supported", Field: field, Actual: value}}
	}
	return []GateCheck{token002Fail(relative, field, "change class vocabulary is unsupported", "Use a TOKEN-010 atomic change class; composite class strings are invalid.", field, strings.Join(allowedValues, ","), value)}
}

func validateToken002RequestedVerdicts(relative string, values []string) []GateCheck {
	checks := []GateCheck{}
	expected := []string{"accepted", "accepted_with_required_rework", "rejected", "blocked"}
	for _, want := range expected {
		if !token002ContainsString(values, want) {
			checks = append(checks, token002Fail(relative, "review_bundle_evidence.requested_verdict_vocabulary", "review bundle requested verdict vocabulary is incomplete", "Record the full requested review verdict vocabulary.", "review_bundle_evidence.requested_verdict_vocabulary", strings.Join(expected, ","), strings.Join(values, ",")))
			break
		}
	}
	for i, value := range values {
		if !allowed(value, expected...) {
			checks = append(checks, token002Fail(relative, fmt.Sprintf("review_bundle_evidence.requested_verdict_vocabulary[%d]", i), "requested verdict vocabulary contains unsupported value", "Use the TOKEN-009 review verdict vocabulary only.", fmt.Sprintf("review_bundle_evidence.requested_verdict_vocabulary[%d]", i), strings.Join(expected, ","), value))
		}
	}
	if len(checks) == 0 {
		checks = append(checks, GateCheck{Name: "review_bundle_evidence.requested_verdict_vocabulary", Status: GateStatusPass, Path: relative, Message: "review requested verdict vocabulary is supported", Field: "review_bundle_evidence.requested_verdict_vocabulary", Actual: strings.Join(values, ",")})
	}
	return checks
}

func token002ContainsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func validateToken002CriticalReferences(root Root, relative string, field string, refs []token002CriticalReference) []GateCheck {
	checks := []GateCheck{}
	if len(refs) == 0 {
		return []GateCheck{token002Fail(relative, field, "token-002 evidence lacks critical references", "Record path/checksum/retrieval instructions for critical references.", field, "one or more refs", "missing")}
	}
	for i, ref := range refs {
		item := fmt.Sprintf("%s[%d]", field, i)
		checks = append(checks, validateToken002EvidenceClass(relative, item+".class", ref.Class)...)
		checks = append(checks, validateToken002RefParts(root, relative, item, ref.Path, ref.Checksum)...)
		checks = append(checks, requireToken002String(relative, item+".retrieval_instruction", ref.RetrievalInstruction)...)
	}
	return checks
}

func validateToken002Retrieval(relative string, field string, retrieval *token002RetrievalInstructions) []GateCheck {
	if retrieval == nil {
		return []GateCheck{token002Fail(relative, field, "token-002 evidence lacks retrieval instructions", "Record artifact-first retrieval instructions.", field, "object", "missing")}
	}
	checks := requireToken002String(relative, field+".default", retrieval.Default)
	checks = append(checks, requireToken002StringSlice(relative, field+".required_when", retrieval.RequiredWhen)...)
	return checks
}

func validateToken002Invalidation(relative string, field string, invalidation *token002InvalidationBehavior) []GateCheck {
	if invalidation == nil {
		return []GateCheck{token002Fail(relative, field, "token-002 evidence lacks invalidation behavior", "Record invalid_if and on_invalid behavior.", field, "object", "missing")}
	}
	checks := requireToken002StringSlice(relative, field+".invalid_if", invalidation.InvalidIf)
	checks = append(checks, requireToken002String(relative, field+".on_invalid", invalidation.OnInvalid)...)
	return checks
}

func validateRequiredToken002Ref(root Root, relative string, field string, ref *tokenEvidenceRef) []GateCheck {
	if ref == nil {
		return []GateCheck{token002Fail(relative, field, "token-002 evidence ref is missing", "Record repository-confined artifact path plus sha256 checksum.", field, "evidence ref", "missing")}
	}
	return validateToken002RefParts(root, relative, field, ref.Path, ref.Checksum)
}

func validateToken002RefList(root Root, relative string, field string, refs []tokenEvidenceRef) []GateCheck {
	checks := []GateCheck{}
	if len(refs) == 0 {
		return []GateCheck{token002Fail(relative, field, "token-002 evidence lacks artifact refs", "Record one or more repository-confined artifact refs with sha256 checksums.", field, "one or more refs", "missing")}
	}
	for i := range refs {
		checks = append(checks, validateRequiredToken002Ref(root, relative, fmt.Sprintf("%s[%d]", field, i), &refs[i])...)
	}
	return checks
}

func validateToken002RefParts(root Root, relative string, field string, path string, checksum string) []GateCheck {
	ref := tokenEvidenceRef{Path: path, Checksum: checksum}
	checks := validateTokenEvidenceRef(root, relative, field, ref)
	if strings.TrimSpace(checksum) == "" {
		checks = append(checks, token002Fail(relative, field+".checksum", "token-002 evidence ref checksum is missing", "Record sha256 checksum for every token-002 artifact pointer.", field+".checksum", "sha256:<64hex>", "missing"))
	} else {
		checks = append(checks, validateToken002Checksum(relative, field+".checksum", checksum)...)
	}
	return checks
}

func validateToken002Checksum(relative string, field string, checksum string) []GateCheck {
	if tokenEconomyChecksumPattern.MatchString(checksum) {
		return []GateCheck{{Name: field, Status: GateStatusPass, Path: relative, Message: "token-002 checksum format is supported", Field: field, Actual: checksum}}
	}
	actual := checksum
	if strings.TrimSpace(actual) == "" {
		actual = "missing"
	}
	return []GateCheck{token002Fail(relative, field, "token-002 checksum has an unsupported format", "Use sha256:<64 hex characters>.", field, "sha256:<64hex>", actual)}
}

func validateToken002ChangedPaths(root Root, relative string, field string, paths []string) []GateCheck {
	checks := []GateCheck{}
	if len(paths) == 0 {
		return []GateCheck{token002Fail(relative, field, "changed paths are missing", "Record at least one changed path or explicit not_applicable path record.", field, "one or more paths", "missing")}
	}
	for i, path := range paths {
		checks = append(checks, validateToken002SafePath(root, relative, fmt.Sprintf("%s[%d]", field, i), path, false)...)
	}
	return checks
}

func validateToken002SafePath(root Root, relative string, field string, value string, allowNotApplicable bool) []GateCheck {
	if allowNotApplicable && value == GateStatusNotApplicable {
		return []GateCheck{{Name: field, Status: GateStatusPass, Path: relative, Message: "not_applicable path sentinel is recorded", Field: field, Actual: value}}
	}
	if strings.TrimSpace(value) == "" {
		return []GateCheck{token002Fail(relative, field, "path is missing", "Record repository-confined relative paths.", field, "non-empty relative path", "missing")}
	}
	if _, err := ResolveRelativePath(root, value); err != nil {
		return []GateCheck{token002Fail(relative, field, "path is unsafe", "Use repository-confined paths without absolute paths or parent traversal.", field, "repository-confined relative path", err.Error())}
	}
	return []GateCheck{{Name: field, Status: GateStatusPass, Path: relative, Message: "path is repository-confined", Field: field, Actual: value}}
}

func requireToken002String(relative, field, value string) []GateCheck {
	if strings.TrimSpace(value) != "" {
		return []GateCheck{{Name: field, Status: GateStatusPass, Path: relative, Message: "token-002 required string is present", Field: field, Actual: value}}
	}
	return []GateCheck{token002Fail(relative, field, "token-002 required string is missing", "Record the required normalized token-002 field.", field, "non-empty string", "missing")}
}

func requireExactToken002String(relative, field, value, expected string) []GateCheck {
	if value == expected {
		return []GateCheck{{Name: field, Status: GateStatusPass, Path: relative, Message: "token-002 required value matches", Field: field, Actual: value}}
	}
	actual := value
	if strings.TrimSpace(actual) == "" {
		actual = "missing"
	}
	return []GateCheck{token002Fail(relative, field, "token-002 required value does not match", "Use the normalized token-002 contract value.", field, expected, actual)}
}

func requireToken002StringSlice(relative, field string, values []string) []GateCheck {
	checks := []GateCheck{}
	if len(values) == 0 {
		return []GateCheck{token002Fail(relative, field, "token-002 required string list is missing", "Record one or more deterministic entries.", field, "one or more strings", "missing")}
	}
	for i, value := range values {
		checks = append(checks, requireToken002String(relative, fmt.Sprintf("%s[%d]", field, i), value)...)
	}
	return checks
}

func requireContainsAll(relative, field string, values []string, required []string) []GateCheck {
	checks := requireToken002StringSlice(relative, field, values)
	joined := strings.Join(values, "\n")
	for _, want := range required {
		if !strings.Contains(joined, want) {
			checks = append(checks, token002Fail(relative, field, "token-002 required boundary text is missing", "Record the KAS/KAH boundary note exactly enough for mechanical verification.", field, want, joined))
		}
	}
	return checks
}

func missingToken002Section(relative, name string) []GateCheck {
	return []GateCheck{{Name: name, Status: GateStatusFail, Path: relative, Message: "token-002 evidence section is missing", Hint: "Record the required token002.v1 section.", Field: name, Expected: "object", Actual: "missing"}}
}

func token002Fail(relative, name, message, hint, field, expected, actual string) GateCheck {
	return GateCheck{Name: name, Status: GateStatusFail, Path: relative, Message: message, Hint: hint, Field: field, Expected: expected, Actual: actual}
}

func validateToken002EconomyEvidenceSchema(relative string, raw map[string]any) []SchemaCheck {
	var evidence token002Evidence
	_ = mapToStruct(raw, &evidence)
	checks := []SchemaCheck{}
	for _, field := range token002RequiredFields {
		if _, ok := raw[field]; !ok {
			checks = append(checks, schemaFail(field, relative, "token-002 evidence required field is missing", "Use the token002.v1 evidence schema.", field, "present", "missing"))
		} else {
			checks = append(checks, schemaPass(field, relative, "token-002 evidence required field is present"))
		}
	}
	if evidence.SchemaVersion != tokenEconomyToken002SchemaVersion {
		checks = append(checks, schemaFail("schema_version", relative, "token-economy schema version is unsupported", "Use schema_version token002.v1.", "schema_version", tokenEconomyToken002SchemaVersion, evidence.SchemaVersion))
	} else {
		checks = append(checks, schemaPass("schema_version", relative, "token-economy schema version is supported"))
	}
	if evidence.TaskID != tokenEconomyToken002TaskID {
		checks = append(checks, schemaFail("task_id", relative, "token-002 evidence task_id is unsupported", "Use task_id token-002 with schema_version token002.v1.", "task_id", tokenEconomyToken002TaskID, evidence.TaskID))
	}
	for _, item := range []struct{ name, status, reason string }{
		{"scope", sectionStatus(evidence.Scope), sectionReason(evidence.Scope)},
		{"verification_profile_evidence", token002VerificationStatus(evidence.VerificationProfileEvidence), token002VerificationReason(evidence.VerificationProfileEvidence)},
		{"phase_packet_evidence", token002PhasePacketStatus(evidence.PhasePacketEvidence), token002PhasePacketReason(evidence.PhasePacketEvidence)},
		{"review_bundle_evidence", token002ReviewStatus(evidence.ReviewBundleEvidence), token002ReviewReason(evidence.ReviewBundleEvidence)},
		{"watcher_evidence", token002WatcherStatus(evidence.WatcherEvidence), token002WatcherReason(evidence.WatcherEvidence)},
		{"change_verification_matrix_evidence", token002ChangeStatus(evidence.ChangeVerificationMatrixEvidence), token002ChangeReason(evidence.ChangeVerificationMatrixEvidence)},
		{"mutation_approval_evidence", mutationStatus(evidence.MutationApprovalEvidence), mutationReason(evidence.MutationApprovalEvidence)},
	} {
		checks = append(checks, schemaCheckToken002Status(relative, item.name, item.status, item.reason))
	}
	return checks
}

func schemaCheckToken002Status(relative, name, status, reason string) SchemaCheck {
	switch status {
	case GateStatusPass, GateStatusFail:
		return schemaPass(name+".status", relative, "token-002 status is valid")
	case GateStatusNotApplicable:
		if strings.TrimSpace(reason) == "" {
			return schemaFail(name+".reason", relative, "not_applicable token-002 status requires a reason", "Record a deterministic not_applicable reason.", name+".reason", "non-empty reason", "missing")
		}
		return schemaPass(name+".status", relative, "token-002 not_applicable status has a reason")
	default:
		actual := status
		if actual == "" {
			actual = "missing"
		}
		return schemaFail(name+".status", relative, "token-002 status vocabulary is unsupported", "Use pass, fail, or not_applicable in token-economy-evidence.json.", name+".status", "pass,fail,not_applicable", actual)
	}
}

func token002VerificationStatus(section *token002VerificationEvidence) string {
	if section == nil {
		return ""
	}
	return section.Status
}

func token002VerificationReason(section *token002VerificationEvidence) string {
	if section == nil {
		return ""
	}
	return section.Reason
}

func token002PhasePacketStatus(section *token002PhasePacketEvidence) string {
	if section == nil {
		return ""
	}
	return section.Status
}

func token002PhasePacketReason(section *token002PhasePacketEvidence) string {
	if section == nil {
		return ""
	}
	return section.Reason
}

func token002ReviewStatus(section *token002ReviewBundleEvidence) string {
	if section == nil {
		return ""
	}
	return section.Status
}

func token002ReviewReason(section *token002ReviewBundleEvidence) string {
	if section == nil {
		return ""
	}
	return section.Reason
}

func token002WatcherStatus(section *token002WatcherEvidence) string {
	if section == nil {
		return ""
	}
	return section.Status
}

func token002WatcherReason(section *token002WatcherEvidence) string {
	if section == nil {
		return ""
	}
	return section.Reason
}

func token002ChangeStatus(section *token002ChangeVerificationEvidence) string {
	if section == nil {
		return ""
	}
	return section.Status
}

func token002ChangeReason(section *token002ChangeVerificationEvidence) string {
	if section == nil {
		return ""
	}
	return section.Reason
}
