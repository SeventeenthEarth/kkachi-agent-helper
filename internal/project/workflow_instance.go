package project

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	WorkflowInstanceVersion = "workflow-instance/v1"
	WorkflowInstanceFile    = "workflow-instance.json"

	WorkflowInstanceStatusPass    = "pass"
	WorkflowInstanceStatusInvalid = "invalid"

	WorkflowNodePending   = "pending"
	WorkflowNodeRunning   = "running"
	WorkflowNodeSucceeded = "succeeded"
	WorkflowNodeBlocked   = "blocked"
)

type WorkflowCreateOptions struct {
	RunID                string
	File                 string
	Catalog              string
	WorkflowID           string
	NodeContractRegistry string
	Now                  func() time.Time
}

type WorkflowRunOptions struct {
	RunID string
}

type WorkflowNodeOptions struct {
	RunID            string
	NodeID           string
	Evidence         string
	Reason           string
	ExpectedRevision *int
	Now              func() time.Time
}

type WorkflowInstanceResult struct {
	Status      string                 `json:"status"`
	OK          bool                   `json:"ok"`
	Reason      string                 `json:"reason"`
	ReasonCodes []string               `json:"reason_codes"`
	RunID       string                 `json:"run_id"`
	EventID     string                 `json:"event_id,omitempty"`
	Instance    *WorkflowInstance      `json:"instance,omitempty"`
	Ready       []WorkflowReadyNode    `json:"ready,omitempty"`
	Diagnostics []WorkflowDiagnostic   `json:"diagnostics,omitempty"`
	TaskDAG     *TaskDAGResult         `json:"task_dag,omitempty"`
	Catalog     *WorkflowCatalogResult `json:"catalog,omitempty"`
}

type WorkflowDiagnostic struct {
	Code     string `json:"code"`
	Message  string `json:"message"`
	NodeID   string `json:"node_id,omitempty"`
	Field    string `json:"field,omitempty"`
	Expected string `json:"expected,omitempty"`
	Actual   string `json:"actual,omitempty"`
	Path     string `json:"path,omitempty"`
}

type WorkflowInstance struct {
	Version        string                 `json:"version"`
	RunID          string                 `json:"run_id"`
	WorkflowID     string                 `json:"workflow_id"`
	SchemaVersion  string                 `json:"schema_version"`
	SourcePath     string                 `json:"source_path"`
	Revision       int                    `json:"revision"`
	CreatedEventID string                 `json:"created_event_id"`
	UpdatedEventID string                 `json:"updated_event_id"`
	Nodes          []WorkflowInstanceNode `json:"nodes"`
}

type WorkflowInstanceNode struct {
	ID                    string   `json:"id"`
	DependsOn             []string `json:"depends_on"`
	Join                  string   `json:"join"`
	RequiredOutputs       []string `json:"required_outputs"`
	State                 string   `json:"state"`
	Evidence              []string `json:"evidence,omitempty"`
	BlockedReason         string   `json:"blocked_reason,omitempty"`
	LastTransitionEventID string   `json:"last_transition_event_id,omitempty"`
}

type WorkflowReadyNode struct {
	ID      string   `json:"id"`
	Reasons []string `json:"reasons"`
}

func CreateWorkflowInstance(root Root, options WorkflowCreateOptions) (WorkflowInstanceResult, error) {
	if options.Now == nil {
		options.Now = func() time.Time { return time.Now().UTC() }
	}
	var result WorkflowInstanceResult
	err := withProjectWriteLock(root, "workflow create", options.RunID, func() error {
		runID, err := ResolveRunID(root, options.RunID)
		if err != nil {
			return err
		}
		if _, _, err := ReadRunMetadata(root, runID); err != nil {
			return err
		}
		taskDAG, catalog, err := workflowCreateTaskDAG(root, options)
		if err != nil {
			return err
		}
		if !taskDAG.OK {
			result = workflowInvalidFromTaskDAG(runID, taskDAG.Reason, taskDAG.ReasonCodes, taskDAG.Diagnostics)
			result.TaskDAG = &taskDAG
			result.Catalog = catalog
			return nil
		}
		if catalog != nil && !catalog.OK {
			result = workflowInvalidFromCatalog(runID, catalog.Reason, catalog.ReasonCodes, catalog.Diagnostics)
			result.Catalog = catalog
			return nil
		}
		path, err := workflowInstancePath(root, runID)
		if err != nil {
			return err
		}
		if _, err := os.Lstat(path.Absolute); err == nil {
			result = workflowInvalid(runID, "workflow_instance_exists", []string{"workflow_instance_exists"}, []WorkflowDiagnostic{{Code: "workflow_instance_exists", Message: "workflow instance already exists for this run", Path: path.Relative, Field: "run_id", Expected: "no existing workflow-instance.json", Actual: runID}})
			return nil
		} else if err != nil && !os.IsNotExist(err) {
			return &Problem{Code: "workflow_instance_inspection_failed", Message: "cannot inspect workflow instance path", Hint: "Check run directory permissions before creating workflow state.", Path: path.Relative, Field: "path", Expected: "inspectable workflow instance path", Actual: err.Error()}
		}

		var instance WorkflowInstance
		appendResult, err := appendEventWithPreparedStatusMutation(root, AppendEventOptions{Type: "workflow.instance.created", RunID: runID, Payload: map[string]any{"run_id": runID, "workflow_id": taskDAG.WorkflowID, "source_path": taskDAG.Path}, Now: options.Now}, func(_ map[string]any, nextID string, _ string) (preparedEventStatusMutation, error) {
			instance = WorkflowInstance{Version: WorkflowInstanceVersion, RunID: runID, WorkflowID: taskDAG.WorkflowID, SchemaVersion: taskDAG.SchemaVersion, SourcePath: taskDAG.Path, Revision: 1, CreatedEventID: nextID, UpdatedEventID: nextID, Nodes: workflowNodesFromTaskDAG(taskDAG.Nodes, nextID)}
			data, err := json.MarshalIndent(instance, "", "  ")
			if err != nil {
				return preparedEventStatusMutation{}, &Problem{Code: "workflow_instance_encode_failed", Message: "cannot encode workflow instance", Hint: "Preserve stderr for diagnosis if this repeats.", Field: "workflow_instance", Expected: "JSON-encodable workflow instance", Actual: err.Error()}
			}
			data = append(data, '\n')
			payload := map[string]any{"run_id": runID, "workflow_id": instance.WorkflowID, "source_path": instance.SourcePath, "revision": instance.Revision}
			return preparedEventStatusMutation{Payload: payload, BeforeAppend: func() error { return writeNewFileAtomically(path, data) }}, nil
		})
		if err != nil {
			return err
		}
		result = workflowPass(runID, "workflow_instance_created", []string{"workflow_instance_created"}, &instance)
		result.EventID = appendResult.EventID
		result.TaskDAG = &taskDAG
		result.Catalog = catalog
		result.Ready = readyWorkflowNodes(instance)
		return nil
	})
	return result, err
}

func workflowCreateTaskDAG(root Root, options WorkflowCreateOptions) (TaskDAGResult, *WorkflowCatalogResult, error) {
	if strings.TrimSpace(options.Catalog) == "" {
		taskDAG, err := ValidateTaskDAG(root, options.File)
		return taskDAG, nil, err
	}
	catalog, err := ValidateWorkflowCatalog(root, WorkflowCatalogOptions{File: options.Catalog, WorkflowID: options.WorkflowID, NodeContractRegistry: options.NodeContractRegistry})
	if err != nil {
		return TaskDAGResult{}, nil, err
	}
	if !catalog.OK {
		return TaskDAGResult{Status: TaskDAGStatusInvalid, OK: false, Reason: catalog.Reason, ReasonCodes: append([]string{}, catalog.ReasonCodes...)}, &catalog, nil
	}
	taskDAG := workflowCatalogSelectedTaskDAG(catalog)
	if taskDAG == nil {
		catalog.addDiagnostic("workflow_catalog_ambiguous_reference", "workflow id did not resolve to a valid task-DAG", options.WorkflowID, catalog.Path, "workflow_id", "exactly one valid workflow", options.WorkflowID, "")
		catalog.finalize()
		return TaskDAGResult{Status: TaskDAGStatusInvalid, OK: false, Reason: catalog.Reason, ReasonCodes: append([]string{}, catalog.ReasonCodes...)}, &catalog, nil
	}
	return *taskDAG, &catalog, nil
}

func ShowWorkflowInstance(root Root, options WorkflowRunOptions) (WorkflowInstanceResult, error) {
	runID, instance, err := readWorkflowInstanceForRun(root, options.RunID)
	if err != nil {
		return WorkflowInstanceResult{}, err
	}
	result := workflowPass(runID, "workflow_instance_loaded", []string{"workflow_instance_loaded"}, &instance)
	result.Ready = readyWorkflowNodes(instance)
	return result, nil
}

func ReadyWorkflowNodes(root Root, options WorkflowRunOptions) (WorkflowInstanceResult, error) {
	runID, instance, err := readWorkflowInstanceForRun(root, options.RunID)
	if err != nil {
		return WorkflowInstanceResult{}, err
	}
	result := workflowPass(runID, "workflow_ready_nodes_computed", []string{"workflow_ready_nodes_computed"}, &instance)
	result.Ready = readyWorkflowNodes(instance)
	return result, nil
}

func StartWorkflowNode(root Root, options WorkflowNodeOptions) (WorkflowInstanceResult, error) {
	return mutateWorkflowNode(root, "workflow node start", "workflow.node.started", options, func(instance *WorkflowInstance, index int) (string, WorkflowDiagnostic, error) {
		node := &instance.Nodes[index]
		if node.State != WorkflowNodePending {
			return "node_transition_invalid", diagnosticForNodeReason("node_transition_invalid", *node), nil
		}
		if !workflowNodeDependenciesSucceeded(*instance, *node) {
			return "node_dependency_unsatisfied", diagnosticForNodeReason("node_dependency_unsatisfied", *node), nil
		}
		node.State = WorkflowNodeRunning
		node.BlockedReason = ""
		return "workflow_node_started", WorkflowDiagnostic{}, nil
	})
}

func CompleteWorkflowNode(root Root, options WorkflowNodeOptions) (WorkflowInstanceResult, error) {
	return mutateWorkflowNode(root, "workflow node complete", "workflow.node.completed", options, func(instance *WorkflowInstance, index int) (string, WorkflowDiagnostic, error) {
		node := &instance.Nodes[index]
		if node.State != WorkflowNodeRunning {
			return "node_transition_invalid", diagnosticForNodeReason("node_transition_invalid", *node), nil
		}
		for _, output := range node.RequiredOutputs {
			path, err := ResolveRelativePath(root, output)
			if err != nil {
				return "node_required_output_missing", WorkflowDiagnostic{Code: "node_required_output_missing", Message: "node required output path is unsafe", NodeID: node.ID, Field: "required_outputs", Expected: "repo-confined existing output file", Actual: output}, nil
			}
			info, err := os.Lstat(path.Absolute)
			if err != nil || info.IsDir() {
				return "node_required_output_missing", WorkflowDiagnostic{Code: "node_required_output_missing", Message: "node required output is missing", NodeID: node.ID, Field: "required_outputs", Expected: "all required output files exist", Actual: output, Path: path.Relative}, nil
			}
		}
		if strings.TrimSpace(options.Evidence) != "" {
			evidence := strings.TrimSpace(options.Evidence)
			path, err := ResolveRelativePath(root, evidence)
			if err != nil {
				return "node_evidence_unsafe", WorkflowDiagnostic{Code: "node_evidence_unsafe", Message: "node completion evidence path is unsafe", NodeID: node.ID, Field: "evidence", Expected: "repo-confined existing evidence file", Actual: evidence}, nil
			}
			info, err := os.Lstat(path.Absolute)
			if err != nil || info.IsDir() {
				return "node_evidence_missing", WorkflowDiagnostic{Code: "node_evidence_missing", Message: "node completion evidence is missing", NodeID: node.ID, Field: "evidence", Expected: "repo-confined existing evidence file", Actual: evidence, Path: path.Relative}, nil
			}
		}
		node.State = WorkflowNodeSucceeded
		node.BlockedReason = ""
		if strings.TrimSpace(options.Evidence) != "" {
			node.Evidence = appendWorkflowUnique(node.Evidence, strings.TrimSpace(options.Evidence))
		}
		return "workflow_node_completed", WorkflowDiagnostic{}, nil
	})
}

func BlockWorkflowNode(root Root, options WorkflowNodeOptions) (WorkflowInstanceResult, error) {
	if strings.TrimSpace(options.Reason) == "" {
		return WorkflowInstanceResult{}, &Problem{Code: "workflow_node_block_reason_required", Message: "workflow node block requires a reason", Hint: "Pass --reason with a concise blocker explanation.", Field: "reason", Expected: "non-empty reason", Actual: "empty"}
	}
	return mutateWorkflowNode(root, "workflow node block", "workflow.node.blocked", options, func(instance *WorkflowInstance, index int) (string, WorkflowDiagnostic, error) {
		node := &instance.Nodes[index]
		if node.State == WorkflowNodeSucceeded {
			return "node_transition_invalid", diagnosticForNodeReason("node_transition_invalid", *node), nil
		}
		node.State = WorkflowNodeBlocked
		node.BlockedReason = strings.TrimSpace(options.Reason)
		return "workflow_node_blocked", WorkflowDiagnostic{}, nil
	})
}

type workflowNodeMutation func(instance *WorkflowInstance, index int) (string, WorkflowDiagnostic, error)

func mutateWorkflowNode(root Root, command string, eventType string, options WorkflowNodeOptions, mutate workflowNodeMutation) (WorkflowInstanceResult, error) {
	if options.Now == nil {
		options.Now = func() time.Time { return time.Now().UTC() }
	}
	var result WorkflowInstanceResult
	err := withProjectWriteLock(root, command, options.RunID, func() error {
		runID, err := ResolveRunID(root, options.RunID)
		if err != nil {
			return err
		}
		path, err := workflowInstancePath(root, runID)
		if err != nil {
			return err
		}
		instance, err := readWorkflowInstance(path)
		if err != nil {
			return err
		}
		if options.ExpectedRevision != nil && instance.Revision != *options.ExpectedRevision {
			result = workflowInvalid(runID, "workflow_instance_stale", []string{"workflow_instance_stale"}, []WorkflowDiagnostic{{Code: "workflow_instance_stale", Message: "workflow instance revision does not match expected revision", Field: "revision", Expected: fmt.Sprintf("%d", *options.ExpectedRevision), Actual: fmt.Sprintf("%d", instance.Revision)}})
			result.Instance = &instance
			result.Ready = readyWorkflowNodes(instance)
			return nil
		}
		index := workflowNodeIndex(instance, options.NodeID)
		if index < 0 {
			result = workflowInvalid(runID, "node_unknown", []string{"node_unknown"}, []WorkflowDiagnostic{{Code: "node_unknown", Message: "workflow node is not present in the instance", NodeID: options.NodeID, Field: "node", Expected: "existing node id", Actual: options.NodeID}})
			result.Instance = &instance
			result.Ready = readyWorkflowNodes(instance)
			return nil
		}
		reason, diagnostic, err := mutate(&instance, index)
		if err != nil {
			return err
		}
		if !strings.HasPrefix(reason, "workflow_node_") {
			result = workflowInvalid(runID, reason, []string{reason}, []WorkflowDiagnostic{diagnostic})
			result.Instance = &instance
			result.Ready = readyWorkflowNodes(instance)
			return nil
		}

		appendResult, err := appendEventWithPreparedStatusMutation(root, AppendEventOptions{Type: eventType, RunID: runID, Payload: map[string]any{"run_id": runID, "node_id": instance.Nodes[index].ID}, Now: options.Now}, func(_ map[string]any, nextID string, _ string) (preparedEventStatusMutation, error) {
			instance.Revision++
			instance.UpdatedEventID = nextID
			instance.Nodes[index].LastTransitionEventID = nextID
			payload := map[string]any{"run_id": runID, "workflow_id": instance.WorkflowID, "node_id": instance.Nodes[index].ID, "state": instance.Nodes[index].State, "revision": instance.Revision}
			if strings.TrimSpace(options.Evidence) != "" {
				payload["evidence"] = strings.TrimSpace(options.Evidence)
			}
			if strings.TrimSpace(options.Reason) != "" {
				payload["reason"] = strings.TrimSpace(options.Reason)
			}
			data, err := json.MarshalIndent(instance, "", "  ")
			if err != nil {
				return preparedEventStatusMutation{}, &Problem{Code: "workflow_instance_encode_failed", Message: "cannot encode workflow instance", Hint: "Preserve stderr for diagnosis if this repeats.", Field: "workflow_instance", Expected: "JSON-encodable workflow instance", Actual: err.Error()}
			}
			data = append(data, '\n')
			return preparedEventStatusMutation{Payload: payload, BeforeAppend: func() error { return writeExistingFileAtomically(path, data) }}, nil
		})
		if err != nil {
			return err
		}
		result = workflowPass(runID, reason, []string{reason}, &instance)
		result.EventID = appendResult.EventID
		result.Ready = readyWorkflowNodes(instance)
		return nil
	})
	return result, err
}

func workflowInstancePath(root Root, runID string) (SafePath, error) {
	if !runIDPattern.MatchString(runID) {
		return SafePath{}, invalidRunField("", "run_id", "run-YYYYMMDDTHHMMSSZ-<12hex>", runID)
	}
	return ResolveRelativePath(root, filepath.ToSlash(filepath.Join(RunRootPath, runID, WorkflowInstanceFile)))
}

func readWorkflowInstanceForRun(root Root, query string) (string, WorkflowInstance, error) {
	runID, err := ResolveRunID(root, query)
	if err != nil {
		return "", WorkflowInstance{}, err
	}
	path, err := workflowInstancePath(root, runID)
	if err != nil {
		return "", WorkflowInstance{}, err
	}
	instance, err := readWorkflowInstance(path)
	return runID, instance, err
}

func readWorkflowInstance(path SafePath) (WorkflowInstance, error) {
	data, err := os.ReadFile(path.Absolute)
	if os.IsNotExist(err) {
		return WorkflowInstance{}, &Problem{Code: "workflow_instance_missing", Message: "workflow instance is missing", Hint: "Run workflow create for this helper run before inspecting or mutating node state.", Path: path.Relative, Field: "path", Expected: "existing workflow-instance.json", Actual: "missing"}
	}
	if err != nil {
		return WorkflowInstance{}, &Problem{Code: "workflow_instance_read_failed", Message: "cannot read workflow instance", Hint: "Check run directory permissions before reading workflow state.", Path: path.Relative, Field: "path", Expected: "readable workflow instance", Actual: err.Error()}
	}
	var instance WorkflowInstance
	if err := json.Unmarshal(data, &instance); err != nil {
		return WorkflowInstance{}, &Problem{Code: "workflow_instance_invalid_json", Message: "workflow instance is not valid JSON", Hint: "Restore workflow-instance.json from audit evidence or recreate the run-local workflow instance.", Path: path.Relative, Field: "json", Expected: "JSON workflow instance", Actual: err.Error()}
	}
	if err := validateWorkflowInstance(instance, path.Relative); err != nil {
		return WorkflowInstance{}, err
	}
	return instance, nil
}

func validateWorkflowInstance(instance WorkflowInstance, relative string) error {
	if instance.Version != WorkflowInstanceVersion {
		return &Problem{Code: "workflow_instance_invalid", Message: "workflow instance has unsupported version", Hint: "Use a workflow instance generated by this helper version.", Path: relative, Field: "version", Expected: WorkflowInstanceVersion, Actual: instance.Version}
	}
	if !runIDPattern.MatchString(instance.RunID) {
		return &Problem{Code: "workflow_instance_invalid", Message: "workflow instance has invalid run id", Hint: "Keep workflow-instance.json scoped to its helper run directory.", Path: relative, Field: "run_id", Expected: "run-YYYYMMDDTHHMMSSZ-<12hex>", Actual: instance.RunID}
	}
	if strings.TrimSpace(instance.WorkflowID) == "" || strings.TrimSpace(instance.SchemaVersion) == "" || strings.TrimSpace(instance.SourcePath) == "" {
		return &Problem{Code: "workflow_instance_invalid", Message: "workflow instance is missing required identity fields", Hint: "Recreate the workflow instance from a valid task DAG.", Path: relative, Field: "workflow_id/schema_version/source_path", Expected: "non-empty identity fields", Actual: "missing"}
	}
	seen := map[string]bool{}
	for _, node := range instance.Nodes {
		if strings.TrimSpace(node.ID) == "" || seen[node.ID] {
			return &Problem{Code: "workflow_instance_invalid", Message: "workflow instance contains invalid node ids", Hint: "Recreate the workflow instance from a valid task DAG.", Path: relative, Field: "nodes.id", Expected: "unique non-empty node ids", Actual: node.ID}
		}
		seen[node.ID] = true
		if !allowed(node.State, WorkflowNodePending, WorkflowNodeRunning, WorkflowNodeSucceeded, WorkflowNodeBlocked) {
			return &Problem{Code: "workflow_instance_invalid", Message: "workflow instance contains unsupported node state", Hint: "Use supported DAGSM-002 node states only.", Path: relative, Field: "nodes.state", Expected: "pending, running, succeeded, or blocked", Actual: node.State}
		}
	}
	return nil
}

func workflowNodesFromTaskDAG(nodes []TaskDAGNodeSummary, eventID string) []WorkflowInstanceNode {
	result := make([]WorkflowInstanceNode, 0, len(nodes))
	for _, node := range nodes {
		result = append(result, WorkflowInstanceNode{ID: node.ID, DependsOn: append([]string{}, node.DependsOn...), Join: node.Join, RequiredOutputs: append([]string{}, node.RequiredOutputs...), State: WorkflowNodePending, LastTransitionEventID: eventID})
	}
	return result
}

func workflowPass(runID string, reason string, codes []string, instance *WorkflowInstance) WorkflowInstanceResult {
	return WorkflowInstanceResult{Status: WorkflowInstanceStatusPass, OK: true, Reason: reason, ReasonCodes: codes, RunID: runID, Instance: instance}
}

func workflowInvalid(runID string, reason string, codes []string, diagnostics []WorkflowDiagnostic) WorkflowInstanceResult {
	return WorkflowInstanceResult{Status: WorkflowInstanceStatusInvalid, OK: false, Reason: reason, ReasonCodes: codes, RunID: runID, Diagnostics: diagnostics}
}

func workflowInvalidFromTaskDAG(runID string, reason string, codes []string, diagnostics []TaskDAGDiagnostic) WorkflowInstanceResult {
	result := workflowInvalid(runID, reason, codes, nil)
	for _, diagnostic := range diagnostics {
		result.Diagnostics = append(result.Diagnostics, WorkflowDiagnostic{Code: diagnostic.Code, Message: diagnostic.Message, NodeID: diagnostic.NodeID, Field: diagnostic.Field, Actual: diagnostic.Value, Path: diagnostic.Path})
	}
	return result
}

func workflowInvalidFromCatalog(runID string, reason string, codes []string, diagnostics []WorkflowCatalogDiagnostic) WorkflowInstanceResult {
	result := workflowInvalid(runID, reason, codes, nil)
	for _, diagnostic := range diagnostics {
		result.Diagnostics = append(result.Diagnostics, WorkflowDiagnostic{Code: diagnostic.Code, Message: diagnostic.Message, NodeID: diagnostic.NodeID, Field: diagnostic.Field, Expected: diagnostic.Expected, Actual: diagnostic.Actual, Path: diagnostic.Path})
	}
	return result
}

func readyWorkflowNodes(instance WorkflowInstance) []WorkflowReadyNode {
	ready := []WorkflowReadyNode{}
	for _, node := range instance.Nodes {
		if node.State != WorkflowNodePending {
			continue
		}
		if workflowNodeDependenciesSucceeded(instance, node) {
			ready = append(ready, WorkflowReadyNode{ID: node.ID, Reasons: []string{"dependencies_satisfied", "state_pending"}})
		}
	}
	sort.Slice(ready, func(i, j int) bool { return ready[i].ID < ready[j].ID })
	return ready
}

func workflowNodeDependenciesSucceeded(instance WorkflowInstance, node WorkflowInstanceNode) bool {
	states := map[string]string{}
	for _, candidate := range instance.Nodes {
		states[candidate.ID] = candidate.State
	}
	for _, dep := range node.DependsOn {
		if states[dep] != WorkflowNodeSucceeded {
			return false
		}
	}
	return true
}

func workflowNodeIndex(instance WorkflowInstance, nodeID string) int {
	nodeID = strings.TrimSpace(nodeID)
	for i, node := range instance.Nodes {
		if node.ID == nodeID {
			return i
		}
	}
	return -1
}

func diagnosticForNodeReason(reason string, node WorkflowInstanceNode) WorkflowDiagnostic {
	switch reason {
	case "node_dependency_unsatisfied":
		return WorkflowDiagnostic{Code: reason, Message: "node dependencies are not all succeeded", NodeID: node.ID, Field: "depends_on", Expected: "all dependencies succeeded", Actual: strings.Join(node.DependsOn, ",")}
	case "node_required_output_missing":
		return WorkflowDiagnostic{Code: reason, Message: "node required output is missing", NodeID: node.ID, Field: "required_outputs", Expected: "all required output files exist", Actual: strings.Join(node.RequiredOutputs, ",")}
	default:
		return WorkflowDiagnostic{Code: reason, Message: "node transition is invalid", NodeID: node.ID, Field: "state", Expected: "legal DAGSM-002 transition", Actual: node.State}
	}
}

func appendWorkflowUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func WorkflowInstanceHumanSummary(result WorkflowInstanceResult) string {
	if result.Instance == nil {
		return fmt.Sprintf("workflow instance %s: %s (%s)\n", result.RunID, result.Status, result.Reason)
	}
	return fmt.Sprintf("workflow instance %s/%s: %s (%s) revision=%d ready=%d\n", result.RunID, result.Instance.WorkflowID, result.Status, result.Reason, result.Instance.Revision, len(result.Ready))
}
