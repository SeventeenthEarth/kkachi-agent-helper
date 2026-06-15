package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWorkflowInstanceCreateReadyAndCompleteSequence(t *testing.T) {
	repo, root, runID := workflowInstanceTestRun(t)
	writeWorkflowFixture(t, repo, `schema_version: task-dag/v1
workflow_id: demo
nodes:
  - id: setup
    depends_on: []
    join: all_of
    required_outputs: ["out/setup.txt"]
  - id: build
    depends_on: [setup]
    join: all_of
    required_outputs: ["out/build.txt"]
`)

	created, err := CreateWorkflowInstance(root, WorkflowCreateOptions{RunID: runID, File: "workflow.yaml", Now: testRunNow(4)})
	if err != nil {
		t.Fatalf("CreateWorkflowInstance() error = %v", err)
	}
	if !created.OK || created.Reason != "workflow_instance_created" || created.Instance == nil || created.Instance.Revision != 1 {
		t.Fatalf("created = %#v, want created revision 1", created)
	}
	if len(created.Ready) != 1 || created.Ready[0].ID != "setup" {
		t.Fatalf("ready after create = %#v, want setup", created.Ready)
	}

	blocked, err := StartWorkflowNode(root, WorkflowNodeOptions{RunID: runID, NodeID: "build", Now: testRunNow(5)})
	if err != nil {
		t.Fatalf("StartWorkflowNode(build) error = %v", err)
	}
	if blocked.OK || blocked.Reason != "node_dependency_unsatisfied" {
		t.Fatalf("blocked start = %#v, want node_dependency_unsatisfied", blocked)
	}

	started, err := StartWorkflowNode(root, WorkflowNodeOptions{RunID: runID, NodeID: "setup", ExpectedRevision: intPtr(1), Now: testRunNow(6)})
	if err != nil {
		t.Fatalf("StartWorkflowNode(setup) error = %v", err)
	}
	if !started.OK || started.Instance.Nodes[0].State != WorkflowNodeRunning || started.Instance.Revision != 2 || started.EventID == "" {
		t.Fatalf("started = %#v, want setup running revision 2 with event", started)
	}

	missing, err := CompleteWorkflowNode(root, WorkflowNodeOptions{RunID: runID, NodeID: "setup", Evidence: "missing-output-check", Now: testRunNow(7)})
	if err != nil {
		t.Fatalf("CompleteWorkflowNode(setup missing output) error = %v", err)
	}
	if missing.OK || missing.Reason != "node_required_output_missing" {
		t.Fatalf("missing = %#v, want node_required_output_missing", missing)
	}

	mustWriteText(t, filepath.Join(repo, "out", "setup.txt"), "done\n")
	completed, err := CompleteWorkflowNode(root, WorkflowNodeOptions{RunID: runID, NodeID: "setup", Evidence: "out/setup.txt", ExpectedRevision: intPtr(2), Now: testRunNow(8)})
	if err != nil {
		t.Fatalf("CompleteWorkflowNode(setup) error = %v", err)
	}
	if !completed.OK || completed.Instance.Nodes[0].State != WorkflowNodeSucceeded || completed.Instance.Revision != 3 {
		t.Fatalf("completed = %#v, want setup succeeded revision 3", completed)
	}
	if len(completed.Ready) != 1 || completed.Ready[0].ID != "build" {
		t.Fatalf("ready after setup = %#v, want build", completed.Ready)
	}
	shown, err := ShowWorkflowInstance(root, WorkflowRunOptions{RunID: runID})
	if err != nil {
		t.Fatalf("ShowWorkflowInstance() error = %v", err)
	}
	if shown.Instance.Nodes[0].LastTransitionEventID == "" || shown.Instance.Nodes[0].Evidence[0] != "out/setup.txt" {
		t.Fatalf("shown setup = %#v, want event id and evidence", shown.Instance.Nodes[0])
	}
}

func TestWorkflowInstanceFanOutAllOfAndStaleRevision(t *testing.T) {
	repo, root, runID := workflowInstanceTestRun(t)
	writeWorkflowFixture(t, repo, `schema_version: task-dag/v1
workflow_id: fan
nodes:
  - id: setup
    depends_on: []
    join: all_of
    required_outputs: ["out/setup.txt"]
  - id: lint
    depends_on: [setup]
    join: all_of
    required_outputs: ["out/lint.txt"]
  - id: test
    depends_on: [setup]
    join: all_of
    required_outputs: ["out/test.txt"]
  - id: final
    depends_on: [lint, test]
    join: all_of
    required_outputs: ["out/final.txt"]
`)
	created, err := CreateWorkflowInstance(root, WorkflowCreateOptions{RunID: runID, File: "workflow.yaml", Now: testRunNow(4)})
	if err != nil {
		t.Fatalf("CreateWorkflowInstance() error = %v", err)
	}
	if len(created.Ready) != 1 || created.Ready[0].ID != "setup" {
		t.Fatalf("ready after create = %#v, want setup", created.Ready)
	}
	if _, err := StartWorkflowNode(root, WorkflowNodeOptions{RunID: runID, NodeID: "setup", ExpectedRevision: intPtr(1), Now: testRunNow(5)}); err != nil {
		t.Fatalf("start setup: %v", err)
	}
	mustWriteText(t, filepath.Join(repo, "out", "setup.txt"), "done\n")
	completed, err := CompleteWorkflowNode(root, WorkflowNodeOptions{RunID: runID, NodeID: "setup", ExpectedRevision: intPtr(2), Now: testRunNow(6)})
	if err != nil {
		t.Fatalf("complete setup: %v", err)
	}
	if got := workflowReadyIDs(completed.Ready); len(got) != 2 || got[0] != "lint" || got[1] != "test" {
		t.Fatalf("ready after setup = %#v, want lint/test", got)
	}
	stale, err := StartWorkflowNode(root, WorkflowNodeOptions{RunID: runID, NodeID: "lint", ExpectedRevision: intPtr(2), Now: testRunNow(7)})
	if err != nil {
		t.Fatalf("stale start error = %v", err)
	}
	if stale.OK || stale.Reason != "workflow_instance_stale" {
		t.Fatalf("stale = %#v, want workflow_instance_stale", stale)
	}
}

func TestWorkflowInstanceRejectsEscapingRequiredOutput(t *testing.T) {
	repo, root, runID := workflowInstanceTestRun(t)
	writeWorkflowFixture(t, repo, `schema_version: task-dag/v1
workflow_id: escape
nodes:
  - id: setup
    depends_on: []
    join: all_of
    required_outputs: ["../outside.txt"]
`)
	result, err := CreateWorkflowInstance(root, WorkflowCreateOptions{RunID: runID, File: "workflow.yaml", Now: testRunNow(4)})
	if err != nil {
		t.Fatalf("CreateWorkflowInstance() error = %v", err)
	}
	if result.OK || result.Reason != "task_dag_invalid_schema" {
		t.Fatalf("result = %#v, want task_dag_invalid_schema", result)
	}
}

func TestWorkflowInstanceRejectsUnsafeCompletionEvidenceAndBlocksNode(t *testing.T) {
	repo, root, runID := workflowInstanceTestRun(t)
	writeWorkflowFixture(t, repo, `schema_version: task-dag/v1
workflow_id: safety
nodes:
  - id: setup
    depends_on: []
    join: all_of
    required_outputs: ["out/setup.txt"]
  - id: build
    depends_on: [setup]
    join: all_of
    required_outputs: ["out/build.txt"]
`)
	if _, err := CreateWorkflowInstance(root, WorkflowCreateOptions{RunID: runID, File: "workflow.yaml", Now: testRunNow(4)}); err != nil {
		t.Fatalf("create workflow instance: %v", err)
	}
	if _, err := StartWorkflowNode(root, WorkflowNodeOptions{RunID: runID, NodeID: "setup", ExpectedRevision: intPtr(1), Now: testRunNow(5)}); err != nil {
		t.Fatalf("start setup: %v", err)
	}
	mustWriteText(t, filepath.Join(repo, "out", "setup.txt"), "done\n")
	unsafe, err := CompleteWorkflowNode(root, WorkflowNodeOptions{RunID: runID, NodeID: "setup", Evidence: "../outside.txt", ExpectedRevision: intPtr(2), Now: testRunNow(6)})
	if err != nil {
		t.Fatalf("complete with unsafe evidence error = %v", err)
	}
	if unsafe.OK || unsafe.Reason != "node_evidence_unsafe" {
		t.Fatalf("unsafe = %#v, want node_evidence_unsafe", unsafe)
	}
	completed, err := CompleteWorkflowNode(root, WorkflowNodeOptions{RunID: runID, NodeID: "setup", Evidence: "out/setup.txt", ExpectedRevision: intPtr(2), Now: testRunNow(7)})
	if err != nil {
		t.Fatalf("complete setup: %v", err)
	}
	if !completed.OK || completed.Instance.Revision != 3 {
		t.Fatalf("completed = %#v, want pass revision 3", completed)
	}
	blocked, err := BlockWorkflowNode(root, WorkflowNodeOptions{RunID: runID, NodeID: "build", Reason: "waiting for reviewer", ExpectedRevision: intPtr(3), Now: testRunNow(8)})
	if err != nil {
		t.Fatalf("block build: %v", err)
	}
	if !blocked.OK || blocked.Instance.Nodes[1].State != WorkflowNodeBlocked || blocked.Instance.Nodes[1].BlockedReason != "waiting for reviewer" {
		t.Fatalf("blocked = %#v, want build blocked with reason", blocked)
	}
	if len(blocked.Ready) != 0 {
		t.Fatalf("ready after block = %#v, want none", blocked.Ready)
	}
}

func workflowInstanceTestRun(t *testing.T) (string, Root, string) {
	t.Helper()
	repo := initializedRepo(t)
	root, err := DiscoverRoot(repo)
	if err != nil {
		t.Fatalf("DiscoverRoot() error = %v", err)
	}
	created, err := CreateRun(root, deterministicCreateRunOptions())
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	return repo, root, created.Metadata.RunID
}

func writeWorkflowFixture(t *testing.T, repo string, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repo, "workflow.yaml"), []byte(content), 0o600); err != nil {
		t.Fatalf("write workflow fixture: %v", err)
	}
}

func mustWriteText(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func intPtr(value int) *int { return &value }

func workflowReadyIDs(nodes []WorkflowReadyNode) []string {
	ids := make([]string, 0, len(nodes))
	for _, node := range nodes {
		ids = append(ids, node.ID)
	}
	return ids
}
