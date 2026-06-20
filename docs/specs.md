# kkachi-agent-helper Specs

Date: 2026-04-30
Owner: KAH maintainers
Status: source of truth for implemented helper behavior; graph init, validation/explanation, semantic diff, proposal records, approval-gated apply, non-authoritative graph export, compatibility diagnostics, configurable feedback-intake activation evidence, and graph-policy-driven phase-plan feedback validation are implemented; graph workflow sync diagnostics now also expose stable reason codes; DAGSM-001 task-DAG schema validation/explain, DAGSM-002 run-local workflow instance state, DAGSM-003 multi-DAG catalog diagnostics/final-gate integration, and DAGSM-006 explicit workflow catalog proposal/apply substrate are implemented source-side; token-001/token-002 token-economy evidence gating and MAREV-002 multi-agent-review evidence gate/schema support are implemented
Planning graph update date: 2026-05-24
Planning graph evidence: governance evidence record in kanban task `t_2fb00394`

## 1. Purpose

`kkachi-agent-helper` is the deterministic command-line helper for the Kkachi software delivery harness. It owns local project state, run artifacts, locks, schema validation, event logging, and initialization of Kkachi project scaffolding. It does not plan work, choose a coding backend, review code, or act as an intelligence layer.

The helper exists so Hermes team members and external coding CLIs can operate through a repeatable, auditable workflow without relying on chat memory or prompt claims as the source of truth. KAH owns deterministic state only after KHS or a user chooses to apply the Kkachi workflow; it does not decide whether KHS should trigger.

## 2. Repository role

Kkachi is split into three independently versioned repositories:

| Repository | Responsibility |
|---|---|
| `kkachi-agent-bridge` | Runtime integration with external AI coding CLIs. |
| `kkachi-agent-helper` | Deterministic state, artifact, schema, lock, and bootstrap tooling. |
| `kkachi-hermes-skills` | Hermes phase skills, orchestration skills, templates, registries, and evaluation assets. |

`kkachi-agent-helper` must stay small, local-first, scriptable, and safe to call from agents, shell scripts, and future UI surfaces. KHS owns workflow policy, phase applicability, checklist normalization, and release recommendations; KAH owns deterministic helper state, artifacts, schemas, gates, events, locks, diagnostics, and command-surface validation.

## 3. Non-goals

The helper must not:

- decide whether KHS should trigger for a user request;
- decide which external backend is best for a task;
- generate implementation plans using model reasoning;
- decide KHS phase applicability, phase ordering, or checklist normalization policy;
- replace Hermes skills or project overlays;
- install Hermes/KHS skill content;
- replace `kkachi-agent-bridge` session control;
- mutate user source files except explicitly managed Kkachi scaffold files;
- hide failed checks behind best-effort warnings when a gate requires fail-closed behavior;
- require a network service for the default local workflow.

## 4. Design principles

1. **Deterministic first.** The same inputs should produce the same state transitions and validation results.
2. **Fail closed.** Missing required fields, invalid schemas, unsafe paths, lock conflicts, unsupported versions, and ambiguous run resolution must fail.
3. **One active write lane.** A project may have multiple preserved runs, but the default policy allows only one active write run per repository.
4. **Artifacts over claims.** Work is complete only when required artifacts and status fields prove it.
5. **Append-only history.** `events.jsonl` records state changes and checks; it is not silently rewritten.
6. **Project-local ownership.** All runtime state lives under the target repository's `.kkachi/` directory unless explicitly exported.
7. **Versioned contracts.** Status, metadata, event, and artifact schemas carry versions and compatibility rules.
8. **Agent-friendly UX.** Commands must return clear exit codes, compact human output, and optional JSON output.

## 5. Canonical project layout

Default target-project layout:

```text
.kkachi-workflow.yaml          # project-level workflow graph artifact; init/validate/explain/diff/apply implemented, proposal records stored under .kkachi/graph/proposals
.kkachi/
  config.yaml                  # helper runtime/config only; never workflow graph SOT
  status.json
  events.jsonl
  active_run.lock
  project_write.lock
  schemas/
    config.schema.json
    status.schema.json
    event.schema.json
    run-metadata.schema.json
    selected-cli.schema.json
    bridge-session-snapshot.schema.json
    token-economy-evidence.schema.json
    multi-agent-review-evidence.schema.json
    policy-promotion-evidence.schema.json
  capabilities/               # planned capability cache/evidence, not native inventory SOT
    current.json
    snapshots/<snapshot_id>.json
    reports/<refresh_report_id>.json
    fingerprints/<fingerprint_id>.json
    drift/<drift_report_id>.json
  runs/
    <run_id>/
      run-metadata.json
      phase-plan.yaml
      intake-classification.md
      sot-basis.md
      task-brief.md
      acceptance-criteria.md
      plan.md
      checklist.md
      selected-cli.json
      capability-snapshot.json
      capability-check.md
      bridge-session-snapshot.json
      bridge-events.md
      prompt.md
      context-pack.md
      cli-output.md
      diff.patch
      impl-log.md
      test-log.md
      verification.md
      token-economy-evidence.json
      multi-agent-review/
        status.json
      policy-promotion-evidence.json
      review.md
      docs-update.md
      sot-update.md
      roadmap-update.md
      improvement-note.md
      feedback-request.md
      feedback-1.md
      feedback-triage-1.md
      handle-feedback-1.md
      redteam/
        plan-review.md
        impl-review.md
        test-review.md
        qa-review.md
        shaping-review.md
        final-gate-review.md
      discovery/
        existing-docs-review.md
        problem-framing.md
        research-notes.md
        strategy-options.md
        selected-strategy.md
        task-breakdown.md
        implementation-readiness.md
        handoff-to-development.md
      final-report.md
      gate-reports/
        <gate>.json
```

Light mode may use the same artifact names with shorter content or explicit not-applicable records. It must not introduce an incompatible artifact schema without a versioned migration.

### Project workflow graph note

Status: `graph-011` external feedback intake migration, diagnostics, and capability activation evidence implemented. This file is the permanent behavior SOT for `.kkachi-workflow.yaml`, the graph command surface, phase-plan validation, and graph compatibility diagnostics.

| Path / artifact | Meaning | Owner | Authority |
|---|---|---|---|
| `.kkachi-workflow.yaml` | Project-level workflow graph instance | KHS proposes policy/templates; KAH initializes/validates/explains/diffs, records proposal evidence, applies approved proposal records, and reports compatibility diagnostics | Project graph file for implemented init/validation/explanation/diff/proposal/apply evidence |
| `.kkachi/config.yaml` | KAH helper runtime/configuration | KAH | Helper config only; never workflow graph SOT |
| `.kkachi/` | Runtime state, evidence, events, locks, schemas, run artifacts | KAH | Runtime/evidence substrate |
| `.kkachi/runs/<run_id>/phase-plan.yaml` | Run-local execution state/evidence for a KHS run | KHS content stored/validated by KAH | Run-local workflow/execution state; not project graph replacement |
| `.kkachi/config/workflows/` | Kkachi v2 workflow runtime config if present | Kkachi v2, not KAH/KHS graph docs | Out of KAH/KHS graph scope; no merge/fallback |
| Mermaid/PlantUML exports | Generated visualization | KAH export command | Non-authoritative artifact only |

`phase-plan.yaml` remains run-local execution state/evidence and is not deprecated. If project graph, KHS phase policy, and run-local phase state conflict, KAH/KHS fail closed and require responsible role confirmation before acting. KAH must not read Kkachi v2 `.kkachi/config/workflows/` as fallback graph policy, merge it silently, or treat it as equivalent to `.kkachi-workflow.yaml`.

Graph source and evidence precedence is explicit. Applied `.kkachi-workflow.yaml` state whose checksum/version matches KAH graph audit evidence is the effective graph authority, followed by graph init/proposal/apply audit records, then run-local `phase-plan.yaml` for one run's execution state, and `.kkachi/config.yaml` for helper config only. KHS defaults and explicit candidate graph files are init/proposal/diff inputs only; inspection commands do not make a candidate file authoritative. Generated Mermaid/PlantUML diagrams, stale `.kkachi/` runtime state, KHS defaults, Kkachi v2 `.kkachi/config/workflows/`, and `.kkachi/config.yaml` are never fallback graph authority.

KAH fails closed when graph-managed workflow support is required but `.kkachi-workflow.yaml` is missing, invalid, ambiguous, duplicated, or conflicts with KHS phase policy or run-local `phase-plan.yaml`; when direct manual edits lack validation/proposal/apply/audit evidence; when candidate graph changes affect gates, approvals, review policy, or dependencies without approval/audit evidence; or when KHS asks KAH to use imperative workflow-policy commands.

The built-in `policy-promotion` gate validates POLPR-007 helper evidence for `task_id=POLPR-007` using canonical `.kkachi/runs/<run_id>/policy-promotion-evidence.json` and embedded/exported `policy-promotion-evidence` schema version `polpr007.v1`. KAH checks deterministic presence and shape only: document impact map refs, project-Gray coverage refs, test-layer labels, failed-test repair ownership fields, final stale-status surfaces, KAS ownership boundary evidence, mutation approval evidence, repository-confined refs, checksums, markers, and required `not_applicable` reasons. KAH does not judge policy quality or review sufficiency.

Workflow graph gates are additive to built-in gates. Built-in gate names are evaluated first; otherwise `gate check <run_id> <gate_id>` falls back to a graph-declared gate with deterministic `checks`. Existing gates that declare only `id` and `requires` remain valid graph rows. A graph gate may set `final_required: true`, causing `gate final` to require that graph gate's current pass report and freshness. Supported check types are fixed: `artifact.exists`, `markdown.field`, `text.contains`, `text.contains_all`, `gitignore.contains_all`, `codegraph.evidence`, and `phase.status`; no regex, expression language, command execution, or fallback policy is supported. Optional check `name`, `message`, and `hint` fields only shape the emitted gate report and do not change evaluation semantics.

`gitignore.contains_all` reads a repository-relative ignore file, defaulting to `.gitignore` when `path` is omitted, and requires every configured `tokens` entry to be present as a non-comment ignore line. Directory entries accept either trailing-slash or non-trailing-slash spelling. `codegraph.evidence` reads a run-local artifact, defaulting to `.kkachi/runs/<run_id>/codegraph-evidence.md`, requires a markdown `Status` value allowed by `one_of` or by the default `complete,degraded`, and requires at least one configured evidence marker token or one of the default markers: `codegraph index`, `codegraph init -i`, `codegraph unavailable`, or `codegraph deferred`. KAH validates this evidence; KAS/Hermes remains responsible for deciding when to run `codegraph index <repo>`, when an empty bootstrap project may defer indexing, and when to run `codegraph init -i <repo>` after source files exist.

`EXTERNAL_FEEDBACK_INTAKE` graph policy is implemented for graph schema validation/projection, phase-plan feedback-bound validation, explicit stale-bound migration evidence, diagnostics, and capability activation evidence. When present, `.kkachi-workflow.yaml` must declare:

```yaml
feedback_intake:
  policy: "EXTERNAL_FEEDBACK_INTAKE"
  schema_version: "external-feedback-intake/v1"
  min_rounds: 1
  max_rounds: 5
  required_rounds: [1]
  optional_rounds: [2,3,4,5]
```

Missing, duplicate, unsupported, conflicting, stale `max3`/`1..3`, or round 6+ declarations fail closed. Absence of `feedback_intake` remains valid graph input with no graph-declared feedback policy, but `phase-plan validate` fails closed rather than accepting feedback phase rows without a valid graph feedback policy. KAH projects valid declarations through `graph validate --json`, `graph explain --json`, and `graph diff --json`; `graph diff` marks changed bounds with `feedback_intake_changed`. A project whose default `.kkachi-workflow.yaml` fails only with `feedback_intake_stale_bounds` may record a migration proposal when the candidate graph is otherwise valid, declares canonical `1..5` feedback bounds, and changes no other graph semantics; `graph apply` still requires matching proposal/base/candidate checksums and explicit `--approval <evidence-ref>`. KHS activation must require both `workflow_graph_configurable_feedback_intake=true` in `capabilities --json` and diagnostics `graph_compatibility.feedback_intake.status=="pass"`. `.kkachi/config.yaml`, generated diagrams, stale `.kkachi/` runtime state, KHS defaults, Kkachi v2 `.kkachi/config/workflows/`, and KAB runtime state are never fallback authorities for feedback bounds.

Graph workflow sync diagnostics expose stable `reason_codes` arrays in `graph validate --json`, `graph explain --json`, nested `validation_summary`, proposal `base`/`candidate` endpoints, and `diagnostics export` `graph_compatibility`. The initial compatibility vocabulary is: `graph_missing`, `graph_valid`, `graph_invalid_schema`, `graph_parse_error`, `graph_source_precedence_violation`, `graph_checksum_mismatch`, `graph_audit_missing`, `graph_feedback_intake_missing`, `graph_feedback_intake_stale_bounds`, `graph_feedback_intake_invalid`, `graph_manual_edit_unverified`, `graph_conflicts_with_phase_plan`, `graph_apply_requires_approval`, `graph_repair_candidate_supported`, `graph_repair_candidate_unsupported`, and `forbidden_fallback_source_detected`. Missing, stale, broken, or otherwise invalid project `.kkachi-workflow.yaml` states that can be replaced by a valid complete candidate graph report `graph_repair_candidate_supported`; forbidden fallback sources remain unsupported. KAS decides its supported envelope, update guidance, and policy meaning, and must inspect nested diagnostics such as `feedback_intake.reason_codes` and `forbidden_fallback_sources[].reason_code` when it needs those specialized facts. Valid custom graphs are not rejected merely because they differ from the KAS default template. Approval-gated complete-candidate repair is implemented through `graph propose`/`graph apply`; direct YAML edit fallback remains forbidden.

### Task-DAG validation note

Status: `DAGSM-001` task-DAG schema validation/explain, `DAGSM-002` run-local workflow instance state, and `DAGSM-003` multi-DAG catalog diagnostics/final-gate integration are implemented. `workflow validate --file <workflow.yaml> --json` and `workflow explain --file <workflow.yaml> --json` expose stable JSON with `status`, `ok`, `reason`, `reason_codes`, `workflow_id`, `schema_version`, `diagnostics`, `nodes`, and `edges`. Invalid DAGs return exit code 3 with validation data on stdout; unsafe repository-relative paths and unreadable files fail closed through structured errors. The supported MVP schema is `schema_version: task-dag/v1`, `workflow_id`, and `nodes` containing `id`, `depends_on`, `join: all_of`, and `required_outputs`. KAH checks duplicate/missing ids, unknown dependencies, cycles, unsupported joins, required output presence, and repository-confined output paths. `workflow explain` is read-only projection evidence only.

`workflow create --run <run_id> --file <workflow.yaml> --json` creates `.kkachi/runs/<run_id>/workflow-instance.json` from a valid task-DAG file. `workflow show` and `workflow ready` expose persisted state and deterministic ready-node calculation. `workflow node start|complete|block` enforces pending/running/succeeded/blocked FSM transitions, refuses stale `--expect-revision` updates, checks declared `required_outputs` and repository-confined evidence paths before completion, increments revision, and appends workflow audit events.

Successful `workflow.node.started`, `workflow.node.completed`, and `workflow.node.blocked` events carry strict transition ledger payloads. The event top-level `run_id` and payload `run_id` must match. Payloads include `workflow_id`, `node_id`, `transition_kind`, `previous_revision`, `resulting_revision`, optional `expected_revision`, `previous_state`, `resulting_state`, `dependency_states`, `ready_before`, and `ready_after`, plus bounded transition-specific evidence such as completion evidence or block reason when supplied. Rejected command-path transitions remain reject-before-mutation: stale expected revisions, unknown nodes, non-ready starts, invalid FSM transitions, unsafe/missing completion evidence, and empty block reasons must not append rejected-attempt events.

`gate final` includes a distinct `workflow_transition_order` check, and `diagnostics export` includes `workflow_transition_order` when a run has workflow instance context. Transition-order verification reconstructs successful transition events for the run/workflow from `.kkachi/events.jsonl`, enforces contiguous revisions, validates pending->running, running->succeeded, and non-succeeded->blocked FSM transitions, enforces dependency order, and correlates the latest workflow instance revision plus each node `last_transition_event_id` with the ledger. It does not claim cross-file atomicity between `workflow-instance.json` and `.kkachi/events.jsonl`; incoherent state fails closed. Stable reason codes include `workflow_transition_order_valid`, `workflow_transition_order_invalid`, `workflow_transition_payload_malformed`, `workflow_transition_instance_invalid`, `workflow_transition_instance_run_mismatch`, `workflow_transition_node_unknown`, `workflow_transition_workflow_mismatch`, `workflow_transition_revision_stale`, `workflow_transition_revision_gap`, `workflow_transition_start_before_dependencies`, `workflow_transition_complete_without_start`, `workflow_transition_succeeded_node_restarted`, and `workflow_transition_instance_event_mismatch`.

`workflow catalog validate --file <catalog.yaml> --json` and `workflow catalog explain --file <catalog.yaml> --workflow-id <id> --json` validate `.kkachi/workflow-catalog.yaml` style project multi-DAG catalogs without selecting a workflow. Catalog entries declare explicit `workflow_id`, repository-confined task-DAG `path`, `schema_version: task-dag/v1`, and optional `node_contract_registry` evidence. `workflow create --run <run_id> --catalog <catalog.yaml> --workflow-id <id> --json` creates a workflow instance only when the operator/KAS supplies an explicit workflow id resolving to exactly one valid catalog entry; mixed `--file` and catalog mode fails closed with `workflow_catalog_explicit_mode_conflict`. Optional KAS WFLOW-003 node-contract registry validation checks deterministic structural evidence such as node coverage, `task_class`, `completion_authority: kah_only`, and `direct_kah_state_write: false` while ignoring KAS-owned selector metadata. `diagnostics export` includes `workflow_catalog` and run-local `workflow_instance` completeness evidence. `gate final` treats missing workflow-instance evidence as not applicable only for non-workflow-managed runs. Runs with `workflow_managed=true` fail closed when workflow-instance state is absent, when declared `selected_workflow_id` or `workflow_source` metadata is missing, when declared `selected_workflow_id` mismatches the instance `workflow_id`, when declared `workflow_source` mismatches instance `source_path`, or when any node is incomplete or required output/evidence paths are missing at final-gate time.

`workflow catalog propose --packet <repo-relative-kas-workflow-promote-packet.json> --reason <text> --json` records an explicit project-local promotion proposal for KAS WFLOW-009 packets. KAH accepts KAS-supplied complete candidate content only; it does not classify tasks, rank selectors, choose bundles, generate trigger policy, assign agents, execute backends, or decide whether promotion is semantically preferred. The packet must use `schema_version: kas-workflow-promote-packet/v1`, contain no conflicts or error diagnostics, prove no-write posture, and supply complete generated content for the workflow DAG, `.kkachi/workflow-catalog.yaml`, and node-contract registry, plus optional thin-trigger evidence. Target paths are restricted to `.kkachi/workflows/*.yaml`, `.kkachi/workflow-catalog.yaml`, and optional `.kkachi/workflow-triggers/*/SKILL.md`; unsafe, duplicate, ambiguous, mixed, or unsupported paths fail closed. Proposal evidence is stored under `.kkachi/workflow-catalog/proposals/wcat-prop-*.json` and includes schema/version, proposal id/path, canonical proposal hash, source packet reference and approval hash, target paths, base checksums, candidate checksums, changed paths, validation summary, conflicts, diagnostics, approval requirements, and no-write evidence. Proposal recording appends `workflow_catalog.proposal_recorded` but does not write target workflow catalog files.

`workflow catalog apply --proposal <proposal-id> --approval <evidence-ref> --proposal-hash sha256:<64hex> --json` applies only a recorded passing proposal. It fails closed before backup, target write, or `workflow_catalog.applied` audit event when approval evidence is missing, proposal hash is missing/malformed/mismatched, source KAS approval hash binding is absent or inconsistent, proposal identity/path/schema/status is invalid, current base checksums drift, candidate checksums drift, candidate DAG/catalog/node-contract content no longer validates, target paths are unsafe or ambiguous, helper status/events are incoherent, or backup/write validation fails. When the source KAS packet has an approval hash, `--approval` must be `dry-run:<hash>`, the hash itself, a ref containing the hash, or a repository-confined evidence file containing that hash. Apply creates backup/recovery evidence under `.kkachi/backups/workflow-catalog-promotions/<proposal-id>/<event-id>-<basename>` for existing targets, writes approved targets atomically under repository-confined paths, returns new checksums, and appends `workflow_catalog.applied`.

KAH still does not decide KAS workflow policy, select agents, rank workflows, choose fallbacks, execute backends, create dynamic nodes, automate retry/rollback, apply catalogs automatically, or mutate KAB/Kanban/Hermes profile/provider/gateway/auth/token/model state.

### Planned capability cache/evidence note

Status: docs/design lock for future capability storage. Implementation, schemas, and commands remain separately gated.

| Path / artifact | Meaning | Owner | Authority |
|---|---|---|---|
| `.kkachi/capabilities/current.json` | Current effective project snapshot pointer/copy for list/read | KAH stores KAB output | Cache/evidence only; not backend-native inventory SOT |
| `.kkachi/capabilities/snapshots/<snapshot_id>.json` | Immutable raw KAB capability snapshot | KAB produces; KAH persists | Raw discovery evidence with fingerprints |
| `.kkachi/capabilities/reports/<refresh_report_id>.json` | Refresh report with scanned sources, changes, failures, and next steps | KAB produces; KAH persists | Refresh audit evidence |
| `.kkachi/capabilities/fingerprints/<fingerprint_id>.json` | Source/cache key fingerprint evidence | KAB produces; KAH persists | Drift/freshness evidence |
| `.kkachi/capabilities/drift/<drift_report_id>.json` | Snapshot comparison and stale/conflict markers | KAB/KAH planned | Drift evidence |
| `.kkachi/runs/<run_id>/capability-snapshot.json` | Run-local copy/ref of snapshot used for backend/prompt decision | KHS content stored by KAH | Run evidence, not new callability authority |
| `.kkachi/runs/<run_id>/capability-check.md` | Human-readable capability selection/check record | KHS content stored by KAH | Operator/reviewer evidence |

KAH owns project-local persistence, atomic writes, validation, diagnostics, and audit events for these paths after schema acceptance. KAH must not perform unbounded backend-native scans, infer callability from KHS semantic guidance, or treat cached `.kkachi/` records as the only source for `capability refresh`. Refresh remains a KAB raw discovery action over bounded backend-native sources; KHS semantic enrichment may annotate raw snapshots only with explicit trust labels and review-gated promotion.

## 6. Core state files

### `.kkachi/config.yaml`

Repository-local helper configuration.

Required conceptual fields:

| Field | Purpose |
|---|---|
| `version` | Config schema version. |
| `project.name` | Stable project name. |
| `project.root_policy` | Path and symlink safety policy. |
| `paths.run_root` | Relative run artifact root, default `.kkachi/runs`. |
| `paths.status_file` | Relative status path, default `.kkachi/status.json`. |
| `paths.events_file` | Relative event log path, default `.kkachi/events.jsonl`. |
| `locks.one_active_write_run` | Whether `project_write.lock` is enforced. Default `true`. |
| `schemas.mode` | Use embedded schemas, project-local schemas, or both. |
| `compat.required_skills` | Compatible `kkachi-hermes-skills` version range, when known. |
| `compat.required_bridge` | Compatible `kkachi-agent-bridge` version range, when known. |

### `.kkachi/status.json`

Current project-level helper status. It summarizes the active run and gate state. It is not a replacement for run-local artifacts.

Minimum fields:

| Field | Type | Purpose |
|---|---|---|
| `version` | string | Status schema version. |
| `project_id` | string | Stable project identity. |
| `active_run_id` | string or null | Current active run. |
| `active_run_state` | string or null | Current active run lifecycle summary. In `runwf-001`, this is `active` while a run is active and `null` otherwise; later gate workflows may add richer gate-phase summaries. |
| `last_event_id` | string | Latest appended event id. |
| `updated_at` | string | ISO timestamp from helper clock. |
| `gate_summary` | object | Current pass/fail/blocked summary. |

### `.kkachi/events.jsonl`

Append-only event stream. Each line is one JSON object.

Minimum fields:

| Field | Purpose |
|---|---|
| `version` | Event schema version. |
| `event_id` | Stable event id. |
| `occurred_at` | ISO timestamp. |
| `run_id` | Related run id, when applicable. |
| `type` | Event type. |
| `actor` | `helper`, `commander`, `bridge`, `reviewer`, or `operator`. |
| `payload` | Event-specific data. |

`project init` writes the first event as `evt-000001` with type `project.initialized`. `event append` allocates later ids as zero-padded sequential values (`evt-000002`, `evt-000003`, and so on) through `evt-999999`; exhaustion fails closed until a future migration widens the id policy. It appends exactly one JSONL line and advances `status.last_event_id` plus `status.updated_at`. Before appending, the helper verifies that `status.last_event_id` matches the tail event id in `events.jsonl`; a mismatch fails closed without appending another event. If the event append succeeds but the later status advance fails, the project is intentionally left fail-closed for `project doctor` or a future recovery command instead of silently rewriting history.

Initial event types:

- `project.initialized`
- `run.created`
- `run.activated`
- `run.locked`
- `artifact.written`
- `artifact.validated`
- `gate.checked`
- `gate.failed`
- `gate.passed`
- `run.blocked`
- `run.closed`
- `run.aborted`
- `schema.exported`
- `schema.migrated`
- `phase_plan.initialized`
- `phase_plan.updated`
- `approval.requested`
- `approval.recorded`

## 7. Run metadata

`run-metadata.json` records how the work is classified and who owns it.

Minimum fields:

| Field | Allowed values or shape |
|---|---|
| `version` | schema version |
| `run_id` | helper-generated id using `run-YYYYMMDDTHHMMSSZ-<12hex>` |
| `task_id` | roadmap task id or null |
| `title` | short title |
| `work_path` | `A_development_execution` or `B_discovery_shaping` |
| `work_mode` | `standard` or `light` |
| `urgency` | `normal`, `urgent`, or `critical` |
| `sot_policy` | `existing_sot_basis`, `minimal_sot_before_code`, or `full_sot_before_code` |
| `execution_mode` | `production_write`, `adapter_qa`, `readiness_hardening`, `research`, `verification`, or `docs_only` |
| `backend_evidence` | `required` or `not_applicable`; created from `--backend-evidence auto|required|not_applicable` |
| `commander` | assigned team-member profile name |
| `redteam` | assigned red-team profile name or null |
| `created_at` | ISO timestamp |
| `state` | `created`, `active`, `closed`, or `aborted` |
| `required_artifacts` | list of artifact paths required by gates |
| `gate_state` | per-gate pass/fail/block summary |

Run id lookup accepts an exact run id, or a prefix only when it resolves to exactly one run. Missing and ambiguous prefixes fail closed. Existing `0.1` run metadata without `backend_evidence` is read as `not_applicable` for backward compatibility.

Run lifecycle commands in `runwf-001` use these state transitions. In `runwf-002`, mutating run lifecycle commands are additionally serialized by the lock policy in [Locking](#11-locking).

- `run create` records `.kkachi/runs/<run_id>/run-metadata.json` with `state: "created"`, resolved `backend_evidence`, `required_artifacts: []`, and `gate_state: {}` as part of the `run.created` lifecycle event. If metadata recording fails after the event line is appended, `status.last_event_id` is not advanced, leaving an explicit status/event mismatch for fail-closed recovery.
- `run activate <run_id>` only accepts `created` runs and records metadata `state: "active"`, `status.active_run_id`, and `status.active_run_state` as part of the `run.activated` lifecycle event. If another run is already active, it fails closed.
- `run close <run_id>` and `run abort <run_id>` only accept `created` or `active` runs and record metadata `state: "closed"` / `"aborted"` as part of the `run.closed` or `run.aborted` lifecycle event. If the target is active, they clear the status active-run fields in the same status update.
- Before run lookup or mutation, the helper verifies that `status.last_event_id` matches the event log tail. Mismatch fails closed without starting another run transition. Lifecycle commands append the event before advancing status so any post-event metadata/status write failure is surfaced as a status/event mismatch rather than silently disappearing.

`runwf-001` does not define artifact manifests or baseline artifact files; those remain scoped to `runwf-003`. `runwf-002` adds transient lock-file enforcement around mutating project and run lifecycle operations.

`runwf-003` initializes the canonical run artifact home after run creation:

- `artifact init <run_id>` resolves full run ids or unique prefixes through the same run lookup policy as `run show`.
- `artifact init` is a mutating helper-state command and is serialized by `.kkachi/project_write.lock`.
- Before writing artifacts, `artifact init` verifies status/event-log coherence and refuses to mutate when `status.last_event_id` does not match the event tail.
- `artifact init` only accepts runs in `created` or `active` state. Closed and aborted runs are preserved read-only.
- The command derives `required_artifacts` from `work_path`, `work_mode`, `execution_mode`, resolved `backend_evidence`, and `redteam`, ordered by the canonical artifact list in [Canonical project layout](#5-canonical-project-layout).
- The command creates baseline non-empty files for every canonical run artifact listed in the layout, including nested `redteam/` and `discovery/` artifacts.
- Existing non-empty artifact files are preserved exactly. Existing empty artifact files are reinitialized with baseline content.
- On success, the command updates `run-metadata.json.required_artifacts`, appends one `artifact.written` event, and advances `status.last_event_id`.
- If an artifact write succeeds but the later metadata/status update fails, the project is intentionally left fail-closed through the existing status/event coherence checks rather than silently rewriting history.
- `artifact list <run_id> [--json]` is read-only. It does not append events, create files, repair files, create locks, remove locks, or rewrite metadata. It reports every canonical artifact path with required/present/empty/byte status.
- `artifact write <run_id> <artifact_path> --from <repo-relative-file> [--json]` atomically replaces or creates one canonical run artifact from exact source bytes.
- `artifact append <run_id> <artifact_path> --from <repo-relative-file> [--json]` atomically appends exact source bytes to one canonical run artifact, creating it if absent.
- `artifact set-status <run_id> <artifact_path> --status <pending|complete|not_applicable> [--reason <text>] [--json]` atomically updates markdown `Status:` fields; `not_applicable` requires a non-empty reason. Patch artifacts do not support status mutation. Schema-owned backend JSON artifacts such as `selected-cli.json` and `bridge-session-snapshot.json` reject generic lifecycle status updates with `artifact_status_not_applicable`; use `artifact write` with valid backend JSON schema fields and rely on `gate check backend` for completion validation.
- Artifact mutation commands are serialized by `.kkachi/project_write.lock`, refuse closed/aborted runs, verify status/event-log coherence before mutation, reject unsafe source paths, and only target canonical artifact paths. Unmanaged KHS supplemental paths are rejected in this release while direct-file compatibility remains available during migration.
- Each artifact mutation records an `artifact.written` event with `operation`, `path`, `artifact_kind: "canonical"`, byte count, and source/status metadata as applicable.

`runwf-004` adds read-only intake validation before later gate commands exist:

- `artifact validate <run_id> [--gate intake]` resolves full run ids or unique prefixes through the same run lookup policy as `run show`. Omitting `--gate` validates `intake`. Unknown gates fail as usage errors until the `gates` epic implements the wider gate surface.
- Validation does not append events, create files, repair files, create locks, remove locks, or rewrite metadata. Passing validation exits `0`; failed validation exits `3` and returns a report with `run_id`, `gate`, `status`, and `checks`.
- Intake validation requires `run-metadata.json.required_artifacts` to match the current manifest, `intake-classification.md` to be a non-empty regular file, `Status: complete`, and exact metadata fields for `Work Path`, `Work Mode`, `SOT Policy`, and `Urgency`.
- Path A (`A_development_execution`) requires `sot_policy=existing_sot_basis`. Path B (`B_discovery_shaping`) requires `sot_policy=minimal_sot_before_code` or `full_sot_before_code`.
- Light mode must retain the safety artifact requirements and record `Light Mode Reason: <reason>` in `intake-classification.md`.
- Explicit not-applicable markdown records use `Status: not_applicable` plus `Reason: <non-empty reason>`. Intake classification itself cannot be marked not applicable.

Initial required-artifact derivation:

| Run metadata condition | Required artifacts added |
|---|---|
| All runs | `intake-classification.md`, `acceptance-criteria.md`, `test-log.md`, `verification.md`, `docs-update.md`, `final-report.md` |
| `work_path=A_development_execution` | `sot-basis.md`, `roadmap-update.md`, `plan.md`, `checklist.md` |
| `work_path=B_discovery_shaping` | `discovery/existing-docs-review.md`, `discovery/problem-framing.md`, `discovery/research-notes.md`, `discovery/strategy-options.md`, `discovery/selected-strategy.md`, `discovery/task-breakdown.md`, `discovery/implementation-readiness.md`, `discovery/handoff-to-development.md`, `sot-update.md`, `roadmap-update.md` |
| `work_mode=standard` | `task-brief.md`, `prompt.md`, `context-pack.md` |
| `execution_mode=production_write` or `readiness_hardening` | `diff.patch`, `impl-log.md`, `review.md`, `redteam/impl-review.md`, `redteam/test-review.md`, `redteam/final-gate-review.md` |
| `execution_mode=adapter_qa` | `selected-cli.json`, `capability-check.md`, `bridge-session-snapshot.json`, `bridge-events.md`, `cli-output.md`, `redteam/qa-review.md` |
| `backend_evidence=required` | `selected-cli.json`, `capability-check.md`, `bridge-session-snapshot.json`, `bridge-events.md` |
| `execution_mode=research` | `discovery/research-notes.md`, `discovery/strategy-options.md`, `discovery/selected-strategy.md` |
| `execution_mode=verification` | `review.md` |
| `execution_mode=docs_only` | `sot-update.md`, `roadmap-update.md` |
| `redteam` assigned | `redteam/plan-review.md`, `redteam/shaping-review.md`, `redteam/final-gate-review.md` |
| `task_id=token-001` or `task_id=token-002` | `token-economy-evidence.json` |
| `task_id` with `mar-` prefix | `multi-agent-review/status.json` |

## 8. Work paths and gates

### Path A: development execution

Used when a durable SOT basis already exists. Required helper checks:

- `sot-basis.md` exists and is not empty.
- Roadmap trace exists or an explicit not-applicable reason is recorded.
- Acceptance criteria exist before implementation evidence is accepted.
- If a bridge backend is used, `selected-cli.json` and `capability-check.md` exist and validate.
- Verification and docs-update decisions are recorded before final gate.

### Path B: discovery and shaping

Used when the SOT basis is missing or incomplete. Required helper checks:

- Request classification artifact exists.
- Discovery artifacts exist or are explicitly marked not applicable.
- Minimal or full SOT update is recorded before Path A handoff.
- Roadmap update or explicit existing trace is recorded.
- Acceptance criteria and handoff readiness are recorded.

### Light mode policy

Light mode reduces depth, not safety. Helper validation must still require:

- work path classification;
- SOT basis or SOT creation;
- roadmap trace or explicit reason;
- acceptance criteria;
- required bridge evidence when bridge is used;
- verification evidence;
- docs-update decision;
- final report.

## 9. Backend evidence validation

When a run uses `kkachi-agent-bridge`, helper validation covers artifact shape only. The commander owns the reasoning. KHS declares the requirement with `run-metadata.json.backend_evidence` (created by `run create --backend-evidence auto|required|not_applicable`), independently of `execution_mode`. `auto` resolves to `required` for `adapter_qa` and `not_applicable` for other modes. The `backend` gate is activated when `backend_evidence` is `required` or when existing `required_artifacts` already contain backend artifacts; otherwise it records a deterministic not-applicable pass instead of inferring backend use from baseline files.

Required files:

| Artifact | Helper responsibility |
|---|---|
| `selected-cli.json` | Validate required fields, supported status values, source ledger reference, and declared caveats. |
| `capability-check.md` | Validate presence and link to selected backend identity. |
| `bridge-session-snapshot.json` | Validate session identity fields such as `session_id`, `backend_type`, `adapter_type`, state, lifecycle class, and open pendings. |
| `bridge-events.md` | Validate presence when backend behavior matters. |

The helper must not override the commander's backend choice. It may fail a gate if backend evidence was declared required but canonical backend artifacts are missing from `required_artifacts`, or if the selected backend record is missing, malformed, stale, or marked unsupported for the declared execution mode. `selected-cli.json` passes only with an object containing non-empty `version`, `status`, `backend_type`, `adapter_type`, and `source_ledger_ref`, plus a declared `caveats` array of strings; `status` must be `supported` or `degraded`. Do not mark `selected-cli.json` complete with `artifact set-status`; that command is rejected for schema-owned backend JSON so the semantic `status` field remains intact. `capability-check.md` and `bridge-events.md` require `Status: complete`; the capability check must mention the selected backend and adapter, and bridge events must include non-empty behavior evidence. `bridge-session-snapshot.json` must match the selected backend/adapter, declare a non-empty `session_id`, `state`, and `lifecycle_class`, and have `open_pendings: 0`.

## 10. CLI surface

Initial command groups:

```text
kkachi-agent-helper project init \
  --project-name kkachi-agent-bridge \
  --stack go \
  --repo-path "$PWD" \
  --commander responsible-approver \
  --redteam required-reviewer \
  --docs-map-roadmap docs/roadmap.md \
  --docs-map-spec docs/specs.md \
  --docs-map-architecture docs/architecture.md \
  --docs-map-adr-dir docs/adr \
  --docs-map-todo-dir docs/todo \
  --docs-map-spec-dir docs/specs \
  --test-commands "go test ./...,make test" \
  --backend-policy codex \
  --execution-mode production_write \
  --sot-policy existing_sot_basis [--force]
kkachi-agent-helper project doctor
kkachi-agent-helper project status [--json]
kkachi-agent-helper project probe-toolchain --json [--project-root <path>]
kkachi-agent-helper run create --title <title> --work-path <A_development_execution|B_discovery_shaping> --work-mode <standard|light> --urgency <normal|urgent|critical> --sot-policy <existing_sot_basis|minimal_sot_before_code|full_sot_before_code> --execution-mode <production_write|adapter_qa|readiness_hardening|research|verification|docs_only> --commander <profile> [--backend-evidence <auto|required|not_applicable>] [--task-id <id>] [--redteam <profile>]
kkachi-agent-helper run list [--json]
kkachi-agent-helper run show <run_id-or-prefix> [--json]
kkachi-agent-helper run activate <run_id-or-prefix>
kkachi-agent-helper run close <run_id-or-prefix>
kkachi-agent-helper run abort <run_id-or-prefix>
kkachi-agent-helper artifact init <run_id>
kkachi-agent-helper artifact list <run_id> [--json]
kkachi-agent-helper artifact validate <run_id> [--gate intake]
kkachi-agent-helper gate check <run_id> <gate>
kkachi-agent-helper gate final <run_id>
kkachi-agent-helper event append <type> --run <run_id> --payload <json>
kkachi-agent-helper schema validate <file> --schema <schema>
kkachi-agent-helper schema export [--schema <schema>|--all] [--dry-run]
kkachi-agent-helper schema migrate --from <version> --to <version>
kkachi-agent-helper diagnostics export [--run <run_id-or-prefix>] [--output <repo-relative-path>]
kkachi-agent-helper phase-plan init <run_id-or-prefix>
kkachi-agent-helper phase-plan show <run_id-or-prefix>
kkachi-agent-helper phase-plan set <run_id-or-prefix> <phase-id> --status <pending|in_progress|complete|skipped|not_applicable|blocked> [--evidence <path>] [--reason <text>] [--approval-required true|false]
kkachi-agent-helper phase-plan validate <run_id-or-prefix> [--final]
kkachi-agent-helper approval request <run_id-or-prefix> --phase <phase-id> --reason <reason> [--evidence <ref>]
kkachi-agent-helper approval record <run_id-or-prefix> --phase <phase-id> --decision <approved|rejected> --by <approver> --evidence <ref> [--reason <reason>]
kkachi-agent-helper approval show <run_id-or-prefix> [--phase <phase-id>]
kkachi-agent-helper capabilities --json
kkachi-agent-helper help
kkachi-agent-helper --help
kkachi-agent-helper <command> --help
kkachi-agent-helper project init --help
kkachi-agent-helper run create --help
```

### `capabilities --json`

`align-003` introduces a project-independent capabilities report for KHS activation checks. The command exits `0` on a healthy binary and does not require `.kkachi/` project state. JSON output is the compatibility contract; human output is informational only.

Stable JSON output includes helper build info, `capabilities_schema_version`, embedded `project_schema_version`, supported command groups/subcommands, compatibility booleans, deprecated surfaces, and omitted surfaces. Current compatibility flags report project init/status/doctor, run lifecycle, artifact init/list/validate/mutation, gates, declared backend evidence requirements, diagnostics export, phase-plan support, approval records, read-only workflow graph support, workflow graph init support, workflow graph apply support, workflow graph export support, workflow graph diagnostics support, workflow graph no-direct-YAML-fallback support, workflow graph configurable feedback-intake support, task-DAG schema validation support, workflow instance state support, workflow catalog diagnostics support, workflow catalog proposal/apply support, workflow final-gate integration support, KAS node-contract registry evidence support, token-economy evidence gate support, and MAR evidence gate/schema support as supported; the project command inventory advertises implemented `init`, `status`, `doctor`, and `probe-toolchain` subcommands, the graph command inventory advertises implemented `init`, `validate`, `explain`, `diff`, `propose`, `apply`, and `export` subcommands, and the workflow command inventory advertises implemented `validate`, `explain`, `catalog`, `catalog propose`, `catalog apply`, `create`, `show`, `ready`, and `node` subcommands. The removed `install` command is reported as an omitted surface because Hermes/KHS skill installation belongs to Hermes native tooling. KHS `main` may use KAH `@latest` when the effective binary's report advertises all required surfaces; source-built checkout evidence must not be promoted into installed/effective runtime readiness when the installed binary lacks those surfaces. KHS release tags should publish tested/recommended KAH versions for reproducible historical runs.

### Help UX

`align-004` introduces project-independent help output for top-level discovery, implemented command groups, selected high-argument subcommands (`project init`, `project probe-toolchain`, and `run create`), and the `phase-plan` surface. `help`, `help help`, `--help`, supported `<command> --help`, and supported subcommand help topics exit `0`, write help to stdout, do not require `.kkachi/` state, and list usage, required arguments/options, subcommands, and JSON behavior. Implemented command groups have group help pages, including `schema`, `event`, `lock`, `phase-plan`, `approval`, and `graph`. `--json` with help emits structured help JSON. Machine compatibility checks should continue to use `capabilities --json`; help JSON is supplemental command documentation.

### `gate check`

`gates-001` introduces a small declarative gate registry and the mutating readiness command `gate check <run_id> <gate>`. Run id lookup accepts full ids or unique prefixes through the same policy as `run show`.

Stable JSON output has the following shape:

```json
{
  "run_id": "run-...",
  "gate": "intake",
  "status": "pass|fail|blocked",
  "checks": [
    {
      "name": "required_artifacts",
      "status": "pass|fail|blocked",
      "path": ".kkachi/runs/.../run-metadata.json",
      "message": "...",
      "hint": "...",
      "field": "...",
      "expected": "...",
      "actual": "..."
    }
  ],
  "missing_evidence": [],
  "event_id": "evt-000004",
  "report_path": ".kkachi/runs/run-.../gate-reports/intake.json"
}
```

Behavior in `gates-001` through `gates-005`:

- `intake` is implemented by reusing the deterministic intake checks from `artifact validate`.
- `sot` is implemented by requiring completed `sot-basis.md` for Path A or completed `sot-update.md` for Path B.
- `roadmap` is implemented by accepting a non-empty run metadata `task_id`, completed `roadmap-update.md`, or `roadmap-update.md` with `Status: not_applicable` plus a non-empty `Reason:`.
- `plan` is implemented by requiring completed `acceptance-criteria.md`, `plan.md`, and `checklist.md`.
  KHS owns any checklist normalization needed before writing `checklist.md`; KAH validates only the completed artifacts and does not parse or require KAB-specific planner sections such as `KHS Checklist Seed`.
- `backend` is implemented as a declared/manifest-driven gate. If `backend_evidence` is `required` or `required_artifacts` includes backend evidence, it validates `selected-cli.json`, `capability-check.md`, `bridge-session-snapshot.json`, and `bridge-events.md`; if not, it passes as not applicable with a check tied to `run-metadata.json`.
- `implementation` is implemented by requiring a non-empty `diff.patch`, completed `impl-log.md`, and `cli-output.md` only when the run manifest requires it.
- `review` is implemented by requiring completed `review.md` and every `redteam/*` artifact listed in `required_artifacts`.
- `verification` is implemented by requiring completed `test-log.md` and completed `verification.md` that declares `Verdict: pass` or `Verdict: fail`.
- `docs` is implemented by requiring completed `docs-update.md` that records either `Changed Docs` or `No Change Reason`.
- `final` is implemented by requiring completed `final-report.md` and `pass` status in `metadata.GateState` for `intake`, `sot`, `roadmap`, `plan`, `implementation`, `review`, `verification`, and `docs`. The `backend` gate is also required when `backend_evidence` is `required` or the run manifest includes backend evidence artifacts.
- `token-economy` is implemented for token-001 and token-002. Non-token runs emit `not_applicable`. Token-001 runs require canonical `token-economy-evidence.json` with `schema_version: "token001.v1"`, matching run id and `task_id: "token-001"`, non-empty `task_class`, and sections `scope`, `compact_output_policy`, `artifact_first_detail`, `agent_instruction_evidence`, `final_report_evidence`, `kas_lifecycle_evidence`, and `mutation_approval_evidence`. Section statuses are limited to `pass` or `not_applicable`; every `not_applicable` section requires a non-empty reason. Token-002 runs require `schema_version: "token002.v1"`, matching run id and `task_id: "token-002"`, non-empty `task_class`, and sections `scope`, `verification_profile_evidence`, `phase_packet_evidence`, `review_bundle_evidence`, `watcher_evidence`, `change_verification_matrix_evidence`, and `mutation_approval_evidence`. Token-002 validation checks repository-confined paths, `sha256:<64hex>` checksums, selected gate/runner result records, bounded failure excerpts for failed runner results, token008 phase packet/run summary records, token009 review bundle/watcher terminal evidence, token010 change-verification matrix records, required KAS/KAH boundary notes, and required reasons for `not_applicable` evidence. The gate rejects unsupported schemas, token-002-only fields in token-001 evidence, unsafe paths, malformed JSON, missing required fields, checksum mismatch, missing broad-mutation approval refs, fake runner fields for `not_applicable` gates, missing boundary notes, or unsupported result/applicability/change-class vocabulary. It emits only `pass`, `fail`, or `not_applicable`, and failing outcomes exit `3`.
- `multi-agent-review` is implemented for `task_id` values with the `mar-` prefix or run manifests that explicitly require `multi-agent-review/status.json`; otherwise it emits `not_applicable`. Required MAR runs validate canonical `multi-agent-review/status.json` with `schema_version: "mar-evidence.v1"`, matching run id/task id, status vocabulary `PASS|PASS_WITH_FINDINGS|REQUEST_CHANGES|BLOCKED|DEGRADED|FAILED`, passable top-level status limited to `PASS` or `PASS_WITH_FINDINGS`, required role coverage records, provider attempt records with candidate/status vocabulary and non-clean failure reason evidence, successful-attempt mutation guard fields, repository-confined raw/parsed/disposition refs, optional `sha256:<64hex>` checksums and marker checks, Blue disposition refs, Red adjudication refs when `coverage.red_trigger_summary.red_adjudication_required=true`, alternate approval refs when secondary/alternate provider evidence is counted, waiver refs when role coverage is accepted by explicit waiver, and premium approval refs when premium review is used or a premium attempt is present. KAH does not choose review roles/providers/models, execute reviewers, adjudicate findings, approve waivers, or downgrade failed role coverage to warning-only pass.
- `gate final <run_id>` is implemented with the same lock, event, and status-update contract as `gate check`. It exits `0` on pass and `3` on fail.
- Unknown gate names are usage errors.
- Passing checks append `gate.passed`; failing checks append `gate.failed`; blocked checks append `gate.checked`.
- Every successful `gate check` writes a run-local JSON report to `.kkachi/runs/<run_id>/gate-reports/<gate>.json` with `run_id`, `gate`, `status`, `event_id`, `generated_at`, `report_path`, `checks`, and `missing_evidence`. Re-checking a gate overwrites that gate's report with the latest result.
- Every successful `gate check` updates both `run-metadata.json.gate_state[gate]` and `status.json.gate_summary[gate]` with the status, event id, checked timestamp, and report path.
- A passing gate exits `0`; failed or blocked gates exit `3`.
- `gate check` is serialized by `.kkachi/project_write.lock` and refuses status/event incoherence before mutation.
- `gates-005` regression fixtures cover valid and invalid gate outcomes for Path A Standard, Path A Light, Path B Standard, and Path B Light runs, including malformed evidence and missing artifact cases.

### `schema validate` and `schema export`

`packg-001` introduces deterministic schema validation and schema export.

`schema validate <file> --schema <schema>` JSON output has the following stable shape:

```json
{
  "schema": "status",
  "file_path": ".kkachi/status.json",
  "status": "pass|fail",
  "checks": [
    {
      "name": "last_event_id",
      "status": "pass|fail",
      "path": ".kkachi/status.json",
      "message": "...",
      "hint": "...",
      "field": "last_event_id",
      "expected": "evt-000001-style event id or null",
      "actual": "..."
    }
  ]
}
```

The schema selector accepts canonical embedded names (`config`, `status`, `event`, `run-metadata`, `selected-cli`, `bridge-session-snapshot`, `token-economy-evidence`, `multi-agent-review-evidence`) or canonical project-local schema paths under `.kkachi/schemas/`. Project-local schema paths are identity-checked, but validation remains embedded-registry-backed so a relaxed local schema cannot make invalid helper state pass.

`schema export [--schema <schema>|--all] [--dry-run]` JSON output has the following stable shape:

```json
{
  "dry_run": false,
  "schemas": ["status"],
  "written": [".kkachi/schemas/status.schema.json"],
  "unchanged": [],
  "would_write": [],
  "event_id": "evt-000002"
}
```

Dry-run exports are read-only and report `would_write` without an event. Real exports are serialized by `.kkachi/project_write.lock`, refuse status/event incoherence, replace only canonical schema files, and append `schema.exported` only when at least one schema file changes.

`schema migrate --from <version> --to <version> [--dry-run]` JSON output has the following stable shape:

```json
{
  "dry_run": false,
  "from_version": "0.1",
  "to_version": "0.1",
  "status": "pass",
  "migration": "noop-0.1-to-0.1",
  "would_backup": [".kkachi/status.json"],
  "backed_up": [".kkachi/status.json"],
  "backup_path": ".kkachi/backups/schema-migrations/20260503T000000Z-0.1-to-0.1",
  "would_migrate": [],
  "migrated": [],
  "unchanged": [".kkachi/status.json"],
  "event_id": "evt-000002"
}
```

`packg-002` registers the first `0.1 -> 0.1` no-op migration. Dry-run migrations are read-only and report backup/migration intent without taking a lock, writing backups, or appending an event. Real migrations are serialized by `.kkachi/project_write.lock`, refuse status/event incoherence, refuse unknown source versions and unregistered paths, copy versioned helper state into `.kkachi/backups/schema-migrations/<timestamp>-<from>-to-<to>/`, and append `schema.migrated` after backup creation.


### `pilot-003` user docs and release packaging

`pilot-003` makes the MVP helper adoptable without network services or secret-bearing examples. The release surface consists of:

- README quickstart with local build, PATH setup, project init, run creation, artifact initialization, and diagnostics export examples;
- command reference linked back to this specs document and the roadmap;
- helper/bridge/skills compatibility matrix in `docs/compatibility.md`;
- release notes format in `docs/release-notes-template.md`;
- `go install github.com/SeventeenthEarth/kkachi-agent-helper@<version>` for global installation from a tagged release;
- `make PREFIX=<path> install-local` for local binary installation;
- `make VERSION=<semver> release` for release artifacts under `dist/`;
- a raw binary, `.tar.gz` package, versioned `RELEASE-MANIFEST.json`, bundled docs, and `SHA256SUMS`;
- e2e packaging coverage that verifies `make release`, artifact presence, archive contents, multi-platform checksum preservation, versioned release manifest content, embedded version metadata, root-package buildability, and `make install-local`.

Release packaging remains local and deterministic. It must not fetch dependencies beyond normal Go module resolution, call Kkachi services, include repository `.kkachi/` runtime state, or embed secrets. The canonical module path is case-sensitive and matches the GitHub URL: `github.com/SeventeenthEarth/kkachi-agent-helper`. Examples must use placeholder local paths and synthetic run data only.

### `pilot-004` MVP pilot acceptance run

`pilot-004` proves the MVP helper can execute one complete Kkachi-style local pilot run and preserve the evidence expected by the roadmap. It does not add a new CLI command or make the helper choose a backend. The proof is a black-box acceptance run over the existing public commands.

The acceptance run must create a temporary git repository, initialize helper state, create and activate an `adapter_qa` Path A standard run for `task_id=pilot-004`, initialize artifacts, record all required evidence, pass the required gates including `backend` and `final`, export a diagnostics bundle for the run, and close the run.

Required preserved evidence for the pilot acceptance run:

- project status showing active-run state during the run and no active run after close;
- append-only events for run lifecycle and passing gates;
- completed run artifacts for intake, SOT, roadmap, plan, implementation, review, verification, docs-update decision, and final report;
- bridge-shaped adapter evidence: `selected-cli.json`, `capability-check.md`, `bridge-session-snapshot.json`, and `bridge-events.md`;
- run-local gate reports, especially `gate-reports/final.json` with `status: pass`;
- diagnostics bundle containing project status/events, selected artifacts, bridge evidence, verification/docs/final artifacts, and final gate report.

The pilot evidence remains local, deterministic, and secret-free. Bridge evidence is validated only at the helper-owned artifact-shape and identity-consistency boundary; live external bridge execution and backend choice remain outside `kkachi-agent-helper` scope.

### `pilot-005` Go-native E2E harness cleanup

`pilot-005` keeps the existing black-box CLI and golden workspace coverage but moves the E2E harness to Go-native tests. `make test-e2e` runs the Go E2E package through `scripts/test-e2e.sh`; the harness must not require `python3` or legacy shell scenario files.

The Go E2E package preserves coverage for project lifecycle, lock recovery, golden workspace failures, diagnostics redaction, release packaging, and the MVP pilot acceptance run. It also includes harness-contract checks that prevent reintroducing Python-assisted E2E helpers or references to removed shell scenarios.

### Project bootstrap via `project init`

KAH no longer exposes an `install` command. Hermes skill installation is handled by Hermes native skill tooling, while KAH initializes the project-local files that KHS/Hermes skills use as their deterministic working contract. KAH bootstrap must not install, update, or vendor Hermes skill content, templates, registries, or evaluation assets.

`project init` is a one-shot bootstrap command that requires explicit project parameters. It creates existing helper state plus:

- `.kkachi/project-overlay.yaml`
- `docs/kkachi-docs-map.yaml`

`project init --force` is a reconfiguration command, not a destructive reset. It rewrites `.kkachi/config.yaml`, `.kkachi/project-overlay.yaml`, `docs/kkachi-docs-map.yaml`, and schema copies from the supplied parameters, preserves `status.json`, `events.jsonl`, `.kkachi/runs/**`, run metadata, artifacts, and gate history, and appends `project.reconfigured`.

Command UX rules:

- `--json` emits machine-readable output and no decorative text.
- Help output exits `0`, writes to stdout, and does not require repository or helper state discovery.
- Non-zero exit means the requested action did not succeed.
- Validation failures include path, field, expected value, actual value, and remediation hint.
- Canonical exit codes are `0` for success, including read-only diagnostic reports with warnings only; `1` for internal helper failures; `2` for usage errors, unsupported commands, or unsupported command options; `3` for fail-closed state/safety problems such as malformed helper state, unsafe paths, schema failures, or status/event coherence mismatches; and `4` for repository root discovery failure.
- Mutating commands append an event unless the command fails before mutation.
- Read-only diagnostic commands (`project status` and `project doctor`) must not append events, repair files, create locks, remove locks, or otherwise mutate `.kkachi/` state.
- `event append` is itself the primitive append-only event mutation; it fails if status and event-log tail ids are incoherent.
- `event append` keeps payloads compact: CLI payload input is limited to 256 KiB and serialized JSONL event lines are limited to 1 MiB. Larger evidence belongs in run artifacts.
- Event run ids may be omitted/null; when present, they must be printable, newline-free strings. Full run id syntax is defined by later run workflow tasks.
- State-file creation and replacement use atomic temp-file writes with durable sync before publish where the host filesystem supports it.
- Commands must reject absolute paths, paths escaping the repository root, and ambiguous run ids.

### `phase-plan`

`align-005` introduces `.kkachi/runs/<run_id>/phase-plan.yaml` as the KAH-managed storage surface for KHS-declared run-local phase state. KHS owns workflow policy, phase applicability, phase ordering, and the decision to mark a phase skipped or not applicable. KAH stores and validates declared rows only; it must not infer phases from `work_path`, `work_mode`, `execution_mode`, task semantics, backend choice, or user intent, and it must not intelligently reorder phases. This file is the run-local KHS workflow/execution state for one run; it is not the planned project-level `.kkachi-workflow.yaml` graph and is not deprecated by that graph.

The supported commands are:

```sh
kkachi-agent-helper phase-plan init <run_id> [--json]
kkachi-agent-helper phase-plan show <run_id> [--json]
kkachi-agent-helper phase-plan set <run_id> <phase-id> --status <pending|in_progress|complete|skipped|not_applicable|blocked> [--evidence <path>] [--reason <text>] [--approval-required true|false] [--json]
kkachi-agent-helper phase-plan validate <run_id> [--final] [--json]
```

`phase-plan init` creates the constrained YAML file with `version`, `run_id`, and declared phase rows including `ask`, `optimize`, `request-feedback-1`, and `handle-feedback-1`. Mutating phase-plan commands are serialized by `.kkachi/project_write.lock`, refuse status/event incoherence, write atomically, and append `phase_plan.initialized` or `phase_plan.updated`.

`phase-plan validate` checks deterministic structure and completeness only: required rows are present, phase statuses are from the supported enum, skipped/not-applicable rows include non-empty reasons, feedback rounds follow the effective `.kkachi-workflow.yaml` `feedback_intake` policy, and `request-feedback-N` / `handle-feedback-N` rows are paired. A valid graph feedback policy requires round 1, permits optional declared continuation rounds 2 through 5, and rejects round 6+. Missing or invalid graph state, absent `feedback_intake`, or forbidden fallback sources fail closed for declared feedback phase rows; KAH does not fall back to legacy `1..3` bounds. With `--final`, required rows and declared feedback rows must be terminal (`complete`, `skipped`, or `not_applicable`), completed rows must include evidence links, and rows marked `approval_required: true` must have a latest `approval.recorded` decision of `approved`. Passing validation exits `0`; failing validation exits `3`.


### `graph` surface

Status: `kkachi-agent-helper graph init`, `graph validate`, `graph explain`, `graph diff`, `graph propose`, `graph apply`, and `graph export` are implemented. `graph init` writes the initial `.kkachi-workflow.yaml` only when no graph exists. `graph propose` records proposal evidence only and does not apply graph changes. `graph apply` is the approval-gated replacement path for approved proposal records. `graph export` renders generated visualization artifacts only. Replacement init and `kah graph` alias behavior remain planned/candidate.

Implemented commands:

```text
kkachi-agent-helper graph init --from-template <khs-default|repo-relative-template.yaml> [--output .kkachi-workflow.yaml] [--json]
kkachi-agent-helper graph validate [--file .kkachi-workflow.yaml] [--json]
kkachi-agent-helper graph explain [--file .kkachi-workflow.yaml] [--json]
kkachi-agent-helper graph diff --from <repo-relative-graph> --to <repo-relative-graph> [--semantic] [--json]
kkachi-agent-helper graph propose --candidate-file <repo-relative-candidate-graph> --reason <text> [--json]
kkachi-agent-helper graph propose --patch <repo-relative-candidate-graph> --reason <text> [--json]  # legacy alias
kkachi-agent-helper graph apply --proposal <proposal-id> --approval <evidence-ref> [--json]
kkachi-agent-helper graph export --format mermaid|plantuml [--output <path>] [--json]
```

`graph init` requires initialized helper state, project event coherence, and `--from-template <template-id-or-path>`. The built-in template id `khs-default` is a deterministic seed from the current KHS phase-plan default spine, with linear edges and no generated gates, approvals, review policy, or policy decisions. Values containing `/` or ending in `.yaml` or `.yml` are treated as explicit repository-relative YAML template paths and must pass the same fail-closed graph source and schema validation as other graph inputs. Other bare values fail as `graph_template_unknown`; `--profile` is not supported. `--output` may be omitted or exactly `.kkachi-workflow.yaml`; any other output path is rejected so graph init cannot create multiple graph authorities. Existing `.kkachi-workflow.yaml` files, invalid files, symlinks, and directories fail closed as `graph_already_exists`. Successful init stamps the current project id/name, `managed_by: "kah"`, `source_template`, and `last_applied_event_id`, writes canonical graph YAML atomically, appends `graph.initialized`, and returns the rendered checksum. Approved replacement uses `graph apply`, not `graph init`.

`graph validate` checks the source path, constrained YAML structure, `version: "workflow-graph/v1"`, non-empty `graph_id`, required metadata, unique phase ids, explicit phase `required` fields, edge and gate references, acyclic edges, approval fields, duplicate sections/fields, duplicate gate ids, duplicate approval scopes, `proposals.policy: "proposal-first"` when present, and read-only `feedback_intake` bounds when declared. It rejects `.kkachi/config.yaml`, Kkachi v2 `.kkachi/config/workflows/**`, generated Mermaid/PlantUML files, unsafe paths, missing graph files, and invalid schema content fail-closed. Passing validation exits `0`; failing validation exits `3` and emits the graph result on stdout.

The accepted graph YAML subset is intentionally narrower than full YAML: top-level fields use `key: value`, sections use a single `section:` header, list rows use inline `- key: value` items followed by indented fields, indentation is spaces-only, string lists are inline (`["item"]`), integer lists are inline (`[1,2]`), and scalars are plain or double-quoted. Anchors, aliases, block scalars, nested maps beyond the documented shape, tab indentation, bare `-` list rows, and unquoted inline comments (`value # comment`) are rejected. Quote scalar values when a literal `#` is part of the value.

`graph explain` reuses validation and emits graph version, effective source, phases, edges, gates, approval requirements, valid `feedback_intake` bounds when declared, pending proposals, validation summary, and next action. It is read-only and does not append events, acquire locks, write graph files, create proposals, apply changes, or export diagrams.

`graph diff` validates both `--from` and `--to` graph files with the same fail-closed source/schema checks, then emits deterministic semantic changes by stable keys: phases by `id`, edges by `from -> to`, gates by `id`, approvals by `scope`, and the single `feedback_intake` declaration. It reports added, removed, and modified records plus `changed_feedback_intake`, `risk_flags`, `requires_approval`, `validation_summary`, and `next_action`. `--semantic` is accepted for command clarity; semantic comparison is the only implemented diff mode. Diff does not write graph files, proposal records, events, locks, or exports.

`graph propose` requires initialized helper state, project event coherence, one candidate graph input option, and `--reason <text>`. Prefer `--candidate-file <repo-relative-candidate-graph>` for new operator use. Legacy `--patch <repo-relative-candidate-graph>` remains accepted as a compatibility alias for the same complete candidate workflow graph input; it is not a partial patch DSL. KAH validates the current `.kkachi-workflow.yaml` and the candidate, computes the semantic diff or deterministic replacement comparison, writes `.kkachi/graph/proposals/gprop-000001.json` style proposal records atomically under the project write lock, and appends `graph.proposal_recorded`. If the base graph is valid, KAH records a normal semantic diff. If the base graph is missing, stale, broken, or otherwise invalid while the candidate validates under current KAH schema/source-precedence rules, KAH records a complete-candidate repair proposal with `graph_replacement`, base validation status/reason-code evidence, and approval required. A stale-only feedback-bound migration may still record the narrower `feedback_intake_changed` diff when the candidate graph is valid, declares canonical `1..5` feedback bounds, and changes no other graph semantics. Proposal records include proposal id/path, timestamp, reason, base/candidate file, checksum when available, validation summary, embedded semantic diff or replacement comparison, approval requirement, and next action. `approval_required=false` means the semantic diff did not trigger graph approval policy, not that apply can proceed without an audit trail. Successful CLI output also includes the appended event id. Proposal records never apply changes to `.kkachi-workflow.yaml`.

`graph apply` requires initialized helper state, project event coherence, `--proposal <proposal-id>`, and `--approval <evidence-ref>`. The approval value is recorded as an approval or audit evidence reference; KAH does not decide approval policy or validate external approver semantics. Apply loads `.kkachi/graph/proposals/<proposal-id>.json`, verifies schema/status/id/path consistency, validates the current `.kkachi-workflow.yaml` and the candidate graph, requires the current base evidence and candidate checksum to match the proposal record, stamps the candidate `metadata.last_applied_event_id` with the pending apply event id, renders canonical graph YAML, validates the rendered graph, writes `.kkachi-workflow.yaml` atomically, and appends `graph.applied`. Complete-candidate repair proposals are allowed when the proposal record and current base both prove the same missing/stale/broken invalid base state; existing graph bytes are copied to `.kkachi/backups/graph-repairs/<proposal-id>/<event-id>-.kkachi-workflow.yaml` before replacement and returned as `backup_path` plus `recovery_ref`. Missing-base repairs have no backup path. The stale feedback-bound migration exception is allowed only when the proposal record and current base both prove stale-only `feedback_intake_stale_bounds`, the semantic diff reports only `feedback_intake_changed`, and the candidate validates with canonical bounds. JSON output includes `schema_version`, `status`, `proposal_id`, `approval_ref`, `graph_path`, optional `backup_path`, optional `recovery_ref`, `new_checksum`, `event_ids`, and `next_action`. Apply fails closed without writing graph state or appending events when proposal evidence is missing/invalid, graph sources are invalid outside the explicit migration or complete-candidate repair paths, base evidence conflicts, candidate checksums conflict, or the explicit evidence reference is absent.

`graph export` validates `.kkachi-workflow.yaml` with the same fail-closed graph validation path, then renders either Mermaid (`flowchart TD`) or PlantUML (`@startuml`/`@enduml`) as generated visualization only. `--format mermaid|plantuml` is required. When `--output` is omitted, human output is the diagram body on stdout; JSON output includes metadata plus the `diagram` string. When `--output` is provided, KAH writes a new repository-relative generated diagram file with a matching extension (`.mmd`/`.mermaid` for Mermaid, `.puml`/`.plantuml` for PlantUML) and rejects unsafe paths, graph source paths, existing files, directories, and mismatched extensions. JSON output includes `schema_version`, `status`, `format`, `output_path`, `source_file`, `source_checksum`, `authoritative: false`, `diagram`, `validation_summary`, and `next_action`. Export does not write graph state, create proposals, append events, decide policy, or make diagrams graph authority.

Graph proposal lifecycle is proposal-first. KHS, a responsible approver, or a human drafts a complete candidate graph or selects a KHS template; KAH validates the candidate, explains the current effective graph, computes a semantic diff, records `.kkachi/graph/proposals/gprop-*.json` evidence, accepts explicit approval/audit evidence through `graph apply`, atomically applies the approved candidate, appends audit events, and lets KHS copy relevant evidence into run artifacts when the graph change affects a run. Direct YAML editing is not the normal path; unmanaged direct edits must be repaired through validation/proposal/apply evidence before KHS relies on them.

Stable graph JSON output keeps these compact top-level fields:

| Command | Required fields |
|---|---|
| `validate --json` | `schema_version`, `status`, `file`, `checksum`, `effective_source`, optional `feedback_intake`, `errors`, `warnings`, `conflicts`, `next_action` |
| `explain --json` | `schema_version`, `status`, `graph_version`, `effective_source`, `phases`, `edges`, `gates`, `approval_requirements`, optional `feedback_intake`, `pending_proposals`, `validation_summary`, `next_action` |
| `diff --json` | `schema_version`, `status`, `from`, `to`, `changed_phases`, `changed_edges`, `changed_gates`, `changed_approvals`, `changed_feedback_intake`, `risk_flags`, `requires_approval`, `validation_summary`, `next_action` |
| `propose --json` | `schema_version`, `status`, `proposal_id`, `proposal_path`, `validation_summary`, `semantic_diff_ref`, `approval_required`, `event_id`, `next_action` |
| `apply --json` | `schema_version`, `status`, `proposal_id`, `approval_ref`, `graph_path`, optional `backup_path`, optional `recovery_ref`, `new_checksum`, `event_ids`, `next_action` |
| `export --json` | `schema_version`, `status`, `format`, `output_path`, `source_file`, `source_checksum`, `authoritative: false`, `diagram`, `validation_summary`, `next_action` |

KAH policy-mutation command category is empty. Do not document policy-setting surfaces as normal commands; this excludes workflow subcommands under the `kah` prefix, profile-driven graph initialization, gate-setting commands, review-policy setters, and graph policy setters.

Forbidden examples:

```text
kah workflow ...
kah graph init --profile ...
kah gate set ...
kah review-policy set ...
kah graph set-policy ...
```

### `approval`

`align-007` introduces strict approval request/decision records for KHS-declared high-risk phases. KHS decides when approval is required; KAH only validates and stores declarations as `approval.requested` and `approval.recorded` events.

```sh
kkachi-agent-helper approval request <run_id> --phase <phase-id> --reason <reason> [--evidence <ref>] [--json]
kkachi-agent-helper approval record <run_id> --phase <phase-id> --decision <approved|rejected> --by <approver> --evidence <ref> [--reason <reason>] [--json]
kkachi-agent-helper approval show <run_id> [--phase <phase-id>] [--json]
```

Approval events include `phase`, helper-generated RFC3339 `timestamp`, and either request `reason` or decision fields (`decision`, `approver`, and `evidence`). The `approval record` command accepts `approved` or `rejected`; only the latest `approved` decision satisfies `phase-plan validate --final` for phases marked `--approval-required true`.

### `diagnostics export`

`pilot-002` introduces `diagnostics export [--run <run_id-or-prefix>] [--output <repo-relative-path>]` as a support-safe diagnostic bundle. The command is deterministic and does not append events, take locks, recover state, or repair `.kkachi/`. Without `--output`, it writes the JSON bundle to stdout. With `--output`, it writes one new repository-confined JSON file and prints a compact summary unless `--json` is used.

The bundle includes redacted project config, status, event entries, project-local schema versions, run-local gate reports, graph compatibility evidence, and a selected support artifact set for the requested run. `graph_compatibility.feedback_intake` reports `status: "pass"` with effective bounds when canonical feedback intake is valid, `status: "missing"` when graph state or the policy declaration is absent, and `status: "fail"` with validation issues for stale or invalid declarations. If `--run` is omitted, the active run is used when one is recorded in `status.json`; otherwise the bundle contains project-level diagnostics only. Selected artifacts are intentionally narrower than the full artifact tree: `run-metadata.json`, `phase-plan.yaml`, intake classification, backend evidence files, test/verification/docs-update evidence, and `final-report.md`. Missing selected artifacts are reported in the bundle with `status: "absent"` rather than causing diagnostics export to fail. Run-scoped approval records are included as `approval_records`.

Token-like values are redacted in both diagnostics bundles and CLI errors. Redaction preserves field names and replaces sensitive values with `[REDACTED]`; key names such as `token`, `secret`, `password`, `api_key`, `authorization`, and credential-like variants are redacted recursively in JSON payloads, and bearer/assignment/long-token patterns are redacted from text.

### `project status`

`project status` is a read-only project summary intended for humans and scripts. JSON output has the following stable shape:

```json
{
  "root_path": "...",
  "health": "ok|warning|fail",
  "project_id": "...",
  "project_name": "...",
  "active_run_id": null,
  "active_run_state": null,
  "last_event_id": "evt-000001",
  "event_tail_id": "evt-000001",
  "event_count": 1,
  "updated_at": "...",
  "gate_summary": {},
  "issues": []
}
```

`health` is `ok` when all checks pass, `warning` when only non-fatal issues such as present lock files are found, and `fail` when helper state is unsafe or malformed. Status/event tail mismatch is a fail-closed state problem and returns exit code `3`.

### `project doctor`

`project doctor` is a read-only diagnostic report. It checks:

- `.kkachi/config.yaml` exists, is readable, and declares the generated fields required by the embedded config schema: `version`, `project.name`, root policy, canonical paths, lock policy, schema mode, and compatibility declarations;
- `.kkachi/status.json` is a JSON object with required typed fields, valid `last_event_id`, RFC3339 `updated_at`, and object `gate_summary`;
- `.kkachi/events.jsonl` is readable, non-empty JSONL with no blank lines, valid event ids, and sequential `evt-000001`-style ids;
- status/event coherence, requiring `status.last_event_id` to match the event-log tail id;
- canonical `.kkachi/*` state, schema, and lock paths stay within the repository and do not symlink-escape;
- the seven canonical schema files exist, are readable JSON objects, and declare their own `version`;
- lock files are absent, present, unreadable, or path-unsafe.

JSON output has the following stable shape:

```json
{
  "root_path": "...",
  "health": "ok|warning|fail",
  "summary": {"passed": 0, "warnings": 0, "failed": 0},
  "checks": [
    {
      "name": "status",
      "status": "pass|warn|fail",
      "path": ".kkachi/status.json",
      "message": "...",
      "hint": "...",
      "field": "",
      "expected": "",
      "actual": ""
    }
  ]
}
```

Warnings return exit code `0`; failures return exit code `3`. `corex-005` introduced read-only lock diagnostics; `runwf-002` adds stale-lock interpretation and explicit recovery while keeping `project doctor` read-only.

### `project probe-toolchain`

`project probe-toolchain --json [--project-root <path>]` emits read-only KAH helper/project facts for KAS TOLMR toolchain generation. When `--project-root` is supplied, it must name an existing directory and is canonicalized through absolute/symlink resolution. Without `--project-root`, the helper uses normal repository root discovery when possible and otherwise probes the current working directory so uninitialized projects can still return stable JSON.

The command never creates, updates, or repairs `.kkachi/`, events, graphs, schemas, locks, runs, project files, or `.kkachi/toolchain.yaml`. A successful probe exits `0` even when the project is uninitialized; missing helper state is reported through `doctor.status`, `doctor.reason_codes`, and project fact booleans. `project_initialized` reports presence of the core KAH project state, while `doctor.status` separately reports PASS/WARN/FAIL/UNKNOWN health.

JSON output has the following stable shape:

```json
{
  "ok": true,
  "schema_version": "kah.toolchain_probe.v1",
  "no_write": {"guaranteed": true, "write_count": 0},
  "kah": {"helper_command": "kkachi-agent-helper", "version": "0.1.x", "binary_path": "/absolute/path/or/unknown"},
  "project": {
    "root": "/absolute/project/root",
    "kkachi_dir": "/absolute/project/root/.kkachi",
    "kkachi_dir_present": true,
    "project_initialized": true,
    "workflow_graph_present": false
  },
  "doctor": {"status": "PASS|WARN|FAIL|UNKNOWN", "reason_codes": []},
  "diagnostics": []
}
```

Reason codes are deterministic strings such as `kkachi_dir_missing`, `status_fail`, or `coherence_fail`. KAH reports facts only; KAS owns `.kkachi/toolchain.yaml` schema, stage selection, MAR/provider policy, legacy import, and interpretation.

## 11. Locking

Default lock files:

| Lock | Purpose |
|---|---|
| `.kkachi/active_run.lock` | Prevent conflicting active-run transitions. |
| `.kkachi/project_write.lock` | Enforce one active write lane per project. |

Lock requirements:

- Use atomic create where possible.
- Record owner process id, hostname, command, run id, and timestamp.
- `project_write.lock` serializes helper-state mutations such as event appends and run creation.
- `active_run.lock` serializes run lifecycle transitions; lifecycle commands also acquire `project_write.lock` so status/events/metadata updates remain one-active-write by default.
- `project doctor` reports absent locks as pass, fresh or stale readable locks as warning, and malformed, unreadable, non-regular, or path-unsafe lock paths as failure.
- A lock is stale when it is older than 30 minutes, or when it belongs to the current host and its owner pid is no longer alive.
- A mutating command encountering a fresh lock fails closed with `lock_conflict`.
- A mutating command encountering a stale lock fails closed with `lock_stale_recovery_required`; it must not silently remove or reuse the lock.
- `lock recover <active-run|project-write|all> --reason <text> [--run <run_id>]` is the explicit recovery command. It refuses fresh locks, malformed lock metadata, absent targeted locks, and recovery without a reason.
- Recovery appends `lock.recovered` before lock removal and advances `status.last_event_id`.
- Provide explicit recovery commands rather than silent lock removal.
- Do not allow forced unlock without an event record.

## 12. Schema and migration policy

- Schemas live embedded in the binary and may also be copied under `.kkachi/schemas/` for transparency.
- `project init` writes project-local JSON Schema draft 2020-12 copies from the embedded canonical registry for config, status, event, run metadata, selected CLI, and bridge session snapshot. These copies are transparency artifacts; validation uses the embedded registry so a relaxed local schema cannot make invalid helper state pass.
- `schema validate <file> --schema <schema>` accepts embedded schema names, canonical schema filenames, or repository-confined `.kkachi/schemas/*.schema.json` references. It validates config YAML through the deterministic helper config parser, validates event JSONL line-by-line for `events.jsonl`, and validates JSON state/evidence objects for the other schemas. Passing validation exits `0`; schema failures exit `3`; usage errors exit `2`.
- `schema export [--schema <schema>|--all] [--dry-run]` copies embedded schemas into `.kkachi/schemas/`. Dry runs are read-only previews. Real exports are serialized by `project_write.lock`, refuse status/event incoherence before mutation, write only canonical schema paths, append one `schema.exported` event when files change, and leave unchanged files untouched.
- `schema migrate --from <version> --to <version> [--dry-run]` runs registered state migrations. The initial registered path is `0.1 -> 0.1` no-op. Dry runs are read-only summaries. Real migrations are serialized by `project_write.lock`, refuse unknown source versions and incoherent status/event state, write a backup under `.kkachi/backups/schema-migrations/`, and append `schema.migrated`.
- Every schema has a version.
- Backward-compatible additions are allowed within a minor version when fields are optional.
- Required field changes need a migration command and tests.
- Unknown fields should be preserved where practical but never used to pass a gate.
- Migration must write a backup or reversible event record before overwriting state.

## 13. Artifact gate examples

Initial gate names:

| Gate | Minimum checks |
|---|---|
| `intake` | `run-metadata.json`, `intake-classification.md`, path/mode eligibility. |
| `sot` | `sot-basis.md` or Path B SOT update evidence. |
| `roadmap` | `task_id` trace, `roadmap-update.md`, or explicit not-applicable reason. |
| `plan` | Completed `acceptance-criteria.md`, `plan.md`, and KHS-normalized `checklist.md`; KAH does not parse KAB planner seed sections. |
| `backend` | bridge evidence artifacts when `backend_evidence=required` or bridge artifacts are already required. |
| `implementation` | `diff.patch`, `impl-log.md`, optional `cli-output.md`. |
| `review` | `review.md` and required red-team artifacts. |
| `verification` | `test-log.md`, `verification.md`, pass/fail verdict. |
| `docs` | `docs-update.md` and changed docs list or no-change reason. |
| `final` | all required gates pass, no open blockers, `final-report.md` exists. |

## 14. Project initialization and bootstrap

`project init` creates `.kkachi/`, config, schemas, status, event log, project overlay, and docs map. It does not install Hermes skills; use Hermes native skill installation for KHS skill content.

Required bootstrap parameters:

- `--project-name`, `--stack`, `--repo-path`
- `--commander`, `--redteam`
- `--docs-map-roadmap`, `--docs-map-spec`, `--docs-map-architecture`
- `--docs-map-adr-dir`, `--docs-map-todo-dir`, `--docs-map-spec-dir`
- `--test-commands`, `--backend-policy`, `--execution-mode`, `--sot-policy`

Initial `project init` behavior:

- `status.project_id` uses `kkachi-project-<project-slug>-<random-hex>`.
- `status.last_event_id` is `evt-000001`.
- `.kkachi/events.jsonl` contains exactly one `project.initialized` record with bootstrap summary payload.
- `.kkachi/schemas/` contains local schema copies for config, status, event, run metadata, selected CLI, and bridge session snapshot.
- `.kkachi/project-overlay.yaml` records project, stack, repo path, commander/redteam, test commands, backend policy, execution mode, and SOT policy.
- `docs/kkachi-docs-map.yaml` records roadmap, SOT docs, ADR/todo/spec directories, and test commands.

`project init --force` behavior:

- preserves `status.json`, `events.jsonl`, `.kkachi/runs/**`, run metadata, artifacts, and gate history;
- rewrites config, overlay, docs map, and schema copies from the supplied parameters;
- appends `project.reconfigured`;
- is not a full reset/delete command.

## 15. Testing standard

Minimum implementation test layers:

| Layer | Required coverage |
|---|---|
| Unit | schema validation, path safety, id generation, project status/doctor diagnostics, lock behavior, gate rules. |
| Integration | project init, project status/doctor, run create/activate/close, event append, schema validation/export, and later schema migration. |
| Local E2E | User-visible CLI flows such as project init success, generated state files, status/doctor JSON output, schema validate/export, event coherence failure, and unsafe overwrite refusal. |
| Golden fixtures | valid and invalid `.kkachi/` workspaces. |
| CLI tests | exit codes, JSON output, failure messages, dry-run behavior. |
| Compatibility tests | migration from previous schema versions and helper-oc lessons where applicable. |

## 16. Security and safety

- Treat all project files as untrusted input.
- Never execute scripts discovered from project config during validation.
- Do not follow symlinks that escape repository root.
- Do not store secrets in events, config, or artifacts.
- Redact token-like values in diagnostics.
- Use conservative file permissions for lock and state files.
- Prefer explicit operator confirmation for destructive reset or migration commands.

## 17. Compatibility contract

KHS/KAH integration follows a capability-first model:

- KHS `main` may install KAH with `go install github.com/SeventeenthEarth/kkachi-agent-helper@latest`, then run `kkachi-agent-helper capabilities --json` and fail closed if required surfaces or compatibility flags are absent.
- KHS release tags should record tested/recommended KAH versions for reproducible release use, even when `@latest` remains acceptable for compatible `main` activation.
- `project init` and `project init --force` are the KAH bootstrap/reconfiguration contract; KAH does not provide a Hermes skill installer.
- KHS decides whether to trigger, which backend path to use, what phases apply, and how to normalize workflow artifacts. KAH validates declared state and evidence after those decisions are made.
- `docs/compatibility.md` is the release-facing matrix for helper/schema/bridge/skills expectations and must stay consistent with this spec.

## 18. Open decisions

The following items remain open until roadmap tasks close them:

- run id format;
- exact lock stale detection policy;
- whether helper exports a library API in addition to the CLI;
- whether bridge capability registry validation is direct or delegated to `kkachi-hermes-skills` assets;
- authoritative bridge and skills version sources for enforced compatibility beyond `compat.required_helper`.

## Planning graph record appendix

Date: 2026-05-21
Owner: KAH deterministic helper boundary
Confirming role: Responsible approver / governance evidence record
Status: graph init, validation/explanation, semantic diff, proposal records, approval-gated apply, non-authoritative export, compatibility diagnostics, configurable feedback-intake activation evidence, and phase-plan feedback-bound validation implemented
Authority level: `docs/specs.md` remains authoritative for implemented helper behavior and generated visualization export boundaries
Scope: KAH helper docs only
Related docs: `README.md`, `roadmap.md`, `compatibility.md`, KHS `docs/sot/workflow-graph-integration.md`
Decision summary: add `.kkachi-workflow.yaml` as candidate project-level graph state while preserving `.kkachi/config.yaml` as helper config and `phase-plan.yaml` as run-local execution evidence; diagnostics now publish `graph_compatibility` so KHS can fail closed without direct YAML fallback; graph validation/projection recognizes declared feedback-intake bounds, phase-plan validation consumes those bounds, stale-only bounds migrate through proposal/apply evidence, and final activation depends on capabilities plus diagnostics.
Evidence/source paths: governance evidence record in kanban task `t_2fb00394`
Stale/conflict markers: older wording that treats `phase-plan.yaml` as the whole workflow SOT is narrowed to run-local state; prior root-level kkachi config YAML/JSON graph phrasing is superseded by `.kkachi-workflow.yaml` if encountered.
Open questions: command alias remains an implementation task.
Next record action: keep graph compatibility diagnostics aligned with capabilities and release evidence without widening export into generated-artifact authority or alias behavior.
