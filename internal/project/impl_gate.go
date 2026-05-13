package project

import (
	"fmt"
	"strings"
)

func checkImplementationGate(root Root, metadata RunMetadata) (GateCheckResult, error) {
	checks := []GateCheck{
		checkDiffPatchArtifact(root, metadata.RunID),
		checkMarkdownGateArtifact(root, metadata.RunID, markdownGateArtifactRule{Name: "impl_log", Artifact: "impl-log.md"}),
	}
	if stringSet(metadata.RequiredArtifacts)["cli-output.md"] {
		checks = append(checks, checkMarkdownGateArtifact(root, metadata.RunID, markdownGateArtifactRule{Name: "cli_output", Artifact: "cli-output.md"}))
	}
	return gateResultFromChecks(metadata.RunID, GateImplementation, checks), nil
}

func checkDiffPatchArtifact(root Root, runID string) GateCheck {
	path, content, failure, ok := readGateArtifact(root, runID, "diff.patch", "diff_patch")
	if !ok {
		return failure
	}
	if strings.TrimSpace(string(content)) == "" {
		return GateCheck{Name: "diff_patch", Status: GateStatusFail, Path: path, Message: "diff patch artifact is empty", Hint: "Record the implementation diff before checking this gate.", Field: "path", Expected: "non-empty patch file", Actual: "empty"}
	}
	return GateCheck{Name: "diff_patch", Status: GateStatusPass, Path: path, Message: "diff patch artifact is present"}
}

func checkReviewGate(root Root, metadata RunMetadata) (GateCheckResult, error) {
	checks := []GateCheck{
		checkMarkdownGateArtifact(root, metadata.RunID, markdownGateArtifactRule{Name: "review", Artifact: "review.md"}),
	}
	for _, artifact := range metadata.RequiredArtifacts {
		if strings.HasPrefix(artifact, "redteam/") {
			name := strings.TrimPrefix(artifact, "redteam/")
			name = strings.TrimSuffix(name, ".md")
			checks = append(checks, checkMarkdownGateArtifact(root, metadata.RunID, markdownGateArtifactRule{Name: "redteam_" + name, Artifact: artifact}))
		}
	}
	return gateResultFromChecks(metadata.RunID, GateReview, checks), nil
}

func checkVerificationGate(root Root, metadata RunMetadata) (GateCheckResult, error) {
	checks := []GateCheck{
		checkMarkdownGateArtifact(root, metadata.RunID, markdownGateArtifactRule{Name: "test_log", Artifact: "test-log.md"}),
	}
	verdictCheck := checkVerificationArtifact(root, metadata.RunID)
	checks = append(checks, verdictCheck)
	return gateResultFromChecks(metadata.RunID, GateVerification, checks), nil
}

func checkVerificationArtifact(root Root, runID string) GateCheck {
	path, content, failure, ok := readGateArtifact(root, runID, "verification.md", "verification")
	if !ok {
		return failure
	}
	base := validateMarkdownGateArtifact(markdownGateArtifactRule{Name: "verification", Artifact: "verification.md"}, path, content)
	if base.Status != GateStatusPass {
		return base
	}
	fields := parseMarkdownFields(string(content))
	verdict := strings.ToLower(strings.TrimSpace(fields["verdict"]))
	switch verdict {
	case "pass", "fail":
		return GateCheck{Name: "verification", Status: GateStatusPass, Path: path, Message: fmt.Sprintf("verification verdict is %s", verdict), Field: "verdict", Expected: "pass or fail", Actual: verdict}
	case "":
		return GateCheck{Name: "verification", Status: GateStatusFail, Path: path, Message: "verification verdict is missing", Hint: "Record Verdict: pass or Verdict: fail in verification.md.", Field: "verdict", Expected: "pass or fail", Actual: "missing"}
	default:
		return GateCheck{Name: "verification", Status: GateStatusFail, Path: path, Message: "verification verdict is invalid", Hint: "Use Verdict: pass or Verdict: fail.", Field: "verdict", Expected: "pass or fail", Actual: verdict}
	}
}

func checkDocsGate(root Root, metadata RunMetadata) (GateCheckResult, error) {
	checks := []GateCheck{
		checkDocsUpdateArtifact(root, metadata.RunID),
	}
	return gateResultFromChecks(metadata.RunID, GateDocs, checks), nil
}

func checkDocsUpdateArtifact(root Root, runID string) GateCheck {
	path, content, failure, ok := readGateArtifact(root, runID, "docs-update.md", "docs_update")
	if !ok {
		return failure
	}
	base := validateMarkdownGateArtifact(markdownGateArtifactRule{Name: "docs_update", Artifact: "docs-update.md"}, path, content)
	if base.Status != GateStatusPass {
		return base
	}
	fields := parseMarkdownFields(string(content))
	changedDocs := strings.TrimSpace(fields["changed_docs"])
	noChangeReason := strings.TrimSpace(fields["no_change_reason"])
	if changedDocs == "" && noChangeReason == "" {
		return GateCheck{Name: "docs_update", Status: GateStatusFail, Path: path, Message: "docs update lacks changed docs list or no-change reason", Hint: "Add Changed Docs: <list> or No Change Reason: <why docs were not updated>.", Field: "changed_docs,no_change_reason", Expected: "non-empty changed_docs or no_change_reason", Actual: "missing"}
	}
	return GateCheck{Name: "docs_update", Status: GateStatusPass, Path: path, Message: "docs update decision is recorded"}
}

func checkFinalGate(root Root, metadata RunMetadata, _ string) (GateCheckResult, error) {
	checks := []GateCheck{checkFinalReportArtifact(root, metadata.RunID)}

	requiredGates := []string{GateIntake, GateSOT, GateRoadmap, GatePlan, GateImplementation, GateReview, GateVerification, GateDocs}
	if backendGateRequired(metadata) {
		requiredGates = append(requiredGates, GateBackend)
	}

	for _, gate := range requiredGates {
		checks = append(checks, checkRequiredGatePass(metadata, gate))
	}

	return gateResultFromChecks(metadata.RunID, GateFinal, checks), nil
}

func checkFinalReportArtifact(root Root, runID string) GateCheck {
	path, content, failure, ok := readGateArtifact(root, runID, "final-report.md", "final_report")
	if !ok {
		return failure
	}
	return validateMarkdownGateArtifact(markdownGateArtifactRule{Name: "final_report", Artifact: "final-report.md"}, path, content)
}

func checkRequiredGatePass(metadata RunMetadata, gate string) GateCheck {
	state, ok := metadata.GateState[gate].(map[string]any)
	if !ok {
		return GateCheck{Name: gate + "_gate", Status: GateStatusFail, Path: "", Message: fmt.Sprintf("%s gate has not been checked", gate), Hint: fmt.Sprintf("Run gate check %s %s before running gate final.", metadata.RunID, gate), Field: "gate_state", Expected: GateStatusPass, Actual: "missing"}
	}
	status, ok := state["status"].(string)
	if !ok {
		return GateCheck{Name: gate + "_gate", Status: GateStatusFail, Path: "", Message: fmt.Sprintf("%s gate state is malformed", gate), Hint: "Repair gate_state or restore run-metadata.json from a coherent backup.", Field: "gate_state[" + gate + "].status", Expected: GateStatusPass, Actual: fmt.Sprintf("%T", state["status"])}
	}
	if status == GateStatusPass {
		return GateCheck{Name: gate + "_gate", Status: GateStatusPass, Path: "", Message: fmt.Sprintf("%s gate passed", gate), Field: "gate_state[" + gate + "].status", Expected: GateStatusPass, Actual: status}
	}
	return GateCheck{Name: gate + "_gate", Status: GateStatusFail, Path: "", Message: fmt.Sprintf("%s gate did not pass", gate), Hint: fmt.Sprintf("Resolve the %s gate failure and re-run gate check before gate final.", gate), Field: "gate_state[" + gate + "].status", Expected: GateStatusPass, Actual: status}
}
