package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SeventeenthEarth/kkachi-agent-helper/internal/project"
)

func TestGraphValidateAndExplainJSON(t *testing.T) {
	repo := tempGitRepo(t)
	writeCLIGraph(t, repo, cliValidWorkflowGraph())

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := runWithOptions([]string{"graph", "validate", "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo})
	if exitCode != ExitOK {
		t.Fatalf("exitCode = %d, want %d stderr=%s", exitCode, ExitOK, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	assertNoHumanDecoration(t, stdout.String())
	var validation project.GraphValidationResult
	if err := json.Unmarshal(stdout.Bytes(), &validation); err != nil {
		t.Fatalf("graph validate output is not JSON: %v\n%s", err, stdout.String())
	}
	if validation.Status != project.GraphStatusPass || validation.File != project.WorkflowGraphDefaultPath || validation.Checksum == "" || validation.EffectiveSource != "project_file" {
		t.Fatalf("validation = %#v, want passing graph result", validation)
	}

	stdout.Reset()
	stderr.Reset()
	exitCode = runWithOptions([]string{"graph", "explain", "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo})
	if exitCode != ExitOK {
		t.Fatalf("exitCode = %d, want %d stderr=%s", exitCode, ExitOK, stderr.String())
	}
	var explained project.GraphExplanationResult
	if err := json.Unmarshal(stdout.Bytes(), &explained); err != nil {
		t.Fatalf("graph explain output is not JSON: %v\n%s", err, stdout.String())
	}
	if explained.Status != project.GraphStatusPass || explained.GraphVersion != project.WorkflowGraphSchemaVersion || len(explained.Phases) != 2 || len(explained.Edges) != 1 {
		t.Fatalf("explanation = %#v, want graph projection", explained)
	}
}

func TestGraphValidateExplicitFileJSON(t *testing.T) {
	repo := tempGitRepo(t)
	relative := "docs/graphs/candidate-workflow.yaml"
	writeCLIGraphFile(t, repo, relative, cliValidWorkflowGraph())

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := runWithOptions([]string{"graph", "validate", "--file", relative, "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo})
	if exitCode != ExitOK {
		t.Fatalf("exitCode = %d, want %d stderr=%s", exitCode, ExitOK, stderr.String())
	}
	var validation project.GraphValidationResult
	if err := json.Unmarshal(stdout.Bytes(), &validation); err != nil {
		t.Fatalf("graph validate output is not JSON: %v\n%s", err, stdout.String())
	}
	if validation.Status != project.GraphStatusPass || validation.File != relative || validation.Checksum == "" {
		t.Fatalf("validation = %#v, want passing explicit file graph result", validation)
	}
}

func TestGraphDiffJSONAndHumanOutput(t *testing.T) {
	repo := tempGitRepo(t)
	from := "graphs/from.yaml"
	to := "graphs/to.yaml"
	writeCLIGraphFile(t, repo, from, cliValidWorkflowGraph())
	writeCLIGraphFile(t, repo, to, cliCandidateWorkflowGraph())

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := runWithOptions([]string{"graph", "diff", "--from", from, "--to", to, "--semantic", "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo})
	if exitCode != ExitOK {
		t.Fatalf("exitCode = %d, want %d stderr=%s", exitCode, ExitOK, stderr.String())
	}
	var diff project.GraphDiffResult
	if err := json.Unmarshal(stdout.Bytes(), &diff); err != nil {
		t.Fatalf("graph diff output is not JSON: %v\n%s", err, stdout.String())
	}
	if diff.Status != project.GraphStatusPass || len(diff.ChangedPhases.Added) != 1 || len(diff.ChangedEdges.Added) != 2 || !diff.RequiresApproval {
		t.Fatalf("diff = %#v, want semantic changes requiring approval", diff)
	}

	stdout.Reset()
	stderr.Reset()
	exitCode = runWithOptions([]string{"graph", "diff", "--from", from, "--to", to}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo})
	if exitCode != ExitOK {
		t.Fatalf("human exitCode = %d, want %d stderr=%s", exitCode, ExitOK, stderr.String())
	}
	for _, want := range []string{"graph diff: pass", "changed_phases: added=1", "edge added ask -> implement", "requires_approval: true", "dependencies_changed"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("human diff output = %q, want %q", stdout.String(), want)
		}
	}
}

func TestGraphProposeRecordsProposalJSONAndHumanOutput(t *testing.T) {
	repo := tempGitRepo(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := runWithOptions(projectInitArgs(), &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("project init exit = %d stderr=%s", code, stderr.String())
	}
	writeCLIGraph(t, repo, cliValidWorkflowGraph())
	patch := "graphs/candidate.yaml"
	writeCLIGraphFile(t, repo, patch, cliCandidateWorkflowGraph())
	beforeGraph := readCLITestText(t, filepath.Join(repo, project.WorkflowGraphDefaultPath))
	stdout.Reset()
	stderr.Reset()

	exitCode := runWithOptions([]string{"graph", "propose", "--patch", patch, "--reason", "add ask phase", "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo})
	if exitCode != ExitOK {
		t.Fatalf("exitCode = %d, want %d stderr=%s stdout=%s", exitCode, ExitOK, stderr.String(), stdout.String())
	}
	var proposal project.GraphProposalResult
	if err := json.Unmarshal(stdout.Bytes(), &proposal); err != nil {
		t.Fatalf("graph propose output is not JSON: %v\n%s", err, stdout.String())
	}
	if proposal.ProposalID != "gprop-000001" || proposal.ProposalPath != ".kkachi/graph/proposals/gprop-000001.json" || proposal.SemanticDiffRef != proposal.ProposalPath+"#semantic_diff" || !proposal.ApprovalRequired {
		t.Fatalf("proposal = %#v, want first proposal result", proposal)
	}
	if got := readCLITestText(t, filepath.Join(repo, project.WorkflowGraphDefaultPath)); got != beforeGraph {
		t.Fatalf("graph file mutated\nbefore=%s\nafter=%s", beforeGraph, got)
	}
	proposalBytes := readCLITestText(t, filepath.Join(repo, filepath.FromSlash(proposal.ProposalPath)))
	for _, want := range []string{`"proposal_id": "gprop-000001"`, `"semantic_diff": {`, `"approval_required": true`} {
		if !strings.Contains(proposalBytes, want) {
			t.Fatalf("proposal record = %s, want %s", proposalBytes, want)
		}
	}

	stdout.Reset()
	stderr.Reset()
	exitCode = runWithOptions([]string{"graph", "propose", "--patch", patch, "--reason", "second proposal"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo})
	if exitCode != ExitOK {
		t.Fatalf("human exitCode = %d, want %d stderr=%s", exitCode, ExitOK, stderr.String())
	}
	for _, want := range []string{"graph proposal: pass", "proposal_id: gprop-000002", "semantic_diff_ref: .kkachi/graph/proposals/gprop-000002.json#semantic_diff", "approval_required: true"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("human proposal output = %q, want %q", stdout.String(), want)
		}
	}
}

func TestGraphValidationFailureEmitsResultOnStdout(t *testing.T) {
	repo := tempGitRepo(t)
	writeCLIGraph(t, repo, strings.Replace(cliValidWorkflowGraph(), `to: "implement"`, `to: "missing"`, 1))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := runWithOptions([]string{"graph", "validate", "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo})
	if exitCode != ExitSafety {
		t.Fatalf("exitCode = %d, want %d", exitCode, ExitSafety)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	var validation project.GraphValidationResult
	if err := json.Unmarshal(stdout.Bytes(), &validation); err != nil {
		t.Fatalf("graph validate failure output is not JSON: %v\n%s", err, stdout.String())
	}
	if validation.Status != project.GraphStatusFail || !cliGraphIssueNamed(validation.Errors, "edge_to") {
		t.Fatalf("validation = %#v, want edge_to failure", validation)
	}
}

func TestGraphHumanOutput(t *testing.T) {
	repo := tempGitRepo(t)
	writeCLIGraph(t, repo, cliValidWorkflowGraph())

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := runWithOptions([]string{"graph", "validate"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo})
	if exitCode != ExitOK {
		t.Fatalf("exitCode = %d, want %d stderr=%s", exitCode, ExitOK, stderr.String())
	}
	for _, want := range []string{"graph validation: pass", "effective_source: project_file", "errors: 0", "next_action:"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("human validate output = %q, want %q", stdout.String(), want)
		}
	}

	writeCLIGraph(t, repo, strings.Replace(cliValidWorkflowGraph(), `to: "implement"`, `to: "missing"`, 1))
	stdout.Reset()
	stderr.Reset()
	exitCode = runWithOptions([]string{"graph", "explain"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo})
	if exitCode != ExitSafety {
		t.Fatalf("exitCode = %d, want %d", exitCode, ExitSafety)
	}
	for _, want := range []string{"graph explanation: fail", "errors: ", "edge_to", "edge target phase", "pending_proposals: 0", "next_action:"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("human explain output = %q, want %q", stdout.String(), want)
		}
	}
}

func TestGraphRejectsUsageErrorsOnStderr(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		wantCode string
	}{
		{name: "missing subcommand", args: []string{"graph", "--json"}, wantCode: "graph_subcommand_required"},
		{name: "unknown subcommand", args: []string{"graph", "render", "--json"}, wantCode: "graph_subcommand_unknown"},
		{name: "unknown option", args: []string{"graph", "validate", "--unknown", "--json"}, wantCode: "unknown_option"},
		{name: "missing file value", args: []string{"graph", "validate", "--file", "--json"}, wantCode: "missing_option_value"},
		{name: "empty file value", args: []string{"graph", "validate", "--file", "", "--json"}, wantCode: "missing_option_value"},
		{name: "duplicate file option", args: []string{"graph", "validate", "--file", ".kkachi-workflow.yaml", "--file", "other.yaml", "--json"}, wantCode: "duplicate_option"},
		{name: "missing diff from", args: []string{"graph", "diff", "--to", "candidate.yaml", "--json"}, wantCode: "missing_required_option"},
		{name: "missing diff to", args: []string{"graph", "diff", "--from", "base.yaml", "--json"}, wantCode: "missing_required_option"},
		{name: "empty diff from", args: []string{"graph", "diff", "--from", "", "--to", "candidate.yaml", "--json"}, wantCode: "missing_option_value"},
		{name: "empty diff to", args: []string{"graph", "diff", "--from", "base.yaml", "--to", "", "--json"}, wantCode: "missing_option_value"},
		{name: "duplicate diff from", args: []string{"graph", "diff", "--from", "base.yaml", "--from", "other.yaml", "--to", "candidate.yaml", "--json"}, wantCode: "duplicate_option"},
		{name: "duplicate diff to", args: []string{"graph", "diff", "--from", "base.yaml", "--to", "candidate.yaml", "--to", "other.yaml", "--json"}, wantCode: "duplicate_option"},
		{name: "duplicate diff semantic", args: []string{"graph", "diff", "--from", "base.yaml", "--to", "candidate.yaml", "--semantic", "--semantic", "--json"}, wantCode: "duplicate_option"},
		{name: "unknown diff option", args: []string{"graph", "diff", "--from", "base.yaml", "--to", "candidate.yaml", "--patch", "other.yaml", "--json"}, wantCode: "unknown_option"},
		{name: "missing propose patch", args: []string{"graph", "propose", "--reason", "test", "--json"}, wantCode: "missing_required_option"},
		{name: "missing propose reason", args: []string{"graph", "propose", "--patch", "candidate.yaml", "--json"}, wantCode: "missing_required_option"},
		{name: "empty propose patch", args: []string{"graph", "propose", "--patch", "", "--reason", "test", "--json"}, wantCode: "missing_option_value"},
		{name: "empty propose reason", args: []string{"graph", "propose", "--patch", "candidate.yaml", "--reason", "", "--json"}, wantCode: "missing_option_value"},
		{name: "duplicate propose patch", args: []string{"graph", "propose", "--patch", "candidate.yaml", "--patch", "other.yaml", "--reason", "test", "--json"}, wantCode: "duplicate_option"},
		{name: "duplicate propose reason", args: []string{"graph", "propose", "--patch", "candidate.yaml", "--reason", "test", "--reason", "again", "--json"}, wantCode: "duplicate_option"},
		{name: "unknown propose option", args: []string{"graph", "propose", "--patch", "candidate.yaml", "--reason", "test", "--from", "base.yaml", "--json"}, wantCode: "unknown_option"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := tempGitRepo(t)
			var stdout bytes.Buffer
			var stderr bytes.Buffer

			exitCode := runWithOptions(tc.args, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo})
			if exitCode != ExitUsage {
				t.Fatalf("exitCode = %d, want %d", exitCode, ExitUsage)
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
			env := decodeErrorEnvelope(t, stderr.Bytes())
			if env.Error.Code != tc.wantCode {
				t.Fatalf("error code = %q, want %s", env.Error.Code, tc.wantCode)
			}
		})
	}
}

func writeCLIGraph(t *testing.T, repo string, body string) {
	t.Helper()
	writeCLIGraphFile(t, repo, project.WorkflowGraphDefaultPath, body)
}

func writeCLIGraphFile(t *testing.T, repo string, relative string, body string) {
	t.Helper()
	path := filepath.Join(repo, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir workflow graph parent: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write workflow graph: %v", err)
	}
}

func cliGraphIssueNamed(issues []project.GraphIssue, name string) bool {
	for _, issue := range issues {
		if issue.Name == name {
			return true
		}
	}
	return false
}

func readCLITestText(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func cliValidWorkflowGraph() string {
	return `version: "workflow-graph/v1"
graph_id: "graph-cli"
metadata:
  project: "kkachi-cli"
  created_by: "human"
  managed_by: "kah"
phases:
  - id: "plan"
    title: "Plan"
    owner_layer: "khs"
    required: true
    evidence: ["plan.md"]
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
    requires: ["plan"]
approvals:
  - scope: "sot-change"
    required_role: "responsible-approver"
proposals:
  policy: "proposal-first"
	`
}

func cliCandidateWorkflowGraph() string {
	return `version: "workflow-graph/v1"
graph_id: "graph-cli"
metadata:
  project: "kkachi-cli"
  created_by: "human"
  managed_by: "kah"
phases:
  - id: "plan"
    title: "Plan"
    owner_layer: "khs"
    required: true
    evidence: ["plan.md"]
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
    requires: ["plan", "ask"]
approvals:
  - scope: "sot-change"
    required_role: "required-reviewer"
proposals:
  policy: "proposal-first"
`
}
