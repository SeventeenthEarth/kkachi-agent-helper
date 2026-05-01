package project

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckGateIntakePassesAndRecordsState(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	created, err := CreateRun(root, deterministicCreateRunOptions())
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	if _, err := InitArtifacts(root, ArtifactInitOptions{RunID: created.Metadata.RunID, Now: testRunNow(4)}); err != nil {
		t.Fatalf("InitArtifacts() error = %v", err)
	}
	writeIntakeClassification(t, repo, created.Metadata, "")

	result, err := CheckGate(root, GateCheckOptions{RunID: created.Metadata.RunID, Gate: GateIntake, Now: testRunNow(5)})
	if err != nil {
		t.Fatalf("CheckGate() error = %v", err)
	}
	if result.RunID != created.Metadata.RunID || result.Gate != GateIntake || result.Status != GateStatusPass || result.EventID != "evt-000004" {
		t.Fatalf("result = %#v, want passing intake gate with evt-000004", result)
	}
	if len(result.MissingEvidence) != 0 || !gateCheckStatus(result.Checks, "required_artifacts", GateStatusPass) {
		t.Fatalf("checks = %#v missing=%#v, want pass checks and no missing evidence", result.Checks, result.MissingEvidence)
	}

	metadata := readRunMetadata(t, repo, created.Metadata.RunID)
	assertGateState(t, metadata.GateState, GateIntake, GateStatusPass, "evt-000004")
	var status map[string]any
	readJSONFile(t, filepath.Join(repo, ".kkachi", "status.json"), &status)
	assertStatusGateSummary(t, status, GateIntake, created.Metadata.RunID, GateStatusPass, "evt-000004")
	assertStatusAndEventTail(t, repo, "evt-000004", "evt-000004")
	if got := eventTypes(t, repo); got[len(got)-1] != "gate.passed" {
		t.Fatalf("last event type = %q, want gate.passed", got[len(got)-1])
	}
}

func TestCheckGateIntakeFailureRecordsMissingEvidence(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	created, err := CreateRun(root, deterministicCreateRunOptions())
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	if _, err := InitArtifacts(root, ArtifactInitOptions{RunID: created.Metadata.RunID, Now: testRunNow(4)}); err != nil {
		t.Fatalf("InitArtifacts() error = %v", err)
	}

	result, err := CheckGate(root, GateCheckOptions{RunID: created.Metadata.RunID, Gate: GateIntake, Now: testRunNow(5)})
	if err != nil {
		t.Fatalf("CheckGate() error = %v", err)
	}
	if result.Status != GateStatusFail || result.EventID != "evt-000004" || len(result.MissingEvidence) == 0 {
		t.Fatalf("result = %#v, want fail with missing evidence", result)
	}
	if !gateCheckStatus(result.Checks, "intake_status", GateStatusFail) {
		t.Fatalf("checks = %#v, want intake_status failure", result.Checks)
	}
	metadata := readRunMetadata(t, repo, created.Metadata.RunID)
	assertGateState(t, metadata.GateState, GateIntake, GateStatusFail, "evt-000004")
	if got := eventTypes(t, repo); got[len(got)-1] != "gate.failed" {
		t.Fatalf("last event type = %q, want gate.failed", got[len(got)-1])
	}
}

func TestCheckGateRecheckOverwritesGateSummary(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	created, err := CreateRun(root, deterministicCreateRunOptions())
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	if _, err := InitArtifacts(root, ArtifactInitOptions{RunID: created.Metadata.RunID, Now: testRunNow(4)}); err != nil {
		t.Fatalf("InitArtifacts() error = %v", err)
	}

	failed, err := CheckGate(root, GateCheckOptions{RunID: created.Metadata.RunID, Gate: GateIntake, Now: testRunNow(5)})
	if err != nil {
		t.Fatalf("CheckGate(fail) error = %v", err)
	}
	if failed.Status != GateStatusFail || failed.EventID != "evt-000004" {
		t.Fatalf("failed = %#v, want fail evt-000004", failed)
	}

	writeIntakeClassification(t, repo, created.Metadata, "")
	passed, err := CheckGate(root, GateCheckOptions{RunID: created.Metadata.RunID, Gate: GateIntake, Now: testRunNow(6)})
	if err != nil {
		t.Fatalf("CheckGate(pass) error = %v", err)
	}
	if passed.Status != GateStatusPass || passed.EventID != "evt-000005" {
		t.Fatalf("passed = %#v, want pass evt-000005", passed)
	}

	metadata := readRunMetadata(t, repo, created.Metadata.RunID)
	assertGateState(t, metadata.GateState, GateIntake, GateStatusPass, "evt-000005")
	var status map[string]any
	readJSONFile(t, filepath.Join(repo, ".kkachi", "status.json"), &status)
	assertStatusGateSummary(t, status, GateIntake, created.Metadata.RunID, GateStatusPass, "evt-000005")
}

func TestCheckGateSOTPathAPassesAndFailsWithArtifactEvidence(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	created, err := CreateRun(root, deterministicCreateRunOptions())
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	if _, err := InitArtifacts(root, ArtifactInitOptions{RunID: created.Metadata.RunID, Now: testRunNow(4)}); err != nil {
		t.Fatalf("InitArtifacts() error = %v", err)
	}

	failed, err := CheckGate(root, GateCheckOptions{RunID: created.Metadata.RunID, Gate: GateSOT, Now: testRunNow(5)})
	if err != nil {
		t.Fatalf("CheckGate(sot fail) error = %v", err)
	}
	if failed.Status != GateStatusFail || failed.EventID != "evt-000004" || !gateCheckStatus(failed.Checks, "sot_basis", GateStatusFail) || len(failed.MissingEvidence) == 0 {
		t.Fatalf("failed = %#v, want pending SOT basis failure", failed)
	}
	metadata := readRunMetadata(t, repo, created.Metadata.RunID)
	assertGateState(t, metadata.GateState, GateSOT, GateStatusFail, "evt-000004")

	writeMarkdownArtifact(t, repo, created.Metadata.RunID, "sot-basis.md", "Status: complete\nSource: docs/specs.md\n")
	passed, err := CheckGate(root, GateCheckOptions{RunID: created.Metadata.RunID, Gate: GateSOT, Now: testRunNow(6)})
	if err != nil {
		t.Fatalf("CheckGate(sot pass) error = %v", err)
	}
	if passed.Status != GateStatusPass || passed.EventID != "evt-000005" || !gateCheckStatus(passed.Checks, "sot_basis", GateStatusPass) || len(passed.MissingEvidence) != 0 {
		t.Fatalf("passed = %#v, want completed SOT basis pass", passed)
	}
	metadata = readRunMetadata(t, repo, created.Metadata.RunID)
	assertGateState(t, metadata.GateState, GateSOT, GateStatusPass, "evt-000005")
	if got := eventTypes(t, repo); got[len(got)-1] != "gate.passed" {
		t.Fatalf("last event type = %q, want gate.passed", got[len(got)-1])
	}
}

func TestCheckGateSOTPathBUsesSOTUpdate(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	options := deterministicCreateRunOptions()
	options.WorkPath = "B_discovery_shaping"
	options.SOTPolicy = "minimal_sot_before_code"
	options.ExecutionMode = "research"
	created, err := CreateRun(root, options)
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	if _, err := InitArtifacts(root, ArtifactInitOptions{RunID: created.Metadata.RunID, Now: testRunNow(4)}); err != nil {
		t.Fatalf("InitArtifacts() error = %v", err)
	}
	writeMarkdownArtifact(t, repo, created.Metadata.RunID, "sot-update.md", "Status: complete\nSOT: created for handoff\n")

	result, err := CheckGate(root, GateCheckOptions{RunID: created.Metadata.RunID, Gate: GateSOT, Now: testRunNow(5)})
	if err != nil {
		t.Fatalf("CheckGate(sot Path B) error = %v", err)
	}
	if result.Status != GateStatusPass || result.EventID != "evt-000004" || !gateCheckStatus(result.Checks, "sot_update", GateStatusPass) {
		t.Fatalf("result = %#v, want Path B SOT update pass", result)
	}
}

func TestCheckGateSOTRejectsNotApplicableEvidence(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	created, err := CreateRun(root, deterministicCreateRunOptions())
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	if _, err := InitArtifacts(root, ArtifactInitOptions{RunID: created.Metadata.RunID, Now: testRunNow(4)}); err != nil {
		t.Fatalf("InitArtifacts() error = %v", err)
	}
	writeMarkdownArtifact(t, repo, created.Metadata.RunID, "sot-basis.md", "Status: not_applicable\nReason: SOT is optional\n")

	result, err := CheckGate(root, GateCheckOptions{RunID: created.Metadata.RunID, Gate: GateSOT, Now: testRunNow(5)})
	if err != nil {
		t.Fatalf("CheckGate(sot not_applicable) error = %v", err)
	}
	if result.Status != GateStatusFail || result.EventID != "evt-000004" || !gateCheckStatus(result.Checks, "sot_basis", GateStatusFail) || !gateCheckActual(result.Checks, "sot_basis", "not_applicable") {
		t.Fatalf("result = %#v, want SOT not_applicable rejection", result)
	}
}

func TestCheckGateRoadmapPassesByTaskIDOrExplicitArtifactException(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	created, err := CreateRun(root, deterministicCreateRunOptions())
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	if _, err := InitArtifacts(root, ArtifactInitOptions{RunID: created.Metadata.RunID, Now: testRunNow(4)}); err != nil {
		t.Fatalf("InitArtifacts() error = %v", err)
	}

	taskTrace, err := CheckGate(root, GateCheckOptions{RunID: created.Metadata.RunID, Gate: GateRoadmap, Now: testRunNow(5)})
	if err != nil {
		t.Fatalf("CheckGate(roadmap task trace) error = %v", err)
	}
	if taskTrace.Status != GateStatusPass || taskTrace.EventID != "evt-000004" || !gateCheckStatus(taskTrace.Checks, "roadmap_trace", GateStatusPass) {
		t.Fatalf("taskTrace = %#v, want task_id roadmap pass", taskTrace)
	}

	metadata := readRunMetadata(t, repo, created.Metadata.RunID)
	metadata.TaskID = nil
	writeRunMetadataForTest(t, repo, metadata)
	pending, err := CheckGate(root, GateCheckOptions{RunID: created.Metadata.RunID, Gate: GateRoadmap, Now: testRunNow(6)})
	if err != nil {
		t.Fatalf("CheckGate(roadmap pending) error = %v", err)
	}
	if pending.Status != GateStatusFail || pending.EventID != "evt-000005" || !gateCheckStatus(pending.Checks, "roadmap_trace", GateStatusFail) {
		t.Fatalf("pending = %#v, want roadmap-update failure without task_id", pending)
	}

	writeMarkdownArtifact(t, repo, created.Metadata.RunID, "roadmap-update.md", "Status: not_applicable\nReason: existing roadmap already traces this non-roadmap run\n")
	exception, err := CheckGate(root, GateCheckOptions{RunID: created.Metadata.RunID, Gate: GateRoadmap, Now: testRunNow(7)})
	if err != nil {
		t.Fatalf("CheckGate(roadmap exception) error = %v", err)
	}
	if exception.Status != GateStatusPass || exception.EventID != "evt-000006" || len(exception.MissingEvidence) != 0 {
		t.Fatalf("exception = %#v, want explicit not-applicable pass", exception)
	}
}

func TestCheckGateRoadmapPassesByCompletedUpdateAndRejectsMissingExceptionReason(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	created, err := CreateRun(root, deterministicCreateRunOptions())
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	if _, err := InitArtifacts(root, ArtifactInitOptions{RunID: created.Metadata.RunID, Now: testRunNow(4)}); err != nil {
		t.Fatalf("InitArtifacts() error = %v", err)
	}
	metadata := readRunMetadata(t, repo, created.Metadata.RunID)
	metadata.TaskID = nil
	writeRunMetadataForTest(t, repo, metadata)

	writeMarkdownArtifact(t, repo, created.Metadata.RunID, "roadmap-update.md", "Status: complete\nTrace: docs/roadmap.md gates-002\n")
	completed, err := CheckGate(root, GateCheckOptions{RunID: created.Metadata.RunID, Gate: GateRoadmap, Now: testRunNow(5)})
	if err != nil {
		t.Fatalf("CheckGate(roadmap complete) error = %v", err)
	}
	if completed.Status != GateStatusPass || completed.EventID != "evt-000004" || !gateCheckStatus(completed.Checks, "roadmap_trace", GateStatusPass) {
		t.Fatalf("completed = %#v, want completed roadmap-update pass", completed)
	}

	writeMarkdownArtifact(t, repo, created.Metadata.RunID, "roadmap-update.md", "Status: not_applicable\nReason:   \n")
	missingReason, err := CheckGate(root, GateCheckOptions{RunID: created.Metadata.RunID, Gate: GateRoadmap, Now: testRunNow(6)})
	if err != nil {
		t.Fatalf("CheckGate(roadmap missing reason) error = %v", err)
	}
	if missingReason.Status != GateStatusFail || missingReason.EventID != "evt-000005" || !gateCheckStatus(missingReason.Checks, "roadmap_trace", GateStatusFail) || !gateCheckActual(missingReason.Checks, "roadmap_trace", "not_applicable without reason") {
		t.Fatalf("missingReason = %#v, want not_applicable without reason failure", missingReason)
	}
}

func TestCheckGatePlanRequiresAcceptancePlanAndChecklist(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	created, err := CreateRun(root, deterministicCreateRunOptions())
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	if _, err := InitArtifacts(root, ArtifactInitOptions{RunID: created.Metadata.RunID, Now: testRunNow(4)}); err != nil {
		t.Fatalf("InitArtifacts() error = %v", err)
	}
	writeMarkdownArtifact(t, repo, created.Metadata.RunID, "acceptance-criteria.md", "Status: complete\nCriteria: deterministic\n")
	writeMarkdownArtifact(t, repo, created.Metadata.RunID, "plan.md", "Status: complete\nPlan: validate gates\n")

	failed, err := CheckGate(root, GateCheckOptions{RunID: created.Metadata.RunID, Gate: GatePlan, Now: testRunNow(5)})
	if err != nil {
		t.Fatalf("CheckGate(plan fail) error = %v", err)
	}
	if failed.Status != GateStatusFail || failed.EventID != "evt-000004" || !gateCheckStatus(failed.Checks, "checklist_artifact", GateStatusFail) || len(failed.MissingEvidence) != 1 {
		t.Fatalf("failed = %#v, want checklist failure", failed)
	}

	writeMarkdownArtifact(t, repo, created.Metadata.RunID, "checklist.md", "Status: complete\n- [x] lock plan gate\n")
	passed, err := CheckGate(root, GateCheckOptions{RunID: created.Metadata.RunID, Gate: GatePlan, Now: testRunNow(6)})
	if err != nil {
		t.Fatalf("CheckGate(plan pass) error = %v", err)
	}
	if passed.Status != GateStatusPass || passed.EventID != "evt-000005" || len(passed.MissingEvidence) != 0 {
		t.Fatalf("passed = %#v, want plan pass", passed)
	}
}

func TestCheckGatePlanRejectsNotApplicableEvidence(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	created, err := CreateRun(root, deterministicCreateRunOptions())
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	if _, err := InitArtifacts(root, ArtifactInitOptions{RunID: created.Metadata.RunID, Now: testRunNow(4)}); err != nil {
		t.Fatalf("InitArtifacts() error = %v", err)
	}
	writeMarkdownArtifact(t, repo, created.Metadata.RunID, "acceptance-criteria.md", "Status: not_applicable\nReason: low risk\n")
	writeMarkdownArtifact(t, repo, created.Metadata.RunID, "plan.md", "Status: complete\nPlan: validate gates\n")
	writeMarkdownArtifact(t, repo, created.Metadata.RunID, "checklist.md", "Status: complete\n- [x] lock plan gate\n")

	result, err := CheckGate(root, GateCheckOptions{RunID: created.Metadata.RunID, Gate: GatePlan, Now: testRunNow(5)})
	if err != nil {
		t.Fatalf("CheckGate(plan not_applicable) error = %v", err)
	}
	if result.Status != GateStatusFail || result.EventID != "evt-000004" || !gateCheckStatus(result.Checks, "acceptance_criteria", GateStatusFail) || !gateCheckActual(result.Checks, "acceptance_criteria", "not_applicable") {
		t.Fatalf("result = %#v, want plan not_applicable rejection", result)
	}
}

func TestCheckGateMarkdownArtifactFailures(t *testing.T) {
	tests := []struct {
		name        string
		artifact    string
		gate        string
		check       string
		setup       func(t *testing.T, path string)
		wantActual  string
		wantMissing bool
	}{
		{name: "empty SOT basis", artifact: "sot-basis.md", gate: GateSOT, check: "sot_basis", setup: func(t *testing.T, path string) {
			t.Helper()
			mustWriteFile(t, path, nil)
		}, wantActual: "empty", wantMissing: true},
		{name: "directory plan", artifact: "plan.md", gate: GatePlan, check: "plan_artifact", setup: func(t *testing.T, path string) {
			t.Helper()
			mustRemove(t, path)
			mustMkdir(t, path)
		}, wantActual: "directory", wantMissing: true},
		{name: "missing checklist", artifact: "checklist.md", gate: GatePlan, check: "checklist_artifact", setup: func(t *testing.T, path string) {
			t.Helper()
			mustRemove(t, path)
		}, wantActual: "missing", wantMissing: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := initializedRepo(t)
			root, _ := DiscoverRoot(repo)
			created, err := CreateRun(root, deterministicCreateRunOptions())
			if err != nil {
				t.Fatalf("CreateRun() error = %v", err)
			}
			if _, err := InitArtifacts(root, ArtifactInitOptions{RunID: created.Metadata.RunID, Now: testRunNow(4)}); err != nil {
				t.Fatalf("InitArtifacts() error = %v", err)
			}
			writeCompletedPlanArtifacts(t, repo, created.Metadata.RunID)
			path := filepath.Join(repo, ".kkachi", "runs", created.Metadata.RunID, filepath.FromSlash(tt.artifact))
			tt.setup(t, path)

			result, err := CheckGate(root, GateCheckOptions{RunID: created.Metadata.RunID, Gate: tt.gate, Now: testRunNow(5)})
			if err != nil {
				t.Fatalf("CheckGate(%s) error = %v", tt.gate, err)
			}
			if result.Status != GateStatusFail || !gateCheckStatus(result.Checks, tt.check, GateStatusFail) || !gateCheckActual(result.Checks, tt.check, tt.wantActual) {
				t.Fatalf("result = %#v, want %s actual %q failure", result, tt.check, tt.wantActual)
			}
			if tt.wantMissing && len(result.MissingEvidence) == 0 {
				t.Fatalf("missing evidence = %#v, want non-empty", result.MissingEvidence)
			}
		})
	}
}

func TestCheckGateRefusesUnknownAndCoherenceMismatchWithoutMutation(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	created, err := CreateRun(root, deterministicCreateRunOptions())
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	before := len(runEventLines(t, repo))
	_, err = CheckGate(root, GateCheckOptions{RunID: created.Metadata.RunID, Gate: "bogus", Now: testRunNow(4)})
	assertProblemCode(t, err, "gate_unknown")
	if after := len(runEventLines(t, repo)); after != before {
		t.Fatalf("event count changed from %d to %d for unknown gate", before, after)
	}

	appendCrashEvent(t, repo, "evt-000003", created.Metadata.RunID)
	_, err = CheckGate(root, GateCheckOptions{RunID: created.Metadata.RunID, Gate: GateIntake, Now: testRunNow(4)})
	assertProblemCode(t, err, "last_event_id_mismatch")
	if after := len(runEventLines(t, repo)); after != before+1 {
		t.Fatalf("event count after mismatch = %d, want crash line only", after)
	}
}

func gateCheckStatus(checks []GateCheck, name string, status string) bool {
	for _, check := range checks {
		if check.Name == name && check.Status == status {
			return true
		}
	}
	return false
}

func gateCheckActual(checks []GateCheck, name string, actual string) bool {
	for _, check := range checks {
		if check.Name == name && check.Actual == actual {
			return true
		}
	}
	return false
}

func assertGateState(t *testing.T, gateState map[string]any, gate string, status string, eventID string) {
	t.Helper()
	entry, ok := gateState[gate].(map[string]any)
	if !ok {
		t.Fatalf("gate_state[%s] = %#v, want object", gate, gateState[gate])
	}
	if entry["status"] != status || entry["event_id"] != eventID {
		t.Fatalf("gate_state[%s] = %#v, want status %s event %s", gate, entry, status, eventID)
	}
}

func assertStatusGateSummary(t *testing.T, status map[string]any, gate string, runID string, gateStatus string, eventID string) {
	t.Helper()
	summary, ok := status["gate_summary"].(map[string]any)
	if !ok {
		t.Fatalf("gate_summary = %#v, want object", status["gate_summary"])
	}
	entry, ok := summary[gate].(map[string]any)
	if !ok {
		t.Fatalf("gate_summary[%s] = %#v, want object", gate, summary[gate])
	}
	if entry["run_id"] != runID || entry["status"] != gateStatus || entry["event_id"] != eventID {
		t.Fatalf("gate_summary[%s] = %#v, want run/status/event", gate, entry)
	}
}

func appendCrashEvent(t *testing.T, repo string, eventID string, runID string) {
	t.Helper()
	line := fmt.Sprintf(`{"version":"0.1","event_id":"%s","occurred_at":"2026-04-30T03:00:00Z","run_id":"%s","type":"gate.checked","actor":"helper","payload":{}}`, eventID, runID)
	path := filepath.Join(repo, ".kkachi", "events.jsonl")
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("open event log: %v", err)
	}
	defer file.Close()
	if _, err := file.WriteString(line + "\n"); err != nil {
		t.Fatalf("append crash event: %v", err)
	}
}

func eventTypes(t *testing.T, repo string) []string {
	t.Helper()
	lines := runEventLines(t, repo)
	types := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var event Event
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("decode event line %q: %v", line, err)
		}
		types = append(types, event.Type)
	}
	return types
}

func writeCompletedPlanArtifacts(t *testing.T, repo string, runID string) {
	t.Helper()
	writeMarkdownArtifact(t, repo, runID, "acceptance-criteria.md", "Status: complete\nCriteria: deterministic\n")
	writeMarkdownArtifact(t, repo, runID, "plan.md", "Status: complete\nPlan: validate gates\n")
	writeMarkdownArtifact(t, repo, runID, "checklist.md", "Status: complete\n- [x] lock plan gate\n")
}

func writeMarkdownArtifact(t *testing.T, repo string, runID string, artifact string, body string) {
	t.Helper()
	path := filepath.Join(repo, ".kkachi", "runs", runID, filepath.FromSlash(artifact))
	mustWriteFile(t, path, []byte("# "+artifact+"\n\n"+body+"\n"))
}
