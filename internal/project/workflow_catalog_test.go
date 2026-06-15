package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateWorkflowCatalogAcceptsMultiDAGCatalogAndRegistry(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	writeCatalogWorkflow(t, repo, "alpha")
	writeCatalogWorkflow(t, repo, "beta")
	writeWorkflowCatalog(t, repo, `schema_version: workflow-catalog/v1
catalog_id: kah-test
workflows:
  - workflow_id: alpha
    path: .kkachi/workflows/alpha.yaml
    schema_version: task-dag/v1
    node_contract_registry: registries/task-dag-workflow-registry.yaml
  - workflow_id: beta
    path: .kkachi/workflows/beta.yaml
    schema_version: task-dag/v1
`)
	writeNodeContractRegistry(t, repo, `schema_version: kas-task-dag-workflow-registry/v1
selector_defaults:
  mode: kas-owned
node_contracts:
  - workflow_id: alpha
    node_id: setup
    task_class: development
    completion_authority: kah_only
    direct_kah_state_write: false
  - workflow_id: alpha
    node_id: build
    task_class: development
    completion_authority: kah_only
    direct_kah_state_write: false
`)

	result, err := ValidateWorkflowCatalog(root, WorkflowCatalogOptions{File: WorkflowCatalogDefaultPath, WorkflowID: "alpha"})
	if err != nil {
		t.Fatalf("ValidateWorkflowCatalog() error = %v", err)
	}
	if !result.OK || result.Reason != "workflow_catalog_valid" || len(result.Workflows) != 2 {
		t.Fatalf("result = %#v, want valid multi-DAG catalog", result)
	}
	if !containsTaskDAGString(result.ReasonCodes, "node_contract_registry_valid") {
		t.Fatalf("reason_codes = %#v, want node_contract_registry_valid", result.ReasonCodes)
	}
	if result.Workflows[0].TaskDAG == nil || !result.Workflows[0].TaskDAG.OK || result.Workflows[0].NodeContracts == nil || !result.Workflows[0].NodeContracts.OK {
		t.Fatalf("workflow alpha = %#v, want task-DAG and registry pass", result.Workflows[0])
	}
}

func TestValidateWorkflowCatalogFailsClosedForReferenceProblems(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	writeCatalogWorkflow(t, repo, "alpha")
	writeWorkflowCatalog(t, repo, `schema_version: workflow-catalog/v1
catalog_id: kah-test
workflows:
  - workflow_id: alpha
    path: .kkachi/workflows/alpha.yaml
    schema_version: task-dag/v1
  - workflow_id: alpha
    path: .kkachi/workflows/missing.yaml
    schema_version: task-dag/v1
  - workflow_id: beta
    path: ../escape.yaml
    schema_version: task-dag/v1
`)

	result, err := ValidateWorkflowCatalog(root, WorkflowCatalogOptions{File: WorkflowCatalogDefaultPath, WorkflowID: "alpha"})
	if err != nil {
		t.Fatalf("ValidateWorkflowCatalog() error = %v", err)
	}
	for _, code := range []string{"workflow_catalog_duplicate_workflow", "workflow_catalog_ambiguous_reference", "workflow_catalog_workflow_missing", "workflow_catalog_unsafe_path"} {
		if !containsTaskDAGString(result.ReasonCodes, code) {
			t.Fatalf("reason_codes = %#v, want %s", result.ReasonCodes, code)
		}
	}
}

func TestValidateWorkflowCatalogChecksRegistryCoverageAndSafetyFields(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	writeCatalogWorkflow(t, repo, "alpha")
	writeWorkflowCatalog(t, repo, `schema_version: workflow-catalog/v1
catalog_id: kah-test
workflows:
  - workflow_id: alpha
    path: .kkachi/workflows/alpha.yaml
    schema_version: task-dag/v1
    node_contract_registry: registries/task-dag-workflow-registry.yaml
`)
	writeNodeContractRegistry(t, repo, `schema_version: kas-task-dag-workflow-registry/v1
node_contracts:
  - workflow_id: alpha
    node_id: setup
    task_class:
    completion_authority: kas
    direct_kah_state_write: true
  - workflow_id: alpha
    node_id: ghost
    task_class: development
    completion_authority: kah_only
    direct_kah_state_write: false
`)

	result, err := ValidateWorkflowCatalog(root, WorkflowCatalogOptions{File: WorkflowCatalogDefaultPath})
	if err != nil {
		t.Fatalf("ValidateWorkflowCatalog() error = %v", err)
	}
	for _, code := range []string{"node_contract_missing", "node_contract_unknown_node", "node_contract_task_class_missing", "node_contract_completion_authority_invalid", "node_contract_direct_kah_state_write_forbidden"} {
		if !containsTaskDAGString(result.ReasonCodes, code) {
			t.Fatalf("reason_codes = %#v, want %s", result.ReasonCodes, code)
		}
	}
}

func TestCreateWorkflowInstanceFromCatalogRequiresExplicitWorkflowID(t *testing.T) {
	repo, root, runID := workflowInstanceTestRun(t)
	writeCatalogWorkflow(t, repo, "alpha")
	writeWorkflowCatalog(t, repo, `schema_version: workflow-catalog/v1
catalog_id: kah-test
workflows:
  - workflow_id: alpha
    path: .kkachi/workflows/alpha.yaml
    schema_version: task-dag/v1
`)

	result, err := CreateWorkflowInstance(root, WorkflowCreateOptions{RunID: runID, Catalog: WorkflowCatalogDefaultPath, WorkflowID: "alpha", Now: testRunNow(4)})
	if err != nil {
		t.Fatalf("CreateWorkflowInstance(catalog) error = %v", err)
	}
	if !result.OK || result.Instance == nil || result.Instance.WorkflowID != "alpha" || result.Catalog == nil || !result.Catalog.OK {
		t.Fatalf("result = %#v, want alpha instance from catalog", result)
	}
}

func TestWorkflowInstanceCompletenessRechecksOutputsAndEvidence(t *testing.T) {
	repo, root, runID := workflowInstanceTestRun(t)
	writeWorkflowFixture(t, repo, `schema_version: task-dag/v1
workflow_id: demo
nodes:
  - id: setup
    depends_on: []
    join: all_of
    required_outputs: ["out/setup.txt"]
`)
	if _, err := CreateWorkflowInstance(root, WorkflowCreateOptions{RunID: runID, File: "workflow.yaml", Now: testRunNow(4)}); err != nil {
		t.Fatalf("create workflow: %v", err)
	}
	if _, err := StartWorkflowNode(root, WorkflowNodeOptions{RunID: runID, NodeID: "setup", Now: testRunNow(5)}); err != nil {
		t.Fatalf("start setup: %v", err)
	}
	mustWriteText(t, filepath.Join(repo, "out", "setup.txt"), "done\n")
	if _, err := CompleteWorkflowNode(root, WorkflowNodeOptions{RunID: runID, NodeID: "setup", Evidence: "out/setup.txt", Now: testRunNow(6)}); err != nil {
		t.Fatalf("complete setup: %v", err)
	}
	complete, err := CheckWorkflowInstanceCompleteness(root, runID)
	if err != nil {
		t.Fatalf("CheckWorkflowInstanceCompleteness() error = %v", err)
	}
	if !complete.OK || complete.Reason != "workflow_instance_complete" {
		t.Fatalf("complete = %#v, want workflow_instance_complete", complete)
	}
	if err := os.Remove(filepath.Join(repo, "out", "setup.txt")); err != nil {
		t.Fatalf("remove output: %v", err)
	}
	missing, err := CheckWorkflowInstanceCompleteness(root, runID)
	if err != nil {
		t.Fatalf("CheckWorkflowInstanceCompleteness(missing) error = %v", err)
	}
	if missing.OK || missing.Reason != "workflow_required_output_missing" {
		t.Fatalf("missing = %#v, want workflow_required_output_missing", missing)
	}
}

func writeCatalogWorkflow(t *testing.T, repo string, workflowID string) {
	t.Helper()
	path := filepath.Join(repo, ".kkachi", "workflows", workflowID+".yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir workflow dir: %v", err)
	}
	content := `schema_version: task-dag/v1
workflow_id: ` + workflowID + `
nodes:
  - id: setup
    depends_on: []
    join: all_of
    required_outputs: ["out/` + workflowID + `/setup.txt"]
  - id: build
    depends_on: [setup]
    join: all_of
    required_outputs: ["out/` + workflowID + `/build.txt"]
`
	mustWriteFile(t, path, []byte(content))
}

func writeWorkflowCatalog(t *testing.T, repo string, content string) {
	t.Helper()
	mustWriteFile(t, filepath.Join(repo, filepath.FromSlash(WorkflowCatalogDefaultPath)), []byte(content))
}

func writeNodeContractRegistry(t *testing.T, repo string, content string) {
	t.Helper()
	path := filepath.Join(repo, "registries", "task-dag-workflow-registry.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir registry dir: %v", err)
	}
	mustWriteFile(t, path, []byte(content))
}
