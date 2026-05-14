package project

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestApprovalRequestRecordShowAndFinalValidation(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	created, err := CreateRun(root, deterministicCreateRunOptions())
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	runID := created.Metadata.RunID
	if _, err := InitPhasePlan(root, PhasePlanInitOptions{RunID: runID, Now: testRunNow(4)}); err != nil {
		t.Fatalf("InitPhasePlan() error = %v", err)
	}
	if _, err := SetPhasePlanPhase(root, PhasePlanSetOptions{RunID: runID, PhaseID: "implement", Status: PhaseStatusComplete, Evidence: "diff.patch", ApprovalRequiredSet: true, ApprovalRequired: true, Now: testRunNow(5)}); err != nil {
		t.Fatalf("SetPhasePlanPhase() error = %v", err)
	}

	requested, err := RequestApproval(root, ApprovalRequestOptions{RunID: runID, Phase: "implement", Reason: "High-risk production write.", Evidence: "plan.md#risk", Now: testRunNow(6)})
	if err != nil {
		t.Fatalf("RequestApproval() error = %v", err)
	}
	if requested.EventID != "evt-000005" || requested.Record.Type != ApprovalEventRequested || requested.Record.Timestamp == "" {
		t.Fatalf("requested = %#v, want request event", requested)
	}

	failed, err := ValidatePhasePlan(root, PhasePlanValidationOptions{RunID: runID, Final: true})
	if err != nil {
		t.Fatalf("ValidatePhasePlan() before decision error = %v", err)
	}
	if failed.Status != PhasePlanStatusFail || !phaseCheckFailed(failed.Checks, "final_approval_records") {
		t.Fatalf("failed checks = %#v, want approval failure", failed.Checks)
	}

	decision, err := RecordApproval(root, ApprovalRecordOptions{RunID: runID, Phase: "implement", Decision: ApprovalDecisionApproved, Approver: "master", Evidence: "messages/123", Reason: "Approved after review.", Now: testRunNow(7)})
	if err != nil {
		t.Fatalf("RecordApproval() error = %v", err)
	}
	if decision.EventID != "evt-000006" || decision.Record.Decision != ApprovalDecisionApproved || decision.Record.Approver != "master" {
		t.Fatalf("decision = %#v, want approved decision", decision)
	}

	shown, err := ShowApprovals(root, ApprovalShowOptions{RunID: runID, Phase: "implement"})
	if err != nil {
		t.Fatalf("ShowApprovals() error = %v", err)
	}
	if len(shown.Records) != 2 || shown.Records[0].Type != ApprovalEventRequested || shown.Records[1].Decision != ApprovalDecisionApproved {
		t.Fatalf("shown = %#v, want request and decision", shown)
	}
	lines := runEventLines(t, repo)
	if !strings.Contains(lines[4], `"approval.requested"`) || !strings.Contains(lines[5], `"approval.recorded"`) {
		t.Fatalf("events = %#v, want approval events", lines)
	}
}

func TestApprovalValidationRejectsMissingRequiredFields(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	created, err := CreateRun(root, deterministicCreateRunOptions())
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	_, err = RequestApproval(root, ApprovalRequestOptions{RunID: created.Metadata.RunID, Phase: "implement", Now: testRunNow(4)})
	assertProblemCode(t, err, "approval_reason_required")
	_, err = RecordApproval(root, ApprovalRecordOptions{RunID: created.Metadata.RunID, Phase: "implement", Decision: "maybe", Approver: "master", Evidence: "messages/123", Now: testRunNow(4)})
	assertProblemCode(t, err, "approval_decision_invalid")
	_, err = AppendEvent(root, AppendEventOptions{Type: ApprovalEventRecorded, RunID: created.Metadata.RunID, Payload: map[string]any{"phase": "implement", "timestamp": "2026-04-30T01:02:03Z", "decision": "approved"}, Now: testRunNow(4)})
	assertProblemCode(t, err, "approval_approver_required")
}

func TestApprovalLatestDecisionWinsFinalValidation(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	created, err := CreateRun(root, deterministicCreateRunOptions())
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	runID := created.Metadata.RunID
	if _, err := InitPhasePlan(root, PhasePlanInitOptions{RunID: runID, Now: testRunNow(4)}); err != nil {
		t.Fatalf("InitPhasePlan() error = %v", err)
	}
	if _, err := SetPhasePlanPhase(root, PhasePlanSetOptions{RunID: runID, PhaseID: "implement", Status: PhaseStatusComplete, Evidence: "diff.patch", ApprovalRequiredSet: true, ApprovalRequired: true, Now: testRunNow(5)}); err != nil {
		t.Fatalf("SetPhasePlanPhase() error = %v", err)
	}

	if _, err := RecordApproval(root, ApprovalRecordOptions{RunID: runID, Phase: "implement", Decision: ApprovalDecisionRejected, Approver: "master", Evidence: "messages/reject", Now: testRunNow(6)}); err != nil {
		t.Fatalf("RecordApproval(rejected) error = %v", err)
	}
	rejected, err := ValidatePhasePlan(root, PhasePlanValidationOptions{RunID: runID, Final: true})
	if err != nil {
		t.Fatalf("ValidatePhasePlan(rejected) error = %v", err)
	}
	if rejected.Status != PhasePlanStatusFail || !phaseCheckFailed(rejected.Checks, "final_approval_records") {
		t.Fatalf("rejected checks = %#v, want latest rejected approval failure", rejected.Checks)
	}

	if _, err := RecordApproval(root, ApprovalRecordOptions{RunID: runID, Phase: "implement", Decision: ApprovalDecisionApproved, Approver: "master", Evidence: "messages/approve", Now: testRunNow(7)}); err != nil {
		t.Fatalf("RecordApproval(approved) error = %v", err)
	}
	approved, err := ValidatePhasePlan(root, PhasePlanValidationOptions{RunID: runID, Final: true})
	if err != nil {
		t.Fatalf("ValidatePhasePlan(approved) error = %v", err)
	}
	if !phaseCheckPassed(approved.Checks, "final_approval_records") {
		t.Fatalf("approved checks = %#v, want approval check pass after latest approved decision", approved.Checks)
	}

	if _, err := RecordApproval(root, ApprovalRecordOptions{RunID: runID, Phase: "implement", Decision: ApprovalDecisionRejected, Approver: "master", Evidence: "messages/reject-again", Now: testRunNow(8)}); err != nil {
		t.Fatalf("RecordApproval(rejected again) error = %v", err)
	}
	rejectedAgain, err := ValidatePhasePlan(root, PhasePlanValidationOptions{RunID: runID, Final: true})
	if err != nil {
		t.Fatalf("ValidatePhasePlan(rejected again) error = %v", err)
	}
	if rejectedAgain.Status != PhasePlanStatusFail || !phaseCheckFailed(rejectedAgain.Checks, "final_approval_records") {
		t.Fatalf("rejectedAgain checks = %#v, want latest rejected approval failure", rejectedAgain.Checks)
	}
}

func TestApprovalMutationsRejectFinishedRuns(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	closed, err := CreateRun(root, deterministicCreateRunOptions())
	if err != nil {
		t.Fatalf("CreateRun(closed) error = %v", err)
	}
	if _, err := CloseRun(root, RunLifecycleOptions{RunID: closed.Metadata.RunID, Now: testRunNow(4)}); err != nil {
		t.Fatalf("CloseRun() error = %v", err)
	}
	_, err = RequestApproval(root, ApprovalRequestOptions{RunID: closed.Metadata.RunID, Phase: "implement", Reason: "Needs approval.", Now: testRunNow(5)})
	assertProblemCode(t, err, "run_approval_invalid_state")
	_, err = RecordApproval(root, ApprovalRecordOptions{RunID: closed.Metadata.RunID, Phase: "implement", Decision: ApprovalDecisionApproved, Approver: "master", Evidence: "messages/123", Now: testRunNow(5)})
	assertProblemCode(t, err, "run_approval_invalid_state")

	abortedOptions := deterministicCreateRunOptions()
	abortedOptions.TaskID = "runwf-002"
	abortedOptions.Title = "Aborted approval run"
	abortedOptions.Now = testRunNow(6)
	abortedOptions.RandomHex = func(int) (string, error) { return "bbbbbbbbbbbb", nil }
	aborted, err := CreateRun(root, abortedOptions)
	if err != nil {
		t.Fatalf("CreateRun(aborted) error = %v", err)
	}
	if _, err := AbortRun(root, RunLifecycleOptions{RunID: aborted.Metadata.RunID, Now: testRunNow(7)}); err != nil {
		t.Fatalf("AbortRun() error = %v", err)
	}
	_, err = RequestApproval(root, ApprovalRequestOptions{RunID: aborted.Metadata.RunID, Phase: "implement", Reason: "Needs approval.", Now: testRunNow(8)})
	assertProblemCode(t, err, "run_approval_invalid_state")
	_, err = RecordApproval(root, ApprovalRecordOptions{RunID: aborted.Metadata.RunID, Phase: "implement", Decision: ApprovalDecisionApproved, Approver: "master", Evidence: "messages/123", Now: testRunNow(8)})
	assertProblemCode(t, err, "run_approval_invalid_state")
}

func TestApprovalShowWithoutPhaseReturnsAllRecords(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	created, err := CreateRun(root, deterministicCreateRunOptions())
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	runID := created.Metadata.RunID
	if _, err := RequestApproval(root, ApprovalRequestOptions{RunID: runID, Phase: "implement", Reason: "Implementation risk.", Now: testRunNow(4)}); err != nil {
		t.Fatalf("RequestApproval(implement) error = %v", err)
	}
	if _, err := RecordApproval(root, ApprovalRecordOptions{RunID: runID, Phase: "verify", Decision: ApprovalDecisionApproved, Approver: "master", Evidence: "messages/verify", Now: testRunNow(5)}); err != nil {
		t.Fatalf("RecordApproval(verify) error = %v", err)
	}

	shown, err := ShowApprovals(root, ApprovalShowOptions{RunID: runID})
	if err != nil {
		t.Fatalf("ShowApprovals() error = %v", err)
	}
	data, err := json.Marshal(shown.Records)
	if err != nil {
		t.Fatalf("marshal shown records: %v", err)
	}
	if len(shown.Records) != 2 || !strings.Contains(string(data), `"phase":"implement"`) || !strings.Contains(string(data), `"phase":"verify"`) {
		t.Fatalf("shown = %s, want all approval records across phases", data)
	}
}
