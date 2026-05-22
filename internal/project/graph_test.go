package project

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
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

func TestValidateWorkflowGraphMissingAndForbiddenSources(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)

	missing := ValidateWorkflowGraph(root, GraphOptions{})
	if missing.Status != GraphStatusFail || !graphIssueNamed(missing.Errors, "graph_file") {
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
