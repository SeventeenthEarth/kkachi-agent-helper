package project

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

const maxWorkflowTransitionDiagnostics = 10

type WorkflowTransitionOrderResult struct {
	Status      string               `json:"status"`
	OK          bool                 `json:"ok"`
	Reason      string               `json:"reason"`
	ReasonCodes []string             `json:"reason_codes"`
	RunID       string               `json:"run_id,omitempty"`
	Path        string               `json:"path,omitempty"`
	WorkflowID  string               `json:"workflow_id,omitempty"`
	Revision    int                  `json:"revision,omitempty"`
	Diagnostics []WorkflowDiagnostic `json:"diagnostics,omitempty"`
}

type workflowTransitionEvent struct {
	EventID           string
	NodeID            string
	Kind              string
	PreviousRevision  int
	ResultingRevision int
	PreviousState     string
	ResultingState    string
}

func CheckWorkflowTransitionOrder(root Root, runID string) (WorkflowTransitionOrderResult, error) {
	resolved, err := ResolveRunID(root, runID)
	if err != nil {
		return WorkflowTransitionOrderResult{}, err
	}
	instancePath, err := workflowInstancePath(root, resolved)
	if err != nil {
		return WorkflowTransitionOrderResult{}, err
	}
	result := WorkflowTransitionOrderResult{RunID: resolved, Path: instancePath.Relative, Diagnostics: []WorkflowDiagnostic{}}
	if _, err := os.Lstat(instancePath.Absolute); os.IsNotExist(err) {
		result.Status = WorkflowCatalogStatusMissing
		result.OK = true
		result.Reason = "workflow_instance_missing"
		result.ReasonCodes = []string{"workflow_instance_missing"}
		return result, nil
	} else if err != nil {
		addWorkflowTransitionDiagnostic(&result, "workflow_transition_instance_invalid", "workflow instance cannot be inspected", "", "path", "inspectable workflow-instance.json", err.Error(), instancePath.Relative)
		result.Status = WorkflowCatalogStatusFail
		result.OK = false
		result.Reason = "workflow_transition_instance_invalid"
		result.ReasonCodes = []string{"workflow_transition_instance_invalid"}
		return result, nil
	}
	instance, err := readWorkflowInstance(instancePath)
	if err != nil {
		addWorkflowTransitionDiagnostic(&result, "workflow_transition_instance_invalid", "workflow instance is invalid", "", "workflow_instance", "valid workflow-instance.json", err.Error(), instancePath.Relative)
		result.Status = WorkflowCatalogStatusFail
		result.OK = false
		result.Reason = "workflow_transition_instance_invalid"
		result.ReasonCodes = []string{"workflow_transition_instance_invalid"}
		return result, nil
	}
	if instance.RunID != resolved {
		addWorkflowTransitionDiagnostic(&result, "workflow_transition_instance_run_mismatch", "workflow instance run_id does not match the selected run", "", "run_id", resolved, instance.RunID, instancePath.Relative)
		result.Status = WorkflowCatalogStatusFail
		result.OK = false
		result.Reason = "workflow_transition_instance_run_mismatch"
		result.ReasonCodes = []string{"workflow_transition_instance_run_mismatch"}
		return result, nil
	}
	result.WorkflowID = instance.WorkflowID
	result.Revision = instance.Revision

	eventsPath, err := ResolveRelativePath(root, EventsPath)
	if err != nil {
		return WorkflowTransitionOrderResult{}, err
	}
	currentStates := map[string]string{}
	nodeByID := map[string]WorkflowInstanceNode{}
	for _, node := range instance.Nodes {
		currentStates[node.ID] = WorkflowNodePending
		nodeByID[node.ID] = node
	}
	latestTransitionEventByNode := map[string]string{}
	expectedPreviousRevision := 1
	seenTransition := false

	file, err := os.Open(eventsPath.Absolute)
	if err != nil {
		return WorkflowTransitionOrderResult{}, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), MaxEventLineBytes)
	line := 0
	for scanner.Scan() {
		line++
		var event Event
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			addWorkflowTransitionDiagnostic(&result, "workflow_transition_payload_malformed", "workflow transition event is not valid JSON", "", "json", "valid JSON event", err.Error(), eventsPath.Relative)
			continue
		}
		if !isWorkflowTransitionEventType(event.Type) {
			continue
		}
		if event.RunID == nil || *event.RunID != resolved {
			continue
		}
		seenTransition = true
		transition, ok := decodeWorkflowTransitionEvent(event, instance.WorkflowID, nodeByID, eventsPath.Relative, &result)
		if !ok {
			continue
		}
		if transition.PreviousRevision != expectedPreviousRevision {
			code := "workflow_transition_revision_gap"
			if transition.PreviousRevision < expectedPreviousRevision {
				code = "workflow_transition_revision_stale"
			}
			addWorkflowTransitionDiagnostic(&result, code, "workflow transition previous revision is not contiguous", transition.NodeID, "previous_revision", fmt.Sprintf("%d", expectedPreviousRevision), fmt.Sprintf("%d", transition.PreviousRevision), eventsPath.Relative)
			continue
		}
		if transition.ResultingRevision != transition.PreviousRevision+1 {
			addWorkflowTransitionDiagnostic(&result, "workflow_transition_revision_gap", "workflow transition resulting revision is not previous_revision + 1", transition.NodeID, "resulting_revision", fmt.Sprintf("%d", transition.PreviousRevision+1), fmt.Sprintf("%d", transition.ResultingRevision), eventsPath.Relative)
			continue
		}
		if !applyWorkflowTransition(transition, nodeByID[transition.NodeID], currentStates, &result, eventsPath.Relative) {
			continue
		}
		expectedPreviousRevision = transition.ResultingRevision
		latestTransitionEventByNode[transition.NodeID] = transition.EventID
	}
	if err := scanner.Err(); err != nil {
		addWorkflowTransitionDiagnostic(&result, "workflow_transition_payload_malformed", "workflow transition event log cannot be scanned", "", "events", "readable JSONL event log", err.Error(), eventsPath.Relative)
	}

	if !seenTransition && instance.Revision > 1 {
		addWorkflowTransitionDiagnostic(&result, "workflow_transition_payload_malformed", "workflow instance revision has no reconstructable transition ledger", "", "events", "workflow transition events", "missing", eventsPath.Relative)
	}
	if len(result.Diagnostics) == 0 {
		correlateWorkflowTransitionInstance(instance, currentStates, latestTransitionEventByNode, expectedPreviousRevision, &result)
	}
	if len(result.Diagnostics) != 0 {
		result.Status = WorkflowCatalogStatusFail
		result.OK = false
		result.ReasonCodes = uniqueWorkflowTransitionCodes(result.Diagnostics)
		result.Reason = result.ReasonCodes[0]
		return result, nil
	}
	result.Status = WorkflowCatalogStatusPass
	result.OK = true
	result.Reason = "workflow_transition_order_valid"
	result.ReasonCodes = []string{"workflow_transition_order_valid"}
	return result, nil
}

func decodeWorkflowTransitionEvent(event Event, workflowID string, nodeByID map[string]WorkflowInstanceNode, path string, result *WorkflowTransitionOrderResult) (workflowTransitionEvent, bool) {
	payloadRunID, ok := workflowPayloadString(event.Payload, "run_id")
	if !ok || event.RunID == nil || payloadRunID != *event.RunID {
		addWorkflowTransitionDiagnostic(result, "workflow_transition_payload_malformed", "workflow transition payload run_id does not match event run_id", "", "payload.run_id", "matching run_id", fmt.Sprintf("%v", event.Payload["run_id"]), path)
		return workflowTransitionEvent{}, false
	}
	payloadWorkflowID, ok := workflowPayloadString(event.Payload, "workflow_id")
	if !ok {
		addWorkflowTransitionDiagnostic(result, "workflow_transition_payload_malformed", "workflow transition payload is missing workflow_id", "", "payload.workflow_id", "workflow id", "missing", path)
		return workflowTransitionEvent{}, false
	}
	if payloadWorkflowID != workflowID {
		addWorkflowTransitionDiagnostic(result, "workflow_transition_workflow_mismatch", "workflow transition workflow id does not match instance", "", "workflow_id", workflowID, payloadWorkflowID, path)
		return workflowTransitionEvent{}, false
	}
	nodeID, ok := workflowPayloadString(event.Payload, "node_id")
	if !ok {
		addWorkflowTransitionDiagnostic(result, "workflow_transition_payload_malformed", "workflow transition payload is missing node_id", "", "payload.node_id", "node id", "missing", path)
		return workflowTransitionEvent{}, false
	}
	if _, ok := nodeByID[nodeID]; !ok {
		addWorkflowTransitionDiagnostic(result, "workflow_transition_node_unknown", "workflow transition references an unknown node", nodeID, "node_id", "known workflow node id", nodeID, path)
		return workflowTransitionEvent{}, false
	}
	kind, ok := workflowPayloadString(event.Payload, "transition_kind")
	if !ok || kind != workflowTransitionKind(event.Type) {
		addWorkflowTransitionDiagnostic(result, "workflow_transition_payload_malformed", "workflow transition payload kind is missing or mismatched", nodeID, "transition_kind", workflowTransitionKind(event.Type), fmt.Sprintf("%v", event.Payload["transition_kind"]), path)
		return workflowTransitionEvent{}, false
	}
	previousRevision, ok := workflowPayloadInt(event.Payload, "previous_revision")
	if !ok {
		addWorkflowTransitionDiagnostic(result, "workflow_transition_payload_malformed", "workflow transition payload has invalid previous_revision", nodeID, "previous_revision", "integer", fmt.Sprintf("%v", event.Payload["previous_revision"]), path)
		return workflowTransitionEvent{}, false
	}
	resultingRevision, ok := workflowPayloadInt(event.Payload, "resulting_revision")
	if !ok {
		addWorkflowTransitionDiagnostic(result, "workflow_transition_payload_malformed", "workflow transition payload has invalid resulting_revision", nodeID, "resulting_revision", "integer", fmt.Sprintf("%v", event.Payload["resulting_revision"]), path)
		return workflowTransitionEvent{}, false
	}
	previousState, ok := workflowPayloadString(event.Payload, "previous_state")
	if !ok {
		addWorkflowTransitionDiagnostic(result, "workflow_transition_payload_malformed", "workflow transition payload has invalid previous_state", nodeID, "previous_state", "node state", fmt.Sprintf("%v", event.Payload["previous_state"]), path)
		return workflowTransitionEvent{}, false
	}
	resultingState, ok := workflowPayloadString(event.Payload, "resulting_state")
	if !ok {
		addWorkflowTransitionDiagnostic(result, "workflow_transition_payload_malformed", "workflow transition payload has invalid resulting_state", nodeID, "resulting_state", "node state", fmt.Sprintf("%v", event.Payload["resulting_state"]), path)
		return workflowTransitionEvent{}, false
	}
	if _, ok := event.Payload["dependency_states"].(map[string]any); !ok {
		addWorkflowTransitionDiagnostic(result, "workflow_transition_payload_malformed", "workflow transition payload has invalid dependency_states", nodeID, "dependency_states", "object", fmt.Sprintf("%v", event.Payload["dependency_states"]), path)
		return workflowTransitionEvent{}, false
	}
	if _, ok := event.Payload["ready_before"].([]any); !ok {
		addWorkflowTransitionDiagnostic(result, "workflow_transition_payload_malformed", "workflow transition payload has invalid ready_before", nodeID, "ready_before", "array", fmt.Sprintf("%v", event.Payload["ready_before"]), path)
		return workflowTransitionEvent{}, false
	}
	if _, ok := event.Payload["ready_after"].([]any); !ok {
		addWorkflowTransitionDiagnostic(result, "workflow_transition_payload_malformed", "workflow transition payload has invalid ready_after", nodeID, "ready_after", "array", fmt.Sprintf("%v", event.Payload["ready_after"]), path)
		return workflowTransitionEvent{}, false
	}
	return workflowTransitionEvent{EventID: event.EventID, NodeID: nodeID, Kind: kind, PreviousRevision: previousRevision, ResultingRevision: resultingRevision, PreviousState: previousState, ResultingState: resultingState}, true
}

func applyWorkflowTransition(transition workflowTransitionEvent, node WorkflowInstanceNode, currentStates map[string]string, result *WorkflowTransitionOrderResult, path string) bool {
	current := currentStates[transition.NodeID]
	if transition.PreviousState != current {
		if transition.Kind == "complete" && current != WorkflowNodeRunning {
			addWorkflowTransitionDiagnostic(result, "workflow_transition_complete_without_start", "workflow node completed without a running start", transition.NodeID, "previous_state", WorkflowNodeRunning, current, path)
			return false
		}
		addWorkflowTransitionDiagnostic(result, "workflow_transition_order_invalid", "workflow transition previous state does not match reconstructed state", transition.NodeID, "previous_state", current, transition.PreviousState, path)
		return false
	}
	switch transition.Kind {
	case "start":
		if current == WorkflowNodeSucceeded {
			addWorkflowTransitionDiagnostic(result, "workflow_transition_succeeded_node_restarted", "succeeded workflow node was restarted", transition.NodeID, "previous_state", "not succeeded", current, path)
			return false
		}
		if current != WorkflowNodePending || transition.ResultingState != WorkflowNodeRunning {
			addWorkflowTransitionDiagnostic(result, "workflow_transition_order_invalid", "workflow node start transition is invalid", transition.NodeID, "state", "pending->running", current+"->"+transition.ResultingState, path)
			return false
		}
		for _, dep := range node.DependsOn {
			if currentStates[dep] != WorkflowNodeSucceeded {
				addWorkflowTransitionDiagnostic(result, "workflow_transition_start_before_dependencies", "workflow node started before dependencies succeeded", transition.NodeID, "depends_on", WorkflowNodeSucceeded, dep+"="+currentStates[dep], path)
				return false
			}
		}
	case "complete":
		if current != WorkflowNodeRunning || transition.ResultingState != WorkflowNodeSucceeded {
			addWorkflowTransitionDiagnostic(result, "workflow_transition_complete_without_start", "workflow node completed without a running start", transition.NodeID, "state", "running->succeeded", current+"->"+transition.ResultingState, path)
			return false
		}
		for _, dep := range node.DependsOn {
			if currentStates[dep] != WorkflowNodeSucceeded {
				addWorkflowTransitionDiagnostic(result, "workflow_transition_start_before_dependencies", "workflow node completed before dependencies succeeded", transition.NodeID, "depends_on", WorkflowNodeSucceeded, dep+"="+currentStates[dep], path)
				return false
			}
		}
	case "block":
		if current == WorkflowNodeSucceeded {
			addWorkflowTransitionDiagnostic(result, "workflow_transition_order_invalid", "succeeded workflow node cannot be blocked", transition.NodeID, "previous_state", "not succeeded", current, path)
			return false
		}
		if transition.ResultingState != WorkflowNodeBlocked {
			addWorkflowTransitionDiagnostic(result, "workflow_transition_order_invalid", "workflow node block transition is invalid", transition.NodeID, "resulting_state", WorkflowNodeBlocked, transition.ResultingState, path)
			return false
		}
	default:
		addWorkflowTransitionDiagnostic(result, "workflow_transition_payload_malformed", "workflow transition kind is unsupported", transition.NodeID, "transition_kind", "start, complete, or block", transition.Kind, path)
		return false
	}
	currentStates[transition.NodeID] = transition.ResultingState
	return true
}

func correlateWorkflowTransitionInstance(instance WorkflowInstance, currentStates map[string]string, latestEventByNode map[string]string, expectedPreviousRevision int, result *WorkflowTransitionOrderResult) {
	if instance.Revision != expectedPreviousRevision {
		addWorkflowTransitionDiagnostic(result, "workflow_transition_instance_event_mismatch", "workflow instance revision is not backed by transition ledger", "", "revision", fmt.Sprintf("%d", expectedPreviousRevision), fmt.Sprintf("%d", instance.Revision), result.Path)
	}
	for _, node := range instance.Nodes {
		if currentStates[node.ID] != node.State {
			addWorkflowTransitionDiagnostic(result, "workflow_transition_instance_event_mismatch", "workflow instance node state is not backed by transition ledger", node.ID, "state", currentStates[node.ID], node.State, result.Path)
		}
		latestEventID := latestEventByNode[node.ID]
		lastTransitionEventID := strings.TrimSpace(node.LastTransitionEventID)
		if latestEventID == "" && node.State == WorkflowNodePending && lastTransitionEventID == strings.TrimSpace(instance.CreatedEventID) {
			continue
		}
		if lastTransitionEventID != latestEventID {
			addWorkflowTransitionDiagnostic(result, "workflow_transition_instance_event_mismatch", "workflow instance node LastTransitionEventID is not backed by transition ledger", node.ID, "last_transition_event_id", latestEventID, node.LastTransitionEventID, result.Path)
		}
	}
}

func isWorkflowTransitionEventType(eventType string) bool {
	return eventType == "workflow.node.started" || eventType == "workflow.node.completed" || eventType == "workflow.node.blocked"
}

func workflowPayloadString(payload map[string]any, key string) (string, bool) {
	value, ok := payload[key].(string)
	value = strings.TrimSpace(value)
	return value, ok && value != ""
}

func workflowPayloadInt(payload map[string]any, key string) (int, bool) {
	switch value := payload[key].(type) {
	case float64:
		asInt := int(value)
		return asInt, value == float64(asInt)
	case int:
		return value, true
	default:
		return 0, false
	}
}

func addWorkflowTransitionDiagnostic(result *WorkflowTransitionOrderResult, code string, message string, nodeID string, field string, expected string, actual string, path string) {
	if len(result.Diagnostics) >= maxWorkflowTransitionDiagnostics {
		return
	}
	result.Diagnostics = append(result.Diagnostics, WorkflowDiagnostic{Code: code, Message: message, NodeID: nodeID, Field: field, Expected: expected, Actual: actual, Path: path})
}

func uniqueWorkflowTransitionCodes(diagnostics []WorkflowDiagnostic) []string {
	seen := map[string]bool{}
	codes := []string{}
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == "" || seen[diagnostic.Code] {
			continue
		}
		seen[diagnostic.Code] = true
		codes = append(codes, diagnostic.Code)
	}
	sort.SliceStable(codes, func(i, j int) bool {
		return workflowTransitionCodeRank(codes[i]) < workflowTransitionCodeRank(codes[j])
	})
	return codes
}

func workflowTransitionCodeRank(code string) int {
	ranks := map[string]int{
		"workflow_transition_payload_malformed":         10,
		"workflow_transition_node_unknown":              20,
		"workflow_transition_workflow_mismatch":         30,
		"workflow_transition_revision_stale":            40,
		"workflow_transition_revision_gap":              50,
		"workflow_transition_start_before_dependencies": 60,
		"workflow_transition_complete_without_start":    70,
		"workflow_transition_succeeded_node_restarted":  80,
		"workflow_transition_instance_invalid":          90,
		"workflow_transition_instance_run_mismatch":     100,
		"workflow_transition_instance_event_mismatch":   110,
		"workflow_transition_order_invalid":             120,
	}
	if rank, ok := ranks[code]; ok {
		return rank
	}
	return 1000
}
