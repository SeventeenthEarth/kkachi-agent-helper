package project

import (
	"encoding/json"
	"os"
	"path/filepath"
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
