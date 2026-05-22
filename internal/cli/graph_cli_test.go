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
