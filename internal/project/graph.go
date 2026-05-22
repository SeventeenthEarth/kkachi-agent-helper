package project

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	WorkflowGraphSchemaVersion = "workflow-graph/v1"
	WorkflowGraphDefaultPath   = ".kkachi-workflow.yaml"

	GraphStatusPass = "pass"
	GraphStatusFail = "fail"

	graphEffectiveSourceProject = "project_file"
	graphNextActionValid        = "Graph is valid; KHS may use this read-only evidence."
	graphNextActionRepair       = "Repair .kkachi-workflow.yaml, then rerun graph validate."
)

type GraphOptions struct {
	File string
}

type GraphIssue struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Message  string `json:"message"`
	Hint     string `json:"hint,omitempty"`
	Field    string `json:"field,omitempty"`
	Expected string `json:"expected,omitempty"`
	Actual   string `json:"actual,omitempty"`
	Line     int    `json:"line,omitempty"`
}

type GraphValidationResult struct {
	SchemaVersion   string       `json:"schema_version"`
	Status          string       `json:"status"`
	File            string       `json:"file"`
	Checksum        string       `json:"checksum"`
	EffectiveSource string       `json:"effective_source"`
	Errors          []GraphIssue `json:"errors"`
	Warnings        []GraphIssue `json:"warnings"`
	Conflicts       []GraphIssue `json:"conflicts"`
	NextAction      string       `json:"next_action"`
}

type GraphExplanationResult struct {
	SchemaVersion        string                  `json:"schema_version"`
	Status               string                  `json:"status"`
	GraphVersion         string                  `json:"graph_version"`
	EffectiveSource      string                  `json:"effective_source"`
	Phases               []WorkflowGraphPhase    `json:"phases"`
	Edges                []WorkflowGraphEdge     `json:"edges"`
	Gates                []WorkflowGraphGate     `json:"gates"`
	ApprovalRequirements []WorkflowGraphApproval `json:"approval_requirements"`
	PendingProposals     []string                `json:"pending_proposals"`
	ValidationSummary    GraphValidationResult   `json:"validation_summary"`
	NextAction           string                  `json:"next_action"`
}

type WorkflowGraph struct {
	Version   string
	GraphID   string
	Metadata  WorkflowGraphMetadata
	Phases    []WorkflowGraphPhase
	Edges     []WorkflowGraphEdge
	Gates     []WorkflowGraphGate
	Approvals []WorkflowGraphApproval
	Proposals WorkflowGraphProposals
}

type WorkflowGraphMetadata struct {
	Project            string `json:"project"`
	CreatedBy          string `json:"created_by"`
	ManagedBy          string `json:"managed_by"`
	SourceTemplate     string `json:"source_template,omitempty"`
	LastAppliedEventID string `json:"last_applied_event_id,omitempty"`
}

type WorkflowGraphPhase struct {
	ID         string   `json:"id"`
	Title      string   `json:"title,omitempty"`
	OwnerLayer string   `json:"owner_layer,omitempty"`
	Required   bool     `json:"required"`
	Evidence   []string `json:"evidence,omitempty"`

	requiredSet bool
	seenFields  map[string]bool
}

type WorkflowGraphEdge struct {
	From string `json:"from"`
	To   string `json:"to"`

	seenFields map[string]bool
}

type WorkflowGraphGate struct {
	ID       string   `json:"id"`
	Requires []string `json:"requires"`

	seenFields map[string]bool
}

type WorkflowGraphApproval struct {
	Scope        string `json:"scope"`
	RequiredRole string `json:"required_role"`

	seenFields map[string]bool
}

type WorkflowGraphProposals struct {
	Policy string `json:"policy,omitempty"`
}

type graphDocument struct {
	graph          WorkflowGraph
	errors         []GraphIssue
	metadataFields map[string]bool
	proposalFields map[string]bool
}

type loadedWorkflowGraph struct {
	graph      WorkflowGraph
	validation GraphValidationResult
}

func ValidateWorkflowGraph(root Root, options GraphOptions) GraphValidationResult {
	return loadWorkflowGraph(root, options).validation
}

func ExplainWorkflowGraph(root Root, options GraphOptions) GraphExplanationResult {
	loaded := loadWorkflowGraph(root, options)
	result := GraphExplanationResult{
		SchemaVersion:        WorkflowGraphSchemaVersion,
		Status:               loaded.validation.Status,
		EffectiveSource:      loaded.validation.EffectiveSource,
		Phases:               []WorkflowGraphPhase{},
		Edges:                []WorkflowGraphEdge{},
		Gates:                []WorkflowGraphGate{},
		ApprovalRequirements: []WorkflowGraphApproval{},
		PendingProposals:     []string{},
		ValidationSummary:    loaded.validation,
		NextAction:           loaded.validation.NextAction,
	}
	if loaded.validation.Status != GraphStatusPass {
		return result
	}
	result.GraphVersion = loaded.graph.Version
	result.Phases = append([]WorkflowGraphPhase{}, loaded.graph.Phases...)
	result.Edges = append([]WorkflowGraphEdge{}, loaded.graph.Edges...)
	result.Gates = normalizeWorkflowGraphGates(loaded.graph.Gates)
	result.ApprovalRequirements = append([]WorkflowGraphApproval{}, loaded.graph.Approvals...)
	return result
}

func normalizeWorkflowGraphGates(gates []WorkflowGraphGate) []WorkflowGraphGate {
	result := append([]WorkflowGraphGate{}, gates...)
	for i := range result {
		if result[i].Requires == nil {
			result[i].Requires = []string{}
		}
	}
	return result
}

func loadWorkflowGraph(root Root, options GraphOptions) loadedWorkflowGraph {
	file := graphFileOption(options.File)
	path, issue := resolveGraphSource(root, file)
	if issue != nil {
		return loadedWorkflowGraph{validation: failedGraphValidation(file, []GraphIssue{*issue})}
	}
	data, err := os.ReadFile(path.Absolute)
	if os.IsNotExist(err) {
		return loadedWorkflowGraph{validation: failedGraphValidation(path.Relative, []GraphIssue{{
			Name:     "graph_file",
			Path:     path.Relative,
			Message:  "workflow graph file is missing",
			Hint:     "Create .kkachi-workflow.yaml through an approved graph init/apply flow before relying on graph support.",
			Field:    "file",
			Expected: "existing workflow graph file",
			Actual:   "missing",
		}})}
	}
	if err != nil {
		return loadedWorkflowGraph{validation: failedGraphValidation(path.Relative, []GraphIssue{{
			Name:     "graph_file",
			Path:     path.Relative,
			Message:  "cannot read workflow graph file",
			Hint:     "Check file permissions before validating the workflow graph.",
			Field:    "file",
			Expected: "readable workflow graph file",
			Actual:   err.Error(),
		}})}
	}
	doc := parseWorkflowGraph(data, path.Relative)
	checks := validateWorkflowGraph(doc.graph, path.Relative)
	errors := append([]GraphIssue{}, doc.errors...)
	errors = append(errors, checks...)
	sum := sha256.Sum256(data)
	status := GraphStatusPass
	nextAction := graphNextActionValid
	if len(errors) > 0 {
		status = GraphStatusFail
		nextAction = graphNextActionRepair
	}
	return loadedWorkflowGraph{
		graph: doc.graph,
		validation: GraphValidationResult{
			SchemaVersion:   WorkflowGraphSchemaVersion,
			Status:          status,
			File:            path.Relative,
			Checksum:        hex.EncodeToString(sum[:]),
			EffectiveSource: graphEffectiveSourceProject,
			Errors:          errors,
			Warnings:        []GraphIssue{},
			Conflicts:       []GraphIssue{},
			NextAction:      nextAction,
		},
	}
}

func graphFileOption(file string) string {
	if strings.TrimSpace(file) == "" {
		return WorkflowGraphDefaultPath
	}
	return strings.TrimSpace(file)
}

func resolveGraphSource(root Root, file string) (SafePath, *GraphIssue) {
	path, err := ResolveRelativePath(root, file)
	if err != nil {
		return SafePath{}, issueFromProblem("graph_source", file, err)
	}
	rel := filepath.ToSlash(path.Relative)
	switch {
	case rel == ".kkachi/config.yaml":
		return SafePath{}, forbiddenGraphSourceIssue(rel, "helper config is never workflow graph source of truth")
	case strings.HasPrefix(rel, ".kkachi/config/workflows/"):
		return SafePath{}, forbiddenGraphSourceIssue(rel, "Kkachi v2 workflow runtime config is outside KAH/KHS graph authority")
	case isGeneratedDiagramPath(rel):
		return SafePath{}, forbiddenGraphSourceIssue(rel, "generated diagrams are non-authoritative visualization artifacts")
	}
	path.Relative = rel
	return path, nil
}

func issueFromProblem(name string, path string, err error) *GraphIssue {
	var problemErr *Problem
	if errors.As(err, &problemErr) {
		return &GraphIssue{Name: name, Path: path, Message: problemErr.Message, Hint: problemErr.Hint, Field: problemErr.Field, Expected: problemErr.Expected, Actual: problemErr.Actual}
	}
	return &GraphIssue{Name: name, Path: path, Message: "workflow graph source is invalid", Hint: "Use a repository-relative workflow graph path.", Field: "file", Expected: "repository-confined graph source", Actual: err.Error()}
}

func forbiddenGraphSourceIssue(path string, reason string) *GraphIssue {
	return &GraphIssue{
		Name:     "graph_source",
		Path:     path,
		Message:  "workflow graph source is forbidden",
		Hint:     "Use .kkachi-workflow.yaml or an explicit repository-relative graph candidate file; do not use fallback authority.",
		Field:    "file",
		Expected: ".kkachi-workflow.yaml or explicit graph candidate",
		Actual:   reason,
	}
}

func isGeneratedDiagramPath(path string) bool {
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".mmd") || strings.HasSuffix(lower, ".mermaid") || strings.HasSuffix(lower, ".puml") || strings.HasSuffix(lower, ".plantuml")
}

func failedGraphValidation(file string, errors []GraphIssue) GraphValidationResult {
	return GraphValidationResult{
		SchemaVersion:   WorkflowGraphSchemaVersion,
		Status:          GraphStatusFail,
		File:            file,
		Checksum:        "",
		EffectiveSource: "",
		Errors:          errors,
		Warnings:        []GraphIssue{},
		Conflicts:       []GraphIssue{},
		NextAction:      graphNextActionRepair,
	}
}

func parseWorkflowGraph(data []byte, path string) graphDocument {
	doc := graphDocument{metadataFields: map[string]bool{}, proposalFields: map[string]bool{}}
	lines := strings.Split(string(data), "\n")
	section := ""
	seenSections := map[string]bool{}
	topLevelFields := map[string]bool{}
	var phase *WorkflowGraphPhase
	var edge *WorkflowGraphEdge
	var gate *WorkflowGraphGate
	var approval *WorkflowGraphApproval
	flush := func() {
		if phase != nil {
			item := *phase
			item.seenFields = nil
			doc.graph.Phases = append(doc.graph.Phases, item)
			phase = nil
		}
		if edge != nil {
			item := *edge
			item.seenFields = nil
			doc.graph.Edges = append(doc.graph.Edges, item)
			edge = nil
		}
		if gate != nil {
			item := *gate
			item.seenFields = nil
			doc.graph.Gates = append(doc.graph.Gates, item)
			gate = nil
		}
		if approval != nil {
			item := *approval
			item.seenFields = nil
			doc.graph.Approvals = append(doc.graph.Approvals, item)
			approval = nil
		}
	}
	addParseError := func(line int, field string, message string, expected string, actual string) {
		doc.errors = append(doc.errors, GraphIssue{Name: "graph_yaml", Path: path, Message: message, Hint: "Use the constrained .kkachi-workflow.yaml format documented by graph validate.", Field: field, Expected: expected, Actual: actual, Line: line})
	}
	for lineNumber, raw := range lines {
		lineNo := lineNumber + 1
		if strings.TrimSpace(raw) == "" || strings.HasPrefix(strings.TrimSpace(raw), "#") {
			continue
		}
		indent := leadingSpaces(raw)
		line := strings.TrimSpace(raw)
		if indent == 0 {
			flush()
			if strings.HasSuffix(line, ":") {
				section = strings.TrimSuffix(line, ":")
				if !knownGraphSection(section) {
					addParseError(lineNo, section, "workflow graph contains an unsupported section", "supported graph section", section)
					continue
				}
				if seenSections[section] {
					addParseError(lineNo, section, "workflow graph section is duplicated", "section appears once", section)
					continue
				}
				seenSections[section] = true
				continue
			}
			key, value, ok := strings.Cut(line, ":")
			if !ok {
				addParseError(lineNo, "yaml", "workflow graph contains an unsupported YAML line", "key: value line", line)
				continue
			}
			key = strings.TrimSpace(key)
			value = strings.TrimSpace(value)
			parsed, err := parseWorkflowGraphScalar(value)
			if err != nil {
				addParseError(lineNo, key, "workflow graph scalar is invalid", "string scalar", err.Error())
				continue
			}
			switch key {
			case "version":
				if !markGraphField(&doc, topLevelFields, lineNo, key) {
					continue
				}
				doc.graph.Version = parsed
			case "graph_id":
				if !markGraphField(&doc, topLevelFields, lineNo, key) {
					continue
				}
				doc.graph.GraphID = parsed
			default:
				addParseError(lineNo, key, "workflow graph contains an unsupported top-level field", "version, graph_id, metadata, phases, edges, gates, approvals, or proposals", key)
			}
			continue
		}
		if section == "" {
			addParseError(lineNo, "yaml", "workflow graph field appears before a section", "top-level section", line)
			continue
		}
		if strings.HasPrefix(line, "- ") {
			flush()
			item := strings.TrimSpace(strings.TrimPrefix(line, "- "))
			switch section {
			case "phases":
				phase = &WorkflowGraphPhase{seenFields: map[string]bool{}}
			case "edges":
				edge = &WorkflowGraphEdge{seenFields: map[string]bool{}}
			case "gates":
				gate = &WorkflowGraphGate{seenFields: map[string]bool{}}
			case "approvals":
				approval = &WorkflowGraphApproval{seenFields: map[string]bool{}}
			default:
				addParseError(lineNo, section, "workflow graph section does not accept list items", "phases, edges, gates, or approvals list item", section)
				continue
			}
			setGraphListItemField(&doc, section, lineNo, item, phase, edge, gate, approval)
			continue
		}
		if section == "metadata" || section == "proposals" {
			setGraphMappingField(&doc, section, lineNo, line)
			continue
		}
		setGraphListItemField(&doc, section, lineNo, line, phase, edge, gate, approval)
	}
	flush()
	for i := range doc.errors {
		if doc.errors[i].Path == "" {
			doc.errors[i].Path = path
		}
	}
	return doc
}

func knownGraphSection(section string) bool {
	switch section {
	case "metadata", "phases", "edges", "gates", "approvals", "proposals":
		return true
	default:
		return false
	}
}

func leadingSpaces(value string) int {
	count := 0
	for _, r := range value {
		if r != ' ' {
			break
		}
		count++
	}
	return count
}

func markGraphField(doc *graphDocument, seen map[string]bool, lineNo int, field string) bool {
	if seen[field] {
		doc.errors = append(doc.errors, GraphIssue{Name: "graph_yaml", Message: "workflow graph field is duplicated", Field: field, Expected: "field appears once", Actual: field, Line: lineNo})
		return false
	}
	seen[field] = true
	return true
}

func setGraphMappingField(doc *graphDocument, section string, lineNo int, line string) {
	key, value, ok := strings.Cut(line, ":")
	if !ok {
		doc.errors = append(doc.errors, GraphIssue{Name: "graph_yaml", Message: "workflow graph mapping line is invalid", Field: section, Expected: "key: value line", Actual: line, Line: lineNo})
		return
	}
	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)
	parsed, err := parseWorkflowGraphScalar(value)
	if err != nil {
		doc.errors = append(doc.errors, GraphIssue{Name: "graph_yaml", Message: "workflow graph scalar is invalid", Field: key, Expected: "string scalar", Actual: err.Error(), Line: lineNo})
		return
	}
	switch section {
	case "metadata":
		switch key {
		case "project":
			if !markGraphField(doc, doc.metadataFields, lineNo, key) {
				return
			}
			doc.graph.Metadata.Project = parsed
		case "created_by":
			if !markGraphField(doc, doc.metadataFields, lineNo, key) {
				return
			}
			doc.graph.Metadata.CreatedBy = parsed
		case "managed_by":
			if !markGraphField(doc, doc.metadataFields, lineNo, key) {
				return
			}
			doc.graph.Metadata.ManagedBy = parsed
		case "source_template":
			if !markGraphField(doc, doc.metadataFields, lineNo, key) {
				return
			}
			doc.graph.Metadata.SourceTemplate = parsed
		case "last_applied_event_id":
			if !markGraphField(doc, doc.metadataFields, lineNo, key) {
				return
			}
			doc.graph.Metadata.LastAppliedEventID = parsed
		default:
			doc.errors = append(doc.errors, GraphIssue{Name: "graph_yaml", Message: "workflow graph metadata field is unsupported", Field: key, Expected: "project, created_by, managed_by, source_template, or last_applied_event_id", Actual: key, Line: lineNo})
		}
	case "proposals":
		if key == "policy" {
			if !markGraphField(doc, doc.proposalFields, lineNo, key) {
				return
			}
			doc.graph.Proposals.Policy = parsed
			return
		}
		doc.errors = append(doc.errors, GraphIssue{Name: "graph_yaml", Message: "workflow graph proposals field is unsupported", Field: key, Expected: "policy", Actual: key, Line: lineNo})
	}
}

func setGraphListItemField(doc *graphDocument, section string, lineNo int, line string, phase *WorkflowGraphPhase, edge *WorkflowGraphEdge, gate *WorkflowGraphGate, approval *WorkflowGraphApproval) {
	key, value, ok := strings.Cut(line, ":")
	if !ok {
		doc.errors = append(doc.errors, GraphIssue{Name: "graph_yaml", Message: "workflow graph list item line is invalid", Field: section, Expected: "key: value line", Actual: line, Line: lineNo})
		return
	}
	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)
	parsed, err := parseWorkflowGraphScalar(value)
	if err != nil && !strings.HasPrefix(value, "[") {
		doc.errors = append(doc.errors, GraphIssue{Name: "graph_yaml", Message: "workflow graph scalar is invalid", Field: key, Expected: "string scalar", Actual: err.Error(), Line: lineNo})
		return
	}
	switch section {
	case "phases":
		if phase == nil {
			doc.errors = append(doc.errors, GraphIssue{Name: "graph_yaml", Message: "phase field appears outside a phase row", Field: key, Expected: "field below phases list item", Actual: fmt.Sprintf("line %d", lineNo), Line: lineNo})
			return
		}
		if !markGraphField(doc, phase.seenFields, lineNo, key) {
			return
		}
		switch key {
		case "id":
			phase.ID = parsed
		case "title":
			phase.Title = parsed
		case "owner_layer":
			phase.OwnerLayer = parsed
		case "required":
			phase.requiredSet = true
			value, ok := parseYAMLBool(parsed)
			if !ok {
				doc.errors = append(doc.errors, GraphIssue{Name: "graph_yaml", Message: "phase required field is invalid", Field: key, Expected: "true or false", Actual: parsed, Line: lineNo})
				return
			}
			phase.Required = value
		case "evidence":
			items, err := parseYAMLStringList(value)
			if err != nil {
				doc.errors = append(doc.errors, GraphIssue{Name: "graph_yaml", Message: "phase evidence list is invalid", Field: key, Expected: "inline string list", Actual: err.Error(), Line: lineNo})
				return
			}
			phase.Evidence = items
		default:
			doc.errors = append(doc.errors, GraphIssue{Name: "graph_yaml", Message: "phase field is unsupported", Field: key, Expected: "id, title, owner_layer, required, or evidence", Actual: key, Line: lineNo})
		}
	case "edges":
		if edge == nil {
			doc.errors = append(doc.errors, GraphIssue{Name: "graph_yaml", Message: "edge field appears outside an edge row", Field: key, Expected: "field below edges list item", Actual: fmt.Sprintf("line %d", lineNo), Line: lineNo})
			return
		}
		if !markGraphField(doc, edge.seenFields, lineNo, key) {
			return
		}
		switch key {
		case "from":
			edge.From = parsed
		case "to":
			edge.To = parsed
		default:
			doc.errors = append(doc.errors, GraphIssue{Name: "graph_yaml", Message: "edge field is unsupported", Field: key, Expected: "from or to", Actual: key, Line: lineNo})
		}
	case "gates":
		if gate == nil {
			doc.errors = append(doc.errors, GraphIssue{Name: "graph_yaml", Message: "gate field appears outside a gate row", Field: key, Expected: "field below gates list item", Actual: fmt.Sprintf("line %d", lineNo), Line: lineNo})
			return
		}
		if !markGraphField(doc, gate.seenFields, lineNo, key) {
			return
		}
		switch key {
		case "id":
			gate.ID = parsed
		case "requires":
			items, err := parseYAMLStringList(value)
			if err != nil {
				doc.errors = append(doc.errors, GraphIssue{Name: "graph_yaml", Message: "gate requires list is invalid", Field: key, Expected: "inline string list", Actual: err.Error(), Line: lineNo})
				return
			}
			gate.Requires = items
		default:
			doc.errors = append(doc.errors, GraphIssue{Name: "graph_yaml", Message: "gate field is unsupported", Field: key, Expected: "id or requires", Actual: key, Line: lineNo})
		}
	case "approvals":
		if approval == nil {
			doc.errors = append(doc.errors, GraphIssue{Name: "graph_yaml", Message: "approval field appears outside an approval row", Field: key, Expected: "field below approvals list item", Actual: fmt.Sprintf("line %d", lineNo), Line: lineNo})
			return
		}
		if !markGraphField(doc, approval.seenFields, lineNo, key) {
			return
		}
		switch key {
		case "scope":
			approval.Scope = parsed
		case "required_role":
			approval.RequiredRole = parsed
		default:
			doc.errors = append(doc.errors, GraphIssue{Name: "graph_yaml", Message: "approval field is unsupported", Field: key, Expected: "scope or required_role", Actual: key, Line: lineNo})
		}
	default:
		doc.errors = append(doc.errors, GraphIssue{Name: "graph_yaml", Message: "workflow graph section does not accept item fields", Field: section, Expected: "phases, edges, gates, or approvals", Actual: section, Line: lineNo})
	}
}

func parseYAMLBool(value string) (bool, bool) {
	switch strings.TrimSpace(value) {
	case "true":
		return true, true
	case "false":
		return false, true
	default:
		return false, false
	}
}

func parseYAMLStringList(value string) ([]string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	if !strings.HasPrefix(value, "[") || !strings.HasSuffix(value, "]") {
		return nil, fmt.Errorf("not an inline list")
	}
	inner := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(value, "["), "]"))
	if inner == "" {
		return []string{}, nil
	}
	parts := splitInlineList(inner)
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		item, err := parseWorkflowGraphScalar(strings.TrimSpace(part))
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(item) == "" {
			return nil, fmt.Errorf("empty list item")
		}
		result = append(result, item)
	}
	return result, nil
}

func parseWorkflowGraphScalar(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if !strings.HasPrefix(trimmed, `"`) && strings.Contains(trimmed, " #") {
		return "", fmt.Errorf("inline comments require quoted scalars")
	}
	return parseYAMLScalar(value)
}

func splitInlineList(inner string) []string {
	parts := []string{}
	var current strings.Builder
	inQuote := false
	escaped := false
	for _, r := range inner {
		if escaped {
			current.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' && inQuote {
			current.WriteRune(r)
			escaped = true
			continue
		}
		if r == '"' {
			inQuote = !inQuote
			current.WriteRune(r)
			continue
		}
		if r == ',' && !inQuote {
			parts = append(parts, current.String())
			current.Reset()
			continue
		}
		current.WriteRune(r)
	}
	parts = append(parts, current.String())
	return parts
}

func validateWorkflowGraph(graph WorkflowGraph, path string) []GraphIssue {
	errors := []GraphIssue{}
	add := func(name string, field string, message string, expected string, actual string) {
		errors = append(errors, GraphIssue{Name: name, Path: path, Message: message, Hint: "Repair .kkachi-workflow.yaml so KAH can validate it deterministically.", Field: field, Expected: expected, Actual: actual})
	}
	if graph.Version != WorkflowGraphSchemaVersion {
		actual := graph.Version
		if actual == "" {
			actual = "missing"
		}
		add("version", "version", "workflow graph version is unsupported", WorkflowGraphSchemaVersion, actual)
	}
	if strings.TrimSpace(graph.GraphID) == "" {
		add("graph_id", "graph_id", "workflow graph id is required", "non-empty graph_id", "missing")
	}
	if strings.TrimSpace(graph.Metadata.Project) == "" {
		add("metadata_project", "metadata.project", "workflow graph metadata project is required", "non-empty project", "missing")
	}
	if strings.TrimSpace(graph.Metadata.CreatedBy) == "" {
		add("metadata_created_by", "metadata.created_by", "workflow graph metadata created_by is required", "non-empty created_by", "missing")
	}
	if graph.Metadata.ManagedBy != "kah" {
		actual := graph.Metadata.ManagedBy
		if actual == "" {
			actual = "missing"
		}
		add("metadata_managed_by", "metadata.managed_by", "workflow graph must be managed by kah", "kah", actual)
	}
	phaseIDs := map[string]bool{}
	duplicates := []string{}
	for _, phase := range graph.Phases {
		id := strings.TrimSpace(phase.ID)
		if id == "" {
			add("phase_id", "phases[].id", "phase id is required", "non-empty phase id", "missing")
		} else {
			if phaseIDs[id] {
				duplicates = append(duplicates, id)
			}
			phaseIDs[id] = true
		}
		if !phase.requiredSet {
			actual := phase.ID
			if actual == "" {
				actual = "missing"
			}
			add("phase_required", "phases[].required", "phase required field is required", "explicit true or false", actual)
		}
	}
	if len(graph.Phases) == 0 {
		add("phases", "phases", "workflow graph requires at least one phase", "one or more phases", "missing")
	}
	if len(duplicates) > 0 {
		sort.Strings(duplicates)
		add("duplicate_phase", "phases[].id", "phase ids must be unique", "unique phase ids", strings.Join(duplicates, ","))
	}
	gateIDs := map[string]bool{}
	duplicateGates := []string{}
	for _, edge := range graph.Edges {
		if strings.TrimSpace(edge.From) == "" || strings.TrimSpace(edge.To) == "" {
			add("edge_shape", "edges[]", "edge from and to are required", "non-empty from and to", fmt.Sprintf("%s->%s", edge.From, edge.To))
			continue
		}
		if edge.From == edge.To {
			add("self_edge", "edges[].to", "edge must not point to itself", "different from and to", edge.From)
		}
		if !phaseIDs[edge.From] {
			add("edge_from", "edges[].from", "edge source phase is not declared", "declared phase id", edge.From)
		}
		if !phaseIDs[edge.To] {
			add("edge_to", "edges[].to", "edge target phase is not declared", "declared phase id", edge.To)
		}
	}
	if cycle := firstGraphCycle(graph.Edges, phaseIDs); len(cycle) > 0 {
		add("cycle", "edges", "workflow graph edges must be acyclic", "acyclic phase dependencies", strings.Join(cycle, " -> "))
	}
	for _, gate := range graph.Gates {
		id := strings.TrimSpace(gate.ID)
		if id == "" {
			add("gate_id", "gates[].id", "gate id is required", "non-empty gate id", "missing")
		} else {
			if gateIDs[id] {
				duplicateGates = append(duplicateGates, id)
			}
			gateIDs[id] = true
		}
		for _, required := range gate.Requires {
			if !phaseIDs[required] {
				add("gate_requires", "gates[].requires", "gate requirement phase is not declared", "declared phase id", required)
			}
		}
	}
	if len(duplicateGates) > 0 {
		sort.Strings(duplicateGates)
		add("duplicate_gate", "gates[].id", "gate ids must be unique", "unique gate ids", strings.Join(duplicateGates, ","))
	}
	approvalScopes := map[string]bool{}
	duplicateApprovals := []string{}
	for _, approval := range graph.Approvals {
		scope := strings.TrimSpace(approval.Scope)
		if scope == "" {
			add("approval_scope", "approvals[].scope", "approval scope is required", "non-empty scope", "missing")
		} else {
			if approvalScopes[scope] {
				duplicateApprovals = append(duplicateApprovals, scope)
			}
			approvalScopes[scope] = true
		}
		if strings.TrimSpace(approval.RequiredRole) == "" {
			add("approval_required_role", "approvals[].required_role", "approval required_role is required", "non-empty required_role", "missing")
		}
	}
	if len(duplicateApprovals) > 0 {
		sort.Strings(duplicateApprovals)
		add("duplicate_approval", "approvals[].scope", "approval scopes must be unique", "unique approval scopes", strings.Join(duplicateApprovals, ","))
	}
	if graph.Proposals.Policy != "" && graph.Proposals.Policy != "proposal-first" {
		add("proposals_policy", "proposals.policy", "workflow graph proposals policy is unsupported", "proposal-first", graph.Proposals.Policy)
	}
	return errors
}

func firstGraphCycle(edges []WorkflowGraphEdge, phaseIDs map[string]bool) []string {
	adjacent := map[string][]string{}
	for _, edge := range edges {
		if phaseIDs[edge.From] && phaseIDs[edge.To] {
			adjacent[edge.From] = append(adjacent[edge.From], edge.To)
		}
	}
	visiting := map[string]bool{}
	visited := map[string]bool{}
	stack := []string{}
	var cycle []string
	var visit func(string) bool
	visit = func(node string) bool {
		if visiting[node] {
			for i, item := range stack {
				if item == node {
					cycle = append(append([]string{}, stack[i:]...), node)
					return true
				}
			}
			cycle = []string{node, node}
			return true
		}
		if visited[node] {
			return false
		}
		visiting[node] = true
		stack = append(stack, node)
		for _, next := range adjacent[node] {
			if visit(next) {
				return true
			}
		}
		stack = stack[:len(stack)-1]
		visiting[node] = false
		visited[node] = true
		return false
	}
	nodes := make([]string, 0, len(phaseIDs))
	for node := range phaseIDs {
		nodes = append(nodes, node)
	}
	sort.Strings(nodes)
	for _, node := range nodes {
		if visit(node) {
			return cycle
		}
	}
	return nil
}
