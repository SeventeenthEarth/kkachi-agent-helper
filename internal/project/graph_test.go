package project

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestValidateAndExplainWorkflowGraph(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	writeWorkflowGraph(t, repo, validWorkflowGraph())

	result := ValidateWorkflowGraph(root, GraphOptions{})
	if result.Status != GraphStatusPass || result.File != WorkflowGraphDefaultPath || result.Checksum == "" || result.EffectiveSource != "project_file" || len(result.Errors) != 0 {
		t.Fatalf("validation = %#v, want passing default graph", result)
	}

	explained := ExplainWorkflowGraph(root, GraphOptions{})
	if explained.Status != GraphStatusPass || explained.GraphVersion != WorkflowGraphSchemaVersion || len(explained.Phases) != 2 || len(explained.Edges) != 1 || len(explained.Gates) != 1 || len(explained.ApprovalRequirements) != 1 {
		t.Fatalf("explanation = %#v, want projected graph", explained)
	}
	if explained.Phases[0].ID != "plan" || explained.Edges[0].From != "plan" || explained.Gates[0].Requires[1] != "implement" || explained.ApprovalRequirements[0].RequiredRole != "responsible-approver" {
		t.Fatalf("explanation details = %#v, want graph projection", explained)
	}
}

func TestValidateAndExplainWorkflowGraphFeedbackIntake(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	writeWorkflowGraph(t, repo, workflowGraphWithFeedbackIntake(validWorkflowGraph()))

	result := ValidateWorkflowGraph(root, GraphOptions{})
	if result.Status != GraphStatusPass || result.FeedbackIntake == nil || result.FeedbackIntake.Policy != graphFeedbackIntakePolicy || result.FeedbackIntake.MaxRounds != 5 {
		t.Fatalf("validation = %#v, want passing feedback intake projection", result)
	}
	if !reflect.DeepEqual(result.FeedbackIntake.RequiredRounds, []int{1}) || !reflect.DeepEqual(result.FeedbackIntake.OptionalRounds, []int{2, 3, 4, 5}) {
		t.Fatalf("feedback rounds = required:%#v optional:%#v, want min1/max5 projection", result.FeedbackIntake.RequiredRounds, result.FeedbackIntake.OptionalRounds)
	}

	explained := ExplainWorkflowGraph(root, GraphOptions{})
	if explained.Status != GraphStatusPass || explained.FeedbackIntake == nil || explained.FeedbackIntake.SchemaVersion != graphFeedbackIntakeSchema {
		t.Fatalf("explanation = %#v, want feedback intake projection", explained)
	}
}

func TestValidateWorkflowGraphAcceptsExplicitRepoRelativeFile(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	relative := "docs/graphs/candidate-workflow.yaml"
	writeGraphFile(t, repo, relative, validWorkflowGraph())

	result := ValidateWorkflowGraph(root, GraphOptions{File: relative})
	if result.Status != GraphStatusPass || result.File != relative || result.Checksum == "" || result.EffectiveSource != "project_file" {
		t.Fatalf("validation = %#v, want passing explicit graph candidate", result)
	}

	explained := ExplainWorkflowGraph(root, GraphOptions{File: relative})
	if explained.Status != GraphStatusPass || explained.ValidationSummary.File != relative || len(explained.Phases) != 2 {
		t.Fatalf("explanation = %#v, want explicit candidate graph projection", explained)
	}
}

func TestExplainWorkflowGraphUsesEmptyArrays(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	writeWorkflowGraph(t, repo, minimalWorkflowGraph())

	explained := ExplainWorkflowGraph(root, GraphOptions{})
	if explained.Status != GraphStatusPass {
		t.Fatalf("explanation = %#v, want passing minimal graph", explained)
	}
	if explained.Phases == nil || explained.Edges == nil || explained.Gates == nil || explained.ApprovalRequirements == nil || explained.PendingProposals == nil {
		t.Fatalf("explanation slices = phases:%v edges:%v gates:%v approvals:%v proposals:%v, want non-nil slices", explained.Phases, explained.Edges, explained.Gates, explained.ApprovalRequirements, explained.PendingProposals)
	}
	payload, err := json.Marshal(explained)
	if err != nil {
		t.Fatalf("marshal explanation: %v", err)
	}
	for _, want := range []string{`"edges":[]`, `"gates":[]`, `"approval_requirements":[]`, `"pending_proposals":[]`} {
		if !strings.Contains(string(payload), want) {
			t.Fatalf("explanation JSON = %s, want %s", payload, want)
		}
	}

	repo = initializedRepo(t)
	root, _ = DiscoverRoot(repo)
	failed := ExplainWorkflowGraph(root, GraphOptions{})
	if failed.Status != GraphStatusFail {
		t.Fatalf("failed explanation = %#v, want failing result", failed)
	}
	if failed.Phases == nil || failed.Edges == nil || failed.Gates == nil || failed.ApprovalRequirements == nil || failed.PendingProposals == nil {
		t.Fatalf("failed explanation slices = phases:%v edges:%v gates:%v approvals:%v proposals:%v, want non-nil slices", failed.Phases, failed.Edges, failed.Gates, failed.ApprovalRequirements, failed.PendingProposals)
	}
}

func TestExplainWorkflowGraphNormalizesNestedArrays(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	writeWorkflowGraph(t, repo, minimalWorkflowGraph()+`gates:
  - id: "empty"
`)

	explained := ExplainWorkflowGraph(root, GraphOptions{})
	if explained.Status != GraphStatusPass || len(explained.Gates) != 1 {
		t.Fatalf("explanation = %#v, want one valid gate", explained)
	}
	if explained.Gates[0].Requires == nil {
		t.Fatalf("gate requires = nil, want empty slice")
	}
	payload, err := json.Marshal(explained)
	if err != nil {
		t.Fatalf("marshal explanation: %v", err)
	}
	if !strings.Contains(string(payload), `"requires":[]`) {
		t.Fatalf("explanation JSON = %s, want gate requires []", payload)
	}
}

func TestValidateWorkflowGraphAcceptsDeclarativeGateChecks(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	writeWorkflowGraph(t, repo, minimalWorkflowGraph()+`gates:
  - id: "custom-gate"
    requires: ["plan"]
    final_required: true
    checks:
      - type: "artifact.exists"
        path: "final-report.md"
      - type: "markdown.field"
        path: "final-report.md"
        field: "Status"
        equals: "complete"
      - type: "text.contains"
        path: "final-report.md"
        token: "Needle"
      - type: "text.contains_all"
        path: "final-report.md"
        tokens: ["A", "B"]
      - type: "gitignore.contains_all"
        tokens: [".kkachi/", ".codegraph/", ".omx/", ".omc/", ".claude-octopus/"]
      - type: "codegraph.evidence"
        path: "codegraph-evidence.md"
        one_of: ["complete", "degraded"]
        tokens: ["codegraph index", "codegraph init -i"]
      - type: "phase.status"
        phase: "plan"
        status: "complete"
`)

	result := ValidateWorkflowGraph(root, GraphOptions{})
	if result.Status != GraphStatusPass || len(result.Errors) != 0 {
		t.Fatalf("validation = %#v, want pass", result)
	}
	explained := ExplainWorkflowGraph(root, GraphOptions{})
	if len(explained.Gates) != 1 || !explained.Gates[0].FinalRequired || len(explained.Gates[0].Checks) != 7 {
		t.Fatalf("gates = %#v, want final_required gate with seven checks", explained.Gates)
	}
}

func TestValidateWorkflowGraphRejectsUnsupportedDeclarativeGateCheck(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	writeWorkflowGraph(t, repo, minimalWorkflowGraph()+`gates:
  - id: "custom-gate"
    requires: ["final"]
    checks:
      - type: "regex.magic"
        path: "final-report.md"
`)

	result := ValidateWorkflowGraph(root, GraphOptions{})
	if result.Status != GraphStatusFail || !graphIssueNamed(result.Errors, "gate_check_type") {
		t.Fatalf("validation = %#v, want unsupported gate_check_type failure", result)
	}
}

func TestValidateWorkflowGraphRejectsBlankDeclarativeGateCheckValues(t *testing.T) {
	issues := []GraphIssue{}
	add := func(name string, field string, message string, expected string, actual string) {
		issues = append(issues, GraphIssue{Name: name, Field: field, Message: message, Expected: expected, Actual: actual})
	}
	phases := map[string]bool{"final": true}

	validateWorkflowGraphCheck(add, "custom-gate", WorkflowGraphCheck{Type: "text.contains_all", Path: "final-report.md", Tokens: []string{""}}, phases)
	validateWorkflowGraphCheck(add, "custom-gate", WorkflowGraphCheck{Type: "gitignore.contains_all", Tokens: []string{".kkachi/", " "}}, phases)
	validateWorkflowGraphCheck(add, "custom-gate", WorkflowGraphCheck{Type: "codegraph.evidence", OneOf: []string{""}, Tokens: []string{"codegraph index", ""}}, phases)

	if !graphIssueNamed(issues, "gate_check_tokens") || !graphIssueNamed(issues, "gate_check_status") {
		t.Fatalf("issues = %#v, want blank token/status failures", issues)
	}
}

func TestInitWorkflowGraphFromKHSDefault(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)

	result, err := InitWorkflowGraph(root, GraphInitOptions{
		FromTemplate: graphTemplateKHSDefault,
		Now:          func() time.Time { return time.Date(2026, 5, 22, 1, 2, 3, 0, time.UTC) },
	})
	if err != nil {
		t.Fatalf("InitWorkflowGraph() error = %v", err)
	}
	if result.Status != GraphStatusPass || result.TemplateID != graphTemplateKHSDefault || result.TemplateSource != graphTemplateSourceBuiltin || result.GraphPath != WorkflowGraphDefaultPath || result.EventID != "evt-000002" || result.Checksum == "" {
		t.Fatalf("result = %#v, want initialized graph result", result)
	}
	graphPath := filepath.Join(repo, WorkflowGraphDefaultPath)
	graphBytes, err := os.ReadFile(graphPath)
	if err != nil {
		t.Fatalf("read graph: %v", err)
	}
	sum := sha256.Sum256(graphBytes)
	if result.Checksum != hex.EncodeToString(sum[:]) {
		t.Fatalf("checksum = %s, want actual graph checksum", result.Checksum)
	}
	loaded := loadWorkflowGraph(root, GraphOptions{})
	if loaded.validation.Status != GraphStatusPass {
		t.Fatalf("validation = %#v, want pass", loaded.validation)
	}
	if loaded.graph.GraphID != "graph-kkachi-project-kkachi-test-abcdef123456" || loaded.graph.Metadata.Project != "kkachi-test" || loaded.graph.Metadata.SourceTemplate != graphTemplateKHSDefault || loaded.graph.Metadata.LastAppliedEventID != result.EventID {
		t.Fatalf("metadata = graph_id:%s metadata:%#v, want stamped project metadata", loaded.graph.GraphID, loaded.graph.Metadata)
	}
	if len(loaded.graph.Phases) != len(defaultPhaseIDs) || len(loaded.graph.Edges) != len(defaultPhaseIDs)-1 || len(loaded.graph.Gates) != 0 || len(loaded.graph.Approvals) != 0 {
		t.Fatalf("graph = phases:%d edges:%d gates:%d approvals:%d, want khs-default spine only", len(loaded.graph.Phases), len(loaded.graph.Edges), len(loaded.graph.Gates), len(loaded.graph.Approvals))
	}
	for i, id := range defaultPhaseIDs {
		if loaded.graph.Phases[i].ID != id {
			t.Fatalf("phase[%d] = %s, want %s", i, loaded.graph.Phases[i].ID, id)
		}
	}
	lines := runEventLines(t, repo)
	if !strings.Contains(lines[len(lines)-1], graphInitEventType) || !strings.Contains(lines[len(lines)-1], result.Checksum) {
		t.Fatalf("last event = %s, want graph initialized event with checksum", lines[len(lines)-1])
	}
}

func TestInitWorkflowGraphFromTemplatePathStampsProjectMetadata(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	templatePath := "docs/workflows/template.yaml"
	writeGraphFile(t, repo, templatePath, strings.Replace(strings.Replace(validWorkflowGraph(), `graph_id: "graph-test"`, `graph_id: "foreign-graph"`, 1), `project: "kkachi-test"`, `project: "foreign-project"`, 1))

	result, err := InitWorkflowGraph(root, GraphInitOptions{FromTemplate: templatePath})
	if err != nil {
		t.Fatalf("InitWorkflowGraph() error = %v", err)
	}
	loaded := loadWorkflowGraph(root, GraphOptions{})
	if loaded.validation.Status != GraphStatusPass {
		t.Fatalf("validation = %#v, want pass", loaded.validation)
	}
	if result.TemplateID != templatePath || result.TemplateSource != graphTemplateSourcePath {
		t.Fatalf("result = %#v, want path template source", result)
	}
	if loaded.graph.GraphID != "graph-kkachi-project-kkachi-test-abcdef123456" || loaded.graph.Metadata.Project != "kkachi-test" || loaded.graph.Metadata.SourceTemplate != templatePath || loaded.graph.Metadata.ManagedBy != "kah" {
		t.Fatalf("stamped graph = graph_id:%s metadata:%#v, want current project metadata", loaded.graph.GraphID, loaded.graph.Metadata)
	}
	if len(loaded.graph.Gates) != 1 || len(loaded.graph.Approvals) != 1 || loaded.graph.Proposals.Policy != "proposal-first" {
		t.Fatalf("stamped graph = %#v, want template graph policy content preserved", loaded.graph)
	}
}

func TestInitWorkflowGraphFailsClosedWhenGraphAlreadyExists(t *testing.T) {
	cases := []struct {
		name         string
		fromTemplate string
		setup        func(t *testing.T, repo string)
	}{
		{name: "valid file", setup: func(t *testing.T, repo string) { writeWorkflowGraph(t, repo, validWorkflowGraph()) }},
		{name: "invalid file", setup: func(t *testing.T, repo string) { writeWorkflowGraph(t, repo, "not yaml\n") }},
		{name: "invalid file before unknown template", fromTemplate: "foo", setup: func(t *testing.T, repo string) { writeWorkflowGraph(t, repo, "not yaml\n") }},
		{name: "directory", setup: func(t *testing.T, repo string) { mustMkdir(t, filepath.Join(repo, WorkflowGraphDefaultPath)) }},
		{name: "symlink", setup: func(t *testing.T, repo string) {
			target := filepath.Join(repo, "target-workflow.yaml")
			if err := os.WriteFile(target, []byte(validWorkflowGraph()), 0o600); err != nil {
				t.Fatalf("write symlink target: %v", err)
			}
			if err := os.Symlink("target-workflow.yaml", filepath.Join(repo, WorkflowGraphDefaultPath)); err != nil {
				t.Fatalf("symlink graph: %v", err)
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := initializedRepo(t)
			root, _ := DiscoverRoot(repo)
			beforeEvents := runEventLines(t, repo)
			tc.setup(t, repo)

			fromTemplate := tc.fromTemplate
			if fromTemplate == "" {
				fromTemplate = graphTemplateKHSDefault
			}
			_, err := InitWorkflowGraph(root, GraphInitOptions{FromTemplate: fromTemplate})
			assertProblemCode(t, err, "graph_already_exists")
			if afterEvents := runEventLines(t, repo); len(afterEvents) != len(beforeEvents) {
				t.Fatalf("events changed after existing graph rejection: before=%d after=%d", len(beforeEvents), len(afterEvents))
			}
		})
	}
}

func TestInitWorkflowGraphRejectsInvalidTemplateInputs(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)

	_, err := InitWorkflowGraph(root, GraphInitOptions{FromTemplate: "foo"})
	assertProblemCode(t, err, "graph_template_unknown")

	writeGraphFile(t, repo, "graphs/invalid.yaml", strings.Replace(validWorkflowGraph(), `to: "implement"`, `to: "missing"`, 1))
	_, err = InitWorkflowGraph(root, GraphInitOptions{FromTemplate: "graphs/invalid.yaml"})
	assertProblemCode(t, err, "graph_template_invalid")

	_, err = InitWorkflowGraph(root, GraphInitOptions{FromTemplate: ".kkachi/config/workflows/templates/default.yaml"})
	assertProblemCode(t, err, "graph_template_invalid")

	_, err = InitWorkflowGraph(root, GraphInitOptions{FromTemplate: graphTemplateKHSDefault, Output: "docs/workflow.yaml"})
	assertProblemCode(t, err, "graph_output_invalid")
}

func TestValidateWorkflowGraphMissingAndForbiddenSources(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)

	missing := ValidateWorkflowGraph(root, GraphOptions{})
	if missing.Status != GraphStatusFail || !graphIssueNamed(missing.Errors, graphIssueGraphFile) {
		t.Fatalf("missing validation = %#v, want graph_file failure", missing)
	}

	forbidden := ValidateWorkflowGraph(root, GraphOptions{File: ".kkachi/config.yaml"})
	if forbidden.Status != GraphStatusFail || !graphIssueNamed(forbidden.Errors, "graph_source") {
		t.Fatalf("forbidden config validation = %#v, want graph_source failure", forbidden)
	}

	diagram := ValidateWorkflowGraph(root, GraphOptions{File: "docs/generated.mmd"})
	if diagram.Status != GraphStatusFail || !graphIssueNamed(diagram.Errors, "graph_source") {
		t.Fatalf("diagram validation = %#v, want graph_source failure", diagram)
	}

	escaped := ValidateWorkflowGraph(root, GraphOptions{File: "../graph.yaml"})
	if escaped.Status != GraphStatusFail || !graphIssueNamed(escaped.Errors, "graph_source") {
		t.Fatalf("escaped validation = %#v, want graph_source failure", escaped)
	}
}

func TestValidateWorkflowGraphRejectsInvalidShape(t *testing.T) {
	cases := []struct {
		name      string
		body      string
		wantIssue string
	}{
		{
			name:      "unsupported field",
			body:      strings.Replace(validWorkflowGraph(), `graph_id: "graph-test"`, "unexpected: true", 1),
			wantIssue: "graph_yaml",
		},
		{
			name:      "duplicate phase",
			body:      strings.Replace(validWorkflowGraph(), `  - id: "implement"`, `  - id: "plan"`, 1),
			wantIssue: "duplicate_phase",
		},
		{
			name:      "missing required phase field",
			body:      strings.Replace(validWorkflowGraph(), "    required: true\n    evidence: [\"plan.md\", \"checklist.md\"]", "    evidence: [\"plan.md\", \"checklist.md\"]", 1),
			wantIssue: "phase_required",
		},
		{
			name:      "edge reference",
			body:      strings.Replace(validWorkflowGraph(), `to: "implement"`, `to: "missing"`, 1),
			wantIssue: "edge_to",
		},
		{
			name: "duplicate gate",
			body: strings.Replace(validWorkflowGraph(), `gates:
  - id: "pre-implementation"
    requires: ["plan", "implement"]`, `gates:
  - id: "pre-implementation"
    requires: ["plan", "implement"]
  - id: "pre-implementation"
    requires: ["plan"]`, 1),
			wantIssue: "duplicate_gate",
		},
		{
			name: "duplicate approval scope",
			body: strings.Replace(validWorkflowGraph(), `approvals:
  - scope: "sot-change"
    required_role: "responsible-approver"`, `approvals:
  - scope: "sot-change"
    required_role: "responsible-approver"
  - scope: "sot-change"
    required_role: "required-reviewer"`, 1),
			wantIssue: "duplicate_approval",
		},
		{
			name: "duplicate top-level section",
			body: strings.Replace(validWorkflowGraph(), `phases:
  - id: "plan"`, `phases:
phases:
  - id: "plan"`, 1),
			wantIssue: "graph_yaml",
		},
		{
			name:      "duplicate metadata key",
			body:      strings.Replace(validWorkflowGraph(), `  project: "kkachi-test"`, "  project: \"kkachi-test\"\n  project: \"other\"", 1),
			wantIssue: "graph_yaml",
		},
		{
			name:      "duplicate list item key",
			body:      strings.Replace(validWorkflowGraph(), `  - id: "plan"`, "  - id: \"plan\"\n    id: \"plan-duplicate\"", 1),
			wantIssue: "graph_yaml",
		},
		{
			name: "cycle",
			body: strings.Replace(validWorkflowGraph(), `edges:
  - from: "plan"
    to: "implement"`, `edges:
  - from: "plan"
    to: "implement"
  - from: "implement"
    to: "plan"`, 1),
			wantIssue: "cycle",
		},
		{
			name:      "invalid proposals policy",
			body:      strings.Replace(validWorkflowGraph(), `policy: "proposal-first"`, `policy: "direct-edit"`, 1),
			wantIssue: "proposals_policy",
		},
		{
			name:      "missing managed by",
			body:      strings.Replace(validWorkflowGraph(), `managed_by: "kah"`, `managed_by: "khs"`, 1),
			wantIssue: "metadata_managed_by",
		},
		{
			name:      "malformed yaml",
			body:      strings.Replace(validWorkflowGraph(), `version: "workflow-graph/v1"`, `version "workflow-graph/v1"`, 1),
			wantIssue: "graph_yaml",
		},
		{
			name:      "unquoted inline comment",
			body:      strings.Replace(validWorkflowGraph(), `graph_id: "graph-test"`, `graph_id: graph-test # comment`, 1),
			wantIssue: "graph_yaml",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := initializedRepo(t)
			root, _ := DiscoverRoot(repo)
			writeWorkflowGraph(t, repo, tc.body)

			result := ValidateWorkflowGraph(root, GraphOptions{})
			if result.Status != GraphStatusFail || !graphIssueNamed(result.Errors, tc.wantIssue) {
				t.Fatalf("validation = %#v, want issue %s", result, tc.wantIssue)
			}
		})
	}
}

func TestValidateWorkflowGraphRejectsInvalidFeedbackIntake(t *testing.T) {
	cases := []struct {
		name      string
		section   string
		wantIssue string
	}{
		{
			name:      "missing field",
			section:   strings.Replace(validFeedbackIntakeSection(), `  max_rounds: 5`+"\n", "", 1),
			wantIssue: "feedback_intake_max_rounds",
		},
		{
			name:      "unsupported schema",
			section:   strings.Replace(validFeedbackIntakeSection(), graphFeedbackIntakeSchema, "external-feedback-intake/v2", 1),
			wantIssue: "feedback_intake_schema_version",
		},
		{
			name:      "unsupported policy",
			section:   strings.Replace(validFeedbackIntakeSection(), graphFeedbackIntakePolicy, "OTHER_POLICY", 1),
			wantIssue: "feedback_intake_policy",
		},
		{
			name:      "duplicate declaration",
			section:   strings.Replace(validFeedbackIntakeSection(), `  policy: "EXTERNAL_FEEDBACK_INTAKE"`, "  policy: \"EXTERNAL_FEEDBACK_INTAKE\"\n  policy: \"EXTERNAL_FEEDBACK_INTAKE\"", 1),
			wantIssue: "graph_yaml",
		},
		{
			name:      "conflicting declarations",
			section:   strings.Replace(validFeedbackIntakeSection(), `optional_rounds: [2,3,4,5]`, `optional_rounds: [1,2,3,4,5]`, 1),
			wantIssue: "feedback_intake_rounds_conflict",
		},
		{
			name:      "stale max3",
			section:   strings.Replace(strings.Replace(validFeedbackIntakeSection(), `max_rounds: 5`, `max_rounds: 3`, 1), `optional_rounds: [2,3,4,5]`, `optional_rounds: [2,3]`, 1),
			wantIssue: "feedback_intake_stale_bounds",
		},
		{
			name:      "max below min",
			section:   strings.Replace(strings.Replace(validFeedbackIntakeSection(), `min_rounds: 1`, `min_rounds: 4`, 1), `max_rounds: 5`, `max_rounds: 3`, 1),
			wantIssue: "feedback_intake_round_bounds",
		},
		{
			name:      "min below one",
			section:   strings.Replace(validFeedbackIntakeSection(), `min_rounds: 1`, `min_rounds: 0`, 1),
			wantIssue: "feedback_intake_min_rounds",
		},
		{
			name:      "max above five",
			section:   strings.Replace(validFeedbackIntakeSection(), `max_rounds: 5`, `max_rounds: 6`, 1),
			wantIssue: "feedback_intake_max_rounds",
		},
		{
			name:      "required round one missing",
			section:   strings.Replace(validFeedbackIntakeSection(), `required_rounds: [1]`, `required_rounds: [2]`, 1),
			wantIssue: "feedback_intake_required_rounds",
		},
		{
			name:      "round six rejected",
			section:   strings.Replace(validFeedbackIntakeSection(), `optional_rounds: [2,3,4,5]`, `optional_rounds: [2,3,4,5,6]`, 1),
			wantIssue: "feedback_intake_round_range",
		},
		{
			name:      "duplicate section",
			section:   validFeedbackIntakeSection() + validFeedbackIntakeSection(),
			wantIssue: "graph_yaml",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := initializedRepo(t)
			root, _ := DiscoverRoot(repo)
			writeWorkflowGraph(t, repo, validWorkflowGraph()+tc.section)

			result := ValidateWorkflowGraph(root, GraphOptions{})
			if result.Status != GraphStatusFail || !graphIssueNamed(result.Errors, tc.wantIssue) {
				t.Fatalf("validation = %#v, want issue %s", result, tc.wantIssue)
			}
		})
	}
}

func TestDiffWorkflowGraphReportsSemanticChanges(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	from := "graphs/from.yaml"
	to := "graphs/to.yaml"
	writeGraphFile(t, repo, from, validWorkflowGraph())
	writeGraphFile(t, repo, to, candidateWorkflowGraph())

	result := DiffWorkflowGraph(root, GraphDiffOptions{From: from, To: to})
	if result.Status != GraphStatusPass {
		t.Fatalf("diff = %#v, want pass", result)
	}
	if result.From.File != from || result.To.File != to || result.From.Checksum == "" || result.To.Checksum == "" {
		t.Fatalf("diff endpoints = %#v -> %#v, want files and checksums", result.From, result.To)
	}
	if len(result.ChangedPhases.Added) != 1 || result.ChangedPhases.Added[0].ID != "ask" {
		t.Fatalf("phase additions = %#v, want ask", result.ChangedPhases.Added)
	}
	if len(result.ChangedPhases.Modified) != 1 || result.ChangedPhases.Modified[0].Key != "implement" {
		t.Fatalf("phase modifications = %#v, want implement", result.ChangedPhases.Modified)
	}
	if len(result.ChangedEdges.Removed) != 1 || len(result.ChangedEdges.Added) != 2 {
		t.Fatalf("edge changes = %#v, want one removed and two added", result.ChangedEdges)
	}
	if len(result.ChangedGates.Modified) != 1 || result.ChangedGates.Modified[0].Key != "pre-implementation" {
		t.Fatalf("gate modifications = %#v, want pre-implementation", result.ChangedGates.Modified)
	}
	if len(result.ChangedApprovals.Modified) != 1 || result.ChangedApprovals.Modified[0].Key != "sot-change" {
		t.Fatalf("approval modifications = %#v, want sot-change", result.ChangedApprovals.Modified)
	}
	for _, want := range []string{"approvals_changed", "dependencies_changed", "gates_changed", "phase_required_changed"} {
		if !graphStringSliceContains(result.RiskFlags, want) {
			t.Fatalf("risk flags = %#v, want %s", result.RiskFlags, want)
		}
	}
	if !result.RequiresApproval {
		t.Fatalf("requires approval = false, want true")
	}
}

func TestDiffWorkflowGraphReportsFeedbackIntakeChanges(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	from := "graphs/from.yaml"
	to := "graphs/to.yaml"
	writeGraphFile(t, repo, from, validWorkflowGraph())
	writeGraphFile(t, repo, to, workflowGraphWithFeedbackIntake(validWorkflowGraph()))

	result := DiffWorkflowGraph(root, GraphDiffOptions{From: from, To: to})
	if result.Status != GraphStatusPass || !result.ChangedFeedbackIntake.Changed || result.ChangedFeedbackIntake.Before != nil || result.ChangedFeedbackIntake.After == nil {
		t.Fatalf("diff = %#v, want added feedback intake change", result)
	}
	if !graphStringSliceContains(result.RiskFlags, "feedback_intake_changed") || !result.RequiresApproval {
		t.Fatalf("risk flags = %#v requires=%t, want feedback intake approval risk", result.RiskFlags, result.RequiresApproval)
	}
}

func TestDiffWorkflowGraphReportsSemanticRemovalsAndAddedRecords(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	from := "graphs/from.yaml"
	to := "graphs/to.yaml"
	writeGraphFile(t, repo, from, expandedWorkflowGraph())
	writeGraphFile(t, repo, to, validWorkflowGraph())

	result := DiffWorkflowGraph(root, GraphDiffOptions{From: from, To: to})
	if result.Status != GraphStatusPass {
		t.Fatalf("diff = %#v, want pass", result)
	}
	if len(result.ChangedPhases.Removed) != 1 || result.ChangedPhases.Removed[0].ID != "ask" {
		t.Fatalf("phase removals = %#v, want ask", result.ChangedPhases.Removed)
	}
	if len(result.ChangedEdges.Removed) != 2 || len(result.ChangedEdges.Added) != 1 {
		t.Fatalf("edge changes = %#v, want two removed and one added", result.ChangedEdges)
	}
	if len(result.ChangedGates.Removed) != 1 || result.ChangedGates.Removed[0].ID != "post-implementation" {
		t.Fatalf("gate removals = %#v, want post-implementation", result.ChangedGates.Removed)
	}
	if len(result.ChangedApprovals.Removed) != 1 || result.ChangedApprovals.Removed[0].Scope != "release" {
		t.Fatalf("approval removals = %#v, want release", result.ChangedApprovals.Removed)
	}
	for _, want := range []string{"approvals_changed", "dependencies_changed", "gates_changed", "phase_removed"} {
		if !graphStringSliceContains(result.RiskFlags, want) {
			t.Fatalf("risk flags = %#v, want %s", result.RiskFlags, want)
		}
	}
	if !result.RequiresApproval {
		t.Fatalf("requires approval = false, want true")
	}

	added := DiffWorkflowGraph(root, GraphDiffOptions{From: to, To: from})
	if len(added.ChangedGates.Added) != 1 || added.ChangedGates.Added[0].ID != "post-implementation" {
		t.Fatalf("gate additions = %#v, want post-implementation", added.ChangedGates.Added)
	}
	if len(added.ChangedApprovals.Added) != 1 || added.ChangedApprovals.Added[0].Scope != "release" {
		t.Fatalf("approval additions = %#v, want release", added.ChangedApprovals.Added)
	}
}

func TestDiffWorkflowGraphRequiresApprovalForIdentityAndMetadataChanges(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	from := "graphs/from.yaml"
	to := "graphs/to.yaml"
	writeGraphFile(t, repo, from, validWorkflowGraph())
	writeGraphFile(t, repo, to, strings.Replace(strings.Replace(validWorkflowGraph(), `graph_id: "graph-test"`, `graph_id: "graph-next"`, 1), `project: "kkachi-test"`, `project: "kkachi-next"`, 1))

	result := DiffWorkflowGraph(root, GraphDiffOptions{From: from, To: to})
	if result.Status != GraphStatusPass || !result.RequiresApproval {
		t.Fatalf("diff = %#v, want pass requiring approval", result)
	}
	for _, want := range []string{"graph_identity_changed", "metadata_changed"} {
		if !graphStringSliceContains(result.RiskFlags, want) {
			t.Fatalf("risk flags = %#v, want %s", result.RiskFlags, want)
		}
	}
}

func TestDiffWorkflowGraphFailsClosedForInvalidInputs(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	writeGraphFile(t, repo, "graphs/from.yaml", validWorkflowGraph())
	writeGraphFile(t, repo, "graphs/to.yaml", strings.Replace(validWorkflowGraph(), `to: "implement"`, `to: "missing"`, 1))

	result := DiffWorkflowGraph(root, GraphDiffOptions{From: "graphs/from.yaml", To: "graphs/to.yaml"})
	if result.Status != GraphStatusFail || result.NextAction != graphNextActionRepair {
		t.Fatalf("diff = %#v, want fail/repair", result)
	}
	if !graphIssueNamed(result.ValidationSummary.To.Errors, "edge_to") {
		t.Fatalf("to validation = %#v, want edge_to", result.ValidationSummary.To)
	}

	writeGraphFile(t, repo, "graphs/from-invalid.yaml", strings.Replace(validWorkflowGraph(), `to: "implement"`, `to: "missing"`, 1))
	result = DiffWorkflowGraph(root, GraphDiffOptions{From: "graphs/from-invalid.yaml", To: "graphs/from.yaml"})
	if result.Status != GraphStatusFail || !graphIssueNamed(result.ValidationSummary.From.Errors, "edge_to") {
		t.Fatalf("from validation = %#v, want edge_to failure", result.ValidationSummary.From)
	}
}

func TestProposeWorkflowGraphRecordsProposalWithoutMutatingGraph(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	writeWorkflowGraph(t, repo, validWorkflowGraph())
	patch := "graphs/candidate.yaml"
	writeGraphFile(t, repo, patch, candidateWorkflowGraph())
	beforeGraph := readGraphTestText(t, filepath.Join(repo, WorkflowGraphDefaultPath))

	result, err := ProposeWorkflowGraph(root, GraphProposeOptions{
		Patch:  patch,
		Reason: "add ask phase",
		Now:    func() time.Time { return time.Date(2026, 5, 22, 1, 2, 3, 0, time.UTC) },
	})
	if err != nil {
		t.Fatalf("ProposeWorkflowGraph() error = %v", err)
	}
	if result.ProposalID != "gprop-000001" || result.ProposalPath != ".kkachi/graph/proposals/gprop-000001.json" || result.SemanticDiffRef != result.ProposalPath+"#semantic_diff" || result.EventID != "evt-000002" {
		t.Fatalf("proposal result = %#v, want first proposal with event", result)
	}
	if got := readGraphTestText(t, filepath.Join(repo, WorkflowGraphDefaultPath)); got != beforeGraph {
		t.Fatalf("workflow graph mutated\nbefore=%s\nafter=%s", beforeGraph, got)
	}
	var record WorkflowGraphProposalRecord
	readGraphTestJSON(t, filepath.Join(repo, filepath.FromSlash(result.ProposalPath)), &record)
	if record.ProposalID != result.ProposalID || record.Reason != "add ask phase" || record.SemanticDiff.To.File != patch || !record.ApprovalRequired {
		t.Fatalf("proposal record = %#v, want stored diff and approval requirement", record)
	}
	lines := runEventLines(t, repo)
	if !strings.Contains(lines[len(lines)-1], graphProposalEventType) || !strings.Contains(lines[len(lines)-1], result.ProposalID) {
		t.Fatalf("last event = %s, want graph proposal event", lines[len(lines)-1])
	}
}

func TestProposeWorkflowGraphRejectsInvalidPatchWithoutWriting(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	writeWorkflowGraph(t, repo, validWorkflowGraph())
	patch := "graphs/invalid.yaml"
	writeGraphFile(t, repo, patch, strings.Replace(validWorkflowGraph(), `to: "implement"`, `to: "missing"`, 1))
	beforeEvents := runEventLines(t, repo)

	_, err := ProposeWorkflowGraph(root, GraphProposeOptions{Patch: patch, Reason: "invalid candidate"})
	if err == nil {
		t.Fatalf("ProposeWorkflowGraph() succeeded, want invalid patch error")
	}
	if afterEvents := runEventLines(t, repo); len(afterEvents) != len(beforeEvents) {
		t.Fatalf("events changed after rejection: before=%d after=%d", len(beforeEvents), len(afterEvents))
	}
	if _, statErr := os.Stat(filepath.Join(repo, ".kkachi", "graph", "proposals")); !os.IsNotExist(statErr) {
		t.Fatalf("proposal directory stat err = %v, want missing directory", statErr)
	}
}

func TestProposeWorkflowGraphRequiresInitializedAndValidBaseState(t *testing.T) {
	t.Run("uninitialized helper state", func(t *testing.T) {
		repo := t.TempDir()
		if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
			t.Fatalf("mkdir .git: %v", err)
		}
		root, _ := DiscoverRoot(repo)
		writeWorkflowGraph(t, repo, validWorkflowGraph())
		writeGraphFile(t, repo, "graphs/candidate.yaml", candidateWorkflowGraph())

		if _, err := ProposeWorkflowGraph(root, GraphProposeOptions{Patch: "graphs/candidate.yaml", Reason: "add ask phase"}); err == nil {
			t.Fatalf("ProposeWorkflowGraph() succeeded, want helper state failure")
		}
		if _, statErr := os.Stat(filepath.Join(repo, ".kkachi", "graph", "proposals")); !os.IsNotExist(statErr) {
			t.Fatalf("proposal directory stat err = %v, want missing directory", statErr)
		}
	})

	t.Run("invalid base graph", func(t *testing.T) {
		repo := initializedRepo(t)
		root, _ := DiscoverRoot(repo)
		writeWorkflowGraph(t, repo, strings.Replace(validWorkflowGraph(), `to: "implement"`, `to: "missing"`, 1))
		writeGraphFile(t, repo, "graphs/candidate.yaml", candidateWorkflowGraph())

		if _, err := ProposeWorkflowGraph(root, GraphProposeOptions{Patch: "graphs/candidate.yaml", Reason: "add ask phase"}); err == nil {
			t.Fatalf("ProposeWorkflowGraph() succeeded, want invalid base graph failure")
		}
		if _, statErr := os.Stat(filepath.Join(repo, ".kkachi", "graph", "proposals")); !os.IsNotExist(statErr) {
			t.Fatalf("proposal directory stat err = %v, want missing directory", statErr)
		}
	})
}

func TestApplyWorkflowGraphAppliesApprovedProposal(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	writeWorkflowGraph(t, repo, validWorkflowGraph())
	patch := "graphs/candidate.yaml"
	writeGraphFile(t, repo, patch, candidateWorkflowGraph())
	proposal, err := ProposeWorkflowGraph(root, GraphProposeOptions{Patch: patch, Reason: "add ask phase"})
	if err != nil {
		t.Fatalf("ProposeWorkflowGraph() error = %v", err)
	}
	proposalBefore := readGraphTestText(t, filepath.Join(repo, filepath.FromSlash(proposal.ProposalPath)))

	result, err := ApplyWorkflowGraph(root, GraphApplyOptions{
		Proposal: proposal.ProposalID,
		Approval: "approval-record:grafana-123",
		Now:      func() time.Time { return time.Date(2026, 5, 22, 2, 3, 4, 0, time.UTC) },
	})
	if err != nil {
		t.Fatalf("ApplyWorkflowGraph() error = %v", err)
	}
	if result.Status != GraphStatusPass || result.ProposalID != proposal.ProposalID || result.ApprovalRef != "approval-record:grafana-123" || result.GraphPath != WorkflowGraphDefaultPath || len(result.EventIDs) != 1 || result.EventIDs[0] != "evt-000003" || result.NewChecksum == "" {
		t.Fatalf("apply result = %#v, want applied proposal", result)
	}
	graphBytes, err := os.ReadFile(filepath.Join(repo, WorkflowGraphDefaultPath))
	if err != nil {
		t.Fatalf("read applied graph: %v", err)
	}
	sum := sha256.Sum256(graphBytes)
	if result.NewChecksum != hex.EncodeToString(sum[:]) {
		t.Fatalf("new checksum = %s, want actual graph checksum", result.NewChecksum)
	}
	graphText := string(graphBytes)
	for _, want := range []string{`id: "ask"`, `required: false`, `last_applied_event_id: "evt-000003"`} {
		if !strings.Contains(graphText, want) {
			t.Fatalf("applied graph = %s, want %s", graphText, want)
		}
	}
	if got := readGraphTestText(t, filepath.Join(repo, filepath.FromSlash(proposal.ProposalPath))); got != proposalBefore {
		t.Fatalf("proposal record changed\nbefore=%s\nafter=%s", proposalBefore, got)
	}
	lines := runEventLines(t, repo)
	if !strings.Contains(lines[len(lines)-1], graphApplyEventType) || !strings.Contains(lines[len(lines)-1], result.NewChecksum) || !strings.Contains(lines[len(lines)-1], "approval-record:grafana-123") {
		t.Fatalf("last event = %s, want graph applied audit event", lines[len(lines)-1])
	}
}

func TestGraphProposalAndApplyAllowStaleFeedbackIntakeMigration(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	writeWorkflowGraph(t, repo, staleFeedbackIntakeWorkflowGraph())
	candidate := "graphs/candidate.yaml"
	writeGraphFile(t, repo, candidate, workflowGraphWithFeedbackIntake(validWorkflowGraph()))

	proposal, err := ProposeWorkflowGraph(root, GraphProposeOptions{
		Patch:  candidate,
		Reason: "migrate stale feedback intake bounds",
		Now:    func() time.Time { return time.Date(2026, 5, 24, 1, 2, 3, 0, time.UTC) },
	})
	if err != nil {
		t.Fatalf("ProposeWorkflowGraph() error = %v", err)
	}
	if !proposal.ApprovalRequired {
		t.Fatalf("proposal = %#v, want approval required for feedback migration", proposal)
	}
	var record WorkflowGraphProposalRecord
	readGraphTestJSON(t, filepath.Join(repo, filepath.FromSlash(proposal.ProposalPath)), &record)
	if record.ValidationSummary.Base.Status != GraphStatusFail || !graphValidationOnlyFeedbackStaleBounds(record.ValidationSummary.Base) {
		t.Fatalf("base validation = %#v, want stale-only migration evidence", record.ValidationSummary.Base)
	}
	if record.SemanticDiff.Status != GraphStatusPass || !graphStringSliceContains(record.SemanticDiff.RiskFlags, "feedback_intake_changed") {
		t.Fatalf("semantic diff = %#v, want feedback intake migration risk", record.SemanticDiff)
	}

	applied, err := ApplyWorkflowGraph(root, GraphApplyOptions{
		Proposal: proposal.ProposalID,
		Approval: "audit:feedback-intake-migration",
		Now:      func() time.Time { return time.Date(2026, 5, 24, 2, 3, 4, 0, time.UTC) },
	})
	if err != nil {
		t.Fatalf("ApplyWorkflowGraph() error = %v", err)
	}
	if applied.Status != GraphStatusPass || applied.ApprovalRef != "audit:feedback-intake-migration" || applied.NewChecksum == "" {
		t.Fatalf("applied = %#v, want stale feedback intake migration applied", applied)
	}
	graph := readGraphTestText(t, filepath.Join(repo, WorkflowGraphDefaultPath))
	for _, want := range []string{`max_rounds: 5`, `optional_rounds: [2, 3, 4, 5]`, `last_applied_event_id: "evt-000003"`} {
		if !strings.Contains(graph, want) {
			t.Fatalf("applied graph = %s, want %s", graph, want)
		}
	}
}

func TestGraphProposalRejectsNonMigrationInvalidBase(t *testing.T) {
	cases := []struct {
		name      string
		base      string
		candidate string
	}{
		{
			name:      "stale plus invalid edge",
			base:      strings.Replace(staleFeedbackIntakeWorkflowGraph(), `to: "implement"`, `to: "missing"`, 1),
			candidate: workflowGraphWithFeedbackIntake(validWorkflowGraph()),
		},
		{
			name:      "candidate omits canonical feedback intake",
			base:      staleFeedbackIntakeWorkflowGraph(),
			candidate: validWorkflowGraph(),
		},
		{
			name:      "candidate includes unrelated graph changes",
			base:      staleFeedbackIntakeWorkflowGraph(),
			candidate: workflowGraphWithFeedbackIntake(candidateWorkflowGraph()),
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			repo := initializedRepo(t)
			root, _ := DiscoverRoot(repo)
			writeWorkflowGraph(t, repo, tt.base)
			writeGraphFile(t, repo, "graphs/candidate.yaml", tt.candidate)

			_, err := ProposeWorkflowGraph(root, GraphProposeOptions{Patch: "graphs/candidate.yaml", Reason: "invalid migration"})
			assertProblemCode(t, err, "graph_proposal_invalid")
		})
	}
}

func TestApplyWorkflowGraphFailsClosedWithoutMutation(t *testing.T) {
	cases := []struct {
		name     string
		mutate   func(t *testing.T, repo string, proposal GraphProposalResult)
		options  func(proposal GraphProposalResult) GraphApplyOptions
		wantCode string
	}{
		{
			name: "stale base checksum",
			mutate: func(t *testing.T, repo string, _ GraphProposalResult) {
				writeWorkflowGraph(t, repo, strings.Replace(validWorkflowGraph(), `title: "Plan"`, `title: "Plan Updated"`, 1))
			},
			wantCode: "graph_base_checksum_mismatch",
		},
		{
			name: "changed candidate checksum",
			mutate: func(t *testing.T, repo string, _ GraphProposalResult) {
				writeGraphFile(t, repo, "graphs/candidate.yaml", strings.Replace(candidateWorkflowGraph(), `title: "Ask"`, `title: "Ask Updated"`, 1))
			},
			wantCode: "graph_candidate_checksum_mismatch",
		},
		{
			name: "missing candidate",
			mutate: func(t *testing.T, repo string, _ GraphProposalResult) {
				if err := os.Remove(filepath.Join(repo, "graphs", "candidate.yaml")); err != nil {
					t.Fatalf("remove candidate: %v", err)
				}
			},
			wantCode: "graph_apply_invalid",
		},
		{
			name: "invalid candidate",
			mutate: func(t *testing.T, repo string, _ GraphProposalResult) {
				writeGraphFile(t, repo, "graphs/candidate.yaml", strings.Replace(candidateWorkflowGraph(), `to: "implement"`, `to: "missing"`, 1))
			},
			wantCode: "graph_apply_invalid",
		},
		{
			name: "proposal record invalid JSON",
			mutate: func(t *testing.T, repo string, proposal GraphProposalResult) {
				writeGraphProposalRecordRaw(t, repo, proposal, "{not json\n")
			},
			wantCode: "graph_proposal_invalid_json",
		},
		{
			name: "proposal record schema mismatch",
			mutate: func(t *testing.T, repo string, proposal GraphProposalResult) {
				updateGraphProposalRecord(t, repo, proposal, func(record *WorkflowGraphProposalRecord) {
					record.SchemaVersion = "workflow-graph/v0"
				})
			},
			wantCode: "graph_proposal_schema_invalid",
		},
		{
			name: "proposal record status mismatch",
			mutate: func(t *testing.T, repo string, proposal GraphProposalResult) {
				updateGraphProposalRecord(t, repo, proposal, func(record *WorkflowGraphProposalRecord) {
					record.Status = GraphStatusFail
				})
			},
			wantCode: "graph_proposal_status_invalid",
		},
		{
			name: "proposal record id mismatch",
			mutate: func(t *testing.T, repo string, proposal GraphProposalResult) {
				updateGraphProposalRecord(t, repo, proposal, func(record *WorkflowGraphProposalRecord) {
					record.ProposalID = "gprop-000002"
				})
			},
			wantCode: "graph_proposal_id_mismatch",
		},
		{
			name: "proposal record path mismatch",
			mutate: func(t *testing.T, repo string, proposal GraphProposalResult) {
				updateGraphProposalRecord(t, repo, proposal, func(record *WorkflowGraphProposalRecord) {
					record.ProposalPath = ".kkachi/graph/proposals/gprop-000002.json"
				})
			},
			wantCode: "graph_proposal_path_mismatch",
		},
		{
			name: "proposal record missing candidate evidence",
			mutate: func(t *testing.T, repo string, proposal GraphProposalResult) {
				updateGraphProposalRecord(t, repo, proposal, func(record *WorkflowGraphProposalRecord) {
					record.Candidate.Checksum = ""
				})
			},
			wantCode: "graph_proposal_record_invalid",
		},
		{
			name: "proposal record base file mismatch",
			mutate: func(t *testing.T, repo string, proposal GraphProposalResult) {
				updateGraphProposalRecord(t, repo, proposal, func(record *WorkflowGraphProposalRecord) {
					record.Base.File = "graphs/base.yaml"
				})
			},
			wantCode: "graph_proposal_base_invalid",
		},
		{
			name: "malformed proposal id",
			options: func(GraphProposalResult) GraphApplyOptions {
				return GraphApplyOptions{Proposal: "gprop-1.json", Approval: "approval:evidence"}
			},
			wantCode: "graph_proposal_invalid",
		},
		{
			name: "unknown proposal id",
			options: func(GraphProposalResult) GraphApplyOptions {
				return GraphApplyOptions{Proposal: "gprop-999999", Approval: "approval:evidence"}
			},
			wantCode: "graph_proposal_missing",
		},
		{
			name: "blank approval",
			options: func(proposal GraphProposalResult) GraphApplyOptions {
				return GraphApplyOptions{Proposal: proposal.ProposalID, Approval: " "}
			},
			wantCode: "graph_approval_required",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := initializedRepo(t)
			root, _ := DiscoverRoot(repo)
			writeWorkflowGraph(t, repo, validWorkflowGraph())
			writeGraphFile(t, repo, "graphs/candidate.yaml", candidateWorkflowGraph())
			proposal, err := ProposeWorkflowGraph(root, GraphProposeOptions{Patch: "graphs/candidate.yaml", Reason: "add ask phase"})
			if err != nil {
				t.Fatalf("ProposeWorkflowGraph() error = %v", err)
			}
			if tc.mutate != nil {
				tc.mutate(t, repo, proposal)
			}
			beforeGraph := readGraphTestText(t, filepath.Join(repo, WorkflowGraphDefaultPath))
			beforeEvents := runEventLines(t, repo)

			options := defaultGraphApplyOptions(proposal)
			if tc.options != nil {
				options = tc.options(proposal)
			}
			_, err = ApplyWorkflowGraph(root, options)
			assertProblemCode(t, err, tc.wantCode)
			if got := readGraphTestText(t, filepath.Join(repo, WorkflowGraphDefaultPath)); got != beforeGraph {
				t.Fatalf("graph mutated after rejected apply\nbefore=%s\nafter=%s", beforeGraph, got)
			}
			if afterEvents := runEventLines(t, repo); len(afterEvents) != len(beforeEvents) {
				t.Fatalf("events changed after rejected apply: before=%d after=%d", len(beforeEvents), len(afterEvents))
			}
		})
	}
}

func TestExportWorkflowGraphRendersMermaidAndPlantUML(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	writeWorkflowGraph(t, repo, strings.Replace(validWorkflowGraph(), `title: "Plan"`, `title: "Plan <draft> & \"Review\""`, 1))

	mermaid, err := ExportWorkflowGraph(root, GraphExportOptions{Format: "mermaid"})
	if err != nil {
		t.Fatalf("ExportWorkflowGraph(mermaid) error = %v", err)
	}
	if mermaid.Status != GraphStatusPass || mermaid.Format != "mermaid" || mermaid.SourceFile != WorkflowGraphDefaultPath || mermaid.SourceChecksum == "" || mermaid.Authoritative || mermaid.OutputPath != "" {
		t.Fatalf("mermaid result = %#v, want non-authoritative stdout export metadata", mermaid)
	}
	for _, want := range []string{"flowchart TD\n", `p001_plan["Plan &lt;draft&gt; &amp; 'Review' [plan]"]`, "p001_plan --> p002_implement", `g001_pre_implementation["gate: pre-implementation<br/>requires: plan, implement"]`, `a001_sot_change["approval: sot-change<br/>role: responsible-approver"]`} {
		if !strings.Contains(mermaid.Diagram, want) {
			t.Fatalf("mermaid diagram = %s, want %s", mermaid.Diagram, want)
		}
	}

	output := "docs/generated/workflow.puml"
	plantuml, err := ExportWorkflowGraph(root, GraphExportOptions{Format: "plantuml", Output: output})
	if err != nil {
		t.Fatalf("ExportWorkflowGraph(plantuml) error = %v", err)
	}
	if plantuml.Status != GraphStatusPass || plantuml.Format != "plantuml" || plantuml.OutputPath != output || plantuml.SourceChecksum != mermaid.SourceChecksum || plantuml.Authoritative {
		t.Fatalf("plantuml result = %#v, want non-authoritative file export metadata", plantuml)
	}
	for _, want := range []string{"@startuml\n", `rectangle "Plan <draft> & \"Review\" [plan]" as p001_plan`, "p001_plan --> p002_implement", `note "gate: pre-implementation\nrequires: plan, implement" as g001_pre_implementation`, "@enduml\n"} {
		if !strings.Contains(plantuml.Diagram, want) {
			t.Fatalf("plantuml diagram = %s, want %s", plantuml.Diagram, want)
		}
	}
	if got := readGraphTestText(t, filepath.Join(repo, filepath.FromSlash(output))); got != plantuml.Diagram {
		t.Fatalf("written export = %s, want diagram", got)
	}
}

func TestExportWorkflowGraphFailsClosedWithoutWritingInvalidGraph(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	writeWorkflowGraph(t, repo, strings.Replace(validWorkflowGraph(), `to: "implement"`, `to: "missing"`, 1))

	output := "docs/generated/workflow.mmd"
	result, err := ExportWorkflowGraph(root, GraphExportOptions{Format: "mermaid", Output: output})
	if err != nil {
		t.Fatalf("ExportWorkflowGraph() error = %v", err)
	}
	if result.Status != GraphStatusFail || result.Diagram != "" || !graphIssueNamed(result.ValidationSummary.Errors, "edge_to") {
		t.Fatalf("result = %#v, want validation failure without diagram", result)
	}
	if _, err := os.Stat(filepath.Join(repo, filepath.FromSlash(output))); !os.IsNotExist(err) {
		t.Fatalf("export output stat err = %v, want missing output", err)
	}
}

func TestExportWorkflowGraphRejectsInvalidOutputPaths(t *testing.T) {
	cases := []struct {
		name   string
		format string
		output string
		setup  func(t *testing.T, repo string, output string)
		code   string
	}{
		{name: "source graph path", format: "mermaid", output: WorkflowGraphDefaultPath, code: "graph_export_output_invalid"},
		{name: "parent path escape", format: "mermaid", output: "../workflow.mmd", code: "path_escape"},
		{name: "absolute path", format: "mermaid", output: filepath.Join(t.TempDir(), "workflow.mmd"), code: "absolute_path"},
		{name: "symlink escape", format: "mermaid", output: "docs/out/workflow.mmd", setup: func(t *testing.T, repo string, output string) {
			outside := t.TempDir()
			if err := os.MkdirAll(filepath.Join(repo, "docs"), 0o755); err != nil {
				t.Fatalf("mkdir docs: %v", err)
			}
			if err := os.Symlink(outside, filepath.Join(repo, "docs", "out")); err != nil {
				t.Fatalf("symlink output dir: %v", err)
			}
		}, code: "symlink_escape"},
		{name: "wrong extension", format: "plantuml", output: "docs/generated/workflow.mmd", code: "graph_export_output_invalid"},
		{name: "directory", format: "mermaid", output: "docs/generated/workflow.mmd", setup: func(t *testing.T, repo string, output string) {
			if err := os.MkdirAll(filepath.Join(repo, filepath.FromSlash(output)), 0o755); err != nil {
				t.Fatalf("mkdir output dir: %v", err)
			}
		}, code: "graph_export_output_invalid"},
		{name: "existing file", format: "mermaid", output: "docs/generated/workflow.mmd", setup: func(t *testing.T, repo string, output string) {
			writeGraphFile(t, repo, output, "existing\n")
		}, code: "graph_export_output_exists"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := initializedRepo(t)
			root, _ := DiscoverRoot(repo)
			writeWorkflowGraph(t, repo, validWorkflowGraph())
			if tc.setup != nil {
				tc.setup(t, repo, tc.output)
			}
			_, err := ExportWorkflowGraph(root, GraphExportOptions{Format: tc.format, Output: tc.output})
			if err == nil {
				t.Fatalf("ExportWorkflowGraph() succeeded, want %s", tc.code)
			}
			var problemErr *Problem
			if !errors.As(err, &problemErr) || problemErr.Code != tc.code {
				t.Fatalf("error = %#v, want code %s", err, tc.code)
			}
		})
	}
}

func defaultGraphApplyOptions(proposal GraphProposalResult) GraphApplyOptions {
	return GraphApplyOptions{Proposal: proposal.ProposalID, Approval: "approval:evidence"}
}

func writeWorkflowGraph(t *testing.T, repo string, body string) {
	t.Helper()
	writeGraphFile(t, repo, WorkflowGraphDefaultPath, body)
}

func writeGraphFile(t *testing.T, repo string, relative string, body string) {
	t.Helper()
	path := filepath.Join(repo, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir graph file parent: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write workflow graph: %v", err)
	}
}

func graphIssueNamed(issues []GraphIssue, name string) bool {
	for _, issue := range issues {
		if issue.Name == name {
			return true
		}
	}
	return false
}

func graphStringSliceContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func readGraphTestText(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func readGraphTestJSON(t *testing.T, path string, target any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if err := json.Unmarshal(data, target); err != nil {
		t.Fatalf("decode %s: %v\n%s", path, err, string(data))
	}
}

func updateGraphProposalRecord(t *testing.T, repo string, proposal GraphProposalResult, mutate func(*WorkflowGraphProposalRecord)) {
	t.Helper()
	var record WorkflowGraphProposalRecord
	path := filepath.Join(repo, filepath.FromSlash(proposal.ProposalPath))
	readGraphTestJSON(t, path, &record)
	mutate(&record)
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		t.Fatalf("encode proposal record: %v", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write proposal record: %v", err)
	}
}

func writeGraphProposalRecordRaw(t *testing.T, repo string, proposal GraphProposalResult, body string) {
	t.Helper()
	path := filepath.Join(repo, filepath.FromSlash(proposal.ProposalPath))
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write raw proposal record: %v", err)
	}
}

func minimalWorkflowGraph() string {
	return `version: "workflow-graph/v1"
graph_id: "graph-minimal"
metadata:
  project: "kkachi-test"
  created_by: "human"
  managed_by: "kah"
phases:
  - id: "plan"
    required: true
`
}

func validWorkflowGraph() string {
	return `version: "workflow-graph/v1"
graph_id: "graph-test"
metadata:
  project: "kkachi-test"
  created_by: "human"
  managed_by: "kah"
  source_template: "test-template"
  last_applied_event_id: "evt-000001"
phases:
  - id: "plan"
    title: "Plan"
    owner_layer: "khs"
    required: true
    evidence: ["plan.md", "checklist.md"]
  - id: "implement"
    title: "Implement"
    owner_layer: "khs"
    required: true
    evidence: ["diff.patch"]
edges:
  - from: "plan"
    to: "implement"
gates:
  - id: "pre-implementation"
    requires: ["plan", "implement"]
approvals:
  - scope: "sot-change"
    required_role: "responsible-approver"
proposals:
  policy: "proposal-first"
	`
}

func workflowGraphWithFeedbackIntake(body string) string {
	return body + validFeedbackIntakeSection()
}

func validFeedbackIntakeSection() string {
	return `feedback_intake:
  policy: "EXTERNAL_FEEDBACK_INTAKE"
  schema_version: "external-feedback-intake/v1"
  min_rounds: 1
  max_rounds: 5
  required_rounds: [1]
  optional_rounds: [2,3,4,5]
`
}

func staleFeedbackIntakeWorkflowGraph() string {
	return validWorkflowGraph() + `feedback_intake:
  policy: "EXTERNAL_FEEDBACK_INTAKE"
  schema_version: "external-feedback-intake/v1"
  min_rounds: 1
  max_rounds: 3
  required_rounds: [1]
  optional_rounds: [2,3]
`
}

func candidateWorkflowGraph() string {
	return `version: "workflow-graph/v1"
graph_id: "graph-test"
metadata:
  project: "kkachi-test"
  created_by: "human"
  managed_by: "kah"
  source_template: "test-template"
  last_applied_event_id: "evt-000001"
phases:
  - id: "plan"
    title: "Plan"
    owner_layer: "khs"
    required: true
    evidence: ["plan.md", "checklist.md"]
  - id: "ask"
    title: "Ask"
    owner_layer: "khs"
    required: true
    evidence: ["feedback-request.md"]
  - id: "implement"
    title: "Implement"
    owner_layer: "khs"
    required: false
    evidence: ["diff.patch"]
edges:
  - from: "plan"
    to: "ask"
  - from: "ask"
    to: "implement"
gates:
  - id: "pre-implementation"
    requires: ["plan", "ask", "implement"]
approvals:
  - scope: "sot-change"
    required_role: "required-reviewer"
proposals:
  policy: "proposal-first"
	`
}

func expandedWorkflowGraph() string {
	return `version: "workflow-graph/v1"
graph_id: "graph-test"
metadata:
  project: "kkachi-test"
  created_by: "human"
  managed_by: "kah"
  source_template: "test-template"
  last_applied_event_id: "evt-000001"
phases:
  - id: "plan"
    title: "Plan"
    owner_layer: "khs"
    required: true
    evidence: ["plan.md", "checklist.md"]
  - id: "ask"
    title: "Ask"
    owner_layer: "khs"
    required: true
    evidence: ["feedback-request.md"]
  - id: "implement"
    title: "Implement"
    owner_layer: "khs"
    required: true
    evidence: ["diff.patch"]
edges:
  - from: "plan"
    to: "ask"
  - from: "ask"
    to: "implement"
gates:
  - id: "pre-implementation"
    requires: ["plan", "implement"]
  - id: "post-implementation"
    requires: ["implement"]
approvals:
  - scope: "sot-change"
    required_role: "responsible-approver"
  - scope: "release"
    required_role: "maintainer"
proposals:
  policy: "proposal-first"
`
}
