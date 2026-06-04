package project

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	WorkflowGraphSchemaVersion = "workflow-graph/v1"
	WorkflowGraphDefaultPath   = ".kkachi-workflow.yaml"

	GraphStatusPass = "pass"
	GraphStatusFail = "fail"

	graphEffectiveSourceProject = "project_file"
	graphNextActionValid        = "Graph is valid; KHS may use this read-only evidence."
	graphNextActionRepair       = "Repair .kkachi-workflow.yaml, then rerun graph validate."
	graphNextActionDiffReady    = "Review the semantic diff; record a proposal before applying graph changes."
	graphNextActionProposal     = "Record an approval or audit evidence reference, then run graph apply --approval <evidence-ref>; proposal storage does not apply graph changes."
	graphNextActionInitialized  = "Graph is initialized; run graph validate or graph explain for read-only evidence."
	graphNextActionApplied      = "Graph proposal applied; run graph validate or graph explain for updated evidence."
	graphNextActionExported     = "Graph diagram exported as a non-authoritative generated artifact; do not use it as workflow graph source of truth."
	graphInitEventType          = "graph.initialized"
	graphProposalEventType      = "graph.proposal_recorded"
	graphApplyEventType         = "graph.applied"
	graphProposalDir            = ".kkachi/graph/proposals"
	graphTemplateKHSDefault     = "khs-default"
	graphTemplateSourceBuiltin  = "built_in"
	graphTemplateSourcePath     = "path"
	graphExportFormatMermaid    = "mermaid"
	graphExportFormatPlantUML   = "plantuml"
	graphFeedbackIntakePolicy   = "EXTERNAL_FEEDBACK_INTAKE"
	graphFeedbackIntakeSchema   = "external-feedback-intake/v1"
	graphIssueGraphFile         = "graph_file"
	graphIssueActualMissing     = "missing"
)

var graphProposalIDPattern = regexp.MustCompile(`^gprop-(\d{6})\.json$`)

type GraphOptions struct {
	File string
}

type GraphDiffOptions struct {
	From string
	To   string
}

type GraphProposeOptions struct {
	Patch  string
	Reason string
	Now    func() time.Time
}

type GraphApplyOptions struct {
	Proposal string
	Approval string
	Now      func() time.Time
}

type GraphInitOptions struct {
	FromTemplate string
	Output       string
	Now          func() time.Time
}

type GraphExportOptions struct {
	Format string
	Output string
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
	SchemaVersion   string                       `json:"schema_version"`
	Status          string                       `json:"status"`
	File            string                       `json:"file"`
	Checksum        string                       `json:"checksum"`
	EffectiveSource string                       `json:"effective_source"`
	FeedbackIntake  *WorkflowGraphFeedbackIntake `json:"feedback_intake,omitempty"`
	Errors          []GraphIssue                 `json:"errors"`
	Warnings        []GraphIssue                 `json:"warnings"`
	Conflicts       []GraphIssue                 `json:"conflicts"`
	NextAction      string                       `json:"next_action"`
}

type GraphExplanationResult struct {
	SchemaVersion        string                       `json:"schema_version"`
	Status               string                       `json:"status"`
	GraphVersion         string                       `json:"graph_version"`
	EffectiveSource      string                       `json:"effective_source"`
	Phases               []WorkflowGraphPhase         `json:"phases"`
	Edges                []WorkflowGraphEdge          `json:"edges"`
	Gates                []WorkflowGraphGate          `json:"gates"`
	ApprovalRequirements []WorkflowGraphApproval      `json:"approval_requirements"`
	FeedbackIntake       *WorkflowGraphFeedbackIntake `json:"feedback_intake,omitempty"`
	PendingProposals     []string                     `json:"pending_proposals"`
	ValidationSummary    GraphValidationResult        `json:"validation_summary"`
	NextAction           string                       `json:"next_action"`
}

type GraphDiffEndpoint struct {
	File            string `json:"file"`
	Checksum        string `json:"checksum"`
	EffectiveSource string `json:"effective_source"`
}

type GraphDiffValidationSummary struct {
	From GraphValidationResult `json:"from"`
	To   GraphValidationResult `json:"to"`
}

type GraphDiffResult struct {
	SchemaVersion         string                            `json:"schema_version"`
	Status                string                            `json:"status"`
	From                  GraphDiffEndpoint                 `json:"from"`
	To                    GraphDiffEndpoint                 `json:"to"`
	ChangedPhases         WorkflowGraphPhaseChanges         `json:"changed_phases"`
	ChangedEdges          WorkflowGraphEdgeChanges          `json:"changed_edges"`
	ChangedGates          WorkflowGraphGateChanges          `json:"changed_gates"`
	ChangedApprovals      WorkflowGraphApprovalChanges      `json:"changed_approvals"`
	ChangedFeedbackIntake WorkflowGraphFeedbackIntakeChange `json:"changed_feedback_intake"`
	RiskFlags             []string                          `json:"risk_flags"`
	RequiresApproval      bool                              `json:"requires_approval"`
	ValidationSummary     GraphDiffValidationSummary        `json:"validation_summary"`
	NextAction            string                            `json:"next_action"`
}

type WorkflowGraphPhaseChanges struct {
	Added    []WorkflowGraphPhase       `json:"added"`
	Removed  []WorkflowGraphPhase       `json:"removed"`
	Modified []WorkflowGraphPhaseChange `json:"modified"`
}

type WorkflowGraphPhaseChange struct {
	Key    string             `json:"key"`
	Before WorkflowGraphPhase `json:"before"`
	After  WorkflowGraphPhase `json:"after"`
}

type WorkflowGraphEdgeChanges struct {
	Added    []WorkflowGraphEdge       `json:"added"`
	Removed  []WorkflowGraphEdge       `json:"removed"`
	Modified []WorkflowGraphEdgeChange `json:"modified"`
}

type WorkflowGraphEdgeChange struct {
	Key    string            `json:"key"`
	Before WorkflowGraphEdge `json:"before"`
	After  WorkflowGraphEdge `json:"after"`
}

type WorkflowGraphGateChanges struct {
	Added    []WorkflowGraphGate       `json:"added"`
	Removed  []WorkflowGraphGate       `json:"removed"`
	Modified []WorkflowGraphGateChange `json:"modified"`
}

type WorkflowGraphGateChange struct {
	Key    string            `json:"key"`
	Before WorkflowGraphGate `json:"before"`
	After  WorkflowGraphGate `json:"after"`
}

type WorkflowGraphApprovalChanges struct {
	Added    []WorkflowGraphApproval       `json:"added"`
	Removed  []WorkflowGraphApproval       `json:"removed"`
	Modified []WorkflowGraphApprovalChange `json:"modified"`
}

type WorkflowGraphApprovalChange struct {
	Key    string                `json:"key"`
	Before WorkflowGraphApproval `json:"before"`
	After  WorkflowGraphApproval `json:"after"`
}

type WorkflowGraphFeedbackIntakeChange struct {
	Changed bool                         `json:"changed"`
	Before  *WorkflowGraphFeedbackIntake `json:"before,omitempty"`
	After   *WorkflowGraphFeedbackIntake `json:"after,omitempty"`
}

type GraphProposalValidationSummary struct {
	Base      GraphValidationResult `json:"base"`
	Candidate GraphValidationResult `json:"candidate"`
}

type GraphProposalResult struct {
	SchemaVersion     string                         `json:"schema_version"`
	Status            string                         `json:"status"`
	ProposalID        string                         `json:"proposal_id"`
	ProposalPath      string                         `json:"proposal_path"`
	ValidationSummary GraphProposalValidationSummary `json:"validation_summary"`
	SemanticDiffRef   string                         `json:"semantic_diff_ref"`
	ApprovalRequired  bool                           `json:"approval_required"`
	EventID           string                         `json:"event_id,omitempty"`
	NextAction        string                         `json:"next_action"`
}

type GraphApplyResult struct {
	SchemaVersion string   `json:"schema_version"`
	Status        string   `json:"status"`
	ProposalID    string   `json:"proposal_id"`
	ApprovalRef   string   `json:"approval_ref"`
	GraphPath     string   `json:"graph_path"`
	NewChecksum   string   `json:"new_checksum"`
	EventIDs      []string `json:"event_ids"`
	NextAction    string   `json:"next_action"`
}

type GraphInitResult struct {
	SchemaVersion  string `json:"schema_version"`
	Status         string `json:"status"`
	TemplateID     string `json:"template_id"`
	TemplateSource string `json:"template_source"`
	GraphPath      string `json:"graph_path"`
	Checksum       string `json:"checksum"`
	EventID        string `json:"event_id"`
	NextAction     string `json:"next_action"`
}

type GraphExportResult struct {
	SchemaVersion     string                `json:"schema_version"`
	Status            string                `json:"status"`
	Format            string                `json:"format"`
	OutputPath        string                `json:"output_path"`
	SourceFile        string                `json:"source_file"`
	SourceChecksum    string                `json:"source_checksum"`
	Authoritative     bool                  `json:"authoritative"`
	Diagram           string                `json:"diagram"`
	ValidationSummary GraphValidationResult `json:"validation_summary"`
	NextAction        string                `json:"next_action"`
}

type WorkflowGraphProposalRecord struct {
	SchemaVersion     string                         `json:"schema_version"`
	Status            string                         `json:"status"`
	ProposalID        string                         `json:"proposal_id"`
	ProposalPath      string                         `json:"proposal_path"`
	CreatedAt         string                         `json:"created_at"`
	Reason            string                         `json:"reason"`
	Base              GraphDiffEndpoint              `json:"base"`
	Candidate         GraphDiffEndpoint              `json:"candidate"`
	ValidationSummary GraphProposalValidationSummary `json:"validation_summary"`
	SemanticDiff      GraphDiffResult                `json:"semantic_diff"`
	ApprovalRequired  bool                           `json:"approval_required"`
	NextAction        string                         `json:"next_action"`
}

type WorkflowGraph struct {
	Version        string
	GraphID        string
	Metadata       WorkflowGraphMetadata
	Phases         []WorkflowGraphPhase
	Edges          []WorkflowGraphEdge
	Gates          []WorkflowGraphGate
	Approvals      []WorkflowGraphApproval
	Proposals      WorkflowGraphProposals
	FeedbackIntake *WorkflowGraphFeedbackIntake
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
	ID            string               `json:"id"`
	Requires      []string             `json:"requires"`
	FinalRequired bool                 `json:"final_required,omitempty"`
	Checks        []WorkflowGraphCheck `json:"checks"`

	seenFields map[string]bool
}

type WorkflowGraphCheck struct {
	Type    string   `json:"type"`
	Name    string   `json:"name,omitempty"`
	Path    string   `json:"path,omitempty"`
	Field   string   `json:"field,omitempty"`
	Equals  string   `json:"equals,omitempty"`
	OneOf   []string `json:"one_of,omitempty"`
	Token   string   `json:"token,omitempty"`
	Tokens  []string `json:"tokens,omitempty"`
	Phase   string   `json:"phase,omitempty"`
	Status  string   `json:"status,omitempty"`
	Message string   `json:"message,omitempty"`
	Hint    string   `json:"hint,omitempty"`

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

type WorkflowGraphFeedbackIntake struct {
	Policy         string `json:"policy"`
	SchemaVersion  string `json:"schema_version"`
	MinRounds      int    `json:"min_rounds"`
	MaxRounds      int    `json:"max_rounds"`
	RequiredRounds []int  `json:"required_rounds"`
	OptionalRounds []int  `json:"optional_rounds"`

	policySet         bool
	schemaVersionSet  bool
	minRoundsSet      bool
	maxRoundsSet      bool
	requiredRoundsSet bool
	optionalRoundsSet bool
}

type graphDocument struct {
	graph                WorkflowGraph
	errors               []GraphIssue
	metadataFields       map[string]bool
	proposalFields       map[string]bool
	feedbackIntakeFields map[string]bool
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
	result.FeedbackIntake = cleanWorkflowGraphFeedbackIntakePtr(loaded.graph.FeedbackIntake)
	return result
}

func DiffWorkflowGraph(root Root, options GraphDiffOptions) GraphDiffResult {
	from := loadWorkflowGraph(root, GraphOptions{File: options.From})
	to := loadWorkflowGraph(root, GraphOptions{File: options.To})
	return diffLoadedWorkflowGraphs(from, to)
}

func ExportWorkflowGraph(root Root, options GraphExportOptions) (GraphExportResult, error) {
	format := strings.TrimSpace(options.Format)
	if !supportedGraphExportFormat(format) {
		return GraphExportResult{}, &Problem{Code: "graph_export_format_invalid", Message: "graph export format is not supported", Hint: "Use graph export --format mermaid or graph export --format plantuml.", Field: "format", Expected: "mermaid or plantuml", Actual: format}
	}
	loaded := loadWorkflowGraph(root, GraphOptions{File: WorkflowGraphDefaultPath})
	result := GraphExportResult{
		SchemaVersion:     WorkflowGraphSchemaVersion,
		Status:            loaded.validation.Status,
		Format:            format,
		SourceFile:        loaded.validation.File,
		SourceChecksum:    loaded.validation.Checksum,
		Authoritative:     false,
		ValidationSummary: loaded.validation,
		NextAction:        loaded.validation.NextAction,
	}
	if loaded.validation.Status != GraphStatusPass {
		return result, nil
	}
	switch format {
	case graphExportFormatMermaid:
		result.Diagram = renderWorkflowGraphMermaid(loaded.graph)
	case graphExportFormatPlantUML:
		result.Diagram = renderWorkflowGraphPlantUML(loaded.graph)
	}
	result.NextAction = graphNextActionExported
	output := strings.TrimSpace(options.Output)
	if output == "" {
		return result, nil
	}
	outputPath, err := resolveGraphExportOutput(root, output, format)
	if err != nil {
		return GraphExportResult{}, err
	}
	result.OutputPath = outputPath.Relative
	if err := writeNewFileAtomically(outputPath, []byte(result.Diagram)); err != nil {
		return GraphExportResult{}, err
	}
	return result, nil
}

func InitWorkflowGraph(root Root, options GraphInitOptions) (GraphInitResult, error) {
	var result GraphInitResult
	err := withProjectWriteLock(root, "graph init", "", func() error {
		var err error
		result, err = initWorkflowGraphUnlocked(root, options)
		return err
	})
	return result, err
}

func initWorkflowGraphUnlocked(root Root, options GraphInitOptions) (GraphInitResult, error) {
	output := strings.TrimSpace(options.Output)
	if output == "" {
		output = WorkflowGraphDefaultPath
	}
	if output != WorkflowGraphDefaultPath {
		return GraphInitResult{}, &Problem{Code: "graph_output_invalid", Message: "graph init output is not supported", Hint: "graph-004 only writes the initial .kkachi-workflow.yaml source of truth.", Field: "output", Expected: WorkflowGraphDefaultPath, Actual: output}
	}
	outputPath, err := ResolveRelativePath(root, output)
	if err != nil {
		return GraphInitResult{}, err
	}
	if err := rejectExistingWorkflowGraph(outputPath); err != nil {
		return GraphInitResult{}, err
	}
	template, err := resolveGraphTemplate(root, options.FromTemplate)
	if err != nil {
		return GraphInitResult{}, err
	}
	if options.Now == nil {
		options.Now = func() time.Time { return time.Now().UTC() }
	}

	var rendered []byte
	var checksum string
	appendResult, err := appendEventWithPreparedStatusMutation(root, AppendEventOptions{Type: graphInitEventType, Now: options.Now}, func(status map[string]any, nextID string, _ string) (preparedEventStatusMutation, error) {
		projectID, projectName, err := graphInitProjectFacts(root, status)
		if err != nil {
			return preparedEventStatusMutation{}, err
		}
		graph := template.Graph
		stampWorkflowGraph(&graph, projectID, projectName, template.ID, nextID)
		rendered = encodeWorkflowGraph(graph)
		validation := validateRenderedWorkflowGraph(rendered, outputPath.Relative)
		if validation.Status != GraphStatusPass {
			return preparedEventStatusMutation{}, graphInitValidationProblem(validation)
		}
		checksum = validation.Checksum
		payload := map[string]any{
			"template_id":     template.ID,
			"template_source": template.Source,
			"graph_path":      outputPath.Relative,
			"checksum":        checksum,
		}
		return preparedEventStatusMutation{
			Payload: payload,
			BeforeAppend: func() error {
				return writeNewFileAtomically(outputPath, rendered)
			},
		}, nil
	})
	if err != nil {
		return GraphInitResult{}, err
	}
	return GraphInitResult{
		SchemaVersion:  WorkflowGraphSchemaVersion,
		Status:         GraphStatusPass,
		TemplateID:     template.ID,
		TemplateSource: template.Source,
		GraphPath:      outputPath.Relative,
		Checksum:       checksum,
		EventID:        appendResult.EventID,
		NextAction:     graphNextActionInitialized,
	}, nil
}

func ProposeWorkflowGraph(root Root, options GraphProposeOptions) (GraphProposalResult, error) {
	var result GraphProposalResult
	err := withProjectWriteLock(root, "graph propose", "", func() error {
		var err error
		result, err = proposeWorkflowGraphUnlocked(root, options)
		return err
	})
	return result, err
}

func proposeWorkflowGraphUnlocked(root Root, options GraphProposeOptions) (GraphProposalResult, error) {
	patch := strings.TrimSpace(options.Patch)
	reason := strings.TrimSpace(options.Reason)
	if patch == "" {
		return GraphProposalResult{}, &Problem{Code: "graph_patch_required", Message: "graph proposal candidate graph is required", Hint: "Pass --candidate-file, or legacy --patch, with a repository-relative complete candidate workflow graph file.", Field: "patch", Expected: "non-empty repository-relative graph path", Actual: "empty"}
	}
	if reason == "" {
		return GraphProposalResult{}, &Problem{Code: "graph_proposal_reason_required", Message: "graph proposal reason is required", Hint: "Pass --reason to explain why this graph change is proposed.", Field: "reason", Expected: "non-empty proposal reason", Actual: "empty"}
	}
	if options.Now == nil {
		options.Now = func() time.Time { return time.Now().UTC() }
	}
	if err := preflightEventCoherence(root); err != nil {
		return GraphProposalResult{}, err
	}

	base := loadWorkflowGraph(root, GraphOptions{File: WorkflowGraphDefaultPath})
	candidate := loadWorkflowGraph(root, GraphOptions{File: patch})
	diff := diffLoadedWorkflowGraphs(base, candidate)
	if diff.Status != GraphStatusPass {
		if !canRecordFeedbackIntakeMigrationProposal(base, candidate) {
			return GraphProposalResult{}, graphValidationProblem("graph_proposal_invalid", "cannot record proposal for invalid workflow graph input", "Repair the base graph and candidate graph, then rerun graph propose.", diff.ValidationSummary)
		}
		diff = diffLoadedWorkflowGraphsAllowingStaleFeedbackIntakeBase(base, candidate)
		if !workflowGraphDiffOnlyFeedbackIntake(diff) {
			return GraphProposalResult{}, graphValidationProblem("graph_proposal_invalid", "cannot record stale feedback intake migration with unrelated graph changes", "Record feedback intake migration separately from other graph changes.", diff.ValidationSummary)
		}
	}

	proposalID, proposalPath, err := nextGraphProposalPath(root)
	if err != nil {
		return GraphProposalResult{}, err
	}
	created := options.Now().UTC()
	createdAt := created.Format(time.RFC3339)
	semanticDiffRef := proposalPath.Relative + "#semantic_diff"
	record := WorkflowGraphProposalRecord{
		SchemaVersion: WorkflowGraphSchemaVersion,
		Status:        GraphStatusPass,
		ProposalID:    proposalID,
		ProposalPath:  proposalPath.Relative,
		CreatedAt:     createdAt,
		Reason:        reason,
		Base:          diff.From,
		Candidate:     diff.To,
		ValidationSummary: GraphProposalValidationSummary{
			Base:      base.validation,
			Candidate: candidate.validation,
		},
		SemanticDiff:     diff,
		ApprovalRequired: diff.RequiresApproval,
		NextAction:       graphNextActionProposal,
	}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return GraphProposalResult{}, &Problem{Code: "graph_proposal_encode_failed", Message: "cannot encode graph proposal record", Hint: "Inspect graph proposal fields for unsupported JSON values.", Field: "proposal", Expected: "JSON-encodable proposal record", Actual: err.Error()}
	}
	data = append(data, '\n')
	payload := map[string]any{
		"proposal_id":        proposalID,
		"proposal_path":      proposalPath.Relative,
		"semantic_diff_ref":  semanticDiffRef,
		"base_checksum":      diff.From.Checksum,
		"candidate_checksum": diff.To.Checksum,
		"approval_required":  diff.RequiresApproval,
		"reason":             reason,
	}
	appendResult, err := appendEventWithStatusMutation(root, AppendEventOptions{Type: graphProposalEventType, Payload: payload, Now: func() time.Time {
		return created
	}}, func(map[string]any, string) error {
		return writeNewFileAtomically(proposalPath, data)
	})
	if err != nil {
		return GraphProposalResult{}, err
	}
	return GraphProposalResult{
		SchemaVersion:     WorkflowGraphSchemaVersion,
		Status:            GraphStatusPass,
		ProposalID:        proposalID,
		ProposalPath:      proposalPath.Relative,
		ValidationSummary: record.ValidationSummary,
		SemanticDiffRef:   semanticDiffRef,
		ApprovalRequired:  diff.RequiresApproval,
		EventID:           appendResult.EventID,
		NextAction:        graphNextActionProposal,
	}, nil
}

func ApplyWorkflowGraph(root Root, options GraphApplyOptions) (GraphApplyResult, error) {
	var result GraphApplyResult
	err := withProjectWriteLock(root, "graph apply", "", func() error {
		var err error
		result, err = applyWorkflowGraphUnlocked(root, options)
		return err
	})
	return result, err
}

func applyWorkflowGraphUnlocked(root Root, options GraphApplyOptions) (GraphApplyResult, error) {
	proposalID := strings.TrimSpace(options.Proposal)
	approvalRef := strings.TrimSpace(options.Approval)
	if proposalID == "" {
		return GraphApplyResult{}, &Problem{Code: "graph_proposal_required", Message: "graph apply proposal is required", Hint: "Pass --proposal with a graph proposal id such as gprop-000001.", Field: "proposal", Expected: "non-empty proposal id", Actual: "empty"}
	}
	if !isGraphProposalID(proposalID) {
		return GraphApplyResult{}, &Problem{Code: "graph_proposal_invalid", Message: "graph apply proposal id is invalid", Hint: "Use a proposal id returned by graph propose, such as gprop-000001.", Field: "proposal", Expected: "gprop- followed by six digits", Actual: proposalID}
	}
	if approvalRef == "" {
		return GraphApplyResult{}, &Problem{Code: "graph_approval_required", Message: "graph apply approval evidence is required", Hint: "Pass --approval with the responsible approval evidence reference.", Field: "approval", Expected: "non-empty approval evidence reference", Actual: "empty"}
	}
	if options.Now == nil {
		options.Now = func() time.Time { return time.Now().UTC() }
	}
	if err := preflightEventCoherence(root); err != nil {
		return GraphApplyResult{}, err
	}

	record, proposalPath, err := readWorkflowGraphProposalRecord(root, proposalID)
	if err != nil {
		return GraphApplyResult{}, err
	}
	if err := validateWorkflowGraphProposalRecord(record, proposalID, proposalPath.Relative); err != nil {
		return GraphApplyResult{}, err
	}

	base := loadWorkflowGraph(root, GraphOptions{File: WorkflowGraphDefaultPath})
	allowStaleFeedbackMigration := canApplyFeedbackIntakeMigrationProposal(record, base)
	if base.validation.Status != GraphStatusPass && !allowStaleFeedbackMigration {
		return GraphApplyResult{}, graphValidationProblem("graph_apply_invalid", "cannot apply proposal while current workflow graph is invalid", "Repair .kkachi-workflow.yaml, then rerun graph apply.", GraphDiffValidationSummary{From: base.validation, To: record.ValidationSummary.Candidate})
	}
	if base.validation.Checksum != record.Base.Checksum {
		return GraphApplyResult{}, &Problem{Code: "graph_base_checksum_mismatch", Message: "current workflow graph no longer matches proposal base", Hint: "Record a new proposal against the current .kkachi-workflow.yaml before applying.", Path: WorkflowGraphDefaultPath, Field: "checksum", Expected: record.Base.Checksum, Actual: base.validation.Checksum}
	}

	candidate := loadWorkflowGraph(root, GraphOptions{File: record.Candidate.File})
	if candidate.validation.Status != GraphStatusPass {
		return GraphApplyResult{}, graphValidationProblem("graph_apply_invalid", "cannot apply proposal with invalid candidate workflow graph", "Repair the candidate graph or record a new proposal, then rerun graph apply.", GraphDiffValidationSummary{From: base.validation, To: candidate.validation})
	}
	if allowStaleFeedbackMigration && !workflowGraphHasCanonicalFeedbackIntake(candidate.validation) {
		return GraphApplyResult{}, graphValidationProblem("graph_apply_invalid", "cannot apply stale feedback intake migration without valid candidate bounds", "Record a new proposal whose candidate declares min_rounds=1, max_rounds=5, required_rounds=[1], and optional_rounds=[2,3,4,5].", GraphDiffValidationSummary{From: base.validation, To: candidate.validation})
	}
	if candidate.validation.Checksum != record.Candidate.Checksum {
		return GraphApplyResult{}, &Problem{Code: "graph_candidate_checksum_mismatch", Message: "candidate workflow graph no longer matches proposal record", Hint: "Record a new proposal after changing the candidate graph.", Path: record.Candidate.File, Field: "checksum", Expected: record.Candidate.Checksum, Actual: candidate.validation.Checksum}
	}

	graphPath, err := ResolveRelativePath(root, WorkflowGraphDefaultPath)
	if err != nil {
		return GraphApplyResult{}, err
	}
	var rendered []byte
	var newChecksum string
	appendResult, err := appendEventWithPreparedStatusMutation(root, AppendEventOptions{Type: graphApplyEventType, Now: options.Now}, func(_ map[string]any, nextID string, _ string) (preparedEventStatusMutation, error) {
		applied := candidate.graph
		applied.Metadata.LastAppliedEventID = nextID
		rendered = encodeWorkflowGraph(applied)
		validation := validateRenderedWorkflowGraph(rendered, graphPath.Relative)
		if validation.Status != GraphStatusPass {
			return preparedEventStatusMutation{}, graphApplyRenderedValidationProblem(validation)
		}
		newChecksum = validation.Checksum
		payload := map[string]any{
			"proposal_id":         record.ProposalID,
			"proposal_path":       record.ProposalPath,
			"approval_ref":        approvalRef,
			"graph_path":          graphPath.Relative,
			"base_checksum":       record.Base.Checksum,
			"candidate_checksum":  record.Candidate.Checksum,
			"new_checksum":        newChecksum,
			"semantic_diff_ref":   record.ProposalPath + "#semantic_diff",
			"proposal_created_at": record.CreatedAt,
			"proposal_reason":     record.Reason,
		}
		return preparedEventStatusMutation{
			Payload: payload,
			BeforeAppend: func() error {
				return writeExistingFileAtomically(graphPath, rendered)
			},
		}, nil
	})
	if err != nil {
		return GraphApplyResult{}, err
	}
	return GraphApplyResult{
		SchemaVersion: WorkflowGraphSchemaVersion,
		Status:        GraphStatusPass,
		ProposalID:    record.ProposalID,
		ApprovalRef:   approvalRef,
		GraphPath:     graphPath.Relative,
		NewChecksum:   newChecksum,
		EventIDs:      []string{appendResult.EventID},
		NextAction:    graphNextActionApplied,
	}, nil
}

func readWorkflowGraphProposalRecord(root Root, proposalID string) (WorkflowGraphProposalRecord, SafePath, error) {
	path, err := ResolveRelativePath(root, graphProposalDir+"/"+proposalID+".json")
	if err != nil {
		return WorkflowGraphProposalRecord{}, SafePath{}, err
	}
	data, err := os.ReadFile(path.Absolute)
	if os.IsNotExist(err) {
		return WorkflowGraphProposalRecord{}, path, &Problem{Code: "graph_proposal_missing", Message: "graph proposal record is missing", Hint: "Run graph propose first and pass the returned proposal id.", Path: path.Relative, Field: "proposal", Expected: "existing graph proposal record", Actual: "missing"}
	}
	if err != nil {
		return WorkflowGraphProposalRecord{}, path, &Problem{Code: "graph_proposal_read_failed", Message: "cannot read graph proposal record", Hint: "Check repository permissions before applying graph proposals.", Path: path.Relative, Field: "path", Expected: "readable graph proposal record", Actual: err.Error()}
	}
	var record WorkflowGraphProposalRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return WorkflowGraphProposalRecord{}, path, &Problem{Code: "graph_proposal_invalid_json", Message: "graph proposal record is not valid JSON", Hint: "Record proposals with graph propose so the strict proposal schema is preserved.", Path: path.Relative, Field: "json", Expected: "valid graph proposal JSON", Actual: err.Error()}
	}
	return record, path, nil
}

func isGraphProposalID(value string) bool {
	return graphProposalIDPattern.MatchString(value + ".json")
}

func validateWorkflowGraphProposalRecord(record WorkflowGraphProposalRecord, proposalID string, proposalPath string) error {
	if record.SchemaVersion != WorkflowGraphSchemaVersion {
		return &Problem{Code: "graph_proposal_schema_invalid", Message: "graph proposal schema version is invalid", Hint: "Record a fresh proposal with the current helper before applying.", Path: proposalPath, Field: "schema_version", Expected: WorkflowGraphSchemaVersion, Actual: record.SchemaVersion}
	}
	if record.Status != GraphStatusPass {
		return &Problem{Code: "graph_proposal_status_invalid", Message: "graph proposal status is invalid", Hint: "Only passing proposal records can be applied.", Path: proposalPath, Field: "status", Expected: GraphStatusPass, Actual: record.Status}
	}
	if record.ProposalID != proposalID {
		return &Problem{Code: "graph_proposal_id_mismatch", Message: "graph proposal id does not match the requested proposal", Hint: "Inspect the proposal record and rerun graph apply with the matching proposal id.", Path: proposalPath, Field: "proposal_id", Expected: proposalID, Actual: record.ProposalID}
	}
	if record.ProposalPath != proposalPath {
		return &Problem{Code: "graph_proposal_path_mismatch", Message: "graph proposal path does not match its stored record", Hint: "Record a fresh proposal with graph propose before applying.", Path: proposalPath, Field: "proposal_path", Expected: proposalPath, Actual: record.ProposalPath}
	}
	if strings.TrimSpace(record.Base.File) == "" || strings.TrimSpace(record.Base.Checksum) == "" || strings.TrimSpace(record.Candidate.File) == "" || strings.TrimSpace(record.Candidate.Checksum) == "" {
		return &Problem{Code: "graph_proposal_record_invalid", Message: "graph proposal record is missing base or candidate evidence", Hint: "Record a fresh proposal with graph propose before applying.", Path: proposalPath, Field: "base_candidate", Expected: "base and candidate file/checksum evidence", Actual: "missing"}
	}
	if record.Base.File != WorkflowGraphDefaultPath {
		return &Problem{Code: "graph_proposal_base_invalid", Message: "graph proposal base is not the project workflow graph", Hint: "Record a fresh proposal against .kkachi-workflow.yaml before applying.", Path: proposalPath, Field: "base.file", Expected: WorkflowGraphDefaultPath, Actual: record.Base.File}
	}
	return nil
}

func graphApplyRenderedValidationProblem(validation GraphValidationResult) error {
	return graphValidationProblem("graph_apply_rendered_invalid", "applied workflow graph would be invalid", "Inspect the proposal candidate and record a fresh valid proposal before applying.", GraphDiffValidationSummary{From: validation, To: validation})
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

type workflowGraphTemplate struct {
	ID     string
	Source string
	Graph  WorkflowGraph
}

func resolveGraphTemplate(root Root, value string) (workflowGraphTemplate, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return workflowGraphTemplate{}, &Problem{Code: "graph_template_required", Message: "graph init template is required", Hint: "Pass --from-template khs-default or a repository-relative workflow graph YAML template.", Field: "from_template", Expected: "template id or repository-relative YAML path", Actual: "empty"}
	}
	if value == graphTemplateKHSDefault {
		return workflowGraphTemplate{ID: graphTemplateKHSDefault, Source: graphTemplateSourceBuiltin, Graph: builtInKHSDefaultWorkflowGraph()}, nil
	}
	lower := strings.ToLower(value)
	if strings.Contains(value, "/") || strings.HasSuffix(lower, ".yaml") || strings.HasSuffix(lower, ".yml") {
		loaded := loadWorkflowGraph(root, GraphOptions{File: value})
		if loaded.validation.Status != GraphStatusPass {
			return workflowGraphTemplate{}, graphTemplateValidationProblem(loaded.validation)
		}
		return workflowGraphTemplate{ID: loaded.validation.File, Source: graphTemplateSourcePath, Graph: loaded.graph}, nil
	}
	return workflowGraphTemplate{}, &Problem{Code: "graph_template_unknown", Message: "graph init template is unknown", Hint: "Use built-in template khs-default or pass a repository-relative .yaml/.yml template path.", Field: "from_template", Expected: graphTemplateKHSDefault + " or repository-relative YAML path", Actual: value}
}

func builtInKHSDefaultWorkflowGraph() WorkflowGraph {
	phases := make([]WorkflowGraphPhase, 0, len(defaultPhaseIDs))
	for _, id := range defaultPhaseIDs {
		phases = append(phases, WorkflowGraphPhase{ID: id, Title: graphPhaseTitle(id), OwnerLayer: "khs", Required: true, Evidence: []string{}})
	}
	edges := make([]WorkflowGraphEdge, 0, len(defaultPhaseIDs)-1)
	for i := 0; i+1 < len(defaultPhaseIDs); i++ {
		edges = append(edges, WorkflowGraphEdge{From: defaultPhaseIDs[i], To: defaultPhaseIDs[i+1]})
	}
	return WorkflowGraph{
		Version: WorkflowGraphSchemaVersion,
		Metadata: WorkflowGraphMetadata{
			CreatedBy: "khs",
			ManagedBy: "kah",
		},
		Phases:    phases,
		Edges:     edges,
		Gates:     []WorkflowGraphGate{},
		Approvals: []WorkflowGraphApproval{},
		Proposals: WorkflowGraphProposals{Policy: "proposal-first"},
	}
}

func graphPhaseTitle(id string) string {
	parts := strings.Split(id, "-")
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, " ")
}

func stampWorkflowGraph(graph *WorkflowGraph, projectID string, projectName string, sourceTemplate string, eventID string) {
	graph.Version = WorkflowGraphSchemaVersion
	graph.GraphID = "graph-" + projectID
	graph.Metadata.Project = projectName
	if strings.TrimSpace(graph.Metadata.CreatedBy) == "" {
		graph.Metadata.CreatedBy = "khs"
	}
	graph.Metadata.ManagedBy = "kah"
	graph.Metadata.SourceTemplate = sourceTemplate
	graph.Metadata.LastAppliedEventID = eventID
}

func graphInitProjectFacts(root Root, status map[string]any) (string, string, error) {
	projectID, ok := status["project_id"].(string)
	projectID = strings.TrimSpace(projectID)
	if !ok || projectID == "" {
		return "", "", &Problem{Code: "status_project_id_invalid", Message: "project status is missing a project id", Hint: "Restore status.json before initializing workflow graph state.", Path: StatusPath, Field: "project_id", Expected: "non-empty string", Actual: "missing"}
	}
	configPath, err := ResolveRelativePath(root, ConfigPath)
	if err != nil {
		return "", "", err
	}
	data, err := os.ReadFile(configPath.Absolute)
	if err != nil {
		return "", "", &Problem{Code: "project_config_read_failed", Message: "cannot read project config", Hint: "Run project init first or restore .kkachi/config.yaml from backup.", Path: configPath.Relative, Field: "path", Expected: "readable project config", Actual: err.Error()}
	}
	values := parseSimpleConfig(data)
	projectName := strings.TrimSpace(values["project.name"])
	if projectName == "" {
		return "", "", &Problem{Code: "project_config_invalid", Message: "project config is missing project name", Hint: "Restore the config generated by project init before initializing workflow graph state.", Path: configPath.Relative, Field: "project.name", Expected: "non-empty project name", Actual: "missing"}
	}
	return projectID, projectName, nil
}

func rejectExistingWorkflowGraph(path SafePath) error {
	info, err := os.Lstat(path.Absolute)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return &Problem{Code: "path_inspection_failed", Message: "cannot inspect workflow graph path", Hint: "Check repository permissions before initializing workflow graph state.", Path: path.Relative, Field: "path", Expected: "inspectable workflow graph path", Actual: err.Error()}
	}
	actual := "file exists"
	if info.IsDir() {
		actual = "directory exists"
	} else if info.Mode()&os.ModeSymlink != 0 {
		actual = "symlink exists"
	}
	return &Problem{Code: "graph_already_exists", Message: "workflow graph already exists", Hint: "Use graph propose and graph apply, or remove/repair manually with explicit governance.", Path: path.Relative, Field: "path", Expected: WorkflowGraphDefaultPath + " does not exist", Actual: actual}
}

func validateRenderedWorkflowGraph(data []byte, path string) GraphValidationResult {
	doc := parseWorkflowGraph(data, path)
	errors := append([]GraphIssue{}, doc.errors...)
	errors = append(errors, validateWorkflowGraph(doc.graph, path)...)
	sum := sha256.Sum256(data)
	status := GraphStatusPass
	nextAction := graphNextActionValid
	if len(errors) > 0 {
		status = GraphStatusFail
		nextAction = graphNextActionRepair
	}
	return GraphValidationResult{SchemaVersion: WorkflowGraphSchemaVersion, Status: status, File: path, Checksum: hex.EncodeToString(sum[:]), EffectiveSource: graphEffectiveSourceProject, FeedbackIntake: cleanWorkflowGraphFeedbackIntakePtr(doc.graph.FeedbackIntake), Errors: errors, Warnings: []GraphIssue{}, Conflicts: []GraphIssue{}, NextAction: nextAction}
}

func encodeWorkflowGraph(graph WorkflowGraph) []byte {
	var builder strings.Builder
	builder.WriteString("version: ")
	builder.WriteString(yamlQuotedScalar(graph.Version))
	builder.WriteString("\n")
	builder.WriteString("graph_id: ")
	builder.WriteString(yamlQuotedScalar(graph.GraphID))
	builder.WriteString("\n")
	builder.WriteString("metadata:\n")
	builder.WriteString("  project: ")
	builder.WriteString(yamlQuotedScalar(graph.Metadata.Project))
	builder.WriteString("\n")
	builder.WriteString("  created_by: ")
	builder.WriteString(yamlQuotedScalar(graph.Metadata.CreatedBy))
	builder.WriteString("\n")
	builder.WriteString("  managed_by: ")
	builder.WriteString(yamlQuotedScalar(graph.Metadata.ManagedBy))
	builder.WriteString("\n")
	if graph.Metadata.SourceTemplate != "" {
		builder.WriteString("  source_template: ")
		builder.WriteString(yamlQuotedScalar(graph.Metadata.SourceTemplate))
		builder.WriteString("\n")
	}
	if graph.Metadata.LastAppliedEventID != "" {
		builder.WriteString("  last_applied_event_id: ")
		builder.WriteString(yamlQuotedScalar(graph.Metadata.LastAppliedEventID))
		builder.WriteString("\n")
	}
	builder.WriteString("phases:\n")
	for _, phase := range graph.Phases {
		builder.WriteString("  - id: ")
		builder.WriteString(yamlQuotedScalar(phase.ID))
		builder.WriteString("\n")
		if phase.Title != "" {
			builder.WriteString("    title: ")
			builder.WriteString(yamlQuotedScalar(phase.Title))
			builder.WriteString("\n")
		}
		if phase.OwnerLayer != "" {
			builder.WriteString("    owner_layer: ")
			builder.WriteString(yamlQuotedScalar(phase.OwnerLayer))
			builder.WriteString("\n")
		}
		builder.WriteString("    required: ")
		builder.WriteString(strconv.FormatBool(phase.Required))
		builder.WriteString("\n")
		if len(phase.Evidence) > 0 {
			builder.WriteString("    evidence: ")
			builder.WriteString(graphYAMLStringList(phase.Evidence))
			builder.WriteString("\n")
		}
	}
	if len(graph.Edges) > 0 {
		builder.WriteString("edges:\n")
		for _, edge := range graph.Edges {
			builder.WriteString("  - from: ")
			builder.WriteString(yamlQuotedScalar(edge.From))
			builder.WriteString("\n")
			builder.WriteString("    to: ")
			builder.WriteString(yamlQuotedScalar(edge.To))
			builder.WriteString("\n")
		}
	}
	if len(graph.Gates) > 0 {
		builder.WriteString("gates:\n")
		for _, gate := range graph.Gates {
			builder.WriteString("  - id: ")
			builder.WriteString(yamlQuotedScalar(gate.ID))
			builder.WriteString("\n")
			builder.WriteString("    requires: ")
			builder.WriteString(graphYAMLStringList(gate.Requires))
			builder.WriteString("\n")
			if gate.FinalRequired {
				builder.WriteString("    final_required: true\n")
			}
			if len(gate.Checks) > 0 {
				builder.WriteString("    checks:\n")
				for _, check := range gate.Checks {
					builder.WriteString("      - type: ")
					builder.WriteString(yamlQuotedScalar(check.Type))
					builder.WriteString("\n")
					writeWorkflowGraphCheckScalar(&builder, "name", check.Name)
					writeWorkflowGraphCheckScalar(&builder, "path", check.Path)
					writeWorkflowGraphCheckScalar(&builder, "field", check.Field)
					writeWorkflowGraphCheckScalar(&builder, "equals", check.Equals)
					if len(check.OneOf) > 0 {
						builder.WriteString("        one_of: ")
						builder.WriteString(graphYAMLStringList(check.OneOf))
						builder.WriteString("\n")
					}
					writeWorkflowGraphCheckScalar(&builder, "token", check.Token)
					if len(check.Tokens) > 0 {
						builder.WriteString("        tokens: ")
						builder.WriteString(graphYAMLStringList(check.Tokens))
						builder.WriteString("\n")
					}
					writeWorkflowGraphCheckScalar(&builder, "phase", check.Phase)
					writeWorkflowGraphCheckScalar(&builder, "status", check.Status)
					writeWorkflowGraphCheckScalar(&builder, "message", check.Message)
					writeWorkflowGraphCheckScalar(&builder, "hint", check.Hint)
				}
			}
		}
	}
	if len(graph.Approvals) > 0 {
		builder.WriteString("approvals:\n")
		for _, approval := range graph.Approvals {
			builder.WriteString("  - scope: ")
			builder.WriteString(yamlQuotedScalar(approval.Scope))
			builder.WriteString("\n")
			builder.WriteString("    required_role: ")
			builder.WriteString(yamlQuotedScalar(approval.RequiredRole))
			builder.WriteString("\n")
		}
	}
	if graph.Proposals.Policy != "" {
		builder.WriteString("proposals:\n")
		builder.WriteString("  policy: ")
		builder.WriteString(yamlQuotedScalar(graph.Proposals.Policy))
		builder.WriteString("\n")
	}
	if graph.FeedbackIntake != nil {
		feedback := cleanWorkflowGraphFeedbackIntake(*graph.FeedbackIntake)
		builder.WriteString("feedback_intake:\n")
		builder.WriteString("  policy: ")
		builder.WriteString(yamlQuotedScalar(feedback.Policy))
		builder.WriteString("\n")
		builder.WriteString("  schema_version: ")
		builder.WriteString(yamlQuotedScalar(feedback.SchemaVersion))
		builder.WriteString("\n")
		builder.WriteString("  min_rounds: ")
		builder.WriteString(strconv.Itoa(feedback.MinRounds))
		builder.WriteString("\n")
		builder.WriteString("  max_rounds: ")
		builder.WriteString(strconv.Itoa(feedback.MaxRounds))
		builder.WriteString("\n")
		builder.WriteString("  required_rounds: ")
		builder.WriteString(graphYAMLIntList(feedback.RequiredRounds))
		builder.WriteString("\n")
		builder.WriteString("  optional_rounds: ")
		builder.WriteString(graphYAMLIntList(feedback.OptionalRounds))
		builder.WriteString("\n")
	}
	return []byte(builder.String())
}

func writeWorkflowGraphCheckScalar(builder *strings.Builder, key string, value string) {
	if value == "" {
		return
	}
	builder.WriteString("        ")
	builder.WriteString(key)
	builder.WriteString(": ")
	builder.WriteString(yamlQuotedScalar(value))
	builder.WriteString("\n")
}

func graphYAMLStringList(values []string) string {
	if len(values) == 0 {
		return "[]"
	}
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, yamlQuotedScalar(value))
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

func graphYAMLIntList(values []int) string {
	if len(values) == 0 {
		return "[]"
	}
	items := make([]string, 0, len(values))
	for _, value := range values {
		items = append(items, strconv.Itoa(value))
	}
	return "[" + strings.Join(items, ", ") + "]"
}

func diffLoadedWorkflowGraphs(from loadedWorkflowGraph, to loadedWorkflowGraph) GraphDiffResult {
	result := newGraphDiffResult(from, to)
	if from.validation.Status != GraphStatusPass || to.validation.Status != GraphStatusPass {
		result.Status = GraphStatusFail
		result.NextAction = graphNextActionRepair
		return result
	}
	return populateGraphDiffChanges(result, from.graph, to.graph)
}

func graphDiffEndpoint(validation GraphValidationResult) GraphDiffEndpoint {
	return GraphDiffEndpoint{File: validation.File, Checksum: validation.Checksum, EffectiveSource: validation.EffectiveSource}
}

func diffLoadedWorkflowGraphsAllowingStaleFeedbackIntakeBase(from loadedWorkflowGraph, to loadedWorkflowGraph) GraphDiffResult {
	return populateGraphDiffChanges(newGraphDiffResult(from, to), from.graph, to.graph)
}

func newGraphDiffResult(from loadedWorkflowGraph, to loadedWorkflowGraph) GraphDiffResult {
	return GraphDiffResult{
		SchemaVersion:         WorkflowGraphSchemaVersion,
		Status:                GraphStatusPass,
		From:                  graphDiffEndpoint(from.validation),
		To:                    graphDiffEndpoint(to.validation),
		ChangedPhases:         WorkflowGraphPhaseChanges{Added: []WorkflowGraphPhase{}, Removed: []WorkflowGraphPhase{}, Modified: []WorkflowGraphPhaseChange{}},
		ChangedEdges:          WorkflowGraphEdgeChanges{Added: []WorkflowGraphEdge{}, Removed: []WorkflowGraphEdge{}, Modified: []WorkflowGraphEdgeChange{}},
		ChangedGates:          WorkflowGraphGateChanges{Added: []WorkflowGraphGate{}, Removed: []WorkflowGraphGate{}, Modified: []WorkflowGraphGateChange{}},
		ChangedApprovals:      WorkflowGraphApprovalChanges{Added: []WorkflowGraphApproval{}, Removed: []WorkflowGraphApproval{}, Modified: []WorkflowGraphApprovalChange{}},
		ChangedFeedbackIntake: WorkflowGraphFeedbackIntakeChange{},
		RiskFlags:             []string{},
		ValidationSummary: GraphDiffValidationSummary{
			From: from.validation,
			To:   to.validation,
		},
		NextAction: graphNextActionDiffReady,
	}
}

func populateGraphDiffChanges(result GraphDiffResult, from WorkflowGraph, to WorkflowGraph) GraphDiffResult {
	result.ChangedPhases = diffWorkflowGraphPhases(from.Phases, to.Phases)
	result.ChangedEdges = diffWorkflowGraphEdges(from.Edges, to.Edges)
	result.ChangedGates = diffWorkflowGraphGates(from.Gates, to.Gates)
	result.ChangedApprovals = diffWorkflowGraphApprovals(from.Approvals, to.Approvals)
	result.ChangedFeedbackIntake = diffWorkflowGraphFeedbackIntake(from.FeedbackIntake, to.FeedbackIntake)
	result.RiskFlags = workflowGraphRiskFlags(from, to, result)
	result.RequiresApproval = len(result.RiskFlags) > 0
	return result
}

func canRecordFeedbackIntakeMigrationProposal(base loadedWorkflowGraph, candidate loadedWorkflowGraph) bool {
	return graphValidationOnlyFeedbackStaleBounds(base.validation) && workflowGraphHasCanonicalFeedbackIntake(candidate.validation)
}

func canApplyFeedbackIntakeMigrationProposal(record WorkflowGraphProposalRecord, base loadedWorkflowGraph) bool {
	return graphValidationOnlyFeedbackStaleBounds(base.validation) &&
		graphValidationOnlyFeedbackStaleBounds(record.ValidationSummary.Base) &&
		record.ValidationSummary.Candidate.Status == GraphStatusPass &&
		record.ValidationSummary.Candidate.FeedbackIntake != nil &&
		workflowGraphDiffOnlyFeedbackIntake(record.SemanticDiff)
}

func graphValidationOnlyFeedbackStaleBounds(validation GraphValidationResult) bool {
	if validation.Status != GraphStatusFail || validation.File != WorkflowGraphDefaultPath || len(validation.Errors) == 0 || len(validation.Warnings) != 0 || len(validation.Conflicts) != 0 {
		return false
	}
	for _, issue := range validation.Errors {
		if issue.Name != "feedback_intake_stale_bounds" {
			return false
		}
	}
	return true
}

func workflowGraphHasCanonicalFeedbackIntake(validation GraphValidationResult) bool {
	if validation.Status != GraphStatusPass || validation.FeedbackIntake == nil {
		return false
	}
	return reflect.DeepEqual(cleanWorkflowGraphFeedbackIntake(*validation.FeedbackIntake), WorkflowGraphFeedbackIntake{
		Policy:         graphFeedbackIntakePolicy,
		SchemaVersion:  graphFeedbackIntakeSchema,
		MinRounds:      1,
		MaxRounds:      5,
		RequiredRounds: []int{1},
		OptionalRounds: []int{2, 3, 4, 5},
	})
}

func workflowGraphDiffOnlyFeedbackIntake(diff GraphDiffResult) bool {
	return diff.Status == GraphStatusPass &&
		diff.ChangedFeedbackIntake.Changed &&
		len(diff.ChangedPhases.Added) == 0 &&
		len(diff.ChangedPhases.Removed) == 0 &&
		len(diff.ChangedPhases.Modified) == 0 &&
		len(diff.ChangedEdges.Added) == 0 &&
		len(diff.ChangedEdges.Removed) == 0 &&
		len(diff.ChangedEdges.Modified) == 0 &&
		len(diff.ChangedGates.Added) == 0 &&
		len(diff.ChangedGates.Removed) == 0 &&
		len(diff.ChangedGates.Modified) == 0 &&
		len(diff.ChangedApprovals.Added) == 0 &&
		len(diff.ChangedApprovals.Removed) == 0 &&
		len(diff.ChangedApprovals.Modified) == 0 &&
		slices.Equal(diff.RiskFlags, []string{"feedback_intake_changed"})
}

func diffWorkflowGraphPhases(from []WorkflowGraphPhase, to []WorkflowGraphPhase) WorkflowGraphPhaseChanges {
	added, removed, modified := diffWorkflowGraphEntities(
		from,
		to,
		func(phase WorkflowGraphPhase) string { return phase.ID },
		cleanWorkflowGraphPhase,
		func(phase WorkflowGraphPhase) any { return canonicalWorkflowGraphPhase(phase) },
		func(key string, before WorkflowGraphPhase, after WorkflowGraphPhase) WorkflowGraphPhaseChange {
			return WorkflowGraphPhaseChange{Key: key, Before: before, After: after}
		},
	)
	return WorkflowGraphPhaseChanges{Added: added, Removed: removed, Modified: modified}
}

func diffWorkflowGraphEdges(from []WorkflowGraphEdge, to []WorkflowGraphEdge) WorkflowGraphEdgeChanges {
	added, removed, modified := diffWorkflowGraphEntities(
		from,
		to,
		workflowGraphEdgeKey,
		cleanWorkflowGraphEdge,
		func(edge WorkflowGraphEdge) any { return edge },
		func(key string, before WorkflowGraphEdge, after WorkflowGraphEdge) WorkflowGraphEdgeChange {
			return WorkflowGraphEdgeChange{Key: key, Before: before, After: after}
		},
	)
	return WorkflowGraphEdgeChanges{Added: added, Removed: removed, Modified: modified}
}

func diffWorkflowGraphGates(from []WorkflowGraphGate, to []WorkflowGraphGate) WorkflowGraphGateChanges {
	added, removed, modified := diffWorkflowGraphEntities(
		from,
		to,
		func(gate WorkflowGraphGate) string { return gate.ID },
		cleanWorkflowGraphGate,
		func(gate WorkflowGraphGate) any { return canonicalWorkflowGraphGate(gate) },
		func(key string, before WorkflowGraphGate, after WorkflowGraphGate) WorkflowGraphGateChange {
			return WorkflowGraphGateChange{Key: key, Before: before, After: after}
		},
	)
	return WorkflowGraphGateChanges{Added: added, Removed: removed, Modified: modified}
}

func diffWorkflowGraphApprovals(from []WorkflowGraphApproval, to []WorkflowGraphApproval) WorkflowGraphApprovalChanges {
	added, removed, modified := diffWorkflowGraphEntities(
		from,
		to,
		func(approval WorkflowGraphApproval) string { return approval.Scope },
		cleanWorkflowGraphApproval,
		func(approval WorkflowGraphApproval) any { return approval },
		func(key string, before WorkflowGraphApproval, after WorkflowGraphApproval) WorkflowGraphApprovalChange {
			return WorkflowGraphApprovalChange{Key: key, Before: before, After: after}
		},
	)
	return WorkflowGraphApprovalChanges{Added: added, Removed: removed, Modified: modified}
}

func diffWorkflowGraphFeedbackIntake(from *WorkflowGraphFeedbackIntake, to *WorkflowGraphFeedbackIntake) WorkflowGraphFeedbackIntakeChange {
	before := cleanWorkflowGraphFeedbackIntakePtr(from)
	after := cleanWorkflowGraphFeedbackIntakePtr(to)
	if reflect.DeepEqual(before, after) {
		return WorkflowGraphFeedbackIntakeChange{}
	}
	return WorkflowGraphFeedbackIntakeChange{Changed: true, Before: before, After: after}
}

func diffWorkflowGraphEntities[T any, C any](
	from []T,
	to []T,
	keyFor func(T) string,
	clean func(T) T,
	canonical func(T) any,
	changeFor func(string, T, T) C,
) ([]T, []T, []C) {
	added := []T{}
	removed := []T{}
	modified := []C{}
	fromByKey := map[string]T{}
	toByKey := map[string]T{}
	keys := map[string]bool{}
	for _, item := range from {
		cleaned := clean(item)
		key := keyFor(cleaned)
		fromByKey[key] = cleaned
		keys[key] = true
	}
	for _, item := range to {
		cleaned := clean(item)
		key := keyFor(cleaned)
		toByKey[key] = cleaned
		keys[key] = true
	}
	for _, key := range sortedGraphKeys(keys) {
		before, hadBefore := fromByKey[key]
		after, hasAfter := toByKey[key]
		switch {
		case !hadBefore && hasAfter:
			added = append(added, after)
		case hadBefore && !hasAfter:
			removed = append(removed, before)
		case hadBefore && hasAfter && !reflect.DeepEqual(canonical(before), canonical(after)):
			modified = append(modified, changeFor(key, before, after))
		}
	}
	return added, removed, modified
}

func workflowGraphRiskFlags(from WorkflowGraph, to WorkflowGraph, diff GraphDiffResult) []string {
	flags := map[string]bool{}
	if from.GraphID != to.GraphID || from.Version != to.Version {
		flags["graph_identity_changed"] = true
	}
	if !reflect.DeepEqual(from.Metadata, to.Metadata) {
		flags["metadata_changed"] = true
	}
	if len(diff.ChangedPhases.Removed) > 0 {
		flags["phase_removed"] = true
	}
	for _, change := range diff.ChangedPhases.Modified {
		if change.Before.Required != change.After.Required {
			flags["phase_required_changed"] = true
			break
		}
	}
	if len(diff.ChangedEdges.Added) > 0 || len(diff.ChangedEdges.Removed) > 0 || len(diff.ChangedEdges.Modified) > 0 {
		flags["dependencies_changed"] = true
	}
	if len(diff.ChangedGates.Added) > 0 || len(diff.ChangedGates.Removed) > 0 || len(diff.ChangedGates.Modified) > 0 {
		flags["gates_changed"] = true
	}
	if len(diff.ChangedApprovals.Added) > 0 || len(diff.ChangedApprovals.Removed) > 0 || len(diff.ChangedApprovals.Modified) > 0 {
		flags["approvals_changed"] = true
	}
	if diff.ChangedFeedbackIntake.Changed {
		flags["feedback_intake_changed"] = true
	}
	return sortedGraphKeys(flags)
}

func cleanWorkflowGraphPhase(phase WorkflowGraphPhase) WorkflowGraphPhase {
	phase.requiredSet = false
	phase.seenFields = nil
	if phase.Evidence == nil {
		phase.Evidence = []string{}
	}
	return phase
}

func canonicalWorkflowGraphPhase(phase WorkflowGraphPhase) WorkflowGraphPhase {
	phase = cleanWorkflowGraphPhase(phase)
	phase.Evidence = sortedStrings(phase.Evidence)
	return phase
}

func cleanWorkflowGraphEdge(edge WorkflowGraphEdge) WorkflowGraphEdge {
	edge.seenFields = nil
	return edge
}

func workflowGraphEdgeKey(edge WorkflowGraphEdge) string {
	return edge.From + " -> " + edge.To
}

func cleanWorkflowGraphGate(gate WorkflowGraphGate) WorkflowGraphGate {
	gate.seenFields = nil
	if gate.Requires == nil {
		gate.Requires = []string{}
	}
	if gate.Checks == nil {
		gate.Checks = []WorkflowGraphCheck{}
	}
	for i := range gate.Checks {
		gate.Checks[i] = cleanWorkflowGraphCheck(gate.Checks[i])
	}
	return gate
}

func cleanWorkflowGraphCheck(check WorkflowGraphCheck) WorkflowGraphCheck {
	check.seenFields = nil
	if check.OneOf == nil {
		check.OneOf = []string{}
	}
	if check.Tokens == nil {
		check.Tokens = []string{}
	}
	return check
}

func canonicalWorkflowGraphGate(gate WorkflowGraphGate) WorkflowGraphGate {
	gate = cleanWorkflowGraphGate(gate)
	gate.Requires = sortedStrings(gate.Requires)
	return gate
}

func cleanWorkflowGraphApproval(approval WorkflowGraphApproval) WorkflowGraphApproval {
	approval.seenFields = nil
	return approval
}

func cleanWorkflowGraphFeedbackIntakePtr(feedback *WorkflowGraphFeedbackIntake) *WorkflowGraphFeedbackIntake {
	if feedback == nil {
		return nil
	}
	cleaned := cleanWorkflowGraphFeedbackIntake(*feedback)
	return &cleaned
}

func cleanWorkflowGraphFeedbackIntake(feedback WorkflowGraphFeedbackIntake) WorkflowGraphFeedbackIntake {
	feedback.policySet = false
	feedback.schemaVersionSet = false
	feedback.minRoundsSet = false
	feedback.maxRoundsSet = false
	feedback.requiredRoundsSet = false
	feedback.optionalRoundsSet = false
	if feedback.RequiredRounds == nil {
		feedback.RequiredRounds = []int{}
	}
	if feedback.OptionalRounds == nil {
		feedback.OptionalRounds = []int{}
	}
	return feedback
}

func sortedStrings(values []string) []string {
	result := append([]string{}, values...)
	sort.Strings(result)
	return result
}

func sortedGraphKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func supportedGraphExportFormat(format string) bool {
	return format == graphExportFormatMermaid || format == graphExportFormatPlantUML
}

func resolveGraphExportOutput(root Root, output string, format string) (SafePath, error) {
	path, err := ResolveRelativePath(root, output)
	if err != nil {
		return SafePath{}, err
	}
	rel := filepath.ToSlash(path.Relative)
	path.Relative = rel
	if rel == WorkflowGraphDefaultPath || strings.HasSuffix(strings.ToLower(rel), ".yaml") || strings.HasSuffix(strings.ToLower(rel), ".yml") {
		return SafePath{}, &Problem{Code: "graph_export_output_invalid", Message: "graph export output must be a generated diagram artifact", Hint: "Use a Mermaid or PlantUML diagram path; exported diagrams are never workflow graph source of truth.", Path: rel, Field: "output", Expected: "generated diagram path", Actual: rel}
	}
	if !graphExportOutputExtensionMatches(rel, format) {
		return SafePath{}, &Problem{Code: "graph_export_output_invalid", Message: "graph export output extension does not match the requested format", Hint: "Use .mmd or .mermaid for Mermaid and .puml or .plantuml for PlantUML.", Path: rel, Field: "output", Expected: graphExportExpectedExtensions(format), Actual: filepath.Ext(rel)}
	}
	info, err := os.Lstat(path.Absolute)
	if os.IsNotExist(err) {
		return path, nil
	}
	if err != nil {
		return SafePath{}, &Problem{Code: "graph_export_output_inspection_failed", Message: "cannot inspect graph export output path", Hint: "Check repository permissions before exporting graph diagrams.", Path: rel, Field: "output", Expected: "inspectable output path", Actual: err.Error()}
	}
	if info.IsDir() {
		return SafePath{}, &Problem{Code: "graph_export_output_invalid", Message: "graph export output path must be a file", Hint: "Choose a generated diagram file path, not a directory.", Path: rel, Field: "output", Expected: "file path", Actual: "directory"}
	}
	return SafePath{}, &Problem{Code: "graph_export_output_exists", Message: "graph export output path already exists", Hint: "Choose a new generated diagram path or remove the existing artifact before exporting.", Path: rel, Field: "output", Expected: "absent output path", Actual: "exists"}
}

func graphExportOutputExtensionMatches(path string, format string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch format {
	case graphExportFormatMermaid:
		return ext == ".mmd" || ext == ".mermaid"
	case graphExportFormatPlantUML:
		return ext == ".puml" || ext == ".plantuml"
	default:
		return false
	}
}

func graphExportExpectedExtensions(format string) string {
	if format == graphExportFormatMermaid {
		return ".mmd or .mermaid"
	}
	return ".puml or .plantuml"
}

func renderWorkflowGraphMermaid(graph WorkflowGraph) string {
	var b strings.Builder
	b.WriteString("flowchart TD\n")
	phaseIDs := make(map[string]string, len(graph.Phases))
	for i, phase := range graph.Phases {
		nodeID := diagramNodeID("p", i+1, phase.ID)
		phaseIDs[phase.ID] = nodeID
		fmt.Fprintf(&b, "  %s[\"%s\"]\n", nodeID, mermaidLabel(phaseLabel(phase)))
	}
	for _, edge := range graph.Edges {
		from := phaseIDs[edge.From]
		to := phaseIDs[edge.To]
		if from != "" && to != "" {
			fmt.Fprintf(&b, "  %s --> %s\n", from, to)
		}
	}
	for i, gate := range normalizeWorkflowGraphGates(graph.Gates) {
		nodeID := diagramNodeID("g", i+1, gate.ID)
		fmt.Fprintf(&b, "  %s[\"%s\"]\n", nodeID, mermaidLabel("gate: "+gate.ID, "requires: "+strings.Join(gate.Requires, ", ")))
		for _, required := range gate.Requires {
			if phaseNode := phaseIDs[required]; phaseNode != "" {
				fmt.Fprintf(&b, "  %s -. requires .-> %s\n", nodeID, phaseNode)
			}
		}
	}
	for i, approval := range graph.Approvals {
		nodeID := diagramNodeID("a", i+1, approval.Scope)
		fmt.Fprintf(&b, "  %s[\"%s\"]\n", nodeID, mermaidLabel("approval: "+approval.Scope, "role: "+approval.RequiredRole))
		if phaseNode := phaseIDs[approval.Scope]; phaseNode != "" {
			fmt.Fprintf(&b, "  %s -. approval .-> %s\n", nodeID, phaseNode)
		}
	}
	return b.String()
}

func renderWorkflowGraphPlantUML(graph WorkflowGraph) string {
	var b strings.Builder
	b.WriteString("@startuml\n")
	phaseIDs := make(map[string]string, len(graph.Phases))
	for i, phase := range graph.Phases {
		nodeID := diagramNodeID("p", i+1, phase.ID)
		phaseIDs[phase.ID] = nodeID
		fmt.Fprintf(&b, "rectangle \"%s\" as %s\n", plantUMLText(phaseLabel(phase)), nodeID)
	}
	for _, edge := range graph.Edges {
		from := phaseIDs[edge.From]
		to := phaseIDs[edge.To]
		if from != "" && to != "" {
			fmt.Fprintf(&b, "%s --> %s\n", from, to)
		}
	}
	for i, gate := range normalizeWorkflowGraphGates(graph.Gates) {
		nodeID := diagramNodeID("g", i+1, gate.ID)
		fmt.Fprintf(&b, "note \"%s\" as %s\n", plantUMLText("gate: "+gate.ID+"\nrequires: "+strings.Join(gate.Requires, ", ")), nodeID)
		for _, required := range gate.Requires {
			if phaseNode := phaseIDs[required]; phaseNode != "" {
				fmt.Fprintf(&b, "%s .. %s\n", nodeID, phaseNode)
			}
		}
	}
	for i, approval := range graph.Approvals {
		nodeID := diagramNodeID("a", i+1, approval.Scope)
		fmt.Fprintf(&b, "note \"%s\" as %s\n", plantUMLText("approval: "+approval.Scope+"\nrole: "+approval.RequiredRole), nodeID)
		if phaseNode := phaseIDs[approval.Scope]; phaseNode != "" {
			fmt.Fprintf(&b, "%s .. %s\n", nodeID, phaseNode)
		}
	}
	b.WriteString("@enduml\n")
	return b.String()
}

func phaseLabel(phase WorkflowGraphPhase) string {
	if strings.TrimSpace(phase.Title) == "" {
		return phase.ID
	}
	return phase.Title + " [" + phase.ID + "]"
}

func diagramNodeID(prefix string, index int, key string) string {
	return fmt.Sprintf("%s%03d_%s", prefix, index, sanitizeDiagramIdentifier(key))
}

func sanitizeDiagramIdentifier(value string) string {
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	result := strings.Trim(b.String(), "_")
	if result == "" {
		return "item"
	}
	return result
}

func mermaidLabel(parts ...string) string {
	escaped := make([]string, 0, len(parts))
	for _, part := range parts {
		escaped = append(escaped, strings.NewReplacer(`\`, `\\`, "&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, `'`, "\r", " ", "\n", " ").Replace(part))
	}
	return strings.Join(escaped, "<br/>")
}

func plantUMLText(value string) string {
	return strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\r\n", `\n`, "\n", `\n`, "\r", `\n`).Replace(value)
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
			Name:     graphIssueGraphFile,
			Path:     path.Relative,
			Message:  "workflow graph file is missing",
			Hint:     "Create .kkachi-workflow.yaml through an approved graph init/apply flow before relying on graph support.",
			Field:    "file",
			Expected: "existing workflow graph file",
			Actual:   graphIssueActualMissing,
		}})}
	}
	if err != nil {
		return loadedWorkflowGraph{validation: failedGraphValidation(path.Relative, []GraphIssue{{
			Name:     graphIssueGraphFile,
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
			FeedbackIntake:  cleanWorkflowGraphFeedbackIntakePtr(doc.graph.FeedbackIntake),
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
	doc := graphDocument{metadataFields: map[string]bool{}, proposalFields: map[string]bool{}, feedbackIntakeFields: map[string]bool{}}
	lines := strings.Split(string(data), "\n")
	section := ""
	seenSections := map[string]bool{}
	topLevelFields := map[string]bool{}
	var phase *WorkflowGraphPhase
	var edge *WorkflowGraphEdge
	var gate *WorkflowGraphGate
	var check *WorkflowGraphCheck
	var approval *WorkflowGraphApproval
	flushCheck := func() {
		if check != nil {
			if gate == nil {
				doc.errors = append(doc.errors, GraphIssue{Name: "graph_yaml", Path: path, Message: "workflow graph check appears outside a gate row", Field: "checks", Expected: "check below gates list item", Actual: check.Type})
			} else {
				item := *check
				item.seenFields = nil
				gate.Checks = append(gate.Checks, item)
			}
			check = nil
		}
	}
	flush := func() {
		flushCheck()
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
				addParseError(lineNo, key, "workflow graph contains an unsupported top-level field", "version, graph_id, metadata, phases, edges, gates, approvals, proposals, or feedback_intake", key)
			}
			continue
		}
		if section == "" {
			addParseError(lineNo, "yaml", "workflow graph field appears before a section", "top-level section", line)
			continue
		}
		if strings.HasPrefix(line, "- ") {
			item := strings.TrimSpace(strings.TrimPrefix(line, "- "))
			if section == "gates" && indent > 2 {
				if gate == nil {
					addParseError(lineNo, "checks", "workflow graph check appears outside a gate row", "check below gates list item", line)
					continue
				}
				flushCheck()
				check = &WorkflowGraphCheck{seenFields: map[string]bool{}}
				setGraphCheckField(&doc, lineNo, item, check)
				continue
			}
			flush()
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
		if section == "metadata" || section == "proposals" || section == "feedback_intake" {
			setGraphMappingField(&doc, section, lineNo, line)
			continue
		}
		if section == "gates" && check != nil && indent > 4 {
			setGraphCheckField(&doc, lineNo, line, check)
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
	case "metadata", "phases", "edges", "gates", "approvals", "proposals", "feedback_intake":
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
	case "feedback_intake":
		if doc.graph.FeedbackIntake == nil {
			doc.graph.FeedbackIntake = &WorkflowGraphFeedbackIntake{}
		}
		switch key {
		case "policy":
			if !markGraphField(doc, doc.feedbackIntakeFields, lineNo, key) {
				return
			}
			doc.graph.FeedbackIntake.Policy = parsed
			doc.graph.FeedbackIntake.policySet = true
		case "schema_version":
			if !markGraphField(doc, doc.feedbackIntakeFields, lineNo, key) {
				return
			}
			doc.graph.FeedbackIntake.SchemaVersion = parsed
			doc.graph.FeedbackIntake.schemaVersionSet = true
		case "min_rounds":
			if !markGraphField(doc, doc.feedbackIntakeFields, lineNo, key) {
				return
			}
			round, err := parseYAMLInt(value)
			if err != nil {
				doc.errors = append(doc.errors, GraphIssue{Name: "graph_yaml", Message: "feedback_intake min_rounds field is invalid", Field: key, Expected: "integer scalar", Actual: err.Error(), Line: lineNo})
				return
			}
			doc.graph.FeedbackIntake.MinRounds = round
			doc.graph.FeedbackIntake.minRoundsSet = true
		case "max_rounds":
			if !markGraphField(doc, doc.feedbackIntakeFields, lineNo, key) {
				return
			}
			round, err := parseYAMLInt(value)
			if err != nil {
				doc.errors = append(doc.errors, GraphIssue{Name: "graph_yaml", Message: "feedback_intake max_rounds field is invalid", Field: key, Expected: "integer scalar", Actual: err.Error(), Line: lineNo})
				return
			}
			doc.graph.FeedbackIntake.MaxRounds = round
			doc.graph.FeedbackIntake.maxRoundsSet = true
		case "required_rounds":
			if !markGraphField(doc, doc.feedbackIntakeFields, lineNo, key) {
				return
			}
			rounds, err := parseYAMLIntList(value)
			if err != nil {
				doc.errors = append(doc.errors, GraphIssue{Name: "graph_yaml", Message: "feedback_intake required_rounds list is invalid", Field: key, Expected: "inline integer list", Actual: err.Error(), Line: lineNo})
				return
			}
			doc.graph.FeedbackIntake.RequiredRounds = rounds
			doc.graph.FeedbackIntake.requiredRoundsSet = true
		case "optional_rounds":
			if !markGraphField(doc, doc.feedbackIntakeFields, lineNo, key) {
				return
			}
			rounds, err := parseYAMLIntList(value)
			if err != nil {
				doc.errors = append(doc.errors, GraphIssue{Name: "graph_yaml", Message: "feedback_intake optional_rounds list is invalid", Field: key, Expected: "inline integer list", Actual: err.Error(), Line: lineNo})
				return
			}
			doc.graph.FeedbackIntake.OptionalRounds = rounds
			doc.graph.FeedbackIntake.optionalRoundsSet = true
		default:
			doc.errors = append(doc.errors, GraphIssue{Name: "graph_yaml", Message: "workflow graph feedback_intake field is unsupported", Field: key, Expected: "policy, schema_version, min_rounds, max_rounds, required_rounds, or optional_rounds", Actual: key, Line: lineNo})
		}
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
		case "final_required":
			value, ok := parseYAMLBool(parsed)
			if !ok {
				doc.errors = append(doc.errors, GraphIssue{Name: "graph_yaml", Message: "gate final_required field is invalid", Field: key, Expected: "true or false", Actual: parsed, Line: lineNo})
				return
			}
			gate.FinalRequired = value
		case "checks":
			if strings.TrimSpace(value) != "" {
				doc.errors = append(doc.errors, GraphIssue{Name: "graph_yaml", Message: "gate checks field must be a nested list", Field: key, Expected: "checks: followed by nested check rows", Actual: value, Line: lineNo})
			}
		default:
			doc.errors = append(doc.errors, GraphIssue{Name: "graph_yaml", Message: "gate field is unsupported", Field: key, Expected: "id, requires, final_required, or checks", Actual: key, Line: lineNo})
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

func setGraphCheckField(doc *graphDocument, lineNo int, line string, check *WorkflowGraphCheck) {
	key, value, ok := strings.Cut(line, ":")
	if !ok {
		doc.errors = append(doc.errors, GraphIssue{Name: "graph_yaml", Message: "workflow graph check line is invalid", Field: "checks", Expected: "key: value line", Actual: line, Line: lineNo})
		return
	}
	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)
	parsed, err := parseWorkflowGraphScalar(value)
	if err != nil && !strings.HasPrefix(value, "[") {
		doc.errors = append(doc.errors, GraphIssue{Name: "graph_yaml", Message: "workflow graph check scalar is invalid", Field: key, Expected: "string scalar", Actual: err.Error(), Line: lineNo})
		return
	}
	if check == nil {
		doc.errors = append(doc.errors, GraphIssue{Name: "graph_yaml", Message: "check field appears outside a check row", Field: key, Expected: "field below checks list item", Actual: fmt.Sprintf("line %d", lineNo), Line: lineNo})
		return
	}
	if !markGraphField(doc, check.seenFields, lineNo, key) {
		return
	}
	switch key {
	case "type":
		check.Type = parsed
	case "name":
		check.Name = parsed
	case "path":
		check.Path = parsed
	case "field":
		check.Field = parsed
	case "equals":
		check.Equals = parsed
	case "one_of":
		items, err := parseYAMLStringList(value)
		if err != nil {
			doc.errors = append(doc.errors, GraphIssue{Name: "graph_yaml", Message: "check one_of list is invalid", Field: key, Expected: "inline string list", Actual: err.Error(), Line: lineNo})
			return
		}
		check.OneOf = items
	case "token":
		check.Token = parsed
	case "tokens":
		items, err := parseYAMLStringList(value)
		if err != nil {
			doc.errors = append(doc.errors, GraphIssue{Name: "graph_yaml", Message: "check tokens list is invalid", Field: key, Expected: "inline string list", Actual: err.Error(), Line: lineNo})
			return
		}
		check.Tokens = items
	case "phase":
		check.Phase = parsed
	case "status":
		check.Status = parsed
	case "message":
		check.Message = parsed
	case "hint":
		check.Hint = parsed
	default:
		doc.errors = append(doc.errors, GraphIssue{Name: "graph_yaml", Message: "workflow graph check field is unsupported", Field: key, Expected: "type, name, path, field, equals, one_of, token, tokens, phase, status, message, or hint", Actual: key, Line: lineNo})
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
	return parseYAMLInlineList(value, func(part string) (string, error) {
		item, err := parseWorkflowGraphScalar(strings.TrimSpace(part))
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(item) == "" {
			return "", fmt.Errorf("empty list item")
		}
		return item, nil
	})
}

func parseYAMLInt(value string) (int, error) {
	parsed, err := parseWorkflowGraphScalar(value)
	if err != nil {
		return 0, err
	}
	round, err := strconv.Atoi(strings.TrimSpace(parsed))
	if err != nil {
		return 0, fmt.Errorf("not an integer")
	}
	return round, nil
}

func parseYAMLIntList(value string) ([]int, error) {
	return parseYAMLInlineList(value, parseYAMLInt)
}

func parseYAMLInlineList[T any](value string, parseItem func(string) (T, error)) ([]T, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	if !strings.HasPrefix(value, "[") || !strings.HasSuffix(value, "]") {
		return nil, fmt.Errorf("not an inline list")
	}
	inner := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(value, "["), "]"))
	if inner == "" {
		return []T{}, nil
	}
	parts := splitInlineList(inner)
	result := make([]T, 0, len(parts))
	for _, part := range parts {
		item, err := parseItem(strings.TrimSpace(part))
		if err != nil {
			return nil, err
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
		for _, check := range gate.Checks {
			validateWorkflowGraphCheck(add, gate.ID, check, phaseIDs)
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
	errors = append(errors, validateWorkflowGraphFeedbackIntake(graph.FeedbackIntake, path)...)
	return errors
}

func validateWorkflowGraphCheck(add func(string, string, string, string, string), gateID string, check WorkflowGraphCheck, phaseIDs map[string]bool) {
	checkType := strings.TrimSpace(check.Type)
	fieldPrefix := "gates[" + gateID + "].checks[]"
	if checkType == "" {
		add("gate_check_type", fieldPrefix+".type", "workflow gate check type is required", workflowGraphCheckTypes(), "missing")
		return
	}
	switch checkType {
	case "artifact.exists":
		if strings.TrimSpace(check.Path) == "" {
			add("gate_check_path", fieldPrefix+".path", "artifact.exists check requires path", "non-empty artifact path", "missing")
		}
	case "markdown.field":
		if strings.TrimSpace(check.Path) == "" {
			add("gate_check_path", fieldPrefix+".path", "markdown.field check requires path", "non-empty artifact path", "missing")
		}
		if strings.TrimSpace(check.Field) == "" {
			add("gate_check_field", fieldPrefix+".field", "markdown.field check requires field", "non-empty markdown field", "missing")
		}
		if strings.TrimSpace(check.Equals) == "" && len(check.OneOf) == 0 {
			add("gate_check_expected", fieldPrefix+".equals", "markdown.field check requires equals or one_of", "equals or one_of", "missing")
		}
		if strings.TrimSpace(check.Equals) != "" && len(check.OneOf) > 0 {
			add("gate_check_expected", fieldPrefix+".equals", "markdown.field check must not mix equals and one_of", "equals or one_of", "both")
		}
	case "text.contains":
		if strings.TrimSpace(check.Path) == "" {
			add("gate_check_path", fieldPrefix+".path", "text.contains check requires path", "non-empty artifact path", "missing")
		}
		if strings.TrimSpace(check.Token) == "" {
			add("gate_check_token", fieldPrefix+".token", "text.contains check requires token", "non-empty token", "missing")
		}
	case "text.contains_all":
		if strings.TrimSpace(check.Path) == "" {
			add("gate_check_path", fieldPrefix+".path", "text.contains_all check requires path", "non-empty artifact path", "missing")
		}
		if len(check.Tokens) == 0 {
			add("gate_check_tokens", fieldPrefix+".tokens", "text.contains_all check requires tokens", "one or more tokens", "missing")
		} else if hasBlankString(check.Tokens) {
			add("gate_check_tokens", fieldPrefix+".tokens", "text.contains_all check tokens must be non-empty", "non-empty tokens", "blank")
		}
	case "gitignore.contains_all":
		if len(check.Tokens) == 0 {
			add("gate_check_tokens", fieldPrefix+".tokens", "gitignore.contains_all check requires tokens", "one or more .gitignore entries", "missing")
		} else if hasBlankString(check.Tokens) {
			add("gate_check_tokens", fieldPrefix+".tokens", "gitignore.contains_all check tokens must be non-empty", "non-empty .gitignore entries", "blank")
		}
	case "codegraph.evidence":
		if strings.TrimSpace(check.Equals) != "" {
			add("gate_check_expected", fieldPrefix+".equals", "codegraph.evidence check does not support equals", "one_of statuses and/or tokens", "equals")
		}
		if hasBlankString(check.OneOf) {
			add("gate_check_status", fieldPrefix+".one_of", "codegraph.evidence one_of statuses must be non-empty", "non-empty statuses", "blank")
		}
		if hasBlankString(check.Tokens) {
			add("gate_check_tokens", fieldPrefix+".tokens", "codegraph.evidence marker tokens must be non-empty", "non-empty marker tokens", "blank")
		}
	case "phase.status":
		if strings.TrimSpace(check.Phase) == "" {
			add("gate_check_phase", fieldPrefix+".phase", "phase.status check requires phase", "declared phase id", "missing")
		} else if !phaseIDs[check.Phase] {
			add("gate_check_phase", fieldPrefix+".phase", "phase.status check phase is not declared", "declared phase id", check.Phase)
		}
		if !knownPhaseStatus(check.Status) {
			actual := check.Status
			if actual == "" {
				actual = "missing"
			}
			add("gate_check_status", fieldPrefix+".status", "phase.status check status is invalid", strings.Join(phaseStatuses, ","), actual)
		}
	default:
		add("gate_check_type", fieldPrefix+".type", "workflow gate check type is unsupported", workflowGraphCheckTypes(), checkType)
	}
}

func workflowGraphCheckTypes() string {
	return "artifact.exists,markdown.field,text.contains,text.contains_all,gitignore.contains_all,codegraph.evidence,phase.status"
}

func hasBlankString(values []string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return true
		}
	}
	return false
}

func validateWorkflowGraphFeedbackIntake(feedback *WorkflowGraphFeedbackIntake, path string) []GraphIssue {
	if feedback == nil {
		return nil
	}
	errors := []GraphIssue{}
	add := func(name string, field string, message string, expected string, actual string) {
		errors = append(errors, GraphIssue{Name: name, Path: path, Message: message, Hint: "Repair feedback_intake so KAH can validate external feedback intake bounds deterministically.", Field: field, Expected: expected, Actual: actual})
	}
	if !feedback.policySet {
		add("feedback_intake_policy", "feedback_intake.policy", "feedback intake policy is required", graphFeedbackIntakePolicy, graphIssueActualMissing)
	} else if feedback.Policy != graphFeedbackIntakePolicy {
		add("feedback_intake_policy", "feedback_intake.policy", "feedback intake policy is unsupported", graphFeedbackIntakePolicy, feedback.Policy)
	}
	if !feedback.schemaVersionSet {
		add("feedback_intake_schema_version", "feedback_intake.schema_version", "feedback intake schema version is required", graphFeedbackIntakeSchema, graphIssueActualMissing)
	} else if feedback.SchemaVersion != graphFeedbackIntakeSchema {
		add("feedback_intake_schema_version", "feedback_intake.schema_version", "feedback intake schema version is unsupported", graphFeedbackIntakeSchema, feedback.SchemaVersion)
	}
	if !feedback.minRoundsSet {
		add("feedback_intake_min_rounds", "feedback_intake.min_rounds", "feedback intake min_rounds is required", "1", graphIssueActualMissing)
	} else if feedback.MinRounds != 1 {
		add("feedback_intake_min_rounds", "feedback_intake.min_rounds", "feedback intake min_rounds is unsupported", "1", strconv.Itoa(feedback.MinRounds))
	}
	if !feedback.maxRoundsSet {
		add("feedback_intake_max_rounds", "feedback_intake.max_rounds", "feedback intake max_rounds is required", "5", graphIssueActualMissing)
	} else if feedback.MaxRounds != 5 {
		name := "feedback_intake_max_rounds"
		message := "feedback intake max_rounds is unsupported"
		if feedback.MaxRounds == 3 {
			name = "feedback_intake_stale_bounds"
			message = "feedback intake max_rounds preserves stale fixed max3 bounds"
		}
		add(name, "feedback_intake.max_rounds", message, "5", strconv.Itoa(feedback.MaxRounds))
	}
	if feedback.minRoundsSet && feedback.maxRoundsSet && feedback.MaxRounds < feedback.MinRounds {
		add("feedback_intake_round_bounds", "feedback_intake.max_rounds", "feedback intake max_rounds must be greater than or equal to min_rounds", "max_rounds >= min_rounds", fmt.Sprintf("%d < %d", feedback.MaxRounds, feedback.MinRounds))
	}
	if !feedback.requiredRoundsSet {
		add("feedback_intake_required_rounds", "feedback_intake.required_rounds", "feedback intake required_rounds is required", "[1]", graphIssueActualMissing)
	} else {
		validateWorkflowGraphFeedbackRounds(feedback.RequiredRounds, "feedback_intake.required_rounds", true, add)
		if !reflect.DeepEqual(feedback.RequiredRounds, []int{1}) {
			add("feedback_intake_required_rounds", "feedback_intake.required_rounds", "feedback intake required rounds must declare round 1 only", "[1]", graphYAMLIntList(feedback.RequiredRounds))
		}
	}
	if !feedback.optionalRoundsSet {
		add("feedback_intake_optional_rounds", "feedback_intake.optional_rounds", "feedback intake optional_rounds is required", "[2,3,4,5]", graphIssueActualMissing)
	} else {
		validateWorkflowGraphFeedbackRounds(feedback.OptionalRounds, "feedback_intake.optional_rounds", false, add)
		if reflect.DeepEqual(feedback.OptionalRounds, []int{2, 3}) {
			add("feedback_intake_stale_bounds", "feedback_intake.optional_rounds", "feedback intake optional_rounds preserves stale fixed 1..3 bounds", "[2,3,4,5]", graphYAMLIntList(feedback.OptionalRounds))
		} else if !reflect.DeepEqual(feedback.OptionalRounds, []int{2, 3, 4, 5}) {
			add("feedback_intake_optional_rounds", "feedback_intake.optional_rounds", "feedback intake optional rounds must declare continuation rounds 2 through 5", "[2,3,4,5]", graphYAMLIntList(feedback.OptionalRounds))
		}
	}
	if feedback.requiredRoundsSet && feedback.optionalRoundsSet {
		overlap := graphRoundOverlap(feedback.RequiredRounds, feedback.OptionalRounds)
		if len(overlap) > 0 {
			add("feedback_intake_rounds_conflict", "feedback_intake.required_rounds", "feedback intake rounds cannot be both required and optional", "disjoint required and optional rounds", graphYAMLIntList(overlap))
		}
	}
	return errors
}

func validateWorkflowGraphFeedbackRounds(rounds []int, field string, required bool, add func(string, string, string, string, string)) {
	seen := map[int]bool{}
	duplicates := []int{}
	invalid := []int{}
	for _, round := range rounds {
		if seen[round] {
			duplicates = append(duplicates, round)
		}
		seen[round] = true
		if round < 1 || round >= 6 {
			invalid = append(invalid, round)
		}
	}
	if len(duplicates) > 0 {
		sort.Ints(duplicates)
		add("feedback_intake_duplicate_round", field, "feedback intake round declarations must be unique", "unique rounds", graphYAMLIntList(uniqueInts(duplicates)))
	}
	if len(invalid) > 0 {
		sort.Ints(invalid)
		name := "feedback_intake_round_range"
		if required {
			name = "feedback_intake_required_rounds"
		}
		add(name, field, "feedback intake round is outside the supported range", "rounds 1 through 5", graphYAMLIntList(uniqueInts(invalid)))
	}
}

func graphRoundOverlap(left []int, right []int) []int {
	rightSet := map[int]bool{}
	for _, value := range right {
		rightSet[value] = true
	}
	overlap := []int{}
	for _, value := range left {
		if rightSet[value] {
			overlap = append(overlap, value)
		}
	}
	sort.Ints(overlap)
	return uniqueInts(overlap)
}

func uniqueInts(values []int) []int {
	result := []int{}
	seen := map[int]bool{}
	for _, value := range values {
		if seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
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

func nextGraphProposalPath(root Root) (string, SafePath, error) {
	dir, err := ResolveRelativePath(root, graphProposalDir)
	if err != nil {
		return "", SafePath{}, err
	}
	entries, err := os.ReadDir(dir.Absolute)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", SafePath{}, &Problem{Code: "graph_proposal_inspection_failed", Message: "cannot inspect graph proposal directory", Hint: "Check .kkachi/graph/proposals permissions before recording a proposal.", Path: dir.Relative, Field: "path", Expected: "inspectable proposal directory", Actual: err.Error()}
	}
	next := 1
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		matches := graphProposalIDPattern.FindStringSubmatch(entry.Name())
		if matches == nil {
			continue
		}
		value, err := strconv.Atoi(matches[1])
		if err == nil && value >= next {
			next = value + 1
		}
	}
	if next > 999999 {
		return "", SafePath{}, &Problem{Code: "graph_proposal_id_exhausted", Message: "cannot allocate next graph proposal id", Hint: "Archive old proposal records through a future migration before recording more proposals.", Path: dir.Relative, Field: "proposal_id", Expected: "available gprop id below gprop-999999", Actual: "exhausted"}
	}
	proposalID := fmt.Sprintf("gprop-%06d", next)
	path, err := ResolveRelativePath(root, filepath.ToSlash(filepath.Join(graphProposalDir, proposalID+".json")))
	if err != nil {
		return "", SafePath{}, err
	}
	return proposalID, path, nil
}

func graphValidationProblem(code string, message string, hint string, summary GraphDiffValidationSummary) *Problem {
	actual := "invalid graph input"
	if summary.From.Status != GraphStatusPass {
		actual = "from:" + summary.From.File
	}
	if summary.To.Status != GraphStatusPass {
		if actual == "invalid graph input" {
			actual = "to:" + summary.To.File
		} else {
			actual += ",to:" + summary.To.File
		}
	}
	return &Problem{Code: code, Message: message, Hint: hint, Field: "validation", Expected: "passing base and candidate workflow graphs", Actual: actual}
}

func graphTemplateValidationProblem(validation GraphValidationResult) *Problem {
	actual := "invalid template"
	if len(validation.Errors) > 0 {
		actual = validation.Errors[0].Name + ":" + validation.File
	}
	return &Problem{Code: "graph_template_invalid", Message: "graph init template is invalid", Hint: "Repair the workflow graph template, then rerun graph init.", Path: validation.File, Field: "from_template", Expected: "valid workflow graph template", Actual: actual}
}

func graphInitValidationProblem(validation GraphValidationResult) *Problem {
	actual := "invalid rendered graph"
	if len(validation.Errors) > 0 {
		actual = validation.Errors[0].Name + ":" + validation.File
	}
	return &Problem{Code: "graph_init_invalid", Message: "rendered workflow graph is invalid", Hint: "Inspect the selected template and preserve stderr for diagnosis.", Path: validation.File, Field: "graph", Expected: "valid initialized workflow graph", Actual: actual}
}
