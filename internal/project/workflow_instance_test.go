package project

import (
	"bufio"
	"encoding/json"
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

func TestStartWorkflowNodeRecordsStrictLedgerPayload(t *testing.T) {
	repo, root, runID := workflowInstanceTestRun(t)
	writeWorkflowFixture(t, repo, strictLedgerWorkflowYAML())
	if _, err := CreateWorkflowInstance(root, WorkflowCreateOptions{RunID: runID, File: "workflow.yaml", Now: testRunNow(4)}); err != nil {
		t.Fatalf("create workflow instance: %v", err)
	}

	started, err := StartWorkflowNode(root, WorkflowNodeOptions{RunID: runID, NodeID: "setup", ExpectedRevision: intPtr(1), Now: testRunNow(5)})
	if err != nil {
		t.Fatalf("StartWorkflowNode() error = %v", err)
	}
	if !started.OK {
		t.Fatalf("started = %#v, want pass", started)
	}

	event := readWorkflowEventByID(t, repo, started.EventID)
	assertStrictLedgerPayload(t, event, runID, "strict-ledger", "setup", "start", 1, 2, WorkflowNodePending, WorkflowNodeRunning)
	assertPayloadStringSlice(t, event.Payload, "ready_before", []string{"setup"})
	assertPayloadStringSlice(t, event.Payload, "ready_after", []string{})
	assertPayloadStringMap(t, event.Payload, "dependency_states", map[string]string{})
}

func TestCompleteWorkflowNodeRecordsStrictLedgerPayload(t *testing.T) {
	repo, root, runID := workflowInstanceTestRun(t)
	writeWorkflowFixture(t, repo, strictLedgerWorkflowYAML())
	if _, err := CreateWorkflowInstance(root, WorkflowCreateOptions{RunID: runID, File: "workflow.yaml", Now: testRunNow(4)}); err != nil {
		t.Fatalf("create workflow instance: %v", err)
	}
	if _, err := StartWorkflowNode(root, WorkflowNodeOptions{RunID: runID, NodeID: "setup", ExpectedRevision: intPtr(1), Now: testRunNow(5)}); err != nil {
		t.Fatalf("start setup: %v", err)
	}
	mustWriteText(t, filepath.Join(repo, "out", "setup.txt"), "done\n")

	completed, err := CompleteWorkflowNode(root, WorkflowNodeOptions{RunID: runID, NodeID: "setup", Evidence: "out/setup.txt", ExpectedRevision: intPtr(2), Now: testRunNow(6)})
	if err != nil {
		t.Fatalf("CompleteWorkflowNode() error = %v", err)
	}
	if !completed.OK {
		t.Fatalf("completed = %#v, want pass", completed)
	}

	event := readWorkflowEventByID(t, repo, completed.EventID)
	assertStrictLedgerPayload(t, event, runID, "strict-ledger", "setup", "complete", 2, 3, WorkflowNodeRunning, WorkflowNodeSucceeded)
	assertPayloadStringSlice(t, event.Payload, "ready_before", []string{})
	assertPayloadStringSlice(t, event.Payload, "ready_after", []string{"build"})
}

func TestBlockWorkflowNodeRecordsStrictLedgerPayload(t *testing.T) {
	repo, root, runID := workflowInstanceTestRun(t)
	writeWorkflowFixture(t, repo, strictLedgerWorkflowYAML())
	if _, err := CreateWorkflowInstance(root, WorkflowCreateOptions{RunID: runID, File: "workflow.yaml", Now: testRunNow(4)}); err != nil {
		t.Fatalf("create workflow instance: %v", err)
	}

	blocked, err := BlockWorkflowNode(root, WorkflowNodeOptions{RunID: runID, NodeID: "setup", Reason: "blocked by review", ExpectedRevision: intPtr(1), Now: testRunNow(5)})
	if err != nil {
		t.Fatalf("BlockWorkflowNode() error = %v", err)
	}
	if !blocked.OK {
		t.Fatalf("blocked = %#v, want pass", blocked)
	}

	event := readWorkflowEventByID(t, repo, blocked.EventID)
	assertStrictLedgerPayload(t, event, runID, "strict-ledger", "setup", "block", 1, 2, WorkflowNodePending, WorkflowNodeBlocked)
}

func TestWorkflowNodeRejectsInvalidTransitionsWithoutMutation(t *testing.T) {
	tests := []struct {
		name       string
		mutate     func(t *testing.T, repo string, root Root, runID string)
		action     func(root Root, runID string) (WorkflowInstanceResult, error)
		wantReason string
		wantError  string
	}{
		{
			name: "stale expected revision",
			action: func(root Root, runID string) (WorkflowInstanceResult, error) {
				return StartWorkflowNode(root, WorkflowNodeOptions{RunID: runID, NodeID: "setup", ExpectedRevision: intPtr(2), Now: testRunNow(5)})
			},
			wantReason: "workflow_instance_stale",
		},
		{
			name: "unknown node",
			action: func(root Root, runID string) (WorkflowInstanceResult, error) {
				return StartWorkflowNode(root, WorkflowNodeOptions{RunID: runID, NodeID: "missing", ExpectedRevision: intPtr(1), Now: testRunNow(5)})
			},
			wantReason: "node_unknown",
		},
		{
			name: "non-ready dependency start",
			action: func(root Root, runID string) (WorkflowInstanceResult, error) {
				return StartWorkflowNode(root, WorkflowNodeOptions{RunID: runID, NodeID: "build", ExpectedRevision: intPtr(1), Now: testRunNow(5)})
			},
			wantReason: "node_dependency_unsatisfied",
		},
		{
			name: "complete without running start",
			action: func(root Root, runID string) (WorkflowInstanceResult, error) {
				return CompleteWorkflowNode(root, WorkflowNodeOptions{RunID: runID, NodeID: "setup", ExpectedRevision: intPtr(1), Now: testRunNow(5)})
			},
			wantReason: "node_transition_invalid",
		},
		{
			name: "downstream complete before upstream",
			action: func(root Root, runID string) (WorkflowInstanceResult, error) {
				return CompleteWorkflowNode(root, WorkflowNodeOptions{RunID: runID, NodeID: "build", ExpectedRevision: intPtr(1), Now: testRunNow(5)})
			},
			wantReason: "node_transition_invalid",
		},
		{
			name: "succeeded-node restart",
			mutate: func(t *testing.T, repo string, root Root, runID string) {
				t.Helper()
				completeSetupForStrictLedgerTest(t, repo, root, runID)
			},
			action: func(root Root, runID string) (WorkflowInstanceResult, error) {
				return StartWorkflowNode(root, WorkflowNodeOptions{RunID: runID, NodeID: "setup", ExpectedRevision: intPtr(3), Now: testRunNow(8)})
			},
			wantReason: "node_transition_invalid",
		},
		{
			name: "block succeeded node",
			mutate: func(t *testing.T, repo string, root Root, runID string) {
				t.Helper()
				completeSetupForStrictLedgerTest(t, repo, root, runID)
			},
			action: func(root Root, runID string) (WorkflowInstanceResult, error) {
				return BlockWorkflowNode(root, WorkflowNodeOptions{RunID: runID, NodeID: "setup", Reason: "too late", ExpectedRevision: intPtr(3), Now: testRunNow(8)})
			},
			wantReason: "node_transition_invalid",
		},
		{
			name: "empty block reason",
			action: func(root Root, runID string) (WorkflowInstanceResult, error) {
				return BlockWorkflowNode(root, WorkflowNodeOptions{RunID: runID, NodeID: "setup", Reason: " ", ExpectedRevision: intPtr(1), Now: testRunNow(5)})
			},
			wantError: "workflow_node_block_reason_required",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, root, runID := workflowInstanceTestRun(t)
			writeWorkflowFixture(t, repo, strictLedgerWorkflowYAML())
			if _, err := CreateWorkflowInstance(root, WorkflowCreateOptions{RunID: runID, File: "workflow.yaml", Now: testRunNow(4)}); err != nil {
				t.Fatalf("create workflow instance: %v", err)
			}
			if tt.mutate != nil {
				tt.mutate(t, repo, root, runID)
			}
			before := readWorkflowInstanceForTest(t, repo, runID)
			beforeEvents := countWorkflowTestEvents(t, repo)

			result, err := tt.action(root, runID)
			if tt.wantError != "" {
				problem, _ := err.(*Problem)
				if err == nil || problem == nil || problem.Code != tt.wantError {
					t.Fatalf("error = %v, want %s", err, tt.wantError)
				}
			} else {
				if err != nil {
					t.Fatalf("action error = %v", err)
				}
				if result.OK || result.Reason != tt.wantReason {
					t.Fatalf("result = %#v, want rejected reason %s", result, tt.wantReason)
				}
			}

			after := readWorkflowInstanceForTest(t, repo, runID)
			afterEvents := countWorkflowTestEvents(t, repo)
			if after.Revision != before.Revision || after.UpdatedEventID != before.UpdatedEventID || afterEvents != beforeEvents {
				t.Fatalf("mutation occurred: before rev/event/count=%d/%s/%d after=%d/%s/%d", before.Revision, before.UpdatedEventID, beforeEvents, after.Revision, after.UpdatedEventID, afterEvents)
			}
			for i := range before.Nodes {
				if after.Nodes[i].State != before.Nodes[i].State || after.Nodes[i].LastTransitionEventID != before.Nodes[i].LastTransitionEventID {
					t.Fatalf("node %s changed: before=%#v after=%#v", before.Nodes[i].ID, before.Nodes[i], after.Nodes[i])
				}
			}
		})
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

func strictLedgerWorkflowYAML() string {
	return `schema_version: task-dag/v1
workflow_id: strict-ledger
nodes:
  - id: setup
    depends_on: []
    join: all_of
    required_outputs: ["out/setup.txt"]
  - id: build
    depends_on: [setup]
    join: all_of
    required_outputs: ["out/build.txt"]
`
}

func completeSetupForStrictLedgerTest(t *testing.T, repo string, root Root, runID string) {
	t.Helper()
	if _, err := StartWorkflowNode(root, WorkflowNodeOptions{RunID: runID, NodeID: "setup", ExpectedRevision: intPtr(1), Now: testRunNow(5)}); err != nil {
		t.Fatalf("start setup: %v", err)
	}
	mustWriteText(t, filepath.Join(repo, "out", "setup.txt"), "done\n")
	if _, err := CompleteWorkflowNode(root, WorkflowNodeOptions{RunID: runID, NodeID: "setup", Evidence: "out/setup.txt", ExpectedRevision: intPtr(2), Now: testRunNow(6)}); err != nil {
		t.Fatalf("complete setup: %v", err)
	}
}

func readWorkflowInstanceForTest(t *testing.T, repo string, runID string) WorkflowInstance {
	t.Helper()
	var instance WorkflowInstance
	readJSONFile(t, filepath.Join(repo, ".kkachi", "runs", runID, WorkflowInstanceFile), &instance)
	return instance
}

func readWorkflowEventByID(t *testing.T, repo string, eventID string) Event {
	t.Helper()
	file, err := os.Open(filepath.Join(repo, EventsPath))
	if err != nil {
		t.Fatalf("open events: %v", err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var event Event
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			t.Fatalf("decode event: %v", err)
		}
		if event.EventID == eventID {
			return event
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan events: %v", err)
	}
	t.Fatalf("event %s not found", eventID)
	return Event{}
}

func countWorkflowTestEvents(t *testing.T, repo string) int {
	t.Helper()
	file, err := os.Open(filepath.Join(repo, EventsPath))
	if err != nil {
		t.Fatalf("open events: %v", err)
	}
	defer file.Close()
	count := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		count++
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan events: %v", err)
	}
	return count
}

func assertStrictLedgerPayload(t *testing.T, event Event, runID string, workflowID string, nodeID string, kind string, previousRevision int, resultingRevision int, previousState string, resultingState string) {
	t.Helper()
	if event.RunID == nil || *event.RunID != runID {
		t.Fatalf("event run_id = %#v, want %s", event.RunID, runID)
	}
	assertPayloadString(t, event.Payload, "run_id", runID)
	assertPayloadString(t, event.Payload, "workflow_id", workflowID)
	assertPayloadString(t, event.Payload, "node_id", nodeID)
	assertPayloadString(t, event.Payload, "transition_kind", kind)
	assertPayloadNumber(t, event.Payload, "previous_revision", previousRevision)
	assertPayloadNumber(t, event.Payload, "resulting_revision", resultingRevision)
	assertPayloadString(t, event.Payload, "previous_state", previousState)
	assertPayloadString(t, event.Payload, "resulting_state", resultingState)
}

func assertPayloadString(t *testing.T, payload map[string]any, key string, want string) {
	t.Helper()
	got, ok := payload[key].(string)
	if !ok || got != want {
		t.Fatalf("payload[%s] = %#v, want %q", key, payload[key], want)
	}
}

func assertPayloadNumber(t *testing.T, payload map[string]any, key string, want int) {
	t.Helper()
	got, ok := payload[key].(float64)
	if !ok || int(got) != want || got != float64(want) {
		t.Fatalf("payload[%s] = %#v, want %d", key, payload[key], want)
	}
}

func assertPayloadStringSlice(t *testing.T, payload map[string]any, key string, want []string) {
	t.Helper()
	values, ok := payload[key].([]any)
	if !ok || len(values) != len(want) {
		t.Fatalf("payload[%s] = %#v, want %#v", key, payload[key], want)
	}
	for i, value := range values {
		got, ok := value.(string)
		if !ok || got != want[i] {
			t.Fatalf("payload[%s][%d] = %#v, want %q", key, i, value, want[i])
		}
	}
}

func assertPayloadStringMap(t *testing.T, payload map[string]any, key string, want map[string]string) {
	t.Helper()
	values, ok := payload[key].(map[string]any)
	if !ok || len(values) != len(want) {
		t.Fatalf("payload[%s] = %#v, want %#v", key, payload[key], want)
	}
	for key, wantValue := range want {
		got, ok := values[key].(string)
		if !ok || got != wantValue {
			t.Fatalf("payload map value = %#v, want %s=%s", values, key, wantValue)
		}
	}
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
