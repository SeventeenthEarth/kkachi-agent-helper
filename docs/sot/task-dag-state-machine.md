# KAH task DAG state machine SOT

Date: 2026-06-12
Owner: KAH deterministic helper layer
Confirming role: Responsible approver / governance evidence record pending
Status: planning SOT for KAH `DAGSM` task-DAG state-machine workstream; not implemented behavior until roadmap tasks pass evidence and release gates
Authority level: KAH-side planning authority for deterministic task-DAG schema validation, workflow instance state, node FSM/order enforcement, required evidence checks, diagnostics, gates, and audit events
Scope: `kkachi-agent-helper` docs, schemas, command JSON contracts, deterministic state transitions, diagnostics, gates, tests, and release notes; no KAS policy selection, no agent suitability judgment, no backend execution, no Kanban assignment authority, no Hermes profile/provider/gateway/auth/token/model mutation
Related docs: `docs/specs.md`, `docs/compatibility.md`, `docs/roadmap.md`, `docs/sot/graph-workflow-sync-diagnostics-and-repair.md`, KAS `docs/sot/task-dag-workflow-contract.md`
Evidence/source paths:
- Master direction in 17번째 지구 Discord `#kas` thread `1514986770456903781` on 2026-06-12: KAH should manage task DAG connection/order state while KAS records per-node agent/role contracts and trigger skills start supported workflows.
- This KAH SOT/docs registration is a paired documentation companion for KAS `WFLOW-001` planning acceptance. It may be committed under `[WFLOW-001]` with the KAS planning SOT, but it does not complete KAH `DAGSM-001`; `DAGSM-001` starts only when KAH implementation, tests, specs, compatibility, and release-note evidence are produced.

## Decision summary

KAH will provide a deterministic task-DAG substrate that lets KAS and KAO orchestrate multi-agent workflows without turning KAH into a policy or execution engine. KAH owns local state, validation, ready-node calculation, node state transitions, evidence checks, diagnostics, gates, and audit events. KAS owns workflow policy, selector rules, node agent/role contracts, prompts, and custom trigger skill scaffolding.

The initial KAH design must be simple and fail-closed: DAG nodes, dependencies, `all_of` fan-in, required outputs, node state FSM, event/audit evidence, and final gate integration. Dynamic node creation, retry/rollback automation, arbitrary external triggers, automatic apply, and agent fallback are deferred.

## Deterministic boundary

KAH may answer:

- Is this task DAG structurally valid?
- Are all dependencies for node `X` satisfied?
- Which nodes are ready?
- Is this node transition legal?
- Are required artifacts/evidence present before completion?
- Which graph/catalog/workflow diagnostics explain a blocked state?
- Did the final gate see complete node state and required evidence?

KAH must not answer:

- Which agent is best for this node?
- Which workflow is semantically best for the user's request?
- Whether KAS should use default vs custom policy when both are valid.
- Whether to fall back to a different agent/backend after failure.
- Whether long-lived team-member delegation is complete without Kanban/evidence input.

## Minimum DAG model

The exact schema may be refined during `DAGSM-001`, but the MVP must support these concepts:

```yaml
workflow_id: example-flow
schema_version: task-dag/v1
nodes:
  - id: b
    depends_on: []
    required_outputs:
      - artifacts/b/result.md
  - id: f
    depends_on: [b]
    required_outputs:
      - artifacts/f/result.md
  - id: j
    depends_on: [g, h, i]
    join: all_of
    required_outputs:
      - artifacts/j/final.md
```

Validation must fail closed on missing node ids, duplicate node ids, unknown dependencies, cycles, unsupported join semantics, missing required outputs, path escapes, and unsupported schema versions.

## Node FSM

The MVP node state machine should use a small deterministic vocabulary:

```text
pending -> ready -> dispatched -> running -> evidence_submitted -> succeeded
```

Exception states:

```text
blocked | failed | cancelled | skipped | needs_approval
```

Required rules:

- A node cannot enter `ready` until dependency policy is satisfied.
- A node cannot enter `dispatched` or `running` if it is not ready or if an approval-required gate is unresolved.
- A node cannot enter `succeeded` until all required outputs/evidence exist and validate.
- Workflow instance and node-state writes must reuse KAH path-safety, active-run/project-write lock, atomic-write, status/event-coherence, and crash-safety protections.
- Concurrent transitions against stale workflow/node state must fail closed with a deterministic diagnostic.
- A fan-in node with `join: all_of` cannot become ready until all dependencies have succeeded.
- Skipped/not-applicable nodes require explicit reason and must not satisfy a required dependency unless the workflow policy declares that skip admissible.
- State changes must be evented and auditable.

## Command surface target

The roadmap may adjust names before implementation, but `DAGSM-001` must pin the final command names before any support claim, release note, or downstream KAS trigger depends on them. Until then, this SOT defines the required behavior rather than a completed CLI promise. KAH should expose a command surface equivalent to:

```text
kkachi-agent-helper workflow validate --file <workflow.yaml> --json
kkachi-agent-helper workflow explain --file <workflow.yaml> --json
kkachi-agent-helper workflow create --workflow <workflow-id-or-file> --run <run_id> --json
kkachi-agent-helper workflow show --run <run_id> --json
kkachi-agent-helper workflow ready --run <run_id> --json
kkachi-agent-helper workflow node start --run <run_id> --node <node_id> --json
kkachi-agent-helper workflow node complete --run <run_id> --node <node_id> --evidence <path-or-ref> --json
kkachi-agent-helper workflow node block --run <run_id> --node <node_id> --reason <text> --json
kkachi-agent-helper workflow gate final --run <run_id> --json
```

KAH should also advertise capabilities for the implemented surfaces, for example task-DAG validation, workflow instances, node FSM, ready-node calculation, catalog diagnostics, and final gate integration.

## Catalog and multi-DAG boundary

A project may define multiple task DAGs. KAH should validate deterministic catalog shape and workflow references, but KAS owns selector policy. KAH must not choose a workflow from a user request. If KAS supplies a workflow id or resolved candidate, KAH may validate the catalog entry, workflow file, schema/capability requirements, and run-local instance state.

Supported user-custom workflows must be judged by schema/supportability and evidence, not by equality to a default KAS template. A valid custom graph is not stale merely because it differs from a default template.

## Evidence and diagnostics

KAH should preserve machine-readable evidence for:

- effective helper version and capabilities;
- workflow id/path/schema version;
- catalog id/path/schema version when catalog support is used;
- validation and explain diagnostics;
- workflow instance id/run id;
- node states and last transition event ids;
- dependency blockers and ready-node list;
- required artifacts/evidence checks;
- approval blockers where node or workflow policy requires approval;
- final gate result and missing evidence.

Diagnostics should use stable reason codes where possible, including at least:

- `task_dag_valid`
- `task_dag_missing`
- `task_dag_invalid_schema`
- `task_dag_parse_error`
- `task_dag_duplicate_node`
- `task_dag_unknown_dependency`
- `task_dag_cycle_detected`
- `task_dag_unsupported_join`
- `node_dependency_unsatisfied`
- `node_not_ready`
- `node_required_output_missing`
- `node_transition_invalid`
- `workflow_catalog_invalid`
- `workflow_catalog_ambiguous_reference`
- `workflow_apply_requires_approval`

## Required roadmap sequence

KAH `DAGSM` tasks are part of the cross-repo seven-PR sequence:

1. KAS `WFLOW-001` accepts policy/selector/node-contract boundaries and the paired KAH planning SOT/docs registration.
2. KAH `DAGSM-001` implements task-DAG schema validation/explain and capability evidence.
3. KAH `DAGSM-002` implements workflow instance state, node FSM, ready-node calculation, and required-output completion checks.
4. KAS `WFLOW-002` implements the generic trigger against `DAGSM-002` evidence.
5. KAS `WFLOW-003` implements selector and node-contract registry shape consumed by later catalog integration.
6. KAH `DAGSM-003` implements multi-DAG catalog diagnostics and final gate integration after KAS metadata shape is settled.
7. KAS `WFLOW-004` implements custom workflow creator and optional thin trigger generation.

KAH implementation must not add selector policy, custom skill generation, or agent assignment logic in any `DAGSM` task.

## Non-goals and deferrals

- No KAS selector implementation in KAH.
- No KAB backend/session execution in KAH.
- No Kanban card assignment in KAH.
- No automatic KAH/KAS update.
- No automatic graph/catalog apply from periodic checks.
- No direct `.kkachi-workflow.yaml` manual-edit fallback.
- No arbitrary webhook/event daemon in the MVP.
- No dynamic node creation, retry policy, rollback automation, or fallback agent selection in the MVP.
- No Hermes profile/provider/gateway/auth/token/model mutation.

## Acceptance gates before KAH claims DAGSM support

- `workflow validate/explain` or equivalent rejects invalid DAGs and reports stable JSON diagnostics.
- Workflow instance and node FSM commands enforce dependency order and required output evidence.
- All workflow-instance and node-state mutations use existing KAH safe-path checks, active-run/project-write locking, atomic writes, status/event coherence, event audit, and stale/concurrent transition refusal.
- Ready-node calculation is deterministic and fixture-tested for sequence, fan-out, and `all_of` fan-in.
- Final gate fails when required nodes or evidence are incomplete.
- Docs/specs/compatibility/roadmap/release notes match implemented command behavior.
- Red/Orange/Gray review gates are resolved if the active Kkachi run requires them.
