package project

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

const (
	designEvidenceArtifact      = "design-evidence.json"
	designEvidenceSchemaVersion = "design004.v1"
	designBoundaryPolicyOwner   = "KAS"
	designKAHValidationRole     = "deterministic_shape_only"
)

var designEvidenceRequiredFields = []string{
	"schema_version",
	"run_id",
	"task_id",
	"task_class",
	"teal_applicability",
	"design_plan_evidence",
	"design_fidelity_evidence",
	"color_review_evidence",
	"boundary_evidence",
}

type designEvidence struct {
	SchemaVersion          string                         `json:"schema_version"`
	RunID                  string                         `json:"run_id"`
	TaskID                 string                         `json:"task_id"`
	TaskClass              string                         `json:"task_class"`
	TealApplicability      *designTealApplicability       `json:"teal_applicability"`
	DesignPlanEvidence     *designEvidenceSection         `json:"design_plan_evidence"`
	DesignFidelityEvidence *designEvidenceSection         `json:"design_fidelity_evidence"`
	ColorReviewEvidence    *designEvidenceSection         `json:"color_review_evidence"`
	BoundaryEvidence       *designBoundaryEvidenceSection `json:"boundary_evidence"`
}

type designTealApplicability struct {
	ProjectHasTealLane       *bool    `json:"project_has_teal_lane"`
	UIUXChange               *bool    `json:"ui_ux_change"`
	TealRequired             *bool    `json:"teal_required"`
	Derivation               string   `json:"derivation,omitempty"`
	UIUXClassificationOwner  *string  `json:"ui_ux_classification_owner,omitempty"`
	TealSkipReason           *string  `json:"teal_skip_reason,omitempty"`
	TealOwner                *string  `json:"teal_owner,omitempty"`
	TealWaiverApproved       *bool    `json:"teal_waiver_approved,omitempty"`
	TealWaiverApprovalRef    string   `json:"teal_waiver_approval_ref,omitempty"`
	TealWaiverScope          string   `json:"teal_waiver_scope,omitempty"`
	TealWaiverExpiresAt      string   `json:"teal_waiver_expires_at,omitempty"`
	RequiredWhenTealRequired []string `json:"required_when_teal_required,omitempty"`
	MissingRequiredStatus    string   `json:"missing_required_status,omitempty"`
}

type designEvidenceSection struct {
	Status       string              `json:"status"`
	Reason       string              `json:"reason,omitempty"`
	EvidenceRefs []designEvidenceRef `json:"evidence_refs,omitempty"`
	DetailRef    *designEvidenceRef  `json:"detail_ref,omitempty"`
}

type designBoundaryEvidenceSection struct {
	Status                string              `json:"status"`
	Reason                string              `json:"reason,omitempty"`
	PolicyOwner           string              `json:"policy_owner,omitempty"`
	KAHValidationRole     string              `json:"kah_validation_role,omitempty"`
	KAHForbiddenDecisions []string            `json:"kah_forbidden_decisions,omitempty"`
	EvidenceRefs          []designEvidenceRef `json:"evidence_refs,omitempty"`
	DetailRef             *designEvidenceRef  `json:"detail_ref,omitempty"`
}

type designEvidenceRef struct {
	Path     string   `json:"path"`
	Checksum string   `json:"checksum,omitempty"`
	Markers  []string `json:"markers,omitempty"`
}

func designEvidenceBaseline(metadata RunMetadata) ([]byte, error) {
	taskID := metadataTaskID(metadata)
	if taskID == "missing" {
		taskID = ""
	}
	payload := map[string]any{
		"schema_version": designEvidenceSchemaVersion,
		"run_id":         metadata.RunID,
		"task_id":        taskID,
		"task_class":     "",
		"template_note":  "DESIGN-004 bootstrap template only; KAS/KHS must replace placeholders with factual Teal/UI evidence before relying on this artifact.",
		"teal_applicability": map[string]any{
			"project_has_teal_lane":       nil,
			"ui_ux_change":                nil,
			"teal_required":               nil,
			"derivation":                  "project_has_teal_lane && ui_ux_change",
			"ui_ux_classification_owner":  nil,
			"teal_skip_reason":            nil,
			"teal_owner":                  nil,
			"teal_waiver_approved":        false,
			"teal_waiver_approval_ref":    "",
			"teal_waiver_scope":           "",
			"teal_waiver_expires_at":      "",
			"required_when_teal_required": []string{"DESIGN_PLAN_GATE", "DESIGN_FIDELITY_REVIEW"},
			"missing_required_status":     "required_teal_verdict_missing",
		},
		"design_plan_evidence":     designEvidenceBaselineSection(),
		"design_fidelity_evidence": designEvidenceBaselineSection(),
		"color_review_evidence":    designEvidenceBaselineSection(),
		"boundary_evidence": map[string]any{
			"status":                  "pending",
			"reason":                  "Bootstrap placeholder; record KAS/KAH authority boundary evidence before relying on this artifact.",
			"policy_owner":            designBoundaryPolicyOwner,
			"kah_validation_role":     designKAHValidationRole,
			"kah_forbidden_decisions": []string{"classify UI", "select Teal owner", "judge design quality", "score screenshots", "approve waiver", "waive gates"},
			"evidence_refs":           []any{},
		},
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, &Problem{Code: "artifact_baseline_encode_failed", Message: "cannot encode design evidence baseline JSON", Hint: "Retry artifact initialization and preserve stderr if the problem repeats.", Field: "artifact", Expected: "JSON-encodable design baseline payload", Actual: err.Error()}
	}
	return append(data, '\n'), nil
}

func designEvidenceBaselineSection() map[string]any {
	return map[string]any{
		"status":        "pending",
		"reason":        "Bootstrap placeholder; replace with factual evidence or explicit not_applicable reason.",
		"evidence_refs": []any{},
	}
}

func validateDesignEvidenceSchema(relative string, content []byte) []SchemaCheck {
	var raw map[string]any
	if err := json.Unmarshal(content, &raw); err != nil || raw == nil {
		actual := "not an object"
		if err != nil {
			actual = err.Error()
		}
		return []SchemaCheck{schemaFail("json", relative, "file is not valid design evidence JSON", "Fix the file so it contains one JSON object before validating again.", "json", "JSON object", actual)}
	}
	var evidence designEvidence
	_ = mapToStruct(raw, &evidence)
	checks := []SchemaCheck{}
	for _, field := range designEvidenceRequiredFields {
		if _, ok := raw[field]; !ok {
			checks = append(checks, schemaFail(field, relative, "design evidence required field is missing", "Use the design004.v1 evidence schema.", field, "present", "missing"))
		} else {
			checks = append(checks, schemaPass(field, relative, "design evidence required field is present"))
		}
	}
	checks = append(checks, validateDesignEvidenceIdentity(relative, evidence)...)
	checks = append(checks, validateDesignTealApplicability(relative, evidence.TealApplicability)...)
	checks = append(checks, validateDesignRawFieldTypes(relative, raw)...)
	checks = append(checks, validateDesignEvidenceSectionShape(relative, "design_plan_evidence", evidence.DesignPlanEvidence)...)
	checks = append(checks, validateDesignEvidenceSectionShape(relative, "design_fidelity_evidence", evidence.DesignFidelityEvidence)...)
	checks = append(checks, validateDesignEvidenceSectionShape(relative, "color_review_evidence", evidence.ColorReviewEvidence)...)
	checks = append(checks, validateDesignBoundaryEvidenceShape(relative, evidence.BoundaryEvidence)...)
	return checks
}

func validateDesignRawFieldTypes(relative string, raw map[string]any) []SchemaCheck {
	checks := []SchemaCheck{}
	if teal, ok := raw["teal_applicability"].(map[string]any); ok {
		for _, field := range []string{"ui_ux_classification_owner", "teal_skip_reason", "teal_owner"} {
			checks = append(checks, validateDesignRawNullableString(relative, "teal_applicability."+field, teal[field], fieldPresent(teal, field)))
		}
		for _, field := range []string{"derivation", "teal_waiver_approval_ref", "teal_waiver_scope", "teal_waiver_expires_at", "missing_required_status"} {
			checks = append(checks, validateDesignRawOptionalString(relative, "teal_applicability."+field, teal[field], fieldPresent(teal, field)))
		}
		checks = append(checks, validateDesignRawStringArray(relative, "teal_applicability.required_when_teal_required", teal["required_when_teal_required"], fieldPresent(teal, "required_when_teal_required")))
	}
	for _, section := range []string{"design_plan_evidence", "design_fidelity_evidence", "color_review_evidence", "boundary_evidence"} {
		checks = append(checks, validateDesignRawSectionTypes(relative, section, raw[section])...)
	}
	return checks
}

func validateDesignRawSectionTypes(relative, name string, value any) []SchemaCheck {
	section, ok := value.(map[string]any)
	if !ok {
		if value == nil {
			return nil
		}
		return []SchemaCheck{schemaFail(name, relative, "design evidence section has an invalid type", "Record this section as a JSON object.", name, "object", fmt.Sprintf("%T", value))}
	}
	checks := []SchemaCheck{}
	checks = append(checks, validateDesignRawOptionalString(relative, name+".reason", section["reason"], fieldPresent(section, "reason")))
	checks = append(checks, validateDesignRawRefs(relative, name+".evidence_refs", section["evidence_refs"], fieldPresent(section, "evidence_refs"))...)
	checks = append(checks, validateDesignRawRef(relative, name+".detail_ref", section["detail_ref"], fieldPresent(section, "detail_ref"))...)
	if name == "boundary_evidence" {
		for _, field := range []string{"policy_owner", "kah_validation_role"} {
			checks = append(checks, validateDesignRawOptionalString(relative, name+"."+field, section[field], fieldPresent(section, field)))
		}
		checks = append(checks, validateDesignRawStringArray(relative, name+".kah_forbidden_decisions", section["kah_forbidden_decisions"], fieldPresent(section, "kah_forbidden_decisions")))
	}
	return checks
}

func validateDesignRawRefs(relative, field string, value any, present bool) []SchemaCheck {
	if !present {
		return nil
	}
	items, ok := value.([]any)
	if !ok {
		return []SchemaCheck{schemaFail(field, relative, "design evidence refs field has an invalid type", "Record evidence_refs as an array of objects.", field, "array", fmt.Sprintf("%T", value))}
	}
	checks := []SchemaCheck{}
	for i, item := range items {
		checks = append(checks, validateDesignRawRef(relative, fmt.Sprintf("%s[%d]", field, i), item, true)...)
	}
	return checks
}

func validateDesignRawRef(relative, field string, value any, present bool) []SchemaCheck {
	if !present || value == nil {
		return nil
	}
	ref, ok := value.(map[string]any)
	if !ok {
		return []SchemaCheck{schemaFail(field, relative, "design evidence ref has an invalid type", "Record evidence refs as JSON objects.", field, "object", fmt.Sprintf("%T", value))}
	}
	checks := []SchemaCheck{}
	if path, ok := ref["path"]; ok {
		if _, ok := path.(string); !ok {
			checks = append(checks, schemaFail(field+".path", relative, "design evidence ref path has an invalid type", "Record path as a string.", field+".path", "string", fmt.Sprintf("%T", path)))
		}
	}
	if checksum, ok := ref["checksum"]; ok {
		if _, ok := checksum.(string); !ok {
			checks = append(checks, schemaFail(field+".checksum", relative, "design evidence checksum has an invalid type", "Record checksum as a string.", field+".checksum", "string", fmt.Sprintf("%T", checksum)))
		}
	}
	checks = append(checks, validateDesignRawStringArray(relative, field+".markers", ref["markers"], fieldPresent(ref, "markers")))
	return checks
}

func validateDesignRawNullableString(relative, field string, value any, present bool) SchemaCheck {
	if !present || value == nil {
		return schemaPass(field, relative, "design evidence nullable string field is valid")
	}
	if _, ok := value.(string); !ok {
		return schemaFail(field, relative, "design evidence nullable string field has an invalid type", "Record this field with a string value or null.", field, "string or null", fmt.Sprintf("%T", value))
	}
	return schemaPass(field, relative, "design evidence nullable string field is valid")
}

func validateDesignRawOptionalString(relative, field string, value any, present bool) SchemaCheck {
	if !present {
		return schemaPass(field, relative, "design evidence optional string field is absent")
	}
	if _, ok := value.(string); !ok {
		return schemaFail(field, relative, "design evidence optional string field has an invalid type", "Record this field with a string value.", field, "string", fmt.Sprintf("%T", value))
	}
	return schemaPass(field, relative, "design evidence optional string field is valid")
}

func validateDesignRawStringArray(relative, field string, value any, present bool) SchemaCheck {
	if !present {
		return schemaPass(field, relative, "design evidence optional string array is absent")
	}
	items, ok := value.([]any)
	if !ok {
		return schemaFail(field, relative, "design evidence string array has an invalid type", "Record this field as an array of strings.", field, "array of strings", fmt.Sprintf("%T", value))
	}
	for _, item := range items {
		if text, ok := item.(string); !ok || strings.TrimSpace(text) == "" {
			return schemaFail(field, relative, "design evidence string array contains an invalid item", "Record only non-empty strings.", field, "array of non-empty strings", fmt.Sprintf("%v", item))
		}
	}
	return schemaPass(field, relative, "design evidence string array shape is valid")
}

func fieldPresent(values map[string]any, field string) bool {
	_, ok := values[field]
	return ok
}

func validateDesignEvidenceIdentity(relative string, evidence designEvidence) []SchemaCheck {
	checks := []SchemaCheck{}
	if evidence.SchemaVersion == designEvidenceSchemaVersion {
		checks = append(checks, schemaPass("schema_version", relative, "design evidence schema version is supported"))
	} else {
		checks = append(checks, schemaFail("schema_version", relative, "design evidence schema version is unsupported", "Use schema_version design004.v1.", "schema_version", designEvidenceSchemaVersion, missingIfBlank(evidence.SchemaVersion)))
	}
	if runIDPattern.MatchString(evidence.RunID) {
		checks = append(checks, schemaPass("run_id", relative, "design evidence run_id has supported shape"))
	} else {
		checks = append(checks, schemaFail("run_id", relative, "design evidence run_id is invalid", "Record a KAH run id in run_id.", "run_id", "run-YYYYMMDDTHHMMSSZ-<12hex>", missingIfBlank(evidence.RunID)))
	}
	if strings.TrimSpace(evidence.TaskID) == "" {
		checks = append(checks, schemaFail("task_id", relative, "design evidence task_id is missing", "Record the KAS/KAH task id.", "task_id", "non-empty string", "missing"))
	} else {
		checks = append(checks, schemaPass("task_id", relative, "design evidence task_id is recorded"))
	}
	if strings.TrimSpace(evidence.TaskClass) == "" {
		checks = append(checks, schemaFail("task_class", relative, "design evidence task_class is missing", "Record the deterministic task class.", "task_class", "non-empty string", "missing"))
	} else {
		checks = append(checks, schemaPass("task_class", relative, "design evidence task_class is recorded"))
	}
	return checks
}

func validateDesignTealApplicability(relative string, teal *designTealApplicability) []SchemaCheck {
	if teal == nil {
		return []SchemaCheck{schemaFail("teal_applicability", relative, "design evidence lacks Teal applicability object", "Record KAS-derived Teal applicability fields.", "teal_applicability", "object", "missing")}
	}
	checks := []SchemaCheck{}
	checks = append(checks, validateDesignBoolPointer(relative, "teal_applicability.project_has_teal_lane", teal.ProjectHasTealLane))
	checks = append(checks, validateDesignBoolPointer(relative, "teal_applicability.ui_ux_change", teal.UIUXChange))
	checks = append(checks, validateDesignBoolPointer(relative, "teal_applicability.teal_required", teal.TealRequired))
	if strings.TrimSpace(teal.Derivation) != "" && teal.Derivation != "project_has_teal_lane && ui_ux_change" {
		checks = append(checks, schemaFail("teal_applicability.derivation", relative, "Teal applicability derivation is unsupported", "Use project_has_teal_lane && ui_ux_change.", "teal_applicability.derivation", "project_has_teal_lane && ui_ux_change", teal.Derivation))
	} else {
		checks = append(checks, schemaPass("teal_applicability.derivation", relative, "Teal applicability derivation shape is supported"))
	}
	if teal.ProjectHasTealLane != nil && teal.UIUXChange != nil && teal.TealRequired != nil {
		expected := *teal.ProjectHasTealLane && *teal.UIUXChange
		if *teal.TealRequired != expected {
			checks = append(checks, schemaFail("teal_applicability.teal_required", relative, "teal_required does not match KAS derivation", "Keep teal_required equal to project_has_teal_lane && ui_ux_change.", "teal_applicability.teal_required", fmt.Sprintf("%t", expected), fmt.Sprintf("%t", *teal.TealRequired)))
		} else {
			checks = append(checks, schemaPass("teal_applicability.teal_required", relative, "teal_required matches KAS derivation"))
		}
		if !*teal.TealRequired && strings.TrimSpace(stringPtrValue(teal.TealSkipReason)) == "" {
			checks = append(checks, schemaFail("teal_applicability.teal_skip_reason", relative, "non-UI design evidence requires a concrete skip reason", "Record the KAS skip reason when teal_required is false.", "teal_applicability.teal_skip_reason", "non-empty string", "missing"))
		}
	}
	checks = append(checks, validateDesignNullableString(relative, "teal_applicability.ui_ux_classification_owner", teal.UIUXClassificationOwner))
	checks = append(checks, validateDesignNullableString(relative, "teal_applicability.teal_skip_reason", teal.TealSkipReason))
	checks = append(checks, validateDesignNullableString(relative, "teal_applicability.teal_owner", teal.TealOwner))
	if teal.TealWaiverApproved == nil {
		checks = append(checks, schemaFail("teal_applicability.teal_waiver_approved", relative, "Teal waiver approval flag is missing", "Record teal_waiver_approved as true or false.", "teal_applicability.teal_waiver_approved", "boolean", "missing"))
	} else {
		checks = append(checks, schemaPass("teal_applicability.teal_waiver_approved", relative, "Teal waiver approval flag is valid"))
	}
	checks = append(checks, validateDesignStringList(relative, "teal_applicability.required_when_teal_required", teal.RequiredWhenTealRequired, false))
	return checks
}

func validateDesignBoolPointer(relative, field string, value *bool) SchemaCheck {
	if value == nil {
		return schemaFail(field, relative, "design evidence boolean field is missing or null", "Record this KAS-owned fact as true or false.", field, "boolean", "missing")
	}
	return schemaPass(field, relative, "design evidence boolean field is valid")
}

func validateDesignNullableString(relative, field string, value *string) SchemaCheck {
	if value == nil {
		return schemaPass(field, relative, "design evidence nullable string field is valid")
	}
	return schemaPass(field, relative, "design evidence nullable string field is valid")
}

func validateDesignEvidenceSectionShape(relative, name string, section *designEvidenceSection) []SchemaCheck {
	if section == nil {
		return []SchemaCheck{schemaFail(name, relative, "design evidence section is missing", "Record the required design004.v1 section.", name, "object", "missing")}
	}
	checks := []SchemaCheck{validateDesignSectionStatus(relative, name, section.Status, section.Reason)}
	checks = append(checks, validateDesignRefs(relative, name+".evidence_refs", section.EvidenceRefs)...)
	if section.DetailRef != nil {
		checks = append(checks, validateDesignRef(relative, name+".detail_ref", *section.DetailRef)...)
	}
	return checks
}

func validateDesignBoundaryEvidenceShape(relative string, section *designBoundaryEvidenceSection) []SchemaCheck {
	if section == nil {
		return []SchemaCheck{schemaFail("boundary_evidence", relative, "design boundary evidence section is missing", "Record KAS/KAH authority boundary fields.", "boundary_evidence", "object", "missing")}
	}
	checks := []SchemaCheck{validateDesignSectionStatus(relative, "boundary_evidence", section.Status, section.Reason)}
	if section.PolicyOwner != designBoundaryPolicyOwner {
		actual := missingIfBlank(section.PolicyOwner)
		checks = append(checks, schemaFail("boundary_evidence.policy_owner", relative, "design boundary policy owner is unsupported", "Record KAS as the policy owner without transferring authority to KAH.", "boundary_evidence.policy_owner", designBoundaryPolicyOwner, actual))
	} else {
		checks = append(checks, schemaPass("boundary_evidence.policy_owner", relative, "design boundary policy owner is KAS"))
	}
	if section.KAHValidationRole != designKAHValidationRole {
		actual := missingIfBlank(section.KAHValidationRole)
		checks = append(checks, schemaFail("boundary_evidence.kah_validation_role", relative, "KAH validation role is unsupported", "Record deterministic_shape_only for the KAH validation role.", "boundary_evidence.kah_validation_role", designKAHValidationRole, actual))
	} else {
		checks = append(checks, schemaPass("boundary_evidence.kah_validation_role", relative, "KAH validation role is deterministic shape only"))
	}
	checks = append(checks, validateDesignStringList(relative, "boundary_evidence.kah_forbidden_decisions", section.KAHForbiddenDecisions, false))
	checks = append(checks, validateDesignRefs(relative, "boundary_evidence.evidence_refs", section.EvidenceRefs)...)
	if section.DetailRef != nil {
		checks = append(checks, validateDesignRef(relative, "boundary_evidence.detail_ref", *section.DetailRef)...)
	}
	return checks
}

func validateDesignSectionStatus(relative, name, status, reason string) SchemaCheck {
	switch status {
	case GateStatusPass, GateStatusFail, "pending":
		return schemaPass(name+".status", relative, "design evidence section status shape is valid")
	case GateStatusNotApplicable:
		if strings.TrimSpace(reason) == "" {
			return schemaFail(name+".reason", relative, "not_applicable design evidence section requires a reason", "Record a deterministic not_applicable reason.", name+".reason", "non-empty reason", "missing")
		}
		return schemaPass(name+".status", relative, "design evidence not_applicable status has a reason")
	default:
		return schemaFail(name+".status", relative, "design evidence status vocabulary is unsupported", "Use pass, fail, pending, or not_applicable.", name+".status", "pass, fail, pending, or not_applicable", missingIfBlank(status))
	}
}

func validateDesignRefs(relative, field string, refs []designEvidenceRef) []SchemaCheck {
	checks := []SchemaCheck{}
	for i, ref := range refs {
		checks = append(checks, validateDesignRef(relative, fmt.Sprintf("%s[%d]", field, i), ref)...)
	}
	return checks
}

func validateDesignRef(relative, field string, ref designEvidenceRef) []SchemaCheck {
	checks := []SchemaCheck{}
	if strings.TrimSpace(ref.Path) == "" {
		checks = append(checks, schemaFail(field+".path", relative, "design evidence ref path is missing", "Record a repository-confined relative path.", field+".path", "repository-relative path", "missing"))
	} else if !designRelativePathShape(ref.Path) {
		checks = append(checks, schemaFail(field+".path", relative, "design evidence ref path is not repository-confined", "Use a clean relative path without parent traversal or root aliases.", field+".path", "repository-confined relative path", ref.Path))
	} else {
		checks = append(checks, schemaPass(field+".path", relative, "design evidence ref path shape is repository-confined"))
	}
	if strings.TrimSpace(ref.Checksum) != "" {
		if !tokenEconomyChecksumPattern.MatchString(ref.Checksum) {
			checks = append(checks, schemaFail(field+".checksum", relative, "design evidence checksum has an unsupported format", "Use sha256:<64 hex characters>.", field+".checksum", "sha256:<64hex>", ref.Checksum))
		} else {
			checks = append(checks, schemaPass(field+".checksum", relative, "design evidence checksum format is supported"))
		}
	}
	for i, marker := range ref.Markers {
		if strings.TrimSpace(marker) == "" {
			checks = append(checks, schemaFail(fmt.Sprintf("%s.markers[%d]", field, i), relative, "design evidence marker is empty", "Record only non-empty marker strings.", fmt.Sprintf("%s.markers[%d]", field, i), "non-empty string", "empty"))
		}
	}
	return checks
}

func designRelativePathShape(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || strings.Contains(value, `\`) {
		return false
	}
	value = filepath.ToSlash(value)
	if strings.HasPrefix(value, "/") || value == "." || value == ".." {
		return false
	}
	if strings.HasPrefix(value, "../") || strings.Contains(value, "/../") {
		return false
	}
	clean := filepath.Clean(value)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || strings.Contains(clean, "/../") {
		return false
	}
	return true
}

func validateDesignStringList(relative, field string, values []string, requireNonEmpty bool) SchemaCheck {
	if requireNonEmpty && len(values) == 0 {
		return schemaFail(field, relative, "design evidence string array is empty", "Record one or more strings.", field, "one or more strings", "empty")
	}
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return schemaFail(field, relative, "design evidence string array contains an empty item", "Record only non-empty strings.", field, "array of non-empty strings", "empty item")
		}
	}
	return schemaPass(field, relative, "design evidence string array shape is valid")
}

func stringPtrValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
