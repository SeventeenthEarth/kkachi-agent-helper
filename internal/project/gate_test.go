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

func TestCheckGateFutureGateBlocksAndRecordsState(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	created, err := CreateRun(root, deterministicCreateRunOptions())
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}

	result, err := CheckGate(root, GateCheckOptions{RunID: created.Metadata.RunID, Gate: GatePlan, Now: testRunNow(4)})
	if err != nil {
		t.Fatalf("CheckGate(plan) error = %v", err)
	}
	if result.Status != GateStatusBlocked || result.EventID != "evt-000003" || len(result.MissingEvidence) != 1 {
		t.Fatalf("result = %#v, want blocked placeholder", result)
	}
	if !gateCheckStatus(result.Checks, "plan_implemented", GateStatusBlocked) {
		t.Fatalf("checks = %#v, want plan_implemented blocked", result.Checks)
	}
	metadata := readRunMetadata(t, repo, created.Metadata.RunID)
	assertGateState(t, metadata.GateState, GatePlan, GateStatusBlocked, "evt-000003")
	if got := eventTypes(t, repo); got[len(got)-1] != "gate.checked" {
		t.Fatalf("last event type = %q, want gate.checked", got[len(got)-1])
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
