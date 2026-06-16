package project

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

func TestWorkflowCatalogProposeRecordsHashBoundNoWriteEvidence(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	packetPath := writeWorkflowCatalogPromotionPacket(t, repo, "promoted", nil)
	beforeEvents := runEventLines(t, repo)

	result, err := ProposeWorkflowCatalogPromotion(root, WorkflowCatalogProposeOptions{
		Packet: packetPath,
		Reason: "promote reviewed KAS workflow",
		Now:    func() time.Time { return time.Date(2026, 6, 16, 1, 2, 3, 0, time.UTC) },
	})
	if err != nil {
		t.Fatalf("ProposeWorkflowCatalogPromotion() error = %v", err)
	}
	if result.Status != WorkflowCatalogStatusPass || result.ProposalID != "wcat-prop-000001" || result.ProposalHash != workflowCatalogTestApprovalHash() || !result.ApprovalRequirements.ProposalHashRequired {
		t.Fatalf("proposal result = %#v, want pass hash-bound proposal", result)
	}
	if result.NoWrite.Guaranteed != true || result.NoWrite.ProjectWriteCount != 0 || result.NoWrite.CandidateFileWriteCount != 0 {
		t.Fatalf("no_write = %#v, want guaranteed no-write posture", result.NoWrite)
	}
	if _, err := os.Stat(filepath.Join(repo, ".kkachi", "workflows", "promoted.yaml")); !os.IsNotExist(err) {
		t.Fatalf("target workflow stat err = %v, want no target write during propose", err)
	}
	var record WorkflowCatalogProposalRecord
	readWorkflowCatalogTestJSON(t, filepath.Join(repo, filepath.FromSlash(result.ProposalPath)), &record)
	if record.ProposalHash != workflowCatalogTestApprovalHash() || record.SourcePacket.ApprovalHash != workflowCatalogTestApprovalHash() || len(record.Candidates) != 3 {
		t.Fatalf("record = %#v, want source approval hash and candidates", record)
	}
	if afterEvents := runEventLines(t, repo); len(afterEvents) != len(beforeEvents)+1 || !strings.Contains(afterEvents[len(afterEvents)-1], workflowCatalogProposalEventType) || !strings.Contains(afterEvents[len(afterEvents)-1], result.ProposalHash) {
		t.Fatalf("events = %#v, want proposal audit event", afterEvents)
	}
}

func TestWorkflowCatalogProposeRejectsMissingApprovalHash(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	packetPath := writeWorkflowCatalogPromotionPacket(t, repo, "promoted", nil, func(packet *workflowCatalogKASPacket) {
		packet.ApprovalHash = ""
	})
	beforeEvents := runEventLines(t, repo)

	_, err := ProposeWorkflowCatalogPromotion(root, WorkflowCatalogProposeOptions{Packet: packetPath, Reason: "promote"})
	assertProblemCode(t, err, "workflow_catalog_approval_hash_required")
	assertWorkflowCatalogApplyRejectedWithoutWrites(t, repo, beforeEvents, ".kkachi/workflows/promoted.yaml")
}

func TestWorkflowCatalogProposeRejectsCandidateFileWriteCount(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	packetPath := writeWorkflowCatalogPromotionPacket(t, repo, "promoted", nil, func(packet *workflowCatalogKASPacket) {
		packet.NoWrite.CandidateFileWriteCount = 1
	})
	beforeEvents := runEventLines(t, repo)

	_, err := ProposeWorkflowCatalogPromotion(root, WorkflowCatalogProposeOptions{Packet: packetPath, Reason: "promote"})
	assertProblemCode(t, err, "workflow_catalog_packet_not_no_write")
	assertWorkflowCatalogApplyRejectedWithoutWrites(t, repo, beforeEvents, ".kkachi/workflows/promoted.yaml")
}

func TestWorkflowCatalogApplyRequiresApprovalAndProposalHash(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	packetPath := writeWorkflowCatalogPromotionPacket(t, repo, "promoted", nil)
	proposal, err := ProposeWorkflowCatalogPromotion(root, WorkflowCatalogProposeOptions{Packet: packetPath, Reason: "promote"})
	if err != nil {
		t.Fatalf("propose: %v", err)
	}
	beforeEvents := runEventLines(t, repo)

	_, err = ApplyWorkflowCatalogPromotion(root, WorkflowCatalogApplyOptions{Proposal: proposal.ProposalID, Approval: "dry-run:" + proposal.ProposalHash})
	assertProblemCode(t, err, "workflow_catalog_proposal_hash_required")
	_, err = ApplyWorkflowCatalogPromotion(root, WorkflowCatalogApplyOptions{Proposal: proposal.ProposalID, ProposalHash: proposal.ProposalHash})
	assertProblemCode(t, err, "workflow_catalog_apply_requires_approval")
	_, err = ApplyWorkflowCatalogPromotion(root, WorkflowCatalogApplyOptions{Proposal: proposal.ProposalID, Approval: "approval:generic", ProposalHash: proposal.ProposalHash})
	assertProblemCode(t, err, "workflow_catalog_hash_bound_approval_missing")

	assertWorkflowCatalogApplyRejectedWithoutWrites(t, repo, beforeEvents, ".kkachi/workflows/promoted.yaml")
}

func TestWorkflowCatalogApplyRejectsMalformedAndMismatchedProposalHashBeforeWrites(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	packetPath := writeWorkflowCatalogPromotionPacket(t, repo, "promoted", nil)
	proposal, err := ProposeWorkflowCatalogPromotion(root, WorkflowCatalogProposeOptions{Packet: packetPath, Reason: "promote"})
	if err != nil {
		t.Fatalf("propose: %v", err)
	}
	beforeEvents := runEventLines(t, repo)

	_, err = ApplyWorkflowCatalogPromotion(root, WorkflowCatalogApplyOptions{Proposal: proposal.ProposalID, Approval: "dry-run:" + proposal.ProposalHash, ProposalHash: "sha256:not64hex"})
	assertProblemCode(t, err, "workflow_catalog_proposal_hash_malformed")
	_, err = ApplyWorkflowCatalogPromotion(root, WorkflowCatalogApplyOptions{Proposal: proposal.ProposalID, Approval: "dry-run:" + proposal.ProposalHash, ProposalHash: "sha256:" + strings.Repeat("c", 64)})
	assertProblemCode(t, err, "workflow_catalog_proposal_hash_mismatch")

	assertWorkflowCatalogApplyRejectedWithoutWrites(t, repo, beforeEvents, ".kkachi/workflows/promoted.yaml")
}

func TestWorkflowCatalogApplyRejectsStaleBaseBeforeWrites(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	packetPath := writeWorkflowCatalogPromotionPacket(t, repo, "promoted", nil)
	proposal, err := ProposeWorkflowCatalogPromotion(root, WorkflowCatalogProposeOptions{Packet: packetPath, Reason: "promote"})
	if err != nil {
		t.Fatalf("propose: %v", err)
	}
	mustWriteFile(t, filepath.Join(repo, ".kkachi", "workflow-catalog.yaml"), []byte("schema_version: workflow-catalog/v1\ncatalog_id: drift\nworkflows: []\n"))
	beforeEvents := runEventLines(t, repo)

	_, err = ApplyWorkflowCatalogPromotion(root, WorkflowCatalogApplyOptions{Proposal: proposal.ProposalID, Approval: "dry-run:" + proposal.ProposalHash, ProposalHash: proposal.ProposalHash})
	assertProblemCode(t, err, "workflow_catalog_base_checksum_mismatch")
	assertWorkflowCatalogApplyRejectedWithoutWrites(t, repo, beforeEvents, ".kkachi/workflows/promoted.yaml")
}

func TestWorkflowCatalogApplyRejectsCandidateChecksumDriftBeforeWrites(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	packetPath := writeWorkflowCatalogPromotionPacket(t, repo, "promoted", nil)
	proposal, err := ProposeWorkflowCatalogPromotion(root, WorkflowCatalogProposeOptions{Packet: packetPath, Reason: "promote"})
	if err != nil {
		t.Fatalf("propose: %v", err)
	}
	proposalPath := filepath.Join(repo, filepath.FromSlash(proposal.ProposalPath))
	var record WorkflowCatalogProposalRecord
	readWorkflowCatalogTestJSON(t, proposalPath, &record)
	record.Candidates[0].Content += "# stale candidate edit\n"
	writeWorkflowCatalogTestJSON(t, proposalPath, record)
	beforeEvents := runEventLines(t, repo)

	_, err = ApplyWorkflowCatalogPromotion(root, WorkflowCatalogApplyOptions{Proposal: proposal.ProposalID, Approval: "dry-run:" + proposal.ProposalHash, ProposalHash: proposal.ProposalHash})
	assertProblemCode(t, err, "workflow_catalog_candidate_checksum_mismatch")
	assertWorkflowCatalogApplyRejectedWithoutWrites(t, repo, beforeEvents, ".kkachi/workflows/promoted.yaml")
}

func TestWorkflowCatalogApplyRejectsInvalidCandidateContentBeforeWrites(t *testing.T) {
	cases := []struct {
		name    string
		kind    string
		content string
	}{
		{name: "workflow DAG", kind: "workflow_dag", content: "schema_version: task-dag/v1\nworkflow_id: promoted\n"},
		{name: "workflow catalog", kind: "workflow_catalog", content: "schema_version: workflow-catalog/v1\ncatalog_id:\nworkflows: []\n"},
		{name: "node contract registry", kind: "node_contract_registry", content: "schema_version: kas-task-dag-workflow-registry/v1\nnode_contracts:\n  - workflow_id: promoted\n    node_id: setup\n    task_class: development\n    completion_authority: kah_only\n    direct_kah_state_write: true\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := initializedRepo(t)
			root, _ := DiscoverRoot(repo)
			packetPath := writeWorkflowCatalogPromotionPacket(t, repo, "promoted", nil)
			proposal, err := ProposeWorkflowCatalogPromotion(root, WorkflowCatalogProposeOptions{Packet: packetPath, Reason: "promote"})
			if err != nil {
				t.Fatalf("propose: %v", err)
			}
			proposalPath := filepath.Join(repo, filepath.FromSlash(proposal.ProposalPath))
			var record WorkflowCatalogProposalRecord
			readWorkflowCatalogTestJSON(t, proposalPath, &record)
			for i := range record.Candidates {
				if record.Candidates[i].Kind == tc.kind {
					checksum := workflowCatalogChecksum([]byte(tc.content))
					record.Candidates[i].Content = tc.content
					record.Candidates[i].SHA256 = checksum
					record.CandidateChecksums[record.Candidates[i].Path] = checksum
					break
				}
			}
			writeWorkflowCatalogTestJSON(t, proposalPath, record)
			beforeEvents := runEventLines(t, repo)

			_, err = ApplyWorkflowCatalogPromotion(root, WorkflowCatalogApplyOptions{Proposal: proposal.ProposalID, Approval: "dry-run:" + proposal.ProposalHash, ProposalHash: proposal.ProposalHash})
			assertProblemCode(t, err, "workflow_catalog_candidate_invalid")
			assertWorkflowCatalogApplyRejectedWithoutWrites(t, repo, beforeEvents, ".kkachi/workflows/promoted.yaml")
		})
	}
}

func TestWorkflowCatalogApplyRejectsUnsafeTargetPathBeforeWrites(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	packetPath := writeWorkflowCatalogPromotionPacket(t, repo, "promoted", nil)
	proposal, err := ProposeWorkflowCatalogPromotion(root, WorkflowCatalogProposeOptions{Packet: packetPath, Reason: "promote"})
	if err != nil {
		t.Fatalf("propose: %v", err)
	}
	proposalPath := filepath.Join(repo, filepath.FromSlash(proposal.ProposalPath))
	var record WorkflowCatalogProposalRecord
	readWorkflowCatalogTestJSON(t, proposalPath, &record)
	record.TargetPaths[0] = "../escape.yaml"
	writeWorkflowCatalogTestJSON(t, proposalPath, record)
	beforeEvents := runEventLines(t, repo)

	_, err = ApplyWorkflowCatalogPromotion(root, WorkflowCatalogApplyOptions{Proposal: proposal.ProposalID, Approval: "dry-run:" + proposal.ProposalHash, ProposalHash: proposal.ProposalHash})
	assertProblemCode(t, err, "workflow_catalog_target_path_unsafe")
	assertWorkflowCatalogApplyRejectedWithoutWrites(t, repo, beforeEvents, "escape.yaml")
}

func TestWorkflowCatalogApplyWritesTargetsBackupsAndAudit(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	oldWorkflow := "schema_version: task-dag/v1\nworkflow_id: promoted\nnodes: []\n"
	oldRegistry := "schema_version: kas-task-dag-workflow-registry/v1\nnode_contracts: []\n"
	oldCatalog := "schema_version: workflow-catalog/v1\ncatalog_id: old\nworkflows: []\n"
	if err := os.MkdirAll(filepath.Join(repo, ".kkachi", "workflows"), 0o755); err != nil {
		t.Fatalf("mkdir workflows: %v", err)
	}
	mustWriteFile(t, filepath.Join(repo, ".kkachi", "workflows", "promoted.yaml"), []byte(oldWorkflow))
	mustWriteFile(t, filepath.Join(repo, ".kkachi", "workflows", "promoted-node-contracts.yaml"), []byte(oldRegistry))
	mustWriteFile(t, filepath.Join(repo, ".kkachi", "workflow-catalog.yaml"), []byte(oldCatalog))
	base := map[string]string{
		".kkachi/workflows/promoted.yaml":                workflowCatalogChecksum([]byte(oldWorkflow)),
		".kkachi/workflows/promoted-node-contracts.yaml": workflowCatalogChecksum([]byte(oldRegistry)),
		".kkachi/workflow-catalog.yaml":                  workflowCatalogChecksum([]byte(oldCatalog)),
	}
	packetPath := writeWorkflowCatalogPromotionPacket(t, repo, "promoted", base)
	proposal, err := ProposeWorkflowCatalogPromotion(root, WorkflowCatalogProposeOptions{Packet: packetPath, Reason: "promote"})
	if err != nil {
		t.Fatalf("propose: %v", err)
	}

	applied, err := ApplyWorkflowCatalogPromotion(root, WorkflowCatalogApplyOptions{
		Proposal:     proposal.ProposalID,
		Approval:     "dry-run:" + proposal.ProposalHash,
		ProposalHash: proposal.ProposalHash,
		Now:          func() time.Time { return time.Date(2026, 6, 16, 2, 3, 4, 0, time.UTC) },
	})
	if err != nil {
		t.Fatalf("ApplyWorkflowCatalogPromotion() error = %v", err)
	}
	if applied.Status != WorkflowCatalogStatusPass || len(applied.AppliedPaths) != 3 || len(applied.BackupPaths) != 3 || applied.EventIDs[0] != "evt-000003" {
		t.Fatalf("applied = %#v, want backed-up apply", applied)
	}
	if got := readWorkflowCatalogTestText(t, filepath.Join(repo, ".kkachi", "workflows", "promoted.yaml")); !strings.Contains(got, "id: setup") {
		t.Fatalf("workflow content = %s, want promoted candidate", got)
	}
	workflowBackup := applied.BackupPaths[".kkachi/workflows/promoted.yaml"]
	if got := readWorkflowCatalogTestText(t, filepath.Join(repo, filepath.FromSlash(workflowBackup))); got != oldWorkflow {
		t.Fatalf("workflow backup = %q, want old workflow", got)
	}
	lines := runEventLines(t, repo)
	last := lines[len(lines)-1]
	for _, want := range []string{workflowCatalogApplyEventType, proposal.ProposalHash, "dry-run:" + proposal.ProposalHash, workflowBackup} {
		if !strings.Contains(last, want) {
			t.Fatalf("last event = %s, want %s", last, want)
		}
	}
}

func writeWorkflowCatalogPromotionPacket(t *testing.T, repo string, workflowID string, base map[string]string, mutate ...func(*workflowCatalogKASPacket)) string {
	t.Helper()
	workflowPath := ".kkachi/workflows/" + workflowID + ".yaml"
	registryPath := ".kkachi/workflows/" + workflowID + "-node-contracts.yaml"
	catalogPath := WorkflowCatalogDefaultPath
	workflow := `schema_version: task-dag/v1
workflow_id: ` + workflowID + `
nodes:
  - id: setup
    depends_on: []
    join: all_of
    required_outputs: ["out/` + workflowID + `/setup.txt"]
`
	catalog := `schema_version: workflow-catalog/v1
catalog_id: kas-promoted-workflows
workflows:
  - workflow_id: ` + workflowID + `
    path: ` + workflowPath + `
    schema_version: task-dag/v1
    node_contract_registry: ` + registryPath + `
`
	registry := `schema_version: kas-task-dag-workflow-registry/v1
node_contracts:
  - workflow_id: ` + workflowID + `
    node_id: setup
    task_class: development
    completion_authority: kah_only
    direct_kah_state_write: false
`
	if base == nil {
		base = map[string]string{workflowPath: workflowCatalogMissingChecksum, registryPath: workflowCatalogMissingChecksum, catalogPath: workflowCatalogMissingChecksum}
	}
	packet := workflowCatalogKASPacket{
		SchemaVersion:    kasWorkflowPromotePacketSchema,
		Canonicalization: "utf8-json-sorted-keys-normalized-relative-paths/v1",
		TargetPaths:      []string{catalogPath, registryPath, workflowPath},
		CandidatePaths: workflowCatalogKASCandidatePaths{
			WorkflowDAG:          workflowPath,
			Catalog:              catalogPath,
			NodeContractRegistry: registryPath,
		},
		GeneratedContent: []WorkflowCatalogCandidateContent{
			{Path: workflowPath, Kind: "workflow_dag", Content: workflow, SHA256: workflowCatalogChecksum([]byte(workflow))},
			{Path: catalogPath, Kind: "workflow_catalog", Content: catalog, SHA256: workflowCatalogChecksum([]byte(catalog))},
			{Path: registryPath, Kind: "node_contract_registry", Content: registry, SHA256: workflowCatalogChecksum([]byte(registry))},
		},
		BaseChecksums: base,
		ChangedPaths: []WorkflowCatalogChangedPath{
			{Path: workflowPath, Action: "create", Kind: "workflow_dag"},
			{Path: catalogPath, Action: "create", Kind: "workflow_catalog"},
			{Path: registryPath, Action: "create", Kind: "node_contract_registry"},
		},
		NoWrite:      WorkflowCatalogNoWriteEvidence{Guaranteed: true},
		ApprovalHash: workflowCatalogTestApprovalHash(),
	}
	for _, fn := range mutate {
		fn(&packet)
	}
	data, err := json.MarshalIndent(packet, "", "  ")
	if err != nil {
		t.Fatalf("marshal packet: %v", err)
	}
	path := filepath.Join(repo, ".kkachi", "runs", "run-test", "artifacts", "workflow-promote-packet.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir packet dir: %v", err)
	}
	mustWriteFile(t, path, append(data, '\n'))
	return ".kkachi/runs/run-test/artifacts/workflow-promote-packet.json"
}

func workflowCatalogTestApprovalHash() string {
	return "sha256:" + strings.Repeat("a", 64)
}

func assertWorkflowCatalogApplyRejectedWithoutWrites(t *testing.T, repo string, beforeEvents []string, relativePath string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(repo, filepath.FromSlash(relativePath))); !os.IsNotExist(err) {
		t.Fatalf("target stat err = %v, want no write after rejected apply for %s", err, relativePath)
	}
	if afterEvents := runEventLines(t, repo); len(afterEvents) != len(beforeEvents) {
		t.Fatalf("events changed after rejected apply: before=%d after=%d", len(beforeEvents), len(afterEvents))
	}
}

func readWorkflowCatalogTestJSON(t *testing.T, path string, out any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read json %s: %v", path, err)
	}
	if err := json.Unmarshal(data, out); err != nil {
		t.Fatalf("unmarshal json %s: %v\n%s", path, err, string(data))
	}
}

func writeWorkflowCatalogTestJSON(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("marshal json %s: %v", path, err)
	}
	mustWriteFile(t, path, append(data, '\n'))
}

func readWorkflowCatalogTestText(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read text %s: %v", path, err)
	}
	return string(data)
}
