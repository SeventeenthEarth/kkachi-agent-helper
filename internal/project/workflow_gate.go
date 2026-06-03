package project

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func checkWorkflowGate(root Root, metadata RunMetadata, gateID string) (GateCheckResult, error) {
	gate, loaded, ok := workflowGateByID(root, gateID)
	if loaded.validation.Status != GraphStatusPass {
		return gateResultFromChecks(metadata.RunID, gateID, []GateCheck{workflowGraphValidationBlockedCheck(loaded.validation)}), nil
	}
	if !ok {
		return gateResultFromChecks(metadata.RunID, gateID, []GateCheck{{
			Name:     "workflow_gate_declared",
			Status:   GateStatusBlocked,
			Message:  "gate is not a built-in gate and is not declared in the workflow graph",
			Hint:     "Declare the gate in .kkachi-workflow.yaml with checks before running it, or use a built-in gate name.",
			Field:    "gate",
			Expected: "built-in gate or workflow-declared gate",
			Actual:   gateID,
		}}), nil
	}
	if len(gate.Checks) == 0 {
		return gateResultFromChecks(metadata.RunID, gateID, []GateCheck{{
			Name:     "workflow_gate_checks",
			Status:   GateStatusBlocked,
			Message:  "workflow gate does not declare executable checks",
			Hint:     "Add declarative checks to .kkachi-workflow.yaml before using this gate as a machine gate.",
			Field:    "gates[].checks",
			Expected: "one or more workflow gate checks",
			Actual:   "missing",
		}}), nil
	}
	checks := make([]GateCheck, 0, len(gate.Checks))
	for i, check := range gate.Checks {
		checks = append(checks, evaluateWorkflowGateCheck(root, metadata.RunID, gateID, i+1, check))
	}
	return gateResultFromChecks(metadata.RunID, gateID, checks), nil
}

func workflowGateByID(root Root, gateID string) (WorkflowGraphGate, loadedWorkflowGraph, bool) {
	loaded := loadWorkflowGraph(root, GraphOptions{File: WorkflowGraphDefaultPath})
	if loaded.validation.Status != GraphStatusPass {
		return WorkflowGraphGate{}, loaded, false
	}
	for _, gate := range loaded.graph.Gates {
		if gate.ID == gateID {
			return cleanWorkflowGraphGate(gate), loaded, true
		}
	}
	return WorkflowGraphGate{}, loaded, false
}

func workflowFinalRequiredGateIDs(root Root) ([]string, *GateCheck) {
	path, err := ResolveRelativePath(root, WorkflowGraphDefaultPath)
	if err != nil {
		check := GateCheck{Name: "workflow_graph", Status: GateStatusBlocked, Message: "workflow graph path is invalid", Hint: "Repair the workflow graph path before final gate evaluation.", Field: "file", Expected: WorkflowGraphDefaultPath, Actual: err.Error()}
		return nil, &check
	}
	if _, err := os.Lstat(path.Absolute); os.IsNotExist(err) {
		return []string{}, nil
	} else if err != nil {
		check := GateCheck{Name: "workflow_graph", Status: GateStatusBlocked, Path: path.Relative, Message: "cannot inspect workflow graph", Hint: "Check repository permissions before final gate evaluation.", Field: "file", Expected: "inspectable workflow graph", Actual: err.Error()}
		return nil, &check
	}
	loaded := loadWorkflowGraph(root, GraphOptions{File: WorkflowGraphDefaultPath})
	if loaded.validation.Status != GraphStatusPass {
		check := workflowGraphValidationBlockedCheck(loaded.validation)
		return nil, &check
	}
	ids := []string{}
	for _, gate := range loaded.graph.Gates {
		if gate.FinalRequired {
			ids = append(ids, gate.ID)
		}
	}
	return ids, nil
}

func workflowGraphValidationBlockedCheck(validation GraphValidationResult) GateCheck {
	actual := "invalid workflow graph"
	if len(validation.Errors) > 0 {
		actual = validation.Errors[0].Message
	}
	return GateCheck{Name: "workflow_graph", Status: GateStatusBlocked, Path: validation.File, Message: "workflow graph is invalid, so workflow gate checks cannot run", Hint: "Run kkachi-agent-helper graph validate and repair .kkachi-workflow.yaml.", Field: "workflow_graph", Expected: GraphStatusPass, Actual: actual}
}

func evaluateWorkflowGateCheck(root Root, runID string, gateID string, index int, check WorkflowGraphCheck) GateCheck {
	name := workflowCheckName(gateID, index, check)
	switch check.Type {
	case "artifact.exists":
		path, _, failure, ok := readGateArtifact(root, runID, check.Path, name)
		if !ok {
			return failure
		}
		return GateCheck{Name: name, Status: GateStatusPass, Path: path, Message: "workflow gate artifact exists", Field: "path", Expected: check.Path, Actual: path}
	case "markdown.field":
		path, content, failure, ok := readGateArtifact(root, runID, check.Path, name)
		if !ok {
			return failure
		}
		fields := parseMarkdownFields(string(content))
		fieldKey := normalizeMarkdownFieldKey(check.Field)
		actual := strings.TrimSpace(fields[fieldKey])
		if actual == "" {
			return GateCheck{Name: name, Status: GateStatusFail, Path: path, Message: "workflow gate markdown field is missing", Hint: "Record the required markdown field in the gate artifact.", Field: check.Field, Expected: workflowMarkdownExpected(check), Actual: "missing"}
		}
		if check.Equals != "" {
			if strings.EqualFold(strings.TrimSpace(actual), strings.TrimSpace(check.Equals)) {
				return GateCheck{Name: name, Status: GateStatusPass, Path: path, Message: "workflow gate markdown field matches", Field: check.Field, Expected: check.Equals, Actual: actual}
			}
			return GateCheck{Name: name, Status: GateStatusFail, Path: path, Message: "workflow gate markdown field does not match", Hint: "Update the gate artifact field to the configured value.", Field: check.Field, Expected: check.Equals, Actual: actual}
		}
		for _, allowed := range check.OneOf {
			if strings.EqualFold(strings.TrimSpace(actual), strings.TrimSpace(allowed)) {
				return GateCheck{Name: name, Status: GateStatusPass, Path: path, Message: "workflow gate markdown field is allowed", Field: check.Field, Expected: strings.Join(check.OneOf, ","), Actual: actual}
			}
		}
		return GateCheck{Name: name, Status: GateStatusFail, Path: path, Message: "workflow gate markdown field is not allowed", Hint: "Update the gate artifact field to one of the configured values.", Field: check.Field, Expected: strings.Join(check.OneOf, ","), Actual: actual}
	case "text.contains":
		path, content, failure, ok := readGateArtifact(root, runID, check.Path, name)
		if !ok {
			return failure
		}
		if strings.Contains(strings.ToLower(string(content)), strings.ToLower(check.Token)) {
			return GateCheck{Name: name, Status: GateStatusPass, Path: path, Message: "workflow gate text token is present", Field: "token", Expected: check.Token, Actual: "present"}
		}
		return GateCheck{Name: name, Status: GateStatusFail, Path: path, Message: workflowCheckMessage(check, "workflow gate text token is missing"), Hint: workflowCheckHint(check, "Add the configured token to the gate artifact or remove this check from the workflow."), Field: "token", Expected: check.Token, Actual: "missing"}
	case "text.contains_all":
		path, content, failure, ok := readGateArtifact(root, runID, check.Path, name)
		if !ok {
			return failure
		}
		missing := []string{}
		body := strings.ToLower(string(content))
		for _, token := range check.Tokens {
			if !strings.Contains(body, strings.ToLower(token)) {
				missing = append(missing, token)
			}
		}
		if len(missing) == 0 {
			return GateCheck{Name: name, Status: GateStatusPass, Path: path, Message: "workflow gate text tokens are present", Field: "tokens", Expected: strings.Join(check.Tokens, ","), Actual: "present"}
		}
		return GateCheck{Name: name, Status: GateStatusFail, Path: path, Message: workflowCheckMessage(check, "workflow gate text tokens are missing"), Hint: workflowCheckHint(check, "Add the configured tokens to the gate artifact or remove this check from the workflow."), Field: "tokens", Expected: strings.Join(check.Tokens, ","), Actual: "missing " + strings.Join(missing, ",")}
	case "phase.status":
		plan, err := readPhasePlan(root, runID)
		if err != nil {
			return GateCheck{Name: name, Status: GateStatusFail, Message: "cannot read phase plan for workflow gate check", Hint: "Initialize and maintain phase-plan.yaml before checking workflow gates.", Field: "phase_plan", Expected: "readable phase-plan.yaml", Actual: err.Error()}
		}
		for _, phase := range plan.Phases {
			if phase.ID == check.Phase {
				if phase.Status == check.Status {
					return GateCheck{Name: name, Status: GateStatusPass, Path: plan.Path, Message: "workflow gate phase status matches", Field: check.Phase + ".status", Expected: check.Status, Actual: phase.Status}
				}
				return GateCheck{Name: name, Status: GateStatusFail, Path: plan.Path, Message: "workflow gate phase status does not match", Hint: "Advance the declared phase state or update the workflow check.", Field: check.Phase + ".status", Expected: check.Status, Actual: phase.Status}
			}
		}
		return GateCheck{Name: name, Status: GateStatusFail, Path: plan.Path, Message: "workflow gate phase row is missing", Hint: "Re-initialize or update phase-plan.yaml with workflow-required rows.", Field: "phases[].id", Expected: check.Phase, Actual: "missing"}
	default:
		return GateCheck{Name: name, Status: GateStatusBlocked, Message: "workflow gate check type is unsupported", Hint: "Use a supported declarative workflow gate check type.", Field: "type", Expected: workflowGraphCheckTypes(), Actual: check.Type}
	}
}

func workflowCheckName(gateID string, index int, check WorkflowGraphCheck) string {
	if name := strings.TrimSpace(check.Name); name != "" {
		return name
	}
	return fmt.Sprintf("%s_check_%02d", gateID, index)
}

func workflowCheckMessage(check WorkflowGraphCheck, fallback string) string {
	if message := strings.TrimSpace(check.Message); message != "" {
		return message
	}
	return fallback
}

func workflowCheckHint(check WorkflowGraphCheck, fallback string) string {
	if hint := strings.TrimSpace(check.Hint); hint != "" {
		return hint
	}
	return fallback
}

func workflowMarkdownExpected(check WorkflowGraphCheck) string {
	if check.Equals != "" {
		return check.Equals
	}
	return strings.Join(check.OneOf, ",")
}

func workflowGateReportName(gate string) string {
	return strings.ReplaceAll(filepath.Base(gate), string(filepath.Separator), "_")
}
