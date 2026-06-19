package project

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkflowTransitionOrderVerificationPassesForValidDAG(t *testing.T) {
	repo, root, runID := workflowInstanceTestRun(t)
	createCompletedStrictLedgerWorkflow(t, repo, root, runID)

	result, err := CheckWorkflowTransitionOrder(root, runID)
	if err != nil {
		t.Fatalf("CheckWorkflowTransitionOrder() error = %v", err)
	}
	if !result.OK || result.Reason != "workflow_transition_order_valid" || result.WorkflowID != "strict-ledger" || result.Revision != 5 {
		t.Fatalf("result = %#v, want valid transition order", result)
	}
}

func TestWorkflowTransitionOrderVerificationPassesForNewPendingWorkflow(t *testing.T) {
	repo, root, runID := workflowInstanceTestRun(t)
	writeWorkflowFixture(t, repo, strictLedgerWorkflowYAML())
	if _, err := CreateWorkflowInstance(root, WorkflowCreateOptions{RunID: runID, File: "workflow.yaml", Now: testRunNow(4)}); err != nil {
		t.Fatalf("create workflow instance: %v", err)
	}

	result, err := CheckWorkflowTransitionOrder(root, runID)
	if err != nil {
		t.Fatalf("CheckWorkflowTransitionOrder() error = %v", err)
	}
	if !result.OK || result.Reason != "workflow_transition_order_valid" || len(result.Diagnostics) != 0 {
		t.Fatalf("result = %#v, want new pending workflow to have valid transition order", result)
	}
}

func TestWorkflowTransitionOrderVerificationRejectsInvalidWorkflowInstance(t *testing.T) {
	repo, root, runID := workflowInstanceTestRun(t)
	writeWorkflowFixture(t, repo, strictLedgerWorkflowYAML())
	if _, err := CreateWorkflowInstance(root, WorkflowCreateOptions{RunID: runID, File: "workflow.yaml", Now: testRunNow(4)}); err != nil {
		t.Fatalf("create workflow instance: %v", err)
	}
	instancePath := filepath.Join(repo, ".kkachi", "runs", runID, "workflow-instance.json")
	if err := os.WriteFile(instancePath, []byte("{not-json"), 0o600); err != nil {
		t.Fatalf("write invalid workflow instance: %v", err)
	}

	assertWorkflowTransitionOrderReason(t, root, runID, "workflow_transition_instance_invalid")
}

func TestWorkflowTransitionOrderVerificationRejectsInstanceRunMismatch(t *testing.T) {
	repo, root, runID := workflowInstanceTestRun(t)
	writeWorkflowFixture(t, repo, strictLedgerWorkflowYAML())
	if _, err := CreateWorkflowInstance(root, WorkflowCreateOptions{RunID: runID, File: "workflow.yaml", Now: testRunNow(4)}); err != nil {
		t.Fatalf("create workflow instance: %v", err)
	}
	instance := readWorkflowInstanceForTest(t, repo, runID)
	instance.RunID = "run-20260619T000000Z-aaaaaaaaaaaa"
	writeWorkflowInstanceForTest(t, repo, runID, instance)

	assertWorkflowTransitionOrderReason(t, root, runID, "workflow_transition_instance_run_mismatch")
	completeness, err := CheckWorkflowInstanceCompleteness(root, runID)
	if err != nil {
		t.Fatalf("CheckWorkflowInstanceCompleteness() error = %v", err)
	}
	if completeness.OK || completeness.Reason != "workflow_instance_run_mismatch" {
		t.Fatalf("completeness = %#v, want workflow_instance_run_mismatch", completeness)
	}
}

func TestWorkflowTransitionOrderVerificationRejectsStartBeforeDependenciesSucceeded(t *testing.T) {
	repo, root, runID := workflowInstanceTestRun(t)
	writeWorkflowFixture(t, repo, strictLedgerWorkflowYAML())
	if _, err := CreateWorkflowInstance(root, WorkflowCreateOptions{RunID: runID, File: "workflow.yaml", Now: testRunNow(4)}); err != nil {
		t.Fatalf("create workflow instance: %v", err)
	}
	eventID := appendManualWorkflowTransitionEvent(t, root, runID, "workflow.node.started", map[string]any{
		"run_id":             runID,
		"workflow_id":        "strict-ledger",
		"node_id":            "build",
		"transition_kind":    "start",
		"previous_revision":  1,
		"resulting_revision": 2,
		"previous_state":     WorkflowNodePending,
		"resulting_state":    WorkflowNodeRunning,
		"dependency_states":  map[string]string{"setup": WorkflowNodePending},
		"ready_before":       []string{"setup"},
		"ready_after":        []string{},
		"expected_revision":  1,
	})
	instance := readWorkflowInstanceForTest(t, repo, runID)
	instance.Revision = 2
	instance.UpdatedEventID = eventID
	instance.Nodes[1].State = WorkflowNodeRunning
	instance.Nodes[1].LastTransitionEventID = eventID
	writeWorkflowInstanceForTest(t, repo, runID, instance)

	assertWorkflowTransitionOrderReason(t, root, runID, "workflow_transition_start_before_dependencies")
}

func TestWorkflowTransitionOrderVerificationRejectsCompleteWithoutStart(t *testing.T) {
	repo, root, runID := workflowInstanceTestRun(t)
	writeWorkflowFixture(t, repo, strictLedgerWorkflowYAML())
	if _, err := CreateWorkflowInstance(root, WorkflowCreateOptions{RunID: runID, File: "workflow.yaml", Now: testRunNow(4)}); err != nil {
		t.Fatalf("create workflow instance: %v", err)
	}
	eventID := appendManualWorkflowTransitionEvent(t, root, runID, "workflow.node.completed", strictTransitionPayload(runID, "setup", "complete", 1, 2, WorkflowNodePending, WorkflowNodeSucceeded))
	instance := readWorkflowInstanceForTest(t, repo, runID)
	instance.Revision = 2
	instance.UpdatedEventID = eventID
	instance.Nodes[0].State = WorkflowNodeSucceeded
	instance.Nodes[0].LastTransitionEventID = eventID
	writeWorkflowInstanceForTest(t, repo, runID, instance)

	assertWorkflowTransitionOrderReason(t, root, runID, "workflow_transition_complete_without_start")
}

func TestWorkflowTransitionOrderVerificationRejectsUnknownNodeTransition(t *testing.T) {
	repo, root, runID := workflowInstanceTestRun(t)
	writeWorkflowFixture(t, repo, strictLedgerWorkflowYAML())
	if _, err := CreateWorkflowInstance(root, WorkflowCreateOptions{RunID: runID, File: "workflow.yaml", Now: testRunNow(4)}); err != nil {
		t.Fatalf("create workflow instance: %v", err)
	}
	appendManualWorkflowTransitionEvent(t, root, runID, "workflow.node.started", strictTransitionPayload(runID, "missing", "start", 1, 2, WorkflowNodePending, WorkflowNodeRunning))

	assertWorkflowTransitionOrderReason(t, root, runID, "workflow_transition_node_unknown")
}

func TestWorkflowTransitionOrderVerificationRejectsWorkflowMismatch(t *testing.T) {
	repo, root, runID := workflowInstanceTestRun(t)
	writeWorkflowFixture(t, repo, strictLedgerWorkflowYAML())
	if _, err := CreateWorkflowInstance(root, WorkflowCreateOptions{RunID: runID, File: "workflow.yaml", Now: testRunNow(4)}); err != nil {
		t.Fatalf("create workflow instance: %v", err)
	}
	appendManualWorkflowTransitionEvent(t, root, runID, "workflow.node.started", mapWithOverride(strictTransitionPayload(runID, "setup", "start", 1, 2, WorkflowNodePending, WorkflowNodeRunning), "workflow_id", "other"))

	assertWorkflowTransitionOrderReason(t, root, runID, "workflow_transition_workflow_mismatch")
}

func TestWorkflowTransitionOrderVerificationRejectsStaleRevisionTransition(t *testing.T) {
	repo, root, runID := workflowInstanceTestRun(t)
	writeWorkflowFixture(t, repo, strictLedgerWorkflowYAML())
	if _, err := CreateWorkflowInstance(root, WorkflowCreateOptions{RunID: runID, File: "workflow.yaml", Now: testRunNow(4)}); err != nil {
		t.Fatalf("create workflow instance: %v", err)
	}
	appendManualWorkflowTransitionEvent(t, root, runID, "workflow.node.started", strictTransitionPayload(runID, "setup", "start", 0, 1, WorkflowNodePending, WorkflowNodeRunning))

	assertWorkflowTransitionOrderReason(t, root, runID, "workflow_transition_revision_stale")
}

func TestWorkflowTransitionOrderVerificationRejectsRevisionGap(t *testing.T) {
	repo, root, runID := workflowInstanceTestRun(t)
	writeWorkflowFixture(t, repo, strictLedgerWorkflowYAML())
	if _, err := CreateWorkflowInstance(root, WorkflowCreateOptions{RunID: runID, File: "workflow.yaml", Now: testRunNow(4)}); err != nil {
		t.Fatalf("create workflow instance: %v", err)
	}
	appendManualWorkflowTransitionEvent(t, root, runID, "workflow.node.started", strictTransitionPayload(runID, "setup", "start", 1, 3, WorkflowNodePending, WorkflowNodeRunning))

	assertWorkflowTransitionOrderReason(t, root, runID, "workflow_transition_revision_gap")
}

func TestWorkflowTransitionOrderVerificationRejectsSucceededNodeRestart(t *testing.T) {
	repo, root, runID := workflowInstanceTestRun(t)
	completeSetupForTransitionOrderTest(t, repo, root, runID)
	eventID := appendManualWorkflowTransitionEvent(t, root, runID, "workflow.node.started", strictTransitionPayload(runID, "setup", "start", 3, 4, WorkflowNodeSucceeded, WorkflowNodeRunning))
	instance := readWorkflowInstanceForTest(t, repo, runID)
	instance.Revision = 4
	instance.UpdatedEventID = eventID
	instance.Nodes[0].State = WorkflowNodeRunning
	instance.Nodes[0].LastTransitionEventID = eventID
	writeWorkflowInstanceForTest(t, repo, runID, instance)

	assertWorkflowTransitionOrderReason(t, root, runID, "workflow_transition_succeeded_node_restarted")
}

func TestWorkflowTransitionOrderVerificationRejectsMalformedPayload(t *testing.T) {
	repo, root, runID := workflowInstanceTestRun(t)
	writeWorkflowFixture(t, repo, strictLedgerWorkflowYAML())
	if _, err := CreateWorkflowInstance(root, WorkflowCreateOptions{RunID: runID, File: "workflow.yaml", Now: testRunNow(4)}); err != nil {
		t.Fatalf("create workflow instance: %v", err)
	}
	appendManualWorkflowTransitionEvent(t, root, runID, "workflow.node.started", map[string]any{"run_id": runID, "workflow_id": "strict-ledger", "node_id": "setup"})

	assertWorkflowTransitionOrderReason(t, root, runID, "workflow_transition_payload_malformed")
}

func TestWorkflowTransitionOrderVerificationRejectsInstanceEventMismatch(t *testing.T) {
	repo, root, runID := workflowInstanceTestRun(t)
	createCompletedStrictLedgerWorkflow(t, repo, root, runID)
	instance := readWorkflowInstanceForTest(t, repo, runID)
	instance.Nodes[1].LastTransitionEventID = "evt-999999"
	writeWorkflowInstanceForTest(t, repo, runID, instance)

	assertWorkflowTransitionOrderReason(t, root, runID, "workflow_transition_instance_event_mismatch")
}

func TestWorkflowManagedFinalGateIncludesTransitionOrderVerification(t *testing.T) {
	repo, root, runID := workflowInstanceTestRun(t)
	createCompletedStrictLedgerWorkflow(t, repo, root, runID)
	instance := readWorkflowInstanceForTest(t, repo, runID)
	instance.Nodes[1].LastTransitionEventID = "evt-999999"
	writeWorkflowInstanceForTest(t, repo, runID, instance)
	metadata := readRunMetadata(t, repo, runID)
	metadata.WorkflowManaged = true
	metadata.StrictWorkflowOrder = true
	metadata.SelectedWorkflowID = stringPtr("strict-ledger")
	metadata.WorkflowSource = stringPtr("workflow.yaml")

	result, err := checkFinalGate(root, metadata, "")
	if err != nil {
		t.Fatalf("checkFinalGate() error = %v", err)
	}
	if !gateCheckStatus(result.Checks, "workflow_transition_order", GateStatusFail) || !gateCheckActual(result.Checks, "workflow_transition_order", "workflow_transition_instance_event_mismatch") {
		t.Fatalf("checks = %#v, want workflow_transition_order failure", result.Checks)
	}
}

func TestWorkflowManagedFinalGatePendingWorkflowFailsWorkflowInstanceOnly(t *testing.T) {
	repo, root, runID := workflowInstanceTestRun(t)
	writeWorkflowFixture(t, repo, strictLedgerWorkflowYAML())
	if _, err := CreateWorkflowInstance(root, WorkflowCreateOptions{RunID: runID, File: "workflow.yaml", Now: testRunNow(4)}); err != nil {
		t.Fatalf("create workflow instance: %v", err)
	}
	metadata := readRunMetadata(t, repo, runID)
	metadata.WorkflowManaged = true
	metadata.StrictWorkflowOrder = true
	metadata.SelectedWorkflowID = stringPtr("strict-ledger")
	metadata.WorkflowSource = stringPtr("workflow.yaml")

	result, err := checkFinalGate(root, metadata, "")
	if err != nil {
		t.Fatalf("checkFinalGate() error = %v", err)
	}
	if !gateCheckStatus(result.Checks, "workflow_transition_order", GateStatusPass) {
		t.Fatalf("checks = %#v, want workflow_transition_order pass", result.Checks)
	}
	if !gateCheckStatus(result.Checks, "workflow_instance", GateStatusFail) || !gateCheckActual(result.Checks, "workflow_instance", "workflow_node_incomplete") {
		t.Fatalf("checks = %#v, want workflow_instance incomplete failure", result.Checks)
	}
}

func TestWorkflowTransitionDiagnosticsAreBoundedAndStructured(t *testing.T) {
	repo, root, runID := workflowInstanceTestRun(t)
	writeWorkflowFixture(t, repo, strictLedgerWorkflowYAML())
	if _, err := CreateWorkflowInstance(root, WorkflowCreateOptions{RunID: runID, File: "workflow.yaml", Now: testRunNow(4)}); err != nil {
		t.Fatalf("create workflow instance: %v", err)
	}
	for i := 0; i < 20; i++ {
		appendManualWorkflowTransitionEvent(t, root, runID, "workflow.node.started", map[string]any{"run_id": runID, "workflow_id": "strict-ledger", "node_id": "missing"})
	}

	result, err := CheckWorkflowTransitionOrder(root, runID)
	if err != nil {
		t.Fatalf("CheckWorkflowTransitionOrder() error = %v", err)
	}
	if result.OK || result.Reason != "workflow_transition_node_unknown" || len(result.Diagnostics) == 0 || len(result.Diagnostics) > 10 {
		t.Fatalf("result = %#v, want bounded unknown-node diagnostics", result)
	}
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Code == "" || diagnostic.Message == "" || diagnostic.Field == "" || diagnostic.Path == "" {
			t.Fatalf("diagnostic = %#v, want structured diagnostic", diagnostic)
		}
	}

	bundle, err := ExportDiagnostics(root, DiagnosticsExportOptions{RunID: runID, Now: testRunNow(25)})
	if err != nil {
		t.Fatalf("ExportDiagnostics() error = %v", err)
	}
	if bundle.WorkflowTransitionOrder == nil || bundle.WorkflowTransitionOrder.Reason != result.Reason {
		t.Fatalf("bundle workflow_transition_order = %#v, want reason %s", bundle.WorkflowTransitionOrder, result.Reason)
	}
}

func createCompletedStrictLedgerWorkflow(t *testing.T, repo string, root Root, runID string) {
	t.Helper()
	completeSetupForTransitionOrderTest(t, repo, root, runID)
	if _, err := StartWorkflowNode(root, WorkflowNodeOptions{RunID: runID, NodeID: "build", ExpectedRevision: intPtr(3), Now: testRunNow(7)}); err != nil {
		t.Fatalf("start build: %v", err)
	}
	mustWriteText(t, filepath.Join(repo, "out", "build.txt"), "done\n")
	if _, err := CompleteWorkflowNode(root, WorkflowNodeOptions{RunID: runID, NodeID: "build", Evidence: "out/build.txt", ExpectedRevision: intPtr(4), Now: testRunNow(8)}); err != nil {
		t.Fatalf("complete build: %v", err)
	}
}

func completeSetupForTransitionOrderTest(t *testing.T, repo string, root Root, runID string) {
	t.Helper()
	writeWorkflowFixture(t, repo, strictLedgerWorkflowYAML())
	if _, err := CreateWorkflowInstance(root, WorkflowCreateOptions{RunID: runID, File: "workflow.yaml", Now: testRunNow(4)}); err != nil {
		t.Fatalf("create workflow instance: %v", err)
	}
	completeSetupForStrictLedgerTest(t, repo, root, runID)
}

func assertWorkflowTransitionOrderReason(t *testing.T, root Root, runID string, reason string) {
	t.Helper()
	result, err := CheckWorkflowTransitionOrder(root, runID)
	if err != nil {
		t.Fatalf("CheckWorkflowTransitionOrder() error = %v", err)
	}
	if result.OK || result.Reason != reason {
		t.Fatalf("result = %#v, want reason %s", result, reason)
	}
}

func strictTransitionPayload(runID string, nodeID string, kind string, previousRevision int, resultingRevision int, previousState string, resultingState string) map[string]any {
	return map[string]any{
		"run_id":             runID,
		"workflow_id":        "strict-ledger",
		"node_id":            nodeID,
		"transition_kind":    kind,
		"previous_revision":  previousRevision,
		"resulting_revision": resultingRevision,
		"previous_state":     previousState,
		"resulting_state":    resultingState,
		"dependency_states":  map[string]string{},
		"ready_before":       []string{},
		"ready_after":        []string{},
		"expected_revision":  previousRevision,
	}
}

func mapWithOverride(values map[string]any, key string, value any) map[string]any {
	copied := map[string]any{}
	for k, v := range values {
		copied[k] = v
	}
	copied[key] = value
	return copied
}

func appendManualWorkflowTransitionEvent(t *testing.T, root Root, runID string, eventType string, payload map[string]any) string {
	t.Helper()
	result, err := AppendEvent(root, AppendEventOptions{RunID: runID, Type: eventType, Payload: payload, Now: testRunNow(10)})
	if err != nil {
		t.Fatalf("append manual transition event: %v", err)
	}
	return result.EventID
}

func writeWorkflowInstanceForTest(t *testing.T, repo string, runID string, instance WorkflowInstance) {
	t.Helper()
	data, err := json.MarshalIndent(instance, "", "  ")
	if err != nil {
		t.Fatalf("encode workflow instance: %v", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(filepath.Join(repo, ".kkachi", "runs", runID, WorkflowInstanceFile), data, 0o600); err != nil {
		t.Fatalf("write workflow instance: %v", err)
	}
}

func stringPtr(value string) *string {
	value = strings.TrimSpace(value)
	return &value
}
