package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckGateDesignEvidencePassesRequiredAndSkipEvidence(t *testing.T) {
	for _, tc := range []struct {
		name         string
		tealRequired bool
		wantCheck    string
	}{
		{name: "teal required", tealRequired: true, wantCheck: "design_verification_ref"},
		{name: "teal skipped", tealRequired: false, wantCheck: "teal_skip_reason"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			repo, root, metadata := createDesignEvidenceGateRun(t)
			writeDesignEvidenceForRun(t, repo, metadata.RunID, validDesignEvidence(metadata.RunID, tc.tealRequired))

			result, err := CheckGate(root, GateCheckOptions{RunID: metadata.RunID, Gate: GateDesignEvidence, Now: testRunNow(5)})
			if err != nil {
				t.Fatalf("CheckGate(design-evidence) error = %v", err)
			}
			if result.Status != GateStatusPass || !gateCheckStatus(result.Checks, tc.wantCheck, GateStatusPass) {
				t.Fatalf("result = %#v, want pass with %s", result, tc.wantCheck)
			}
		})
	}
}

func TestCheckGateDesignEvidenceFailsClosedEvidenceCases(t *testing.T) {
	cases := []struct {
		name       string
		mutate     func(map[string]any)
		wantCheck  string
		wantReason string
	}{
		{
			name: "missing plan verdict",
			mutate: func(payload map[string]any) {
				payload["design_plan_evidence"].(map[string]any)["status"] = "pending"
			},
			wantCheck:  "design_plan_verdict",
			wantReason: DesignEvidenceReasonTealRequiredPlanVerdictMissing,
		},
		{
			name: "missing design spec",
			mutate: func(payload map[string]any) {
				delete(payload["design_plan_evidence"].(map[string]any), "detail_ref")
			},
			wantCheck:  "design_spec_ref",
			wantReason: DesignEvidenceReasonTealRequiredDesignSpecMissing,
		},
		{
			name: "missing fidelity and screenshot refs",
			mutate: func(payload map[string]any) {
				payload["design_fidelity_evidence"].(map[string]any)["evidence_refs"] = []any{}
			},
			wantCheck:  "design_fidelity_refs",
			wantReason: DesignEvidenceReasonTealRequiredFidelityRefsMissing,
		},
		{
			name: "missing verification verdict",
			mutate: func(payload map[string]any) {
				payload["color_review_evidence"].(map[string]any)["status"] = "pending"
			},
			wantCheck:  "design_verification_verdict",
			wantReason: DesignEvidenceReasonTealRequiredVerificationMissing,
		},
		{
			name: "unsafe ref",
			mutate: func(payload map[string]any) {
				payload["design_plan_evidence"].(map[string]any)["detail_ref"] = map[string]any{"path": "../escape.md"}
			},
			wantCheck:  "design_evidence_schema_design_plan_evidence.detail_ref.path",
			wantReason: DesignEvidenceReasonRefUnsafe,
		},
		{
			name: "warning-only fallback",
			mutate: func(payload map[string]any) {
				payload["design_fidelity_evidence"].(map[string]any)["status"] = "warning"
			},
			wantCheck:  "warning_only_fallback",
			wantReason: DesignEvidenceReasonWarningOnlyFallbackForbidden,
		},
		{
			name: "invalid waiver evidence",
			mutate: func(payload map[string]any) {
				applicability := payload["teal_applicability"].(map[string]any)
				applicability["teal_waiver_approved"] = true
				applicability["teal_waiver_approval_ref"] = "../waiver.md"
				delete(payload["design_plan_evidence"].(map[string]any), "detail_ref")
			},
			wantCheck:  "teal_waiver_evidence",
			wantReason: DesignEvidenceReasonTealRequiredWaiverInvalid,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			repo, root, metadata := createDesignEvidenceGateRun(t)
			payload := validDesignEvidence(metadata.RunID, true)
			tc.mutate(payload)
			writeDesignEvidenceForRun(t, repo, metadata.RunID, payload)

			result, err := CheckGate(root, GateCheckOptions{RunID: metadata.RunID, Gate: GateDesignEvidence, Now: testRunNow(5)})
			if err != nil {
				t.Fatalf("CheckGate(design-evidence) error = %v", err)
			}
			if result.Status != GateStatusFail || !gateCheckStatus(result.Checks, tc.wantCheck, GateStatusFail) {
				t.Fatalf("result = %#v, want fail with %s", result, tc.wantCheck)
			}
			diagnostics := designEvidenceDiagnostics(root, metadata)
			if !containsString(diagnostics.ReasonCodes, tc.wantReason) {
				t.Fatalf("diagnostics reason codes = %#v, want %s", diagnostics.ReasonCodes, tc.wantReason)
			}
		})
	}
}

func TestCheckGateDesignEvidenceFailsClosedMissingAndInvalidSkip(t *testing.T) {
	t.Run("required missing artifact", func(t *testing.T) {
		repo, root, metadata := createDesignEvidenceGateRun(t)
		if err := os.Remove(filepath.Join(repo, ".kkachi", "runs", metadata.RunID, designEvidenceArtifact)); err != nil {
			t.Fatalf("remove design evidence: %v", err)
		}
		result, err := CheckGate(root, GateCheckOptions{RunID: metadata.RunID, Gate: GateDesignEvidence, Now: testRunNow(5)})
		if err != nil {
			t.Fatalf("CheckGate(design-evidence missing) error = %v", err)
		}
		if result.Status != GateStatusFail || !gateCheckStatus(result.Checks, "design_evidence_artifact", GateStatusFail) {
			t.Fatalf("result = %#v, want missing artifact fail", result)
		}
	})

	t.Run("invalid skip evidence", func(t *testing.T) {
		repo, root, metadata := createDesignEvidenceGateRun(t)
		payload := validDesignEvidence(metadata.RunID, false)
		payload["teal_applicability"].(map[string]any)["teal_skip_reason"] = "   "
		writeDesignEvidenceForRun(t, repo, metadata.RunID, payload)
		result, err := CheckGate(root, GateCheckOptions{RunID: metadata.RunID, Gate: GateDesignEvidence, Now: testRunNow(5)})
		if err != nil {
			t.Fatalf("CheckGate(design-evidence invalid skip) error = %v", err)
		}
		if result.Status != GateStatusFail || !gateCheckStatus(result.Checks, "design_evidence_schema_teal_applicability.teal_skip_reason", GateStatusFail) {
			t.Fatalf("result = %#v, want invalid skip schema failure", result)
		}
	})
}

func TestCheckGateDesignEvidenceValidWaiverSatisfiesRequiredEvidence(t *testing.T) {
	repo, root, metadata := createDesignEvidenceGateRun(t)
	payload := validDesignEvidence(metadata.RunID, true)
	addValidDesignEvidenceWaiver(t, payload, metadata.RunID)
	payload["design_plan_evidence"].(map[string]any)["status"] = "pending"
	writeDesignEvidenceForRun(t, repo, metadata.RunID, payload)

	result, err := CheckGate(root, GateCheckOptions{RunID: metadata.RunID, Gate: GateDesignEvidence, Now: testRunNow(5)})
	if err != nil {
		t.Fatalf("CheckGate(design-evidence waiver) error = %v", err)
	}
	if result.Status != GateStatusPass || !gateCheckStatus(result.Checks, "teal_waiver_evidence", GateStatusPass) {
		t.Fatalf("result = %#v, want valid waiver pass", result)
	}
}

func TestCheckGateDesignEvidenceValidWaiverStillRequiresBoundaryPass(t *testing.T) {
	repo, root, metadata := createDesignEvidenceGateRun(t)
	payload := validDesignEvidence(metadata.RunID, true)
	addValidDesignEvidenceWaiver(t, payload, metadata.RunID)
	payload["design_plan_evidence"].(map[string]any)["status"] = "pending"
	payload["boundary_evidence"].(map[string]any)["status"] = GateStatusFail
	writeDesignEvidenceForRun(t, repo, metadata.RunID, payload)

	result, err := CheckGate(root, GateCheckOptions{RunID: metadata.RunID, Gate: GateDesignEvidence, Now: testRunNow(5)})
	if err != nil {
		t.Fatalf("CheckGate(design-evidence waiver boundary) error = %v", err)
	}
	if result.Status != GateStatusFail || !gateCheckStatus(result.Checks, "design_boundary", GateStatusFail) {
		t.Fatalf("result = %#v, want fail with design_boundary", result)
	}
	diagnostics := designEvidenceDiagnostics(root, metadata)
	if !containsString(diagnostics.ReasonCodes, DesignEvidenceReasonBoundaryInvalid) {
		t.Fatalf("diagnostics reason codes = %#v, want %s", diagnostics.ReasonCodes, DesignEvidenceReasonBoundaryInvalid)
	}
	if containsString(diagnostics.ReasonCodes, DesignEvidenceReasonTealRequiredWaiverInvalid) {
		t.Fatalf("diagnostics reason codes = %#v, did not expect valid waiver to be reported invalid", diagnostics.ReasonCodes)
	}
}

func TestCheckGateFinalRequiresDesignEvidenceGateWhenManifestRequiresIt(t *testing.T) {
	repo, root, metadata := createDesignEvidenceGateRun(t)
	writeDesignEvidenceForRun(t, repo, metadata.RunID, validDesignEvidence(metadata.RunID, false))
	writeCompleteGateFixtureArtifacts(t, repo, metadata)
	passFixturePriorGates(t, root, metadata.RunID)

	failed, err := CheckGate(root, GateCheckOptions{RunID: metadata.RunID, Gate: GateFinal, Now: testRunNow(20)})
	if err != nil {
		t.Fatalf("CheckGate(final without design gate) error = %v", err)
	}
	if failed.Status != GateStatusFail || !gateCheckStatus(failed.Checks, GateDesignEvidence+"_gate", GateStatusFail) {
		t.Fatalf("failed = %#v, want final gate to require design-evidence gate", failed)
	}

	if result, err := CheckGate(root, GateCheckOptions{RunID: metadata.RunID, Gate: GateDesignEvidence, Now: testRunNow(21)}); err != nil {
		t.Fatalf("CheckGate(design-evidence) error = %v", err)
	} else if result.Status != GateStatusPass {
		t.Fatalf("design-evidence result = %#v, want pass", result)
	}
	passed, err := CheckGate(root, GateCheckOptions{RunID: metadata.RunID, Gate: GateFinal, Now: testRunNow(22)})
	if err != nil {
		t.Fatalf("CheckGate(final with design gate) error = %v", err)
	}
	if passed.Status != GateStatusPass || !gateCheckStatus(passed.Checks, GateDesignEvidence+"_gate", GateStatusPass) || !gateCheckStatus(passed.Checks, GateDesignEvidence+"_gate_freshness", GateStatusPass) {
		t.Fatalf("passed = %#v, want final gate design-evidence pass and freshness", passed)
	}
}

func TestExportDiagnosticsIncludesDesignEvidenceSummary(t *testing.T) {
	repo, root, metadata := createDesignEvidenceGateRun(t)
	payload := validDesignEvidence(metadata.RunID, true)
	payload["color_review_evidence"].(map[string]any)["status"] = "pending"
	writeDesignEvidenceForRun(t, repo, metadata.RunID, payload)

	bundle, err := ExportDiagnostics(root, DiagnosticsExportOptions{RunID: metadata.RunID, Now: fixedDiagnosticsTime})
	if err != nil {
		t.Fatalf("ExportDiagnostics() error = %v", err)
	}
	if bundle.DesignEvidence == nil || bundle.DesignEvidence.Status != GateStatusFail || !containsString(bundle.DesignEvidence.ReasonCodes, DesignEvidenceReasonTealRequiredVerificationMissing) {
		t.Fatalf("design evidence diagnostics = %#v, want failing verification reason", bundle.DesignEvidence)
	}
	foundArtifact := false
	for _, artifact := range bundle.SelectedArtifacts {
		if strings.HasSuffix(artifact.Path, "/"+designEvidenceArtifact) && artifact.Status == "present" {
			foundArtifact = true
		}
	}
	if !foundArtifact {
		t.Fatalf("selected artifacts = %#v, want design-evidence.json present", bundle.SelectedArtifacts)
	}
}

func createDesignEvidenceGateRun(t *testing.T) (string, Root, RunMetadata) {
	t.Helper()
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	options := deterministicCreateRunOptions()
	options.TaskID = "DESIGN-005"
	created, err := CreateRun(root, options)
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	if _, err := InitArtifacts(root, ArtifactInitOptions{RunID: created.Metadata.RunID, Now: testRunNow(4)}); err != nil {
		t.Fatalf("InitArtifacts() error = %v", err)
	}
	metadata := readRunMetadata(t, repo, created.Metadata.RunID)
	if !containsString(metadata.RequiredArtifacts, designEvidenceArtifact) {
		t.Fatalf("required_artifacts = %#v, want %s", metadata.RequiredArtifacts, designEvidenceArtifact)
	}
	return repo, root, metadata
}

func writeDesignEvidenceForRun(t *testing.T, repo, runID string, payload map[string]any) {
	t.Helper()
	writeDesignEvidence(t, filepath.Join(repo, ".kkachi", "runs", runID, designEvidenceArtifact), payload)
}

func addValidDesignEvidenceWaiver(t *testing.T, payload map[string]any, runID string) {
	t.Helper()
	applicability := payload["teal_applicability"].(map[string]any)
	applicability["teal_waiver_approved"] = true
	applicability["teal_waiver_approval_ref"] = tokenRunPath(runID, "waiver.md")
	applicability["teal_waiver_scope"] = "DESIGN-005 fixture"
	applicability["teal_waiver_expires_at"] = "2026-12-31T00:00:00Z"
}
