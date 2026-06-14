package project

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
)

const (
	TaskDAGStatusValid   = "valid"
	TaskDAGStatusInvalid = "invalid"
	TaskDAGStatusError   = "error"
)

type TaskDAGResult struct {
	Status        string               `json:"status"`
	OK            bool                 `json:"ok"`
	Reason        string               `json:"reason"`
	ReasonCodes   []string             `json:"reason_codes"`
	WorkflowID    string               `json:"workflow_id"`
	Path          string               `json:"path"`
	SchemaVersion string               `json:"schema_version"`
	Diagnostics   []TaskDAGDiagnostic  `json:"diagnostics"`
	Nodes         []TaskDAGNodeSummary `json:"nodes,omitempty"`
	Edges         []TaskDAGEdge        `json:"edges,omitempty"`
}

type TaskDAGDiagnostic struct {
	Code          string `json:"code"`
	Message       string `json:"message"`
	WorkflowID    string `json:"workflow_id,omitempty"`
	Path          string `json:"path,omitempty"`
	SchemaVersion string `json:"schema_version,omitempty"`
	NodeID        string `json:"node_id,omitempty"`
	Field         string `json:"field,omitempty"`
	Value         string `json:"value,omitempty"`
}

type TaskDAGNodeSummary struct {
	ID              string   `json:"id"`
	DependsOn       []string `json:"depends_on"`
	Join            string   `json:"join"`
	RequiredOutputs []string `json:"required_outputs"`
}

type TaskDAGEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type parsedTaskDAG struct {
	WorkflowID    string
	SchemaVersion string
	Nodes         []TaskDAGNodeSummary
}

// ValidateTaskDAG validates the DAGSM-001 task-DAG YAML subset without executing
// or mutating workflow state.
func ValidateTaskDAG(root Root, file string) (TaskDAGResult, error) {
	path, err := ResolveRelativePath(root, file)
	if err != nil {
		return TaskDAGResult{}, err
	}
	result := TaskDAGResult{Status: TaskDAGStatusInvalid, OK: false, Path: path.Relative, Diagnostics: []TaskDAGDiagnostic{}}
	data, err := os.ReadFile(path.Absolute)
	if errors.Is(err, os.ErrNotExist) {
		result.addDiagnostic("task_dag_missing", "workflow file is missing", "file", path.Relative, "")
		result.finalize()
		return result, nil
	}
	if err != nil {
		return TaskDAGResult{}, &Problem{Code: "task_dag_read_failed", Message: "cannot read workflow file", Hint: "Check workflow file permissions before retrying.", Path: path.Relative, Field: "file", Expected: "readable workflow file", Actual: err.Error()}
	}
	parsed, diagnostics := parseTaskDAGYAML(root, path.Relative, string(data))
	result.WorkflowID = parsed.WorkflowID
	result.SchemaVersion = parsed.SchemaVersion
	result.Nodes = append([]TaskDAGNodeSummary{}, parsed.Nodes...)
	result.Diagnostics = append(result.Diagnostics, diagnostics...)
	validateParsedTaskDAG(root, &result)
	result.finalize()
	return result, nil
}

func parseTaskDAGYAML(root Root, path string, content string) (parsedTaskDAG, []TaskDAGDiagnostic) {
	parsed := parsedTaskDAG{}
	diagnostics := []TaskDAGDiagnostic{}
	topSeen := map[string]bool{}
	inNodes := false
	var current *TaskDAGNodeSummary
	var currentSeen map[string]bool
	listField := ""
	listIndent := 0
	appendDiagnostic := func(code, message, field, value, nodeID string) {
		diagnostics = append(diagnostics, TaskDAGDiagnostic{Code: code, Message: message, Path: path, WorkflowID: parsed.WorkflowID, SchemaVersion: parsed.SchemaVersion, Field: field, Value: value, NodeID: nodeID})
	}
	finishNode := func() {
		if current == nil {
			return
		}
		parsed.Nodes = append(parsed.Nodes, *current)
		current = nil
		currentSeen = nil
		listField = ""
	}

	lines := strings.Split(content, "\n")
	for _, raw := range lines {
		line := stripTaskDAGComment(raw)
		if strings.TrimSpace(line) == "" {
			continue
		}
		indent := taskDAGLeadingSpaces(line)
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, "\t") {
			appendDiagnostic("task_dag_parse_error", "tabs are not supported in task-DAG YAML", "yaml", trimmed, "")
			continue
		}
		if listField == "required_outputs" && indent > listIndent && strings.HasPrefix(trimmed, "- ") {
			value := strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
			if current != nil {
				current.RequiredOutputs = append(current.RequiredOutputs, parseTaskDAGScalar(value))
			}
			continue
		}
		if inNodes && strings.HasPrefix(trimmed, "- ") {
			finishNode()
			current = &TaskDAGNodeSummary{DependsOn: []string{}, RequiredOutputs: []string{}}
			currentSeen = map[string]bool{}
			rest := strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
			if rest != "" {
				key, value, ok := splitTaskDAGKeyValue(rest)
				if !ok {
					appendDiagnostic("task_dag_parse_error", "node list item must be a key/value mapping", "nodes", rest, "")
					continue
				}
				applyTaskDAGNodeField(current, currentSeen, key, value, &diagnostics, path, parsed.WorkflowID, parsed.SchemaVersion)
			}
			listField = ""
			continue
		}
		key, value, ok := splitTaskDAGKeyValue(trimmed)
		if !ok {
			appendDiagnostic("task_dag_parse_error", "line is not a supported key/value mapping", "yaml", trimmed, "")
			continue
		}
		if !inNodes || indent == 0 {
			if inNodes && key != "nodes" && indent == 0 {
				finishNode()
				inNodes = false
			}
			if topSeen[key] {
				appendDiagnostic("task_dag_invalid_schema", "duplicate top-level field", key, value, "")
				continue
			}
			topSeen[key] = true
			switch key {
			case "schema_version":
				parsed.SchemaVersion = parseTaskDAGScalar(value)
			case "workflow_id":
				parsed.WorkflowID = parseTaskDAGScalar(value)
			case "nodes":
				inNodes = true
				if strings.TrimSpace(value) != "" {
					appendDiagnostic("task_dag_invalid_schema", "nodes must be a block list", "nodes", value, "")
				}
			default:
				appendDiagnostic("task_dag_invalid_schema", "unsupported top-level field", key, value, "")
			}
			continue
		}
		if current == nil {
			appendDiagnostic("task_dag_parse_error", "node field appears before a node item", key, value, "")
			continue
		}
		applyTaskDAGNodeField(current, currentSeen, key, value, &diagnostics, path, parsed.WorkflowID, parsed.SchemaVersion)
		if key == "required_outputs" && strings.TrimSpace(value) == "" {
			listField = "required_outputs"
			listIndent = indent
		} else {
			listField = ""
		}
	}
	finishNode()
	return parsed, diagnostics
}

func applyTaskDAGNodeField(node *TaskDAGNodeSummary, seen map[string]bool, key string, value string, diagnostics *[]TaskDAGDiagnostic, path, workflowID, schemaVersion string) {
	if seen[key] {
		*diagnostics = append(*diagnostics, TaskDAGDiagnostic{Code: "task_dag_invalid_schema", Message: "duplicate node field", Path: path, WorkflowID: workflowID, SchemaVersion: schemaVersion, NodeID: node.ID, Field: key, Value: value})
		return
	}
	seen[key] = true
	switch key {
	case "id":
		node.ID = parseTaskDAGScalar(value)
	case "depends_on":
		node.DependsOn = parseTaskDAGInlineList(value)
	case "join":
		node.Join = parseTaskDAGScalar(value)
	case "required_outputs":
		if strings.TrimSpace(value) == "" {
			node.RequiredOutputs = []string{}
		} else {
			node.RequiredOutputs = parseTaskDAGInlineList(value)
		}
	default:
		*diagnostics = append(*diagnostics, TaskDAGDiagnostic{Code: "task_dag_invalid_schema", Message: "unsupported node field", Path: path, WorkflowID: workflowID, SchemaVersion: schemaVersion, NodeID: node.ID, Field: key, Value: value})
	}
}

func validateParsedTaskDAG(root Root, result *TaskDAGResult) {
	if result.SchemaVersion != "task-dag/v1" {
		result.addDiagnostic("task_dag_invalid_schema", "unsupported schema version", "schema_version", result.SchemaVersion, "")
	}
	if strings.TrimSpace(result.WorkflowID) == "" {
		result.addDiagnostic("task_dag_invalid_schema", "workflow_id is required", "workflow_id", "", "")
	}
	if len(result.Nodes) == 0 {
		result.addDiagnostic("task_dag_invalid_schema", "nodes must contain at least one node", "nodes", "", "")
	}
	knownIDs := map[string]bool{}
	for _, node := range result.Nodes {
		if strings.TrimSpace(node.ID) != "" {
			knownIDs[node.ID] = true
		}
	}

	seenIDs := map[string]bool{}
	for _, node := range result.Nodes {
		if strings.TrimSpace(node.ID) == "" {
			result.addDiagnostic("task_dag_invalid_schema", "node id is required", "nodes.id", "", "")
			continue
		}
		if seenIDs[node.ID] {
			result.addDiagnostic("task_dag_duplicate_node", "duplicate node id", "nodes.id", node.ID, node.ID)
		}
		seenIDs[node.ID] = true
		if node.Join != "all_of" {
			result.addDiagnostic("task_dag_unsupported_join", "unsupported join declaration", "nodes.join", node.Join, node.ID)
		}
		if len(node.RequiredOutputs) == 0 {
			result.addDiagnostic("node_required_output_missing", "required_outputs declarations must be present on every node", "nodes.required_outputs", "", node.ID)
		}
		for _, output := range node.RequiredOutputs {
			if _, err := ResolveRelativePath(root, output); err != nil {
				result.addDiagnostic("task_dag_invalid_schema", "required output path is not repository-relative safe", "nodes.required_outputs", output, node.ID)
			}
		}
		for _, dep := range node.DependsOn {
			result.Edges = append(result.Edges, TaskDAGEdge{From: dep, To: node.ID})
			if dep == "" || !knownIDs[dep] {
				result.addDiagnostic("task_dag_unknown_dependency", "dependency references an unknown node", "nodes.depends_on", dep, node.ID)
			}
		}
	}
	if hasTaskDAGCycle(result.Nodes) {
		result.addDiagnostic("task_dag_cycle_detected", "dependency graph contains a cycle", "nodes.depends_on", "", "")
	}
	sort.Slice(result.Edges, func(i, j int) bool {
		if result.Edges[i].From == result.Edges[j].From {
			return result.Edges[i].To < result.Edges[j].To
		}
		return result.Edges[i].From < result.Edges[j].From
	})
}

func hasTaskDAGCycle(nodes []TaskDAGNodeSummary) bool {
	graph := map[string][]string{}
	for _, node := range nodes {
		graph[node.ID] = append([]string{}, node.DependsOn...)
	}
	visiting := map[string]bool{}
	visited := map[string]bool{}
	var visit func(string) bool
	visit = func(id string) bool {
		if visiting[id] {
			return true
		}
		if visited[id] {
			return false
		}
		visiting[id] = true
		for _, dep := range graph[id] {
			if _, ok := graph[dep]; !ok {
				continue
			}
			if visit(dep) {
				return true
			}
		}
		visiting[id] = false
		visited[id] = true
		return false
	}
	for _, node := range nodes {
		if visit(node.ID) {
			return true
		}
	}
	return false
}

func (r *TaskDAGResult) addDiagnostic(code, message, field, value, nodeID string) {
	r.Diagnostics = append(r.Diagnostics, TaskDAGDiagnostic{Code: code, Message: message, WorkflowID: r.WorkflowID, Path: r.Path, SchemaVersion: r.SchemaVersion, NodeID: nodeID, Field: field, Value: value})
}

func (r *TaskDAGResult) finalize() {
	if len(r.Diagnostics) == 0 {
		r.Status = TaskDAGStatusValid
		r.OK = true
		r.Reason = "task_dag_valid"
		r.ReasonCodes = []string{"task_dag_valid"}
		return
	}
	r.Status = TaskDAGStatusInvalid
	r.OK = false
	r.ReasonCodes = uniqueTaskDAGCodes(r.Diagnostics)
	r.Reason = primaryTaskDAGReason(r.ReasonCodes)
}

func uniqueTaskDAGCodes(diagnostics []TaskDAGDiagnostic) []string {
	seen := map[string]bool{}
	codes := []string{}
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == "" || seen[diagnostic.Code] {
			continue
		}
		seen[diagnostic.Code] = true
		codes = append(codes, diagnostic.Code)
	}
	sort.SliceStable(codes, func(i, j int) bool { return taskDAGReasonRank(codes[i]) < taskDAGReasonRank(codes[j]) })
	return codes
}

func primaryTaskDAGReason(codes []string) string {
	if len(codes) == 0 {
		return "task_dag_valid"
	}
	return codes[0]
}

func taskDAGReasonRank(code string) int {
	ranks := map[string]int{
		"task_dag_missing":             10,
		"task_dag_parse_error":         20,
		"task_dag_duplicate_node":      30,
		"task_dag_unknown_dependency":  40,
		"task_dag_cycle_detected":      50,
		"task_dag_unsupported_join":    60,
		"node_required_output_missing": 70,
		"task_dag_invalid_schema":      80,
	}
	if rank, ok := ranks[code]; ok {
		return rank
	}
	return 1000
}

func splitTaskDAGKeyValue(value string) (string, string, bool) {
	key, rest, ok := strings.Cut(value, ":")
	if !ok {
		return "", "", false
	}
	key = strings.TrimSpace(key)
	if key == "" || strings.ContainsAny(key, " []{}") {
		return "", "", false
	}
	return key, strings.TrimSpace(rest), true
}

func stripTaskDAGComment(line string) string {
	inSingle := false
	inDouble := false
	for i, r := range line {
		switch r {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case '#':
			if !inSingle && !inDouble {
				return line[:i]
			}
		}
	}
	return line
}

func taskDAGLeadingSpaces(value string) int {
	count := 0
	for _, r := range value {
		if r != ' ' {
			break
		}
		count++
	}
	return count
}

func parseTaskDAGScalar(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 {
		if (strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"")) || (strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'")) {
			return strings.TrimSpace(value[1 : len(value)-1])
		}
	}
	return value
}

func parseTaskDAGInlineList(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return []string{}
	}
	if !strings.HasPrefix(value, "[") || !strings.HasSuffix(value, "]") {
		return []string{parseTaskDAGScalar(value)}
	}
	inner := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(value, "["), "]"))
	if inner == "" {
		return []string{}
	}
	parts := splitTaskDAGInlineItems(inner)
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = parseTaskDAGScalar(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

func splitTaskDAGInlineItems(value string) []string {
	parts := []string{}
	var b strings.Builder
	inSingle := false
	inDouble := false
	for _, r := range value {
		switch r {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case ',':
			if !inSingle && !inDouble {
				parts = append(parts, strings.TrimSpace(b.String()))
				b.Reset()
				continue
			}
		}
		b.WriteRune(r)
	}
	parts = append(parts, strings.TrimSpace(b.String()))
	return parts
}

func TaskDAGHumanSummary(result TaskDAGResult) string {
	return fmt.Sprintf("workflow %s: %s (%s)\n", result.Path, result.Status, result.Reason)
}
