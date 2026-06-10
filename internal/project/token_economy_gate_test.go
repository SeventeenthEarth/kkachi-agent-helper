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
