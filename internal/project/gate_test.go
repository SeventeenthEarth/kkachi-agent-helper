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
	repo, root, runID := initializedPlanGateRun(t)
	writeMarkdownArtifact(t, repo, runID, "acceptance-criteria.md", "Status: complete\nCriteria: deterministic\n")
	writeMarkdownArtifact(t, repo, runID, "plan.md", "Status: complete\nPlan: validate gates\n")

	failed, err := CheckGate(root, GateCheckOptions{RunID: runID, Gate: GatePlan, Now: testRunNow(5)})
	if err != nil {
		t.Fatalf("CheckGate(plan fail) error = %v", err)
	}
	if failed.Status != GateStatusFail || failed.EventID != "evt-000004" || !gateCheckStatus(failed.Checks, "checklist_artifact", GateStatusFail) || len(failed.MissingEvidence) != 1 {
		t.Fatalf("failed = %#v, want checklist failure", failed)
	}

	writeMarkdownArtifact(t, repo, runID, "checklist.md", "Status: complete\n- [x] lock plan gate\n")
	passed, err := CheckGate(root, GateCheckOptions{RunID: runID, Gate: GatePlan, Now: testRunNow(6)})
	if err != nil {
		t.Fatalf("CheckGate(plan pass) error = %v", err)
	}
	if passed.Status != GateStatusPass || passed.EventID != "evt-000005" || len(passed.MissingEvidence) != 0 {
		t.Fatalf("passed = %#v, want plan pass", passed)
	}
}

func TestCheckGatePlanRejectsNotApplicableEvidence(t *testing.T) {
	repo, root, runID := initializedPlanGateRun(t)
	writeMarkdownArtifact(t, repo, runID, "acceptance-criteria.md", "Status: not_applicable\nReason: low risk\n")
	writeMarkdownArtifact(t, repo, runID, "plan.md", "Status: complete\nPlan: validate gates\n")
	writeMarkdownArtifact(t, repo, runID, "checklist.md", "Status: complete\n- [x] lock plan gate\n")

	result, err := CheckGate(root, GateCheckOptions{RunID: runID, Gate: GatePlan, Now: testRunNow(5)})
	if err != nil {
		t.Fatalf("CheckGate(plan not_applicable) error = %v", err)
	}
	if result.Status != GateStatusFail || result.EventID != "evt-000004" || !gateCheckStatus(result.Checks, "acceptance_criteria", GateStatusFail) || !gateCheckActual(result.Checks, "acceptance_criteria", "not_applicable") {
		t.Fatalf("result = %#v, want plan not_applicable rejection", result)
	}
}

func TestCheckGatePlanIgnoresKABChecklistSeedSections(t *testing.T) {
	tests := []struct {
		name           string
		planBody       string
		writeChecklist bool
		wantStatus     string
		wantCheck      string
	}{
		{
			name:           "complete without KHS Checklist Seed",
			planBody:       "Status: complete\nPlan: implement from normalized KHS artifacts\n",
			writeChecklist: true,
			wantStatus:     GateStatusPass,
		},
		{
			name:           "complete with KHS Checklist Seed",
			planBody:       "Status: complete\nPlan: implement from normalized KHS artifacts\n\n## KHS Checklist Seed\n- seed item from KAB planner\n",
			writeChecklist: true,
			wantStatus:     GateStatusPass,
		},
		{
			name:           "seed section does not replace checklist artifact",
			planBody:       "Status: complete\nPlan: implement from normalized KHS artifacts\n\n## KHS Checklist Seed\n- seed item from KAB planner\n",
			writeChecklist: false,
			wantStatus:     GateStatusFail,
			wantCheck:      "checklist_artifact",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, root, runID := initializedPlanGateRun(t)
			writeMarkdownArtifact(t, repo, runID, "acceptance-criteria.md", "Status: complete\nCriteria: deterministic\n")
			writeMarkdownArtifact(t, repo, runID, "plan.md", tt.planBody)
			if tt.writeChecklist {
				writeMarkdownArtifact(t, repo, runID, "checklist.md", "Status: complete\n- [x] normalized by KHS\n")
			} else {
				mustRemove(t, filepath.Join(repo, ".kkachi", "runs", runID, "checklist.md"))
			}

			result, err := CheckGate(root, GateCheckOptions{RunID: runID, Gate: GatePlan, Now: testRunNow(5)})
			if err != nil {
				t.Fatalf("CheckGate(plan) error = %v", err)
			}
			if result.Status != tt.wantStatus {
				t.Fatalf("result = %#v, want status %s", result, tt.wantStatus)
			}
			if tt.wantCheck != "" && !gateCheckStatus(result.Checks, tt.wantCheck, tt.wantStatus) {
				t.Fatalf("result = %#v, want %s %s", result, tt.wantCheck, tt.wantStatus)
			}
		})
	}
}

func TestCheckGatePlanRejectsMissingEmptyAndPendingArtifacts(t *testing.T) {
	tests := []struct {
		name       string
		artifact   string
		check      string
		setup      func(t *testing.T, path string)
		wantActual string
	}{
		{
			name:     "missing acceptance criteria",
			artifact: "acceptance-criteria.md",
			check:    "acceptance_criteria",
			setup: func(t *testing.T, path string) {
				t.Helper()
				mustRemove(t, path)
			},
			wantActual: "missing",
		},
		{
			name:     "empty plan",
			artifact: "plan.md",
			check:    "plan_artifact",
			setup: func(t *testing.T, path string) {
				t.Helper()
				mustWriteFile(t, path, nil)
			},
			wantActual: "empty",
		},
		{
			name:     "pending checklist",
			artifact: "checklist.md",
			check:    "checklist_artifact",
			setup: func(t *testing.T, path string) {
				t.Helper()
				mustWriteFile(t, path, []byte("# checklist.md\n\nStatus: pending\n- [ ] unresolved\n"))
			},
			wantActual: "pending",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, root, runID := initializedPlanGateRun(t)
			writeCompletedPlanArtifacts(t, repo, runID)
			path := filepath.Join(repo, ".kkachi", "runs", runID, filepath.FromSlash(tt.artifact))
			tt.setup(t, path)

			result, err := CheckGate(root, GateCheckOptions{RunID: runID, Gate: GatePlan, Now: testRunNow(5)})
			if err != nil {
				t.Fatalf("CheckGate(plan) error = %v", err)
			}
			if result.Status != GateStatusFail || !gateCheckStatus(result.Checks, tt.check, GateStatusFail) || !gateCheckActual(result.Checks, tt.check, tt.wantActual) || len(result.MissingEvidence) == 0 {
				t.Fatalf("result = %#v, want failed %s with actual %q and missing evidence", result, tt.check, tt.wantActual)
			}
		})
	}
}

func initializedPlanGateRun(t *testing.T) (string, Root, string) {
	t.Helper()
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	created, err := CreateRun(root, deterministicCreateRunOptions())
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	if _, err := InitArtifacts(root, ArtifactInitOptions{RunID: created.Metadata.RunID, Now: testRunNow(4)}); err != nil {
		t.Fatalf("InitArtifacts() error = %v", err)
	}
	return repo, root, created.Metadata.RunID
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

func TestCheckGateBackendPassesWithValidAdapterQAEvidence(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	created := createAdapterQARunWithArtifacts(t, root)
	writeValidBackendEvidence(t, repo, created.Metadata.RunID)

	result, err := CheckGate(root, GateCheckOptions{RunID: created.Metadata.RunID, Gate: GateBackend, Now: testRunNow(5)})
	if err != nil {
		t.Fatalf("CheckGate(backend pass) error = %v", err)
	}
	if result.Status != GateStatusPass || result.EventID != "evt-000004" || len(result.MissingEvidence) != 0 {
		t.Fatalf("result = %#v, want backend pass with no missing evidence", result)
	}
	for _, name := range []string{"backend_manifest", "selected_cli", "capability_check", "bridge_session_snapshot", "bridge_events"} {
		if !gateCheckStatus(result.Checks, name, GateStatusPass) {
			t.Fatalf("checks = %#v, want %s pass", result.Checks, name)
		}
	}
	metadata := readRunMetadata(t, repo, created.Metadata.RunID)
	assertGateState(t, metadata.GateState, GateBackend, GateStatusPass, "evt-000004")
}

func TestCheckGateBackendNotApplicableWhenManifestDoesNotRequireBackend(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	created, err := CreateRun(root, deterministicCreateRunOptions())
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	if _, err := InitArtifacts(root, ArtifactInitOptions{RunID: created.Metadata.RunID, Now: testRunNow(4)}); err != nil {
		t.Fatalf("InitArtifacts() error = %v", err)
	}

	result, err := CheckGate(root, GateCheckOptions{RunID: created.Metadata.RunID, Gate: GateBackend, Now: testRunNow(5)})
	if err != nil {
		t.Fatalf("CheckGate(backend not applicable) error = %v", err)
	}
	if result.Status != GateStatusPass || result.EventID != "evt-000004" || len(result.Checks) != 1 || !gateCheckStatus(result.Checks, "backend_manifest", GateStatusPass) {
		t.Fatalf("result = %#v, want manifest-driven not-applicable pass", result)
	}
	if !strings.Contains(result.Checks[0].Message, "not applicable") || result.Checks[0].Path == "" {
		t.Fatalf("check = %#v, want manifest-tied not-applicable message", result.Checks[0])
	}
}

func TestCheckGateBackendFailsMissingArtifactsAndMalformedJSON(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(t *testing.T, repo string, runID string)
		checkName  string
		wantActual string
	}{
		{name: "missing selected cli", setup: func(t *testing.T, repo string, runID string) {
			mustRemove(t, filepath.Join(repo, ".kkachi", "runs", runID, "selected-cli.json"))
		}, checkName: "selected_cli", wantActual: "missing"},
		{name: "malformed selected cli", setup: func(t *testing.T, repo string, runID string) {
			mustWriteFile(t, filepath.Join(repo, ".kkachi", "runs", runID, "selected-cli.json"), []byte("{not-json\n"))
		}, checkName: "selected_cli", wantActual: "malformed"},
		{name: "malformed bridge snapshot", setup: func(t *testing.T, repo string, runID string) {
			mustWriteFile(t, filepath.Join(repo, ".kkachi", "runs", runID, "bridge-session-snapshot.json"), []byte("{not-json\n"))
		}, checkName: "bridge_session_snapshot", wantActual: "malformed"},
		{name: "selected cli array", setup: func(t *testing.T, repo string, runID string) {
			mustWriteFile(t, filepath.Join(repo, ".kkachi", "runs", runID, "selected-cli.json"), []byte("[]\n"))
		}, checkName: "selected_cli", wantActual: "malformed"},
		{name: "selected cli null", setup: func(t *testing.T, repo string, runID string) {
			mustWriteFile(t, filepath.Join(repo, ".kkachi", "runs", runID, "selected-cli.json"), []byte("null\n"))
		}, checkName: "selected_cli", wantActual: "null"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := initializedRepo(t)
			root, _ := DiscoverRoot(repo)
			created := createAdapterQARunWithArtifacts(t, root)
			writeValidBackendEvidence(t, repo, created.Metadata.RunID)
			tt.setup(t, repo, created.Metadata.RunID)

			result, err := CheckGate(root, GateCheckOptions{RunID: created.Metadata.RunID, Gate: GateBackend, Now: testRunNow(5)})
			if err != nil {
				t.Fatalf("CheckGate(backend fail) error = %v", err)
			}
			if result.Status != GateStatusFail || !gateCheckStatus(result.Checks, tt.checkName, GateStatusFail) || !gateCheckActual(result.Checks, tt.checkName, tt.wantActual) || len(result.MissingEvidence) == 0 {
				t.Fatalf("result = %#v, want %s failure actual %q", result, tt.checkName, tt.wantActual)
			}
		})
	}
}

func TestCheckGateBackendFailsUnsupportedIdentityPendingsAndIncompleteMarkdown(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(t *testing.T, repo string, runID string)
		checkName  string
		wantActual string
	}{
		{name: "unsupported selected cli", setup: func(t *testing.T, repo string, runID string) {
			writeSelectedCLI(t, repo, runID, "unsupported", "codex", "openai-codex")
		}, checkName: "selected_cli", wantActual: "unsupported"},
		{name: "pending selected cli", setup: func(t *testing.T, repo string, runID string) {
			writeSelectedCLI(t, repo, runID, "pending", "codex", "openai-codex")
		}, checkName: "selected_cli", wantActual: "pending"},
		{name: "invalid caveats number", setup: func(t *testing.T, repo string, runID string) {
			payload := map[string]any{"version": "0.1", "status": "supported", "backend_type": "codex", "adapter_type": "openai-codex", "source_ledger_ref": "docs/ledger.md#codex", "caveats": 42}
			writeJSONArtifact(t, repo, runID, "selected-cli.json", payload)
		}, checkName: "selected_cli", wantActual: "invalid"},
		{name: "invalid caveats null", setup: func(t *testing.T, repo string, runID string) {
			payload := map[string]any{"version": "0.1", "status": "supported", "backend_type": "codex", "adapter_type": "openai-codex", "source_ledger_ref": "docs/ledger.md#codex", "caveats": nil}
			writeJSONArtifact(t, repo, runID, "selected-cli.json", payload)
		}, checkName: "selected_cli", wantActual: "invalid"},
		{name: "identity mismatch", setup: func(t *testing.T, repo string, runID string) {
			writeBridgeSnapshot(t, repo, runID, "claude", "anthropic-claude", 0)
		}, checkName: "bridge_session_snapshot", wantActual: "claude/anthropic-claude"},
		{name: "open pendings", setup: func(t *testing.T, repo string, runID string) {
			writeBridgeSnapshot(t, repo, runID, "codex", "openai-codex", 2)
		}, checkName: "bridge_session_snapshot", wantActual: "2"},
		{name: "incomplete capability check", setup: func(t *testing.T, repo string, runID string) {
			writeMarkdownArtifact(t, repo, runID, "capability-check.md", "Status: pending\nBackend: codex\nAdapter: openai-codex\n")
		}, checkName: "capability_check", wantActual: "pending"},
		{name: "incomplete bridge events", setup: func(t *testing.T, repo string, runID string) {
			writeMarkdownArtifact(t, repo, runID, "bridge-events.md", "Status: pending\nEvent: started\n")
		}, checkName: "bridge_events", wantActual: "pending"},
		{name: "bridge events placeholder only", setup: func(t *testing.T, repo string, runID string) {
			writeMarkdownArtifact(t, repo, runID, "bridge-events.md", "Status: complete\nRun: "+runID+"\nRecord Kkachi evidence here. Use explicit not-applicable reasons when this artifact is intentionally out of scope.\n")
		}, checkName: "bridge_events", wantActual: "missing"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := initializedRepo(t)
			root, _ := DiscoverRoot(repo)
			created := createAdapterQARunWithArtifacts(t, root)
			writeValidBackendEvidence(t, repo, created.Metadata.RunID)
			tt.setup(t, repo, created.Metadata.RunID)

			result, err := CheckGate(root, GateCheckOptions{RunID: created.Metadata.RunID, Gate: GateBackend, Now: testRunNow(5)})
			if err != nil {
				t.Fatalf("CheckGate(backend fail) error = %v", err)
			}
			if result.Status != GateStatusFail || !gateCheckStatus(result.Checks, tt.checkName, GateStatusFail) || !gateCheckActual(result.Checks, tt.checkName, tt.wantActual) {
				t.Fatalf("result = %#v, want %s failure actual %q", result, tt.checkName, tt.wantActual)
			}
		})
	}
}

func TestCheckGateBackendFailsPartialManifest(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	created := createAdapterQARunWithArtifacts(t, root)
	metadata := readRunMetadata(t, repo, created.Metadata.RunID)
	metadata.RequiredArtifacts = removeArtifactForTest(metadata.RequiredArtifacts, "bridge-events.md")
	writeRunMetadataForTest(t, repo, metadata)
	writeValidBackendEvidence(t, repo, created.Metadata.RunID)

	result, err := CheckGate(root, GateCheckOptions{RunID: created.Metadata.RunID, Gate: GateBackend, Now: testRunNow(5)})
	if err != nil {
		t.Fatalf("CheckGate(backend partial manifest) error = %v", err)
	}
	if result.Status != GateStatusFail || !gateCheckStatus(result.Checks, "backend_manifest", GateStatusFail) || !gateCheckActual(result.Checks, "backend_manifest", "missing bridge-events.md") {
		t.Fatalf("result = %#v, want backend_manifest missing bridge-events.md failure", result)
	}
}

func removeArtifactForTest(artifacts []string, remove string) []string {
	kept := []string{}
	for _, artifact := range artifacts {
		if artifact != remove {
			kept = append(kept, artifact)
		}
	}
	return kept
}

func createAdapterQARunWithArtifacts(t *testing.T, root Root) CreateRunResult {
	t.Helper()
	options := deterministicCreateRunOptions()
	options.ExecutionMode = "adapter_qa"
	options.TaskID = "gates-003"
	created, err := CreateRun(root, options)
	if err != nil {
		t.Fatalf("CreateRun(adapter_qa) error = %v", err)
	}
	if _, err := InitArtifacts(root, ArtifactInitOptions{RunID: created.Metadata.RunID, Now: testRunNow(4)}); err != nil {
		t.Fatalf("InitArtifacts(adapter_qa) error = %v", err)
	}
	return created
}

func writeValidBackendEvidence(t *testing.T, repo string, runID string) {
	t.Helper()
	writeSelectedCLI(t, repo, runID, "supported", "codex", "openai-codex")
	writeMarkdownArtifact(t, repo, runID, "capability-check.md", "Status: complete\nBackend Type: codex\nAdapter Type: openai-codex\nCapability: thread resume checked\n")
	writeBridgeSnapshot(t, repo, runID, "codex", "openai-codex", 0)
	writeMarkdownArtifact(t, repo, runID, "bridge-events.md", "Status: complete\nEvent: bridge opened a codex session and emitted output\n")
}

func writeSelectedCLI(t *testing.T, repo string, runID string, status string, backendType string, adapterType string) {
	t.Helper()
	payload := map[string]any{"version": "0.1", "status": status, "backend_type": backendType, "adapter_type": adapterType, "source_ledger_ref": "docs/ledger.md#codex", "caveats": []string{}}
	writeJSONArtifact(t, repo, runID, "selected-cli.json", payload)
}

func writeBridgeSnapshot(t *testing.T, repo string, runID string, backendType string, adapterType string, openPendings int) {
	t.Helper()
	payload := map[string]any{"session_id": "session-123", "backend_type": backendType, "adapter_type": adapterType, "state": "running", "lifecycle_class": "interactive", "open_pendings": openPendings}
	writeJSONArtifact(t, repo, runID, "bridge-session-snapshot.json", payload)
}

func writeJSONArtifact(t *testing.T, repo string, runID string, artifact string, payload any) {
	t.Helper()
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatalf("marshal %s: %v", artifact, err)
	}
	path := filepath.Join(repo, ".kkachi", "runs", runID, filepath.FromSlash(artifact))
	mustWriteFile(t, path, append(data, '\n'))
}

func TestCheckGateImplementationPassesAndFails(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	created, err := CreateRun(root, deterministicCreateRunOptions())
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	if _, err := InitArtifacts(root, ArtifactInitOptions{RunID: created.Metadata.RunID, Now: testRunNow(4)}); err != nil {
		t.Fatalf("InitArtifacts() error = %v", err)
	}

	// diff.patch baseline is non-empty, so only impl_log fails.
	failed, err := CheckGate(root, GateCheckOptions{RunID: created.Metadata.RunID, Gate: GateImplementation, Now: testRunNow(5)})
	if err != nil {
		t.Fatalf("CheckGate(implementation fail) error = %v", err)
	}
	if failed.Status != GateStatusFail || failed.EventID != "evt-000004" || !gateCheckStatus(failed.Checks, "impl_log", GateStatusFail) {
		t.Fatalf("failed = %#v, want impl_log failure", failed)
	}

	writeMarkdownArtifact(t, repo, created.Metadata.RunID, "impl-log.md", "Status: complete\nImplementation: done\n")
	passed, err := CheckGate(root, GateCheckOptions{RunID: created.Metadata.RunID, Gate: GateImplementation, Now: testRunNow(6)})
	if err != nil {
		t.Fatalf("CheckGate(implementation pass) error = %v", err)
	}
	if passed.Status != GateStatusPass || passed.EventID != "evt-000005" || !gateCheckStatus(passed.Checks, "diff_patch", GateStatusPass) || !gateCheckStatus(passed.Checks, "impl_log", GateStatusPass) {
		t.Fatalf("passed = %#v, want implementation pass", passed)
	}
}

func TestCheckGateImplementationRequiresCliOutputWhenManifestRequires(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	created := createAdapterQARunWithArtifacts(t, root)
	writeDiffPatch(t, repo, created.Metadata.RunID)
	writeMarkdownArtifact(t, repo, created.Metadata.RunID, "impl-log.md", "Status: complete\nImplementation: done\n")

	result, err := CheckGate(root, GateCheckOptions{RunID: created.Metadata.RunID, Gate: GateImplementation, Now: testRunNow(5)})
	if err != nil {
		t.Fatalf("CheckGate(implementation adapter_qa) error = %v", err)
	}
	if result.Status != GateStatusFail || !gateCheckStatus(result.Checks, "cli_output", GateStatusFail) {
		t.Fatalf("result = %#v, want cli_output failure when required by manifest", result)
	}

	writeMarkdownArtifact(t, repo, created.Metadata.RunID, "cli-output.md", "Status: complete\nOutput: captured\n")
	passed, err := CheckGate(root, GateCheckOptions{RunID: created.Metadata.RunID, Gate: GateImplementation, Now: testRunNow(6)})
	if err != nil {
		t.Fatalf("CheckGate(implementation adapter_qa pass) error = %v", err)
	}
	if passed.Status != GateStatusPass || !gateCheckStatus(passed.Checks, "cli_output", GateStatusPass) {
		t.Fatalf("passed = %#v, want cli_output pass", passed)
	}
}

func TestCheckGateReviewPassesAndFails(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	options := deterministicCreateRunOptions()
	options.Redteam = "red-team-alpha"
	created, err := CreateRun(root, options)
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	if _, err := InitArtifacts(root, ArtifactInitOptions{RunID: created.Metadata.RunID, Now: testRunNow(4)}); err != nil {
		t.Fatalf("InitArtifacts() error = %v", err)
	}

	failed, err := CheckGate(root, GateCheckOptions{RunID: created.Metadata.RunID, Gate: GateReview, Now: testRunNow(5)})
	if err != nil {
		t.Fatalf("CheckGate(review fail) error = %v", err)
	}
	if failed.Status != GateStatusFail || !gateCheckStatus(failed.Checks, "review", GateStatusFail) || !gateCheckStatus(failed.Checks, "redteam_plan-review", GateStatusFail) {
		t.Fatalf("failed = %#v, want review and redteam failures", failed)
	}

	writeMarkdownArtifact(t, repo, created.Metadata.RunID, "review.md", "Status: complete\nReview: approved\n")
	writeMarkdownArtifact(t, repo, created.Metadata.RunID, "redteam/plan-review.md", "Status: complete\nReview: no issues\n")
	writeMarkdownArtifact(t, repo, created.Metadata.RunID, "redteam/shaping-review.md", "Status: complete\nReview: no issues\n")
	writeMarkdownArtifact(t, repo, created.Metadata.RunID, "redteam/impl-review.md", "Status: complete\nReview: no issues\n")
	writeMarkdownArtifact(t, repo, created.Metadata.RunID, "redteam/test-review.md", "Status: complete\nReview: no issues\n")
	writeMarkdownArtifact(t, repo, created.Metadata.RunID, "redteam/final-gate-review.md", "Status: complete\nReview: no issues\n")
	passed, err := CheckGate(root, GateCheckOptions{RunID: created.Metadata.RunID, Gate: GateReview, Now: testRunNow(6)})
	if err != nil {
		t.Fatalf("CheckGate(review pass) error = %v", err)
	}
	if passed.Status != GateStatusPass || passed.EventID != "evt-000005" || !gateCheckStatus(passed.Checks, "review", GateStatusPass) || !gateCheckStatus(passed.Checks, "redteam_plan-review", GateStatusPass) {
		t.Fatalf("passed = %#v, want review pass", passed)
	}
}

func TestCheckGateVerificationPassesAndFails(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	created, err := CreateRun(root, deterministicCreateRunOptions())
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	if _, err := InitArtifacts(root, ArtifactInitOptions{RunID: created.Metadata.RunID, Now: testRunNow(4)}); err != nil {
		t.Fatalf("InitArtifacts() error = %v", err)
	}

	failed, err := CheckGate(root, GateCheckOptions{RunID: created.Metadata.RunID, Gate: GateVerification, Now: testRunNow(5)})
	if err != nil {
		t.Fatalf("CheckGate(verification fail) error = %v", err)
	}
	if failed.Status != GateStatusFail || !gateCheckStatus(failed.Checks, "test_log", GateStatusFail) || !gateCheckStatus(failed.Checks, "verification", GateStatusFail) {
		t.Fatalf("failed = %#v, want test_log and verification failure", failed)
	}

	writeMarkdownArtifact(t, repo, created.Metadata.RunID, "test-log.md", "Status: complete\nTests: all pass\n")
	writeMarkdownArtifact(t, repo, created.Metadata.RunID, "verification.md", "Status: complete\nVerdict: pass\n")
	passed, err := CheckGate(root, GateCheckOptions{RunID: created.Metadata.RunID, Gate: GateVerification, Now: testRunNow(6)})
	if err != nil {
		t.Fatalf("CheckGate(verification pass) error = %v", err)
	}
	if passed.Status != GateStatusPass || passed.EventID != "evt-000005" || !gateCheckStatus(passed.Checks, "test_log", GateStatusPass) || !gateCheckStatus(passed.Checks, "verification", GateStatusPass) {
		t.Fatalf("passed = %#v, want verification pass", passed)
	}

	writeMarkdownArtifact(t, repo, created.Metadata.RunID, "verification.md", "Status: complete\nVerdict: fail\n")
	failVerdict, err := CheckGate(root, GateCheckOptions{RunID: created.Metadata.RunID, Gate: GateVerification, Now: testRunNow(7)})
	if err != nil {
		t.Fatalf("CheckGate(verification fail verdict) error = %v", err)
	}
	if failVerdict.Status != GateStatusPass || !gateCheckStatus(failVerdict.Checks, "verification", GateStatusPass) || !gateCheckActual(failVerdict.Checks, "verification", "fail") {
		t.Fatalf("failVerdict = %#v, want verification pass with fail verdict", failVerdict)
	}

	writeMarkdownArtifact(t, repo, created.Metadata.RunID, "verification.md", "Status: complete\nVerdict: unknown\n")
	invalidVerdict, err := CheckGate(root, GateCheckOptions{RunID: created.Metadata.RunID, Gate: GateVerification, Now: testRunNow(8)})
	if err != nil {
		t.Fatalf("CheckGate(verification invalid verdict) error = %v", err)
	}
	if invalidVerdict.Status != GateStatusFail || !gateCheckStatus(invalidVerdict.Checks, "verification", GateStatusFail) || !gateCheckActual(invalidVerdict.Checks, "verification", "unknown") {
		t.Fatalf("invalidVerdict = %#v, want verification failure with invalid verdict", invalidVerdict)
	}
}

func TestCheckGateDocsPassesAndFails(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	created, err := CreateRun(root, deterministicCreateRunOptions())
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	if _, err := InitArtifacts(root, ArtifactInitOptions{RunID: created.Metadata.RunID, Now: testRunNow(4)}); err != nil {
		t.Fatalf("InitArtifacts() error = %v", err)
	}

	failed, err := CheckGate(root, GateCheckOptions{RunID: created.Metadata.RunID, Gate: GateDocs, Now: testRunNow(5)})
	if err != nil {
		t.Fatalf("CheckGate(docs fail) error = %v", err)
	}
	if failed.Status != GateStatusFail || !gateCheckStatus(failed.Checks, "docs_update", GateStatusFail) {
		t.Fatalf("failed = %#v, want docs_update failure", failed)
	}

	writeMarkdownArtifact(t, repo, created.Metadata.RunID, "docs-update.md", "Status: complete\nChanged Docs: README.md\n")
	passed, err := CheckGate(root, GateCheckOptions{RunID: created.Metadata.RunID, Gate: GateDocs, Now: testRunNow(6)})
	if err != nil {
		t.Fatalf("CheckGate(docs pass) error = %v", err)
	}
	if passed.Status != GateStatusPass || passed.EventID != "evt-000005" || !gateCheckStatus(passed.Checks, "docs_update", GateStatusPass) {
		t.Fatalf("passed = %#v, want docs pass", passed)
	}

	writeMarkdownArtifact(t, repo, created.Metadata.RunID, "docs-update.md", "Status: complete\nNo Change Reason: no user-visible changes\n")
	noChange, err := CheckGate(root, GateCheckOptions{RunID: created.Metadata.RunID, Gate: GateDocs, Now: testRunNow(7)})
	if err != nil {
		t.Fatalf("CheckGate(docs no change) error = %v", err)
	}
	if noChange.Status != GateStatusPass || !gateCheckStatus(noChange.Checks, "docs_update", GateStatusPass) {
		t.Fatalf("noChange = %#v, want docs no-change pass", noChange)
	}
}

func TestCheckGateFinalPassesAndFails(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	created, err := CreateRun(root, deterministicCreateRunOptions())
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	if _, err := InitArtifacts(root, ArtifactInitOptions{RunID: created.Metadata.RunID, Now: testRunNow(4)}); err != nil {
		t.Fatalf("InitArtifacts() error = %v", err)
	}

	// All prior gates unchecked: final should fail on missing gate states.
	failed, err := CheckGate(root, GateCheckOptions{RunID: created.Metadata.RunID, Gate: GateFinal, Now: testRunNow(5)})
	if err != nil {
		t.Fatalf("CheckGate(final fail unchecked) error = %v", err)
	}
	if failed.Status != GateStatusFail || failed.EventID != "evt-000004" || !gateCheckStatus(failed.Checks, "final_report", GateStatusFail) || !gateCheckStatus(failed.Checks, "intake_gate", GateStatusFail) {
		t.Fatalf("failed = %#v, want final_report and intake_gate failure", failed)
	}

	// Pass all required gates and final-report.
	passAllPriorGates(t, root, repo, created.Metadata.RunID)
	writeMarkdownArtifact(t, repo, created.Metadata.RunID, "final-report.md", "Status: complete\nReport: done\n")
	passed, err := CheckGate(root, GateCheckOptions{RunID: created.Metadata.RunID, Gate: GateFinal, Now: testRunNow(15)})
	if err != nil {
		t.Fatalf("CheckGate(final pass) error = %v", err)
	}
	if passed.Status != GateStatusPass || passed.EventID != "evt-000013" || !gateCheckStatus(passed.Checks, "final_report", GateStatusPass) || !gateCheckStatus(passed.Checks, "intake_gate", GateStatusPass) {
		t.Fatalf("passed = %#v, want final pass", passed)
	}

	// Fail one prior gate and verify final fails.
	metadata := readRunMetadata(t, repo, created.Metadata.RunID)
	metadata.GateState[GatePlan] = map[string]any{"status": GateStatusFail, "event_id": "evt-000009"}
	writeRunMetadataForTest(t, repo, metadata)
	writeMarkdownArtifact(t, repo, created.Metadata.RunID, "final-report.md", "Status: complete\nReport: done\n")
	planFail, err := CheckGate(root, GateCheckOptions{RunID: created.Metadata.RunID, Gate: GateFinal, Now: testRunNow(16)})
	if err != nil {
		t.Fatalf("CheckGate(final plan fail) error = %v", err)
	}
	if planFail.Status != GateStatusFail || !gateCheckStatus(planFail.Checks, "plan_gate", GateStatusFail) {
		t.Fatalf("planFail = %#v, want plan_gate failure", planFail)
	}
}

func TestCheckGateFinalRequiresBackendGateWhenManifestRequires(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	created := createAdapterQARunWithArtifacts(t, root)
	writeValidBackendEvidence(t, repo, created.Metadata.RunID)

	passAllPriorGates(t, root, repo, created.Metadata.RunID, GateBackend)
	// backend gate is not passed; final should fail because backend is required.
	writeMarkdownArtifact(t, repo, created.Metadata.RunID, "final-report.md", "Status: complete\nReport: done\n")
	result, err := CheckGate(root, GateCheckOptions{RunID: created.Metadata.RunID, Gate: GateFinal, Now: testRunNow(15)})
	if err != nil {
		t.Fatalf("CheckGate(final backend missing) error = %v", err)
	}
	if result.Status != GateStatusFail || !gateCheckStatus(result.Checks, "backend_gate", GateStatusFail) {
		t.Fatalf("result = %#v, want backend_gate failure", result)
	}

	// Now pass backend gate and retry final.
	_, err = CheckGate(root, GateCheckOptions{RunID: created.Metadata.RunID, Gate: GateBackend, Now: testRunNow(16)})
	if err != nil {
		t.Fatalf("CheckGate(backend) error = %v", err)
	}
	writeMarkdownArtifact(t, repo, created.Metadata.RunID, "final-report.md", "Status: complete\nReport: done\n")
	passed, err := CheckGate(root, GateCheckOptions{RunID: created.Metadata.RunID, Gate: GateFinal, Now: testRunNow(17)})
	if err != nil {
		t.Fatalf("CheckGate(final pass with backend) error = %v", err)
	}
	if passed.Status != GateStatusPass || !gateCheckStatus(passed.Checks, "backend_gate", GateStatusPass) {
		t.Fatalf("passed = %#v, want final pass with backend", passed)
	}
}

func passAllPriorGates(t *testing.T, root Root, repo string, runID string, skip ...string) {
	t.Helper()
	skipSet := stringSet(skip)
	metadata := readRunMetadata(t, repo, runID)
	writeIntakeClassification(t, repo, metadata, "")
	writeMarkdownArtifact(t, repo, runID, "sot-basis.md", "Status: complete\nSource: docs/specs.md\n")
	writeMarkdownArtifact(t, repo, runID, "roadmap-update.md", "Status: complete\nTrace: docs/roadmap.md\n")
	writeCompletedPlanArtifacts(t, repo, runID)
	writeDiffPatch(t, repo, runID)
	writeMarkdownArtifact(t, repo, runID, "impl-log.md", "Status: complete\nImplementation: done\n")
	writeMarkdownArtifact(t, repo, runID, "review.md", "Status: complete\nReview: approved\n")
	writeMarkdownArtifact(t, repo, runID, "test-log.md", "Status: complete\nTests: all pass\n")
	writeMarkdownArtifact(t, repo, runID, "verification.md", "Status: complete\nVerdict: pass\n")
	writeMarkdownArtifact(t, repo, runID, "docs-update.md", "Status: complete\nChanged Docs: README.md\n")

	required := ArtifactManifest(metadata)
	requiredSet := stringSet(required)
	if requiredSet["cli-output.md"] {
		writeMarkdownArtifact(t, repo, runID, "cli-output.md", "Status: complete\nOutput: captured\n")
	}
	if requiredSet["redteam/impl-review.md"] {
		writeMarkdownArtifact(t, repo, runID, "redteam/impl-review.md", "Status: complete\nReview: no issues\n")
	}
	if requiredSet["redteam/test-review.md"] {
		writeMarkdownArtifact(t, repo, runID, "redteam/test-review.md", "Status: complete\nReview: no issues\n")
	}
	if requiredSet["redteam/qa-review.md"] {
		writeMarkdownArtifact(t, repo, runID, "redteam/qa-review.md", "Status: complete\nReview: no issues\n")
	}
	if requiredSet["redteam/plan-review.md"] {
		writeMarkdownArtifact(t, repo, runID, "redteam/plan-review.md", "Status: complete\nReview: no issues\n")
	}
	if requiredSet["redteam/shaping-review.md"] {
		writeMarkdownArtifact(t, repo, runID, "redteam/shaping-review.md", "Status: complete\nReview: no issues\n")
	}
	if requiredSet["redteam/final-gate-review.md"] {
		writeMarkdownArtifact(t, repo, runID, "redteam/final-gate-review.md", "Status: complete\nReview: no issues\n")
	}

	gates := []string{GateIntake, GateSOT, GateRoadmap, GatePlan, GateImplementation, GateReview, GateVerification, GateDocs}
	if backendArtifactsRequired(required) && !skipSet[GateBackend] {
		gates = append(gates, GateBackend)
	}
	for i, gate := range gates {
		_, err := CheckGate(root, GateCheckOptions{RunID: runID, Gate: gate, Now: testRunNow(6 + i)})
		if err != nil {
			t.Fatalf("CheckGate(%s) error = %v", gate, err)
		}
	}
}

func writeDiffPatch(t *testing.T, repo string, runID string) {
	t.Helper()
	path := filepath.Join(repo, ".kkachi", "runs", runID, "diff.patch")
	mustWriteFile(t, path, []byte("diff --git a/file.txt b/file.txt\n+change\n"))
}

func TestCheckGateReviewWithoutRedteam(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	options := deterministicCreateRunOptions()
	options.Redteam = ""
	options.ExecutionMode = "research"
	created, err := CreateRun(root, options)
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	if _, err := InitArtifacts(root, ArtifactInitOptions{RunID: created.Metadata.RunID, Now: testRunNow(4)}); err != nil {
		t.Fatalf("InitArtifacts() error = %v", err)
	}

	failed, err := CheckGate(root, GateCheckOptions{RunID: created.Metadata.RunID, Gate: GateReview, Now: testRunNow(5)})
	if err != nil {
		t.Fatalf("CheckGate(review fail) error = %v", err)
	}
	if failed.Status != GateStatusFail || !gateCheckStatus(failed.Checks, "review", GateStatusFail) {
		t.Fatalf("failed = %#v, want review failure", failed)
	}
	for _, check := range failed.Checks {
		if strings.HasPrefix(check.Name, "redteam_") {
			t.Fatalf("unexpected redteam check when redteam is not assigned and execution_mode is research: %s", check.Name)
		}
	}

	writeMarkdownArtifact(t, repo, created.Metadata.RunID, "review.md", "Status: complete\nReview: approved\n")
	passed, err := CheckGate(root, GateCheckOptions{RunID: created.Metadata.RunID, Gate: GateReview, Now: testRunNow(6)})
	if err != nil {
		t.Fatalf("CheckGate(review pass) error = %v", err)
	}
	if passed.Status != GateStatusPass || !gateCheckStatus(passed.Checks, "review", GateStatusPass) {
		t.Fatalf("passed = %#v, want review pass", passed)
	}
}

func TestCheckGateFinalFailsMissingFinalReportOnly(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	created, err := CreateRun(root, deterministicCreateRunOptions())
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	if _, err := InitArtifacts(root, ArtifactInitOptions{RunID: created.Metadata.RunID, Now: testRunNow(4)}); err != nil {
		t.Fatalf("InitArtifacts() error = %v", err)
	}

	passAllPriorGates(t, root, repo, created.Metadata.RunID)

	result, err := CheckGate(root, GateCheckOptions{RunID: created.Metadata.RunID, Gate: GateFinal, Now: testRunNow(15)})
	if err != nil {
		t.Fatalf("CheckGate(final) error = %v", err)
	}
	if result.Status != GateStatusFail || !gateCheckStatus(result.Checks, "final_report", GateStatusFail) {
		t.Fatalf("result = %#v, want final_report failure", result)
	}
	for _, gate := range []string{GateIntake, GateSOT, GateRoadmap, GatePlan, GateImplementation, GateReview, GateVerification, GateDocs} {
		if !gateCheckStatus(result.Checks, gate+"_gate", GateStatusPass) {
			t.Fatalf("result = %#v, want %s_gate pass", result, gate)
		}
	}
}

func TestCheckGateWritesRunLocalReport(t *testing.T) {
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
	wantPath := filepath.ToSlash(filepath.Join(".kkachi", "runs", created.Metadata.RunID, "gate-reports", "intake.json"))
	if failed.ReportPath != wantPath {
		t.Fatalf("ReportPath = %q, want %q", failed.ReportPath, wantPath)
	}
	var report gateReport
	readJSONFile(t, filepath.Join(repo, failed.ReportPath), &report)
	if report.RunID != created.Metadata.RunID || report.Gate != GateIntake || report.Status != GateStatusFail || report.EventID != "evt-000004" || report.GeneratedAt != "2026-04-30T01:02:05Z" || report.ReportPath != wantPath {
		t.Fatalf("report = %#v, want failed intake report with event/time/path", report)
	}
	if len(report.MissingEvidence) == 0 || !gateCheckStatus(report.Checks, "intake_status", GateStatusFail) {
		t.Fatalf("report = %#v, want failed checks and missing evidence", report)
	}

	writeIntakeClassification(t, repo, created.Metadata, "")
	passed, err := CheckGate(root, GateCheckOptions{RunID: created.Metadata.RunID, Gate: GateIntake, Now: testRunNow(6)})
	if err != nil {
		t.Fatalf("CheckGate(pass) error = %v", err)
	}
	if passed.ReportPath != wantPath {
		t.Fatalf("passed ReportPath = %q, want %q", passed.ReportPath, wantPath)
	}
	readJSONFile(t, filepath.Join(repo, passed.ReportPath), &report)
	if report.Status != GateStatusPass || report.EventID != "evt-000005" || len(report.MissingEvidence) != 0 || !gateCheckStatus(report.Checks, "intake_status", GateStatusPass) {
		t.Fatalf("report = %#v, want overwritten passing intake report", report)
	}
	metadata := readRunMetadata(t, repo, created.Metadata.RunID)
	state := metadata.GateState[GateIntake].(map[string]any)
	if state["report_path"] != wantPath {
		t.Fatalf("gate_state report_path = %#v, want %q", state["report_path"], wantPath)
	}
	var status map[string]any
	readJSONFile(t, filepath.Join(repo, ".kkachi", "status.json"), &status)
	summary := status["gate_summary"].(map[string]any)[GateIntake].(map[string]any)
	if summary["report_path"] != wantPath {
		t.Fatalf("status gate_summary report_path = %#v, want %q", summary["report_path"], wantPath)
	}
}

type gateRegressionFixture struct {
	Name              string `json:"name"`
	WorkPath          string `json:"work_path"`
	WorkMode          string `json:"work_mode"`
	SOTPolicy         string `json:"sot_policy"`
	ExecutionMode     string `json:"execution_mode"`
	Gate              string `json:"gate"`
	WantStatus        string `json:"want_status"`
	MissingArtifact   string `json:"missing_artifact"`
	MalformedArtifact string `json:"malformed_artifact"`
	MalformedBody     string `json:"malformed_body"`
	WantCheck         string `json:"want_check"`
}

func TestGateRegressionFixturesPathModeMatrix(t *testing.T) {
	fixtures := loadGateRegressionFixtures(t)
	seen := map[string]map[string]bool{}
	for _, fixture := range fixtures {
		fixture := fixture
		t.Run(fixture.Name, func(t *testing.T) {
			fixtureKey := fixture.WorkPath + "/" + fixture.WorkMode
			if seen[fixtureKey] == nil {
				seen[fixtureKey] = map[string]bool{}
			}
			seen[fixtureKey][fixture.WantStatus] = true

			repo := initializedRepo(t)
			root, _ := DiscoverRoot(repo)
			options := deterministicCreateRunOptions()
			options.TaskID = "gates-005"
			options.WorkPath = fixture.WorkPath
			options.WorkMode = fixture.WorkMode
			options.SOTPolicy = fixture.SOTPolicy
			options.ExecutionMode = fixture.ExecutionMode
			options.Redteam = ""
			created, err := CreateRun(root, options)
			if err != nil {
				t.Fatalf("CreateRun() error = %v", err)
			}
			if _, err := InitArtifacts(root, ArtifactInitOptions{RunID: created.Metadata.RunID, Now: testRunNow(4)}); err != nil {
				t.Fatalf("InitArtifacts() error = %v", err)
			}
			metadata := readRunMetadata(t, repo, created.Metadata.RunID)
			if fixture.Gate == GateRoadmap && fixture.MalformedArtifact == "roadmap-update.md" {
				metadata.TaskID = nil
				writeRunMetadataForTest(t, repo, metadata)
			}
			writeCompleteGateFixtureArtifacts(t, repo, metadata)
			if fixture.MissingArtifact != "" {
				if err := os.Remove(filepath.Join(repo, ".kkachi", "runs", metadata.RunID, filepath.FromSlash(fixture.MissingArtifact))); err != nil {
					t.Fatalf("remove missing fixture artifact: %v", err)
				}
			}
			if fixture.MalformedArtifact != "" {
				mustWriteFile(t, filepath.Join(repo, ".kkachi", "runs", metadata.RunID, filepath.FromSlash(fixture.MalformedArtifact)), []byte(fixture.MalformedBody))
			}

			if fixture.Gate == GateFinal && fixture.WantStatus == GateStatusPass {
				passFixturePriorGates(t, root, metadata.RunID)
			}
			result, err := CheckGate(root, GateCheckOptions{RunID: metadata.RunID, Gate: fixture.Gate, Now: testRunNow(20)})
			if err != nil {
				t.Fatalf("CheckGate(%s) error = %v", fixture.Gate, err)
			}
			if result.Status != fixture.WantStatus {
				t.Fatalf("result = %#v, want status %s", result, fixture.WantStatus)
			}
			if fixture.WantCheck != "" && !gateCheckStatus(result.Checks, fixture.WantCheck, fixture.WantStatus) {
				t.Fatalf("checks = %#v, want %s %s", result.Checks, fixture.WantCheck, fixture.WantStatus)
			}
			if result.ReportPath == "" {
				t.Fatalf("result missing report path: %#v", result)
			}
			var report gateReport
			readJSONFile(t, filepath.Join(repo, result.ReportPath), &report)
			if report.Status != fixture.WantStatus || report.Gate != fixture.Gate || report.EventID != result.EventID || report.ReportPath != result.ReportPath {
				t.Fatalf("report = %#v, want status/gate/event/path from result %#v", report, result)
			}
		})
	}

	for _, key := range []string{"A_development_execution/standard", "A_development_execution/light", "B_discovery_shaping/standard", "B_discovery_shaping/light"} {
		if !seen[key][GateStatusPass] || !seen[key][GateStatusFail] {
			t.Fatalf("fixtures for %s = %#v, want valid and invalid coverage", key, seen[key])
		}
	}
}

func loadGateRegressionFixtures(t *testing.T) []gateRegressionFixture {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "gates-005", "scenarios.json"))
	if err != nil {
		t.Fatalf("read gates-005 fixtures: %v", err)
	}
	var fixtures []gateRegressionFixture
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatalf("decode gates-005 fixtures: %v", err)
	}
	if len(fixtures) == 0 {
		t.Fatal("gates-005 fixtures are empty")
	}
	return fixtures
}

func writeCompleteGateFixtureArtifacts(t *testing.T, repo string, metadata RunMetadata) {
	t.Helper()
	extra := ""
	if metadata.WorkMode == "light" {
		extra = "Light Mode Reason: fixture covers a low-risk abbreviated run\n"
	}
	writeIntakeClassification(t, repo, metadata, extra)
	if metadata.WorkPath == "B_discovery_shaping" {
		writeMarkdownArtifact(t, repo, metadata.RunID, "sot-update.md", "Status: complete\nSOT: created before handoff\n")
	} else {
		writeMarkdownArtifact(t, repo, metadata.RunID, "sot-basis.md", "Status: complete\nSource: fixture SOT\n")
	}
	writeMarkdownArtifact(t, repo, metadata.RunID, "roadmap-update.md", "Status: complete\nTrace: gates-005\n")
	writeCompletedPlanArtifacts(t, repo, metadata.RunID)
	writeDiffPatch(t, repo, metadata.RunID)
	writeMarkdownArtifact(t, repo, metadata.RunID, "impl-log.md", "Status: complete\nImplementation: fixture\n")
	writeMarkdownArtifact(t, repo, metadata.RunID, "review.md", "Status: complete\nReview: fixture approved\n")
	writeMarkdownArtifact(t, repo, metadata.RunID, "test-log.md", "Status: complete\nTests: fixture pass\n")
	writeMarkdownArtifact(t, repo, metadata.RunID, "verification.md", "Status: complete\nVerdict: pass\n")
	writeMarkdownArtifact(t, repo, metadata.RunID, "docs-update.md", "Status: complete\nNo Change Reason: fixture only\n")
	writeMarkdownArtifact(t, repo, metadata.RunID, "final-report.md", "Status: complete\nReport: fixture complete\n")
	for _, artifact := range metadata.RequiredArtifacts {
		if strings.HasPrefix(artifact, "redteam/") {
			writeMarkdownArtifact(t, repo, metadata.RunID, artifact, "Status: complete\nReview: fixture no issues\n")
		}
	}
}

func passFixturePriorGates(t *testing.T, root Root, runID string) {
	t.Helper()
	for i, gate := range []string{GateIntake, GateSOT, GateRoadmap, GatePlan, GateImplementation, GateReview, GateVerification, GateDocs} {
		result, err := CheckGate(root, GateCheckOptions{RunID: runID, Gate: gate, Now: testRunNow(10 + i)})
		if err != nil {
			t.Fatalf("CheckGate(%s) error = %v", gate, err)
		}
		if result.Status != GateStatusPass {
			t.Fatalf("CheckGate(%s) = %#v, want pass before final", gate, result)
		}
	}
}

func TestGateReportPathRejectsUnsafeGateNames(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	for _, gate := range []string{"../intake", "nested/gate", `bad\\gate`, `bad:gate`, `bad?gate`, `bad<gate`} {
		if _, err := gateReportPath(root, "run-20260430T010203Z-abcdef123456", gate); err == nil {
			t.Fatalf("gateReportPath(%q) error = nil, want unsafe gate rejection", gate)
		} else {
			assertProblemCode(t, err, "gate_report_path_invalid")
		}
	}
}
