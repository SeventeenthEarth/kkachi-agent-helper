package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkflowPhaseProjectionFailsClosedForRunningCompletedPhase(t *testing.T) {
	repo, root, runID := workflowProjectionTestRun(t, `schema_version: task-dag/v1
workflow_id: projection-demo
nodes:
  - id: plan
    depends_on: []
    join: all_of
    required_outputs: ["plan.md", "checklist.md"]
`)
	if _, err := StartWorkflowNode(root, WorkflowNodeOptions{RunID: runID, NodeID: "plan", ExpectedRevision: intPtr(1), Now: testRunNow(5)}); err != nil {
		t.Fatalf("start plan: %v", err)
	}
	writeWorkflowProjectionPhasePlan(t, repo, runID, `version: "0.1"
run_id: "`+runID+`"
phases:
  - id: "plan"
    status: "complete"
    evidence: "plan.md"
`)

	result, err := ValidatePhasePlan(root, PhasePlanValidationOptions{RunID: runID})
	if err != nil {
		t.Fatalf("ValidatePhasePlan() error = %v", err)
	}
	if result.Status != PhasePlanStatusFail || !phaseCheckActualContains(result.Checks, workflowPhaseProjectionCheckName, "strict_workflow_phase_projection_node_not_succeeded") {
		t.Fatalf("checks = %#v, want running workflow node projection failure", result.Checks)
	}
}

func TestWorkflowPhaseProjectionRequiresEvidenceBoundToNodeOutputs(t *testing.T) {
	repo, root, runID := workflowProjectionTestRun(t, `schema_version: task-dag/v1
workflow_id: projection-demo
nodes:
  - id: plan
    depends_on: []
    join: all_of
    required_outputs: ["plan.md", "checklist.md"]
`)
	if _, err := StartWorkflowNode(root, WorkflowNodeOptions{RunID: runID, NodeID: "plan", ExpectedRevision: intPtr(1), Now: testRunNow(5)}); err != nil {
		t.Fatalf("start plan: %v", err)
	}
	mustWriteText(t, filepath.Join(repo, "plan.md"), "plan\n")
	mustWriteText(t, filepath.Join(repo, "checklist.md"), "checklist\n")
	if _, err := CompleteWorkflowNode(root, WorkflowNodeOptions{RunID: runID, NodeID: "plan", Evidence: "plan.md", ExpectedRevision: intPtr(2), Now: testRunNow(6)}); err != nil {
		t.Fatalf("complete plan: %v", err)
	}
	mustWriteText(t, filepath.Join(repo, "other.md"), "not bound\n")
	writeWorkflowProjectionPhasePlan(t, repo, runID, `version: "0.1"
run_id: "`+runID+`"
phases:
  - id: "plan"
    status: "complete"
    evidence: "other.md"
`)

	result, err := ValidatePhasePlan(root, PhasePlanValidationOptions{RunID: runID})
	if err != nil {
		t.Fatalf("ValidatePhasePlan() error = %v", err)
	}
	if result.Status != PhasePlanStatusFail || !phaseCheckActualContains(result.Checks, workflowPhaseProjectionCheckName, "strict_workflow_phase_projection_evidence_unbound") {
		t.Fatalf("checks = %#v, want evidence binding failure", result.Checks)
	}

	writeWorkflowProjectionPhasePlan(t, repo, runID, `version: "0.1"
run_id: "`+runID+`"
phases:
  - id: "plan"
    status: "complete"
    evidence: "checklist.md"
`)
	result, err = ValidatePhasePlan(root, PhasePlanValidationOptions{RunID: runID})
	if err != nil {
		t.Fatalf("ValidatePhasePlan(bound output) error = %v", err)
	}
	if phaseCheckFailed(result.Checks, workflowPhaseProjectionCheckName) {
		t.Fatalf("checks = %#v, did not expect projection failure for required output evidence", result.Checks)
	}
}

func TestWorkflowPhaseProjectionRejectsCompletedOmittedWorkflowPhase(t *testing.T) {
	repo, root, runID := workflowProjectionTestRun(t, `schema_version: task-dag/v1
workflow_id: light
nodes:
  - id: plan
    depends_on: []
    join: all_of
    required_outputs: ["plan.md"]
`)
	writeWorkflowProjectionPhasePlan(t, repo, runID, `version: "0.1"
run_id: "`+runID+`"
phases:
  - id: "implement"
    status: "complete"
    evidence: "diff.patch"
`)

	result, err := ValidatePhasePlan(root, PhasePlanValidationOptions{RunID: runID, Final: true})
	if err != nil {
		t.Fatalf("ValidatePhasePlan() error = %v", err)
	}
	if result.Status != PhasePlanStatusFail || !phaseCheckActualContains(result.Checks, workflowPhaseProjectionCheckName, "strict_workflow_phase_projection_node_omitted") {
		t.Fatalf("checks = %#v, want omitted phase complete failure", result.Checks)
	}

	writeWorkflowProjectionPhasePlan(t, repo, runID, `version: "0.1"
run_id: "`+runID+`"
phases:
  - id: "implement"
    status: "not_applicable"
    reason: "Light workflow omits implementation."
`)
	result, err = ValidatePhasePlan(root, PhasePlanValidationOptions{RunID: runID})
	if err != nil {
		t.Fatalf("ValidatePhasePlan(not_applicable) error = %v", err)
	}
	if phaseCheckFailed(result.Checks, workflowPhaseProjectionCheckName) {
		t.Fatalf("checks = %#v, did not expect projection failure for reasoned omitted phase", result.Checks)
	}
}

func TestWorkflowPhaseProjectionRejectsDuplicateCompletedPhaseNode(t *testing.T) {
	repo, root, runID := workflowProjectionTestRun(t, `schema_version: task-dag/v1
workflow_id: projection-demo
nodes:
  - id: final_verify
    depends_on: []
    join: all_of
    required_outputs: ["final-report.md"]
`)
	if _, err := StartWorkflowNode(root, WorkflowNodeOptions{RunID: runID, NodeID: "final_verify", ExpectedRevision: intPtr(1), Now: testRunNow(5)}); err != nil {
		t.Fatalf("start final_verify: %v", err)
	}
	mustWriteText(t, filepath.Join(repo, "final-report.md"), "Status: complete\n")
	if _, err := CompleteWorkflowNode(root, WorkflowNodeOptions{RunID: runID, NodeID: "final_verify", Evidence: "final-report.md", ExpectedRevision: intPtr(2), Now: testRunNow(6)}); err != nil {
		t.Fatalf("complete final_verify: %v", err)
	}
	writeWorkflowProjectionPhasePlan(t, repo, runID, `version: "0.1"
run_id: "`+runID+`"
phases:
  - id: "verify"
    status: "complete"
    evidence: "final-report.md"
  - id: "final"
    status: "complete"
    evidence: "final-report.md"
`)

	result, err := ValidatePhasePlan(root, PhasePlanValidationOptions{RunID: runID})
	if err != nil {
		t.Fatalf("ValidatePhasePlan() error = %v", err)
	}
	if result.Status != PhasePlanStatusFail || !phaseCheckActualContains(result.Checks, workflowPhaseProjectionCheckName, "strict_workflow_phase_projection_node_duplicate") {
		t.Fatalf("checks = %#v, want duplicate workflow node projection failure", result.Checks)
	}
}

func TestWorkflowPhaseProjectionRequiresValidTransitionLedger(t *testing.T) {
	repo, root, runID := workflowProjectionTestRun(t, `schema_version: task-dag/v1
workflow_id: projection-demo
nodes:
  - id: plan
    depends_on: []
    join: all_of
    required_outputs: ["plan.md"]
`)
	mustWriteText(t, filepath.Join(repo, "plan.md"), "plan\n")
	instancePath, err := workflowInstancePath(root, runID)
	if err != nil {
		t.Fatalf("workflowInstancePath() error = %v", err)
	}
	instance, err := readWorkflowInstance(instancePath)
	if err != nil {
		t.Fatalf("readWorkflowInstance() error = %v", err)
	}
	instance.Revision = 2
	instance.Nodes[0].State = WorkflowNodeSucceeded
	instance.Nodes[0].Evidence = []string{"plan.md"}
	writeWorkflowInstanceForTest(t, repo, runID, instance)
	writeWorkflowProjectionPhasePlan(t, repo, runID, `version: "0.1"
run_id: "`+runID+`"
phases:
  - id: "plan"
    status: "complete"
    evidence: "plan.md"
`)

	result, err := ValidatePhasePlan(root, PhasePlanValidationOptions{RunID: runID})
	if err != nil {
		t.Fatalf("ValidatePhasePlan() error = %v", err)
	}
	if result.Status != PhasePlanStatusFail || !phaseCheckActualContains(result.Checks, workflowPhaseProjectionCheckName, "strict_workflow_phase_projection_ledger_invalid") {
		t.Fatalf("checks = %#v, want transition-ledger projection failure", result.Checks)
	}
}

func TestFinalGateIncludesWorkflowPhaseProjection(t *testing.T) {
	repo, root, runID := workflowProjectionTestRun(t, `schema_version: task-dag/v1
workflow_id: projection-demo
nodes:
  - id: plan
    depends_on: []
    join: all_of
    required_outputs: ["plan.md"]
`)
	if _, err := StartWorkflowNode(root, WorkflowNodeOptions{RunID: runID, NodeID: "plan", ExpectedRevision: intPtr(1), Now: testRunNow(5)}); err != nil {
		t.Fatalf("start plan: %v", err)
	}
	writeMarkdownArtifact(t, repo, runID, "final-report.md", "Status: complete\n")
	writeWorkflowProjectionPhasePlan(t, repo, runID, `version: "0.1"
run_id: "`+runID+`"
phases:
  - id: "plan"
    status: "complete"
    evidence: "plan.md"
`)

	result, err := CheckGate(root, GateCheckOptions{RunID: runID, Gate: GateFinal, Now: testRunNow(6)})
	if err != nil {
		t.Fatalf("CheckGate(final) error = %v", err)
	}
	if result.Status != GateStatusFail || !gateCheckActualContains(result.Checks, workflowPhaseProjectionCheckName, "strict_workflow_phase_projection_node_not_succeeded") {
		t.Fatalf("checks = %#v, want final gate projection failure", result.Checks)
	}
}

func workflowProjectionTestRun(t *testing.T, workflowYAML string) (string, Root, string) {
	t.Helper()
	repo, root, runID := workflowInstanceTestRun(t)
	writeWorkflowGraph(t, repo, workflowGraphWithFeedbackIntake(validWorkflowGraph()))
	writeWorkflowFixture(t, repo, workflowYAML)
	if _, err := InitPhasePlan(root, PhasePlanInitOptions{RunID: runID, Now: testRunNow(4)}); err != nil {
		t.Fatalf("InitPhasePlan() error = %v", err)
	}
	metadata := readRunMetadata(t, repo, runID)
	metadata.WorkflowManaged = true
	metadata.StrictWorkflowOrder = true
	metadata.SelectedWorkflowID = optionalTrimmedString(workflowIDFromYAML(workflowYAML))
	metadata.WorkflowSource = optionalTrimmedString("workflow.yaml")
	writeRunMetadataForTest(t, repo, metadata)
	if _, err := CreateWorkflowInstance(root, WorkflowCreateOptions{RunID: runID, File: "workflow.yaml", Now: testRunNow(4)}); err != nil {
		t.Fatalf("CreateWorkflowInstance() error = %v", err)
	}
	return repo, root, runID
}

func writeWorkflowProjectionPhasePlan(t *testing.T, repo string, runID string, content string) {
	t.Helper()
	path := filepath.Join(repo, ".kkachi", "runs", runID, "phase-plan.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write phase-plan: %v", err)
	}
}

func workflowIDFromYAML(content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "workflow_id:") {
			return strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, "workflow_id:")), `"`)
		}
	}
	return ""
}

func phaseCheckActualContains(checks []PhasePlanCheck, name string, token string) bool {
	for _, check := range checks {
		if check.Name == name && strings.Contains(check.Actual, token) {
			return true
		}
	}
	return false
}

func gateCheckActualContains(checks []GateCheck, name string, token string) bool {
	for _, check := range checks {
		if check.Name == name && strings.Contains(check.Actual, token) {
			return true
		}
	}
	return false
}
