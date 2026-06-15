package project

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	WorkflowCatalogSchemaVersion      = "workflow-catalog/v1"
	WorkflowCatalogDefaultPath        = ".kkachi/workflow-catalog.yaml"
	NodeContractRegistrySchemaVersion = "kas-task-dag-workflow-registry/v1"

	WorkflowCatalogStatusPass    = "pass"
	WorkflowCatalogStatusFail    = "fail"
	WorkflowCatalogStatusMissing = "missing"
)

type WorkflowCatalogOptions struct {
	File                 string
	WorkflowID           string
	NodeContractRegistry string
}

type WorkflowCatalogResult struct {
	SchemaVersion      string                      `json:"schema_version"`
	Status             string                      `json:"status"`
	OK                 bool                        `json:"ok"`
	Reason             string                      `json:"reason"`
	ReasonCodes        []string                    `json:"reason_codes"`
	CatalogID          string                      `json:"catalog_id,omitempty"`
	Path               string                      `json:"path"`
	SelectedWorkflowID string                      `json:"selected_workflow_id,omitempty"`
	Diagnostics        []WorkflowCatalogDiagnostic `json:"diagnostics"`
	Workflows          []WorkflowCatalogWorkflow   `json:"workflows,omitempty"`
	NextAction         string                      `json:"next_action"`
}

type WorkflowCatalogDiagnostic struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	WorkflowID string `json:"workflow_id,omitempty"`
	NodeID     string `json:"node_id,omitempty"`
	Path       string `json:"path,omitempty"`
	Field      string `json:"field,omitempty"`
	Expected   string `json:"expected,omitempty"`
	Actual     string `json:"actual,omitempty"`
}

type WorkflowCatalogWorkflow struct {
	WorkflowID           string                      `json:"workflow_id"`
	Path                 string                      `json:"path"`
	SchemaVersion        string                      `json:"schema_version"`
	NodeContractRegistry string                      `json:"node_contract_registry,omitempty"`
	TaskDAG              *TaskDAGResult              `json:"task_dag,omitempty"`
	NodeContracts        *NodeContractRegistryResult `json:"node_contract_registry_result,omitempty"`
}

type NodeContractRegistryResult struct {
	SchemaVersion string                      `json:"schema_version"`
	Status        string                      `json:"status"`
	OK            bool                        `json:"ok"`
	Reason        string                      `json:"reason"`
	ReasonCodes   []string                    `json:"reason_codes"`
	Path          string                      `json:"path"`
	Diagnostics   []WorkflowCatalogDiagnostic `json:"diagnostics"`
	Contracts     []NodeContractSummary       `json:"contracts,omitempty"`
}

type NodeContractSummary struct {
	WorkflowID          string `json:"workflow_id"`
	NodeID              string `json:"node_id"`
	TaskClass           string `json:"task_class"`
	CompletionAuthority string `json:"completion_authority"`
	DirectKAHStateWrite string `json:"direct_kah_state_write"`
}

type parsedWorkflowCatalog struct {
	SchemaVersion string
	CatalogID     string
	Workflows     []WorkflowCatalogWorkflow
}

type parsedNodeContractRegistry struct {
	SchemaVersion string
	Contracts     []NodeContractSummary
}

func ValidateWorkflowCatalog(root Root, options WorkflowCatalogOptions) (WorkflowCatalogResult, error) {
	file := strings.TrimSpace(options.File)
	if file == "" {
		file = WorkflowCatalogDefaultPath
	}
	result := WorkflowCatalogResult{Status: WorkflowCatalogStatusFail, Path: file, SelectedWorkflowID: strings.TrimSpace(options.WorkflowID), Diagnostics: []WorkflowCatalogDiagnostic{}}
	path, err := ResolveRelativePath(root, file)
	if err != nil {
		result.addDiagnostic("workflow_catalog_unsafe_path", "workflow catalog path is unsafe", "", file, "file", "repository-confined workflow catalog path", err.Error(), "")
		result.finalize()
		return result, nil
	}
	result.Path = path.Relative
	data, err := os.ReadFile(path.Absolute)
	if errors.Is(err, os.ErrNotExist) {
		result.addDiagnostic("workflow_catalog_missing", "workflow catalog file is missing", "", path.Relative, "file", "existing workflow catalog file", "missing", "")
		result.finalize()
		return result, nil
	}
	if err != nil {
		result.addDiagnostic("workflow_catalog_missing", "workflow catalog file cannot be read", "", path.Relative, "file", "readable workflow catalog file", err.Error(), "")
		result.finalize()
		return result, nil
	}
	parsed, diagnostics := parseWorkflowCatalogYAML(path.Relative, string(data))
	result.SchemaVersion = parsed.SchemaVersion
	result.CatalogID = parsed.CatalogID
	result.Workflows = append([]WorkflowCatalogWorkflow{}, parsed.Workflows...)
	result.Diagnostics = append(result.Diagnostics, diagnostics...)
	validateParsedWorkflowCatalog(root, options, &result)
	result.finalize()
	return result, nil
}

func parseWorkflowCatalogYAML(path string, content string) (parsedWorkflowCatalog, []WorkflowCatalogDiagnostic) {
	parsed := parsedWorkflowCatalog{}
	diagnostics := []WorkflowCatalogDiagnostic{}
	topSeen := map[string]bool{}
	inWorkflows := false
	var current *WorkflowCatalogWorkflow
	var seen map[string]bool
	finish := func() {
		if current != nil {
			parsed.Workflows = append(parsed.Workflows, *current)
			current = nil
			seen = nil
		}
	}
	add := func(code, message, workflowID, field, expected, actual string) {
		diagnostics = append(diagnostics, WorkflowCatalogDiagnostic{Code: code, Message: message, WorkflowID: workflowID, Path: path, Field: field, Expected: expected, Actual: actual})
	}
	for _, raw := range strings.Split(content, "\n") {
		line := stripTaskDAGComment(raw)
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.Contains(line, "\t") {
			add("workflow_catalog_parse_error", "tabs are not supported in workflow catalog YAML", "", "yaml", "space-indented YAML", strings.TrimSpace(line))
			continue
		}
		indent := taskDAGLeadingSpaces(line)
		trimmed := strings.TrimSpace(line)
		if inWorkflows && strings.HasPrefix(trimmed, "- ") {
			finish()
			current = &WorkflowCatalogWorkflow{}
			seen = map[string]bool{}
			rest := strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
			if rest != "" {
				key, value, ok := splitTaskDAGKeyValue(rest)
				if !ok {
					add("workflow_catalog_parse_error", "workflow item must be a key/value mapping", "", "workflows", "key/value workflow item", rest)
					continue
				}
				applyWorkflowCatalogField(current, seen, key, value, path, &diagnostics)
			}
			continue
		}
		key, value, ok := splitTaskDAGKeyValue(trimmed)
		if !ok {
			add("workflow_catalog_parse_error", "line is not a supported key/value mapping", "", "yaml", "key/value mapping", trimmed)
			continue
		}
		if !inWorkflows || indent == 0 {
			if inWorkflows && key != "workflows" && indent == 0 {
				finish()
				inWorkflows = false
			}
			if topSeen[key] {
				add("workflow_catalog_invalid_schema", "duplicate top-level field", "", key, "field appears once", value)
				continue
			}
			topSeen[key] = true
			switch key {
			case "schema_version":
				parsed.SchemaVersion = parseTaskDAGScalar(value)
			case "catalog_id":
				parsed.CatalogID = parseTaskDAGScalar(value)
			case "workflows":
				inWorkflows = true
				if strings.TrimSpace(value) != "" {
					add("workflow_catalog_invalid_schema", "workflows must be a block list", "", "workflows", "block list", value)
				}
			default:
				add("workflow_catalog_invalid_schema", "unsupported top-level field", "", key, "schema_version, catalog_id, or workflows", key)
			}
			continue
		}
		if current == nil {
			add("workflow_catalog_parse_error", "workflow field appears before a workflow item", "", key, "workflow list item", value)
			continue
		}
		applyWorkflowCatalogField(current, seen, key, value, path, &diagnostics)
	}
	finish()
	return parsed, diagnostics
}

func applyWorkflowCatalogField(workflow *WorkflowCatalogWorkflow, seen map[string]bool, key string, value string, path string, diagnostics *[]WorkflowCatalogDiagnostic) {
	if seen[key] {
		*diagnostics = append(*diagnostics, WorkflowCatalogDiagnostic{Code: "workflow_catalog_invalid_schema", Message: "duplicate workflow field", WorkflowID: workflow.WorkflowID, Path: path, Field: key, Expected: "field appears once", Actual: value})
		return
	}
	seen[key] = true
	parsed := parseTaskDAGScalar(value)
	switch key {
	case "workflow_id":
		workflow.WorkflowID = parsed
	case "path":
		workflow.Path = parsed
	case "schema_version":
		workflow.SchemaVersion = parsed
	case "node_contract_registry":
		workflow.NodeContractRegistry = parsed
	default:
		*diagnostics = append(*diagnostics, WorkflowCatalogDiagnostic{Code: "workflow_catalog_invalid_schema", Message: "unsupported workflow field", WorkflowID: workflow.WorkflowID, Path: path, Field: key, Expected: "workflow_id, path, schema_version, or node_contract_registry", Actual: key})
	}
}

func validateParsedWorkflowCatalog(root Root, options WorkflowCatalogOptions, result *WorkflowCatalogResult) {
	if result.SchemaVersion != WorkflowCatalogSchemaVersion {
		result.addDiagnostic("workflow_catalog_invalid_schema", "unsupported workflow catalog schema version", "", result.Path, "schema_version", WorkflowCatalogSchemaVersion, result.SchemaVersion, "")
	}
	if strings.TrimSpace(result.CatalogID) == "" {
		result.addDiagnostic("workflow_catalog_invalid_schema", "catalog_id is required", "", result.Path, "catalog_id", "non-empty catalog id", "missing", "")
	}
	if len(result.Workflows) == 0 {
		result.addDiagnostic("workflow_catalog_invalid_schema", "workflows must contain at least one workflow", "", result.Path, "workflows", "one or more workflows", "missing", "")
	}
	ids := map[string]int{}
	for i := range result.Workflows {
		workflow := &result.Workflows[i]
		workflow.WorkflowID = strings.TrimSpace(workflow.WorkflowID)
		workflow.Path = strings.TrimSpace(workflow.Path)
		workflow.SchemaVersion = strings.TrimSpace(workflow.SchemaVersion)
		workflow.NodeContractRegistry = strings.TrimSpace(workflow.NodeContractRegistry)
		if workflow.WorkflowID == "" {
			result.addDiagnostic("workflow_catalog_invalid_schema", "workflow_id is required", "", result.Path, "workflows.workflow_id", "non-empty workflow id", "missing", "")
		} else {
			ids[workflow.WorkflowID]++
			if ids[workflow.WorkflowID] > 1 {
				result.addDiagnostic("workflow_catalog_duplicate_workflow", "duplicate workflow id", workflow.WorkflowID, result.Path, "workflows.workflow_id", "unique workflow id", workflow.WorkflowID, "")
			}
		}
		if workflow.Path == "" {
			result.addDiagnostic("workflow_catalog_invalid_schema", "workflow path is required", workflow.WorkflowID, result.Path, "workflows.path", "repository-confined task-DAG path", "missing", "")
			continue
		}
		if _, err := ResolveRelativePath(root, workflow.Path); err != nil {
			result.addDiagnostic("workflow_catalog_unsafe_path", "workflow path is unsafe", workflow.WorkflowID, workflow.Path, "workflows.path", "repository-confined task-DAG path", err.Error(), "")
			continue
		}
		if workflow.SchemaVersion != "task-dag/v1" {
			result.addDiagnostic("workflow_catalog_invalid_schema", "workflow schema version is unsupported", workflow.WorkflowID, workflow.Path, "workflows.schema_version", "task-dag/v1", workflow.SchemaVersion, "")
		}
		taskDAG, err := ValidateTaskDAG(root, workflow.Path)
		if err != nil {
			result.addDiagnostic("workflow_catalog_workflow_invalid", "workflow task-DAG cannot be validated", workflow.WorkflowID, workflow.Path, "workflows.path", "valid task-DAG", err.Error(), "")
			continue
		}
		workflow.TaskDAG = &taskDAG
		if !taskDAG.OK {
			code := "workflow_catalog_workflow_invalid"
			if taskDAG.Reason == "task_dag_missing" {
				code = "workflow_catalog_workflow_missing"
			}
			result.addDiagnostic(code, "workflow task-DAG is not valid", workflow.WorkflowID, workflow.Path, "workflows.path", "valid task-DAG", taskDAG.Reason, "")
		}
		if taskDAG.WorkflowID != "" && workflow.WorkflowID != "" && taskDAG.WorkflowID != workflow.WorkflowID {
			result.addDiagnostic("workflow_catalog_workflow_id_mismatch", "catalog workflow id does not match task-DAG workflow_id", workflow.WorkflowID, workflow.Path, "workflow_id", workflow.WorkflowID, taskDAG.WorkflowID, "")
		}
		registryPath := strings.TrimSpace(options.NodeContractRegistry)
		if registryPath == "" {
			registryPath = workflow.NodeContractRegistry
		}
		if registryPath != "" {
			registry, err := ValidateNodeContractRegistry(root, registryPath, workflow.WorkflowID, taskDAG.Nodes)
			if err != nil {
				result.addDiagnostic("node_contract_registry_unreadable", "node contract registry cannot be validated", workflow.WorkflowID, registryPath, "node_contract_registry", "valid node contract registry", err.Error(), "")
				continue
			}
			workflow.NodeContracts = &registry
			if !registry.OK {
				for _, diagnostic := range registry.Diagnostics {
					result.Diagnostics = append(result.Diagnostics, diagnostic)
				}
			}
		}
	}
	if selected := strings.TrimSpace(options.WorkflowID); selected != "" {
		matches := 0
		for _, workflow := range result.Workflows {
			if workflow.WorkflowID == selected {
				matches++
			}
		}
		if matches != 1 {
			result.addDiagnostic("workflow_catalog_ambiguous_reference", "workflow id did not resolve to exactly one catalog entry", selected, result.Path, "workflow_id", "exactly one matching workflow", selected, "")
		}
	}
}

func ValidateNodeContractRegistry(root Root, file string, workflowID string, nodes []TaskDAGNodeSummary) (NodeContractRegistryResult, error) {
	result := NodeContractRegistryResult{Status: WorkflowCatalogStatusFail, Path: strings.TrimSpace(file), Diagnostics: []WorkflowCatalogDiagnostic{}}
	path, err := ResolveRelativePath(root, file)
	if err != nil {
		result.addDiagnostic("node_contract_registry_unreadable", "node contract registry path is unsafe", workflowID, file, "node_contract_registry", "repository-confined registry path", err.Error(), "")
		result.finalize()
		return result, nil
	}
	result.Path = path.Relative
	data, err := os.ReadFile(path.Absolute)
	if errors.Is(err, os.ErrNotExist) {
		result.addDiagnostic("node_contract_registry_missing", "node contract registry file is missing", workflowID, path.Relative, "node_contract_registry", "existing registry file", "missing", "")
		result.finalize()
		return result, nil
	}
	if err != nil {
		result.addDiagnostic("node_contract_registry_unreadable", "node contract registry file cannot be read", workflowID, path.Relative, "node_contract_registry", "readable registry file", err.Error(), "")
		result.finalize()
		return result, nil
	}
	parsed, diagnostics := parseNodeContractRegistryYAML(path.Relative, string(data))
	result.SchemaVersion = parsed.SchemaVersion
	result.Contracts = append([]NodeContractSummary{}, parsed.Contracts...)
	result.Diagnostics = append(result.Diagnostics, diagnostics...)
	validateParsedNodeContractRegistry(workflowID, nodes, &result)
	result.finalize()
	return result, nil
}

func parseNodeContractRegistryYAML(path string, content string) (parsedNodeContractRegistry, []WorkflowCatalogDiagnostic) {
	parsed := parsedNodeContractRegistry{}
	diagnostics := []WorkflowCatalogDiagnostic{}
	inContracts := false
	ignoredSection := ""
	var current *NodeContractSummary
	var seen map[string]bool
	finish := func() {
		if current != nil {
			parsed.Contracts = append(parsed.Contracts, *current)
			current = nil
			seen = nil
		}
	}
	add := func(code, message, workflowID, nodeID, field, expected, actual string) {
		diagnostics = append(diagnostics, WorkflowCatalogDiagnostic{Code: code, Message: message, WorkflowID: workflowID, NodeID: nodeID, Path: path, Field: field, Expected: expected, Actual: actual})
	}
	for _, raw := range strings.Split(content, "\n") {
		line := stripTaskDAGComment(raw)
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.Contains(line, "\t") {
			add("node_contract_registry_schema_unsupported", "tabs are not supported in node contract registry YAML", "", "", "yaml", "space-indented YAML", strings.TrimSpace(line))
			continue
		}
		indent := taskDAGLeadingSpaces(line)
		trimmed := strings.TrimSpace(line)
		if inContracts && strings.HasPrefix(trimmed, "- ") {
			finish()
			current = &NodeContractSummary{}
			seen = map[string]bool{}
			rest := strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
			if rest != "" {
				key, value, ok := splitTaskDAGKeyValue(rest)
				if !ok {
					add("node_contract_registry_schema_unsupported", "node contract item must be a key/value mapping", "", "", "node_contracts", "key/value node contract", rest)
					continue
				}
				applyNodeContractField(current, seen, key, value, path, &diagnostics)
			}
			continue
		}
		key, value, ok := splitTaskDAGKeyValue(trimmed)
		if !ok {
			if ignoredSection != "" {
				continue
			}
			add("node_contract_registry_schema_unsupported", "line is not a supported key/value mapping", "", "", "yaml", "key/value mapping", trimmed)
			continue
		}
		if indent == 0 {
			if inContracts && key != "node_contracts" {
				finish()
				inContracts = false
			}
			ignoredSection = ""
			switch key {
			case "schema_version":
				parsed.SchemaVersion = parseTaskDAGScalar(value)
			case "node_contracts":
				inContracts = true
			default:
				// KAS owns selector metadata. KAH ignores unknown top-level sections
				// instead of treating selector policy as helper-owned behavior.
				ignoredSection = key
			}
			continue
		}
		if ignoredSection != "" && !inContracts {
			continue
		}
		if current == nil {
			continue
		}
		applyNodeContractField(current, seen, key, value, path, &diagnostics)
	}
	finish()
	return parsed, diagnostics
}

func applyNodeContractField(contract *NodeContractSummary, seen map[string]bool, key string, value string, path string, diagnostics *[]WorkflowCatalogDiagnostic) {
	if seen[key] {
		*diagnostics = append(*diagnostics, WorkflowCatalogDiagnostic{Code: "node_contract_duplicate", Message: "duplicate node contract field", WorkflowID: contract.WorkflowID, NodeID: contract.NodeID, Path: path, Field: key, Expected: "field appears once", Actual: value})
		return
	}
	seen[key] = true
	parsed := parseTaskDAGScalar(value)
	switch key {
	case "workflow_id":
		contract.WorkflowID = parsed
	case "node_id":
		contract.NodeID = parsed
	case "task_class":
		contract.TaskClass = parsed
	case "completion_authority":
		contract.CompletionAuthority = parsed
	case "direct_kah_state_write":
		contract.DirectKAHStateWrite = parsed
	default:
		// Extra KAS-owned contract fields are allowed; KAH validates only the
		// deterministic safety fields it consumes as evidence.
	}
}

func validateParsedNodeContractRegistry(workflowID string, nodes []TaskDAGNodeSummary, result *NodeContractRegistryResult) {
	if result.SchemaVersion != NodeContractRegistrySchemaVersion {
		result.addDiagnostic("node_contract_registry_schema_unsupported", "node contract registry schema version is unsupported", workflowID, result.Path, "schema_version", NodeContractRegistrySchemaVersion, result.SchemaVersion, "")
	}
	seen := map[string]bool{}
	contractsByNode := map[string]bool{}
	for _, contract := range result.Contracts {
		if contract.WorkflowID == "" || contract.NodeID == "" {
			result.addDiagnostic("node_contract_missing", "node contract is missing workflow_id or node_id", contract.WorkflowID, result.Path, "workflow_id,node_id", "non-empty workflow and node ids", "missing", contract.NodeID)
			continue
		}
		key := contract.WorkflowID + "/" + contract.NodeID
		if seen[key] {
			result.addDiagnostic("node_contract_duplicate", "duplicate node contract", contract.WorkflowID, result.Path, "node_contracts", "unique workflow_id/node_id pair", key, contract.NodeID)
		}
		seen[key] = true
		if contract.WorkflowID != workflowID {
			continue
		}
		contractsByNode[contract.NodeID] = true
		if strings.TrimSpace(contract.TaskClass) == "" {
			result.addDiagnostic("node_contract_task_class_missing", "node contract task_class is required", contract.WorkflowID, result.Path, "task_class", "non-empty task class", "missing", contract.NodeID)
		}
		if contract.CompletionAuthority != "kah_only" {
			result.addDiagnostic("node_contract_completion_authority_invalid", "node contract completion authority must stay with KAH", contract.WorkflowID, result.Path, "completion_authority", "kah_only", contract.CompletionAuthority, contract.NodeID)
		}
		if strings.ToLower(strings.TrimSpace(contract.DirectKAHStateWrite)) != "false" {
			result.addDiagnostic("node_contract_direct_kah_state_write_forbidden", "node contract must not permit direct KAH state writes", contract.WorkflowID, result.Path, "direct_kah_state_write", "false", contract.DirectKAHStateWrite, contract.NodeID)
		}
	}
	nodeIDs := map[string]bool{}
	for _, node := range nodes {
		nodeIDs[node.ID] = true
		if !contractsByNode[node.ID] {
			result.addDiagnostic("node_contract_missing", "node contract is missing for task-DAG node", workflowID, result.Path, "node_contracts.node_id", node.ID, "missing", node.ID)
		}
	}
	for nodeID := range contractsByNode {
		if !nodeIDs[nodeID] {
			result.addDiagnostic("node_contract_unknown_node", "node contract references a node not present in the task DAG", workflowID, result.Path, "node_contracts.node_id", "task-DAG node id", nodeID, nodeID)
		}
	}
}

func (r *WorkflowCatalogResult) addDiagnostic(code, message, workflowID, path, field, expected, actual, nodeID string) {
	r.Diagnostics = append(r.Diagnostics, WorkflowCatalogDiagnostic{Code: code, Message: message, WorkflowID: workflowID, NodeID: nodeID, Path: path, Field: field, Expected: expected, Actual: actual})
}

func (r *WorkflowCatalogResult) finalize() {
	if len(r.Diagnostics) == 0 {
		r.Status = WorkflowCatalogStatusPass
		r.OK = true
		r.Reason = "workflow_catalog_valid"
		r.ReasonCodes = workflowCatalogSuccessReasonCodes(r.Workflows)
		r.NextAction = "Catalog validates; KAS may use explicit workflow_id inputs only after its own selector policy resolves exactly one workflow."
		return
	}
	if containsWorkflowCatalogCode(r.Diagnostics, "workflow_catalog_missing") {
		r.Status = WorkflowCatalogStatusMissing
	} else {
		r.Status = WorkflowCatalogStatusFail
	}
	r.OK = false
	r.ReasonCodes = uniqueWorkflowCatalogCodes(r.Diagnostics)
	r.Reason = primaryWorkflowCatalogReason(r.ReasonCodes)
	r.NextAction = "Fail closed for task-DAG catalog use; repair the catalog, workflow files, or optional node-contract registry evidence before creating workflow instances."
}

func workflowCatalogSuccessReasonCodes(workflows []WorkflowCatalogWorkflow) []string {
	codes := []string{"workflow_catalog_valid"}
	for _, workflow := range workflows {
		if workflow.NodeContracts != nil && workflow.NodeContracts.OK {
			codes = appendWorkflowUnique(codes, "node_contract_registry_valid")
		}
	}
	return codes
}

func (r *NodeContractRegistryResult) addDiagnostic(code, message, workflowID, path, field, expected, actual, nodeID string) {
	r.Diagnostics = append(r.Diagnostics, WorkflowCatalogDiagnostic{Code: code, Message: message, WorkflowID: workflowID, NodeID: nodeID, Path: path, Field: field, Expected: expected, Actual: actual})
}

func (r *NodeContractRegistryResult) finalize() {
	if len(r.Diagnostics) == 0 {
		r.Status = WorkflowCatalogStatusPass
		r.OK = true
		r.Reason = "node_contract_registry_valid"
		r.ReasonCodes = []string{"node_contract_registry_valid"}
		return
	}
	if containsWorkflowCatalogCode(r.Diagnostics, "node_contract_registry_missing") {
		r.Status = WorkflowCatalogStatusMissing
	} else {
		r.Status = WorkflowCatalogStatusFail
	}
	r.OK = false
	r.ReasonCodes = uniqueWorkflowCatalogCodes(r.Diagnostics)
	r.Reason = primaryWorkflowCatalogReason(r.ReasonCodes)
}

func containsWorkflowCatalogCode(diagnostics []WorkflowCatalogDiagnostic, code string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code {
			return true
		}
	}
	return false
}

func uniqueWorkflowCatalogCodes(diagnostics []WorkflowCatalogDiagnostic) []string {
	seen := map[string]bool{}
	codes := []string{}
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == "" || seen[diagnostic.Code] {
			continue
		}
		seen[diagnostic.Code] = true
		codes = append(codes, diagnostic.Code)
	}
	sort.SliceStable(codes, func(i, j int) bool { return workflowCatalogReasonRank(codes[i]) < workflowCatalogReasonRank(codes[j]) })
	return codes
}

func primaryWorkflowCatalogReason(codes []string) string {
	if len(codes) == 0 {
		return "workflow_catalog_valid"
	}
	return codes[0]
}

func workflowCatalogReasonRank(code string) int {
	ranks := map[string]int{
		"workflow_catalog_missing":                       10,
		"workflow_catalog_unsafe_path":                   20,
		"workflow_catalog_parse_error":                   30,
		"workflow_catalog_invalid_schema":                40,
		"workflow_catalog_duplicate_workflow":            50,
		"workflow_catalog_ambiguous_reference":           60,
		"workflow_catalog_workflow_missing":              70,
		"workflow_catalog_workflow_invalid":              80,
		"workflow_catalog_workflow_id_mismatch":          90,
		"node_contract_registry_missing":                 100,
		"node_contract_registry_unreadable":              110,
		"node_contract_registry_schema_unsupported":      120,
		"node_contract_duplicate":                        130,
		"node_contract_missing":                          140,
		"node_contract_unknown_node":                     150,
		"node_contract_task_class_missing":               160,
		"node_contract_completion_authority_invalid":     170,
		"node_contract_direct_kah_state_write_forbidden": 180,
	}
	if rank, ok := ranks[code]; ok {
		return rank
	}
	return 1000
}

func workflowCatalogSelectedTaskDAG(result WorkflowCatalogResult) *TaskDAGResult {
	for _, workflow := range result.Workflows {
		if workflow.WorkflowID == result.SelectedWorkflowID && workflow.TaskDAG != nil {
			return workflow.TaskDAG
		}
	}
	return nil
}

type WorkflowInstanceCompletenessResult struct {
	Status      string               `json:"status"`
	OK          bool                 `json:"ok"`
	Reason      string               `json:"reason"`
	ReasonCodes []string             `json:"reason_codes"`
	RunID       string               `json:"run_id,omitempty"`
	Path        string               `json:"path,omitempty"`
	WorkflowID  string               `json:"workflow_id,omitempty"`
	Revision    int                  `json:"revision,omitempty"`
	Ready       []WorkflowReadyNode  `json:"ready,omitempty"`
	Diagnostics []WorkflowDiagnostic `json:"diagnostics,omitempty"`
}

func CheckWorkflowInstanceCompleteness(root Root, runID string) (WorkflowInstanceCompletenessResult, error) {
	resolved, err := ResolveRunID(root, runID)
	if err != nil {
		return WorkflowInstanceCompletenessResult{}, err
	}
	path, err := workflowInstancePath(root, resolved)
	if err != nil {
		return WorkflowInstanceCompletenessResult{}, err
	}
	result := WorkflowInstanceCompletenessResult{RunID: resolved, Path: path.Relative, Diagnostics: []WorkflowDiagnostic{}}
	if _, err := os.Lstat(path.Absolute); errors.Is(err, os.ErrNotExist) {
		result.Status = WorkflowCatalogStatusMissing
		result.OK = true
		result.Reason = "workflow_instance_missing"
		result.ReasonCodes = []string{"workflow_instance_missing"}
		return result, nil
	} else if err != nil {
		result.Status = WorkflowCatalogStatusFail
		result.OK = false
		result.Reason = "workflow_instance_invalid"
		result.ReasonCodes = []string{"workflow_instance_invalid"}
		result.Diagnostics = []WorkflowDiagnostic{{Code: "workflow_instance_invalid", Message: "cannot inspect workflow instance", Field: "path", Expected: "inspectable workflow instance", Actual: err.Error(), Path: path.Relative}}
		return result, nil
	}
	instance, err := readWorkflowInstance(path)
	if err != nil {
		result.Status = WorkflowCatalogStatusFail
		result.OK = false
		result.Reason = "workflow_instance_invalid"
		result.ReasonCodes = []string{"workflow_instance_invalid"}
		result.Diagnostics = []WorkflowDiagnostic{{Code: "workflow_instance_invalid", Message: "workflow instance is invalid", Field: "workflow_instance", Expected: "valid workflow-instance.json", Actual: err.Error(), Path: path.Relative}}
		return result, nil
	}
	result.WorkflowID = instance.WorkflowID
	result.Revision = instance.Revision
	result.Ready = readyWorkflowNodes(instance)
	for _, node := range instance.Nodes {
		if node.State != WorkflowNodeSucceeded {
			result.Diagnostics = append(result.Diagnostics, WorkflowDiagnostic{Code: "workflow_node_incomplete", Message: "workflow node is not succeeded", NodeID: node.ID, Field: "state", Expected: WorkflowNodeSucceeded, Actual: node.State, Path: path.Relative})
		}
		for _, output := range node.RequiredOutputs {
			relative, ok := workflowExistingRegularFile(root, output)
			if !ok {
				result.Diagnostics = append(result.Diagnostics, WorkflowDiagnostic{Code: "workflow_required_output_missing", Message: "workflow required output is missing", NodeID: node.ID, Field: "required_outputs", Expected: "repo-confined existing output file", Actual: output, Path: relative})
			}
		}
		for _, evidence := range node.Evidence {
			relative, ok := workflowExistingRegularFile(root, evidence)
			if !ok {
				result.Diagnostics = append(result.Diagnostics, WorkflowDiagnostic{Code: "workflow_evidence_missing", Message: "workflow completion evidence is missing", NodeID: node.ID, Field: "evidence", Expected: "repo-confined existing evidence file", Actual: evidence, Path: relative})
			}
		}
	}
	if len(result.Diagnostics) == 0 {
		result.Status = WorkflowCatalogStatusPass
		result.OK = true
		result.Reason = "workflow_instance_complete"
		result.ReasonCodes = []string{"workflow_instance_complete"}
		return result, nil
	}
	result.Status = WorkflowCatalogStatusFail
	result.OK = false
	result.ReasonCodes = uniqueWorkflowInstanceCompletenessCodes(result.Diagnostics)
	result.Reason = result.ReasonCodes[0]
	return result, nil
}

func workflowExistingRegularFile(root Root, relative string) (string, bool) {
	path, err := ResolveRelativePath(root, relative)
	if err != nil {
		return relative, false
	}
	info, err := os.Lstat(path.Absolute)
	if err != nil || !info.Mode().IsRegular() {
		return path.Relative, false
	}
	return path.Relative, true
}

func uniqueWorkflowInstanceCompletenessCodes(diagnostics []WorkflowDiagnostic) []string {
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
		return workflowInstanceCompletenessRank(codes[i]) < workflowInstanceCompletenessRank(codes[j])
	})
	return codes
}

func workflowInstanceCompletenessRank(code string) int {
	ranks := map[string]int{
		"workflow_instance_invalid":        10,
		"workflow_node_incomplete":         20,
		"workflow_required_output_missing": 30,
		"workflow_evidence_missing":        40,
	}
	if rank, ok := ranks[code]; ok {
		return rank
	}
	return 1000
}

func workflowCatalogPathForDiagnostics(root Root) string {
	path := filepath.ToSlash(WorkflowCatalogDefaultPath)
	if safe, err := ResolveRelativePath(root, path); err == nil {
		return safe.Relative
	}
	return path
}
