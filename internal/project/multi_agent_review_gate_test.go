package project

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCheckGateMultiAgentReviewPassesRoleFirstEvidence(t *testing.T) {
	repo, root, runID := multiAgentReviewRun(t, "MAR-005")
	writeMultiAgentReviewReferencedArtifacts(t, repo, runID)
	writeMultiAgentReviewStatus(t, repo, runID, validMultiAgentReviewStatus(runID, "MAR-005"))

	result, err := CheckGate(root, GateCheckOptions{RunID: runID, Gate: GateMultiAgentReview, Now: testRunNow(5)})
	if err != nil {
		t.Fatalf("CheckGate(multi-agent-review) error = %v", err)
	}
	if result.Status != GateStatusPass || result.EventID != "evt-000004" || len(result.MissingEvidence) != 0 {
		t.Fatalf("result = %#v, want pass", result)
	}
	for _, name := range []string{"mar_status", "required_role:logic", "provider_attempt:logic-primary-001", "blue_disposition_ref"} {
		if !gateCheckStatus(result.Checks, name, GateStatusPass) {
			t.Fatalf("checks = %#v, want %s pass", result.Checks, name)
		}
	}
}

func TestCheckGateMultiAgentReviewFailsClosedRoleEvidenceCases(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, repo string, runID string, payload map[string]any)
		want  string
	}{
		{
			name: "provider attempt run id mismatch fails closed",
			setup: func(t *testing.T, repo string, runID string, payload map[string]any) {
				attempt := payload["provider_attempts"].([]map[string]any)[0]
				attempt["run_id"] = "run-19000101T000000Z-deadbeef0000"
			},
			want: "provider_attempt:logic-primary-001",
		},
		{
			name: "provider attempt task id mismatch fails closed",
			setup: func(t *testing.T, repo string, runID string, payload map[string]any) {
				attempt := payload["provider_attempts"].([]map[string]any)[0]
				attempt["task_id"] = "OTHER-001"
			},
			want: "provider_attempt:logic-primary-001",
		},
		{
			name: "waived role requires linked failed attempt",
			setup: func(t *testing.T, repo string, runID string, payload map[string]any) {
				writeMultiAgentReviewFile(t, repo, runID, "waiver-evidence.yaml", []byte("approval: 주군\nresidual_risk: accepted\n"))
				coverage := payload["coverage"].(map[string]any)
				coverage["covered_roles"] = []string{}
				coverage["by_role"] = map[string]any{"logic": map[string]any{"role_id": "logic", "state": "waived", "resolution": "waived", "reason": "주군 waiver", "attempt_id": "missing-attempt", "provider_failure_reasons": []string{"provider_unavailable"}}}
				payload["status"] = "PASS_WITH_FINDINGS"
				payload["waiver_ref"] = map[string]any{"path": multiAgentReviewRunPath(runID, "waiver-evidence.yaml")}
			},
			want: "required_role:logic",
		},
		{
			name: "non-pass MAR status cannot satisfy gate",
			setup: func(t *testing.T, repo string, runID string, payload map[string]any) {
				payload["status"] = "REQUEST_CHANGES"
				payload["blue_reason"] = "blocking finding remains unresolved"
			},
			want: "mar_status_passable",
		},
		{
			name: "covered alternate provider requires approval ref",
			setup: func(t *testing.T, repo string, runID string, payload map[string]any) {
				attempt := payload["provider_attempts"].([]map[string]any)[0]
				attempt["provider_candidate"] = "alternate"
				coverage := payload["coverage"].(map[string]any)
				role := coverage["by_role"].(map[string]any)["logic"].(map[string]any)
				role["provider_candidate"] = "alternate"
				role["resolution"] = "alternate_provider_success"
				delete(payload, "alternate_approval_ref")
			},
			want: "alternate_approval_ref",
		},
		{
			name: "premium attempt requires approval ref even without boolean",
			setup: func(t *testing.T, repo string, runID string, payload map[string]any) {
				attempt := payload["provider_attempts"].([]map[string]any)[0]
				attempt["provider_candidate"] = "premium"
				coverage := payload["coverage"].(map[string]any)
				role := coverage["by_role"].(map[string]any)["logic"].(map[string]any)
				role["provider_candidate"] = "premium"
				role["resolution"] = "premium_provider_success"
				payload["premium_review_used"] = false
				delete(payload, "premium_approval_ref")
			},
			want: "premium_approval_ref",
		},
		{
			name: "waived role requires waiver ref",
			setup: func(t *testing.T, repo string, runID string, payload map[string]any) {
				coverage := payload["coverage"].(map[string]any)
				coverage["covered_roles"] = []string{}
				coverage["by_role"] = map[string]any{"logic": map[string]any{"role_id": "logic", "state": "waived", "resolution": "waived", "reason": "주군 waiver", "provider_failure_reasons": []string{"provider_unavailable"}}}
				payload["status"] = "PASS_WITH_FINDINGS"
				delete(payload, "waiver_ref")
			},
			want: "waiver_ref",
		},
		{
			name: "unresolved required role cannot pass",
			setup: func(t *testing.T, repo string, runID string, payload map[string]any) {
				coverage := payload["coverage"].(map[string]any)
				coverage["minimum_met"] = false
				coverage["covered_roles"] = []string{}
				coverage["unresolved_required_roles"] = []string{"logic"}
				coverage["by_role"] = map[string]any{"logic": map[string]any{"role_id": "logic", "state": "unresolved", "reason": "unresolved_required_role_coverage", "provider_failure_reasons": []string{"nonzero_exit"}}}
				payload["status"] = "PASS"
			},
			want: "required_role:logic",
		},
		{
			name: "covered role requires matching successful attempt",
			setup: func(t *testing.T, repo string, runID string, payload map[string]any) {
				payload["provider_attempts"] = []map[string]any{}
			},
			want: "required_role:logic",
		},
		{
			name: "unsafe attempt path fails closed",
			setup: func(t *testing.T, repo string, runID string, payload map[string]any) {
				attempt := payload["provider_attempts"].([]map[string]any)[0]
				attempt["raw_output_path"] = "../escape.txt"
			},
			want: "provider_attempt:logic-primary-001.raw_output_path",
		},
		{
			name: "red trigger requires red adjudication ref",
			setup: func(t *testing.T, repo string, runID string, payload map[string]any) {
				coverage := payload["coverage"].(map[string]any)
				coverage["red_trigger_summary"] = map[string]any{"red_adjudication_required": true, "triggers": []string{"high_risk"}}
				delete(payload, "red_adjudication_ref")
			},
			want: "red_adjudication_ref",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, root, runID := multiAgentReviewRun(t, "MAR-005")
			writeMultiAgentReviewReferencedArtifacts(t, repo, runID)
			payload := validMultiAgentReviewStatus(runID, "MAR-005")
			tt.setup(t, repo, runID, payload)
			writeMultiAgentReviewStatus(t, repo, runID, payload)

			result, err := CheckGate(root, GateCheckOptions{RunID: runID, Gate: GateMultiAgentReview, Now: testRunNow(5)})
			if err != nil {
				t.Fatalf("CheckGate(multi-agent-review) error = %v", err)
			}
			if result.Status != GateStatusFail || !gateCheckStatus(result.Checks, tt.want, GateStatusFail) || len(result.MissingEvidence) == 0 {
				t.Fatalf("result = %#v, want fail check %s", result, tt.want)
			}
		})
	}
}

func TestCheckGateMultiAgentReviewNotApplicableForOtherTasksWithoutArtifact(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	created, err := CreateRun(root, deterministicCreateRunOptions())
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}

	result, err := CheckGate(root, GateCheckOptions{RunID: created.Metadata.RunID, Gate: GateMultiAgentReview, Now: testRunNow(4)})
	if err != nil {
		t.Fatalf("CheckGate(multi-agent-review non-MAR task) error = %v", err)
	}
	if result.Status != GateStatusNotApplicable || !gateCheckStatus(result.Checks, "mar_task_scope", GateStatusNotApplicable) {
		t.Fatalf("result = %#v, want not_applicable for non-MAR task without MAR artifact", result)
	}
}

func TestCheckGateMultiAgentReviewNotApplicableForOtherTasksWithBaselineArtifact(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	created, err := CreateRun(root, deterministicCreateRunOptions())
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	if _, err := InitArtifacts(root, ArtifactInitOptions{RunID: created.Metadata.RunID, Now: testRunNow(4)}); err != nil {
		t.Fatalf("InitArtifacts() error = %v", err)
	}

	result, err := CheckGate(root, GateCheckOptions{RunID: created.Metadata.RunID, Gate: GateMultiAgentReview, Now: testRunNow(5)})
	if err != nil {
		t.Fatalf("CheckGate(multi-agent-review non-MAR baseline) error = %v", err)
	}
	if result.Status != GateStatusNotApplicable || !gateCheckStatus(result.Checks, "mar_task_scope", GateStatusNotApplicable) {
		t.Fatalf("result = %#v, want not_applicable for non-MAR task with artifact-init baseline", result)
	}
}

func TestCheckGateMultiAgentReviewFailsWhenManifestRequiresMissingArtifact(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	options := deterministicCreateRunOptions()
	options.TaskID = "support-001"
	created, err := CreateRun(root, options)
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	if _, err := InitArtifacts(root, ArtifactInitOptions{RunID: created.Metadata.RunID, Now: testRunNow(4)}); err != nil {
		t.Fatalf("InitArtifacts() error = %v", err)
	}
	metadata := readRunMetadata(t, repo, created.Metadata.RunID)
	metadata.RequiredArtifacts = append(metadata.RequiredArtifacts, multiAgentReviewArtifact)
	writeRunMetadataForTest(t, repo, metadata)
	mustRemoveMultiAgentReviewStatus(t, repo, created.Metadata.RunID)

	result, err := CheckGate(root, GateCheckOptions{RunID: created.Metadata.RunID, Gate: GateMultiAgentReview, Now: testRunNow(5)})
	if err != nil {
		t.Fatalf("CheckGate(multi-agent-review manifest-required missing artifact) error = %v", err)
	}
	if result.Status != GateStatusFail || !gateCheckStatus(result.Checks, "mar_artifact", GateStatusFail) {
		t.Fatalf("result = %#v, want fail for manifest-required missing MAR artifact", result)
	}
}

func TestValidateSchemaMultiAgentReviewEvidence(t *testing.T) {
	repo, root, runID := multiAgentReviewRun(t, "MAR-005")
	writeMultiAgentReviewReferencedArtifacts(t, repo, runID)
	writeMultiAgentReviewStatus(t, repo, runID, validMultiAgentReviewStatus(runID, "MAR-005"))

	result, err := ValidateSchemaFile(root, SchemaValidateOptions{Schema: SchemaMultiAgentReviewEvidence, File: multiAgentReviewStatusPath(runID)})
	if err != nil {
		t.Fatalf("ValidateSchemaFile(multi-agent-review) error = %v", err)
	}
	if result.Status != GateStatusPass {
		t.Fatalf("schema result = %#v, want pass", result)
	}
}

func multiAgentReviewRun(t *testing.T, taskID string) (string, Root, string) {
	t.Helper()
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	options := deterministicCreateRunOptions()
	options.TaskID = taskID
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

func writeMultiAgentReviewReferencedArtifacts(t *testing.T, repo string, runID string) {
	t.Helper()
	writeMultiAgentReviewFile(t, repo, runID, "raw/logic-primary-001.txt", []byte("raw review output\n"))
	writeMultiAgentReviewFile(t, repo, runID, "parsed/logic-primary-001.json", []byte(`{"status":"PASS","summary":"logic covered"}`+"\n"))
	writeMultiAgentReviewFile(t, repo, runID, "blue-disposition.md", []byte("Status: complete\nBlue disposition recorded.\n"))
	writeMultiAgentReviewFile(t, repo, runID, "red-adjudication.md", []byte("Status: complete\nRed adjudication recorded.\n"))
}

func validMultiAgentReviewStatus(runID string, taskID string) map[string]any {
	attemptID := "logic-primary-001"
	return map[string]any{
		"schema_version": "mar-evidence.v1",
		"run_id":         runID,
		"task_id":        taskID,
		"status":         "PASS",
		"reason":         "all_required_role_coverage_resolved",
		"coverage": map[string]any{
			"required_roles":            []string{"logic"},
			"observed_roles":            []string{"logic"},
			"covered_roles":             []string{"logic"},
			"minimum_met":               true,
			"unresolved_required_roles": []string{},
			"operator_report_text":      "",
			"red_trigger_summary":       map[string]any{"red_adjudication_required": false, "triggers": []string{}},
			"blue_matrix_inputs":        map[string]any{"required_roles": []string{"logic"}, "covered_roles": []string{"logic"}, "acceptance_criteria_matrix": map[string]any{"logic": []string{}}},
			"by_role":                   map[string]any{"logic": map[string]any{"role_id": "logic", "state": "covered", "resolution": "primary_provider_success", "attempt_id": attemptID, "provider_id": "zcode_glm_5_2", "provider_candidate": "primary"}},
		},
		"provider_attempts": []map[string]any{{
			"schema_version":          "mar.provider_attempt.v1",
			"run_id":                  runID,
			"task_id":                 taskID,
			"attempt_id":              attemptID,
			"role_id":                 "logic",
			"provider_id":             "zcode_glm_5_2",
			"provider_candidate":      "primary",
			"command_lane":            "zcode",
			"selected_model":          "glm-5.2",
			"terminal_status":         "PASS",
			"parser_status":           "parsed",
			"raw_output_path":         multiAgentReviewRunPath(runID, "raw/logic-primary-001.txt"),
			"parsed_finding_path":     multiAgentReviewRunPath(runID, "parsed/logic-primary-001.json"),
			"mutation_check":          map[string]any{"checked": true, "detected": false},
			"provider_failure_reason": nil,
		}},
		"blue_disposition_ref": map[string]any{"path": multiAgentReviewRunPath(runID, "blue-disposition.md")},
		"red_adjudication_ref": map[string]any{"path": multiAgentReviewRunPath(runID, "red-adjudication.md")},
	}
}

func writeMultiAgentReviewStatus(t *testing.T, repo string, runID string, payload map[string]any) {
	t.Helper()
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatalf("marshal MAR evidence: %v", err)
	}
	writeMultiAgentReviewFile(t, repo, runID, "status.json", append(data, '\n'))
}

func writeMultiAgentReviewFile(t *testing.T, repo string, runID string, artifact string, data []byte) {
	t.Helper()
	path := filepath.Join(repo, filepath.FromSlash(multiAgentReviewRunPath(runID, artifact)))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir MAR artifact dir: %v", err)
	}
	mustWriteFile(t, path, data)
}

func multiAgentReviewStatusPath(runID string) string {
	return multiAgentReviewRunPath(runID, "status.json")
}

func multiAgentReviewRunPath(runID string, path string) string {
	return filepath.ToSlash(filepath.Join(RunRootPath, runID, "multi-agent-review", path))
}

func mustRemoveMultiAgentReviewStatus(t *testing.T, repo string, runID string) {
	t.Helper()
	if err := os.Remove(filepath.Join(repo, filepath.FromSlash(multiAgentReviewStatusPath(runID)))); err != nil && !os.IsNotExist(err) {
		t.Fatalf("remove MAR status: %v", err)
	}
}
