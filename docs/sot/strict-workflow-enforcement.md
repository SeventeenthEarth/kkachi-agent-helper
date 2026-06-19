# STRICT workflow enforcement helper SOT

Date: 2026-06-19
Owner: KAH deterministic helper layer
Confirming role: Responsible approver / KAS Blue command with Red, Orange, and project-Gray review evidence accepted for `STRICT-001`
Status: accepted companion SOT for KAH-owned `STRICT` tasks; `STRICT-001` docs/SOT registration is complete and does not implement new helper behavior
Authority level: KAH-side planning authority for strict workflow-managed run markers, node transition ledger/order verification, and workflow projection gates
Scope: `kkachi-agent-helper` docs, schemas, command JSON contracts, deterministic state transitions, diagnostics, gates, and tests. No KAS task classification, workflow selection, node owner/prompt/backend policy, KAB runtime activation, profile/provider/gateway/auth/token/model mutation, install, release, push, or automatic rollback is authorized by this document.
Upstream KAS SOT: `kkachi-hermes-skills/docs/sot/strict-workflow-execution-contract.md`
Related docs: `docs/sot/task-dag-state-machine.md`, `docs/compatibility.md`, `docs/specs.md`, `docs/roadmap.md`, KAS `docs/sot/task-dag-workflow-contract.md`
Evidence/source paths:
- 주군 direction in 17번째 지구 Discord `#kas` thread `1517399560626901034` on 2026-06-19: strict order should come from KAS/KAH task classification and selected workflow execution, not realtime warnings; KAH should supply/validate next node ids and reject out-of-order attempts.
- Existing KAH DAGSM substrate: task-DAG validation, workflow instance state, node FSM, ready-node calculation, required evidence checks, audit events, catalog diagnostics, and final gate integration are already the baseline that `STRICT` hardens.
Epic: `STRICT` — strict workflow execution and order enforcement

## Purpose

`STRICT` hardens KAH from "workflow state exists and can be checked" into "workflow-managed runs cannot complete unless node execution order was admitted by KAH." KAH remains deterministic: it validates declared state, records transitions, calculates ready nodes, and fails closed. KAH does not choose the task class, select the workflow, choose agents, author prompts, or decide review policy.

## KAH companion principles

1. **KAS selects; KAH enforces:** KAS supplies the selected workflow and dispatch policy. KAH validates the DAG, creates/resumes the instance, calculates ready nodes, and owns node state transitions.
2. **Ready nodes are execution admission:** a node may be started only if it is in the current KAH ready set and the expected revision matches.
3. **Ledger before evidence claims:** node work should start only after a successful KAH start/claim transition, and completion should be recognized only after a successful KAH complete transition with required evidence.
4. **Reject instead of rollback:** unexpected node ids, stale revisions, unsatisfied dependencies, or invalid states should not mutate authoritative KAH state. Automatic rollback is deferred.
5. **Topological order, not artificial serialization:** KAH verifies selected-DAG dependency order. If multiple nodes are ready, either may be claimed unless a later KAS policy narrows execution.
6. **Phase-plan as projection:** for workflow-managed runs, phase-plan/checklist evidence must not contradict the workflow instance and transition ledger.

## Shared STRICT task sequence

`STRICT` uses a single cross-repo id sequence. KAH owns only deterministic helper slices.

| Task ID | Repo | Title | Status | KAH boundary |
|---|---|---|---|---|
| `STRICT-001` | KAS | Strict workflow execution SOT and roadmap registration | Completed | Upstream policy/registration. KAH carries companion references and uses this accepted baseline before `STRICT-002`. |
| `STRICT-002` | KAH | Workflow-managed run marker and strict final-gate mode | Planned | Add deterministic workflow-managed metadata/marker validation and fail final when the required instance is absent or mismatched. |
| `STRICT-003` | KAS | Classification route/trigger mandatory orchestration | Planned | Upstream KAS orchestration; KAH consumes the resulting selected workflow evidence only. |
| `STRICT-004` | KAH | Node claim ledger and transition-order verification | Planned | Harden node start/complete/block audit events and verify transition order against the selected DAG. |
| `STRICT-005` | KAS | Dispatch packet expected-revision and node execution guard | Planned | Upstream KAS dispatch packet and runner policy; KAH validates expected revisions supplied to node transitions. |
| `STRICT-006` | KAH | Phase-plan projection and workflow consistency gate | Planned | Validate workflow-managed phase-plan/checklist projection against KAH workflow instance and transition ledger. |
| `STRICT-007` | KAS | Strict orchestration skill/templates/e2e adoption | Planned | Upstream KAS skill/template adoption; KAH supplies deterministic test and gate surfaces only as implemented. |

## Planned KAH behavior by task

### STRICT-002 — workflow-managed run marker and strict final-gate mode

KAH should support a run-local marker or run metadata fields such as:

```json
{
  "workflow_managed": true,
  "strict_workflow_order": true,
  "selected_workflow_id": "development_full",
  "workflow_source": ".kkachi/runs/<run_id>/workflow/workflow.yaml"
}
```

Final gate behavior should be:

- workflow-managed run with no workflow instance: fail closed;
- selected workflow id/source mismatch: fail closed;
- incomplete workflow instance or missing required node evidence: fail closed;
- non-workflow-managed run: retain existing not-applicable behavior.

### STRICT-004 — node claim ledger and transition-order verification

KAH should treat `workflow node start` as the claim/admission event. A strict transition event should preserve enough state to reconstruct order, such as run id, workflow id, node id, previous revision, resulting revision, expected revision, transition kind, state, and relevant ready/dependency context.

Final/diagnostic verification should detect at least:

- start attempted before dependencies succeeded;
- complete without a running start;
- transition for a node not in the selected workflow;
- stale revision transition;
- succeeded node restarted without explicit future repair semantics;
- downstream node completed before upstream dependency succeeded.

### STRICT-006 — phase-plan projection and workflow consistency gate

For workflow-managed runs, KAH should validate phase-plan/checklist consistency as deterministic projection evidence:

- workflow node pending/running should not be represented as phase complete;
- selected-workflow omitted phases should be skipped/not_applicable with reasons rather than silently required;
- docs-only/light workflows should not fail because development-only phases are absent;
- phase-plan completion must not override an incomplete KAH workflow instance.

## Deferrals and non-goals

- No KAH task classification, workflow selection, agent assignment, prompt authoring, backend selection, or review adjudication.
- No automatic rollback/revert/checkpoint behavior in this epic unless a later explicitly approved task adds worktree isolation semantics.
- No realtime watcher/notification surface.
- No warning-only strict mode: strict helper checks must return pass/fail/not_applicable or structured fail-closed diagnostics.
- No provider/model/gateway/auth/token/profile mutation, KAB activation, release, install, push, or commit authorization by this SOT alone.

## Acceptance criteria for KAH docs registration

- This SOT exists and states KAH's deterministic-helper boundary for strict workflow order enforcement.
- KAH `docs/roadmap.md` registers KAH-owned `STRICT-002`, `STRICT-004`, and `STRICT-006` with shared cross-repo numbering.
- KAH `docs/README.md` and `docs/kkachi-docs-map.yaml` reference this SOT.
- Cross-links to KAS `STRICT-001` / `strict-workflow-execution-contract.md` are present.
- Verification includes docs readback, docs-map YAML parse, `git diff --check`, and repository test command or explicit blocker/degraded reason.

## Next action

After `STRICT-001` review acceptance, implement `STRICT-002` so KAH can distinguish ordinary runs from workflow-managed strict runs and fail final readiness when required workflow-instance evidence is absent or mismatched.
