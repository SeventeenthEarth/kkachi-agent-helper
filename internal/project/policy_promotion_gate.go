package project

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

const (
	policyPromotionArtifact      = "policy-promotion-evidence.json"
	policyPromotionSchemaVersion = "polpr007.v1"
	policyPromotionTaskID        = "POLPR-007"
)

var policyPromotionRequiredFields = []string{
	"schema_version",
	"run_id",
	"task_id",
	"task_class",
	"scope",
	"document_impact_map",
	"project_gray_coverage",
	"test_layer_evidence",
	"failed_test_repair_ownership",
	"final_stale_status_check",
	"boundary_evidence",
	"mutation_approval_evidence",
}

type policyPromotionEvidence struct {
	SchemaVersion             string                              `json:"schema_version"`
	RunID                     string                              `json:"run_id"`
	TaskID                    string                              `json:"task_id"`
	TaskClass                 string                              `json:"task_class"`
	Scope                     *policyPromotionSection             `json:"scope"`
	DocumentImpactMap         *policyPromotionSection             `json:"document_impact_map"`
	ProjectGrayCoverage       *policyPromotionGrayCoverage        `json:"project_gray_coverage"`
	TestLayerEvidence         *policyPromotionTestLayerEvidence   `json:"test_layer_evidence"`
	FailedTestRepairOwnership *policyPromotionRepairOwnership     `json:"failed_test_repair_ownership"`
	FinalStaleStatusCheck     *policyPromotionStaleStatusEvidence `json:"final_stale_status_check"`
	BoundaryEvidence          *policyPromotionBoundaryEvidence    `json:"boundary_evidence"`
	MutationApprovalEvidence  *tokenMutationEvidence              `json:"mutation_approval_evidence"`
}

type policyPromotionSection struct {
	Status       string             `json:"status"`
	Reason       string             `json:"reason,omitempty"`
	EvidenceRefs []tokenEvidenceRef `json:"evidence_refs,omitempty"`
	DetailRef    *tokenEvidenceRef  `json:"detail_ref,omitempty"`
}

type policyPromotionGrayCoverage struct {
	Status                string             `json:"status"`
	Reason                string             `json:"reason,omitempty"`
	ResolvedRole          string             `json:"resolved_role,omitempty"`
	RoleRegistryRef       *tokenEvidenceRef  `json:"role_registry_ref,omitempty"`
	CoverageRefs          []tokenEvidenceRef `json:"coverage_refs,omitempty"`
	NoHardCodedIndividual string             `json:"no_hard_coded_individual,omitempty"`
}

type policyPromotionTestLayerEvidence struct {
	Status         string                     `json:"status"`
	Reason         string                     `json:"reason,omitempty"`
	RequiredLabels []string                   `json:"required_labels,omitempty"`
	Labels         []policyPromotionTestLayer `json:"labels,omitempty"`
}

type policyPromotionTestLayer struct {
	Label         string             `json:"label"`
	ResourceScope string             `json:"resource_scope"`
	EvidenceRefs  []tokenEvidenceRef `json:"evidence_refs,omitempty"`
}

type policyPromotionRepairOwnership struct {
	Status                string             `json:"status"`
	Reason                string             `json:"reason,omitempty"`
	BlueResponsibility    string             `json:"blue_responsibility,omitempty"`
	ImplementerLane       string             `json:"implementer_lane,omitempty"`
	ForbiddenBlueActions  []string           `json:"forbidden_blue_actions,omitempty"`
	OwnershipEvidenceRefs []tokenEvidenceRef `json:"ownership_evidence_refs,omitempty"`
}

type policyPromotionStaleStatusEvidence struct {
	Status          string                              `json:"status"`
	Reason          string                              `json:"reason,omitempty"`
	SurfacesChecked []policyPromotionStaleStatusSurface `json:"surfaces_checked,omitempty"`
	EvidenceRefs    []tokenEvidenceRef                  `json:"evidence_refs,omitempty"`
}

type policyPromotionStaleStatusSurface struct {
	Path              string            `json:"path"`
	ExpectedStatus    string            `json:"expected_status"`
	ObservedStatus    string            `json:"observed_status"`
	Result            string            `json:"result"`
	StatusEvidenceRef *tokenEvidenceRef `json:"status_evidence_ref,omitempty"`
}

type policyPromotionBoundaryEvidence struct {
	Status                string             `json:"status"`
	Reason                string             `json:"reason,omitempty"`
	PolicyOwner           string             `json:"policy_owner,omitempty"`
	KAHValidationRole     string             `json:"kah_validation_role,omitempty"`
	KAHForbiddenDecisions []string           `json:"kah_forbidden_decisions,omitempty"`
	EvidenceRefs          []tokenEvidenceRef `json:"evidence_refs,omitempty"`
}

func checkPolicyPromotionGate(root Root, metadata RunMetadata) (GateCheckResult, error) {
	taskID := metadataTaskID(metadata)
	if taskID != policyPromotionTaskID {
		check := GateCheck{Name: "policy_promotion_task_scope", Status: GateStatusNotApplicable, Message: "policy-promotion evidence is only required for POLPR-007 runs", Field: "task_id", Expected: policyPromotionTaskID, Actual: taskID}
		return GateCheckResult{RunID: metadata.RunID, Gate: GatePolicyPromotion, Status: GateStatusNotApplicable, Checks: []GateCheck{check}}, nil
	}

	path, err := artifactPath(root, metadata.RunID, policyPromotionArtifact)
	if err != nil {
		check := GateCheck{Name: "policy_promotion_artifact", Status: GateStatusFail, Message: "policy-promotion artifact path is invalid", Hint: "Use artifact init to create canonical policy-promotion evidence.", Field: "path", Expected: policyPromotionArtifact, Actual: err.Error()}
		return gateResultFromChecks(metadata.RunID, GatePolicyPromotion, []GateCheck{check}), nil
	}
	info, err := os.Lstat(path.Absolute)
	if os.IsNotExist(err) {
		check := GateCheck{Name: "policy_promotion_artifact", Status: GateStatusFail, Path: path.Relative, Message: "required policy-promotion evidence artifact is missing", Hint: "Write policy-promotion-evidence.json for the POLPR-007 run before checking the gate.", Field: "path", Expected: "existing regular file", Actual: "missing"}
		return gateResultFromChecks(metadata.RunID, GatePolicyPromotion, []GateCheck{check}), nil
	}
	if err != nil {
		check := GateCheck{Name: "policy_promotion_artifact", Status: GateStatusFail, Path: path.Relative, Message: "cannot inspect policy-promotion evidence artifact", Hint: "Check run artifact permissions before checking the policy-promotion gate.", Field: "path", Expected: "inspectable regular file", Actual: err.Error()}
		return gateResultFromChecks(metadata.RunID, GatePolicyPromotion, []GateCheck{check}), nil
	}
	if !info.Mode().IsRegular() {
		check := GateCheck{Name: "policy_promotion_artifact", Status: GateStatusFail, Path: path.Relative, Message: "policy-promotion evidence artifact must be a regular file", Hint: "Move the conflicting path and rewrite the evidence artifact.", Field: "path", Expected: "regular file", Actual: fileKind(info)}
		return gateResultFromChecks(metadata.RunID, GatePolicyPromotion, []GateCheck{check}), nil
	}
	content, err := os.ReadFile(path.Absolute)
	if err != nil {
		check := GateCheck{Name: "policy_promotion_artifact", Status: GateStatusFail, Path: path.Relative, Message: "cannot read policy-promotion evidence artifact", Hint: "Check run artifact permissions before checking the policy-promotion gate.", Field: "path", Expected: "readable file", Actual: err.Error()}
		return gateResultFromChecks(metadata.RunID, GatePolicyPromotion, []GateCheck{check}), nil
	}
	checks := validatePolicyPromotionEvidence(root, metadata, path.Relative, content)
	return policyPromotionResultFromChecks(metadata.RunID, checks), nil
}

func validatePolicyPromotionEvidence(root Root, metadata RunMetadata, relative string, content []byte) []GateCheck {
	checks := []GateCheck{{Name: "policy_promotion_artifact", Status: GateStatusPass, Path: relative, Message: "policy-promotion evidence artifact is present"}}
	var evidence policyPromotionEvidence
	var raw map[string]any
	if err := json.Unmarshal(content, &raw); err != nil || raw == nil {
		actual := "not an object"
		if err != nil {
			actual = err.Error()
		}
		return append(checks, GateCheck{Name: "policy_promotion_json", Status: GateStatusFail, Path: relative, Message: "policy-promotion evidence is malformed JSON", Hint: "Write policy-promotion-evidence.json as a JSON object.", Field: "json", Expected: "JSON object", Actual: actual})
	}
	if err := json.Unmarshal(content, &evidence); err != nil {
		return append(checks, GateCheck{Name: "policy_promotion_json", Status: GateStatusFail, Path: relative, Message: "policy-promotion evidence cannot be decoded", Hint: "Use the polpr007.v1 evidence schema.", Field: "json", Expected: "polpr007.v1 object", Actual: err.Error()})
	}
	checks = append(checks, validatePolicyPromotionRequiredFields(relative, raw)...)
	checks = append(checks, validatePolicyPromotionIdentity(metadata, relative, evidence)...)
	checks = append(checks, validatePolicyPromotionScope(root, relative, evidence.Scope)...)
	checks = append(checks, validatePolicyPromotionSection(root, relative, "document_impact_map", evidence.DocumentImpactMap, true)...)
	checks = append(checks, validatePolicyPromotionGray(root, relative, evidence.ProjectGrayCoverage)...)
	checks = append(checks, validatePolicyPromotionTestLayers(root, relative, evidence.TestLayerEvidence)...)
	checks = append(checks, validatePolicyPromotionRepairOwnership(root, relative, evidence.FailedTestRepairOwnership)...)
	checks = append(checks, validatePolicyPromotionStaleStatus(root, relative, evidence.FinalStaleStatusCheck)...)
	checks = append(checks, validatePolicyPromotionBoundary(root, relative, evidence.BoundaryEvidence)...)
	checks = append(checks, validateTokenMutationEvidence(root, relative, evidence.MutationApprovalEvidence)...)
	return checks
}

func validatePolicyPromotionRequiredFields(relative string, raw map[string]any) []GateCheck {
	checks := []GateCheck{}
	for _, field := range policyPromotionRequiredFields {
		if _, ok := raw[field]; !ok {
			checks = append(checks, GateCheck{Name: "policy_promotion_required_field", Status: GateStatusFail, Path: relative, Message: "policy-promotion evidence is missing a required field", Hint: "Write all polpr007.v1 required fields before checking the gate.", Field: field, Expected: "present", Actual: "missing"})
		}
	}
	return checks
}

func validatePolicyPromotionIdentity(metadata RunMetadata, relative string, evidence policyPromotionEvidence) []GateCheck {
	checks := []GateCheck{}
	if evidence.SchemaVersion == policyPromotionSchemaVersion {
		checks = append(checks, GateCheck{Name: "schema_version", Status: GateStatusPass, Path: relative, Message: "policy-promotion schema version is supported", Field: "schema_version", Actual: evidence.SchemaVersion})
	} else {
		checks = append(checks, GateCheck{Name: "schema_version", Status: GateStatusFail, Path: relative, Message: "policy-promotion schema version is unsupported", Hint: "Use schema_version polpr007.v1.", Field: "schema_version", Expected: policyPromotionSchemaVersion, Actual: missingIfBlank(evidence.SchemaVersion)})
	}
	if evidence.RunID == metadata.RunID {
		checks = append(checks, GateCheck{Name: "run_id", Status: GateStatusPass, Path: relative, Message: "run_id matches run metadata", Field: "run_id", Actual: evidence.RunID})
	} else {
		checks = append(checks, GateCheck{Name: "run_id", Status: GateStatusFail, Path: relative, Message: "run_id does not match run metadata", Hint: "Record the current run id in policy-promotion-evidence.json.", Field: "run_id", Expected: metadata.RunID, Actual: missingIfBlank(evidence.RunID)})
	}
	if evidence.TaskID == policyPromotionTaskID && metadataTaskID(metadata) == policyPromotionTaskID {
		checks = append(checks, GateCheck{Name: "task_id", Status: GateStatusPass, Path: relative, Message: "task_id is POLPR-007", Field: "task_id", Actual: evidence.TaskID})
	} else {
		checks = append(checks, GateCheck{Name: "task_id", Status: GateStatusFail, Path: relative, Message: "task_id is not POLPR-007", Hint: "The policy-promotion gate validates only POLPR-007 evidence.", Field: "task_id", Expected: policyPromotionTaskID, Actual: missingIfBlank(evidence.TaskID)})
	}
	if strings.TrimSpace(evidence.TaskClass) == "" {
		checks = append(checks, GateCheck{Name: "task_class", Status: GateStatusFail, Path: relative, Message: "policy-promotion evidence is missing task_class", Hint: "Record the deterministic task class in task_class.", Field: "task_class", Expected: "non-empty string", Actual: "missing"})
	} else {
		checks = append(checks, GateCheck{Name: "task_class", Status: GateStatusPass, Path: relative, Message: "task_class is recorded", Field: "task_class", Actual: evidence.TaskClass})
	}
	return checks
}

func validatePolicyPromotionSection(root Root, relative, name string, section *policyPromotionSection, requireEvidence bool) []GateCheck {
	if section == nil {
		return []GateCheck{{Name: name, Status: GateStatusFail, Path: relative, Message: "policy-promotion evidence section is missing", Hint: "Record the required polpr007.v1 section.", Field: name, Expected: "object", Actual: "missing"}}
	}
	checks := []GateCheck{validatePolicyStatus(relative, name, section.Status, section.Reason)}
	if section.Status != GateStatusPass {
		return checks
	}
	refs := append([]tokenEvidenceRef{}, section.EvidenceRefs...)
	if section.DetailRef != nil {
		refs = append(refs, *section.DetailRef)
	}
	if requireEvidence && len(refs) == 0 {
		checks = append(checks, GateCheck{Name: name + "_refs", Status: GateStatusFail, Path: relative, Message: "policy-promotion evidence section lacks evidence refs", Hint: "Record at least one repository-confined evidence ref for this section.", Field: name + ".evidence_refs", Expected: "one or more refs", Actual: "missing"})
	}
	for i, ref := range refs {
		checks = append(checks, validateTokenEvidenceRef(root, relative, fmt.Sprintf("%s.evidence_refs[%d]", name, i), ref)...)
	}
	return checks
}

func validatePolicyPromotionScope(root Root, relative string, section *policyPromotionSection) []GateCheck {
	checks := validatePolicyPromotionSection(root, relative, "scope", section, false)
	if section == nil || strings.TrimSpace(section.Status) != GateStatusNotApplicable {
		return checks
	}
	checks = append(checks, GateCheck{Name: "scope.applicability", Status: GateStatusFail, Path: relative, Message: "POLPR-007 policy-promotion evidence scope must be applicable", Hint: "The run task_id already establishes POLPR-007 applicability; use pass for scope or run the gate on a non-POLPR-007 task to return not_applicable.", Field: "scope.status", Expected: GateStatusPass, Actual: GateStatusNotApplicable})
	return checks
}

func validatePolicyPromotionGray(root Root, relative string, evidence *policyPromotionGrayCoverage) []GateCheck {
	if evidence == nil {
		return []GateCheck{{Name: "project_gray_coverage", Status: GateStatusFail, Path: relative, Message: "project-Gray coverage evidence is missing", Field: "project_gray_coverage", Expected: "object", Actual: "missing"}}
	}
	checks := []GateCheck{validatePolicyStatus(relative, "project_gray_coverage", evidence.Status, evidence.Reason)}
	if evidence.Status != GateStatusPass {
		return checks
	}
	if strings.TrimSpace(evidence.ResolvedRole) == "" {
		checks = append(checks, GateCheck{Name: "project_gray_coverage.resolved_role", Status: GateStatusFail, Path: relative, Message: "project-Gray coverage must record the resolved role label", Hint: "Record the project/team registry role label, not a hard-coded individual as policy authority.", Field: "project_gray_coverage.resolved_role", Expected: "non-empty role label", Actual: "missing"})
	} else {
		checks = append(checks, GateCheck{Name: "project_gray_coverage.resolved_role", Status: GateStatusPass, Path: relative, Message: "project-Gray resolved role is recorded", Field: "project_gray_coverage.resolved_role", Actual: evidence.ResolvedRole})
	}
	if strings.TrimSpace(evidence.NoHardCodedIndividual) == "" {
		checks = append(checks, GateCheck{Name: "project_gray_coverage.no_hard_coded_individual", Status: GateStatusFail, Path: relative, Message: "project-Gray evidence must record the no-hard-coded-individual boundary", Field: "project_gray_coverage.no_hard_coded_individual", Expected: "non-empty boundary note", Actual: "missing"})
	}
	if evidence.RoleRegistryRef == nil {
		checks = append(checks, GateCheck{Name: "project_gray_coverage.role_registry_ref", Status: GateStatusFail, Path: relative, Message: "project-Gray coverage requires a role registry evidence ref", Field: "project_gray_coverage.role_registry_ref", Expected: "evidence ref", Actual: "missing"})
	} else {
		checks = append(checks, validateTokenEvidenceRef(root, relative, "project_gray_coverage.role_registry_ref", *evidence.RoleRegistryRef)...)
	}
	if len(evidence.CoverageRefs) == 0 {
		checks = append(checks, GateCheck{Name: "project_gray_coverage.coverage_refs", Status: GateStatusFail, Path: relative, Message: "project-Gray coverage requires review/evidence refs", Field: "project_gray_coverage.coverage_refs", Expected: "one or more refs", Actual: "missing"})
	}
	for i, ref := range evidence.CoverageRefs {
		checks = append(checks, validateTokenEvidenceRef(root, relative, fmt.Sprintf("project_gray_coverage.coverage_refs[%d]", i), ref)...)
	}
	return checks
}

func validatePolicyPromotionTestLayers(root Root, relative string, evidence *policyPromotionTestLayerEvidence) []GateCheck {
	if evidence == nil {
		return []GateCheck{{Name: "test_layer_evidence", Status: GateStatusFail, Path: relative, Message: "test-layer evidence is missing", Field: "test_layer_evidence", Expected: "object", Actual: "missing"}}
	}
	checks := []GateCheck{validatePolicyStatus(relative, "test_layer_evidence", evidence.Status, evidence.Reason)}
	if evidence.Status != GateStatusPass {
		return checks
	}
	if len(evidence.RequiredLabels) == 0 {
		checks = append(checks, GateCheck{Name: "test_layer_evidence.required_labels", Status: GateStatusFail, Path: relative, Message: "test-layer evidence must record required labels", Field: "test_layer_evidence.required_labels", Expected: "one or more labels", Actual: "missing"})
	}
	observed := map[string]bool{}
	for i, label := range evidence.Labels {
		field := fmt.Sprintf("test_layer_evidence.labels[%d]", i)
		trimmed := strings.TrimSpace(label.Label)
		if trimmed == "" {
			checks = append(checks, GateCheck{Name: field + ".label", Status: GateStatusFail, Path: relative, Message: "test-layer label is missing", Field: field + ".label", Expected: "non-empty label", Actual: "missing"})
		} else {
			observed[trimmed] = true
			checks = append(checks, GateCheck{Name: field + ".label", Status: GateStatusPass, Path: relative, Message: "test-layer label is recorded", Field: field + ".label", Actual: trimmed})
		}
		if strings.TrimSpace(label.ResourceScope) == "" {
			checks = append(checks, GateCheck{Name: field + ".resource_scope", Status: GateStatusFail, Path: relative, Message: "test-layer resource scope is missing", Field: field + ".resource_scope", Expected: "non-empty resource scope", Actual: "missing"})
		}
		if len(label.EvidenceRefs) == 0 {
			checks = append(checks, GateCheck{Name: field + ".evidence_refs", Status: GateStatusFail, Path: relative, Message: "test-layer label lacks evidence refs", Field: field + ".evidence_refs", Expected: "one or more refs", Actual: "missing"})
		}
		for j, ref := range label.EvidenceRefs {
			checks = append(checks, validateTokenEvidenceRef(root, relative, fmt.Sprintf("%s.evidence_refs[%d]", field, j), ref)...)
		}
	}
	for _, required := range evidence.RequiredLabels {
		required = strings.TrimSpace(required)
		if required == "" {
			checks = append(checks, GateCheck{Name: "test_layer_evidence.required_labels", Status: GateStatusFail, Path: relative, Message: "required test-layer label is empty", Field: "test_layer_evidence.required_labels", Expected: "non-empty label", Actual: "empty"})
			continue
		}
		if observed[required] {
			checks = append(checks, GateCheck{Name: "test_layer_evidence.required_label:" + required, Status: GateStatusPass, Path: relative, Message: "required test-layer label is covered", Field: "test_layer_evidence.labels", Actual: required})
		} else {
			checks = append(checks, GateCheck{Name: "test_layer_evidence.required_label:" + required, Status: GateStatusFail, Path: relative, Message: "required test-layer label is not covered", Hint: "Record a label record for every required test-layer label.", Field: "test_layer_evidence.labels", Expected: required, Actual: "missing"})
		}
	}
	return checks
}

func validatePolicyPromotionRepairOwnership(root Root, relative string, evidence *policyPromotionRepairOwnership) []GateCheck {
	if evidence == nil {
		return []GateCheck{{Name: "failed_test_repair_ownership", Status: GateStatusFail, Path: relative, Message: "failed-test repair ownership evidence is missing", Field: "failed_test_repair_ownership", Expected: "object", Actual: "missing"}}
	}
	checks := []GateCheck{validatePolicyStatus(relative, "failed_test_repair_ownership", evidence.Status, evidence.Reason)}
	if evidence.Status != GateStatusPass {
		return checks
	}
	for _, field := range []struct{ name, value string }{{"blue_responsibility", evidence.BlueResponsibility}, {"implementer_lane", evidence.ImplementerLane}} {
		if strings.TrimSpace(field.value) == "" {
			checks = append(checks, GateCheck{Name: "failed_test_repair_ownership." + field.name, Status: GateStatusFail, Path: relative, Message: "failed-test repair ownership field is missing", Field: "failed_test_repair_ownership." + field.name, Expected: "non-empty string", Actual: "missing"})
		}
	}
	if len(evidence.ForbiddenBlueActions) == 0 {
		checks = append(checks, GateCheck{Name: "failed_test_repair_ownership.forbidden_blue_actions", Status: GateStatusFail, Path: relative, Message: "failed-test repair ownership must record forbidden Blue actions", Field: "failed_test_repair_ownership.forbidden_blue_actions", Expected: "one or more actions", Actual: "missing"})
	}
	if len(evidence.OwnershipEvidenceRefs) == 0 {
		checks = append(checks, GateCheck{Name: "failed_test_repair_ownership.ownership_evidence_refs", Status: GateStatusFail, Path: relative, Message: "failed-test repair ownership requires evidence refs", Field: "failed_test_repair_ownership.ownership_evidence_refs", Expected: "one or more refs", Actual: "missing"})
	}
	for i, ref := range evidence.OwnershipEvidenceRefs {
		checks = append(checks, validateTokenEvidenceRef(root, relative, fmt.Sprintf("failed_test_repair_ownership.ownership_evidence_refs[%d]", i), ref)...)
	}
	return checks
}

func validatePolicyPromotionStaleStatus(root Root, relative string, evidence *policyPromotionStaleStatusEvidence) []GateCheck {
	if evidence == nil {
		return []GateCheck{{Name: "final_stale_status_check", Status: GateStatusFail, Path: relative, Message: "final stale-status evidence is missing", Field: "final_stale_status_check", Expected: "object", Actual: "missing"}}
	}
	checks := []GateCheck{validatePolicyStatus(relative, "final_stale_status_check", evidence.Status, evidence.Reason)}
	if evidence.Status != GateStatusPass {
		return checks
	}
	if len(evidence.SurfacesChecked) == 0 {
		checks = append(checks, GateCheck{Name: "final_stale_status_check.surfaces_checked", Status: GateStatusFail, Path: relative, Message: "final stale-status check requires checked surfaces", Field: "final_stale_status_check.surfaces_checked", Expected: "one or more surfaces", Actual: "missing"})
	}
	for i, surface := range evidence.SurfacesChecked {
		field := fmt.Sprintf("final_stale_status_check.surfaces_checked[%d]", i)
		checks = append(checks, validatePolicySurfacePath(root, relative, field+".path", surface.Path)...)
		if strings.TrimSpace(surface.ExpectedStatus) == "" || strings.TrimSpace(surface.ObservedStatus) == "" {
			checks = append(checks, GateCheck{Name: field + ".status_values", Status: GateStatusFail, Path: relative, Message: "stale-status surface must record expected and observed status", Field: field, Expected: "expected_status and observed_status", Actual: "missing"})
		}
		checks = append(checks, validatePolicyStatus(relative, field+".result", surface.Result, "surface result recorded as fail"))
		if surface.StatusEvidenceRef == nil {
			checks = append(checks, GateCheck{Name: field + ".status_evidence_ref", Status: GateStatusFail, Path: relative, Message: "stale-status surface requires status evidence ref", Field: field + ".status_evidence_ref", Expected: "evidence ref", Actual: "missing"})
		} else {
			checks = append(checks, validateTokenEvidenceRef(root, relative, field+".status_evidence_ref", *surface.StatusEvidenceRef)...)
		}
	}
	if len(evidence.EvidenceRefs) == 0 {
		checks = append(checks, GateCheck{Name: "final_stale_status_check.evidence_refs", Status: GateStatusFail, Path: relative, Message: "final stale-status check requires summary evidence refs", Field: "final_stale_status_check.evidence_refs", Expected: "one or more refs", Actual: "missing"})
	}
	for i, ref := range evidence.EvidenceRefs {
		checks = append(checks, validateTokenEvidenceRef(root, relative, fmt.Sprintf("final_stale_status_check.evidence_refs[%d]", i), ref)...)
	}
	return checks
}

func validatePolicyPromotionBoundary(root Root, relative string, evidence *policyPromotionBoundaryEvidence) []GateCheck {
	if evidence == nil {
		return []GateCheck{{Name: "boundary_evidence", Status: GateStatusFail, Path: relative, Message: "boundary evidence is missing", Field: "boundary_evidence", Expected: "object", Actual: "missing"}}
	}
	checks := []GateCheck{validatePolicyStatus(relative, "boundary_evidence", evidence.Status, evidence.Reason)}
	if evidence.Status != GateStatusPass {
		return checks
	}
	if evidence.PolicyOwner != "KAS" {
		checks = append(checks, GateCheck{Name: "boundary_evidence.policy_owner", Status: GateStatusFail, Path: relative, Message: "policy owner must remain KAS", Hint: "KAH validates deterministic evidence presence/shape only.", Field: "boundary_evidence.policy_owner", Expected: "KAS", Actual: missingIfBlank(evidence.PolicyOwner)})
	} else {
		checks = append(checks, GateCheck{Name: "boundary_evidence.policy_owner", Status: GateStatusPass, Path: relative, Message: "policy owner remains KAS", Field: "boundary_evidence.policy_owner", Actual: evidence.PolicyOwner})
	}
	if strings.TrimSpace(evidence.KAHValidationRole) == "" {
		checks = append(checks, GateCheck{Name: "boundary_evidence.kah_validation_role", Status: GateStatusFail, Path: relative, Message: "KAH validation role is missing", Field: "boundary_evidence.kah_validation_role", Expected: "non-empty boundary statement", Actual: "missing"})
	}
	if len(evidence.KAHForbiddenDecisions) == 0 {
		checks = append(checks, GateCheck{Name: "boundary_evidence.kah_forbidden_decisions", Status: GateStatusFail, Path: relative, Message: "KAH forbidden decisions are missing", Field: "boundary_evidence.kah_forbidden_decisions", Expected: "one or more forbidden decisions", Actual: "missing"})
	}
	if len(evidence.EvidenceRefs) == 0 {
		checks = append(checks, GateCheck{Name: "boundary_evidence.evidence_refs", Status: GateStatusFail, Path: relative, Message: "boundary evidence requires evidence refs", Field: "boundary_evidence.evidence_refs", Expected: "one or more refs", Actual: "missing"})
	}
	for i, ref := range evidence.EvidenceRefs {
		checks = append(checks, validateTokenEvidenceRef(root, relative, fmt.Sprintf("boundary_evidence.evidence_refs[%d]", i), ref)...)
	}
	return checks
}

func validatePolicyStatus(relative, name, status, reason string) GateCheck {
	status = strings.TrimSpace(status)
	switch status {
	case GateStatusPass:
		return GateCheck{Name: name, Status: GateStatusPass, Path: relative, Message: "policy-promotion evidence status is pass", Field: name + ".status", Actual: status}
	case GateStatusFail:
		return GateCheck{Name: name, Status: GateStatusFail, Path: relative, Message: "policy-promotion evidence section records fail", Hint: "Repair or mark the section not_applicable with a deterministic reason before using this gate as passing evidence.", Field: name + ".status", Expected: GateStatusPass, Actual: status}
	case GateStatusNotApplicable:
		if strings.TrimSpace(reason) == "" {
			return GateCheck{Name: name, Status: GateStatusFail, Path: relative, Message: "not_applicable policy-promotion evidence requires a reason", Hint: "Record a deterministic reason for every not_applicable section.", Field: name + ".reason", Expected: "non-empty reason", Actual: "missing"}
		}
		return GateCheck{Name: name, Status: GateStatusNotApplicable, Path: relative, Message: "policy-promotion evidence section is not applicable with a reason", Field: name + ".reason", Actual: strings.TrimSpace(reason)}
	default:
		return GateCheck{Name: name, Status: GateStatusFail, Path: relative, Message: "policy-promotion evidence status vocabulary is unsupported", Hint: "Use pass, fail, or not_applicable inside policy-promotion-evidence.json.", Field: name + ".status", Expected: "pass,fail,not_applicable", Actual: missingIfBlank(status)}
	}
}

func validatePolicySurfacePath(root Root, artifactRelative, field, value string) []GateCheck {
	if strings.TrimSpace(value) == "" {
		return []GateCheck{{Name: field, Status: GateStatusFail, Path: artifactRelative, Message: "surface path is missing", Field: field, Expected: "repository-confined relative path", Actual: "missing"}}
	}
	path, err := ResolveRelativePath(root, value)
	if err != nil {
		return []GateCheck{{Name: field, Status: GateStatusFail, Path: artifactRelative, Message: "surface path is unsafe", Hint: "Use repository-relative paths without absolute paths or parent traversal.", Field: field, Expected: "repository-confined relative path", Actual: err.Error()}}
	}
	return []GateCheck{{Name: field, Status: GateStatusPass, Path: artifactRelative, Message: "surface path is repository-confined", Field: field, Actual: path.Relative}}
}

func policyPromotionResultFromChecks(runID string, checks []GateCheck) GateCheckResult {
	status := GateStatusPass
	missing := []string{}
	for _, check := range checks {
		if check.Status == GateStatusFail {
			status = GateStatusFail
			if check.Path != "" {
				missing = appendUnique(missing, check.Path)
			}
		}
	}
	if status != GateStatusFail {
		for _, check := range checks {
			if check.Name == "scope" && check.Status == GateStatusNotApplicable {
				status = GateStatusNotApplicable
				break
			}
		}
	}
	return GateCheckResult{RunID: runID, Gate: GatePolicyPromotion, Status: status, Checks: checks, MissingEvidence: missing}
}

func validatePolicyPromotionEvidenceSchema(relative string, content []byte) []SchemaCheck {
	var raw map[string]any
	if err := json.Unmarshal(content, &raw); err != nil || raw == nil {
		actual := "not an object"
		if err != nil {
			actual = err.Error()
		}
		return []SchemaCheck{schemaFail("json", relative, "file is not valid policy-promotion JSON", "Fix the file so it contains one JSON object before validating again.", "json", "JSON object", actual)}
	}
	checks := []SchemaCheck{
		requireStringField(relative, raw, "schema_version", policyPromotionSchemaVersion, true),
		requirePatternField(relative, raw, "run_id", runIDPattern.String(), runIDPattern.MatchString),
		requireStringField(relative, raw, "task_id", policyPromotionTaskID, true),
		requireStringField(relative, raw, "task_class", "non-empty string", false),
	}
	for _, field := range policyPromotionRequiredFields[4:] {
		checks = append(checks, requireObjectField(relative, raw, field))
	}
	for _, field := range policyPromotionRequiredFields[4:] {
		section, ok := raw[field].(map[string]any)
		if !ok {
			continue
		}
		status, _ := section["status"].(string)
		if !allowed(status, GateStatusPass, GateStatusFail, GateStatusNotApplicable) {
			checks = append(checks, schemaFail(field+".status", relative, "enum field has an invalid value", "Use one of the schema-supported values.", field+".status", strings.Join([]string{GateStatusPass, GateStatusFail, GateStatusNotApplicable}, ","), fmt.Sprintf("%v", section["status"])))
		} else {
			checks = append(checks, schemaPass(field+".status", relative, "enum field is valid"))
		}
		if status == GateStatusNotApplicable {
			reason, _ := section["reason"].(string)
			if strings.TrimSpace(reason) == "" {
				checks = append(checks, schemaFail(field+".reason", relative, "required string field is missing or invalid", "Record a non-empty string for this schema field.", field+".reason", "non-empty string", fmt.Sprintf("%v", section["reason"])))
			} else {
				checks = append(checks, schemaPass(field+".reason", relative, "string field is valid"))
			}
		}
	}
	return checks
}

func missingIfBlank(value string) string {
	if strings.TrimSpace(value) == "" {
		return "missing"
	}
	return value
}
