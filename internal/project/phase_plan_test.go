package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitShowAndSetPhasePlan(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	created, err := CreateRun(root, deterministicCreateRunOptions())
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}

	initialized, err := InitPhasePlan(root, PhasePlanInitOptions{RunID: created.Metadata.RunID, Now: testRunNow(4)})
	if err != nil {
		t.Fatalf("InitPhasePlan() error = %v", err)
	}
	if initialized.EventID != "evt-000003" || initialized.Plan.RunID != created.Metadata.RunID || len(initialized.Plan.Phases) == 0 {
		t.Fatalf("initialized = %#v, want event and phases", initialized)
	}
	path := filepath.Join(repo, ".kkachi", "runs", created.Metadata.RunID, "phase-plan.yaml")
	if got := readText(t, path); !strings.Contains(got, `id: "ask"`) || !strings.Contains(got, `id: "handle-feedback-1"`) {
		t.Fatalf("phase-plan.yaml = %q, want required phases", got)
	}

	updated, err := SetPhasePlanPhase(root, PhasePlanSetOptions{RunID: created.Metadata.RunID, PhaseID: "ask", Status: PhaseStatusNotApplicable, Reason: "No actionable clarification needed.", Now: testRunNow(5)})
	if err != nil {
		t.Fatalf("SetPhasePlanPhase() error = %v", err)
	}
	if updated.EventID != "evt-000004" || updated.Phase.Status != PhaseStatusNotApplicable || updated.Phase.Reason == "" {
		t.Fatalf("updated = %#v, want not-applicable ask phase", updated)
	}
	shown, err := ShowPhasePlan(root, created.Metadata.RunID[:20])
	if err != nil {
		t.Fatalf("ShowPhasePlan() error = %v", err)
	}
	if shown.Phases[4].ID != "ask" || shown.Phases[4].Reason == "" {
		t.Fatalf("shown ask phase = %#v, want persisted reason", shown.Phases[4])
	}
	if lines := runEventLines(t, repo); len(lines) != 4 || !strings.Contains(lines[2], `"phase_plan.initialized"`) || !strings.Contains(lines[3], `"phase_plan.updated"`) {
		t.Fatalf("events = %#v, want phase plan events", lines)
	}
}

func TestValidatePhasePlanRequiresReasonsFeedbackAndFinalEvidence(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	created, err := CreateRun(root, deterministicCreateRunOptions())
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	if _, err := InitPhasePlan(root, PhasePlanInitOptions{RunID: created.Metadata.RunID, Now: testRunNow(4)}); err != nil {
		t.Fatalf("InitPhasePlan() error = %v", err)
	}
	runID := created.Metadata.RunID
	writePlan := func(content string) {
		t.Helper()
		path := filepath.Join(repo, ".kkachi", "runs", runID, "phase-plan.yaml")
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("write phase plan: %v", err)
		}
	}

	writePlan(`version: "0.1"
run_id: "` + runID + `"
phases:
  - id: "intake"
    status: "skipped"
  - id: "sot"
    status: "complete"
    evidence: "sot-basis.md"
  - id: "roadmap"
    status: "complete"
    evidence: "run-metadata.json"
  - id: "plan"
    status: "complete"
    evidence: "plan.md"
  - id: "ask"
    status: "not_applicable"
    reason: "No question."
  - id: "implement"
    status: "complete"
    evidence: "diff.patch"
  - id: "optimize"
    status: "not_applicable"
    reason: "Optimization was not needed."
  - id: "request-feedback-4"
    status: "not_applicable"
    reason: "Out of range."
  - id: "request-feedback-1"
    status: "not_applicable"
    reason: "No feedback."
  - id: "review"
    status: "complete"
  - id: "verify"
    status: "pending"
  - id: "docs"
    status: "complete"
    evidence: "docs-update.md"
  - id: "final"
    status: "complete"
    evidence: "final-report.md"
`)
	result, err := ValidatePhasePlan(root, PhasePlanValidationOptions{RunID: runID, Final: true})
	if err != nil {
		t.Fatalf("ValidatePhasePlan() error = %v", err)
	}
	if result.Status != PhasePlanStatusFail || !phaseCheckFailed(result.Checks, "skip_reason") || !phaseCheckFailed(result.Checks, "feedback_round_range") || !phaseCheckFailed(result.Checks, "feedback_pairs") || !phaseCheckFailed(result.Checks, "final_terminal_states") || !phaseCheckFailed(result.Checks, "final_evidence_links") {
		t.Fatalf("checks = %#v, want reason/feedback/final failures", result.Checks)
	}

	writePlan(`version: "0.1"
run_id: "` + runID + `"
phases:
  - id: "intake"
    status: "complete"
    evidence: "intake.md"
  - id: "sot"
    status: "complete"
    evidence: "sot-basis.md"
  - id: "roadmap"
    status: "complete"
    evidence: "run-metadata.json"
  - id: "plan"
    status: "complete"
    evidence: "plan.md"
  - id: "ask"
    status: "not_applicable"
    reason: "No question."
  - id: "implement"
    status: "complete"
    evidence: "diff.patch"
  - id: "optimize"
    status: "not_applicable"
    reason: "Optimization was not needed."
  - id: "request-feedback-1"
    status: "not_applicable"
    reason: "No feedback."
  - id: "handle-feedback-1"
    status: "not_applicable"
    reason: "No feedback."
  - id: "handle-feedback-2"
    status: "not_applicable"
    reason: "Unpaired reverse case."
  - id: "review"
    status: "complete"
    evidence: "review.md"
  - id: "verify"
    status: "complete"
    evidence: "verify.md"
  - id: "docs"
    status: "complete"
    evidence: "docs-update.md"
  - id: "final"
    status: "complete"
    evidence: "final-report.md"
`)
	result, err = ValidatePhasePlan(root, PhasePlanValidationOptions{RunID: runID})
	if err != nil {
		t.Fatalf("ValidatePhasePlan() reverse feedback error = %v", err)
	}
	if result.Status != PhasePlanStatusFail || !phaseCheckFailed(result.Checks, "feedback_pairs") {
		t.Fatalf("checks = %#v, want reverse feedback pair failure", result.Checks)
	}
}

func TestPhasePlanRoundTripWithSpecialCharacters(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	created, err := CreateRun(root, deterministicCreateRunOptions())
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	if _, err := InitPhasePlan(root, PhasePlanInitOptions{RunID: created.Metadata.RunID, Now: testRunNow(4)}); err != nil {
		t.Fatalf("InitPhasePlan() error = %v", err)
	}
	reason := `KHS said "skip": see #123, null, true, path C:\tmp\phase`
	evidence := `logs/phase:ask#evidence-"quoted"\path.md`
	_, err = SetPhasePlanPhase(root, PhasePlanSetOptions{
		RunID:    created.Metadata.RunID,
		PhaseID:  "ask",
		Status:   PhaseStatusNotApplicable,
		Evidence: evidence,
		Reason:   reason,
		Now:      testRunNow(5),
	})
	if err != nil {
		t.Fatalf("SetPhasePlanPhase() error = %v", err)
	}

	path := filepath.Join(repo, ".kkachi", "runs", created.Metadata.RunID, "phase-plan.yaml")
	raw := readText(t, path)
	if !strings.Contains(raw, `reason: "KHS said \"skip\": see #123, null, true, path C:\\tmp\\phase"`) || !strings.Contains(raw, `evidence: "logs/phase:ask#evidence-\"quoted\"\\path.md"`) {
		t.Fatalf("phase-plan.yaml = %q, want quoted scalar escapes", raw)
	}
	shown, err := ShowPhasePlan(root, created.Metadata.RunID)
	if err != nil {
		t.Fatalf("ShowPhasePlan() error = %v", err)
	}
	ask := shown.Phases[4]
	if ask.ID != "ask" || ask.Reason != reason || ask.Evidence != evidence {
		t.Fatalf("ask phase = %#v, want reason %q and evidence %q", ask, reason, evidence)
	}
}

func TestPhasePlanRejectsInvalidShapeAndDuplicateInit(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	created, err := CreateRun(root, deterministicCreateRunOptions())
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	if _, err := InitPhasePlan(root, PhasePlanInitOptions{RunID: created.Metadata.RunID, Now: testRunNow(4)}); err != nil {
		t.Fatalf("InitPhasePlan() error = %v", err)
	}
	_, err = InitPhasePlan(root, PhasePlanInitOptions{RunID: created.Metadata.RunID, Now: testRunNow(5)})
	assertProblemCode(t, err, "phase_plan_exists")

	_, err = SetPhasePlanPhase(root, PhasePlanSetOptions{RunID: created.Metadata.RunID, PhaseID: "ask", Status: "done", Now: testRunNow(6)})
	assertProblemCode(t, err, "phase_status_invalid")
	_, err = SetPhasePlanPhase(root, PhasePlanSetOptions{RunID: created.Metadata.RunID, PhaseID: "unknown", Status: PhaseStatusComplete, Now: testRunNow(6)})
	assertProblemCode(t, err, "phase_id_unknown")

	path := filepath.Join(repo, ".kkachi", "runs", created.Metadata.RunID, "phase-plan.yaml")
	err = os.WriteFile(path, []byte(`version: "0.1"
run_id: "`+created.Metadata.RunID+`"
phases:
  - id: "intake"
    status: "pending"
    version: "0.1"
`), 0o600)
	if err != nil {
		t.Fatalf("write malformed phase plan: %v", err)
	}
	_, err = ShowPhasePlan(root, created.Metadata.RunID)
	assertProblemCode(t, err, "phase_plan_invalid_yaml")
}

func phaseCheckFailed(checks []PhasePlanCheck, name string) bool {
	for _, check := range checks {
		if check.Name == name && check.Status == PhasePlanStatusFail {
			return true
		}
	}
	return false
}

func phaseCheckPassed(checks []PhasePlanCheck, name string) bool {
	for _, check := range checks {
		if check.Name == name && check.Status == PhasePlanStatusPass {
			return true
		}
	}
	return false
}
