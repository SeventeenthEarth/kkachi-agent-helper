//go:build integration

package integration

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBinaryEntrypointSmoke(t *testing.T) {
	cmd := exec.Command("go", "run", "../..", "--version")

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go run failed: %v\n%s", err, string(output))
	}
	if !strings.HasPrefix(string(output), "kkachi-agent-helper 0.0.0-dev") {
		t.Fatalf("output = %q, want version prefix", string(output))
	}
}

func TestCapabilitiesJSONAtBinaryBoundary(t *testing.T) {
	binary := buildHelperBinary(t)
	output := runHelper(t, binary, t.TempDir(), "capabilities", "--json")
	var payload struct {
		Helper struct {
			Version string `json:"version"`
		} `json:"helper"`
		ProjectSchemaVersion string `json:"project_schema_version"`
		CompatibilityFlags   struct {
			BackendEvidenceRequirements bool `json:"backend_evidence_requirements"`
			PhasePlan                   bool `json:"phase_plan"`
			ArtifactMutation            bool `json:"artifact_mutation"`
			ApprovalRecords             bool `json:"approval_records"`
			WorkflowGraphReadonly       bool `json:"workflow_graph_readonly"`
			WorkflowGraphInit           bool `json:"workflow_graph_init"`
			WorkflowGraphApply          bool `json:"workflow_graph_apply"`
			InstallCommand              bool `json:"install_command"`
		} `json:"compatibility_flags"`
		OmittedSurfaces []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"omitted_surfaces"`
	}
	if err := json.Unmarshal(output, &payload); err != nil {
		t.Fatalf("capabilities output is not JSON: %v\n%s", err, string(output))
	}
	if payload.Helper.Version != "0.1.0" || payload.ProjectSchemaVersion != "0.1" {
		t.Fatalf("payload versions = %#v, want helper 0.1.0 and schema 0.1", payload)
	}
	if !payload.CompatibilityFlags.BackendEvidenceRequirements || !payload.CompatibilityFlags.PhasePlan || !payload.CompatibilityFlags.ArtifactMutation || !payload.CompatibilityFlags.ApprovalRecords || !payload.CompatibilityFlags.WorkflowGraphReadonly || !payload.CompatibilityFlags.WorkflowGraphInit || !payload.CompatibilityFlags.WorkflowGraphApply || payload.CompatibilityFlags.InstallCommand {
		t.Fatalf("compatibility flags = %#v, want current align support matrix", payload.CompatibilityFlags)
	}
	foundInstall := false
	for _, surface := range payload.OmittedSurfaces {
		if surface.Name == "install" && surface.Status == "omitted" {
			foundInstall = true
		}
	}
	if !foundInstall {
		t.Fatalf("omitted surfaces = %#v, want install omitted", payload.OmittedSurfaces)
	}
}

func TestHelpAtBinaryBoundaryDoesNotRequireProjectState(t *testing.T) {
	binary := buildHelperBinary(t)

	output := runHelper(t, binary, t.TempDir(), "--help")
	assertOutputContains(t, output, "kkachi-agent-helper", "top-level help")
	assertOutputContains(t, output, "Usage:", "top-level help")
	assertOutputContains(t, output, "JSON behavior:", "top-level help")

	output = runHelper(t, binary, t.TempDir(), "run", "create", "--help")
	assertOutputContains(t, output, "kkachi-agent-helper run create", "run create help")
	assertOutputContains(t, output, "--title <title> (required)", "run create help")
	assertOutputContains(t, output, "--backend-evidence <auto|required|not_applicable>", "run create help")

	output = runHelper(t, binary, t.TempDir(), "graph", "--help")
	assertOutputContains(t, output, "kkachi-agent-helper graph", "graph help")
	assertOutputContains(t, output, "graph init --from-template <template-id-or-path>", "graph help")
	assertOutputContains(t, output, "graph validate [--file .kkachi-workflow.yaml]", "graph help")
	assertOutputContains(t, output, "graph diff --from <repo-relative-graph>", "graph help")
	assertOutputContains(t, output, "graph propose --patch <repo-relative-candidate-graph>", "graph help")
	assertOutputContains(t, output, "graph apply --proposal <proposal-id>", "graph help")
	assertOutputContains(t, output, "--file <repo-relative-path>", "graph help")

	output = runHelper(t, binary, t.TempDir(), "--json", "phase-plan", "--help")
	var payload struct {
		Command      string `json:"command"`
		Status       string `json:"status"`
		JSONBehavior string `json:"json_behavior"`
	}
	if err := json.Unmarshal(output, &payload); err != nil {
		t.Fatalf("phase-plan help output is not JSON: %v\n%s", err, string(output))
	}
	if payload.Command != "kkachi-agent-helper phase-plan" || payload.Status != "supported" || !strings.Contains(payload.JSONBehavior, "Failing validation exits 3") {
		t.Fatalf("payload = %#v, want supported phase-plan help", payload)
	}
}

func TestGraphReadonlyBinaryFlow(t *testing.T) {
	binary := buildHelperBinary(t)
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	runHelper(t, binary, repo, "project", "init", "--json")

	initOutput := runHelper(t, binary, repo, "graph", "init", "--from-template", "khs-default", "--json")
	assertOutputContains(t, initOutput, `"template_id":"khs-default"`, "graph init")
	assertOutputContains(t, initOutput, `"event_id":"evt-000002"`, "graph init")

	validateOutput := runHelper(t, binary, repo, "graph", "validate", "--json")
	var validation struct {
		Status          string `json:"status"`
		File            string `json:"file"`
		Checksum        string `json:"checksum"`
		EffectiveSource string `json:"effective_source"`
	}
	if err := json.Unmarshal(validateOutput, &validation); err != nil {
		t.Fatalf("graph validate output is not JSON: %v\n%s", err, string(validateOutput))
	}
	if validation.Status != "pass" || validation.File != ".kkachi-workflow.yaml" || validation.Checksum == "" || validation.EffectiveSource != "project_file" {
		t.Fatalf("validation = %#v, want passing graph validation", validation)
	}

	explainOutput := runHelper(t, binary, repo, "graph", "explain", "--json")
	var explained struct {
		Status string `json:"status"`
		Phases []struct {
			ID string `json:"id"`
		} `json:"phases"`
		Edges []struct {
			From string `json:"from"`
			To   string `json:"to"`
		} `json:"edges"`
	}
	if err := json.Unmarshal(explainOutput, &explained); err != nil {
		t.Fatalf("graph explain output is not JSON: %v\n%s", err, string(explainOutput))
	}
	if explained.Status != "pass" || len(explained.Phases) != 13 || explained.Phases[0].ID != "intake" || len(explained.Edges) != 12 || explained.Edges[0].To != "sot" {
		t.Fatalf("explained = %#v, want graph projection", explained)
	}

	templateRepo := t.TempDir()
	if err := os.Mkdir(filepath.Join(templateRepo, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir template repo .git: %v", err)
	}
	runHelper(t, binary, templateRepo, "project", "init", "--json")
	template := "docs/graphs/template-workflow.yaml"
	writeIntegrationTarget(t, templateRepo, template, integrationValidWorkflowGraph())
	templateInitOutput := runHelper(t, binary, templateRepo, "graph", "init", "--from-template", template, "--json")
	assertOutputContains(t, templateInitOutput, `"template_id":"`+template+`"`, "template graph init")
	assertOutputContains(t, templateInitOutput, `"template_source":"path"`, "template graph init")
	assertOutputContains(t, templateInitOutput, `"event_id":"evt-000002"`, "template graph init")
	assertFileContains(t, filepath.Join(templateRepo, ".kkachi-workflow.yaml"), `graph_id: "graph-kkachi-project-kkachi-test-`, "template initialized graph")
	assertFileContains(t, filepath.Join(templateRepo, ".kkachi-workflow.yaml"), `project: "kkachi-test"`, "template initialized graph")
	assertFileContains(t, filepath.Join(templateRepo, ".kkachi-workflow.yaml"), `source_template: "docs/graphs/template-workflow.yaml"`, "template initialized graph")
	templateValidation := runHelper(t, binary, templateRepo, "graph", "validate", "--json")
	assertOutputContains(t, templateValidation, `"status":"pass"`, "template graph validation")

	writeIntegrationTarget(t, repo, ".kkachi-workflow.yaml", integrationValidWorkflowGraph())
	alternate := "docs/graphs/candidate-workflow.yaml"
	writeIntegrationTarget(t, repo, alternate, integrationValidWorkflowGraph())
	alternateOutput := runHelper(t, binary, repo, "graph", "validate", "--file", alternate, "--json")
	var alternateValidation struct {
		Status string `json:"status"`
		File   string `json:"file"`
	}
	if err := json.Unmarshal(alternateOutput, &alternateValidation); err != nil {
		t.Fatalf("alternate graph validate output is not JSON: %v\n%s", err, string(alternateOutput))
	}
	if alternateValidation.Status != "pass" || alternateValidation.File != alternate {
		t.Fatalf("alternate validation = %#v, want explicit graph candidate", alternateValidation)
	}

	candidate := "docs/graphs/proposed-workflow.yaml"
	writeIntegrationTarget(t, repo, candidate, integrationCandidateWorkflowGraph())
	diffOutput := runHelper(t, binary, repo, "graph", "diff", "--from", ".kkachi-workflow.yaml", "--to", candidate, "--semantic", "--json")
	assertOutputContains(t, diffOutput, `"status":"pass"`, "graph diff")
	assertOutputContains(t, diffOutput, `"changed_phases":{"added":[{"id":"ask"`, "graph diff")
	assertOutputContains(t, diffOutput, `"risk_flags":["approvals_changed","dependencies_changed","gates_changed"]`, "graph diff")
	assertOutputContains(t, diffOutput, `"requires_approval":true`, "graph diff")

	proposeOutput := runHelper(t, binary, repo, "graph", "propose", "--patch", candidate, "--reason", "add ask phase", "--json")
	assertOutputContains(t, proposeOutput, `"proposal_id":"gprop-000001"`, "graph propose")
	assertOutputContains(t, proposeOutput, `"semantic_diff_ref":".kkachi/graph/proposals/gprop-000001.json#semantic_diff"`, "graph propose")
	assertFileContains(t, filepath.Join(repo, ".kkachi", "graph", "proposals", "gprop-000001.json"), `"semantic_diff": {`, "graph proposal record")
	assertFileContains(t, filepath.Join(repo, ".kkachi", "events.jsonl"), `"type":"graph.proposal_recorded"`, "graph proposal event")
	applyOutput := runHelper(t, binary, repo, "graph", "apply", "--proposal", "gprop-000001", "--approval", "approval:integration", "--json")
	assertOutputContains(t, applyOutput, `"proposal_id":"gprop-000001"`, "graph apply")
	assertOutputContains(t, applyOutput, `"approval_ref":"approval:integration"`, "graph apply")
	assertOutputContains(t, applyOutput, `"event_ids":["evt-000004"]`, "graph apply")
	assertFileContains(t, filepath.Join(repo, ".kkachi-workflow.yaml"), `last_applied_event_id: "evt-000004"`, "applied graph")
	assertFileContains(t, filepath.Join(repo, ".kkachi-workflow.yaml"), `id: "ask"`, "applied graph")
	assertFileContains(t, filepath.Join(repo, ".kkachi", "events.jsonl"), `"type":"graph.applied"`, "graph apply event")
	assertOutputContains(t, runHelper(t, binary, repo, "graph", "validate", "--json"), `"status":"pass"`, "applied graph validation")

	invalidGraph := "docs/graphs/invalid-workflow.yaml"
	writeIntegrationTarget(t, repo, invalidGraph, strings.Replace(integrationValidWorkflowGraph(), `to: "implement"`, `to: "missing"`, 1))
	invalidDiffOutput, err := runHelperAllowError(binary, repo, "graph", "diff", "--from", invalidGraph, "--to", candidate, "--json")
	if err == nil {
		t.Fatalf("graph diff with invalid --from unexpectedly passed: %s", string(invalidDiffOutput))
	}
	assertOutputContains(t, invalidDiffOutput, `"status":"fail"`, "invalid graph diff")
	assertOutputContains(t, invalidDiffOutput, `"name":"edge_to"`, "invalid graph diff")
	invalidProposeOutput, err := runHelperAllowError(binary, repo, "graph", "propose", "--patch", invalidGraph, "--reason", "invalid candidate", "--json")
	if err == nil {
		t.Fatalf("graph propose with invalid patch unexpectedly passed: %s", string(invalidProposeOutput))
	}
	assertOutputContains(t, invalidProposeOutput, `"code":"graph_proposal_invalid"`, "invalid graph propose")

	writeIntegrationTarget(t, repo, ".kkachi-workflow.yaml", strings.Replace(integrationValidWorkflowGraph(), `to: "implement"`, `to: "missing"`, 1))
	failedOutput, err := runHelperAllowError(binary, repo, "graph", "validate", "--json")
	if err == nil {
		t.Fatalf("graph validate unexpectedly passed: %s", string(failedOutput))
	}
	assertOutputContains(t, failedOutput, `"status":"fail"`, "graph validation failure")
	assertOutputContains(t, failedOutput, `"name":"edge_to"`, "graph validation failure")
}

func TestArtifactMutationBinaryFlow(t *testing.T) {
	binary := buildHelperBinary(t)
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	runHelper(t, binary, repo, "project", "init", "--json")
	createdOutput := runHelper(t, binary, repo,
		"run", "create",
		"--title", "Artifact mutation integration",
		"--work-path", "A_development_execution",
		"--work-mode", "standard",
		"--urgency", "normal",
		"--sot-policy", "existing_sot_basis",
		"--execution-mode", "production_write",
		"--commander", "Gongmyeong",
		"--task-id", "align-006",
		"--json",
	)
	var created struct {
		RunID string `json:"run_id"`
	}
	if err := json.Unmarshal(createdOutput, &created); err != nil {
		t.Fatalf("run create output is not JSON: %v\n%s", err, string(createdOutput))
	}
	runHelper(t, binary, repo, "artifact", "init", created.RunID, "--json")
	if err := os.WriteFile(filepath.Join(repo, "plan-source.md"), []byte("Status: pending\nPlan: preserve pre-start plan\n"), 0o600); err != nil {
		t.Fatalf("write plan source: %v", err)
	}
	writeOutput := runHelper(t, binary, repo, "artifact", "write", created.RunID[:24], "plan.md", "--from", "plan-source.md", "--json")
	var written struct {
		RunID     string `json:"run_id"`
		Path      string `json:"path"`
		Operation string `json:"operation"`
		EventID   string `json:"event_id"`
	}
	if err := json.Unmarshal(writeOutput, &written); err != nil {
		t.Fatalf("artifact write output is not JSON: %v\n%s", err, string(writeOutput))
	}
	if written.RunID != created.RunID || written.Path != "plan.md" || written.Operation != "write" || written.EventID != "evt-000004" {
		t.Fatalf("written = %#v, want canonical write", written)
	}
	assertFileContains(t, filepath.Join(repo, ".kkachi", "runs", created.RunID, "plan.md"), "preserve pre-start plan", "plan artifact")

	if err := os.WriteFile(filepath.Join(repo, "checklist-append.md"), []byte("- [x] implementation ready\n"), 0o600); err != nil {
		t.Fatalf("write checklist append source: %v", err)
	}
	runHelper(t, binary, repo, "artifact", "append", created.RunID, "checklist.md", "--from", "checklist-append.md", "--json")
	setOutput := runHelper(t, binary, repo, "artifact", "set-status", created.RunID, "checklist.md", "--status", "complete", "--json")
	var updated struct {
		Operation string `json:"operation"`
		Status    string `json:"status"`
		EventID   string `json:"event_id"`
	}
	if err := json.Unmarshal(setOutput, &updated); err != nil {
		t.Fatalf("artifact set-status output is not JSON: %v\n%s", err, string(setOutput))
	}
	if updated.Operation != "set-status" || updated.Status != "complete" || updated.EventID != "evt-000006" {
		t.Fatalf("updated = %#v, want set-status complete", updated)
	}
	assertFileContains(t, filepath.Join(repo, ".kkachi", "runs", created.RunID, "checklist.md"), "Status: complete", "checklist artifact")
	assertFileContains(t, filepath.Join(repo, ".kkachi", "events.jsonl"), `"operation":"write"`, "artifact write event")
	assertFileContains(t, filepath.Join(repo, ".kkachi", "events.jsonl"), `"operation":"append"`, "artifact append event")
	assertFileContains(t, filepath.Join(repo, ".kkachi", "events.jsonl"), `"operation":"set-status"`, "artifact set-status event")
}

func TestPhasePlanBinaryFlowAndDiagnostics(t *testing.T) {
	binary := buildHelperBinary(t)
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}

	runHelper(t, binary, repo, "project", "init", "--json")
	createdOutput := runHelper(t, binary, repo,
		"run", "create",
		"--title", "Phase plan integration",
		"--work-path", "A_development_execution",
		"--work-mode", "standard",
		"--urgency", "normal",
		"--sot-policy", "existing_sot_basis",
		"--execution-mode", "production_write",
		"--commander", "Gongmyeong",
		"--task-id", "align-005",
		"--json",
	)
	var created struct {
		RunID   string `json:"run_id"`
		EventID string `json:"event_id"`
	}
	if err := json.Unmarshal(createdOutput, &created); err != nil {
		t.Fatalf("run create output is not JSON: %v\n%s", err, string(createdOutput))
	}
	if created.RunID == "" || created.EventID != "evt-000002" {
		t.Fatalf("created = %#v, want run id and evt-000002", created)
	}

	initOutput := runHelper(t, binary, repo, "phase-plan", "init", created.RunID, "--json")
	var initialized struct {
		PhasePlan struct {
			RunID  string `json:"run_id"`
			Path   string `json:"path"`
			Phases []struct {
				ID     string `json:"id"`
				Status string `json:"status"`
			} `json:"phases"`
		} `json:"phase_plan"`
		EventID string `json:"event_id"`
	}
	if err := json.Unmarshal(initOutput, &initialized); err != nil {
		t.Fatalf("phase-plan init output is not JSON: %v\n%s", err, string(initOutput))
	}
	if initialized.EventID != "evt-000003" || initialized.PhasePlan.RunID != created.RunID || initialized.PhasePlan.Path != ".kkachi/runs/"+created.RunID+"/phase-plan.yaml" || len(initialized.PhasePlan.Phases) == 0 {
		t.Fatalf("initialized = %#v, want phase plan path and evt-000003", initialized)
	}
	assertFileContains(t, filepath.Join(repo, ".kkachi", "runs", created.RunID, "phase-plan.yaml"), "request-feedback-1", "phase plan file")
	assertFileContains(t, filepath.Join(repo, ".kkachi", "events.jsonl"), `"type":"phase_plan.initialized"`, "phase init event")

	setOutput := runHelper(t, binary, repo, "phase-plan", "set", created.RunID, "ask", "--status", "not_applicable", "--reason", "No actionable question.", "--json")
	var updated struct {
		Phase struct {
			ID     string `json:"id"`
			Status string `json:"status"`
			Reason string `json:"reason"`
		} `json:"phase"`
		EventID string `json:"event_id"`
	}
	if err := json.Unmarshal(setOutput, &updated); err != nil {
		t.Fatalf("phase-plan set output is not JSON: %v\n%s", err, string(setOutput))
	}
	if updated.EventID != "evt-000004" || updated.Phase.ID != "ask" || updated.Phase.Status != "not_applicable" || updated.Phase.Reason == "" {
		t.Fatalf("updated = %#v, want ask not_applicable with reason", updated)
	}
	assertFileContains(t, filepath.Join(repo, ".kkachi", "events.jsonl"), `"type":"phase_plan.updated"`, "phase update event")

	validateOutput := runHelper(t, binary, repo, "phase-plan", "validate", created.RunID[:24], "--json")
	var validation struct {
		RunID  string `json:"run_id"`
		Status string `json:"status"`
		Checks []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"checks"`
	}
	if err := json.Unmarshal(validateOutput, &validation); err != nil {
		t.Fatalf("phase-plan validate output is not JSON: %v\n%s", err, string(validateOutput))
	}
	requiredPhasesPassed := false
	for _, check := range validation.Checks {
		if check.Name == "required_phases" && check.Status == "pass" {
			requiredPhasesPassed = true
			break
		}
	}
	if validation.RunID != created.RunID || validation.Status != "pass" || !requiredPhasesPassed {
		t.Fatalf("validation = %#v, want required phase validation pass", validation)
	}

	finalOutput, err := runHelperAllowError(binary, repo, "phase-plan", "validate", created.RunID, "--final", "--json")
	if err == nil {
		t.Fatalf("phase-plan final unexpectedly passed: %s", string(finalOutput))
	}
	assertOutputContains(t, finalOutput, `"status":"fail"`, "phase final validation")
	assertOutputContains(t, finalOutput, `"final_terminal_states"`, "phase final validation")

	diagnostics := runHelper(t, binary, repo, "diagnostics", "export", "--run", created.RunID, "--json")
	assertOutputContains(t, diagnostics, `"path":".kkachi/runs/`+created.RunID+`/phase-plan.yaml"`, "phase diagnostics")
}

func TestApprovalBinaryFlowDiagnosticsAndFinalPhaseValidation(t *testing.T) {
	binary := buildHelperBinary(t)
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}

	runHelper(t, binary, repo, "project", "init", "--json")
	createdOutput := runHelper(t, binary, repo,
		"run", "create",
		"--title", "Approval integration",
		"--work-path", "A_development_execution",
		"--work-mode", "standard",
		"--urgency", "normal",
		"--sot-policy", "existing_sot_basis",
		"--execution-mode", "production_write",
		"--commander", "Gongmyeong",
		"--task-id", "align-007",
		"--json",
	)
	var created struct {
		RunID string `json:"run_id"`
	}
	if err := json.Unmarshal(createdOutput, &created); err != nil {
		t.Fatalf("run create output is not JSON: %v\n%s", err, string(createdOutput))
	}
	runHelper(t, binary, repo, "phase-plan", "init", created.RunID, "--json")
	setOutput := runHelper(t, binary, repo, "phase-plan", "set", created.RunID, "implement", "--status", "in_progress", "--approval-required", "true", "--json")
	var updated struct {
		Phase struct {
			ID               string `json:"id"`
			Status           string `json:"status"`
			ApprovalRequired bool   `json:"approval_required"`
		} `json:"phase"`
	}
	if err := json.Unmarshal(setOutput, &updated); err != nil {
		t.Fatalf("phase-plan set output is not JSON: %v\n%s", err, string(setOutput))
	}
	if updated.Phase.ID != "implement" || !updated.Phase.ApprovalRequired {
		t.Fatalf("updated = %#v, want implement approval_required", updated)
	}

	requestOutput := runHelper(t, binary, repo, "approval", "request", created.RunID, "--phase", "implement", "--reason", "High-risk phase needs master approval.", "--evidence", "plan.md#approval", "--json")
	var requested struct {
		Record struct {
			Type      string `json:"type"`
			Phase     string `json:"phase"`
			Reason    string `json:"reason"`
			Timestamp string `json:"timestamp"`
			Evidence  string `json:"evidence"`
		} `json:"record"`
		EventID string `json:"event_id"`
	}
	if err := json.Unmarshal(requestOutput, &requested); err != nil {
		t.Fatalf("approval request output is not JSON: %v\n%s", err, string(requestOutput))
	}
	if requested.Record.Type != "approval.requested" || requested.Record.Phase != "implement" || requested.Record.Timestamp == "" || requested.EventID == "" {
		t.Fatalf("requested = %#v, want approval request", requested)
	}

	finalBefore, err := runHelperAllowError(binary, repo, "phase-plan", "validate", created.RunID, "--final", "--json")
	if err == nil {
		t.Fatalf("phase-plan final unexpectedly passed before approval: %s", string(finalBefore))
	}
	assertOutputContains(t, finalBefore, `"name":"final_approval_records","status":"fail"`, "final approval check before decision")

	recordOutput := runHelper(t, binary, repo, "approval", "record", created.RunID, "--phase", "implement", "--decision", "approved", "--by", "master", "--evidence", "messages/approval-123", "--reason", "Approved after review.", "--json")
	var recorded struct {
		Record struct {
			Type     string `json:"type"`
			Phase    string `json:"phase"`
			Decision string `json:"decision"`
			Approver string `json:"approver"`
			Evidence string `json:"evidence"`
		} `json:"record"`
		EventID string `json:"event_id"`
	}
	if err := json.Unmarshal(recordOutput, &recorded); err != nil {
		t.Fatalf("approval record output is not JSON: %v\n%s", err, string(recordOutput))
	}
	if recorded.Record.Type != "approval.recorded" || recorded.Record.Decision != "approved" || recorded.Record.Approver != "master" || recorded.Record.Evidence == "" {
		t.Fatalf("recorded = %#v, want approved decision", recorded)
	}

	showOutput := runHelper(t, binary, repo, "approval", "show", created.RunID, "--phase", "implement", "--json")
	var shown struct {
		RunID   string `json:"run_id"`
		Phase   string `json:"phase"`
		Records []struct {
			Type     string `json:"type"`
			Decision string `json:"decision"`
		} `json:"records"`
	}
	if err := json.Unmarshal(showOutput, &shown); err != nil {
		t.Fatalf("approval show output is not JSON: %v\n%s", err, string(showOutput))
	}
	if shown.RunID != created.RunID || shown.Phase != "implement" || len(shown.Records) != 2 || shown.Records[1].Decision != "approved" {
		t.Fatalf("shown = %#v, want request and approved decision", shown)
	}

	finalAfter, err := runHelperAllowError(binary, repo, "phase-plan", "validate", created.RunID, "--final", "--json")
	if err == nil {
		t.Fatalf("phase-plan final unexpectedly passed with incomplete phases: %s", string(finalAfter))
	}
	assertOutputContains(t, finalAfter, `"name":"final_approval_records","status":"pass"`, "final approval check after decision")

	diagnostics := runHelper(t, binary, repo, "diagnostics", "export", "--run", created.RunID, "--json")
	assertOutputContains(t, diagnostics, `"approval_records":[`, "approval diagnostics")
	assertOutputContains(t, diagnostics, `"type":"approval.requested"`, "approval diagnostics")
	assertOutputContains(t, diagnostics, `"decision":"approved"`, "approval diagnostics")
	assertFileContains(t, filepath.Join(repo, ".kkachi", "events.jsonl"), `"type":"approval.requested"`, "approval request event")
	assertFileContains(t, filepath.Join(repo, ".kkachi", "events.jsonl"), `"type":"approval.recorded"`, "approval record event")
}

func TestInstallCommandRemovedAtBinaryBoundary(t *testing.T) {
	binary := buildHelperBinary(t)
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	output, err := runHelperAllowError(binary, repo, "install", "templates", "--json")
	if err == nil {
		t.Fatalf("install unexpectedly succeeded: %s", string(output))
	}
	assertOutputContains(t, output, "unknown_command", "removed install command")
}

func TestProjectInitForceReconfiguresBootstrapWithoutDeletingRuns(t *testing.T) {
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	binary := buildHelperBinary(t)
	runHelper(t, binary, repo, "project", "init", "--json")

	writeIntegrationTarget(t, repo, ".kkachi/runs/manual-run/evidence.md", "keep me\n")
	forceArgs := []string{"project", "init", "--project-name", "kkachi-reset", "--stack", "rust", "--repo-path", "/tmp/kkachi-reset", "--commander", "Sunji", "--redteam", "Macho", "--docs-map-roadmap", "docs/ROADMAP.md", "--docs-map-spec", "docs/SPEC.md", "--docs-map-architecture", "docs/ARCHITECTURE.md", "--docs-map-adr-dir", "docs/decisions", "--docs-map-todo-dir", "docs/tasks", "--docs-map-spec-dir", "docs/specifications", "--test-commands", "cargo test,make verify", "--backend-policy", "codex", "--execution-mode", "readiness_hardening", "--sot-policy", "existing_sot_basis", "--force", "--json"}
	output := runHelper(t, binary, repo, forceArgs...)
	var payload struct {
		ProjectID           string `json:"project_id"`
		ProjectName         string `json:"project_name"`
		Forced              bool   `json:"forced"`
		ReconfiguredEventID string `json:"reconfigured_event_id"`
	}
	if err := json.Unmarshal(output, &payload); err != nil {
		t.Fatalf("force output is not JSON: %v\n%s", err, string(output))
	}
	if !payload.Forced || payload.ProjectName != "kkachi-reset" || payload.ProjectID == "" || payload.ReconfiguredEventID != "evt-000002" {
		t.Fatalf("payload = %#v, want forced reconfigure evt-000002", payload)
	}
	assertFileContains(t, filepath.Join(repo, ".kkachi", "project-overlay.yaml"), `project: "kkachi-reset"`, "force overlay project")
	assertFileContains(t, filepath.Join(repo, ".kkachi", "project-overlay.yaml"), `stack: "rust"`, "force overlay stack")
	assertFileContains(t, filepath.Join(repo, "docs", "kkachi-docs-map.yaml"), `roadmap: "docs/ROADMAP.md"`, "force docs map roadmap")
	assertFileContains(t, filepath.Join(repo, ".kkachi", "events.jsonl"), `"type":"project.reconfigured"`, "force event")
	assertFileContains(t, filepath.Join(repo, ".kkachi", "runs", "manual-run", "evidence.md"), "keep me", "force preserved run evidence")
	status := runHelper(t, binary, repo, "project", "status", "--json")
	assertOutputContains(t, status, `"event_tail_id":"evt-000002"`, "force status tail")
}

func TestProjectInitCreatesStateAndRefusesOverwrite(t *testing.T) {
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	binary := buildHelperBinary(t)

	cmd := exec.Command(binary, expandProjectInitArgs([]string{"project", "init", "--json"})...)
	cmd.Dir = repo
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("project init failed: %v\n%s", err, string(output))
	}

	var payload struct {
		RootPath       string   `json:"root_path"`
		ProjectID      string   `json:"project_id"`
		ProjectName    string   `json:"project_name"`
		CreatedPaths   []string `json:"created_paths"`
		SchemaPaths    []string `json:"schema_paths"`
		InitialEventID string   `json:"initial_event_id"`
	}
	if err := json.Unmarshal(output, &payload); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, string(output))
	}
	wantRoot, err := filepath.EvalSymlinks(repo)
	if err != nil {
		t.Fatalf("eval repo path: %v", err)
	}
	if payload.RootPath != wantRoot {
		t.Fatalf("root_path = %q, want %q", payload.RootPath, wantRoot)
	}
	if payload.ProjectID == "" || payload.ProjectName == "" || payload.InitialEventID != "evt-000001" {
		t.Fatalf("payload = %#v, want project identity and initial event", payload)
	}
	if len(payload.CreatedPaths) != 5 || len(payload.SchemaPaths) != 6 {
		t.Fatalf("paths = %#v/%#v, want state and schema paths", payload.CreatedPaths, payload.SchemaPaths)
	}

	statusCmd := exec.Command(binary, "project", "status", "--json")
	statusCmd.Dir = repo
	statusOutput, err := statusCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("project status failed: %v\n%s", err, string(statusOutput))
	}
	if !strings.Contains(string(statusOutput), `"health":"ok"`) || !strings.Contains(string(statusOutput), `"event_tail_id":"evt-000001"`) {
		t.Fatalf("project status output = %s, want healthy event tail", string(statusOutput))
	}

	doctorCmd := exec.Command(binary, "project", "doctor", "--json")
	doctorCmd.Dir = repo
	doctorOutput, err := doctorCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("project doctor failed: %v\n%s", err, string(doctorOutput))
	}
	if !strings.Contains(string(doctorOutput), `"health":"ok"`) || !strings.Contains(string(doctorOutput), `"failed":0`) {
		t.Fatalf("project doctor output = %s, want healthy checks", string(doctorOutput))
	}

	schemaValidate := exec.Command(binary, "schema", "validate", ".kkachi/status.json", "--schema", "status", "--json")
	schemaValidate.Dir = repo
	schemaOutput, err := schemaValidate.CombinedOutput()
	if err != nil {
		t.Fatalf("schema validate failed: %v\n%s", err, string(schemaOutput))
	}
	if !strings.Contains(string(schemaOutput), `"schema":"status"`) || !strings.Contains(string(schemaOutput), `"status":"pass"`) {
		t.Fatalf("schema validate output = %s, want status pass", string(schemaOutput))
	}
	writeIntegrationJSONFile(t, filepath.Join(repo, ".kkachi", "selected-cli.json"), map[string]any{
		"version":           "0.1",
		"status":            "supported",
		"backend_type":      "codex",
		"adapter_type":      "openai-codex",
		"source_ledger_ref": "docs/ledger.md#codex",
		"caveats":           []string{},
	})
	writeIntegrationJSONFile(t, filepath.Join(repo, ".kkachi", "bridge-session-snapshot.json"), map[string]any{
		"session_id":      "session-123",
		"backend_type":    "codex",
		"adapter_type":    "openai-codex",
		"state":           "running",
		"lifecycle_class": "interactive",
		"open_pendings":   0,
	})
	for _, args := range [][]string{
		{"schema", "validate", ".kkachi/selected-cli.json", "--schema", "selected-cli", "--json"},
		{"schema", "validate", ".kkachi/bridge-session-snapshot.json", "--schema", "bridge-session-snapshot", "--json"},
	} {
		validateCmd := exec.Command(binary, args...)
		validateCmd.Dir = repo
		validateOutput, err := validateCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, string(validateOutput))
		}
		if !strings.Contains(string(validateOutput), `"status":"pass"`) {
			t.Fatalf("%v output = %s, want pass", args, string(validateOutput))
		}
	}

	required := []string{
		".kkachi/config.yaml",
		".kkachi/status.json",
		".kkachi/events.jsonl",
		".kkachi/schemas/config.schema.json",
		".kkachi/schemas/status.schema.json",
		".kkachi/schemas/event.schema.json",
		".kkachi/schemas/run-metadata.schema.json",
		".kkachi/schemas/selected-cli.schema.json",
		".kkachi/schemas/bridge-session-snapshot.schema.json",
	}
	for _, relative := range required {
		if _, err := os.Stat(filepath.Join(repo, filepath.FromSlash(relative))); err != nil {
			t.Fatalf("%s was not created: %v", relative, err)
		}
	}

	oldSchemaPath := filepath.Join(repo, ".kkachi", "schemas", "selected-cli.schema.json")
	if err := os.WriteFile(oldSchemaPath, []byte(`{"$id":"https://kkachi.local/schemas/selected-cli.schema.json","required":["version"]}`+"\n"), 0o600); err != nil {
		t.Fatalf("write old schema: %v", err)
	}
	schemaExport := exec.Command(binary, "schema", "export", "--schema", "selected-cli", "--json")
	schemaExport.Dir = repo
	exportOutput, err := schemaExport.CombinedOutput()
	if err != nil {
		t.Fatalf("schema export failed: %v\n%s", err, string(exportOutput))
	}
	if !strings.Contains(string(exportOutput), `"written":[".kkachi/schemas/selected-cli.schema.json"]`) || !strings.Contains(string(exportOutput), `"event_id":"evt-000002"`) {
		t.Fatalf("schema export output = %s, want written selected-cli schema and event", string(exportOutput))
	}
	schemaExportAll := exec.Command(binary, "schema", "export", "--all", "--json")
	schemaExportAll.Dir = repo
	exportAllOutput, err := schemaExportAll.CombinedOutput()
	if err != nil {
		t.Fatalf("schema export --all failed: %v\n%s", err, string(exportAllOutput))
	}
	if !strings.Contains(string(exportAllOutput), `"written":null`) || strings.Contains(string(exportAllOutput), `"event_id"`) {
		t.Fatalf("schema export --all output = %s, want no event_id field and no writes", string(exportAllOutput))
	}

	runCreate := exec.Command(binary, "run", "create", "--title", "Run workflow metadata", "--work-path", "A_development_execution", "--work-mode", "standard", "--urgency", "normal", "--sot-policy", "existing_sot_basis", "--execution-mode", "production_write", "--commander", "Gongmyeong", "--task-id", "runwf-001", "--json")
	runCreate.Dir = repo
	runCreateOutput, err := runCreate.CombinedOutput()
	if err != nil {
		t.Fatalf("run create failed: %v\n%s", err, string(runCreateOutput))
	}
	var createdRun struct {
		RunID   string `json:"run_id"`
		State   string `json:"state"`
		EventID string `json:"event_id"`
	}
	if err := json.Unmarshal(runCreateOutput, &createdRun); err != nil {
		t.Fatalf("run create output is not JSON: %v\n%s", err, string(runCreateOutput))
	}
	if !strings.HasPrefix(createdRun.RunID, "run-") || createdRun.State != "created" || createdRun.EventID != "evt-000003" {
		t.Fatalf("createdRun = %#v, want created evt-000003", createdRun)
	}

	runShow := exec.Command(binary, "run", "show", createdRun.RunID, "--json")
	runShow.Dir = repo
	runShowOutput, err := runShow.CombinedOutput()
	if err != nil {
		t.Fatalf("run show failed: %v\n%s", err, string(runShowOutput))
	}
	if !strings.Contains(string(runShowOutput), `"task_id":"runwf-001"`) || !strings.Contains(string(runShowOutput), `"required_artifacts":[]`) {
		t.Fatalf("run show output = %s, want metadata", string(runShowOutput))
	}
	runMetadataValidate := exec.Command(binary, "schema", "validate", ".kkachi/runs/"+createdRun.RunID+"/run-metadata.json", "--schema", "run-metadata", "--json")
	runMetadataValidate.Dir = repo
	runMetadataValidateOutput, err := runMetadataValidate.CombinedOutput()
	if err != nil {
		t.Fatalf("run metadata schema validate failed: %v\n%s", err, string(runMetadataValidateOutput))
	}
	if !strings.Contains(string(runMetadataValidateOutput), `"schema":"run-metadata"`) || !strings.Contains(string(runMetadataValidateOutput), `"status":"pass"`) {
		t.Fatalf("run metadata schema validate output = %s, want pass", string(runMetadataValidateOutput))
	}

	runActivate := exec.Command(binary, "run", "activate", createdRun.RunID, "--json")
	runActivate.Dir = repo
	runActivateOutput, err := runActivate.CombinedOutput()
	if err != nil {
		t.Fatalf("run activate failed: %v\n%s", err, string(runActivateOutput))
	}
	if !strings.Contains(string(runActivateOutput), `"state":"active"`) || !strings.Contains(string(runActivateOutput), `"event_id":"evt-000004"`) {
		t.Fatalf("run activate output = %s, want active evt-000004", string(runActivateOutput))
	}

	runClose := exec.Command(binary, "run", "close", createdRun.RunID, "--json")
	runClose.Dir = repo
	runCloseOutput, err := runClose.CombinedOutput()
	if err != nil {
		t.Fatalf("run close failed: %v\n%s", err, string(runCloseOutput))
	}
	if !strings.Contains(string(runCloseOutput), `"state":"closed"`) || !strings.Contains(string(runCloseOutput), `"event_id":"evt-000005"`) {
		t.Fatalf("run close output = %s, want closed evt-000005", string(runCloseOutput))
	}

	appendCmd := exec.Command(binary, "event", "append", "artifact.written", "--run", "run-abc", "--payload", `{"path":"impl-log.md"}`, "--json")
	appendCmd.Dir = repo
	appendOutput, err := appendCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("event append failed: %v\n%s", err, string(appendOutput))
	}
	if !strings.Contains(string(appendOutput), `"event_id":"evt-000006"`) {
		t.Fatalf("event append output = %s, want evt-000006", string(appendOutput))
	}
	statusBytes, err := os.ReadFile(filepath.Join(repo, ".kkachi", "status.json"))
	if err != nil {
		t.Fatalf("read status after event append: %v", err)
	}
	if !strings.Contains(string(statusBytes), `"last_event_id": "evt-000006"`) {
		t.Fatalf("status after event append = %s, want evt-000006", string(statusBytes))
	}

	retry := exec.Command(binary, expandProjectInitArgs([]string{"project", "init", "--json"})...)
	retry.Dir = repo
	retryOutput, err := retry.CombinedOutput()
	if err == nil {
		t.Fatalf("second project init succeeded, want overwrite refusal\n%s", string(retryOutput))
	}
	if !strings.Contains(string(retryOutput), `"code":"helper_state_exists"`) {
		t.Fatalf("retry output = %s, want helper_state_exists", string(retryOutput))
	}
}

func TestPackg002SchemaMigrateBacksUpRunMetadata(t *testing.T) {
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	binary := buildHelperBinary(t)

	runHelper(t, binary, repo, "project", "init", "--json")
	createdOutput := runHelper(t, binary, repo,
		"run", "create",
		"--title", "Packg migration integration",
		"--work-path", "A_development_execution",
		"--work-mode", "standard",
		"--urgency", "normal",
		"--sot-policy", "existing_sot_basis",
		"--execution-mode", "production_write",
		"--commander", "Gongmyeong",
		"--task-id", "packg-002",
		"--json",
	)
	var created struct {
		RunID   string `json:"run_id"`
		EventID string `json:"event_id"`
	}
	if err := json.Unmarshal(createdOutput, &created); err != nil {
		t.Fatalf("run create output is not JSON: %v\n%s", err, string(createdOutput))
	}
	if created.EventID != "evt-000002" {
		t.Fatalf("created event id = %q, want evt-000002", created.EventID)
	}

	beforeEvents := readFile(t, filepath.Join(repo, ".kkachi", "events.jsonl"))
	dryRunOutput := runHelper(t, binary, repo, "schema", "migrate", "--from", "0.1", "--to", "0.1", "--dry-run", "--json")
	var dryRun struct {
		DryRun      bool     `json:"dry_run"`
		WouldBackup []string `json:"would_backup"`
		BackedUp    []string `json:"backed_up"`
		EventID     string   `json:"event_id"`
	}
	if err := json.Unmarshal(dryRunOutput, &dryRun); err != nil {
		t.Fatalf("schema migrate dry-run output is not JSON: %v\n%s", err, string(dryRunOutput))
	}
	if !dryRun.DryRun || dryRun.EventID != "" || len(dryRun.WouldBackup) == 0 || len(dryRun.BackedUp) != 0 {
		t.Fatalf("dryRun = %#v, want read-only summary", dryRun)
	}
	if got := readFile(t, filepath.Join(repo, ".kkachi", "events.jsonl")); string(got) != string(beforeEvents) {
		t.Fatalf("schema migrate dry-run mutated events\nbefore=%s\nafter=%s", string(beforeEvents), string(got))
	}

	migrateOutput := runHelper(t, binary, repo, "schema", "migrate", "--from", "0.1", "--to", "0.1", "--json")
	var migrated struct {
		DryRun     bool     `json:"dry_run"`
		EventID    string   `json:"event_id"`
		BackupPath string   `json:"backup_path"`
		BackedUp   []string `json:"backed_up"`
		Unchanged  []string `json:"unchanged"`
	}
	if err := json.Unmarshal(migrateOutput, &migrated); err != nil {
		t.Fatalf("schema migrate output is not JSON: %v\n%s", err, string(migrateOutput))
	}
	metadataRelative := ".kkachi/runs/" + created.RunID + "/run-metadata.json"
	if migrated.DryRun || migrated.EventID != "evt-000003" || migrated.BackupPath == "" || !stringListed(migrated.BackedUp, metadataRelative) || !stringListed(migrated.Unchanged, metadataRelative) {
		t.Fatalf("migrated = %#v, want run metadata backup and evt-000003", migrated)
	}
	backupMetadata := filepath.Join(repo, filepath.FromSlash(migrated.BackupPath), filepath.FromSlash(metadataRelative))
	assertOutputContains(t, readFile(t, backupMetadata), `"task_id": "packg-002"`, "migration backup run metadata")
	assertOutputContains(t, readFile(t, filepath.Join(repo, ".kkachi", "events.jsonl")), `"type":"schema.migrated"`, "schema migrate events")
	assertOutputContains(t, readFile(t, filepath.Join(repo, ".kkachi", "status.json")), `"last_event_id": "evt-000003"`, "schema migrate status")
}

func TestGates001And002GateCheckWorkflow(t *testing.T) {
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	binary := buildHelperBinary(t)

	runHelper(t, binary, repo, "project", "init", "--json")
	createdOutput := runHelper(t, binary, repo,
		"run", "create",
		"--title", "Gate check workflow",
		"--work-path", "A_development_execution",
		"--work-mode", "standard",
		"--urgency", "normal",
		"--sot-policy", "existing_sot_basis",
		"--execution-mode", "production_write",
		"--commander", "Gongmyeong",
		"--task-id", "gates-002",
		"--json",
	)
	var created struct {
		RunID    string `json:"run_id"`
		EventID  string `json:"event_id"`
		Metadata struct {
			WorkPath  string `json:"work_path"`
			WorkMode  string `json:"work_mode"`
			SOTPolicy string `json:"sot_policy"`
			Urgency   string `json:"urgency"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(createdOutput, &created); err != nil {
		t.Fatalf("run create output is not JSON: %v\n%s", err, string(createdOutput))
	}
	if created.EventID != "evt-000002" {
		t.Fatalf("created event id = %q, want evt-000002", created.EventID)
	}
	runHelper(t, binary, repo, "artifact", "init", created.RunID, "--json")

	pendingOutput, err := runHelperAllowError(binary, repo, "gate", "check", created.RunID, "intake", "--json")
	if err == nil {
		t.Fatalf("gate check succeeded with pending intake\n%s", string(pendingOutput))
	}
	var pending gateCheckOutput
	if err := json.Unmarshal(pendingOutput, &pending); err != nil {
		t.Fatalf("pending gate output is not JSON: %v\n%s", err, string(pendingOutput))
	}
	if pending.RunID != created.RunID || pending.Gate != "intake" || pending.Status != "fail" || pending.EventID != "evt-000004" || pending.ReportPath == "" || len(pending.MissingEvidence) == 0 || !gateCheckListed(pending.Checks, "intake_status", "fail") {
		t.Fatalf("pending gate = %#v, want intake fail with report path, evidence, and evt-000004", pending)
	}
	assertOutputContains(t, readFile(t, filepath.Join(repo, pending.ReportPath)), `"status": "fail"`, "failed gate report")
	assertOutputContains(t, readFile(t, filepath.Join(repo, pending.ReportPath)), `"event_id": "evt-000004"`, "failed gate report")
	assertOutputContains(t, readFile(t, filepath.Join(repo, ".kkachi", "events.jsonl")), `"type":"gate.failed"`, "events after failed gate check")
	assertOutputContains(t, readFile(t, filepath.Join(repo, ".kkachi", "status.json")), `"event_id": "evt-000004"`, "status after failed gate check")
	assertOutputContains(t, readFile(t, filepath.Join(repo, ".kkachi", "runs", created.RunID, "run-metadata.json")), `"status": "fail"`, "metadata after failed gate check")

	writeIntegrationIntake(t, repo, created.RunID, created.Metadata.WorkPath, created.Metadata.WorkMode, created.Metadata.SOTPolicy, created.Metadata.Urgency, "")
	passOutput := runHelper(t, binary, repo, "gate", "check", created.RunID[:24], "intake", "--json")
	var passed gateCheckOutput
	if err := json.Unmarshal(passOutput, &passed); err != nil {
		t.Fatalf("passing gate output is not JSON: %v\n%s", err, string(passOutput))
	}
	if passed.RunID != created.RunID || passed.Gate != "intake" || passed.Status != "pass" || passed.EventID != "evt-000005" || passed.ReportPath != pending.ReportPath || len(passed.MissingEvidence) != 0 || !gateCheckListed(passed.Checks, "required_artifacts", "pass") {
		t.Fatalf("passed gate = %#v, want intake pass with same report path %q and evt-000005", passed, pending.ReportPath)
	}
	assertOutputContains(t, readFile(t, filepath.Join(repo, passed.ReportPath)), `"status": "pass"`, "passing gate report")
	assertOutputContains(t, readFile(t, filepath.Join(repo, passed.ReportPath)), `"event_id": "evt-000005"`, "passing gate report")
	assertOutputContains(t, readFile(t, filepath.Join(repo, ".kkachi", "events.jsonl")), `"type":"gate.passed"`, "events after passing gate check")
	assertOutputContains(t, readFile(t, filepath.Join(repo, ".kkachi", "status.json")), `"status": "pass"`, "status after passing gate check")
	assertOutputContains(t, readFile(t, filepath.Join(repo, ".kkachi", "runs", created.RunID, "run-metadata.json")), `"event_id": "evt-000005"`, "metadata after passing gate check")

	planPendingOutput, err := runHelperAllowError(binary, repo, "gate", "check", created.RunID, "plan", "--json")
	if err == nil {
		t.Fatalf("plan gate check succeeded with pending plan artifacts\n%s", string(planPendingOutput))
	}
	var planPending gateCheckOutput
	if err := json.Unmarshal(planPendingOutput, &planPending); err != nil {
		t.Fatalf("plan pending gate output is not JSON: %v\n%s", err, string(planPendingOutput))
	}
	if planPending.Status != "fail" || planPending.EventID != "evt-000006" || len(planPending.MissingEvidence) != 3 || !gateCheckListed(planPending.Checks, "acceptance_criteria", "fail") || !gateCheckListed(planPending.Checks, "plan_artifact", "fail") || !gateCheckListed(planPending.Checks, "checklist_artifact", "fail") {
		t.Fatalf("planPending = %#v, want failed plan gate with pending artifacts", planPending)
	}

	writeIntegrationMarkdownArtifact(t, repo, created.RunID, "sot-basis.md", "Status: complete\nSource: docs/specs.md\n")
	writeIntegrationMarkdownArtifact(t, repo, created.RunID, "acceptance-criteria.md", "Status: complete\nCriteria: pre-implementation safety\n")
	writeIntegrationMarkdownArtifact(t, repo, created.RunID, "plan.md", "Status: complete\nPlan: gates-002 deterministic validators\n")
	writeIntegrationMarkdownArtifact(t, repo, created.RunID, "checklist.md", "Status: complete\n- [x] SOT gate\n- [x] roadmap gate\n- [x] plan gate\n")

	sotOutput := runHelper(t, binary, repo, "gate", "check", created.RunID, "sot", "--json")
	var sotPassed gateCheckOutput
	if err := json.Unmarshal(sotOutput, &sotPassed); err != nil {
		t.Fatalf("sot gate output is not JSON: %v\n%s", err, string(sotOutput))
	}
	if sotPassed.Status != "pass" || sotPassed.EventID != "evt-000007" || !gateCheckListed(sotPassed.Checks, "sot_basis", "pass") {
		t.Fatalf("sotPassed = %#v, want SOT pass", sotPassed)
	}

	roadmapOutput := runHelper(t, binary, repo, "gate", "check", created.RunID, "roadmap", "--json")
	var roadmapPassed gateCheckOutput
	if err := json.Unmarshal(roadmapOutput, &roadmapPassed); err != nil {
		t.Fatalf("roadmap gate output is not JSON: %v\n%s", err, string(roadmapOutput))
	}
	if roadmapPassed.Status != "pass" || roadmapPassed.EventID != "evt-000008" || !gateCheckListed(roadmapPassed.Checks, "roadmap_trace", "pass") {
		t.Fatalf("roadmapPassed = %#v, want roadmap trace pass", roadmapPassed)
	}

	planOutput := runHelper(t, binary, repo, "gate", "check", created.RunID, "plan", "--json")
	var planPassed gateCheckOutput
	if err := json.Unmarshal(planOutput, &planPassed); err != nil {
		t.Fatalf("plan gate output is not JSON: %v\n%s", err, string(planOutput))
	}
	if planPassed.Status != "pass" || planPassed.EventID != "evt-000009" || len(planPassed.MissingEvidence) != 0 || !gateCheckListed(planPassed.Checks, "acceptance_criteria", "pass") || !gateCheckListed(planPassed.Checks, "plan_artifact", "pass") || !gateCheckListed(planPassed.Checks, "checklist_artifact", "pass") {
		t.Fatalf("planPassed = %#v, want completed plan gate pass", planPassed)
	}
}

func TestGates003BackendGateIntegrationWorkflow(t *testing.T) {
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	binary := buildHelperBinary(t)

	runHelper(t, binary, repo, "project", "init", "--json")
	createdOutput := runHelper(t, binary, repo,
		"run", "create",
		"--title", "Backend gate integration",
		"--work-path", "A_development_execution",
		"--work-mode", "standard",
		"--urgency", "normal",
		"--sot-policy", "existing_sot_basis",
		"--execution-mode", "adapter_qa",
		"--commander", "Gongmyeong",
		"--task-id", "gates-003",
		"--json",
	)
	var created struct {
		RunID   string `json:"run_id"`
		EventID string `json:"event_id"`
	}
	if err := json.Unmarshal(createdOutput, &created); err != nil {
		t.Fatalf("run create output is not JSON: %v\n%s", err, string(createdOutput))
	}
	if created.EventID != "evt-000002" {
		t.Fatalf("created event id = %q, want evt-000002", created.EventID)
	}
	initOutput := runHelper(t, binary, repo, "artifact", "init", created.RunID, "--json")
	assertOutputContains(t, initOutput, `"event_id":"evt-000003"`, "adapter artifact init")
	metadataPath := filepath.Join(repo, ".kkachi", "runs", created.RunID, "run-metadata.json")
	metadata := readFile(t, metadataPath)
	assertOutputContainsArtifacts(t, metadata, integrationBackendArtifacts(), "adapter metadata required artifacts")

	pendingOutput, err := runHelperAllowError(binary, repo, "gate", "check", created.RunID, "backend", "--json")
	if err == nil {
		t.Fatalf("backend gate succeeded with pending evidence\n%s", string(pendingOutput))
	}
	var pending gateCheckOutput
	if err := json.Unmarshal(pendingOutput, &pending); err != nil {
		t.Fatalf("pending backend output is not JSON: %v\n%s", err, string(pendingOutput))
	}
	if pending.RunID != created.RunID || pending.Gate != "backend" || pending.Status != "fail" || pending.EventID != "evt-000004" || len(pending.MissingEvidence) == 0 || !gateCheckListed(pending.Checks, "selected_cli", "fail") {
		t.Fatalf("pending backend = %#v, want selected_cli fail with evt-000004", pending)
	}
	assertOutputContains(t, readFile(t, filepath.Join(repo, ".kkachi", "events.jsonl")), `"type":"gate.failed"`, "events after pending backend gate")
	assertOutputContains(t, readFile(t, filepath.Join(repo, ".kkachi", "status.json")), `"backend": {`, "status after pending backend gate")
	assertOutputContains(t, readFile(t, metadataPath), `"status": "fail"`, "metadata after pending backend gate")

	writeIntegrationBackendEvidence(t, repo, created.RunID)
	passOutput := runHelper(t, binary, repo, "gate", "check", created.RunID[:24], "backend", "--json")
	var passed gateCheckOutput
	if err := json.Unmarshal(passOutput, &passed); err != nil {
		t.Fatalf("passing backend output is not JSON: %v\n%s", err, string(passOutput))
	}
	if passed.RunID != created.RunID || passed.Gate != "backend" || passed.Status != "pass" || passed.EventID != "evt-000005" || len(passed.MissingEvidence) != 0 {
		t.Fatalf("passed backend = %#v, want backend pass evt-000005", passed)
	}
	for _, check := range []string{"backend_manifest", "selected_cli", "capability_check", "bridge_session_snapshot", "bridge_events"} {
		if !gateCheckListed(passed.Checks, check, "pass") {
			t.Fatalf("passed checks = %#v, want %s pass", passed.Checks, check)
		}
	}
	assertOutputContains(t, readFile(t, filepath.Join(repo, ".kkachi", "events.jsonl")), `"type":"gate.passed"`, "events after passing backend gate")
	assertOutputContains(t, readFile(t, filepath.Join(repo, ".kkachi", "status.json")), `"event_id": "evt-000005"`, "status after passing backend gate")
	assertOutputContains(t, readFile(t, metadataPath), `"event_id": "evt-000005"`, "metadata after passing backend gate")
}

func TestAlign002ProductionWriteDeclaredBackendEvidenceIntegration(t *testing.T) {
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	binary := buildHelperBinary(t)

	runHelper(t, binary, repo, "project", "init", "--json")
	createdOutput := runHelper(t, binary, repo,
		"run", "create",
		"--title", "Declared backend production write",
		"--work-path", "A_development_execution",
		"--work-mode", "standard",
		"--urgency", "normal",
		"--sot-policy", "existing_sot_basis",
		"--execution-mode", "production_write",
		"--backend-evidence", "required",
		"--commander", "Gongmyeong",
		"--task-id", "align-002",
		"--json",
	)
	var created struct {
		RunID    string `json:"run_id"`
		Metadata struct {
			ExecutionMode   string `json:"execution_mode"`
			BackendEvidence string `json:"backend_evidence"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(createdOutput, &created); err != nil {
		t.Fatalf("run create output is not JSON: %v\n%s", err, string(createdOutput))
	}
	if created.Metadata.ExecutionMode != "production_write" || created.Metadata.BackendEvidence != "required" {
		t.Fatalf("metadata = %#v, want production_write with required backend evidence", created.Metadata)
	}

	initOutput := runHelper(t, binary, repo, "artifact", "init", created.RunID, "--json")
	assertOutputContainsArtifacts(t, initOutput, integrationProductionWriteBackendArtifacts(), "declared backend artifact init")
	metadataPath := filepath.Join(repo, ".kkachi", "runs", created.RunID, "run-metadata.json")
	metadata := readFile(t, metadataPath)
	assertOutputContains(t, metadata, `"backend_evidence": "required"`, "declared backend metadata")
	assertOutputContainsArtifacts(t, metadata, integrationBackendArtifacts(), "declared backend metadata required artifacts")

	pendingOutput, err := runHelperAllowError(binary, repo, "gate", "check", created.RunID, "backend", "--json")
	if err == nil {
		t.Fatalf("backend gate succeeded with pending declared evidence\n%s", string(pendingOutput))
	}
	var pending gateCheckOutput
	if err := json.Unmarshal(pendingOutput, &pending); err != nil {
		t.Fatalf("pending backend output is not JSON: %v\n%s", err, string(pendingOutput))
	}
	if pending.Status != "fail" || !gateCheckListed(pending.Checks, "selected_cli", "fail") {
		t.Fatalf("pending backend = %#v, want selected_cli fail", pending)
	}

	writeIntegrationBackendEvidence(t, repo, created.RunID)
	passOutput := runHelper(t, binary, repo, "gate", "check", created.RunID, "backend", "--json")
	var passed gateCheckOutput
	if err := json.Unmarshal(passOutput, &passed); err != nil {
		t.Fatalf("passing backend output is not JSON: %v\n%s", err, string(passOutput))
	}
	if passed.Status != "pass" || len(passed.MissingEvidence) != 0 || !gateCheckListed(passed.Checks, "bridge_events", "pass") {
		t.Fatalf("passed backend = %#v, want declared backend pass", passed)
	}
}

func TestRunwf002LockWorkflow(t *testing.T) {
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	binary := buildHelperBinary(t)

	runHelper(t, binary, repo, "project", "init", "--json")

	writeLockMetadata(t, repo, "project_write", lockMetadata{
		Version:   "0.1",
		LockName:  "project_write",
		OwnerPID:  os.Getpid(),
		Hostname:  mustHostname(t),
		Command:   "integration fresh writer",
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	})
	output, err := runHelperAllowError(binary, repo, runwf002CreateRunArgs("Blocked by write lock")...)
	if err == nil {
		t.Fatalf("run create succeeded under fresh project_write lock\n%s", string(output))
	}
	assertOutputContains(t, output, `"code":"lock_conflict"`, "fresh project lock conflict")
	assertOutputContains(t, readFile(t, filepath.Join(repo, ".kkachi", "events.jsonl")), `"event_id":"evt-000001"`, "events after refused create")
	removeLock(t, repo, "project_write")

	createdOutput := runHelper(t, binary, repo, runwf002CreateRunArgs("Lock workflow")...)
	var created struct {
		RunID string `json:"run_id"`
	}
	if err := json.Unmarshal(createdOutput, &created); err != nil {
		t.Fatalf("run create output is not JSON: %v\n%s", err, string(createdOutput))
	}

	writeLockMetadata(t, repo, "active_run", lockMetadata{
		Version:   "0.1",
		LockName:  "active_run",
		RunID:     created.RunID,
		OwnerPID:  os.Getpid(),
		Hostname:  mustHostname(t),
		Command:   "integration active lifecycle",
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	})
	output, err = runHelperAllowError(binary, repo, "run", "activate", created.RunID, "--json")
	if err == nil {
		t.Fatalf("run activate succeeded under fresh active_run lock\n%s", string(output))
	}
	assertOutputContains(t, output, `"code":"lock_conflict"`, "fresh active lock conflict")
	assertOutputContains(t, readFile(t, filepath.Join(repo, ".kkachi", "status.json")), `"active_run_id": null`, "status after refused activate")
	assertOutputContains(t, readFile(t, filepath.Join(repo, ".kkachi", "runs", created.RunID, "run-metadata.json")), `"state": "created"`, "metadata after refused activate")
	removeLock(t, repo, "active_run")

	old := time.Date(2026, 4, 30, 3, 4, 5, 0, time.UTC)
	writeLockMetadata(t, repo, "project_write", lockMetadata{
		Version:   "0.1",
		LockName:  "project_write",
		OwnerPID:  999999,
		Hostname:  "other-host",
		Command:   "integration stale writer",
		CreatedAt: old.Add(-31 * time.Minute).Format(time.RFC3339),
	})
	output, err = runHelperAllowError(binary, repo, runwf002CreateRunArgs("Blocked by stale lock")...)
	if err == nil {
		t.Fatalf("run create succeeded under stale project_write lock before recovery\n%s", string(output))
	}
	assertOutputContains(t, output, `"code":"lock_stale_recovery_required"`, "stale project lock refusal")

	doctorOutput, err := runHelperAllowError(binary, repo, "project", "doctor", "--json")
	if err != nil {
		t.Fatalf("project doctor failed under stale lock: %v\n%s", err, string(doctorOutput))
	}
	assertOutputContains(t, doctorOutput, `"health":"warning"`, "doctor stale lock health")
	assertOutputContains(t, doctorOutput, "lock recover", "doctor stale lock hint")

	recoverOutput := runHelper(t, binary, repo, "lock", "recover", "project-write", "--reason", "integration stale recovery", "--json")
	assertOutputContains(t, recoverOutput, `"lock_name":"project_write"`, "lock recover output")
	if _, err := os.Stat(filepath.Join(repo, ".kkachi", "project_write.lock")); !os.IsNotExist(err) {
		t.Fatalf("project_write lock stat = %v, want absent after recovery", err)
	}
	assertOutputContains(t, readFile(t, filepath.Join(repo, ".kkachi", "events.jsonl")), `"type":"lock.recovered"`, "events after recovery")

	runHelper(t, binary, repo, runwf002CreateRunArgs("After recovery")...)
}

func TestRunwf003ArtifactWorkflow(t *testing.T) {
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	binary := buildHelperBinary(t)

	runHelper(t, binary, repo, "project", "init", "--json")
	createdOutput := runHelper(t, binary, repo, runwf003CreateRunArgs("Artifact workflow")...)
	var created struct {
		RunID string `json:"run_id"`
	}
	if err := json.Unmarshal(createdOutput, &created); err != nil {
		t.Fatalf("run create output is not JSON: %v\n%s", err, string(createdOutput))
	}
	runDir := filepath.Join(repo, ".kkachi", "runs", created.RunID)
	if err := os.WriteFile(filepath.Join(runDir, "plan.md"), []byte("custom integration plan\n"), 0o600); err != nil {
		t.Fatalf("write custom plan: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "checklist.md"), nil, 0o600); err != nil {
		t.Fatalf("write empty checklist: %v", err)
	}

	initOutput := runHelper(t, binary, repo, "artifact", "init", created.RunID, "--json")
	var initialized struct {
		RunID             string             `json:"run_id"`
		EventID           string             `json:"event_id"`
		Created           []artifactPathOnly `json:"created"`
		Reinitialized     []artifactPathOnly `json:"reinitialized"`
		Preserved         []artifactPathOnly `json:"preserved"`
		RequiredArtifacts []string           `json:"required_artifacts"`
	}
	if err := json.Unmarshal(initOutput, &initialized); err != nil {
		t.Fatalf("artifact init output is not JSON: %v\n%s", err, string(initOutput))
	}
	if initialized.RunID != created.RunID || initialized.EventID != "evt-000003" || len(initialized.Created) == 0 || len(initialized.RequiredArtifacts) == 0 {
		t.Fatalf("initialized = %#v, want created artifacts and required manifest", initialized)
	}
	if !artifactPathListed(initialized.Preserved, "plan.md") || !artifactPathListed(initialized.Reinitialized, "checklist.md") {
		t.Fatalf("preserved=%#v reinitialized=%#v, want plan preserved and checklist reinitialized", initialized.Preserved, initialized.Reinitialized)
	}
	assertOutputContains(t, readFile(t, filepath.Join(runDir, "plan.md")), "custom integration plan", "preserved plan")
	assertOutputContains(t, readFile(t, filepath.Join(runDir, "checklist.md")), "Status: pending", "reinitialized checklist")
	for _, relative := range []string{"intake-classification.md", "sot-basis.md", "acceptance-criteria.md", "diff.patch", "impl-log.md", "test-log.md", "verification.md", "docs-update.md", "final-report.md", "redteam/final-gate-review.md"} {
		info, err := os.Stat(filepath.Join(runDir, filepath.FromSlash(relative)))
		if err != nil {
			t.Fatalf("artifact %s was not created: %v", relative, err)
		}
		if info.Size() == 0 {
			t.Fatalf("artifact %s is empty, want baseline content", relative)
		}
	}

	metadata := readFile(t, filepath.Join(runDir, "run-metadata.json"))
	assertOutputContains(t, metadata, `"required_artifacts": [`, "metadata after artifact init")
	assertOutputContains(t, metadata, `"diff.patch"`, "metadata production manifest")
	assertOutputContains(t, metadata, `"task-brief.md"`, "metadata standard mode manifest")
	assertOutputContains(t, metadata, `"redteam/final-gate-review.md"`, "metadata redteam manifest")
	assertOutputContains(t, readFile(t, filepath.Join(repo, ".kkachi", "events.jsonl")), `"type":"artifact.written"`, "events after artifact init")

	listOutput := runHelper(t, binary, repo, "artifact", "list", created.RunID[:24], "--json")
	var listed struct {
		RunID     string                 `json:"run_id"`
		Artifacts []artifactListedStatus `json:"artifacts"`
	}
	if err := json.Unmarshal(listOutput, &listed); err != nil {
		t.Fatalf("artifact list output is not JSON: %v\n%s", err, string(listOutput))
	}
	if listed.RunID != created.RunID || !artifactStatusListed(listed.Artifacts, "intake-classification.md", true, true) || !artifactStatusListed(listed.Artifacts, "plan.md", true, true) {
		t.Fatalf("listed = %#v, want initialized required artifacts", listed)
	}
}

func TestRunwf003ArtifactInitSafety(t *testing.T) {
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	binary := buildHelperBinary(t)

	runHelper(t, binary, repo, "project", "init", "--json")
	createdOutput := runHelper(t, binary, repo, runwf003CreateRunArgs("Artifact safety")...)
	var created struct {
		RunID string `json:"run_id"`
	}
	if err := json.Unmarshal(createdOutput, &created); err != nil {
		t.Fatalf("run create output is not JSON: %v\n%s", err, string(createdOutput))
	}

	writeLockMetadata(t, repo, "project_write", lockMetadata{
		Version:   "0.1",
		LockName:  "project_write",
		RunID:     created.RunID,
		OwnerPID:  os.Getpid(),
		Hostname:  mustHostname(t),
		Command:   "integration artifact init",
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	})
	output, err := runHelperAllowError(binary, repo, "artifact", "init", created.RunID, "--json")
	if err == nil {
		t.Fatalf("artifact init succeeded under fresh project_write lock\n%s", string(output))
	}
	assertOutputContains(t, output, `"code":"lock_conflict"`, "fresh project lock conflict")
	if _, err := os.Stat(filepath.Join(repo, ".kkachi", "runs", created.RunID, "intake-classification.md")); !os.IsNotExist(err) {
		t.Fatalf("artifact stat under lock = %v, want absent", err)
	}
	removeLock(t, repo, "project_write")

	eventsPath := filepath.Join(repo, ".kkachi", "events.jsonl")
	file, err := os.OpenFile(eventsPath, os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatalf("open event log: %v", err)
	}
	if _, err := file.WriteString(`{"version":"0.1","event_id":"evt-000003","occurred_at":"2026-04-30T03:00:00Z","run_id":"` + created.RunID + `","type":"run.created","actor":"helper","payload":{}}` + "\n"); err != nil {
		t.Fatalf("append crash event: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close event log: %v", err)
	}
	output, err = runHelperAllowError(binary, repo, "artifact", "init", created.RunID, "--json")
	if err == nil {
		t.Fatalf("artifact init succeeded under status/event mismatch\n%s", string(output))
	}
	assertOutputContains(t, output, `"code":"last_event_id_mismatch"`, "artifact init mismatch")
	if _, err := os.Stat(filepath.Join(repo, ".kkachi", "runs", created.RunID, "intake-classification.md")); !os.IsNotExist(err) {
		t.Fatalf("artifact stat under mismatch = %v, want absent", err)
	}
}

func TestRunwf004ArtifactValidateWorkflow(t *testing.T) {
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	binary := buildHelperBinary(t)

	runHelper(t, binary, repo, "project", "init", "--json")
	createdOutput := runHelper(t, binary, repo, runwf004CreateRunArgs("Validate workflow")...)
	var created struct {
		RunID    string `json:"run_id"`
		Metadata struct {
			WorkPath  string `json:"work_path"`
			WorkMode  string `json:"work_mode"`
			SOTPolicy string `json:"sot_policy"`
			Urgency   string `json:"urgency"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(createdOutput, &created); err != nil {
		t.Fatalf("run create output is not JSON: %v\n%s", err, string(createdOutput))
	}
	runHelper(t, binary, repo, "artifact", "init", created.RunID, "--json")
	beforeEvents := readFile(t, filepath.Join(repo, ".kkachi", "events.jsonl"))

	output, err := runHelperAllowError(binary, repo, "artifact", "validate", created.RunID, "--json")
	if err == nil {
		t.Fatalf("artifact validate succeeded with pending intake\n%s", string(output))
	}
	assertOutputContains(t, output, `"status":"fail"`, "pending intake validate")
	assertOutputContains(t, output, `"name":"intake_status"`, "pending intake validate")
	if got := readFile(t, filepath.Join(repo, ".kkachi", "events.jsonl")); string(got) != string(beforeEvents) {
		t.Fatalf("artifact validate mutated events\nbefore=%s\nafter=%s", string(beforeEvents), string(got))
	}

	writeIntegrationIntake(t, repo, created.RunID, created.Metadata.WorkPath, created.Metadata.WorkMode, created.Metadata.SOTPolicy, created.Metadata.Urgency, "")
	passOutput := runHelper(t, binary, repo, "artifact", "validate", created.RunID[:24], "--gate", "intake", "--json")
	assertOutputContains(t, passOutput, `"run_id":"`+created.RunID+`"`, "passing intake validate")
	assertOutputContains(t, passOutput, `"gate":"intake"`, "passing intake validate")
	assertOutputContains(t, passOutput, `"status":"pass"`, "passing intake validate")
	assertOutputContains(t, passOutput, `"name":"required_artifacts","status":"pass"`, "passing intake validate")

	output, err = runHelperAllowError(binary, repo, "artifact", "validate", created.RunID, "--gate", "final", "--json")
	if err == nil {
		t.Fatalf("artifact validate succeeded with unsupported gate\n%s", string(output))
	}
	assertOutputContains(t, output, `"code":"unsupported_gate"`, "unsupported gate validate")
}

func TestRunwf004ArtifactValidateLightPathBWorkflow(t *testing.T) {
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	binary := buildHelperBinary(t)

	runHelper(t, binary, repo, "project", "init", "--json")
	createdOutput := runHelper(t, binary, repo,
		"run", "create",
		"--title", "Path B light validate",
		"--work-path", "B_discovery_shaping",
		"--work-mode", "light",
		"--urgency", "critical",
		"--sot-policy", "minimal_sot_before_code",
		"--execution-mode", "research",
		"--commander", "Gongmyeong",
		"--task-id", "runwf-004",
		"--json",
	)
	var created struct {
		RunID string `json:"run_id"`
	}
	if err := json.Unmarshal(createdOutput, &created); err != nil {
		t.Fatalf("run create output is not JSON: %v\n%s", err, string(createdOutput))
	}
	runHelper(t, binary, repo, "artifact", "init", created.RunID, "--json")
	writeIntegrationIntake(t, repo, created.RunID, "B_discovery_shaping", "light", "minimal_sot_before_code", "critical", "")

	output, err := runHelperAllowError(binary, repo, "artifact", "validate", created.RunID, "--json")
	if err == nil {
		t.Fatalf("artifact validate succeeded without light mode reason\n%s", string(output))
	}
	assertOutputContains(t, output, `"name":"light_mode_reason"`, "missing light reason validate")
	assertOutputContains(t, output, `"status":"fail"`, "missing light reason validate")

	writeIntegrationIntake(t, repo, created.RunID, "B_discovery_shaping", "light", "minimal_sot_before_code", "critical", "Light Mode Reason: discovery is low-risk and still records safety artifacts\n")
	passOutput := runHelper(t, binary, repo, "artifact", "validate", created.RunID, "--json")
	assertOutputContains(t, passOutput, `"status":"pass"`, "Path B light validate")
	assertOutputContains(t, passOutput, `"name":"work_path_sot_policy","status":"pass"`, "Path B light validate")
	assertOutputContains(t, passOutput, `"name":"light_mode_reason","status":"pass"`, "Path B light validate")
}

type gateCheckOutput struct {
	RunID           string      `json:"run_id"`
	Gate            string      `json:"gate"`
	Status          string      `json:"status"`
	EventID         string      `json:"event_id"`
	ReportPath      string      `json:"report_path"`
	MissingEvidence []string    `json:"missing_evidence"`
	Checks          []gateCheck `json:"checks"`
}

type gateCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

func gateCheckListed(checks []gateCheck, name string, status string) bool {
	for _, check := range checks {
		if check.Name == name && check.Status == status {
			return true
		}
	}
	return false
}

func stringListed(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func buildHelperBinary(t *testing.T) string {
	t.Helper()

	binary := filepath.Join(t.TempDir(), "kkachi-agent-helper")
	cmd := exec.Command("go", "build", "-ldflags", "-X main.version=0.1.0", "-o", binary, "../..")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build failed: %v\n%s", err, string(output))
	}
	return binary
}

type lockMetadata struct {
	Version   string `json:"version"`
	LockName  string `json:"lock_name"`
	RunID     string `json:"run_id,omitempty"`
	OwnerPID  int    `json:"owner_pid"`
	Hostname  string `json:"hostname"`
	Command   string `json:"command"`
	CreatedAt string `json:"created_at"`
}

type artifactPathOnly struct {
	Path string `json:"path"`
}

type installTargetOnly struct {
	Target string `json:"target"`
}

type artifactListedStatus struct {
	Path     string `json:"path"`
	Required bool   `json:"required"`
	Exists   bool   `json:"exists"`
	Empty    bool   `json:"empty"`
	Bytes    int64  `json:"bytes"`
}

func runHelper(t *testing.T, binary string, repo string, args ...string) []byte {
	t.Helper()
	output, err := runHelperAllowError(binary, repo, args...)
	if err != nil {
		t.Fatalf("%s failed: %v\n%s", strings.Join(args, " "), err, string(output))
	}
	return output
}

func expandProjectInitArgs(args []string) []string {
	if len(args) >= 2 && args[0] == "project" && args[1] == "init" {
		for _, arg := range args[2:] {
			if arg == "--project-name" {
				return args
			}
		}
		extra := append([]string{}, args[2:]...)
		base := []string{"project", "init", "--project-name", "kkachi-test", "--stack", "go", "--repo-path", "/tmp/kkachi-test", "--commander", "Gongmyeong", "--redteam", "Macho", "--docs-map-roadmap", "docs/roadmap.md", "--docs-map-spec", "docs/specs.md", "--docs-map-architecture", "docs/architecture.md", "--docs-map-adr-dir", "docs/adr", "--docs-map-todo-dir", "docs/todo", "--docs-map-spec-dir", "docs/specs", "--test-commands", "go test ./...,make test", "--backend-policy", "codex", "--execution-mode", "production_write", "--sot-policy", "existing_sot_basis"}
		return append(base, extra...)
	}
	return args
}

func runHelperAllowError(binary string, repo string, args ...string) ([]byte, error) {
	args = expandProjectInitArgs(args)
	cmd := exec.Command(binary, args...)
	cmd.Dir = repo
	return cmd.CombinedOutput()
}

func runwf002CreateRunArgs(title string) []string {
	return []string{
		"run", "create",
		"--title", title,
		"--work-path", "A_development_execution",
		"--work-mode", "standard",
		"--urgency", "normal",
		"--sot-policy", "existing_sot_basis",
		"--execution-mode", "production_write",
		"--commander", "Gongmyeong",
		"--task-id", "runwf-002",
		"--json",
	}
}

func runwf003CreateRunArgs(title string) []string {
	return []string{
		"run", "create",
		"--title", title,
		"--work-path", "A_development_execution",
		"--work-mode", "standard",
		"--urgency", "normal",
		"--sot-policy", "existing_sot_basis",
		"--execution-mode", "production_write",
		"--commander", "Gongmyeong",
		"--redteam", "Reviewer",
		"--task-id", "runwf-003",
		"--json",
	}
}

func runwf004CreateRunArgs(title string) []string {
	return []string{
		"run", "create",
		"--title", title,
		"--work-path", "A_development_execution",
		"--work-mode", "standard",
		"--urgency", "normal",
		"--sot-policy", "existing_sot_basis",
		"--execution-mode", "production_write",
		"--commander", "Gongmyeong",
		"--task-id", "runwf-004",
		"--json",
	}
}

func writeIntegrationIntake(t *testing.T, repo string, runID string, workPath string, workMode string, sotPolicy string, urgency string, extra string) {
	t.Helper()
	content := strings.Join([]string{
		"# intake-classification.md",
		"",
		"Status: complete",
		"Work Path: " + workPath,
		"Work Mode: " + workMode,
		"SOT Policy: " + sotPolicy,
		"Urgency: " + urgency,
		strings.TrimRight(extra, "\n"),
		"",
	}, "\n")
	path := filepath.Join(repo, ".kkachi", "runs", runID, "intake-classification.md")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write intake classification: %v", err)
	}
}

func writeIntegrationMarkdownArtifact(t *testing.T, repo string, runID string, artifact string, body string) {
	t.Helper()
	path := filepath.Join(repo, ".kkachi", "runs", runID, filepath.FromSlash(artifact))
	content := "# " + artifact + "\n\n" + strings.TrimRight(body, "\n") + "\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", artifact, err)
	}
}

func writeIntegrationBackendEvidence(t *testing.T, repo string, runID string) {
	t.Helper()
	writeIntegrationJSONArtifact(t, repo, runID, "selected-cli.json", map[string]any{
		"version":           "0.1",
		"status":            "supported",
		"backend_type":      "codex",
		"adapter_type":      "openai-codex",
		"source_ledger_ref": "docs/ledger.md#codex",
		"caveats":           []string{},
	})
	writeIntegrationMarkdownArtifact(t, repo, runID, "capability-check.md", "Status: complete\nBackend Type: codex\nAdapter Type: openai-codex\nCapability: thread resume checked\n")
	writeIntegrationJSONArtifact(t, repo, runID, "bridge-session-snapshot.json", map[string]any{
		"session_id":      "session-123",
		"backend_type":    "codex",
		"adapter_type":    "openai-codex",
		"state":           "running",
		"lifecycle_class": "interactive",
		"open_pendings":   0,
	})
	writeIntegrationMarkdownArtifact(t, repo, runID, "bridge-events.md", "Status: complete\nEvent: bridge opened a codex session and emitted output\n")
}

func writeIntegrationJSONArtifact(t *testing.T, repo string, runID string, artifact string, payload any) {
	t.Helper()
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatalf("marshal %s: %v", artifact, err)
	}
	path := filepath.Join(repo, ".kkachi", "runs", runID, filepath.FromSlash(artifact))
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		t.Fatalf("write %s: %v", artifact, err)
	}
}

func artifactPathListed(artifacts []artifactPathOnly, path string) bool {
	for _, artifact := range artifacts {
		if artifact.Path == path {
			return true
		}
	}
	return false
}

func artifactStatusListed(artifacts []artifactListedStatus, path string, required bool, exists bool) bool {
	for _, artifact := range artifacts {
		if artifact.Path == path {
			return artifact.Required == required && artifact.Exists == exists && !artifact.Empty && artifact.Bytes > 0
		}
	}
	return false
}

func writeLockMetadata(t *testing.T, repo string, name string, metadata lockMetadata) {
	t.Helper()
	path := lockFilePath(repo, name)
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		t.Fatalf("marshal lock metadata: %v", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		t.Fatalf("write lock metadata: %v", err)
	}
}

func removeLock(t *testing.T, repo string, name string) {
	t.Helper()
	path := lockFilePath(repo, name)
	if err := os.Remove(path); err != nil {
		t.Fatalf("remove %s: %v", path, err)
	}
}

func lockFilePath(repo string, name string) string {
	if name == "active_run" {
		return filepath.Join(repo, ".kkachi", "active_run.lock")
	}
	return filepath.Join(repo, ".kkachi", "project_write.lock")
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}

func assertFileContains(t *testing.T, path string, want string, label string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if !strings.Contains(string(data), want) {
		t.Fatalf("%s = %s, want %q", label, string(data), want)
	}
}

func assertOutputContains(t *testing.T, output []byte, pattern string, label string) {
	t.Helper()
	if !strings.Contains(string(output), pattern) {
		t.Fatalf("%s output = %s, want %q", label, string(output), pattern)
	}
}

func assertOutputContainsArtifacts(t *testing.T, output []byte, artifacts []string, label string) {
	t.Helper()
	for _, artifact := range artifacts {
		assertOutputContains(t, output, `"`+artifact+`"`, label)
	}
}

func integrationBackendArtifacts() []string {
	return []string{"selected-cli.json", "capability-check.md", "bridge-session-snapshot.json", "bridge-events.md"}
}

func integrationProductionWriteBackendArtifacts() []string {
	return append(integrationBackendArtifacts(), "diff.patch", "impl-log.md")
}

func writeIntegrationTarget(t *testing.T, repo string, relative string, content string) {
	t.Helper()
	path := filepath.Join(repo, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write target %s: %v", relative, err)
	}
}

func integrationValidWorkflowGraph() string {
	return `version: "workflow-graph/v1"
graph_id: "graph-integration"
metadata:
  project: "kkachi-integration"
  created_by: "human"
  managed_by: "kah"
phases:
  - id: "plan"
    title: "Plan"
    owner_layer: "khs"
    required: true
    evidence: ["plan.md"]
  - id: "implement"
    title: "Implement"
    owner_layer: "khs"
    required: true
    evidence: ["diff.patch"]
edges:
  - from: "plan"
    to: "implement"
gates:
  - id: "pre-implementation"
    requires: ["plan"]
approvals:
  - scope: "sot-change"
    required_role: "responsible-approver"
proposals:
  policy: "proposal-first"
	`
}

func integrationCandidateWorkflowGraph() string {
	return `version: "workflow-graph/v1"
graph_id: "graph-integration"
metadata:
  project: "kkachi-integration"
  created_by: "human"
  managed_by: "kah"
phases:
  - id: "plan"
    title: "Plan"
    owner_layer: "khs"
    required: true
    evidence: ["plan.md"]
  - id: "ask"
    title: "Ask"
    owner_layer: "khs"
    required: true
    evidence: ["feedback-request.md"]
  - id: "implement"
    title: "Implement"
    owner_layer: "khs"
    required: true
    evidence: ["diff.patch"]
edges:
  - from: "plan"
    to: "ask"
  - from: "ask"
    to: "implement"
gates:
  - id: "pre-implementation"
    requires: ["plan", "ask"]
approvals:
  - scope: "sot-change"
    required_role: "required-reviewer"
proposals:
  policy: "proposal-first"
`
}

func mustHostname(t *testing.T) string {
	t.Helper()
	hostname, err := os.Hostname()
	if err != nil {
		t.Fatalf("hostname: %v", err)
	}
	return hostname
}

func writeIntegrationJSONFile(t *testing.T, path string, payload map[string]any) {
	t.Helper()
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatalf("marshal JSON file %s: %v", path, err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		t.Fatalf("write JSON file %s: %v", path, err)
	}
}

func TestPilot004AdapterQAFinalDiagnosticsSmoke(t *testing.T) {
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	binary := buildHelperBinary(t)

	runHelper(t, binary, repo, "project", "init", "--json")
	createdOutput := runHelper(t, binary, repo,
		"run", "create",
		"--title", "Pilot 004 integration acceptance",
		"--work-path", "A_development_execution",
		"--work-mode", "standard",
		"--urgency", "normal",
		"--sot-policy", "existing_sot_basis",
		"--execution-mode", "adapter_qa",
		"--commander", "Gongmyeong",
		"--redteam", "Haneul",
		"--task-id", "pilot-004",
		"--json",
	)
	var created struct {
		RunID string `json:"run_id"`
	}
	if err := json.Unmarshal(createdOutput, &created); err != nil {
		t.Fatalf("run create output is not JSON: %v\n%s", err, string(createdOutput))
	}
	activateOutput := runHelper(t, binary, repo, "run", "activate", created.RunID, "--json")
	var activated struct {
		RunID string `json:"run_id"`
		State string `json:"state"`
	}
	if err := json.Unmarshal(activateOutput, &activated); err != nil {
		t.Fatalf("run activate output is not JSON: %v\n%s", err, string(activateOutput))
	}
	if activated.RunID != created.RunID || activated.State != "active" {
		t.Fatalf("activated = %#v, want active run %s", activated, created.RunID)
	}
	statusActive := runHelper(t, binary, repo, "project", "status", "--json")
	assertOutputContains(t, statusActive, `"active_run_id":"`+created.RunID+`"`, "pilot-004 active status")
	assertOutputContains(t, statusActive, `"active_run_state":"active"`, "pilot-004 active status")

	runHelper(t, binary, repo, "artifact", "init", created.RunID, "--json")
	writePilot004AcceptanceEvidence(t, repo, created.RunID)

	for _, gate := range []string{"intake", "sot", "roadmap", "plan", "backend", "implementation", "review", "verification", "docs"} {
		output := runHelper(t, binary, repo, "gate", "check", created.RunID, gate, "--json")
		var checked gateCheckOutput
		if err := json.Unmarshal(output, &checked); err != nil {
			t.Fatalf("%s gate output is not JSON: %v\n%s", gate, err, string(output))
		}
		if checked.Status != "pass" || checked.ReportPath == "" {
			t.Fatalf("%s gate = %#v, want pass with report path", gate, checked)
		}
	}

	finalOutput := runHelper(t, binary, repo, "gate", "final", created.RunID, "--json")
	var final gateCheckOutput
	if err := json.Unmarshal(finalOutput, &final); err != nil {
		t.Fatalf("final gate output is not JSON: %v\n%s", err, string(finalOutput))
	}
	if final.Gate != "final" || final.Status != "pass" || final.ReportPath == "" || !gateCheckListed(final.Checks, "backend_gate", "pass") {
		t.Fatalf("final gate = %#v, want pass with backend_gate evidence", final)
	}
	assertOutputContains(t, readFile(t, filepath.Join(repo, final.ReportPath)), `"status": "pass"`, "final gate report")

	diagnosticsOutput := runHelper(t, binary, repo, "diagnostics", "export", "--run", created.RunID, "--json")
	assertOutputContains(t, diagnosticsOutput, `"run_id":"`+created.RunID+`"`, "pilot-004 diagnostics bundle")
	assertOutputContains(t, diagnosticsOutput, `gate-reports/final.json`, "pilot-004 diagnostics bundle")
	assertOutputContains(t, diagnosticsOutput, `selected-cli.json`, "pilot-004 diagnostics bundle")
	assertOutputContains(t, diagnosticsOutput, `bridge-session-snapshot.json`, "pilot-004 diagnostics bundle")
	assertOutputContains(t, diagnosticsOutput, `verification.md`, "pilot-004 diagnostics bundle")
	assertOutputContains(t, diagnosticsOutput, `docs-update.md`, "pilot-004 diagnostics bundle")
	assertOutputContains(t, diagnosticsOutput, `final-report.md`, "pilot-004 diagnostics bundle")

	eventsBeforeClose := readFile(t, filepath.Join(repo, ".kkachi", "events.jsonl"))
	assertOutputContains(t, eventsBeforeClose, `"type":"run.activated"`, "pilot-004 event log")
	assertOutputContains(t, eventsBeforeClose, `"type":"gate.passed"`, "pilot-004 event log")
	assertOutputContains(t, eventsBeforeClose, `"gate":"backend"`, "pilot-004 event log")
	assertOutputContains(t, eventsBeforeClose, `"gate":"final"`, "pilot-004 event log")

	closeOutput := runHelper(t, binary, repo, "run", "close", created.RunID, "--json")
	var closed struct {
		RunID string `json:"run_id"`
		State string `json:"state"`
	}
	if err := json.Unmarshal(closeOutput, &closed); err != nil {
		t.Fatalf("run close output is not JSON: %v\n%s", err, string(closeOutput))
	}
	if closed.RunID != created.RunID || closed.State != "closed" {
		t.Fatalf("closed = %#v, want closed run %s", closed, created.RunID)
	}
	statusClosed := runHelper(t, binary, repo, "project", "status", "--json")
	assertOutputContains(t, statusClosed, `"active_run_id":null`, "pilot-004 closed status")
	assertOutputContains(t, readFile(t, filepath.Join(repo, ".kkachi", "events.jsonl")), `"type":"run.closed"`, "pilot-004 event log")
}

func writePilot004AcceptanceEvidence(t *testing.T, repo string, runID string) {
	t.Helper()
	writeIntegrationIntake(t, repo, runID, "A_development_execution", "standard", "existing_sot_basis", "normal", "Acceptance Evidence: pilot-004 integration smoke\n")
	writeIntegrationMarkdownArtifact(t, repo, runID, "sot-basis.md", "Status: complete\nSource: docs/specs.md\n")
	writeIntegrationMarkdownArtifact(t, repo, runID, "roadmap-update.md", "Status: complete\nTrace: docs/roadmap.md pilot-004\n")
	writeIntegrationMarkdownArtifact(t, repo, runID, "acceptance-criteria.md", "Status: complete\nCriteria: adapter QA final gate and diagnostics smoke pass\n")
	writeIntegrationMarkdownArtifact(t, repo, runID, "plan.md", "Status: complete\nPlan: pass required gates before final\n")
	writeIntegrationMarkdownArtifact(t, repo, runID, "checklist.md", "Status: complete\n- [x] backend gate\n- [x] final gate\n- [x] diagnostics evidence\n")
	writeIntegrationBackendEvidence(t, repo, runID)
	writeIntegrationMarkdownArtifact(t, repo, runID, "cli-output.md", "Status: complete\nOutput: adapter QA command output captured\n")
	writeIntegrationTarget(t, repo, ".kkachi/runs/"+runID+"/diff.patch", "diff --git a/.kkachi/evidence b/.kkachi/evidence\n+pilot-004 integration acceptance evidence\n")
	writeIntegrationMarkdownArtifact(t, repo, runID, "impl-log.md", "Status: complete\nImplementation: integration smoke evidence recorded\n")
	writeIntegrationMarkdownArtifact(t, repo, runID, "review.md", "Status: complete\nReview: no blockers\n")
	writeIntegrationMarkdownArtifact(t, repo, runID, "redteam/plan-review.md", "Status: complete\nReview: plan accepted\n")
	writeIntegrationMarkdownArtifact(t, repo, runID, "redteam/shaping-review.md", "Status: complete\nReview: shaping accepted\n")
	writeIntegrationMarkdownArtifact(t, repo, runID, "redteam/qa-review.md", "Status: complete\nReview: adapter QA evidence accepted\n")
	writeIntegrationMarkdownArtifact(t, repo, runID, "redteam/final-gate-review.md", "Status: complete\nReview: final gate ready\n")
	writeIntegrationMarkdownArtifact(t, repo, runID, "test-log.md", "Status: complete\nTests: pilot-004 integration smoke\n")
	writeIntegrationMarkdownArtifact(t, repo, runID, "verification.md", "Status: complete\nVerdict: pass\n")
	writeIntegrationMarkdownArtifact(t, repo, runID, "docs-update.md", "Status: complete\nNo Change Reason: integration smoke only\n")
	writeIntegrationMarkdownArtifact(t, repo, runID, "final-report.md", "Status: complete\nReport: adapter QA final gate and diagnostics evidence preserved\n")
}

func TestPilot002DiagnosticsExportActiveRunAndOverwriteSafety(t *testing.T) {
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	binary := buildHelperBinary(t)
	runHelper(t, binary, repo, "project", "init", "--json")
	createdOutput := runHelper(t, binary, repo,
		"run", "create",
		"--title", "Pilot 002 integration diagnostics",
		"--work-path", "A_development_execution",
		"--work-mode", "standard",
		"--urgency", "normal",
		"--sot-policy", "existing_sot_basis",
		"--execution-mode", "adapter_qa",
		"--commander", "Gongmyeong",
		"--task-id", "pilot-002",
		"--json",
	)
	var created struct {
		RunID string `json:"run_id"`
	}
	if err := json.Unmarshal(createdOutput, &created); err != nil {
		t.Fatalf("run create output is not JSON: %v\n%s", err, string(createdOutput))
	}
	runHelper(t, binary, repo, "artifact", "init", created.RunID, "--json")
	runHelper(t, binary, repo, "run", "activate", created.RunID, "--json")
	secret := "sk-abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ123456"
	writeIntegrationTarget(t, repo, ".kkachi/runs/"+created.RunID+"/selected-cli.json", `{"version":"0.1","api_token":"`+secret+`"}`+"\n")
	writeIntegrationTarget(t, repo, ".kkachi/runs/"+created.RunID+"/verification.md", "Status: complete\nAuthorization: Bearer "+secret+"\n")

	bundleOutput := runHelper(t, binary, repo, "diagnostics", "export", "--json")
	assertOutputContains(t, bundleOutput, `"run_id":"`+created.RunID+`"`, "active diagnostics bundle")
	assertOutputContains(t, bundleOutput, `"schema_versions":`, "active diagnostics bundle")
	assertOutputContains(t, bundleOutput, `"selected_artifacts":`, "active diagnostics bundle")
	assertOutputContains(t, bundleOutput, `"api_token":"[REDACTED]"`, "active diagnostics bundle")
	if strings.Contains(string(bundleOutput), secret) {
		t.Fatalf("diagnostics bundle leaked secret: %s", string(bundleOutput))
	}

	runHelper(t, binary, repo, "diagnostics", "export", "--output", "diagnostics/pilot-002.json")
	written := readFile(t, filepath.Join(repo, "diagnostics", "pilot-002.json"))
	assertOutputContains(t, written, `"run_id": "`+created.RunID+`"`, "written diagnostics bundle")
	assertOutputContains(t, written, `"api_token": "[REDACTED]"`, "written diagnostics bundle")
	if strings.Contains(string(written), secret) {
		t.Fatalf("written diagnostics bundle leaked secret: %s", string(written))
	}

	output, err := runHelperAllowError(binary, repo, "diagnostics", "export", "--output", "diagnostics/pilot-002.json", "--json")
	if err == nil {
		t.Fatalf("diagnostics overwrite unexpectedly succeeded\n%s", string(output))
	}
	assertOutputContains(t, output, `"code":"diagnostics_output_exists"`, "diagnostics overwrite refusal")
	if got := readFile(t, filepath.Join(repo, "diagnostics", "pilot-002.json")); string(got) != string(written) {
		t.Fatalf("diagnostics overwrite changed file\nbefore=%s\nafter=%s", string(written), string(got))
	}
}
