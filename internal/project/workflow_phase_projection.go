package project

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

const workflowPhaseProjectionCheckName = "workflow_phase_projection"

var workflowPhaseProjectionNodeCandidates = map[string][]string{
	"plan":               {"plan"},
	"ask":                {"ask"},
	"implement":          {"implement"},
	"enhance-test":       {"enhance_test", "enhance-test"},
	"ai-slop-cleaner":    {"ai_slop_cleaner", "slop_cleanup"},
	"optimize":           {"optimize"},
	"docs":               {"update_docs", "docs"},
	"request-feedback-1": {"request_feedback", "request-feedback-1"},
	"handle-feedback-1":  {"handle_feedback", "handle-feedback-1"},
	"verify":             {"final_verify", "verify"},
	"final":              {"final_verify", "final"},
}

func validateWorkflowPhaseProjection(root Root, metadata RunMetadata, plan PhasePlan) []PhasePlanCheck {
	if !workflowProjectionRequired(metadata) {
		return []PhasePlanCheck{{Name: workflowPhaseProjectionCheckName, Status: PhasePlanStatusPass, Path: plan.Path, Message: "workflow phase projection is not required for non-workflow-managed run"}}
	}
	instancePath, err := workflowInstancePath(root, metadata.RunID)
	if err != nil {
		return []PhasePlanCheck{workflowPhaseProjectionFailure(plan.Path, "workflow_phase_instance_path_invalid", "workflow instance path is invalid", "run_id", "valid workflow run id", err.Error())}
	}
	if _, err := os.Lstat(instancePath.Absolute); os.IsNotExist(err) {
		return []PhasePlanCheck{workflowPhaseProjectionFailure(instancePath.Relative, "workflow_phase_instance_missing", "workflow-managed run is missing workflow instance state", "workflow_instance", "existing workflow-instance.json", "missing")}
	} else if err != nil {
		return []PhasePlanCheck{workflowPhaseProjectionFailure(instancePath.Relative, "workflow_phase_instance_unreadable", "workflow instance cannot be inspected", "workflow_instance", "inspectable workflow-instance.json", err.Error())}
	}
	instance, err := readWorkflowInstance(instancePath)
	if err != nil {
		return []PhasePlanCheck{workflowPhaseProjectionFailure(instancePath.Relative, "workflow_phase_instance_malformed", "workflow instance is malformed", "workflow_instance", "valid workflow-instance.json", err.Error())}
	}
	checks := workflowPhaseProjectionIdentityChecks(plan.Path, metadata, instance)
	checks = append(checks, workflowPhaseProjectionLedgerChecks(root, metadata.RunID)...)
	nodeByID := mapWorkflowInstanceNodes(instance)
	projectedByNode := map[string]string{}
	for _, phase := range plan.Phases {
		checks = append(checks, validateWorkflowPhaseRowProjection(root, plan.Path, phase, nodeByID, projectedByNode)...)
	}
	if len(checks) == 0 {
		return []PhasePlanCheck{{Name: workflowPhaseProjectionCheckName, Status: PhasePlanStatusPass, Path: instancePath.Relative, Message: "phase-plan completed rows project to succeeded workflow node evidence"}}
	}
	return checks
}

func workflowProjectionRequired(metadata RunMetadata) bool {
	return metadata.WorkflowManaged || metadata.StrictWorkflowOrder
}

func workflowPhaseProjectionLedgerChecks(root Root, runID string) []PhasePlanCheck {
	result, err := CheckWorkflowTransitionOrder(root, runID)
	if err != nil {
		return []PhasePlanCheck{workflowPhaseProjectionFailure("", "workflow_phase_ledger_uncheckable", "workflow transition ledger cannot be checked for phase projection", "workflow_transition_order", "checkable transition ledger", err.Error())}
	}
	if !result.OK || result.Status != WorkflowCatalogStatusPass {
		actual := result.Reason
		if len(result.ReasonCodes) > 0 {
			actual = result.ReasonCodes[0]
		}
		return []PhasePlanCheck{workflowPhaseProjectionFailure(result.Path, "workflow_phase_ledger_invalid", "workflow transition ledger is invalid for phase projection", "workflow_transition_order", "workflow_transition_order_valid", actual)}
	}
	return nil
}

func workflowPhaseProjectionIdentityChecks(path string, metadata RunMetadata, instance WorkflowInstance) []PhasePlanCheck {
	checks := []PhasePlanCheck{}
	if instance.RunID != metadata.RunID {
		checks = append(checks, workflowPhaseProjectionFailure(path, "workflow_phase_run_mismatch", "workflow instance run id does not match phase-plan run", "run_id", metadata.RunID, instance.RunID))
	}
	if metadata.SelectedWorkflowID != nil {
		selected := strings.TrimSpace(*metadata.SelectedWorkflowID)
		if selected != "" && selected != instance.WorkflowID {
			checks = append(checks, workflowPhaseProjectionFailure(path, "workflow_phase_workflow_mismatch", "workflow instance id does not match selected workflow", "selected_workflow_id", selected, instance.WorkflowID))
		}
	}
	if metadata.WorkflowSource != nil {
		source := strings.TrimSpace(*metadata.WorkflowSource)
		if source != "" && source != instance.SourcePath {
			checks = append(checks, workflowPhaseProjectionFailure(path, "workflow_phase_source_mismatch", "workflow instance source does not match selected workflow source", "workflow_source", source, instance.SourcePath))
		}
	}
	return checks
}

func validateWorkflowPhaseRowProjection(root Root, path string, phase PhaseRow, nodeByID map[string]WorkflowInstanceNode, projectedByNode map[string]string) []PhasePlanCheck {
	candidates, ok := workflowPhaseProjectionCandidates(phase.ID)
	if !ok || phase.Status != PhaseStatusComplete {
		return nil
	}
	node, found := firstWorkflowProjectionNode(nodeByID, candidates)
	if !found {
		return []PhasePlanCheck{workflowPhaseProjectionFailure(path, "workflow_phase_node_omitted", "completed phase is omitted from the selected workflow", phase.ID+".workflow_node", strings.Join(candidates, ","), "missing")}
	}
	if previousPhase, ok := projectedByNode[node.ID]; ok {
		return []PhasePlanCheck{workflowPhaseProjectionFailure(path, "workflow_phase_node_duplicate", "multiple completed phases project to the same workflow node", phase.ID+".workflow_node", "one completed phase per workflow node", previousPhase+":"+node.ID)}
	}
	projectedByNode[node.ID] = phase.ID
	checks := []PhasePlanCheck{}
	if node.State != WorkflowNodeSucceeded {
		checks = append(checks, workflowPhaseProjectionFailure(path, "workflow_phase_node_not_succeeded", "completed phase maps to a non-succeeded workflow node", phase.ID+".workflow_node_state", WorkflowNodeSucceeded, node.ID+"="+node.State))
		return checks
	}
	if strings.TrimSpace(phase.Evidence) == "" {
		checks = append(checks, workflowPhaseProjectionFailure(path, "workflow_phase_evidence_missing", "completed phase lacks evidence for workflow projection", phase.ID+".evidence", "workflow node evidence or required output", "missing"))
		return checks
	}
	if !workflowPhaseEvidenceBound(phase.Evidence, node) {
		checks = append(checks, workflowPhaseProjectionFailure(path, "workflow_phase_evidence_unbound", "completed phase evidence is not bound to workflow node evidence or required outputs", phase.ID+".evidence", workflowPhaseNodeEvidenceSummary(node), strings.TrimSpace(phase.Evidence)))
	}
	for _, output := range node.RequiredOutputs {
		relative, ok := workflowExistingRegularFile(root, output)
		if !ok {
			checks = append(checks, workflowPhaseProjectionFailure(path, "workflow_phase_required_output_missing", "workflow node required output for completed phase is missing", phase.ID+".required_outputs", "existing regular file", node.ID+":"+relative))
		}
	}
	for _, evidence := range node.Evidence {
		relative, ok := workflowExistingRegularFile(root, evidence)
		if !ok {
			checks = append(checks, workflowPhaseProjectionFailure(path, "workflow_phase_node_evidence_missing", "workflow node evidence for completed phase is missing", phase.ID+".node_evidence", "existing regular file", node.ID+":"+relative))
		}
	}
	return checks
}

func workflowPhaseProjectionCandidates(phaseID string) ([]string, bool) {
	phaseID = strings.TrimSpace(phaseID)
	if candidates, ok := workflowPhaseProjectionNodeCandidates[phaseID]; ok {
		return candidates, true
	}
	if round, ok := strings.CutPrefix(phaseID, "request-feedback-"); ok && round != "" {
		return []string{"request_feedback_" + round, "request-feedback-" + round}, true
	}
	if round, ok := strings.CutPrefix(phaseID, "handle-feedback-"); ok && round != "" {
		return []string{"handle_feedback_" + round, "handle-feedback-" + round}, true
	}
	return nil, false
}

func mapWorkflowInstanceNodes(instance WorkflowInstance) map[string]WorkflowInstanceNode {
	nodes := make(map[string]WorkflowInstanceNode, len(instance.Nodes))
	for _, node := range instance.Nodes {
		nodes[node.ID] = node
	}
	return nodes
}

func firstWorkflowProjectionNode(nodeByID map[string]WorkflowInstanceNode, candidates []string) (WorkflowInstanceNode, bool) {
	for _, candidate := range candidates {
		if node, ok := nodeByID[candidate]; ok {
			return node, true
		}
	}
	return WorkflowInstanceNode{}, false
}

func workflowPhaseEvidenceBound(evidence string, node WorkflowInstanceNode) bool {
	evidence = strings.TrimSpace(evidence)
	for _, value := range node.Evidence {
		if strings.TrimSpace(value) == evidence {
			return true
		}
	}
	for _, value := range node.RequiredOutputs {
		if strings.TrimSpace(value) == evidence {
			return true
		}
	}
	return false
}

func workflowPhaseNodeEvidenceSummary(node WorkflowInstanceNode) string {
	values := []string{}
	values = append(values, node.Evidence...)
	values = append(values, node.RequiredOutputs...)
	for i := range values {
		values[i] = strings.TrimSpace(values[i])
	}
	sort.Strings(values)
	return fmt.Sprintf("%s:[%s]", node.ID, strings.Join(values, ","))
}

func workflowPhaseProjectionFailure(path string, code string, message string, field string, expected string, actual string) PhasePlanCheck {
	reasonCode := workflowPhaseProjectionReasonCode(code)
	return PhasePlanCheck{
		Name:     workflowPhaseProjectionCheckName,
		Status:   PhasePlanStatusFail,
		Path:     path,
		Message:  message,
		Hint:     "Reconcile phase-plan.yaml with the selected workflow instance and transition evidence; use skipped/not_applicable with a reason for selected-workflow omitted phases.",
		Field:    field,
		Expected: expected,
		Actual:   reasonCode + ":" + actual,
	}
}

func workflowPhaseProjectionReasonCode(code string) string {
	code = strings.TrimSpace(code)
	if strings.HasPrefix(code, "strict_workflow_phase_projection_") {
		return code
	}
	code = strings.TrimPrefix(code, "workflow_phase_")
	return "strict_workflow_phase_projection_" + code
}

func workflowPhaseProjectionGateChecks(root Root, metadata RunMetadata) []GateCheck {
	plan, err := readPhasePlan(root, metadata.RunID)
	if err != nil {
		if !workflowProjectionRequired(metadata) {
			return []GateCheck{{Name: workflowPhaseProjectionCheckName, Status: GateStatusNotApplicable, Message: "workflow phase projection is not required for non-workflow-managed run", Field: "phase_plan", Expected: "workflow-managed phase-plan projection", Actual: "not_applicable"}}
		}
		return []GateCheck{{Name: workflowPhaseProjectionCheckName, Status: GateStatusFail, Message: "cannot read phase plan for workflow projection", Hint: "Initialize and maintain phase-plan.yaml before running gate final.", Field: "phase_plan", Expected: "readable phase-plan.yaml", Actual: err.Error()}}
	}
	phaseChecks := validateWorkflowPhaseProjection(root, metadata, plan)
	gateChecks := make([]GateCheck, 0, len(phaseChecks))
	for _, check := range phaseChecks {
		status := GateStatusPass
		if check.Status == PhasePlanStatusFail {
			status = GateStatusFail
		}
		gateChecks = append(gateChecks, GateCheck{Name: check.Name, Status: status, Path: check.Path, Message: check.Message, Hint: check.Hint, Field: check.Field, Expected: check.Expected, Actual: check.Actual})
	}
	return gateChecks
}
