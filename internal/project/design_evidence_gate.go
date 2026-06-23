package project

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

const (
	DesignEvidenceReasonValid                             = "design_evidence_valid"
	DesignEvidenceReasonNotApplicable                     = "design_evidence_not_applicable"
	DesignEvidenceReasonMissing                           = "design_evidence_missing"
	DesignEvidenceReasonSchemaInvalid                     = "design_evidence_schema_invalid"
	DesignEvidenceReasonRefUnsafe                         = "design_evidence_ref_unsafe"
	DesignEvidenceReasonBoundaryInvalid                   = "design_evidence_boundary_invalid"
	DesignEvidenceReasonTealRequiredEvidenceMissing       = "teal_required_evidence_missing"
	DesignEvidenceReasonTealRequiredPlanVerdictMissing    = "teal_required_plan_verdict_missing"
	DesignEvidenceReasonTealRequiredDesignSpecMissing     = "teal_required_design_spec_missing"
	DesignEvidenceReasonTealRequiredFidelityRefsMissing   = "teal_required_fidelity_refs_missing"
	DesignEvidenceReasonTealRequiredScreenshotRefsMissing = "teal_required_screenshot_refs_missing"
	DesignEvidenceReasonTealRequiredVerificationMissing   = "teal_required_verification_verdict_missing"
	DesignEvidenceReasonTealRequiredWaiverInvalid         = "teal_required_waiver_invalid"
	DesignEvidenceReasonTealSkipEvidenceMissing           = "teal_skip_evidence_missing"
	DesignEvidenceReasonTealSkipReasonMissing             = "teal_skip_reason_missing"
	DesignEvidenceReasonTealWaiverEvidenceMissing         = "teal_waiver_evidence_missing"
	DesignEvidenceReasonTealWaiverEvidenceInvalid         = "teal_waiver_evidence_invalid"
	DesignEvidenceReasonWarningOnlyFallbackForbidden      = "warning_only_fallback_forbidden"
)

type DesignEvidenceDiagnostics struct {
	Status      string      `json:"status"`
	Path        string      `json:"path,omitempty"`
	ReasonCodes []string    `json:"reason_codes"`
	Checks      []GateCheck `json:"checks"`
	NextAction  string      `json:"next_action"`
}

type designEvidenceEvaluation struct {
	RunID       string
	Status      string
	Path        string
	ReasonCodes []string
	Checks      []GateCheck
	NextAction  string
}

func checkDesignEvidenceGate(root Root, metadata RunMetadata) (GateCheckResult, error) {
	evaluation, err := evaluateDesignEvidence(root, metadata)
	if err != nil {
		return GateCheckResult{}, err
	}
	return GateCheckResult{RunID: metadata.RunID, Gate: GateDesignEvidence, Status: evaluation.Status, Checks: evaluation.Checks, MissingEvidence: designEvidenceMissingEvidence(evaluation)}, nil
}

func designEvidenceGateRequired(metadata RunMetadata) bool {
	return stringSet(metadata.RequiredArtifacts)[designEvidenceArtifact]
}

func evaluateDesignEvidence(root Root, metadata RunMetadata) (designEvidenceEvaluation, error) {
	required := designEvidenceGateRequired(metadata)
	path, err := artifactPath(root, metadata.RunID, designEvidenceArtifact)
	if err != nil {
		check := GateCheck{Name: "design_evidence_artifact", Status: GateStatusFail, Message: "design evidence artifact path is invalid", Hint: "Use artifact init to create canonical design evidence.", Field: "path", Expected: designEvidenceArtifact, Actual: err.Error()}
		return designEvidenceEvaluation{RunID: metadata.RunID, Status: GateStatusFail, ReasonCodes: []string{DesignEvidenceReasonMissing}, Checks: []GateCheck{check}, NextAction: "Repair the run artifact path before checking design evidence."}, nil
	}

	info, err := os.Lstat(path.Absolute)
	if os.IsNotExist(err) {
		check := GateCheck{Name: "design_evidence_artifact", Status: GateStatusFail, Path: path.Relative, Message: "required design evidence artifact is missing", Hint: "Write design-evidence.json before checking the design evidence gate.", Field: "path", Expected: "existing regular file", Actual: "missing"}
		if !required {
			check = GateCheck{Name: "design_evidence_task_scope", Status: GateStatusNotApplicable, Path: path.Relative, Message: "design evidence is not required by this run manifest", Field: "required_artifacts", Expected: designEvidenceArtifact, Actual: "not declared"}
			return designEvidenceEvaluation{RunID: metadata.RunID, Status: GateStatusNotApplicable, Path: path.Relative, ReasonCodes: []string{DesignEvidenceReasonNotApplicable}, Checks: []GateCheck{check}, NextAction: "No design evidence action is required for this run."}, nil
		}
		return designEvidenceEvaluation{RunID: metadata.RunID, Status: GateStatusFail, Path: path.Relative, ReasonCodes: []string{DesignEvidenceReasonMissing}, Checks: []GateCheck{check}, NextAction: "Write valid design-evidence.json before checking this gate."}, nil
	}
	if err != nil {
		check := GateCheck{Name: "design_evidence_artifact", Status: GateStatusFail, Path: path.Relative, Message: "cannot inspect design evidence artifact", Hint: "Check run artifact permissions before checking the design evidence gate.", Field: "path", Expected: "inspectable regular file", Actual: err.Error()}
		return designEvidenceEvaluation{RunID: metadata.RunID, Status: GateStatusFail, Path: path.Relative, ReasonCodes: []string{DesignEvidenceReasonMissing}, Checks: []GateCheck{check}, NextAction: "Repair artifact permissions before checking design evidence."}, nil
	}
	if !info.Mode().IsRegular() {
		check := GateCheck{Name: "design_evidence_artifact", Status: GateStatusFail, Path: path.Relative, Message: "design evidence artifact must be a regular file", Hint: "Move the conflicting path and write design-evidence.json.", Field: "path", Expected: "regular file", Actual: fileKind(info)}
		return designEvidenceEvaluation{RunID: metadata.RunID, Status: GateStatusFail, Path: path.Relative, ReasonCodes: []string{DesignEvidenceReasonMissing}, Checks: []GateCheck{check}, NextAction: "Replace the conflicting artifact path with a regular JSON file."}, nil
	}
	content, err := os.ReadFile(path.Absolute)
	if err != nil {
		check := GateCheck{Name: "design_evidence_artifact", Status: GateStatusFail, Path: path.Relative, Message: "cannot read design evidence artifact", Hint: "Check run artifact permissions before checking the design evidence gate.", Field: "path", Expected: "readable file", Actual: err.Error()}
		return designEvidenceEvaluation{RunID: metadata.RunID, Status: GateStatusFail, Path: path.Relative, ReasonCodes: []string{DesignEvidenceReasonMissing}, Checks: []GateCheck{check}, NextAction: "Repair artifact permissions before checking design evidence."}, nil
	}

	var raw map[string]any
	if err := json.Unmarshal(content, &raw); err != nil || raw == nil {
		actual := "not an object"
		if err != nil {
			actual = err.Error()
		}
		check := GateCheck{Name: "design_evidence_json", Status: GateStatusFail, Path: path.Relative, Message: "design evidence is malformed JSON", Hint: "Write design-evidence.json as one JSON object.", Field: "json", Expected: "JSON object", Actual: actual}
		return designEvidenceEvaluation{RunID: metadata.RunID, Status: GateStatusFail, Path: path.Relative, ReasonCodes: []string{DesignEvidenceReasonSchemaInvalid}, Checks: []GateCheck{check}, NextAction: "Fix design-evidence.json JSON syntax and shape."}, nil
	}
	var evidence designEvidence
	_ = mapToStruct(raw, &evidence)

	checks := []GateCheck{{Name: "design_evidence_artifact", Status: GateStatusPass, Path: path.Relative, Message: "design evidence artifact is present"}}
	reasons := map[string]bool{}
	schemaChecks := validateDesignEvidenceSchema(path.Relative, content)
	for _, check := range schemaChecks {
		if check.Status != ValidationStatusFail {
			continue
		}
		reasons[designEvidenceSchemaReason(check)] = true
		checks = append(checks, GateCheck{Name: "design_evidence_schema_" + check.Name, Status: GateStatusFail, Path: check.Path, Message: check.Message, Hint: check.Hint, Field: check.Field, Expected: check.Expected, Actual: check.Actual})
	}
	if designEvidenceHasWarningOnlyFallback(raw) {
		reasons[DesignEvidenceReasonWarningOnlyFallbackForbidden] = true
		checks = append(checks, GateCheck{Name: "warning_only_fallback", Status: GateStatusFail, Path: path.Relative, Message: "warning-only design evidence fallback is forbidden", Hint: "Required design evidence gaps must fail closed; use pass, fail, or not_applicable with deterministic evidence.", Field: "status", Expected: "pass, fail, or not_applicable", Actual: "warning"})
	}
	if designEvidenceWaiverValid(evidence.TealApplicability) && !designBoundaryEvidencePassed(evidence.BoundaryEvidence) {
		reasons[DesignEvidenceReasonBoundaryInvalid] = true
		checks = append(checks, GateCheck{Name: "design_boundary", Status: GateStatusFail, Path: path.Relative, Message: "KAS/KAH design boundary evidence is not pass", Hint: "Record pass boundary evidence preserving KAS policy ownership and KAH deterministic-only validation.", Field: "boundary_evidence.status", Expected: GateStatusPass, Actual: designBoundaryEvidenceStatus(evidence.BoundaryEvidence)})
	}
	if len(reasons) > 0 {
		return designEvidenceEvaluation{RunID: metadata.RunID, Status: GateStatusFail, Path: path.Relative, ReasonCodes: sortedReasonCodes(reasons), Checks: checks, NextAction: "Repair schema-invalid design evidence before using it for gates, diagnostics, or final gate."}, nil
	}

	if evidence.TealApplicability == nil || evidence.TealApplicability.TealRequired == nil {
		return designEvidenceFail(metadata.RunID, path.Relative, checks, "teal_required", DesignEvidenceReasonTealRequiredEvidenceMissing, "Teal applicability is incomplete", "Record KAS-derived Teal applicability before checking design evidence.", "teal_applicability.teal_required", "boolean", "missing"), nil
	}
	if *evidence.TealApplicability.TealRequired {
		return evaluateTealRequiredDesignEvidence(metadata.RunID, path.Relative, evidence, checks), nil
	}
	return evaluateTealSkippedDesignEvidence(metadata.RunID, path.Relative, evidence, checks), nil
}

func evaluateTealRequiredDesignEvidence(runID, relative string, evidence designEvidence, checks []GateCheck) designEvidenceEvaluation {
	waiverValid := designEvidenceWaiverValid(evidence.TealApplicability)
	if waiverValid {
		checks = append(checks, GateCheck{Name: "teal_waiver_evidence", Status: GateStatusPass, Path: relative, Message: "valid KAS-declared Teal waiver evidence is recorded", Field: "teal_waiver_approved", Expected: "true with approval ref, scope, and expiry", Actual: "valid"})
	}

	reasons := map[string]bool{}
	if !waiverValid {
		if !designEvidenceSectionPassed(evidence.DesignPlanEvidence) {
			reasons[DesignEvidenceReasonTealRequiredPlanVerdictMissing] = true
			checks = append(checks, GateCheck{Name: "design_plan_verdict", Status: GateStatusFail, Path: relative, Message: "required Teal plan verdict is missing or not pass", Hint: "Record a pass status from KAS/Teal plan evidence or valid waiver evidence.", Field: "design_plan_evidence.status", Expected: GateStatusPass, Actual: designEvidenceSectionStatus(evidence.DesignPlanEvidence)})
		} else {
			checks = append(checks, GateCheck{Name: "design_plan_verdict", Status: GateStatusPass, Path: relative, Message: "required Teal plan verdict is pass", Field: "design_plan_evidence.status", Actual: GateStatusPass})
		}
		if evidence.DesignPlanEvidence == nil || evidence.DesignPlanEvidence.DetailRef == nil || strings.TrimSpace(evidence.DesignPlanEvidence.DetailRef.Path) == "" {
			reasons[DesignEvidenceReasonTealRequiredDesignSpecMissing] = true
			checks = append(checks, GateCheck{Name: "design_spec_ref", Status: GateStatusFail, Path: relative, Message: "required design spec ref is missing", Hint: "Record design_plan_evidence.detail_ref with a repository-confined design spec path.", Field: "design_plan_evidence.detail_ref.path", Expected: "repository-relative path", Actual: "missing"})
		} else {
			checks = append(checks, GateCheck{Name: "design_spec_ref", Status: GateStatusPass, Path: relative, Message: "required design spec ref is recorded", Field: "design_plan_evidence.detail_ref.path", Actual: evidence.DesignPlanEvidence.DetailRef.Path})
		}
		if !designEvidenceSectionPassed(evidence.DesignFidelityEvidence) {
			reasons[DesignEvidenceReasonTealRequiredEvidenceMissing] = true
			checks = append(checks, GateCheck{Name: "design_fidelity_verdict", Status: GateStatusFail, Path: relative, Message: "required design fidelity verdict is missing or not pass", Hint: "Record pass status for deterministic fidelity evidence or valid waiver evidence.", Field: "design_fidelity_evidence.status", Expected: GateStatusPass, Actual: designEvidenceSectionStatus(evidence.DesignFidelityEvidence)})
		} else {
			checks = append(checks, GateCheck{Name: "design_fidelity_verdict", Status: GateStatusPass, Path: relative, Message: "required design fidelity verdict is pass", Field: "design_fidelity_evidence.status", Actual: GateStatusPass})
		}
		if evidence.DesignFidelityEvidence == nil || len(evidence.DesignFidelityEvidence.EvidenceRefs) == 0 {
			reasons[DesignEvidenceReasonTealRequiredFidelityRefsMissing] = true
			reasons[DesignEvidenceReasonTealRequiredScreenshotRefsMissing] = true
			checks = append(checks, GateCheck{Name: "design_fidelity_refs", Status: GateStatusFail, Path: relative, Message: "required fidelity or screenshot refs are missing", Hint: "Record design_fidelity_evidence.evidence_refs with screenshot and fidelity evidence paths.", Field: "design_fidelity_evidence.evidence_refs", Expected: "one or more refs", Actual: "missing"})
		} else {
			checks = append(checks, GateCheck{Name: "design_fidelity_refs", Status: GateStatusPass, Path: relative, Message: "required fidelity or screenshot refs are recorded", Field: "design_fidelity_evidence.evidence_refs", Actual: fmt.Sprintf("%d refs", len(evidence.DesignFidelityEvidence.EvidenceRefs))})
		}
		if !designEvidenceSectionPassed(evidence.ColorReviewEvidence) {
			reasons[DesignEvidenceReasonTealRequiredVerificationMissing] = true
			checks = append(checks, GateCheck{Name: "design_verification_verdict", Status: GateStatusFail, Path: relative, Message: "required design verification verdict is missing or not pass", Hint: "Record pass status for deterministic verification evidence or valid waiver evidence.", Field: "color_review_evidence.status", Expected: GateStatusPass, Actual: designEvidenceSectionStatus(evidence.ColorReviewEvidence)})
		} else {
			checks = append(checks, GateCheck{Name: "design_verification_verdict", Status: GateStatusPass, Path: relative, Message: "required design verification verdict is pass", Field: "color_review_evidence.status", Actual: GateStatusPass})
		}
		if evidence.ColorReviewEvidence == nil || (len(evidence.ColorReviewEvidence.EvidenceRefs) == 0 && evidence.ColorReviewEvidence.DetailRef == nil) {
			reasons[DesignEvidenceReasonTealRequiredVerificationMissing] = true
			checks = append(checks, GateCheck{Name: "design_verification_ref", Status: GateStatusFail, Path: relative, Message: "required design verification evidence ref is missing", Hint: "Record color_review_evidence evidence_refs or detail_ref with deterministic verification evidence.", Field: "color_review_evidence.evidence_refs", Expected: "one or more refs", Actual: "missing"})
		} else {
			checks = append(checks, GateCheck{Name: "design_verification_ref", Status: GateStatusPass, Path: relative, Message: "required design verification evidence ref is recorded"})
		}
	}
	if !designBoundaryEvidencePassed(evidence.BoundaryEvidence) {
		reasons[DesignEvidenceReasonBoundaryInvalid] = true
		checks = append(checks, GateCheck{Name: "design_boundary", Status: GateStatusFail, Path: relative, Message: "KAS/KAH design boundary evidence is not pass", Hint: "Record pass boundary evidence preserving KAS policy ownership and KAH deterministic-only validation.", Field: "boundary_evidence.status", Expected: GateStatusPass, Actual: designBoundaryEvidenceStatus(evidence.BoundaryEvidence)})
	} else {
		checks = append(checks, GateCheck{Name: "design_boundary", Status: GateStatusPass, Path: relative, Message: "KAS/KAH design boundary evidence is pass", Field: "boundary_evidence.status", Actual: GateStatusPass})
	}
	if len(reasons) > 0 {
		if !waiverValid && !designEvidenceWaiverFieldsAbsent(evidence.TealApplicability) {
			reasons[DesignEvidenceReasonTealRequiredWaiverInvalid] = true
			checks = append(checks, GateCheck{Name: "teal_waiver_evidence", Status: GateStatusFail, Path: relative, Message: "Teal waiver evidence is incomplete or invalid", Hint: "KAH validates waiver shape only; record approved waiver ref, scope, and expiry or complete required design evidence.", Field: "teal_waiver_approval_ref", Expected: "valid waiver evidence or complete design evidence", Actual: "invalid"})
		}
		return designEvidenceEvaluation{RunID: runID, Status: GateStatusFail, Path: relative, ReasonCodes: sortedReasonCodes(reasons), Checks: checks, NextAction: "Record all required Teal design evidence or valid KAS-declared waiver evidence before final gate."}
	}
	return designEvidencePass(runID, relative, checks, DesignEvidenceReasonValid, "Design evidence gate passed for teal_required=true.")
}

func evaluateTealSkippedDesignEvidence(runID, relative string, evidence designEvidence, checks []GateCheck) designEvidenceEvaluation {
	skipReason := strings.TrimSpace(stringPtrValue(evidence.TealApplicability.TealSkipReason))
	if skipReason != "" {
		checks = append(checks, GateCheck{Name: "teal_skip_reason", Status: GateStatusPass, Path: relative, Message: "Teal skip reason is recorded", Field: "teal_applicability.teal_skip_reason", Actual: skipReason})
		return designEvidencePass(runID, relative, checks, DesignEvidenceReasonValid, "Design evidence gate passed for teal_required=false with deterministic skip evidence.")
	}
	if designEvidenceWaiverValid(evidence.TealApplicability) {
		checks = append(checks, GateCheck{Name: "teal_waiver_evidence", Status: GateStatusPass, Path: relative, Message: "valid KAS-declared Teal waiver evidence is recorded", Field: "teal_waiver_approved", Expected: "true with approval ref, scope, and expiry", Actual: "valid"})
		return designEvidencePass(runID, relative, checks, DesignEvidenceReasonValid, "Design evidence gate passed for teal_required=false with deterministic waiver evidence.")
	}
	reasons := map[string]bool{DesignEvidenceReasonTealSkipEvidenceMissing: true, DesignEvidenceReasonTealSkipReasonMissing: true}
	if !designEvidenceWaiverFieldsAbsent(evidence.TealApplicability) {
		reasons[DesignEvidenceReasonTealWaiverEvidenceInvalid] = true
	}
	checks = append(checks, GateCheck{Name: "teal_skip_reason", Status: GateStatusFail, Path: relative, Message: "Teal skip evidence is missing", Hint: "Record teal_skip_reason or valid KAS-declared waiver evidence when teal_required is false.", Field: "teal_applicability.teal_skip_reason", Expected: "non-empty string or valid waiver evidence", Actual: "missing"})
	return designEvidenceEvaluation{RunID: runID, Status: GateStatusFail, Path: relative, ReasonCodes: sortedReasonCodes(reasons), Checks: checks, NextAction: "Record deterministic skip or waiver evidence before this gate can pass."}
}

func designEvidencePass(runID, relative string, checks []GateCheck, reason, nextAction string) designEvidenceEvaluation {
	return designEvidenceEvaluation{RunID: runID, Status: GateStatusPass, Path: relative, ReasonCodes: []string{reason}, Checks: checks, NextAction: nextAction}
}

func designEvidenceFail(runID, relative string, checks []GateCheck, name, reason, message, hint, field, expected, actual string) designEvidenceEvaluation {
	checks = append(checks, GateCheck{Name: name, Status: GateStatusFail, Path: relative, Message: message, Hint: hint, Field: field, Expected: expected, Actual: actual})
	return designEvidenceEvaluation{RunID: runID, Status: GateStatusFail, Path: relative, ReasonCodes: []string{reason}, Checks: checks, NextAction: "Repair deterministic design evidence before checking this gate."}
}

func designEvidenceMissingEvidence(evaluation designEvidenceEvaluation) []string {
	if evaluation.Status != GateStatusFail || evaluation.Path == "" {
		return []string{}
	}
	return []string{evaluation.Path}
}

func designEvidenceSchemaReason(check SchemaCheck) string {
	if check.Field != "" && strings.Contains(check.Field, ".path") && strings.Contains(check.Message, "repository-confined") {
		return DesignEvidenceReasonRefUnsafe
	}
	if strings.Contains(check.Name, ".status") && check.Actual == "warning" {
		return DesignEvidenceReasonWarningOnlyFallbackForbidden
	}
	if strings.HasPrefix(check.Name, "boundary_evidence") || strings.HasPrefix(check.Field, "boundary_evidence") {
		return DesignEvidenceReasonBoundaryInvalid
	}
	return DesignEvidenceReasonSchemaInvalid
}

func designEvidenceHasWarningOnlyFallback(raw map[string]any) bool {
	for _, field := range []string{"design_plan_evidence", "design_fidelity_evidence", "color_review_evidence", "boundary_evidence"} {
		section, _ := raw[field].(map[string]any)
		status, _ := section["status"].(string)
		if strings.EqualFold(strings.TrimSpace(status), "warning") {
			return true
		}
	}
	return false
}

func designEvidenceSectionPassed(section *designEvidenceSection) bool {
	return section != nil && section.Status == GateStatusPass
}

func designEvidenceSectionStatus(section *designEvidenceSection) string {
	if section == nil {
		return "missing"
	}
	return missingIfBlank(section.Status)
}

func designBoundaryEvidencePassed(section *designBoundaryEvidenceSection) bool {
	return section != nil && section.Status == GateStatusPass && section.PolicyOwner == designBoundaryPolicyOwner && section.KAHValidationRole == designKAHValidationRole
}

func designBoundaryEvidenceStatus(section *designBoundaryEvidenceSection) string {
	if section == nil {
		return "missing"
	}
	return missingIfBlank(section.Status)
}

func designEvidenceWaiverValid(teal *designTealApplicability) bool {
	if teal == nil || teal.TealWaiverApproved == nil || !*teal.TealWaiverApproved {
		return false
	}
	return designRelativePathShape(teal.TealWaiverApprovalRef) && strings.TrimSpace(teal.TealWaiverScope) != "" && strings.TrimSpace(teal.TealWaiverExpiresAt) != ""
}

func designEvidenceWaiverFieldsAbsent(teal *designTealApplicability) bool {
	if teal == nil {
		return true
	}
	approved := teal.TealWaiverApproved != nil && *teal.TealWaiverApproved
	return !approved && strings.TrimSpace(teal.TealWaiverApprovalRef) == "" && strings.TrimSpace(teal.TealWaiverScope) == "" && strings.TrimSpace(teal.TealWaiverExpiresAt) == ""
}

func sortedReasonCodes(values map[string]bool) []string {
	codes := make([]string, 0, len(values))
	for code := range values {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	return codes
}

func designEvidenceDiagnostics(root Root, metadata RunMetadata) DesignEvidenceDiagnostics {
	evaluation, err := evaluateDesignEvidence(root, metadata)
	if err != nil {
		return DesignEvidenceDiagnostics{Status: GateStatusFail, ReasonCodes: []string{DesignEvidenceReasonSchemaInvalid}, NextAction: RedactString(err.Error())}
	}
	checks := make([]GateCheck, len(evaluation.Checks))
	for i, check := range evaluation.Checks {
		checks[i] = GateCheck{
			Name:     RedactString(check.Name),
			Status:   RedactString(check.Status),
			Path:     RedactString(check.Path),
			Message:  RedactString(check.Message),
			Hint:     RedactString(check.Hint),
			Field:    RedactString(check.Field),
			Expected: RedactString(check.Expected),
			Actual:   RedactString(check.Actual),
		}
	}
	return DesignEvidenceDiagnostics{Status: evaluation.Status, Path: RedactString(evaluation.Path), ReasonCodes: append([]string(nil), evaluation.ReasonCodes...), Checks: checks, NextAction: RedactString(evaluation.NextAction)}
}
