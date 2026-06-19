package project

import (
	"encoding/json"
	"fmt"
	"os"
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
	checks := []GateCheck{checkFinalReportArtifact(root, metadata.RunID), checkWorkflowInstanceCompletenessGate(root, metadata)}

	requiredGates := []string{GateIntake, GateSOT, GateRoadmap, GatePlan, GateImplementation, GateReview, GateVerification, GateDocs}
	if backendGateRequired(metadata) {
		requiredGates = append(requiredGates, GateBackend)
	}
	if multiAgentReviewGateRequired(root, metadata) {
		requiredGates = append(requiredGates, GateMultiAgentReview)
	}
	workflowRequiredGates, workflowBlocked := workflowFinalRequiredGateIDs(root)
	if workflowBlocked != nil {
		checks = append(checks, *workflowBlocked)
	} else {
		requiredGates = append(requiredGates, workflowRequiredGates...)
	}

	for _, gate := range requiredGates {
		gateCheck := checkRequiredGatePass(metadata, gate)
		checks = append(checks, gateCheck)
		if gateCheck.Status == GateStatusPass {
			checks = append(checks, checkRequiredGateFreshness(root, metadata, gate))
		}
	}

	return gateResultFromChecks(metadata.RunID, GateFinal, checks), nil
}

func checkWorkflowInstanceCompletenessGate(root Root, metadata RunMetadata) GateCheck {
	result, err := CheckWorkflowInstanceCompleteness(root, metadata.RunID)
	if err != nil {
		return GateCheck{Name: "workflow_instance", Status: GateStatusFail, Message: "workflow instance completeness could not be checked", Hint: "Repair run metadata or workflow instance state before running gate final.", Field: "workflow_instance", Expected: "checkable workflow instance state", Actual: err.Error()}
	}
	switch result.Status {
	case WorkflowCatalogStatusMissing:
		if metadata.WorkflowManaged {
			return GateCheck{Name: "workflow_instance", Status: GateStatusFail, Path: result.Path, Message: "workflow-managed run is missing workflow instance state", Hint: "Create the selected workflow instance for this run before running gate final.", Field: "workflow_instance", Expected: "required workflow-instance.json", Actual: result.Reason}
		}
		return GateCheck{Name: "workflow_instance", Status: GateStatusNotApplicable, Path: result.Path, Message: "no workflow instance is present for this run", Field: "workflow_instance", Expected: "optional workflow-instance.json", Actual: "missing"}
	case WorkflowCatalogStatusPass:
		if check := checkWorkflowInstanceIdentity(metadata, result); check != nil {
			return *check
		}
		return GateCheck{Name: "workflow_instance", Status: GateStatusPass, Path: result.Path, Message: "workflow instance is complete", Field: "reason", Expected: "workflow_instance_complete", Actual: result.Reason}
	default:
		if check := checkWorkflowInstanceIdentity(metadata, result); check != nil {
			return *check
		}
		actual := result.Reason
		if len(result.Diagnostics) > 0 {
			actual = result.Diagnostics[0].Code
		}
		return GateCheck{Name: "workflow_instance", Status: GateStatusFail, Path: result.Path, Message: "workflow instance is incomplete", Hint: "Complete all workflow nodes and preserve required outputs/evidence before final gate.", Field: "reason", Expected: "workflow_instance_complete", Actual: actual}
	}
}

func checkWorkflowInstanceIdentity(metadata RunMetadata, result WorkflowInstanceCompletenessResult) *GateCheck {
	if !metadata.WorkflowManaged {
		return nil
	}
	selectedWorkflowID := ""
	if metadata.SelectedWorkflowID != nil {
		selectedWorkflowID = strings.TrimSpace(*metadata.SelectedWorkflowID)
	}
	if selectedWorkflowID == "" {
		return &GateCheck{Name: "workflow_instance", Status: GateStatusFail, Path: result.Path, Message: "workflow-managed run is missing selected workflow id metadata", Hint: "Set selected_workflow_id from the selected workflow before running gate final.", Field: "selected_workflow_id", Expected: result.WorkflowID, Actual: "missing"}
	}
	if selectedWorkflowID != result.WorkflowID {
		return &GateCheck{Name: "workflow_instance", Status: GateStatusFail, Path: result.Path, Message: "workflow instance id does not match run metadata", Hint: "Create the workflow instance from the run metadata selected_workflow_id or repair stale run metadata.", Field: "selected_workflow_id", Expected: selectedWorkflowID, Actual: result.WorkflowID}
	}
	workflowSource := ""
	if metadata.WorkflowSource != nil {
		workflowSource = strings.TrimSpace(*metadata.WorkflowSource)
	}
	if workflowSource == "" {
		return &GateCheck{Name: "workflow_instance", Status: GateStatusFail, Path: result.Path, Message: "workflow-managed run is missing workflow source metadata", Hint: "Set workflow_source from the selected workflow before running gate final.", Field: "workflow_source", Expected: result.SourcePath, Actual: "missing"}
	}
	if workflowSource != result.SourcePath {
		return &GateCheck{Name: "workflow_instance", Status: GateStatusFail, Path: result.Path, Message: "workflow instance source does not match run metadata", Hint: "Create the workflow instance from the run metadata workflow_source or repair stale run metadata.", Field: "workflow_source", Expected: workflowSource, Actual: result.SourcePath}
	}
	return nil
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

func checkRequiredGateFreshness(root Root, metadata RunMetadata, gate string) GateCheck {
	state, ok := metadata.GateState[gate].(map[string]any)
	if !ok {
		return GateCheck{Name: gate + "_gate_freshness", Status: GateStatusFail, Message: fmt.Sprintf("%s gate state is missing freshness metadata", gate), Hint: fmt.Sprintf("Re-run gate check %s %s, then re-run gate final.", metadata.RunID, gate), Field: "gate_state", Expected: "gate report with evidence fingerprints", Actual: "missing"}
	}
	reportRelative, ok := state["report_path"].(string)
	if !ok || strings.TrimSpace(reportRelative) == "" {
		return GateCheck{Name: gate + "_gate_freshness", Status: GateStatusFail, Message: fmt.Sprintf("%s gate state lacks a report path", gate), Hint: fmt.Sprintf("Re-run gate check %s %s to write a report with evidence fingerprints.", metadata.RunID, gate), Field: "gate_state[" + gate + "].report_path", Expected: "non-empty report path", Actual: "missing"}
	}
	reportPath, err := ResolveRelativePath(root, reportRelative)
	if err != nil {
		return GateCheck{Name: gate + "_gate_freshness", Status: GateStatusFail, Path: reportRelative, Message: fmt.Sprintf("%s gate report path is invalid", gate), Hint: fmt.Sprintf("Re-run gate check %s %s after repairing gate state.", metadata.RunID, gate), Field: "report_path", Expected: "safe report path", Actual: err.Error()}
	}
	data, err := os.ReadFile(reportPath.Absolute)
	if err != nil {
		return GateCheck{Name: gate + "_gate_freshness", Status: GateStatusFail, Path: reportPath.Relative, Message: fmt.Sprintf("%s gate report is missing or unreadable", gate), Hint: fmt.Sprintf("Re-run gate check %s %s to regenerate the gate report.", metadata.RunID, gate), Field: "report_path", Expected: "readable gate report", Actual: err.Error()}
	}
	var report gateReport
	if err := json.Unmarshal(data, &report); err != nil {
		return GateCheck{Name: gate + "_gate_freshness", Status: GateStatusFail, Path: reportPath.Relative, Message: fmt.Sprintf("%s gate report is malformed", gate), Hint: fmt.Sprintf("Re-run gate check %s %s to regenerate the gate report.", metadata.RunID, gate), Field: "json", Expected: "valid gate report JSON", Actual: err.Error()}
	}
	if report.Gate != gate || report.Status != GateStatusPass {
		return GateCheck{Name: gate + "_gate_freshness", Status: GateStatusFail, Path: reportPath.Relative, Message: fmt.Sprintf("%s gate report does not match a passing gate", gate), Hint: fmt.Sprintf("Re-run gate check %s %s before final verification.", metadata.RunID, gate), Field: "gate_report", Expected: gate + ":" + GateStatusPass, Actual: report.Gate + ":" + report.Status}
	}
	expectedPaths := passingArtifactEvidencePaths(metadata.RunID, report.Checks)
	if len(expectedPaths) > 0 && len(report.Evidence) == 0 {
		return GateCheck{Name: gate + "_gate_freshness", Status: GateStatusFail, Path: reportPath.Relative, Message: fmt.Sprintf("%s gate report lacks evidence fingerprints", gate), Hint: fmt.Sprintf("Re-run gate check %s %s with the current helper, then re-run gate final.", metadata.RunID, gate), Field: "evidence", Expected: "fingerprints for gate evidence artifacts", Actual: "missing"}
	}
	seen := map[string]bool{}
	for _, evidence := range report.Evidence {
		seen[evidence.Path] = true
		current, err := hashGateReportEvidence(root, evidence.Path)
		if err != nil {
			return GateCheck{Name: gate + "_gate_freshness", Status: GateStatusFail, Path: evidence.Path, Message: fmt.Sprintf("%s gate evidence cannot be rechecked", gate), Hint: fmt.Sprintf("Repair the evidence artifact and re-run gate check %s %s.", metadata.RunID, gate), Field: "path", Expected: "readable evidence artifact", Actual: err.Error()}
		}
		if current.Size != evidence.Size || current.SHA256 != evidence.SHA256 {
			return GateCheck{Name: gate + "_gate_freshness", Status: GateStatusFail, Path: evidence.Path, Message: fmt.Sprintf("%s gate evidence changed after the gate pass", gate), Hint: fmt.Sprintf("Re-run gate check %s %s after updating evidence, then re-run gate final.", metadata.RunID, gate), Field: "sha256", Expected: evidence.SHA256, Actual: current.SHA256}
		}
	}
	for _, path := range expectedPaths {
		if !seen[path] {
			return GateCheck{Name: gate + "_gate_freshness", Status: GateStatusFail, Path: reportPath.Relative, Message: fmt.Sprintf("%s gate report is missing an evidence fingerprint", gate), Hint: fmt.Sprintf("Re-run gate check %s %s with the current helper.", metadata.RunID, gate), Field: "evidence", Expected: path, Actual: "missing"}
		}
	}
	return GateCheck{Name: gate + "_gate_freshness", Status: GateStatusPass, Path: reportPath.Relative, Message: fmt.Sprintf("%s gate evidence is unchanged since gate pass", gate), Field: "report_path", Actual: reportPath.Relative}
}

func passingArtifactEvidencePaths(runID string, checks []GateCheck) []string {
	paths := []string{}
	seen := map[string]bool{}
	for _, check := range checks {
		path := strings.TrimSpace(check.Path)
		if check.Status == GateStatusPass && path != "" && gateReportEvidencePath(runID, path) && !seen[path] {
			paths = append(paths, path)
			seen[path] = true
		}
	}
	return paths
}
