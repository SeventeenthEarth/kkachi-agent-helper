# KAH task DAG state machine SOT

Date: 2026-06-12
Owner: KAH deterministic helper layer
Confirming role: Responsible approver / governance evidence record pending
Status: current SOT for KAH `DAGSM` task-DAG state-machine workstream; DAGSM-001..003 define the initial deterministic substrate, DAGSM-004..005 align/harden that substrate for KAS WFLOW bundle/run-local ephemeral adoption, and DAGSM-006 defines the source-side explicit workflow catalog proposal/apply substrate; installed/effective readiness still requires capability evidence from the binary that will run
Authority level: KAH-side planning authority for deterministic task-DAG schema validation, workflow instance state, node FSM/order enforcement, required evidence checks, diagnostics, gates, and audit events
Scope: `kkachi-agent-helper` docs, schemas, command JSON contracts, deterministic state transitions, diagnostics, gates, tests, and release notes; no KAS policy selection, no agent suitability judgment, no backend execution, no Kanban assignment authority, no Hermes profile/provider/gateway/auth/token/model mutation
Related docs: `docs/specs.md`, `docs/compatibility.md`, `docs/roadmap.md`, `docs/sot/graph-workflow-sync-diagnostics-and-repair.md`, KAS `docs/sot/task-dag-workflow-contract.md`
Evidence/source paths:
- Master direction in 17번째 지구 Discord `#kas` thread `1514986770456903781` on 2026-06-12: KAH should manage task DAG connection/order state while KAS records per-node agent/role contracts and trigger skills start supported workflows.
- This KAH SOT/docs registration is a paired documentation companion for KAS `WFLOW-001` planning acceptance. It may be committed under `[WFLOW-001]` with the KAS planning SOT, but it does not complete KAH `DAGSM-001`; `DAGSM-001` starts only when KAH implementation, tests, specs, compatibility, and release-note evidence are produced.
- Master direction in 17번째 지구 Discord `#kas` thread `1516002725689819209` on 2026-06-16: extend WFLOW/DAGSM directly with WFLOW-005 and DAGSM-004 SOT alignment; run-local ephemeral workflows are the default adoption path, while project-local persistent promotion remains explicit and approval-gated.

## Decision summary

KAH will provide a deterministic task-DAG substrate that lets KAS and KAO orchestrate multi-agent workflows without turning KAH into a policy or execution engine. KAH owns local state, validation, ready-node calculation, node state transitions, evidence checks, diagnostics, gates, and audit events. KAS owns workflow policy, selector rules, node agent/role contracts, prompts, and custom trigger skill scaffolding.

The initial KAH design must be simple and fail-closed: DAG nodes, dependencies, `all_of` fan-in, required outputs, node state FSM, event/audit evidence, and final gate integration. Dynamic node creation, retry/rollback automation, arbitrary external triggers, automatic apply, and agent fallback are deferred.

DAGSM-004 records the KAH-side adoption alignment for KAS WFLOW-005. KAH must remain the deterministic validator/state owner for run-local ephemeral workflows, but it must not own task classification, bundle selection, node owner selection, prompt policy, or project-local promotion decisions. Effective-binary capability evidence is mandatory before KAS may rely on KAH workflow support; a source-built implementation or release note is not enough when the installed runtime lacks the workflow command group.

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

DAGSM-001..003 pinned the implemented workflow command names, but KAS must still check the effective installed binary before relying on them. KAH should expose a command surface equivalent to:

```text
kkachi-agent-helper workflow validate --file <workflow.yaml> --json
kkachi-agent-helper workflow explain --file <workflow.yaml> --json
kkachi-agent-helper workflow create --run <run_id> --file <workflow.yaml> --json
kkachi-agent-helper workflow create --run <run_id> --catalog <catalog.yaml> --workflow-id <workflow_id> [--node-contract-registry <registry.yaml>] --json
kkachi-agent-helper workflow show --run <run_id> --json
kkachi-agent-helper workflow ready --run <run_id> --json
kkachi-agent-helper workflow node start --run <run_id> --node <node_id> --json
kkachi-agent-helper workflow node complete --run <run_id> --node <node_id> --evidence <path-or-ref> --json
kkachi-agent-helper workflow node block --run <run_id> --node <node_id> --reason <text> --json
kkachi-agent-helper workflow catalog propose --packet <kas-promote-packet.json> --reason <text> --json
kkachi-agent-helper workflow catalog apply --proposal <proposal-id> --approval <evidence-ref> --proposal-hash sha256:<64hex> --json
kkachi-agent-helper gate final --run <run_id> --json
```

KAH should also advertise capabilities for the implemented surfaces, for example task-DAG validation, workflow instances, node FSM, ready-node calculation, catalog diagnostics, workflow catalog proposal/apply, and final gate integration.

## Catalog and multi-DAG boundary

A project may define multiple task DAGs. KAH should validate deterministic catalog shape and workflow references, but KAS owns selector policy. KAH must not choose a workflow from a user request. If KAS supplies a workflow id or resolved candidate, KAH may validate the catalog entry, workflow file, schema/capability requirements, and run-local instance state.

Supported user-custom workflows must be judged by schema/supportability and evidence, not by equality to a default KAS template. A valid custom graph is not stale merely because it differs from a default template.

## Run-local ephemeral adoption boundary

KAH accepts run-local task-DAG files as ordinary repository-confined workflow inputs when the effective command surface supports `workflow validate`, `workflow create --run <run_id> --file <workflow.yaml>`, `workflow ready`, and `workflow node ...`. The authoritative state remains `.kkachi/runs/<run_id>/workflow-instance.json` or the implemented equivalent after KAH creates the instance. KAH does not persist a run-local ephemeral DAG into the project workflow catalog and does not infer that a run-local workflow should become reusable.

KAS may materialize bundle-derived or one-off workflows under `.kkachi/runs/<run_id>/...`, but KAH only validates structure, creates/resumes instances, computes ready nodes, checks required outputs/evidence, records transitions, and contributes deterministic final-gate evidence. KAH must fail closed when the effective installed binary does not advertise workflow support, when YAML is not accepted by the current task-DAG parser, when paths escape the repository, or when node completion evidence is missing.

Project-local persistent promotion remains explicit and approval-gated. DAGSM-006 adds KAH-owned `workflow catalog propose/apply` mechanics for KAS WFLOW-009 packets with base checksums, drift checks, candidate checksums, candidate DAG/catalog/node-contract validation, backup/recovery evidence, hash-bound approval evidence, and audit events. KAS supplies candidate content and approval evidence; KAH validates, proposes, applies, backs up, and audits only. If the effective installed binary does not advertise `workflow_catalog_proposal_apply=true`, KAS promotion apply must fail closed rather than directly writing authoritative workflow state.

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
- `workflow_catalog_apply_requires_approval`
- `workflow_catalog_proposal_hash_required`
- `workflow_catalog_proposal_hash_mismatch`
- `workflow_catalog_base_checksum_mismatch`
- `workflow_catalog_candidate_checksum_mismatch`
- `workflow_catalog_hash_bound_approval_missing`

## Required roadmap sequence

KAH `DAGSM` tasks are part of the cross-repo seven-PR sequence:

1. KAS `WFLOW-001` accepts policy/selector/node-contract boundaries and the paired KAH planning SOT/docs registration.
2. KAH `DAGSM-001` implements task-DAG schema validation/explain and capability evidence.
3. KAH `DAGSM-002` implements workflow instance state, node FSM, ready-node calculation, and required-output completion checks.
4. KAS `WFLOW-002` implements the generic trigger against `DAGSM-002` evidence.
5. KAS `WFLOW-003` implements selector and node-contract registry shape consumed by later catalog integration.
6. KAH `DAGSM-003` implements multi-DAG catalog diagnostics and final gate integration after KAS metadata shape is settled.
7. KAS `WFLOW-004` implements custom workflow creator and optional thin trigger generation.
8. KAS `WFLOW-005` plus KAH `DAGSM-004` align the SOT/roadmaps for standard bundle workflows, task-classification routing, run-local ephemeral defaults, effective-binary capability evidence, and explicit promotion boundaries.
9. KAS `WFLOW-006` implements bundle registry/templates and KAH-compatible DAG rendering.
10. KAH `DAGSM-005` hardens effective workflow capability, installed-binary alignment, and KAS-generated DAG compatibility evidence where KAH support is required.
11. KAS `WFLOW-007` and `WFLOW-008` implement deterministic classification routing and run-local ephemeral materialization.
12. KAS `WFLOW-009` plus KAH `DAGSM-006` implement explicit project-local promotion through source-side proposal/apply support; effective use remains gated on installed binary capability evidence.

KAH implementation must not add selector policy, custom skill generation, task classification, bundle routing, project-local promotion policy, or agent assignment logic in any `DAGSM` task.

## Non-goals and deferrals

- No KAS selector implementation in KAH.
- No KAB backend/session execution in KAH.
- No Kanban card assignment in KAH.
- No automatic KAH/KAS update.
- No automatic graph/catalog apply from periodic checks.
- No automatic persistence of run-local ephemeral workflows into project catalogs.
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
- Effective installed-binary workflow capability evidence is captured before KAS relies on workflow commands in a run.
- Red/Orange/Gray review gates are resolved if the active Kkachi run requires them.
