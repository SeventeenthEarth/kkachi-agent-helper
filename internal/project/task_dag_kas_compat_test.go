package project

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These are static shape fixtures derived from KAS WFLOW-006 evidence:
// /Users/draccoon/Workspace/SeventeenthEarth/kkachi/kkachi-hermes-skills/docs/roadmap.md
// WFLOW-006 Completed, registries/task-dag-workflow-registry.yaml, and
// internal/skills/workflowcreator/workflowcreator.go renderDAG. KAH must not
// import KAS or own KAS workflow policy.

func TestValidateTaskDAGAcceptsKASWFLOW006StandardBundleShapes(t *testing.T) {
	// Static compatibility fixtures based on the KAS WFLOW-006 standard bundle
	// registry and renderer shape. KAH validates schema/edges/evidence paths only;
	// it does not import or execute KAS classification or bundle-selection policy.
	bundles := map[string][]string{
		"development_full":        {"codegraph_refresh", "plan", "ask", "implement", "enhance_test", "optimize", "update_docs", "request_feedback", "handle_feedback", "final_verify", "improve"},
		"docs_only_light":         {"task_contract", "plan", "update_docs", "docs_validation", "final_verify"},
		"research_evidence_light": {"task_contract", "evidence_collection", "source_citation", "final_verify"},
		"review_light":            {"task_contract", "review_request", "feedback_evidence", "final_verify"},
		"bootstrap_config":        {"task_contract", "preflight", "configure", "verification", "final_verify"},
		"direct_report":           {"command_evidence", "final_report"},
	}

	for workflowID, nodes := range bundles {
		t.Run(workflowID, func(t *testing.T) {
			repo := t.TempDir()
			workflow := filepath.Join(repo, workflowID+".yaml")
			if err := os.WriteFile(workflow, []byte(renderKASWFLOW006TaskDAGFixture(workflowID, nodes)), 0o600); err != nil {
				t.Fatalf("write workflow fixture: %v", err)
			}

			result, err := ValidateTaskDAG(Root{Path: repo}, filepath.Base(workflow))
			if err != nil {
				t.Fatalf("ValidateTaskDAG returned error: %v", err)
			}
			if !result.OK || result.Status != TaskDAGStatusValid || result.Reason != "task_dag_valid" {
				t.Fatalf("result = %#v, want valid KAS WFLOW-006 bundle fixture", result)
			}
			if result.WorkflowID != workflowID || result.SchemaVersion != "task-dag/v1" {
				t.Fatalf("identity = %q/%q, want %s/task-dag/v1", result.WorkflowID, result.SchemaVersion, workflowID)
			}
			if len(result.Nodes) != len(nodes) {
				t.Fatalf("nodes = %#v, want %d rendered bundle nodes", result.Nodes, len(nodes))
			}
			if len(result.Edges) != len(nodes)-1 {
				t.Fatalf("edges = %#v, want linear bundle edges", result.Edges)
			}
			if len(result.Nodes[0].DependsOn) != 0 {
				t.Fatalf("root depends_on = %#v, want empty inline root dependency list", result.Nodes[0].DependsOn)
			}
			for i, node := range result.Nodes {
				if node.ID != nodes[i] {
					t.Fatalf("node[%d].id = %q, want %q", i, node.ID, nodes[i])
				}
				if node.Join != "all_of" {
					t.Fatalf("node[%d].join = %q, want all_of", i, node.Join)
				}
				if len(node.RequiredOutputs) != 1 || node.RequiredOutputs[0] != fmt.Sprintf(".kkachi/runs/run-123/artifacts/%s/%s.md", workflowID, node.ID) {
					t.Fatalf("node[%d].required_outputs = %#v, want deterministic artifact path", i, node.RequiredOutputs)
				}
			}
		})
	}
}

func TestValidateTaskDAGAcceptsKASWFLOW006FanInShape(t *testing.T) {
	repo := t.TempDir()
	workflow := filepath.Join(repo, "fan-in.yaml")
	content := `workflow_id: development_full
schema_version: task-dag/v1
nodes:
  - id: plan
    depends_on: []
    join: all_of
    required_outputs:
      - .kkachi/runs/run-123/artifacts/development_full/plan.md
  - id: implement
    depends_on: [plan]
    join: all_of
    required_outputs:
      - .kkachi/runs/run-123/artifacts/development_full/implement.md
  - id: docs_validation
    depends_on: [plan]
    join: all_of
    required_outputs:
      - .kkachi/runs/run-123/artifacts/development_full/docs-validation.md
  - id: final_verify
    depends_on: [docs_validation, implement]
    join: all_of
    required_outputs:
      - .kkachi/runs/run-123/artifacts/development_full/final-verify.md
`
	if err := os.WriteFile(workflow, []byte(content), 0o600); err != nil {
		t.Fatalf("write workflow fixture: %v", err)
	}

	result, err := ValidateTaskDAG(Root{Path: repo}, filepath.Base(workflow))
	if err != nil {
		t.Fatalf("ValidateTaskDAG returned error: %v", err)
	}
	if !result.OK {
		t.Fatalf("result = %#v, want valid fan-in fixture", result)
	}
	if len(result.Edges) != 4 {
		t.Fatalf("edges = %#v, want four projected dependencies", result.Edges)
	}
	final := result.Nodes[3]
	if final.ID != "final_verify" || len(final.DependsOn) != 2 || final.DependsOn[0] != "docs_validation" || final.DependsOn[1] != "implement" {
		t.Fatalf("final node = %#v, want inline fan-in dependencies preserved", final)
	}
}

func renderKASWFLOW006TaskDAGFixture(workflowID string, nodes []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "workflow_id: %s\nschema_version: task-dag/v1\nnodes:\n", workflowID)
	for i, node := range nodes {
		fmt.Fprintf(&b, "  - id: %s\n", node)
		if i == 0 {
			b.WriteString("    depends_on: []\n")
		} else {
			fmt.Fprintf(&b, "    depends_on: [%s]\n", nodes[i-1])
		}
		b.WriteString("    join: all_of\n")
		b.WriteString("    required_outputs:\n")
		fmt.Fprintf(&b, "      - .kkachi/runs/run-123/artifacts/%s/%s.md\n", workflowID, node)
	}
	return b.String()
}
