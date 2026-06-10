package project

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckGateTokenEconomyPasses(t *testing.T) {
	repo, root, runID := tokenEconomyRunWithArtifacts(t)
	writeTokenEconomyReferencedArtifacts(t, repo, runID)
	writeTokenEconomyEvidence(t, repo, runID, validTokenEconomyEvidence(t, repo, runID))

	result, err := CheckGate(root, GateCheckOptions{RunID: runID, Gate: GateTokenEconomy, Now: testRunNow(5)})
	if err != nil {
		t.Fatalf("CheckGate(token-economy) error = %v", err)
	}
	if result.Status != GateStatusPass || result.EventID != "evt-000004" || len(result.MissingEvidence) != 0 {
		t.Fatalf("result = %#v, want pass", result)
	}
	for _, name := range []string{"schema_version", "run_id", "task_id", "compact_output_policy", "artifact_first_detail", "agent_instruction_evidence", "final_report_evidence", "mutation_approval_evidence"} {
		if !gateCheckStatus(result.Checks, name, GateStatusPass) {
			t.Fatalf("checks = %#v, want %s pass", result.Checks, name)
		}
	}
}

func TestCheckGateTokenEconomyToken002Passes(t *testing.T) {
	repo, root, runID := token002RunWithArtifacts(t)
	writeToken002ReferencedArtifacts(t, repo, runID)
	writeTokenEconomyEvidence(t, repo, runID, validToken002EconomyEvidence(t, repo, runID))

	result, err := CheckGate(root, GateCheckOptions{RunID: runID, Gate: GateTokenEconomy, Now: testRunNow(5)})
	if err != nil {
		t.Fatalf("CheckGate(token-economy token-002) error = %v", err)
	}
	if result.Status != GateStatusPass || result.EventID != "evt-000004" || len(result.MissingEvidence) != 0 {
		t.Fatalf("result = %#v, want pass", result)
	}
	for _, name := range []string{"schema_version", "run_id", "task_id", "verification_profile_evidence", "phase_packet_evidence", "review_bundle_evidence", "watcher_evidence", "change_verification_matrix_evidence", "mutation_approval_evidence"} {
		if !gateCheckStatus(result.Checks, name, GateStatusPass) {
			t.Fatalf("checks = %#v, want %s pass", result.Checks, name)
		}
	}
}

func TestCheckGateTokenEconomyToken002FailsClosedEvidenceCases(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, repo string, runID string, payload map[string]any)
		want  string
	}{
		{
			name: "missing section",
			setup: func(t *testing.T, repo string, runID string, payload map[string]any) {
				delete(payload, "review_bundle_evidence")
			},
			want: "token_required_field",
		},
		{
			name: "not applicable gate fakes runner fields",
			setup: func(t *testing.T, repo string, runID string, payload map[string]any) {
				gate := payload["verification_profile_evidence"].(map[string]any)["selected_gates"].([]map[string]any)[0]
				gate["applicability"] = "not_applicable"
				gate["status"] = GateStatusNotApplicable
				gate["not_applicable_reason"] = "KAS marked this gate out of scope."
			},
			want: "verification_profile_evidence.selected_gates[0]",
		},
		{
			name: "unsafe changed path",
			setup: func(t *testing.T, repo string, runID string, payload map[string]any) {
				matrix := payload["change_verification_matrix_evidence"].(map[string]any)
				matrix["changed_paths"] = []map[string]any{{"path": "../escape", "change_class": "source-code", "deterministic_evidence_refs": []map[string]any{tokenRefMap(t, repo, tokenRunPath(runID, "diff.patch"))}}}
			},
			want: "change_verification_matrix_evidence.changed_paths[0].path",
		},
		{
			name: "missing boundary note",
			setup: func(t *testing.T, repo string, runID string, payload map[string]any) {
				matrix := payload["change_verification_matrix_evidence"].(map[string]any)
				matrix["boundary_notes"] = []string{"KAS owns verification-selection policy."}
			},
			want: "change_verification_matrix_evidence.boundary_notes",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, root, runID := token002RunWithArtifacts(t)
			writeToken002ReferencedArtifacts(t, repo, runID)
			payload := validToken002EconomyEvidence(t, repo, runID)
			tt.setup(t, repo, runID, payload)
			writeTokenEconomyEvidence(t, repo, runID, payload)
			result, err := CheckGate(root, GateCheckOptions{RunID: runID, Gate: GateTokenEconomy, Now: testRunNow(5)})
			if err != nil {
				t.Fatalf("CheckGate(token-economy token-002) error = %v", err)
			}
			if result.Status != GateStatusFail || !gateCheckStatus(result.Checks, tt.want, GateStatusFail) || len(result.MissingEvidence) == 0 {
				t.Fatalf("result = %#v, want fail check %s", result, tt.want)
			}
		})
	}
}

func TestCheckGateTokenEconomyFailsClosedEvidenceCases(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, repo string, runID string)
		want  string
	}{
		{
			name: "missing artifact",
			setup: func(t *testing.T, repo string, runID string) {
				mustRemove(t, filepath.Join(repo, ".kkachi", "runs", runID, tokenEconomyArtifact))
			},
			want: "token_economy_artifact",
		},
		{
			name: "malformed json",
			setup: func(t *testing.T, repo string, runID string) {
				writeTokenEconomyRaw(t, repo, runID, []byte(`{"schema_version":`))
			},
			want: "token_economy_json",
		},
		{
			name: "unsafe path",
			setup: func(t *testing.T, repo string, runID string) {
				writeTokenEconomyReferencedArtifacts(t, repo, runID)
				payload := validTokenEconomyEvidence(t, repo, runID)
				payload["compact_output_policy"].(map[string]any)["evidence_refs"] = []map[string]any{{"path": "/tmp/escape"}}
				writeTokenEconomyEvidence(t, repo, runID, payload)
			},
			want: "evidence_ref",
		},
		{
			name: "checksum mismatch",
			setup: func(t *testing.T, repo string, runID string) {
				writeTokenEconomyReferencedArtifacts(t, repo, runID)
				payload := validTokenEconomyEvidence(t, repo, runID)
				payload["compact_output_policy"].(map[string]any)["evidence_refs"] = []map[string]any{{"path": tokenRunPath(runID, "cli-output.md"), "checksum": "sha256:" + strings.Repeat("0", 64)}}
				writeTokenEconomyEvidence(t, repo, runID, payload)
			},
			want: "evidence_checksum",
		},
		{
			name: "broad mutation without approval",
			setup: func(t *testing.T, repo string, runID string) {
				writeTokenEconomyReferencedArtifacts(t, repo, runID)
				payload := validTokenEconomyEvidence(t, repo, runID)
				payload["mutation_approval_evidence"] = map[string]any{"status": "pass", "mutation_scope": "broad", "claimed_broad_mutations": []string{"Hermes runtime profile mutation"}}
				writeTokenEconomyEvidence(t, repo, runID, payload)
			},
			want: "mutation_approval_refs",
		},
		{
			name: "token002 field",
			setup: func(t *testing.T, repo string, runID string) {
				writeTokenEconomyReferencedArtifacts(t, repo, runID)
				payload := validTokenEconomyEvidence(t, repo, runID)
				payload["verification_profile_evidence"] = map[string]any{"status": "pass"}
				writeTokenEconomyEvidence(t, repo, runID, payload)
			},
			want: "token002_scope",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, root, runID := tokenEconomyRunWithArtifacts(t)
			tt.setup(t, repo, runID)
			result, err := CheckGate(root, GateCheckOptions{RunID: runID, Gate: GateTokenEconomy, Now: testRunNow(5)})
			if err != nil {
				t.Fatalf("CheckGate(token-economy) error = %v", err)
			}
			if result.Status != GateStatusFail || !gateCheckStatus(result.Checks, tt.want, GateStatusFail) || len(result.MissingEvidence) == 0 {
				t.Fatalf("result = %#v, want fail check %s", result, tt.want)
			}
		})
	}
}

func TestCheckGateTokenEconomyNotApplicableRequiresReason(t *testing.T) {
	repo, root, runID := tokenEconomyRunWithArtifacts(t)
	writeTokenEconomyEvidence(t, repo, runID, notApplicableTokenEconomyEvidence(runID, true))

	result, err := CheckGate(root, GateCheckOptions{RunID: runID, Gate: GateTokenEconomy, Now: testRunNow(5)})
	if err != nil {
		t.Fatalf("CheckGate(token-economy n/a) error = %v", err)
	}
	if result.Status != GateStatusNotApplicable || result.EventID != "evt-000004" || len(result.MissingEvidence) != 0 {
		t.Fatalf("result = %#v, want not_applicable", result)
	}

	writeTokenEconomyEvidence(t, repo, runID, notApplicableTokenEconomyEvidence(runID, false))
	failed, err := CheckGate(root, GateCheckOptions{RunID: runID, Gate: GateTokenEconomy, Now: testRunNow(6)})
	if err != nil {
		t.Fatalf("CheckGate(token-economy missing n/a reason) error = %v", err)
	}
	if failed.Status != GateStatusFail || !gateCheckStatus(failed.Checks, "scope", GateStatusFail) {
		t.Fatalf("failed = %#v, want missing not_applicable reason failure", failed)
	}
}

func TestCheckGateTokenEconomyNotApplicableForOtherTasksWithoutArtifact(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	created, err := CreateRun(root, deterministicCreateRunOptions())
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}

	result, err := CheckGate(root, GateCheckOptions{RunID: created.Metadata.RunID, Gate: GateTokenEconomy, Now: testRunNow(4)})
	if err != nil {
		t.Fatalf("CheckGate(token-economy non token task) error = %v", err)
	}
	if result.Status != GateStatusNotApplicable || !gateCheckStatus(result.Checks, "token_task_scope", GateStatusNotApplicable) {
		t.Fatalf("result = %#v, want not_applicable for non-token-001", result)
	}
}

func tokenEconomyRunWithArtifacts(t *testing.T) (string, Root, string) {
	t.Helper()
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	options := deterministicCreateRunOptions()
	options.TaskID = tokenEconomyTaskID
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

func writeTokenEconomyReferencedArtifacts(t *testing.T, repo string, runID string) {
	t.Helper()
	writeMarkdownArtifact(t, repo, runID, "cli-output.md", "Status: complete\nDetailed artifact: .kkachi/runs/"+runID+"/impl-log.md\n")
	writeMarkdownArtifact(t, repo, runID, "impl-log.md", "Status: complete\nArtifact-first detail recorded.\n")
	writeMarkdownArtifact(t, repo, runID, "final-report.md", "Status: complete\nDetailed artifact: .kkachi/runs/"+runID+"/impl-log.md\n")
	mustWriteFile(t, filepath.Join(repo, "AGENTS.md"), []byte("<!-- KAS:MANAGED:BEGIN core-behavior -->\nEnglish compact output\n<!-- KAS:MANAGED:END core-behavior -->\n"))
}

func validTokenEconomyEvidence(t *testing.T, repo string, runID string) map[string]any {
	t.Helper()
	cliPath := tokenRunPath(runID, "cli-output.md")
	implPath := tokenRunPath(runID, "impl-log.md")
	finalPath := tokenRunPath(runID, "final-report.md")
	return map[string]any{
		"schema_version": tokenEconomySchemaVersion,
		"run_id":         runID,
		"task_id":        tokenEconomyTaskID,
		"task_class":     "development",
		"scope":          map[string]any{"status": "pass"},
		"compact_output_policy": map[string]any{"status": "pass", "evidence_refs": []map[string]any{{
			"path": cliPath, "checksum": tokenChecksum(t, repo, cliPath), "markers": []string{"Detailed artifact:"},
		}}},
		"artifact_first_detail": map[string]any{"status": "pass", "detail_ref": map[string]any{
			"path": implPath, "checksum": tokenChecksum(t, repo, implPath), "markers": []string{"Artifact-first detail recorded."},
		}},
		"agent_instruction_evidence": map[string]any{"status": "pass", "evidence_refs": []map[string]any{{
			"path": "AGENTS.md", "checksum": tokenChecksum(t, repo, "AGENTS.md"), "markers": []string{"KAS:MANAGED:BEGIN core-behavior", "English compact output"},
		}}},
		"final_report_evidence": map[string]any{"status": "pass", "evidence_refs": []map[string]any{{
			"path": finalPath, "checksum": tokenChecksum(t, repo, finalPath), "markers": []string{"Detailed artifact:"},
		}}},
		"kas_lifecycle_evidence":     map[string]any{"status": "not_applicable", "reason": "No project KAS lifecycle mutation is in scope for this run."},
		"mutation_approval_evidence": map[string]any{"status": "pass", "mutation_scope": "none"},
	}
}

func notApplicableTokenEconomyEvidence(runID string, withReason bool) map[string]any {
	reason := "Token-economy evidence is out of scope for this token-001 fixture."
	if !withReason {
		reason = ""
	}
	section := func() map[string]any { return map[string]any{"status": "not_applicable", "reason": reason} }
	return map[string]any{
		"schema_version":             tokenEconomySchemaVersion,
		"run_id":                     runID,
		"task_id":                    tokenEconomyTaskID,
		"task_class":                 "development",
		"scope":                      section(),
		"compact_output_policy":      section(),
		"artifact_first_detail":      section(),
		"agent_instruction_evidence": section(),
		"final_report_evidence":      section(),
		"kas_lifecycle_evidence":     section(),
		"mutation_approval_evidence": section(),
	}
}

func writeTokenEconomyEvidence(t *testing.T, repo string, runID string, payload map[string]any) {
	t.Helper()
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatalf("marshal token evidence: %v", err)
	}
	writeTokenEconomyRaw(t, repo, runID, append(data, '\n'))
}

func writeTokenEconomyRaw(t *testing.T, repo string, runID string, data []byte) {
	t.Helper()
	mustWriteFile(t, filepath.Join(repo, ".kkachi", "runs", runID, tokenEconomyArtifact), data)
}

func tokenRunPath(runID string, artifact string) string {
	return filepath.ToSlash(filepath.Join(RunRootPath, runID, artifact))
}

func tokenChecksum(t *testing.T, repo string, relative string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(relative)))
	if err != nil {
		t.Fatalf("read checksum source %s: %v", relative, err)
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func token002RunWithArtifacts(t *testing.T) (string, Root, string) {
	t.Helper()
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	options := deterministicCreateRunOptions()
	options.TaskID = tokenEconomyToken002TaskID
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

func writeToken002ReferencedArtifacts(t *testing.T, repo string, runID string) {
	t.Helper()
	writeMarkdownArtifact(t, repo, runID, "impl-log.md", "Status: complete\nImplementation log.\n")
	writeMarkdownArtifact(t, repo, runID, "test-log.md", "Status: pass\ngo test ./internal/project\n")
	writeMarkdownArtifact(t, repo, runID, "verification.md", "Status: pass\nVerification summary.\n")
	writeMarkdownArtifact(t, repo, runID, "acceptance-criteria.md", "Status: pass\nAcceptance criteria.\n")
	writeMarkdownArtifact(t, repo, runID, "redteam/plan-review.md", "Status: pass\nRed accepted.\n")
	writeMarkdownArtifact(t, repo, runID, "review.md", "Status: pass\nReview bundle.\n")
	writeMarkdownArtifact(t, repo, runID, "diff.patch", "diff --git a/internal/project/token_economy_gate.go b/internal/project/token_economy_gate.go\n")
}

func validToken002EconomyEvidence(t *testing.T, repo string, runID string) map[string]any {
	t.Helper()
	testRef := tokenRefMap(t, repo, tokenRunPath(runID, "test-log.md"))
	verifyRef := tokenRefMap(t, repo, tokenRunPath(runID, "verification.md"))
	acceptRef := tokenRefMap(t, repo, tokenRunPath(runID, "acceptance-criteria.md"))
	reviewRef := tokenRefMap(t, repo, tokenRunPath(runID, "review.md"))
	redRef := tokenRefMap(t, repo, tokenRunPath(runID, "redteam/plan-review.md"))
	diffRef := tokenRefMap(t, repo, tokenRunPath(runID, "diff.patch"))
	changedPath := "internal/project/token_economy_gate.go"
	return map[string]any{
		"schema_version": tokenEconomyToken002SchemaVersion,
		"run_id":         runID,
		"task_id":        tokenEconomyToken002TaskID,
		"task_class":     "development",
		"scope":          map[string]any{"status": GateStatusPass},
		"verification_profile_evidence": map[string]any{
			"status": GateStatusPass,
			"selected_gates": []map[string]any{{
				"selected_profile_id": "token010-default", "selected_gate_id": "go-test-project", "gate_kind": "test", "command": "go test ./internal/project", "timeout": "120s", "applicability": "required", "status": GateStatusPass,
				"runner_result": map[string]any{"command": "go test ./internal/project", "timeout": "120s", "applicability": "required", "status": GateStatusPass, "exit_code": 0, "duration_ms": 42, "log_ref": testRef},
			}},
		},
		"phase_packet_evidence": map[string]any{
			"status": GateStatusPass,
			"phase_packets": []map[string]any{{
				"packet_version": "token008.v1", "summary_id": "pkt-verify", "run_id": runID, "task_id": tokenEconomyToken002TaskID, "phase_id": "verification", "status": GateStatusPass, "packet_validity": "current", "source_artifact": verifyRef, "evidence_class": "verification_log", "compression_policy": "summary_with_pointer", "changed_paths": []string{changedPath}, "verification_summary": "Focused verification passed.", "critical_references": []map[string]any{{"class": "verification_log", "path": testRef["path"], "checksum": testRef["checksum"], "retrieval_instruction": "Read full log if summary is insufficient."}}, "retrieval_instructions": tokenRetrievalMap(), "invalidation_behavior": tokenInvalidationMap(), "next_action": "continue",
			}},
			"run_summary": map[string]any{"summary_version": "token008.v1", "summary_id": "sum-run", "run_id": runID, "task_id": tokenEconomyToken002TaskID, "status": GateStatusPass, "packet_validity": "current", "phase_packets": []map[string]any{{"phase_id": "verification", "status": GateStatusPass, "packet_path": verifyRef["path"], "packet_checksum": verifyRef["checksum"]}}, "changed_paths": []string{changedPath}, "verification_summary": "Run summary is current.", "critical_references": []map[string]any{{"class": "status_readback", "path": verifyRef["path"], "checksum": verifyRef["checksum"], "retrieval_instruction": "Read verification artifact."}}, "retrieval_instructions": tokenRetrievalMap(), "invalidation_behavior": tokenInvalidationMap(), "next_action": "review"},
		},
		"review_bundle_evidence": map[string]any{
			"status": GateStatusPass, "review_bundle_version": "token009.v1", "task_id": tokenEconomyToken002TaskID, "run_id": runID, "acceptance_reference": acceptRef, "diff_artifact": diffRef, "diff_checksum": diffRef["checksum"], "changed_paths": []string{changedPath},
			"verification_summaries": []map[string]any{{"phase": "verification", "status": GateStatusPass, "artifact_path": testRef["path"], "artifact_checksum": testRef["checksum"]}},
			"forbidden_scope":        []string{"auth mutation forbidden"}, "requested_verdict_vocabulary": []string{"accepted", "accepted_with_required_rework", "rejected", "blocked"}, "role_verdicts": []map[string]any{{"role": "red", "verdict": "accepted", "artifact_path": redRef["path"], "artifact_checksum": redRef["checksum"]}}, "finding_dispositions": []map[string]any{{"finding_id": "none", "disposition": "accepted", "artifact_path": reviewRef["path"], "artifact_checksum": reviewRef["checksum"]}}, "artifact_pointers": []map[string]any{reviewRef}, "blue_synthesis_inputs": map[string]any{"role_verdicts": "red accepted", "finding_dispositions": "none", "artifact_pointers": reviewRef["path"]}, "retrieval_instructions": tokenRetrievalMap(), "invalidation_behavior": tokenInvalidationMap(),
		},
		"watcher_evidence": map[string]any{"status": GateStatusPass, "watcher_version": "token009.v1", "terminal_only": true, "terminal_status": "complete", "mechanical_scope": []string{"artifact_existence"}, "artifact_pointers": []map[string]any{reviewRef}, "no_replacement_claim": "kanban, kah evidence, review cards, and blue synthesis remain authoritative."},
		"change_verification_matrix_evidence": map[string]any{
			"status": GateStatusPass, "matrix_version": "token010.v1", "task_id": tokenEconomyToken002TaskID, "run_id": runID, "policy_owner": "KAS", "verification_selection_policy_owner": "KAS", "kah_validation_role": "mechanical_recorded_evidence_only", "kah_forbidden_decisions": []string{"decide skip policy", "choose tests to skip", "infer skips from file extensions"}, "changed_path_source": map[string]any{"source_type": "git diff", "source_ref": diffRef["path"], "source_checksum": diffRef["checksum"]}, "changed_paths": []map[string]any{{"path": changedPath, "change_class": "source-code", "deterministic_evidence_refs": []map[string]any{diffRef}}},
			"rules":          []map[string]any{{"selected_rule_id": "source-code-default", "change_class": "source-code", "changed_path_set_classes": []string{"source-code"}, "selected_verification_commands": []map[string]any{{"selected_profile_id": "token010-default", "selected_gate_id": "go-test-project", "command": "go test ./internal/project", "timeout": "120s", "applicability": "required", "status": GateStatusPass, "evidence_ref": testRef}}, "scoped_verification": []map[string]any{{"command": "go test ./internal/project", "scope_reason": "source-code change", "status": GateStatusPass, "evidence_ref": testRef}}, "skipped_gates": []map[string]any{}, "no_skipped_gates_reason": "No gates were skipped.", "final_aggregate_preservation": map[string]any{"status": "preserved", "deterministic_evidence_refs": []map[string]any{verifyRef}}}},
			"boundary_notes": []string{"KAS owns verification-selection policy.", "KAH validates recorded deterministic evidence only.", "KAH must not decide skip policy or choose tests to skip."},
		},
		"mutation_approval_evidence": map[string]any{"status": GateStatusPass, "mutation_scope": "none"},
	}
}

func tokenRefMap(t *testing.T, repo string, relative string) map[string]any {
	t.Helper()
	return map[string]any{"path": relative, "checksum": tokenChecksum(t, repo, relative)}
}

func tokenRetrievalMap() map[string]any {
	return map[string]any{"default": "Read the referenced artifact before using this summary.", "required_when": []string{"status is fail"}}
}

func tokenInvalidationMap() map[string]any {
	return map[string]any{"invalid_if": []string{"referenced artifact checksum changes"}, "on_invalid": "Regenerate the evidence before gate use."}
}
