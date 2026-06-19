package project

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCheckGatePolicyPromotionPassesDeterministicEvidenceShape(t *testing.T) {
	repo, root, runID := policyPromotionRunWithArtifacts(t)
	writePolicyPromotionReferencedArtifacts(t, repo, runID)
	writePolicyPromotionEvidence(t, repo, runID, validPolicyPromotionEvidence(t, repo, runID))

	result, err := CheckGate(root, GateCheckOptions{RunID: runID, Gate: GatePolicyPromotion, Now: testRunNow(5)})
	if err != nil {
		t.Fatalf("CheckGate(policy-promotion) error = %v", err)
	}
	if result.Status != GateStatusPass || result.EventID != "evt-000004" || len(result.MissingEvidence) != 0 {
		t.Fatalf("result = %#v, want pass", result)
	}
	for _, name := range []string{"schema_version", "document_impact_map", "project_gray_coverage.resolved_role", "test_layer_evidence.required_label:unit", "failed_test_repair_ownership", "final_stale_status_check", "boundary_evidence.policy_owner", "mutation_approval_evidence"} {
		if !gateCheckStatus(result.Checks, name, GateStatusPass) {
			t.Fatalf("checks = %#v, want %s pass", result.Checks, name)
		}
	}
}

func TestInitArtifactsCreatesPolicyPromotionEvidenceArtifact(t *testing.T) {
	repo, root, runID := policyPromotionRunWithArtifacts(t)

	path := filepath.Join(repo, ".kkachi", "runs", runID, policyPromotionArtifact)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("policy promotion artifact stat error = %v", err)
	}
	if info.IsDir() || info.Size() == 0 {
		t.Fatalf("policy promotion artifact info = %#v, want non-empty regular baseline file", info)
	}

	listed, err := ListArtifacts(root, runID)
	if err != nil {
		t.Fatalf("ListArtifacts() error = %v", err)
	}
	found := false
	for _, artifact := range listed.Artifacts {
		if artifact.Path == policyPromotionArtifact {
			found = true
			if !artifact.Required || !artifact.Exists || artifact.Empty {
				t.Fatalf("policy promotion artifact status = %#v, want required existing non-empty baseline", artifact)
			}
		}
	}
	if !found {
		t.Fatalf("artifacts = %#v, want %s", listed.Artifacts, policyPromotionArtifact)
	}
}

func TestArtifactManifestRequiresPolicyPromotionEvidenceOnlyForPOLPR007(t *testing.T) {
	polprTaskID := policyPromotionTaskID
	if !stringSet(ArtifactManifest(RunMetadata{TaskID: &polprTaskID}))[policyPromotionArtifact] {
		t.Fatalf("ArtifactManifest(POLPR-007) missing %s", policyPromotionArtifact)
	}

	otherTaskID := "OTHER-001"
	if stringSet(ArtifactManifest(RunMetadata{TaskID: &otherTaskID}))[policyPromotionArtifact] {
		t.Fatalf("ArtifactManifest(non-POLPR-007) unexpectedly requires %s", policyPromotionArtifact)
	}
	if stringSet(ArtifactManifest(RunMetadata{}))[policyPromotionArtifact] {
		t.Fatalf("ArtifactManifest(nil task) unexpectedly requires %s", policyPromotionArtifact)
	}
}

func TestCheckGatePolicyPromotionFailsWhenRequiredArtifactMissing(t *testing.T) {
	repo, root, runID := policyPromotionRunWithArtifacts(t)
	mustRemove(t, filepath.Join(repo, ".kkachi", "runs", runID, policyPromotionArtifact))

	result, err := CheckGate(root, GateCheckOptions{RunID: runID, Gate: GatePolicyPromotion, Now: testRunNow(5)})
	if err != nil {
		t.Fatalf("CheckGate(policy-promotion missing artifact) error = %v", err)
	}
	if result.Status != GateStatusFail || !gateCheckStatus(result.Checks, "policy_promotion_artifact", GateStatusFail) {
		t.Fatalf("result = %#v, want missing artifact fail", result)
	}
}

func TestCheckGatePolicyPromotionFailsClosedMalformedJSON(t *testing.T) {
	repo, root, runID := policyPromotionRunWithArtifacts(t)
	mustWriteFile(t, filepath.Join(repo, ".kkachi", "runs", runID, policyPromotionArtifact), []byte(`{"schema_version":`))

	result, err := CheckGate(root, GateCheckOptions{RunID: runID, Gate: GatePolicyPromotion, Now: testRunNow(5)})
	if err != nil {
		t.Fatalf("CheckGate(policy-promotion malformed JSON) error = %v", err)
	}
	if result.Status != GateStatusFail || !gateCheckStatus(result.Checks, "policy_promotion_json", GateStatusFail) {
		t.Fatalf("result = %#v, want malformed JSON fail", result)
	}
}

func TestCheckGatePolicyPromotionFailsClosedEvidenceCases(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, repo string, runID string, payload map[string]any)
		want  string
	}{
		{
			name: "missing document impact map",
			setup: func(t *testing.T, repo string, runID string, payload map[string]any) {
				delete(payload, "document_impact_map")
			},
			want: "policy_promotion_required_field",
		},
		{
			name: "hard-coded individual boundary missing",
			setup: func(t *testing.T, repo string, runID string, payload map[string]any) {
				coverage := payload["project_gray_coverage"].(map[string]any)
				delete(coverage, "no_hard_coded_individual")
			},
			want: "project_gray_coverage.no_hard_coded_individual",
		},
		{
			name: "missing required test label",
			setup: func(t *testing.T, repo string, runID string, payload map[string]any) {
				testLayers := payload["test_layer_evidence"].(map[string]any)
				testLayers["labels"] = []map[string]any{}
			},
			want: "test_layer_evidence.required_label:unit",
		},
		{
			name: "unsafe stale-status surface path",
			setup: func(t *testing.T, repo string, runID string, payload map[string]any) {
				stale := payload["final_stale_status_check"].(map[string]any)
				surfaces := stale["surfaces_checked"].([]map[string]any)
				surfaces[0]["path"] = "../escape.md"
			},
			want: "final_stale_status_check.surfaces_checked[0].path",
		},
		{
			name: "unsafe document impact map evidence ref",
			setup: func(t *testing.T, repo string, runID string, payload map[string]any) {
				impact := payload["document_impact_map"].(map[string]any)
				impact["evidence_refs"] = []map[string]any{{"path": "/tmp/escape.md"}}
			},
			want: "evidence_ref",
		},
		{
			name: "not applicable without reason",
			setup: func(t *testing.T, repo string, runID string, payload map[string]any) {
				coverage := payload["project_gray_coverage"].(map[string]any)
				coverage["status"] = GateStatusNotApplicable
				delete(coverage, "reason")
			},
			want: "project_gray_coverage",
		},
		{
			name: "POLPR scope marked not applicable",
			setup: func(t *testing.T, repo string, runID string, payload map[string]any) {
				scope := payload["scope"].(map[string]any)
				scope["status"] = GateStatusNotApplicable
				scope["reason"] = "incorrectly self-scoped out of POLPR-007"
			},
			want: "scope.applicability",
		},
		{
			name: "KAH claims policy ownership",
			setup: func(t *testing.T, repo string, runID string, payload map[string]any) {
				boundary := payload["boundary_evidence"].(map[string]any)
				boundary["policy_owner"] = "KAH"
			},
			want: "boundary_evidence.policy_owner",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, root, runID := policyPromotionRunWithArtifacts(t)
			writePolicyPromotionReferencedArtifacts(t, repo, runID)
			payload := validPolicyPromotionEvidence(t, repo, runID)
			tt.setup(t, repo, runID, payload)
			writePolicyPromotionEvidence(t, repo, runID, payload)

			result, err := CheckGate(root, GateCheckOptions{RunID: runID, Gate: GatePolicyPromotion, Now: testRunNow(5)})
			if err != nil {
				t.Fatalf("CheckGate(policy-promotion) error = %v", err)
			}
			if result.Status != GateStatusFail || !gateCheckStatus(result.Checks, tt.want, GateStatusFail) || len(result.MissingEvidence) == 0 {
				t.Fatalf("result = %#v, want fail check %s", result, tt.want)
			}
		})
	}
}

func TestCheckGatePolicyPromotionNotApplicableForOtherTasksWithoutArtifact(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	created, err := CreateRun(root, deterministicCreateRunOptions())
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}

	result, err := CheckGate(root, GateCheckOptions{RunID: created.Metadata.RunID, Gate: GatePolicyPromotion, Now: testRunNow(4)})
	if err != nil {
		t.Fatalf("CheckGate(policy-promotion non POLPR-007 task) error = %v", err)
	}
	if result.Status != GateStatusNotApplicable || !gateCheckStatus(result.Checks, "policy_promotion_task_scope", GateStatusNotApplicable) {
		t.Fatalf("result = %#v, want not_applicable for non-POLPR-007", result)
	}
}

func TestSchemaValidatePolicyPromotionEvidence(t *testing.T) {
	repo, root, runID := policyPromotionRunWithArtifacts(t)
	writePolicyPromotionReferencedArtifacts(t, repo, runID)
	writePolicyPromotionEvidence(t, repo, runID, validPolicyPromotionEvidence(t, repo, runID))

	validated, err := ValidateSchemaFile(root, SchemaValidateOptions{File: tokenRunPath(runID, policyPromotionArtifact), Schema: SchemaPolicyPromotionEvidence})
	if err != nil {
		t.Fatalf("ValidateSchemaFile(policy-promotion) error = %v", err)
	}
	if validated.Status != "pass" || !schemaTestCheck(validated.Checks, "schema_version", "pass") || !schemaTestCheck(validated.Checks, "document_impact_map.status", "pass") || !schemaTestCheck(validated.Checks, "mutation_approval_evidence.status", "pass") {
		t.Fatalf("validated = %#v, want policy-promotion schema pass", validated)
	}

	payload := validPolicyPromotionEvidence(t, repo, runID)
	payload["scope"] = map[string]any{"status": GateStatusNotApplicable}
	writePolicyPromotionEvidence(t, repo, runID, payload)
	failed, err := ValidateSchemaFile(root, SchemaValidateOptions{File: tokenRunPath(runID, policyPromotionArtifact), Schema: ".kkachi/schemas/policy-promotion-evidence.schema.json"})
	if err != nil {
		t.Fatalf("ValidateSchemaFile(policy-promotion local schema) error = %v", err)
	}
	if failed.Status != "fail" || !schemaTestCheck(failed.Checks, "scope.reason", "fail") {
		t.Fatalf("failed = %#v, want missing not_applicable reason failure", failed)
	}
}

func policyPromotionRunWithArtifacts(t *testing.T) (string, Root, string) {
	t.Helper()
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	options := deterministicCreateRunOptions()
	options.TaskID = policyPromotionTaskID
	options.ExecutionMode = "adapter_qa"
	created, err := CreateRun(root, options)
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	if _, err := InitArtifacts(root, ArtifactInitOptions{RunID: created.Metadata.RunID, Now: testRunNow(4)}); err != nil {
		t.Fatalf("InitArtifacts() error = %v", err)
	}
	return repo, root, created.Metadata.RunID
}

func writePolicyPromotionReferencedArtifacts(t *testing.T, repo string, runID string) {
	t.Helper()
	writeMarkdownArtifact(t, repo, runID, "impact-map.md", "Status: complete\nDocument impact map.\n")
	writeMarkdownArtifact(t, repo, runID, "gray-coverage.md", "Status: complete\nproject-Gray role coverage.\n")
	writeMarkdownArtifact(t, repo, runID, "test-layer-labels.md", "Status: complete\nunit test layer label.\n")
	writeMarkdownArtifact(t, repo, runID, "repair-ownership.md", "Status: complete\nFailed-test repair ownership.\n")
	writeMarkdownArtifact(t, repo, runID, "stale-status-scan.md", "Status: complete\nPOLPR-007 Completed.\n")
	writeMarkdownArtifact(t, repo, runID, "boundary.md", "Status: complete\nKAS owns policy; KAH validates shape only.\n")
	if err := os.MkdirAll(filepath.Join(repo, "docs", "sot"), 0o755); err != nil {
		t.Fatalf("mkdir docs/sot: %v", err)
	}
	mustWriteFile(t, filepath.Join(repo, "docs", "roadmap.md"), []byte("| POLPR-007 | Completed |\n"))
	mustWriteFile(t, filepath.Join(repo, "docs", "sot", "policy-promotion-helper-evidence.md"), []byte("KAH validates deterministic evidence presence and shape only.\n"))
	mustWriteFile(t, filepath.Join(repo, "docs", "role-registry.md"), []byte("project-Gray reviewer role, no hard-coded individual.\n"))
}

func validPolicyPromotionEvidence(t *testing.T, repo string, runID string) map[string]any {
	t.Helper()
	impactRef := tokenRefMap(t, repo, tokenRunPath(runID, "impact-map.md"))
	grayRef := tokenRefMap(t, repo, tokenRunPath(runID, "gray-coverage.md"))
	testLayerRef := tokenRefMap(t, repo, tokenRunPath(runID, "test-layer-labels.md"))
	repairRef := tokenRefMap(t, repo, tokenRunPath(runID, "repair-ownership.md"))
	staleRef := tokenRefMap(t, repo, tokenRunPath(runID, "stale-status-scan.md"))
	boundaryRef := tokenRefMap(t, repo, tokenRunPath(runID, "boundary.md"))
	roadmapRef := tokenRefMap(t, repo, "docs/roadmap.md")
	sotRef := tokenRefMap(t, repo, "docs/sot/policy-promotion-helper-evidence.md")
	roleRef := tokenRefMap(t, repo, "docs/role-registry.md")
	return map[string]any{
		"schema_version": policyPromotionSchemaVersion,
		"run_id":         runID,
		"task_id":        policyPromotionTaskID,
		"task_class":     "policy-promotion-helper-evidence",
		"scope":          map[string]any{"status": GateStatusPass},
		"document_impact_map": map[string]any{
			"status": GateStatusPass, "evidence_refs": []map[string]any{impactRef, sotRef},
		},
		"project_gray_coverage": map[string]any{
			"status": GateStatusPass, "resolved_role": "project-Gray", "role_registry_ref": roleRef, "coverage_refs": []map[string]any{grayRef}, "no_hard_coded_individual": "Role is resolved by registry/project context; no individual is embedded as policy authority.",
		},
		"test_layer_evidence": map[string]any{
			"status": GateStatusPass, "required_labels": []string{"unit"}, "labels": []map[string]any{{"label": "unit", "resource_scope": "go test ./internal/project", "evidence_refs": []map[string]any{testLayerRef}}},
		},
		"failed_test_repair_ownership": map[string]any{
			"status": GateStatusPass, "blue_responsibility": "route compact failure evidence and synthesis", "implementer_lane": "backend implementer owns repository repair", "forbidden_blue_actions": []string{"semantic test-skip decisions", "direct implementation without recorded exception"}, "ownership_evidence_refs": []map[string]any{repairRef},
		},
		"final_stale_status_check": map[string]any{
			"status": GateStatusPass, "surfaces_checked": []map[string]any{{"path": "docs/roadmap.md", "expected_status": "Completed", "observed_status": "Completed", "result": GateStatusPass, "status_evidence_ref": roadmapRef}}, "evidence_refs": []map[string]any{staleRef, roadmapRef},
		},
		"boundary_evidence": map[string]any{
			"status": GateStatusPass, "policy_owner": "KAS", "kah_validation_role": "mechanical_recorded_evidence_only", "kah_forbidden_decisions": []string{"semantic policy acceptance", "operator-value judgment", "security-risk acceptance"}, "evidence_refs": []map[string]any{boundaryRef, sotRef},
		},
		"mutation_approval_evidence": map[string]any{"status": GateStatusPass, "mutation_scope": "none"},
	}
}

func writePolicyPromotionEvidence(t *testing.T, repo string, runID string, payload map[string]any) {
	t.Helper()
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatalf("marshal policy promotion evidence: %v", err)
	}
	mustWriteFile(t, filepath.Join(repo, ".kkachi", "runs", runID, policyPromotionArtifact), append(data, '\n'))
}
