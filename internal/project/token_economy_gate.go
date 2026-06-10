package project

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
)

const (
	tokenEconomyArtifact                = "token-economy-evidence.json"
	tokenEconomySchemaVersion           = "token001.v1"
	tokenEconomyTaskID                  = "token-001"
	tokenEconomyToken002SchemaVersion   = "token002.v1"
	tokenEconomyToken002TaskID          = "token-002"
	tokenEconomySupportedSchemaVersions = "token001.v1,token002.v1"
)

var tokenEconomyChecksumPattern = regexp.MustCompile(`^sha256:[0-9a-fA-F]{64}$`)

type tokenEconomyEvidence struct {
	SchemaVersion            string                  `json:"schema_version"`
	RunID                    string                  `json:"run_id"`
	TaskID                   string                  `json:"task_id"`
	TaskClass                string                  `json:"task_class"`
	Scope                    *tokenEvidenceSection   `json:"scope"`
	CompactOutputPolicy      *tokenEvidenceSection   `json:"compact_output_policy"`
	ArtifactFirstDetail      *tokenEvidenceSection   `json:"artifact_first_detail"`
	AgentInstructionEvidence *tokenEvidenceSection   `json:"agent_instruction_evidence"`
	FinalReportEvidence      *tokenEvidenceSection   `json:"final_report_evidence"`
	KASLifecycleEvidence     *tokenLifecycleEvidence `json:"kas_lifecycle_evidence"`
	MutationApprovalEvidence *tokenMutationEvidence  `json:"mutation_approval_evidence"`
}

type tokenEvidenceSection struct {
	Status       string             `json:"status"`
	Reason       string             `json:"reason,omitempty"`
	EvidenceRefs []tokenEvidenceRef `json:"evidence_refs,omitempty"`
	DetailRef    *tokenEvidenceRef  `json:"detail_ref,omitempty"`
}

type tokenEvidenceRef struct {
	Path     string   `json:"path"`
	Checksum string   `json:"checksum,omitempty"`
	Markers  []string `json:"markers,omitempty"`
}

type tokenLifecycleEvidence struct {
	Status         string             `json:"status"`
	Reason         string             `json:"reason,omitempty"`
	LifecycleVerb  string             `json:"lifecycle_verb,omitempty"`
	DryRunRef      *tokenEvidenceRef  `json:"dry_run_ref,omitempty"`
	ApprovalRef    *tokenEvidenceRef  `json:"approval_ref,omitempty"`
	ManifestRefs   []tokenEvidenceRef `json:"manifest_refs,omitempty"`
	DoctorRef      *tokenEvidenceRef  `json:"doctor_ref,omitempty"`
	RoleLabels     []string           `json:"role_labels,omitempty"`
	TargetPaths    []string           `json:"target_paths,omitempty"`
	ChecksumFields []string           `json:"checksum_fields,omitempty"`
}

type tokenMutationEvidence struct {
	Status                string             `json:"status"`
	Reason                string             `json:"reason,omitempty"`
	MutationScope         string             `json:"mutation_scope,omitempty"`
	ClaimedBroadMutations []string           `json:"claimed_broad_mutations,omitempty"`
	ApprovalRefs          []tokenEvidenceRef `json:"approval_refs,omitempty"`
}

func checkTokenEconomyGate(root Root, metadata RunMetadata) (GateCheckResult, error) {
	taskID := metadataTaskID(metadata)
	if taskID != tokenEconomyTaskID && taskID != tokenEconomyToken002TaskID {
		check := GateCheck{Name: "token_task_scope", Status: GateStatusNotApplicable, Message: "token-economy evidence is only required for token-001 or token-002 runs", Field: "task_id", Expected: tokenEconomyTaskID + " or " + tokenEconomyToken002TaskID, Actual: taskID}
		return GateCheckResult{RunID: metadata.RunID, Gate: GateTokenEconomy, Status: GateStatusNotApplicable, Checks: []GateCheck{check}}, nil
	}

	path, err := artifactPath(root, metadata.RunID, tokenEconomyArtifact)
	if err != nil {
		check := GateCheck{Name: "token_economy_artifact", Status: GateStatusFail, Message: "token-economy artifact path is invalid", Hint: "Use artifact init to create canonical token-economy evidence.", Field: "path", Expected: tokenEconomyArtifact, Actual: err.Error()}
		return gateResultFromChecks(metadata.RunID, GateTokenEconomy, []GateCheck{check}), nil
	}
	info, err := os.Lstat(path.Absolute)
	if os.IsNotExist(err) {
		check := GateCheck{Name: "token_economy_artifact", Status: GateStatusFail, Path: path.Relative, Message: "required token-economy evidence artifact is missing", Hint: "Write token-economy-evidence.json for the token task before checking the gate.", Field: "path", Expected: "existing regular file", Actual: "missing"}
		return gateResultFromChecks(metadata.RunID, GateTokenEconomy, []GateCheck{check}), nil
	}
	if err != nil {
		check := GateCheck{Name: "token_economy_artifact", Status: GateStatusFail, Path: path.Relative, Message: "cannot inspect token-economy evidence artifact", Hint: "Check run artifact permissions before checking the token-economy gate.", Field: "path", Expected: "inspectable regular file", Actual: err.Error()}
		return gateResultFromChecks(metadata.RunID, GateTokenEconomy, []GateCheck{check}), nil
	}
	if !info.Mode().IsRegular() {
		check := GateCheck{Name: "token_economy_artifact", Status: GateStatusFail, Path: path.Relative, Message: "token-economy evidence artifact must be a regular file", Hint: "Move the conflicting path and rewrite the evidence artifact.", Field: "path", Expected: "regular file", Actual: fileKind(info)}
		return gateResultFromChecks(metadata.RunID, GateTokenEconomy, []GateCheck{check}), nil
	}
	content, err := os.ReadFile(path.Absolute)
	if err != nil {
		check := GateCheck{Name: "token_economy_artifact", Status: GateStatusFail, Path: path.Relative, Message: "cannot read token-economy evidence artifact", Hint: "Check run artifact permissions before checking the token-economy gate.", Field: "path", Expected: "readable file", Actual: err.Error()}
		return gateResultFromChecks(metadata.RunID, GateTokenEconomy, []GateCheck{check}), nil
	}

	checks := validateTokenEconomyEvidence(root, metadata, path.Relative, content)
	result := tokenEconomyResultFromChecks(metadata.RunID, checks)
	return result, nil
}

func validateTokenEconomyEvidence(root Root, metadata RunMetadata, relative string, content []byte) []GateCheck {
	checks := []GateCheck{{Name: "token_economy_artifact", Status: GateStatusPass, Path: relative, Message: "token-economy evidence artifact is present"}}
	var raw map[string]any
	if err := json.Unmarshal(content, &raw); err != nil || raw == nil {
		actual := "not an object"
		if err != nil {
			actual = err.Error()
		}
		return append(checks, GateCheck{Name: "token_economy_json", Status: GateStatusFail, Path: relative, Message: "token-economy evidence is malformed JSON", Hint: "Write token-economy-evidence.json as a JSON object.", Field: "json", Expected: "JSON object", Actual: actual})
	}
	if metadataTaskID(metadata) == tokenEconomyToken002TaskID {
		return append(checks, validateToken002EconomyEvidence(root, metadata, relative, content, raw)...)
	}
	return append(checks, validateToken001EconomyEvidence(root, metadata, relative, content, raw)...)
}

func validateToken001EconomyEvidence(root Root, metadata RunMetadata, relative string, content []byte, raw map[string]any) []GateCheck {
	checks := []GateCheck{}
	checks = append(checks, rejectToken002OnlyFields(relative, raw)...)

	var evidence tokenEconomyEvidence
	if err := json.Unmarshal(content, &evidence); err != nil {
		return append(checks, GateCheck{Name: "token_economy_json", Status: GateStatusFail, Path: relative, Message: "token-economy evidence cannot be decoded", Hint: "Use the token001.v1 evidence schema.", Field: "json", Expected: "token001.v1 object", Actual: err.Error()})
	}
	checks = append(checks, validateTokenEconomySchemaFields(relative, raw, evidence)...)
	checks = append(checks, validateTokenEconomyIdentity(metadata, relative, evidence)...)
	checks = append(checks, validateTokenSection(root, relative, "scope", evidence.Scope, false)...)
	checks = append(checks, validateTokenSection(root, relative, "compact_output_policy", evidence.CompactOutputPolicy, true)...)
	checks = append(checks, validateTokenSection(root, relative, "artifact_first_detail", evidence.ArtifactFirstDetail, true)...)
	checks = append(checks, validateTokenSection(root, relative, "agent_instruction_evidence", evidence.AgentInstructionEvidence, true)...)
	checks = append(checks, validateTokenSection(root, relative, "final_report_evidence", evidence.FinalReportEvidence, true)...)
	checks = append(checks, validateTokenLifecycleEvidence(root, relative, evidence.KASLifecycleEvidence)...)
	checks = append(checks, validateTokenMutationEvidence(root, relative, evidence.MutationApprovalEvidence)...)
	return checks
}

func rejectToken002OnlyFields(relative string, raw map[string]any) []GateCheck {
	checks := []GateCheck{}
	for _, field := range []string{"token_002", "verification_profile_evidence", "evidence_summary", "review_bundle", "watcher_terminal_report", "change_verification_matrix"} {
		if _, ok := raw[field]; ok {
			checks = append(checks, GateCheck{Name: "token002_scope", Status: GateStatusFail, Path: relative, Message: "token-002-only evidence is out of scope for token-001", Hint: "Remove token-002 evidence fields from token-economy-evidence.json for this gate.", Field: field, Expected: "absent", Actual: "present"})
		}
	}
	if version, _ := raw["schema_version"].(string); strings.HasPrefix(strings.ToLower(version), "token002") {
		checks = append(checks, GateCheck{Name: "token002_scope", Status: GateStatusFail, Path: relative, Message: "token-002 schema version is out of scope for token-001", Hint: "Use schema_version token001.v1 for this gate.", Field: "schema_version", Expected: tokenEconomySchemaVersion, Actual: version})
	}
	return checks
}

func validateTokenEconomySchemaFields(relative string, raw map[string]any, evidence tokenEconomyEvidence) []GateCheck {
	checks := []GateCheck{}
	for _, field := range []string{"schema_version", "run_id", "task_id", "task_class", "scope", "compact_output_policy", "artifact_first_detail", "agent_instruction_evidence", "final_report_evidence", "kas_lifecycle_evidence", "mutation_approval_evidence"} {
		if _, ok := raw[field]; !ok {
			checks = append(checks, GateCheck{Name: "token_required_field", Status: GateStatusFail, Path: relative, Message: "token-economy evidence is missing a required field", Hint: "Write all token001.v1 required fields before checking the gate.", Field: field, Expected: "present", Actual: "missing"})
		}
	}
	if evidence.SchemaVersion == tokenEconomySchemaVersion {
		checks = append(checks, GateCheck{Name: "schema_version", Status: GateStatusPass, Path: relative, Message: "token-economy schema version is supported", Field: "schema_version", Actual: evidence.SchemaVersion})
	} else {
		actual := evidence.SchemaVersion
		if strings.TrimSpace(actual) == "" {
			actual = "missing"
		}
		checks = append(checks, GateCheck{Name: "schema_version", Status: GateStatusFail, Path: relative, Message: "token-economy schema version is unsupported", Hint: "Use schema_version token001.v1.", Field: "schema_version", Expected: tokenEconomySchemaVersion, Actual: actual})
	}
	if strings.TrimSpace(evidence.TaskClass) == "" {
		checks = append(checks, GateCheck{Name: "task_class", Status: GateStatusFail, Path: relative, Message: "token-economy evidence is missing task_class", Hint: "Record the deterministic KAS task class in task_class.", Field: "task_class", Expected: "non-empty string", Actual: "missing"})
	} else {
		checks = append(checks, GateCheck{Name: "task_class", Status: GateStatusPass, Path: relative, Message: "task_class is recorded", Field: "task_class", Actual: evidence.TaskClass})
	}
	return checks
}

func validateTokenEconomyIdentity(metadata RunMetadata, relative string, evidence tokenEconomyEvidence) []GateCheck {
	checks := []GateCheck{}
	if evidence.RunID == metadata.RunID {
		checks = append(checks, GateCheck{Name: "run_id", Status: GateStatusPass, Path: relative, Message: "run_id matches run metadata", Field: "run_id", Actual: evidence.RunID})
	} else {
		checks = append(checks, GateCheck{Name: "run_id", Status: GateStatusFail, Path: relative, Message: "run_id does not match run metadata", Hint: "Record the current run id in token-economy-evidence.json.", Field: "run_id", Expected: metadata.RunID, Actual: evidence.RunID})
	}
	if evidence.TaskID == tokenEconomyTaskID {
		checks = append(checks, GateCheck{Name: "task_id", Status: GateStatusPass, Path: relative, Message: "task_id is token-001", Field: "task_id", Actual: evidence.TaskID})
	} else {
		actual := evidence.TaskID
		if strings.TrimSpace(actual) == "" {
			actual = "missing"
		}
		checks = append(checks, GateCheck{Name: "task_id", Status: GateStatusFail, Path: relative, Message: "task_id is not token-001", Hint: "The token-economy gate validates only token-001 evidence.", Field: "task_id", Expected: tokenEconomyTaskID, Actual: actual})
	}
	return checks
}

func validateTokenSection(root Root, artifactRelative string, name string, section *tokenEvidenceSection, requireEvidence bool) []GateCheck {
	if section == nil {
		return []GateCheck{{Name: name, Status: GateStatusFail, Path: artifactRelative, Message: "token-economy evidence section is missing", Hint: "Record the required token001.v1 section.", Field: name, Expected: "object", Actual: "missing"}}
	}
	checks := []GateCheck{validateTokenStatus(artifactRelative, name, section.Status, section.Reason)}
	if section.Status == GateStatusNotApplicable {
		return checks
	}
	if section.Status != GateStatusPass {
		return checks
	}
	refs := append([]tokenEvidenceRef{}, section.EvidenceRefs...)
	if section.DetailRef != nil {
		refs = append(refs, *section.DetailRef)
	}
	if requireEvidence && len(refs) == 0 {
		checks = append(checks, GateCheck{Name: name + "_refs", Status: GateStatusFail, Path: artifactRelative, Message: "token-economy evidence section lacks evidence refs", Hint: "Record at least one repository-confined evidence ref for this section.", Field: name + ".evidence_refs", Expected: "one or more refs", Actual: "missing"})
	}
	for i, ref := range refs {
		checks = append(checks, validateTokenEvidenceRef(root, artifactRelative, fmt.Sprintf("%s.evidence_refs[%d]", name, i), ref)...)
	}
	return checks
}

func validateTokenLifecycleEvidence(root Root, artifactRelative string, evidence *tokenLifecycleEvidence) []GateCheck {
	if evidence == nil {
		return []GateCheck{{Name: "kas_lifecycle_evidence", Status: GateStatusFail, Path: artifactRelative, Message: "KAS lifecycle evidence section is missing", Hint: "Record kas_lifecycle_evidence with pass evidence or not_applicable reason.", Field: "kas_lifecycle_evidence", Expected: "object", Actual: "missing"}}
	}
	checks := []GateCheck{validateTokenStatus(artifactRelative, "kas_lifecycle_evidence", evidence.Status, evidence.Reason)}
	if evidence.Status == GateStatusNotApplicable || evidence.Status != GateStatusPass {
		return checks
	}
	if !allowed(evidence.LifecycleVerb, "install", "update", "doctor", "repair", "uninstall") {
		checks = append(checks, GateCheck{Name: "kas_lifecycle_verb", Status: GateStatusFail, Path: artifactRelative, Message: "KAS lifecycle verb is unsupported", Hint: "Use an operator-facing lifecycle verb from the token-001 contract.", Field: "kas_lifecycle_evidence.lifecycle_verb", Expected: "install,update,doctor,repair,uninstall", Actual: evidence.LifecycleVerb})
	}
	for name, ref := range map[string]*tokenEvidenceRef{"dry_run_ref": evidence.DryRunRef, "approval_ref": evidence.ApprovalRef, "doctor_ref": evidence.DoctorRef} {
		if ref == nil {
			checks = append(checks, GateCheck{Name: "kas_lifecycle_" + name, Status: GateStatusFail, Path: artifactRelative, Message: "KAS lifecycle evidence is missing a required reference", Hint: "Record dry-run, approval/apply, and doctor evidence refs when lifecycle evidence is in scope.", Field: "kas_lifecycle_evidence." + name, Expected: "evidence ref", Actual: "missing"})
			continue
		}
		checks = append(checks, validateTokenEvidenceRef(root, artifactRelative, "kas_lifecycle_evidence."+name, *ref)...)
	}
	if len(evidence.ManifestRefs) == 0 {
		checks = append(checks, GateCheck{Name: "kas_lifecycle_manifest_refs", Status: GateStatusFail, Path: artifactRelative, Message: "KAS lifecycle evidence lacks manifest refs", Hint: "Record per-profile manifest evidence refs when lifecycle evidence is in scope.", Field: "kas_lifecycle_evidence.manifest_refs", Expected: "one or more refs", Actual: "missing"})
	}
	for i, ref := range evidence.ManifestRefs {
		checks = append(checks, validateTokenEvidenceRef(root, artifactRelative, fmt.Sprintf("kas_lifecycle_evidence.manifest_refs[%d]", i), ref)...)
	}
	if len(evidence.RoleLabels) == 0 || len(evidence.TargetPaths) == 0 || len(evidence.ChecksumFields) == 0 {
		checks = append(checks, GateCheck{Name: "kas_lifecycle_fields", Status: GateStatusFail, Path: artifactRelative, Message: "KAS lifecycle evidence lacks required mechanical fields", Hint: "Record role labels, target paths, and checksum fields when lifecycle evidence is in scope.", Field: "kas_lifecycle_evidence", Expected: "role_labels,target_paths,checksum_fields", Actual: "missing one or more"})
	}
	for i, target := range evidence.TargetPaths {
		if _, err := ResolveRelativePath(root, target); err != nil {
			checks = append(checks, GateCheck{Name: "kas_lifecycle_target_path", Status: GateStatusFail, Path: artifactRelative, Message: "KAS lifecycle target path is unsafe", Hint: "Use repository-relative target paths only.", Field: fmt.Sprintf("kas_lifecycle_evidence.target_paths[%d]", i), Expected: "repository-confined relative path", Actual: err.Error()})
		}
	}
	return checks
}

func validateTokenMutationEvidence(root Root, artifactRelative string, evidence *tokenMutationEvidence) []GateCheck {
	if evidence == nil {
		return []GateCheck{{Name: "mutation_approval_evidence", Status: GateStatusFail, Path: artifactRelative, Message: "mutation approval evidence section is missing", Hint: "Record mutation_approval_evidence with pass evidence or not_applicable reason.", Field: "mutation_approval_evidence", Expected: "object", Actual: "missing"}}
	}
	checks := []GateCheck{validateTokenStatus(artifactRelative, "mutation_approval_evidence", evidence.Status, evidence.Reason)}
	if evidence.Status == GateStatusNotApplicable || evidence.Status != GateStatusPass {
		return checks
	}
	if !allowed(evidence.MutationScope, "none", "narrow", "broad") {
		checks = append(checks, GateCheck{Name: "mutation_scope", Status: GateStatusFail, Path: artifactRelative, Message: "mutation approval vocabulary is unsupported", Hint: "Use mutation_scope none, narrow, or broad.", Field: "mutation_approval_evidence.mutation_scope", Expected: "none,narrow,broad", Actual: evidence.MutationScope})
	}
	broadClaim := evidence.MutationScope == "broad" || len(evidence.ClaimedBroadMutations) > 0
	if broadClaim && len(evidence.ApprovalRefs) == 0 {
		checks = append(checks, GateCheck{Name: "mutation_approval_refs", Status: GateStatusFail, Path: artifactRelative, Message: "broad mutation claims require explicit approval evidence", Hint: "Add approval_refs for broad KAB/Hermes/runtime/profile/provider/gateway/auth/token/model mutation claims or remove the broad claim.", Field: "mutation_approval_evidence.approval_refs", Expected: "one or more approval refs", Actual: "missing"})
	}
	for i, ref := range evidence.ApprovalRefs {
		checks = append(checks, validateTokenEvidenceRef(root, artifactRelative, fmt.Sprintf("mutation_approval_evidence.approval_refs[%d]", i), ref)...)
	}
	return checks
}

func validateTokenStatus(relative string, name string, status string, reason string) GateCheck {
	status = strings.TrimSpace(status)
	switch status {
	case GateStatusPass:
		return GateCheck{Name: name, Status: GateStatusPass, Path: relative, Message: "token-economy evidence section status is pass", Field: name + ".status", Actual: status}
	case GateStatusNotApplicable:
		if strings.TrimSpace(reason) == "" {
			return GateCheck{Name: name, Status: GateStatusFail, Path: relative, Message: "not_applicable token-economy evidence requires a reason", Hint: "Record a deterministic reason for every not_applicable section.", Field: name + ".reason", Expected: "non-empty reason", Actual: "missing"}
		}
		return GateCheck{Name: name, Status: GateStatusNotApplicable, Path: relative, Message: "token-economy evidence section is not applicable with a reason", Field: name + ".reason", Actual: strings.TrimSpace(reason)}
	default:
		actual := status
		if actual == "" {
			actual = "missing"
		}
		return GateCheck{Name: name, Status: GateStatusFail, Path: relative, Message: "token-economy evidence status vocabulary is unsupported", Hint: "Use pass or not_applicable inside token-economy-evidence.json; the gate itself emits fail for invalid evidence.", Field: name + ".status", Expected: "pass or not_applicable", Actual: actual}
	}
}

func validateTokenEvidenceRef(root Root, artifactRelative string, field string, ref tokenEvidenceRef) []GateCheck {
	checks := []GateCheck{}
	if strings.TrimSpace(ref.Path) == "" {
		return []GateCheck{{Name: "evidence_ref", Status: GateStatusFail, Path: artifactRelative, Message: "evidence ref path is missing", Hint: "Record repository-confined evidence ref paths.", Field: field + ".path", Expected: "non-empty path", Actual: "missing"}}
	}
	path, err := ResolveRelativePath(root, ref.Path)
	if err != nil {
		return []GateCheck{{Name: "evidence_ref", Status: GateStatusFail, Path: artifactRelative, Message: "evidence ref path is unsafe", Hint: "Use repository-relative paths without absolute paths or parent traversal.", Field: field + ".path", Expected: "repository-confined relative path", Actual: err.Error()}}
	}
	info, err := os.Lstat(path.Absolute)
	if err != nil {
		actual := err.Error()
		if os.IsNotExist(err) {
			actual = "missing"
		}
		checks = append(checks, GateCheck{Name: "evidence_ref", Status: GateStatusFail, Path: path.Relative, Message: "evidence ref path is missing or unreadable", Hint: "Preserve referenced evidence artifacts before checking the token-economy gate.", Field: field + ".path", Expected: "existing regular file", Actual: actual})
		return checks
	}
	if !info.Mode().IsRegular() {
		checks = append(checks, GateCheck{Name: "evidence_ref", Status: GateStatusFail, Path: path.Relative, Message: "evidence ref path must be a regular file", Hint: "Reference regular evidence files only.", Field: field + ".path", Expected: "regular file", Actual: fileKind(info)})
		return checks
	}
	content, err := os.ReadFile(path.Absolute)
	if err != nil {
		checks = append(checks, GateCheck{Name: "evidence_ref", Status: GateStatusFail, Path: path.Relative, Message: "evidence ref path cannot be read", Hint: "Check evidence file permissions before rerunning the gate.", Field: field + ".path", Expected: "readable file", Actual: err.Error()})
		return checks
	}
	if strings.TrimSpace(ref.Checksum) != "" {
		if !tokenEconomyChecksumPattern.MatchString(ref.Checksum) {
			checks = append(checks, GateCheck{Name: "evidence_checksum", Status: GateStatusFail, Path: path.Relative, Message: "evidence checksum has an unsupported format", Hint: "Use sha256:<64 hex characters>.", Field: field + ".checksum", Expected: "sha256:<64hex>", Actual: ref.Checksum})
		} else {
			sum := sha256.Sum256(content)
			actual := "sha256:" + hex.EncodeToString(sum[:])
			if !strings.EqualFold(actual, ref.Checksum) {
				checks = append(checks, GateCheck{Name: "evidence_checksum", Status: GateStatusFail, Path: path.Relative, Message: "evidence checksum does not match current file content", Hint: "Refresh the evidence ref checksum or restore the referenced artifact.", Field: field + ".checksum", Expected: ref.Checksum, Actual: actual})
			} else {
				checks = append(checks, GateCheck{Name: "evidence_checksum", Status: GateStatusPass, Path: path.Relative, Message: "evidence checksum matches", Field: field + ".checksum", Actual: actual})
			}
		}
	}
	for i, marker := range ref.Markers {
		if strings.TrimSpace(marker) == "" {
			checks = append(checks, GateCheck{Name: "evidence_marker", Status: GateStatusFail, Path: path.Relative, Message: "evidence marker must be non-empty", Hint: "Remove empty markers or record the exact marker text to check.", Field: fmt.Sprintf("%s.markers[%d]", field, i), Expected: "non-empty marker", Actual: "empty"})
			continue
		}
		if strings.Contains(string(content), marker) {
			checks = append(checks, GateCheck{Name: "evidence_marker", Status: GateStatusPass, Path: path.Relative, Message: "evidence marker is present", Field: fmt.Sprintf("%s.markers[%d]", field, i), Actual: marker})
		} else {
			checks = append(checks, GateCheck{Name: "evidence_marker", Status: GateStatusFail, Path: path.Relative, Message: "evidence marker is missing", Hint: "Record the required marker in the referenced evidence artifact or update the evidence ref.", Field: fmt.Sprintf("%s.markers[%d]", field, i), Expected: marker, Actual: "missing"})
		}
	}
	if len(checks) == 0 {
		checks = append(checks, GateCheck{Name: "evidence_ref", Status: GateStatusPass, Path: path.Relative, Message: "evidence ref path is safe and present", Field: field + ".path", Actual: path.Relative})
	}
	return checks
}

func tokenEconomyResultFromChecks(runID string, checks []GateCheck) GateCheckResult {
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
	if status != GateStatusFail {
		for _, check := range checks {
			if check.Name == "scope" && check.Status == GateStatusNotApplicable {
				status = GateStatusNotApplicable
				break
			}
		}
	}
	return GateCheckResult{RunID: runID, Gate: GateTokenEconomy, Status: status, Checks: checks, MissingEvidence: missing}
}

func metadataTaskID(metadata RunMetadata) string {
	if metadata.TaskID == nil || strings.TrimSpace(*metadata.TaskID) == "" {
		return "missing"
	}
	return strings.TrimSpace(*metadata.TaskID)
}

func validateTokenEconomyEvidenceSchema(relative string, content []byte) []SchemaCheck {
	var raw map[string]any
	if err := json.Unmarshal(content, &raw); err != nil || raw == nil {
		actual := "not an object"
		if err != nil {
			actual = err.Error()
		}
		return []SchemaCheck{schemaFail("json", relative, "file is not valid token-economy JSON", "Fix the file so it contains one JSON object before validating again.", "json", "JSON object", actual)}
	}
	schemaVersion, _ := raw["schema_version"].(string)
	switch schemaVersion {
	case tokenEconomySchemaVersion:
		return validateToken001EconomyEvidenceSchema(relative, raw)
	case tokenEconomyToken002SchemaVersion:
		return validateToken002EconomyEvidenceSchema(relative, raw)
	default:
		actual := schemaVersion
		if strings.TrimSpace(actual) == "" {
			actual = "missing"
		}
		return []SchemaCheck{schemaFail("schema_version", relative, "token-economy schema version is unsupported", "Use an explicitly supported token-economy schema version.", "schema_version", tokenEconomySupportedSchemaVersions, actual)}
	}
}

func validateToken001EconomyEvidenceSchema(relative string, raw map[string]any) []SchemaCheck {
	var evidence tokenEconomyEvidence
	_ = mapToStruct(raw, &evidence)
	checks := []SchemaCheck{}
	for _, field := range []string{"schema_version", "run_id", "task_id", "task_class", "scope", "compact_output_policy", "artifact_first_detail", "agent_instruction_evidence", "final_report_evidence", "kas_lifecycle_evidence", "mutation_approval_evidence"} {
		if _, ok := raw[field]; !ok {
			checks = append(checks, schemaFail(field, relative, "token-economy evidence required field is missing", "Use the token001.v1 evidence schema.", field, "present", "missing"))
		} else {
			checks = append(checks, schemaPass(field, relative, "token-economy evidence required field is present"))
		}
	}
	if evidence.SchemaVersion != tokenEconomySchemaVersion {
		checks = append(checks, schemaFail("schema_version", relative, "token-economy schema version is unsupported", "Use schema_version token001.v1.", "schema_version", tokenEconomySchemaVersion, evidence.SchemaVersion))
	}
	for _, item := range []struct{ name, status, reason string }{
		{"scope", sectionStatus(evidence.Scope), sectionReason(evidence.Scope)},
		{"compact_output_policy", sectionStatus(evidence.CompactOutputPolicy), sectionReason(evidence.CompactOutputPolicy)},
		{"artifact_first_detail", sectionStatus(evidence.ArtifactFirstDetail), sectionReason(evidence.ArtifactFirstDetail)},
		{"agent_instruction_evidence", sectionStatus(evidence.AgentInstructionEvidence), sectionReason(evidence.AgentInstructionEvidence)},
		{"final_report_evidence", sectionStatus(evidence.FinalReportEvidence), sectionReason(evidence.FinalReportEvidence)},
		{"kas_lifecycle_evidence", lifecycleStatus(evidence.KASLifecycleEvidence), lifecycleReason(evidence.KASLifecycleEvidence)},
		{"mutation_approval_evidence", mutationStatus(evidence.MutationApprovalEvidence), mutationReason(evidence.MutationApprovalEvidence)},
	} {
		checks = append(checks, schemaCheckTokenStatus(relative, item.name, item.status, item.reason))
	}
	return checks
}

func mapToStruct(raw map[string]any, target any) error {
	data, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

func schemaCheckTokenStatus(relative, name, status, reason string) SchemaCheck {
	switch status {
	case GateStatusPass:
		return schemaPass(name+".status", relative, "token-economy status is valid")
	case GateStatusNotApplicable:
		if strings.TrimSpace(reason) == "" {
			return schemaFail(name+".reason", relative, "not_applicable token-economy status requires a reason", "Record a deterministic not_applicable reason.", name+".reason", "non-empty reason", "missing")
		}
		return schemaPass(name+".status", relative, "token-economy not_applicable status has a reason")
	default:
		actual := status
		if actual == "" {
			actual = "missing"
		}
		return schemaFail(name+".status", relative, "token-economy status vocabulary is unsupported", "Use pass or not_applicable in token-economy-evidence.json.", name+".status", "pass or not_applicable", actual)
	}
}

func sectionStatus(section *tokenEvidenceSection) string {
	if section == nil {
		return ""
	}
	return section.Status
}

func sectionReason(section *tokenEvidenceSection) string {
	if section == nil {
		return ""
	}
	return section.Reason
}

func lifecycleStatus(section *tokenLifecycleEvidence) string {
	if section == nil {
		return ""
	}
	return section.Status
}

func lifecycleReason(section *tokenLifecycleEvidence) string {
	if section == nil {
		return ""
	}
	return section.Reason
}

func mutationStatus(section *tokenMutationEvidence) string {
	if section == nil {
		return ""
	}
	return section.Status
}

func mutationReason(section *tokenMutationEvidence) string {
	if section == nil {
		return ""
	}
	return section.Reason
}
