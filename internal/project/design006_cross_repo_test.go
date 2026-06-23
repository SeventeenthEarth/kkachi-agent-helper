package project

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type design006KASScenario struct {
	ID                         string   `json:"id"`
	Project                    string   `json:"project"`
	ProjectHasTealLane         bool     `json:"project_has_teal_lane"`
	UIUXChange                 bool     `json:"ui_ux_change"`
	TealRequired               bool     `json:"teal_required"`
	TealSkipReason             string   `json:"teal_skip_reason"`
	RequiredWhenTealRequired   []string `json:"required_when_teal_required"`
	ExpectedMaterializedNodes  []string `json:"expected_materialized_nodes"`
	OrdinaryReviewIsSubstitute bool     `json:"ordinary_review_is_substitute"`
	MARReviewIsSubstitute      bool     `json:"mar_review_is_substitute"`
	BackendEvidenceSubstitute  bool     `json:"backend_evidence_is_substitute"`
	HelperNotesAreSubstitute   bool     `json:"helper_notes_are_substitute"`
}

func TestDesign006CrossRepoKASDeclarationsMatchKAHDesignEvidenceGate(t *testing.T) {
	scenariosPath := design006ScenarioPath(t)
	if scenariosPath == "" {
		t.Skip("set KAS_DESIGN006_SCENARIOS or keep sibling KAS fixture available to run cross-repo DESIGN-006 readback")
	}
	scenarios := readDesign006KASScenarios(t, scenariosPath)
	if len(scenarios) != 4 {
		t.Fatalf("scenario count = %d, want 4", len(scenarios))
	}
	for _, scenario := range scenarios {
		scenario := scenario
		t.Run(scenario.ID, func(t *testing.T) {
			repo, root, metadata := createDesignEvidenceGateRun(t)
			payload := validDesignEvidence(metadata.RunID, scenario.TealRequired)
			applyDesign006ScenarioToDesignEvidence(payload, scenario)
			writeDesignEvidenceForRun(t, repo, metadata.RunID, payload)

			result, err := CheckGate(root, GateCheckOptions{RunID: metadata.RunID, Gate: GateDesignEvidence, Now: testRunNow(5)})
			if err != nil {
				t.Fatalf("CheckGate(design-evidence) error = %v", err)
			}
			if result.Status != GateStatusPass {
				t.Fatalf("result = %#v, want pass for KAS scenario %s", result, scenario.ID)
			}
			applicability := payload["teal_applicability"].(map[string]any)
			requireDesign006StringSetExact(t, scenario.ID, stringSliceFromAny(applicability["required_when_teal_required"]), scenario.RequiredWhenTealRequired)
			if scenario.TealRequired {
				requireDesign006StringSetExact(t, scenario.ID, scenario.ExpectedMaterializedNodes, []string{"DESIGN_PLAN_GATE", "DESIGN_FIDELITY_REVIEW"})
			} else {
				requireDesign006StringSetExact(t, scenario.ID, scenario.ExpectedMaterializedNodes, []string{})
			}
			if scenario.TealRequired && !gateCheckStatus(result.Checks, "design_plan_verdict", GateStatusPass) {
				t.Fatalf("checks = %#v, want Teal-required design plan pass", result.Checks)
			}
			if !scenario.TealRequired && !gateCheckStatus(result.Checks, "teal_skip_reason", GateStatusPass) {
				t.Fatalf("checks = %#v, want non-UI skip pass", result.Checks)
			}
		})
	}
}

func TestDesign006CrossRepoFixtureFailsClosedForDerivationMismatch(t *testing.T) {
	scenario := design006KASScenario{
		ID:                        "mismatched_or_derivation",
		Project:                   "kkachi",
		ProjectHasTealLane:        true,
		UIUXChange:                false,
		TealRequired:              true,
		RequiredWhenTealRequired:  []string{"DESIGN_PLAN_GATE", "DESIGN_FIDELITY_REVIEW"},
		ExpectedMaterializedNodes: []string{"DESIGN_PLAN_GATE", "DESIGN_FIDELITY_REVIEW"},
	}
	repo, root, metadata := createDesignEvidenceGateRun(t)
	payload := validDesignEvidence(metadata.RunID, scenario.TealRequired)
	applyDesign006ScenarioToDesignEvidence(payload, scenario)
	writeDesignEvidenceForRun(t, repo, metadata.RunID, payload)

	result, err := CheckGate(root, GateCheckOptions{RunID: metadata.RunID, Gate: GateDesignEvidence, Now: testRunNow(5)})
	if err != nil {
		t.Fatalf("CheckGate(design-evidence) error = %v", err)
	}
	if result.Status != GateStatusFail || !gateCheckStatus(result.Checks, "design_evidence_schema_teal_applicability.teal_required", GateStatusFail) {
		t.Fatalf("result = %#v, want fail-closed derivation mismatch", result)
	}
	diagnostics := designEvidenceDiagnostics(root, metadata)
	if !containsString(diagnostics.ReasonCodes, DesignEvidenceReasonSchemaInvalid) {
		t.Fatalf("diagnostics reason codes = %#v, want %s", diagnostics.ReasonCodes, DesignEvidenceReasonSchemaInvalid)
	}
}

func TestDesign006CrossRepoFixtureFailsClosedForForbiddenSubstitution(t *testing.T) {
	scenario := design006KASScenario{
		ID:                        "sudal_ui_required",
		Project:                   "sudal",
		ProjectHasTealLane:        true,
		UIUXChange:                true,
		TealRequired:              true,
		RequiredWhenTealRequired:  []string{"DESIGN_PLAN_GATE", "DESIGN_FIDELITY_REVIEW"},
		ExpectedMaterializedNodes: []string{"DESIGN_PLAN_GATE", "DESIGN_FIDELITY_REVIEW"},
	}
	repo, root, metadata := createDesignEvidenceGateRun(t)
	payload := validDesignEvidence(metadata.RunID, scenario.TealRequired)
	applyDesign006ScenarioToDesignEvidence(payload, scenario)
	payload["design_plan_evidence"].(map[string]any)["status"] = "pending"
	writeDesignEvidenceForRun(t, repo, metadata.RunID, payload)

	result, err := CheckGate(root, GateCheckOptions{RunID: metadata.RunID, Gate: GateDesignEvidence, Now: testRunNow(5)})
	if err != nil {
		t.Fatalf("CheckGate(design-evidence) error = %v", err)
	}
	if result.Status != GateStatusFail || !gateCheckStatus(result.Checks, "design_plan_verdict", GateStatusFail) {
		t.Fatalf("result = %#v, want fail-closed missing Teal plan verdict", result)
	}
}

func design006ScenarioPath(t *testing.T) string {
	t.Helper()
	if path := os.Getenv("KAS_DESIGN006_SCENARIOS"); path != "" {
		return path
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	sibling := filepath.Clean(filepath.Join(cwd, "..", "..", "..", "kkachi-hermes-skills", "docs", "examples", "design006-teal-compatibility-scenarios.json"))
	if _, err := os.Stat(sibling); err == nil {
		return sibling
	}
	return ""
}

func readDesign006KASScenarios(t *testing.T, path string) []design006KASScenario {
	t.Helper()
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatalf("read KAS DESIGN-006 scenarios: %v", err)
	}
	var payload struct {
		Version   string                 `json:"version"`
		Scenarios []design006KASScenario `json:"scenarios"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("decode KAS DESIGN-006 scenarios: %v", err)
	}
	if payload.Version != "design006.v1" {
		t.Fatalf("version = %q, want design006.v1", payload.Version)
	}
	return payload.Scenarios
}

func requireDesign006StringSetExact(t *testing.T, id string, got []string, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s values = %#v, want exactly %#v", id, got, want)
	}
	set := map[string]bool{}
	for _, value := range got {
		set[value] = true
	}
	for _, value := range want {
		if !set[value] {
			t.Fatalf("%s values = %#v, want exactly %#v", id, got, want)
		}
	}
}

func stringSliceFromAny(value any) []string {
	switch typed := value.(type) {
	case []string:
		return typed
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func applyDesign006ScenarioToDesignEvidence(payload map[string]any, scenario design006KASScenario) {
	applicability := payload["teal_applicability"].(map[string]any)
	applicability["project_has_teal_lane"] = scenario.ProjectHasTealLane
	applicability["ui_ux_change"] = scenario.UIUXChange
	applicability["teal_required"] = scenario.TealRequired
	applicability["teal_skip_reason"] = nil
	applicability["teal_owner"] = "teal_reviewer"
	if !scenario.TealRequired {
		applicability["teal_skip_reason"] = scenario.TealSkipReason
		applicability["teal_owner"] = nil
	}
	applicability["required_when_teal_required"] = scenario.RequiredWhenTealRequired
}
