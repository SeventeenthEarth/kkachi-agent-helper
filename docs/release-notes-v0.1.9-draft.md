# kkachi-agent-helper v0.1.9 draft release notes

Release date: pending
Commit: pending

## Summary

- Adds KAH graph workflow sync diagnostics and stable reason-code hardening for KAS v0.1.2 consumption.
- Adds DAGSM-001 task-DAG schema validation/explain diagnostics for local workflow YAML inspection.
- Adds DAGSM-002 run-local workflow instance state, node FSM transitions, ready-node calculation, stale revision refusal, required-output evidence checks, and audit events.
- Adds DAGSM-003 multi-DAG workflow catalog diagnostics, explicit catalog workflow-instance create mode, optional KAS node-contract registry structural evidence checks, diagnostics export coverage, and workflow-instance final-gate integration.

## Compatibility

| Component | Version/range | Verification |
|---|---|---|
| kkachi-agent-helper | `v0.1.9` target | `kkachi-agent-helper --version` after release/install |
| Project schema | `0.1` | `kkachi-agent-helper project doctor --json` |
| kkachi-agent-bridge | Not checked by helper | Manual; no KAB graph authority is introduced |
| kkachi-hermes-skills | KAS v0.1.2 planned consumer | KAS decides support envelope and update guidance |

## Changes

- `graph validate --json` now emits stable `reason_codes` for graph compatibility classification.
- `graph explain --json` now emits top-level `reason_codes` and preserves validation reason codes in `validation_summary.reason_codes`.
- `diagnostics export` `graph_compatibility` now emits top-level reason codes, nested validation reason codes, nested feedback-intake reason codes, and forbidden fallback source `reason_code` fields.
- Initial reason-code vocabulary reserves graph missing/valid/invalid-schema/parse-error/source-precedence/checksum-audit/feedback-intake/manual-edit/phase-conflict/approval-required/repairability/forbidden-fallback facts; this slice emits producer-backed codes for the implemented validation and diagnostics states.
- `workflow validate --file <workflow.yaml> --json` and `workflow explain --file <workflow.yaml> --json` now expose DAGSM-001 task-DAG schema validation/projection for `task-dag/v1` local YAML files.
- Task-DAG validation covers `workflow_id`, `schema_version`, node ids, dependencies, duplicate ids, cycles, `join: all_of`, per-node `required_outputs`, repository-confined output paths, stable public reason codes, JSON safety results, and the `task_dag_schema_validation=true` capability flag.
- Workflow instance state stores `.kkachi/runs/<run_id>/workflow-instance.json`, exposes `workflow create/show/ready/node` JSON, enforces pending/running/succeeded/blocked node transitions, refuses stale `--expect-revision`, checks required output/evidence paths before completion, emits workflow audit events, and advertises `workflow_instance_state=true`.
- Workflow catalog diagnostics validate `.kkachi/workflow-catalog.yaml` style multi-DAG catalogs through `workflow catalog validate/explain`, require explicit `--workflow-id` inputs for catalog create mode, preserve existing `workflow create --file` behavior, and advertise `workflow_catalog_diagnostics=true`, `workflow_final_gate_integration=true`, and `workflow_node_contract_registry_evidence=true`.
- Optional KAS WFLOW-003 node-contract registry checks validate structural evidence such as node coverage, `task_class`, `completion_authority: kah_only`, and `direct_kah_state_write: false`; KAH still does not evaluate selector matches, rank workflows, or choose fallbacks.
- `gate final` now includes workflow-instance completeness evidence: non-DAGSM runs without `.kkachi/runs/<run_id>/workflow-instance.json` are not applicable, while existing workflow instances must have all nodes succeeded and declared outputs/evidence still present.
- KAH still reports deterministic support facts only; KAS remains the policy owner for supported envelope and update guidance.

## Verification

```sh
go test -count=1 ./...
go run . graph validate --file .kkachi-workflow.yaml --json
go run . graph explain --file .kkachi-workflow.yaml --json
go run . diagnostics export --json
go run . workflow validate --file .kkachi/runs/<run_id>/artifacts/implementation/task-dag-valid.yaml --json
go run . workflow explain --file .kkachi/runs/<run_id>/artifacts/implementation/task-dag-valid.yaml --json
go run . workflow create --run <run_id> --file .kkachi/runs/<run_id>/artifacts/implementation/task-dag-valid.yaml --json
go run . workflow catalog validate --file .kkachi/workflow-catalog.yaml --json
go run . workflow catalog explain --file .kkachi/workflow-catalog.yaml --workflow-id <workflow_id> --json
go run . workflow create --run <run_id> --catalog .kkachi/workflow-catalog.yaml --workflow-id <workflow_id> --json
go run . workflow ready --run <run_id> --json
go run . workflow node start --run <run_id> --node <node_id> --expect-revision 1 --json
go run . workflow node complete --run <run_id> --node <node_id> --evidence <required-output-path> --expect-revision 2 --json
```

Maintainer evidence for the draft was captured with `HOME=/Users/draccoon` because local KAH/KAS tooling in this workspace expects the real user home.

## GRSYNC-002 repair substrate

- `grsync-002` implements the approval-gated complete-candidate repair substrate for missing, stale, broken, or otherwise invalid project `.kkachi-workflow.yaml` states.
- `graph propose` now records base validation status/reason-code evidence even when the base graph is missing or invalid, validates a complete candidate graph, and marks repair replacements with `graph_replacement` plus `approval_required=true`.
- `graph apply` now accepts matching repair proposals only with explicit `--approval <evidence-ref>`, rechecks current base evidence for drift, writes `.kkachi-workflow.yaml` atomically, records `graph.applied`, and preserves `backup_path`/`recovery_ref` when replacing an existing stale/broken graph.

## Known gaps / non-goals
- No automatic KAH update, automatic graph/catalog apply, KAS compatibility registry, KAS doctor/repair CLI behavior, KAS selector matching, workflow ranking, fallback choice, KAB graph authority, `kah graph` alias behavior, direct `.kkachi-workflow.yaml` edit fallback, or Hermes profile/provider/gateway/auth/token/model mutation is introduced.
