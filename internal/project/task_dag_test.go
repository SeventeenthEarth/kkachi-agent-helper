package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateTaskDAGAcceptsSupportedSubset(t *testing.T) {
	repo := t.TempDir()
	workflow := filepath.Join(repo, "workflow.yaml")
	content := `# supported DAGSM-001 subset
schema_version: task-dag/v1
workflow_id: "demo"
nodes:
  - id: setup
    depends_on: []
    join: all_of
    required_outputs: ["out/setup.txt"]
  - id: build
    depends_on: [setup]
    join: all_of
    required_outputs:
      - "out/build.txt"
`
	if err := os.WriteFile(workflow, []byte(content), 0o600); err != nil {
		t.Fatalf("write workflow: %v", err)
	}

	result, err := ValidateTaskDAG(Root{Path: repo}, "workflow.yaml")
	if err != nil {
		t.Fatalf("ValidateTaskDAG returned error: %v", err)
	}
	if !result.OK || result.Status != "valid" || result.Reason != "task_dag_valid" {
		t.Fatalf("result = %#v, want valid task_dag_valid", result)
	}
	if result.WorkflowID != "demo" || result.SchemaVersion != "task-dag/v1" {
		t.Fatalf("identity = %q/%q, want demo/task-dag/v1", result.WorkflowID, result.SchemaVersion)
	}
	if len(result.Nodes) != 2 || result.Nodes[1].ID != "build" || len(result.Edges) != 1 || result.Edges[0].From != "setup" || result.Edges[0].To != "build" {
		t.Fatalf("nodes/edges = %#v/%#v, want setup->build", result.Nodes, result.Edges)
	}
}

func TestValidateTaskDAGReportsUnknownDependencyAndMissingRequiredOutputs(t *testing.T) {
	repo := t.TempDir()
	workflow := filepath.Join(repo, "workflow.yaml")
	content := `schema_version: task-dag/v1
workflow_id: demo
nodes:
  - id: build
    depends_on: [missing]
    join: all_of
`
	if err := os.WriteFile(workflow, []byte(content), 0o600); err != nil {
		t.Fatalf("write workflow: %v", err)
	}

	result, err := ValidateTaskDAG(Root{Path: repo}, "workflow.yaml")
	if err != nil {
		t.Fatalf("ValidateTaskDAG returned error: %v", err)
	}
	if result.OK || result.Status != "invalid" || result.Reason != "task_dag_unknown_dependency" {
		t.Fatalf("result = %#v, want invalid unknown dependency", result)
	}
	if !containsTaskDAGString(result.ReasonCodes, "task_dag_unknown_dependency") || !containsTaskDAGString(result.ReasonCodes, "node_required_output_missing") {
		t.Fatalf("reason_codes = %#v, want unknown dependency and missing required output", result.ReasonCodes)
	}
}

func TestValidateTaskDAGRejectsCyclesAndUnsafeOutputPaths(t *testing.T) {
	repo := t.TempDir()
	workflow := filepath.Join(repo, "workflow.yaml")
	content := `schema_version: task-dag/v1
workflow_id: demo
nodes:
  - id: a
    depends_on: [b]
    join: all_of
    required_outputs: ["../escape.txt"]
  - id: b
    depends_on: [a]
    join: any_of
    required_outputs: ["out/b.txt"]
`
	if err := os.WriteFile(workflow, []byte(content), 0o600); err != nil {
		t.Fatalf("write workflow: %v", err)
	}

	result, err := ValidateTaskDAG(Root{Path: repo}, "workflow.yaml")
	if err != nil {
		t.Fatalf("ValidateTaskDAG returned error: %v", err)
	}
	for _, code := range []string{"task_dag_cycle_detected", "task_dag_unsupported_join", "task_dag_invalid_schema"} {
		if !containsTaskDAGString(result.ReasonCodes, code) {
			t.Fatalf("reason_codes = %#v, want %s", result.ReasonCodes, code)
		}
	}
}

func TestValidateTaskDAGPreservesQuotedCommasAndCommentHashes(t *testing.T) {
	repo := t.TempDir()
	workflow := filepath.Join(repo, "workflow.yaml")
	content := `schema_version: task-dag/v1
workflow_id: 'demo # not a comment'
nodes:
  - id: setup
    depends_on: [] # trailing comment
    join: all_of
    required_outputs: ["out/setup,#1.txt", 'out/setup, #2.txt']
  - id: build
    depends_on: ["setup"]
    join: all_of
    required_outputs:
      - "out/build # hash.txt"
`
	if err := os.WriteFile(workflow, []byte(content), 0o600); err != nil {
		t.Fatalf("write workflow: %v", err)
	}

	result, err := ValidateTaskDAG(Root{Path: repo}, "workflow.yaml")
	if err != nil {
		t.Fatalf("ValidateTaskDAG returned error: %v", err)
	}
	if !result.OK {
		t.Fatalf("result = %#v, want valid quoted commas/comment hashes", result)
	}
	if result.WorkflowID != "demo # not a comment" {
		t.Fatalf("workflow_id = %q, want quoted hash preserved", result.WorkflowID)
	}
	if got := result.Nodes[0].RequiredOutputs; len(got) != 2 || got[0] != "out/setup,#1.txt" || got[1] != "out/setup, #2.txt" {
		t.Fatalf("required_outputs = %#v, want quoted comma items preserved", got)
	}
	if got := result.Nodes[1].RequiredOutputs; len(got) != 1 || got[0] != "out/build # hash.txt" {
		t.Fatalf("block required_outputs = %#v, want quoted hash preserved", got)
	}
}

func TestValidateTaskDAGReportsMissingWorkflowFile(t *testing.T) {
	repo := t.TempDir()

	result, err := ValidateTaskDAG(Root{Path: repo}, "missing.yaml")
	if err != nil {
		t.Fatalf("ValidateTaskDAG returned error: %v", err)
	}
	if result.OK || result.Status != "invalid" || result.Reason != "task_dag_missing" {
		t.Fatalf("result = %#v, want invalid task_dag_missing", result)
	}
	if len(result.Diagnostics) != 1 || result.Diagnostics[0].Field != "file" || result.Diagnostics[0].Path != "missing.yaml" {
		t.Fatalf("diagnostics = %#v, want file-scoped missing diagnostic", result.Diagnostics)
	}
}

func containsTaskDAGString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
