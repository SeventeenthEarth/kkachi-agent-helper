# kkachi-agent-helper Roadmap

This roadmap tracks delivery of `kkachi-agent-helper`. Each epic uses a five-letter English slug, and task ids follow `{slug}-001`, `{slug}-002`, and so on.

Status values: `Planned`, `In Progress`, `Blocked`, `Completed`, `Deferred`.

## Task sizing policy

- A task is a **PR candidate**, not a checklist item.
- Split a task only when it cannot be reviewed, tested, or rolled back as one coherent change.
- Keep epic count low; each epic should deliver a user-visible capability layer.
- Do not force equal task counts across epics.
- If a task grows beyond one focused PR, split it during execution and preserve the original id as the parent context.

## Delivery order

| Order | Epic | Delivery outcome |
|---:|---|---|
| 1 | `corex` | Usable helper binary that can initialize and inspect `.kkachi/` state. |
| 2 | `runwf` | Safe run lifecycle with locks and run artifact initialization. |
| 3 | `gates` | Deterministic workflow gates for Kkachi Path A / Path B readiness. |
| 4 | `packg` | Versioned schemas, migration surface, and historical package/bootstrap contract. |
| 5 | `pilot` | End-to-end evidence, diagnostics, docs, release, and MVP pilot proof. |
| 6 | `align` | KHS can consume KAH `@latest` through stable capability checks, declared backend evidence, phase-plan validation, and compatibility diagnostics. |
| 7 | `feedb` | Feedback-driven KAH hardening is preserved in planning history after KHS and PM consumer feedback exposes deterministic-helper risk. |
| 8 | `token` | KAH supplies deterministic evidence gates for KAS token-economy, English-output, agent-instruction, and project KAS lifecycle work without becoming policy or prose-judgment authority. |
| 9 | `grsync` | KAH hardens workflow graph diagnostics and approval-gated repair substrate so KAS can safely manage graph workflow sync compatibility for KAH v0.1.9 / KAS v0.1.2. |
| 10 | `DAGSM` | KAH supplies deterministic task-DAG validation, workflow instance state, node FSM/order enforcement, ready-node calculation, multi-DAG catalog diagnostics, and final gates for KAS WFLOW-triggered workflows. |

## Active roadmap

Section order below is historical/topic grouping and may differ from the delivery sequence. Use `Delivery order` and each epic's explicit dependency notes as the authoritative implementation order.


### EPIC: corex — Core CLI and project state foundation

> Goal: ship the minimum reliable local helper: command shell, safety rules, project initialization, status, events, and doctor checks.

| Task ID | Title | Status | Work guide | Notes |
|---|---|---|---|---|
| corex-001 | Repository, toolchain, and CLI shell | Completed | Initialize the repo layout, build/test/lint commands, binary entrypoint, command groups, `--json`, version output, and baseline README links. Keep implementation minimal but tested. | First PR; no `.kkachi/` mutation beyond fixtures. |
| corex-002 | Path safety, errors, and output contract | Completed | Implement repo-root discovery, safe relative path handling, symlink escape rejection, canonical exit codes, and structured human/JSON errors with remediation hints. | Security prerequisite for every mutating command. |
| corex-003 | Project init with config, status, and events | Completed | Implement `project init` to create `.kkachi/config.yaml`, `.kkachi/status.json`, `.kkachi/events.jsonl`, and initial schema copies or schema references. Refuse unsafe overwrites. | First real adoption path. |
| corex-004 | Atomic state writes and event coherence | Completed | Add atomic writes for state files, append-only event handling, `last_event_id` coherence checks, and crash-safety tests. | Prevents corrupted helper state. |
| corex-005 | Project doctor and status commands | Completed | Implement `project doctor` and `project status [--json]` covering config, status, events, paths, schema availability, and basic lock diagnostics. | Read-only operational baseline. |

### EPIC: runwf — Run workflow, locks, and artifacts

> Goal: let a project create one active Kkachi run, protect it from concurrent writes, and initialize the canonical run artifact home.

| Task ID | Title | Status | Work guide | Notes |
|---|---|---|---|---|
| runwf-001 | Run metadata and lifecycle commands | Completed | Define run id policy and implement `run create`, `run activate`, `run close`, `run abort`, and run listing/lookup. Store `run-metadata.json` with work path, mode, urgency, SOT policy, execution mode, commander, task id, and state. | Implemented with helper-generated run ids, metadata storage, unique-prefix lookup, status/event coherence, and status-based active-run conflict checks. Lock enforcement is implemented in `runwf-002`; artifact initialization remains in `runwf-003`. |
| runwf-002 | One-active-write locking and recovery | Completed | Implement `.kkachi/active_run.lock` and `.kkachi/project_write.lock` with atomic acquire/release, owner metadata, stale diagnostics, and explicit recorded unlock recovery. | Implemented transient lock enforcement for project writes and run lifecycle transitions, stale diagnostics through `project doctor`, and `lock recover <active-run\|project-write\|all> --reason <text> [--run <run_id>]` with `lock.recovered` events. |
| runwf-003 | Artifact manifest and initialization | Completed | Define artifact manifest by work path/mode/execution mode and implement `artifact init` plus `artifact list`. Create baseline run files without overwriting non-empty artifacts. | Implemented with canonical artifact baselines from `docs/specs.md`, metadata `required_artifacts` updates, `artifact.written` events, non-empty preservation, empty-file reinitialization, and read-only artifact listing. |
| runwf-004 | Work path and light-mode validation | Completed | Validate Path A / Path B classification, Standard vs Light eligibility, urgency metadata, and required not-applicable reason format before later gates run. | Implemented as read-only `artifact validate <run_id> [--gate intake]` with intake classification checks, manifest coherence, Path A/B SOT-policy eligibility, Light-mode reason enforcement, and no state mutation. |

### EPIC: gates — Deterministic readiness gates

> Goal: make Kkachi readiness machine-checkable without letting the helper become the reasoning layer.

| Task ID | Title | Status | Work guide | Notes |
|---|---|---|---|---|
| gates-001 | Gate engine and `gate check` command | Completed | Create a small declarative gate model and implement `gate check <run_id> <gate>` with pass/fail/blocked results, exact missing evidence, and stable JSON output. | Implemented mutating gate checks with `gate_state`, `gate_summary`, `gate.passed` / `gate.failed` / `gate.checked` events, intake validation reuse, and blocked placeholders for later gates. |
| gates-002 | SOT, roadmap, and plan gates | Completed | Implement checks for SOT basis or Path B SOT creation, roadmap trace or explicit exception, acceptance criteria, `plan.md`, and `checklist.md`. | Implemented pre-implementation `sot`, `roadmap`, and `plan` gate evaluators with artifact status checks, task-id roadmap trace support, explicit roadmap not-applicable reasons, and pass/fail event recording. |
| gates-003 | Backend evidence gate | Completed | Validate `selected-cli.json`, `capability-check.md`, `bridge-session-snapshot.json`, and `bridge-events.md` shape and declared status when the run manifest requires backend artifacts. Never choose or override the backend. | Implemented as a manifest-driven gate: adapter QA backend evidence is checked fail-closed; non-backend manifests record a deterministic not-applicable pass. |
| gates-004 | Verification, docs, and final readiness gates | Completed | Validate implementation evidence, review/red-team artifacts, `test-log.md`, `verification.md`, `docs-update.md`, blocker state, and `final-report.md`. Add `gate final`. | Main PR-ready boundary. |
| gates-005 | Gate reports and regression fixtures | Completed | Generate run-local gate reports and add valid/invalid fixtures for Path A Standard/Light and Path B Standard/Light, including malformed evidence and missing artifact cases. | Implemented per-gate JSON reports under each run plus fixture-backed regression coverage for valid, malformed, and missing-evidence outcomes across Path A/B and Standard/Light modes. |

### EPIC: packg — Schemas, migrations, and install packaging

> Goal: make helper contracts transparent, versioned, migratable, and safely bootstrap project-local Kkachi state.

| Task ID | Title | Status | Work guide | Notes |
|---|---|---|---|---|
| packg-001 | Embedded schema registry and validation CLI | Completed | Embed config, status, event, run metadata, selected CLI, and bridge snapshot schemas. Implement `schema validate` and schema export/copy into `.kkachi/schemas/`. | Implemented embedded registry-backed validation, `schema validate`, `schema export [--schema <name>|--all] [--dry-run]`, full schema copies during `project init`, and `schema.exported` event recording for real exports. |
| packg-002 | State migration framework | Completed | Add migration registration, dry-run summary, backup behavior, event recording, and refusal of unknown source versions. Include first no-op/sample migration fixture. | Implemented `schema migrate --from <version> --to <version> [--dry-run]` with the initial `0.1 -> 0.1` no-op migration, run-safe backup copies, `schema.migrated` events, and unknown-source/path refusal. |
| packg-003 | Historical install manifest and dry-run contract | Superseded | Defined the first manifest/checksum package contract. | Superseded by project-init bootstrap: KAH no longer exposes an `install` command; Hermes skill installation belongs to Hermes native tooling. |
| packg-004 | Local install/update compatibility gate | Superseded | Implemented local manifest apply/update safety. | Superseded by `project init` plus `project init --force`, which owns bootstrap/reconfiguration without installing skill content. |

### EPIC: pilot — E2E proof, diagnostics, docs, and release

> Goal: prove the helper can support a real Kkachi pilot run and ship a usable MVP release.

| Task ID | Title | Status | Work guide | Notes |
|---|---|---|---|---|
| pilot-001 | CLI e2e harness and golden workspaces | Completed | Build black-box CLI tests against temporary repositories and golden workspaces for successful and failing flows. Cover unsafe paths, bad JSON, lock conflicts, missing artifacts, schema mismatches, and ambiguous run ids. | Implemented black-box CLI golden workspace coverage plus schema-mismatch, status/event-mismatch, and invalid-events JSONL fixtures; coverage now runs through the Go-native `tests/e2e` harness wired into `make test-e2e`. |
| pilot-002 | Diagnostics bundle and redaction | Completed | Add redacted diagnostic export containing config, status, events, schema versions, gate reports, and selected artifacts. Redact token-like values in errors and bundles. | Implemented `diagnostics export [--run <run_id>] [--output <path>]`, schema-version inventory, run gate-report/artifact capture, and shared token-like redaction for CLI errors and bundles. |
| pilot-003 | User docs, compatibility, and release packaging | Completed | Write README quickstart, command reference, specs links, helper/bridge/skills version matrix, release notes format, build artifacts, checksums, and local install command. Keep examples local and secret-free. | Implemented README quickstart/command reference, compatibility matrix, release notes template, `install-local`, release artifact/checksum packaging, and e2e packaging coverage. |
| pilot-004 | MVP pilot acceptance run | Completed | Execute one real Kkachi pilot run and preserve evidence: status, events, artifacts, bridge evidence, verification, docs-update decision, gate report, diagnostics bundle, and final report. | Implemented a black-box E2E acceptance run that records adapter QA bridge evidence, passes all required gates, preserves run-local gate reports, exports a diagnostics bundle, and verifies status/events/final-report evidence. |
| pilot-005 | Go-native E2E harness cleanup | Completed | Replace Python-assisted shell E2E helpers with Go-native test helpers or Go E2E tests. Remove `python3` as an E2E harness dependency while preserving black-box CLI coverage and golden workspace scenarios. | Implemented Go-native black-box E2E tests for lifecycle, locks, golden workspaces, diagnostics, release packaging, and MVP acceptance; `make test-e2e` now runs `go test ./tests/e2e` with no `python3` harness dependency. |

### EPIC: align — KHS/KAH integration alignment

> Goal: let KHS use KAH `@latest` safely while preserving the boundary that KHS owns workflow policy and KAH owns deterministic state, artifact, gate, event, and diagnostics validation.
>
> Source of truth: `docs/specs.md`, `docs/compatibility.md`, and implemented capability evidence own current helper behavior. The former `docs/TODO-ALIGN.md` reference is stale because that file is deleted in the current working tree; do not treat it as active authority.

| Task ID | Title | Status | Work guide | Notes |
|---|---|---|---|---|
| align-001 | Plan/checklist ownership contract | Completed | Document and test that the `plan` gate requires completed `acceptance-criteria.md`, `plan.md`, and `checklist.md`; state that KHS owns checklist normalization and KAH does not parse KAB-specific planner sections such as `KHS Checklist Seed`. | Implemented as docs/spec/compatibility contract hardening plus unit regressions for missing, empty, pending, complete, and seed-section plan cases. |
| align-002 | Declared backend evidence requirement | Completed | Add an explicit run metadata/CLI contract for KHS to require backend evidence independently of `execution_mode`; when declared required, include `selected-cli.json`, `capability-check.md`, `bridge-session-snapshot.json`, and `bridge-events.md` in `required_artifacts` and make the backend gate fail closed until complete. | Implemented `run create --backend-evidence auto|required|not_applicable`, persisted resolved `backend_evidence`, added backend artifacts for declared requirements, and locked production-write backend gate regressions. |
| align-003 | Command-surface capabilities report | Completed | Add `capabilities --json` with helper version, schema version, supported command groups, deprecated/omitted surfaces, and compatibility flags needed by KHS activation checks. | Implemented project-independent capabilities JSON with helper/schema versions, command inventory, KHS compatibility flags, planned-surface reporting, and omitted `install` reporting; align-005 reports phase-plan as supported and align-007 reports approvals as supported. |
| align-004 | Standard help UX | Completed | Support stable `help` / `--help` output for top-level and command groups, including required arguments, options, and documented JSON behavior. | Implemented project-independent human and structured JSON help for top-level, implemented command groups, key subcommands, `help help`, and the phase-plan help surface; unit/integration/E2E regressions cover zero-exit help outside initialized state, release artifact help, and unchanged usage errors. |
| align-005 | Phase-plan validation and diagnostics | Completed | Add deterministic support for KHS-declared phase plans: initialize/show/validate/update phase state or an equivalent KAH-managed representation, require reasons for skipped/not-applicable phases, and include phase-plan evidence in diagnostics. | Implemented `phase-plan init/show/set/validate` over run-local `phase-plan.yaml`, deterministic reason/feedback/final checks, diagnostics inclusion, and capabilities/help support while preserving the KHS-owned phase applicability boundary. |
| align-006 | Deterministic artifact mutation commands | Completed | Add safe `artifact write`, `artifact append`, and `artifact set-status` commands for canonical run artifacts with path safety, atomic writes, status updates, and event recording while keeping direct-file compatibility during migration. | Implemented canonical-only artifact mutation commands with lock/coherence safeguards, atomic writes/appends/status updates, `artifact.written` audit events, help/capabilities/docs updates, and unit/CLI/integration/E2E coverage. |
| align-007 | Approval record surface | Completed | Add approval request/record/show commands or a strict approval event schema so KHS can record high-risk phase approvals with phase, reason, decision, approver, timestamp, and evidence reference. | Implemented `approval request/record/show`, strict `approval.*` events, diagnostics inclusion, capabilities/help/docs updates, and phase-plan final approval checks for rows marked approval-required. |
| align-008 | KHS/KAH compatibility contract docs | Completed | Update README/specs/compatibility docs to state the KHS/KAH boundary, `project init` bootstrap contract, no Hermes skill installation, `@latest` plus capabilities policy, and tested-version recommendation model. | Consolidated README/specs/compatibility docs around KHS/KAH ownership, `capabilities --json` activation checks, tested/recommended release versions, project-init bootstrap, and no Hermes skill installation by KAH; locked with E2E docs-contract regression coverage. |

### EPIC: feedb — Feedback-driven hardening and intake

> Goal: preserve feedback-originated KAH hardening work in the roadmap so small completed fixes remain traceable without promoting KAH beyond deterministic local helper ownership.

| Task ID | Title | Status | Work guide | Notes |
|---|---|---|---|---|
| feedb-001 | Guard schema-owned backend JSON from generic artifact status mutation | Completed | Record the KHS/PM feedback-driven fix that prevents `artifact set-status <run_id> selected-cli.json --status complete` from overwriting schema-owned backend evidence status values such as `supported` or `degraded`. | Completed before this intake rule was formalized; the guard preserves backend gate evidence checks by requiring schema-owned backend JSON artifacts to be written with valid JSON evidence instead of generic lifecycle status mutation. No separate TODO file is needed for this small historical item. |

### EPIC: graph — Command-managed workflow graph

> Goal: add a deterministic KAH graph surface for project-level `.kkachi-workflow.yaml` state while preserving KHS policy ownership and run-local `phase-plan.yaml` evidence.
>
> Status note: the graph contract is merged into permanent specs/compatibility docs, and KAH capabilities/help prove each implemented command surface. Future graph work should proceed one roadmap task at a time after required review gates.

| Task ID | Title | Status | Work guide | Notes |
|---|---|---|---|---|
| graph-001 | Docs/SOT and schema v1 outline for `.kkachi-workflow.yaml` | Completed | Confirm graph authority tables, source precedence, command classification, JSON/human output expectations, and schema outline. | Former temporary workflow graph SOT was merged into permanent specs/compatibility docs and removed after implementation evidence landed. |
| graph-002 | Read-only graph validation and explanation commands | Completed | Implement capability-advertised `graph validate` and `graph explain` with fail-closed schema/source checks and compact human/JSON output. | Implemented read-only `kkachi-agent-helper graph validate/explain`, `workflow_graph_readonly` capability evidence, help/docs coverage, fail-closed source/schema validation, and unit/CLI/integration/E2E regressions. |
| graph-003 | Semantic diff and proposal record format | Completed | Implement semantic graph diff plus proposal record storage that preserves changed phases, edges, gates, approvals, risk flags, and next action. | Implemented `kkachi-agent-helper graph diff` and `graph propose` with proposal records under `.kkachi/graph/proposals/`; proposal records do not apply graph changes. |
| graph-004 | `init --from-template` template ingestion and initial graph write | Completed | Accept explicit KHS template id/path, validate input, write initial `.kkachi-workflow.yaml` only when no graph exists. | Implemented `kkachi-agent-helper graph init --from-template` with built-in `khs-default`, explicit template paths, canonical initial graph write, `graph.initialized` audit event, checksum evidence, `workflow_graph_init` capability, and fail-closed existing-graph behavior. Use `init --from-template`, not `init --profile`; approved replacement is graph apply. |
| graph-005 | Approval-gated apply with audit events and fail-closed source precedence | Completed | Apply approved proposals atomically, record checksum/version and graph audit event ids, and fail closed on invalid/missing/conflicting sources. | Implemented `kkachi-agent-helper graph apply --proposal ... --approval ...` with proposal/base/candidate checksum checks, canonical graph replacement, `last_applied_event_id`, `graph.applied` audit event, `workflow_graph_apply` capability, and no KAH policy decision. |
| graph-006 | Visualization export to Mermaid/PlantUML as generated artifacts only | Completed | Export non-authoritative diagrams with source checksum and `authoritative: false` in JSON output. | Implemented `kkachi-agent-helper graph export --format mermaid|plantuml [--output <path>]` with generated diagram output, source checksum evidence, `workflow_graph_export` capability, and fail-closed validation/output path checks. Exports never become graph source of truth. |
| graph-007 | KHS compatibility diagnostics/capabilities for graph support and no direct YAML fallback | Completed | Advertise graph support through capabilities, publish compatibility diagnostics, and make KHS fail closed when graph support is required but absent. | Implemented `workflow_graph_diagnostics` and `workflow_graph_no_direct_yaml_fallback` capability flags plus diagnostics `graph_compatibility` evidence with graph validation state, forbidden fallback sources, and support-safe missing/invalid graph reporting. No silent direct YAML edit fallback. |
| graph-008 | External feedback intake docs contract and roadmap | Completed | Lock the configurable `EXTERNAL_FEEDBACK_INTAKE` planning contract before code changes: update permanent specs, compatibility, and roadmap references while keeping current implemented behavior and stale `1..3` wording clearly marked. | Completed docs-only contract before implementation; durable behavior now lives in permanent specs, compatibility, roadmap, and docs index. No code, schemas, command behavior, diagnostics, capabilities, KHS templates, or KAB runtime behavior changed. |
| graph-009 | External feedback intake graph schema and read-only validation | Completed | Extend workflow graph parsing/validation and read-only projection to understand schema-owned feedback bounds without advertising full support. Validate min/max bounds, required round 1, optional continuation, stale/unknown/duplicate/conflicting declarations, and round 6 rejection. | Implemented read-only `feedback_intake` parsing, validation, validate/explain projection, semantic diff risk flagging, focused unit/CLI/integration/E2E coverage, and docs parity. Final support remained unadvertised in this slice; no run-local phase-plan behavior, diagnostics capability, KHS template, or KAB behavior changed. |
| graph-010 | External feedback intake phase-plan validation | Completed | Replace fixed `1..3` phase-plan validation with policy-driven feedback bounds while preserving request/handle pairing, round 1 requirement, optional 2..5 continuation, final evidence checks, and fail-closed graph-vs-run conflict reporting. | Implemented graph-policy-driven `phase-plan validate`: valid `feedback_intake` is required for feedback rows, round 4/5 pairs are allowed when declared, round 6+ and missing policy fail closed, optional rounds are not mandatory, and final checks cover declared feedback rows. This slice did not advertise diagnostics/capability activation; graph-011 completed that final activation evidence. |
| graph-011 | External feedback intake migration, diagnostics, capabilities, and final docs | Completed | Add stale `1..3/max3` migration/proposal support, diagnostics evidence, final capability/compatibility advertisement, docs parity, and E2E coverage so KHS can consume configurable feedback intake support fail-closed. | Implemented stale-only feedback-bound migration through proposal/apply evidence with explicit approval/audit reference, nested diagnostics `graph_compatibility.feedback_intake` evidence, `workflow_graph_configurable_feedback_intake` capability advertisement, docs parity, and focused unit/CLI/integration/E2E coverage. |
| graph-012 | Declarative workflow gate checks and final-required graph gates | Completed | Let `.kkachi-workflow.yaml` declare project-specific deterministic gate checks without KAH core changes for every new gate. Preserve built-in gate priority, additive schema compatibility, final gate freshness, and fixed check types only. | Implemented workflow gate fallback for `gate check`, graph `checks` parsing/projection/validation, `final_required` final-gate inclusion, optional report name/message/hint shaping, Red consultation conditions, KAN plugin precommit-template migration evidence, and focused unit/CLI/e2e coverage. |


### EPIC: grsync — Workflow graph sync diagnostics and repair safety

> Goal: let KAS v0.1.2 safely determine whether a project `.kkachi-workflow.yaml` is supported by the effective KAS/KAH pair, while KAH remains the deterministic validator/proposal/apply/audit owner and does not become workflow-policy authority.
>
> Source of truth: `docs/sot/graph-workflow-sync-diagnostics-and-repair.md`. Target release: KAH `v0.1.9`, consumed by KAS `v0.1.2`.

| Task ID | Title | Status | Work guide | Notes |
|---|---|---|---|---|
| grsync-001 | Graph compatibility diagnostics and reason-code hardening | Implemented pending review | Harden `graph validate/explain/diagnostics` JSON so KAS can distinguish old KAH, missing/stale/broken/manual-edit/checksum-mismatch graph state, feedback-intake drift, forbidden fallback sources, and repairability without KAH choosing workflow policy. | Adds stable `reason_codes` to graph validate/explain and diagnostics `graph_compatibility`, fixture tests for valid/missing/parse-error/forbidden-source/stale-feedback states, docs/specs/compatibility/index updates, and v0.1.9 draft release notes. KAH reports deterministic support facts only; KAS decides support envelope and update guidance. |
| grsync-002 | Approval-gated stale/broken graph repair substrate | Implemented pending review | Support proposal/apply repair for missing, stale, or broken `.kkachi-workflow.yaml` through complete candidate graphs, base-state evidence, drift checks, explicit approval refs, atomic writes, backup/recovery evidence, checksums, and audit events. | Implemented complete-candidate repair proposals for missing/stale/broken/invalid base graphs, base reason-code endpoint evidence, `graph_replacement` approval-gated apply, drift-safe base evidence rechecks, atomic writes, backup/recovery refs for existing graph replacement, and regression tests. No direct YAML edit fallback, no partial patch DSL, no automatic apply from periodic checks, no KAS policy decision in KAH. |

GRSYNC deferrals unless separately approved: KAS compatibility registry, KAS doctor/repair CLI behavior, automatic KAH binary update, KAB graph authority, `kah graph` alias behavior, and merge/fallback with Kkachi v2 `.kkachi/config/workflows/`.

### EPIC: DAGSM — Task DAG state machine substrate

> Goal: provide the deterministic KAH substrate for KAS WFLOW task-DAG workflows: schema validation, workflow instance state, node FSM/order enforcement, ready-node calculation, required evidence checks, multi-DAG catalog diagnostics, and final gate integration without KAH choosing agents or workflow policy.
>
> Source of truth: `docs/sot/task-dag-state-machine.md`. Paired KAS policy epic: `WFLOW` in `kkachi-agent-skills/docs/roadmap.md` and KAS SOT `docs/sot/task-dag-workflow-contract.md`. This KAH SOT/docs registration is committed as a `WFLOW-001` companion planning artifact; `DAGSM-001` remains the first KAH implementation PR.
>
> Cross-repo linear order: `WFLOW-001 -> DAGSM-001 -> DAGSM-002 -> WFLOW-002 -> WFLOW-003 -> DAGSM-003 -> WFLOW-004`.

| Task ID | Title | Status | Work guide | Notes |
|---|---|---|---|---|
| DAGSM-001 | Task DAG schema validation and explain diagnostics | Planned | After KAS `WFLOW-001` is accepted, implement deterministic task-DAG schema validation/explain behavior for nodes, dependencies, `all_of` fan-in, required outputs, invalid schema, duplicate nodes, unknown dependencies, cycles, path escapes, and unsupported joins. Pin final workflow command names before any support claim or downstream KAS dependency. Advertise capability evidence only when implemented. | Depends on KAS `WFLOW-001` policy acceptance. The current KAH SOT/docs registration is only the `[WFLOW-001]` companion planning artifact; `DAGSM-001` implementation is not in review yet and still requires tests/specs/compatibility/release-note updates before support claims. |
| DAGSM-002 | Workflow instance state, node FSM, and ready-node calculation | Planned | Implement run-local workflow instance state, node transition commands, ready-node calculation, required-output/evidence completion checks, node blockers, and event/audit recording. Preserve KAS ownership of agent/role policy and KAB/Kanban execution. All state mutations must use KAH safe-path checks, active-run/project-write locking, atomic writes, status/event coherence, and stale/concurrent transition refusal. | Depends on `DAGSM-001`. Enables KAS `WFLOW-002` generic trigger. Evidence must cover sequence, fan-out, `all_of` fan-in, blocked/not-ready transitions, lock/coherence failures, path escapes, and final incomplete evidence failures. |
| DAGSM-003 | Multi-DAG catalog diagnostics and final gate integration | Planned | Validate project multi-DAG catalog references, expose deterministic diagnostics/reason codes, integrate workflow completeness into final gates/diagnostics, and support KAS selector/node-contract metadata without KAH selecting workflows or agents. | Depends on `DAGSM-002` and KAS `WFLOW-003` metadata shape. Enables KAS `WFLOW-004` custom workflow creator. No automatic apply, KAS policy choice, or KAB graph authority. |

DAGSM deferrals unless separately approved: KAS selector implementation, KAS custom trigger generation, KAB backend execution, Kanban assignment, automatic KAH/KAS update, automatic graph/catalog apply, dynamic node creation, retry/rollback automation, fallback agent selection, webhook daemons, and Hermes profile/provider/gateway/auth/token/model mutation.

### EPIC: token — KAS token-economy evidence gates

> Goal: support KAS token-economy and English-output work with deterministic KAH evidence gates while preserving the boundary that KAS owns workflow/prompt/lifecycle policy and KAH owns only machine-checkable artifacts, schemas, events, gates, diagnostics, and pass/fail/not_applicable validation.
>
> Source of truth: `docs/sot/token-economy-evidence-gates.md` is the KAH-side planning SOT. The upstream KAS workstream authority is `kkachi-hermes-skills/docs/sot/token-economy-and-agent-instruction-contract.md`.

| Task ID | Title | Status | Work guide | Notes |
|---|---|---|---|---|
| token-001 | Token-economy and English-output evidence gate contract | Completed | Add a deterministic KAH evidence surface for KAS-managed token-economy work: compact English output evidence, artifact-first detail references, repo-local `AGENTS.md` / `CLAUDE.md` evidence, project KAS lifecycle dry-run/apply/manifest/doctor evidence, and explicit mutation-approval evidence when such claims are in scope. Results must be only `pass`, `fail`, or `not_applicable`. | Implemented as the `token-economy` built-in gate plus canonical `token-economy-evidence.json` / `token-economy-evidence` schema (`token001.v1`). The gate is active only for `task_id=token-001`, fails closed for missing/malformed/unsafe/checksum-mismatched or token-002-only evidence, and keeps KAS policy/lifecycle decisions, KAB activation, Hermes runtime/profile/provider/auth/token/gateway/model mutation, prose quality judgment, and warning-only advisory states out of KAH. |
| token-002 | Verification, evidence-summary, review-bundle, and change-aware gate support | Completed | Add deterministic KAH evidence support for KAS-selected verification profiles, no-agent runner artifacts, reversible evidence summaries and phase packets, compact review bundles, no-agent fan-in watcher terminal reports, and change-aware verification matrix evidence. Results must be only `pass`, `fail`, or `not_applicable`. | Implemented in the TOKEN-002 working tree as a `token-economy` gate extension for `task_id=token-002` / `schema_version=token002.v1`; implementation, docs, verification, first color review, feedback handling, GLM Octo review, and post-Octo color re-review accepted for commit-readiness. Commit, push, install, global binary rebuild, release tagging, KAS lifecycle activation, KAB product activation, and Hermes runtime/profile/provider/auth/token/gateway/model mutation remain unapproved until separate 주군 approval. KAH must not choose verification profiles, decide test skip policy, summarize logs, judge review quality, operate watcher policy, replace Kanban/color review evidence, activate KAB, mutate Hermes runtime/profile/provider/auth/token/gateway/model config, or introduce warning-only advisory states. |

## Backlog and review points

- Revisit implementation language and package manager before `corex-001` starts.
- Keep `docs/specs.md` authoritative for helper behavior; this roadmap controls delivery order.
- Keep helper validation deterministic. Backend choice, planning, and review reasoning remain commander/general responsibilities.
- Do not promote helper behavior into shared Kkachi skills until the behavior is implemented, tested, and reflected in the install/package contracts.
- Review this roadmap after each epic; split only tasks that prove too large for one reviewable PR.

## Planning graph record appendix

Date: 2026-05-21
Owner: KAH roadmap archive
Confirming role: Responsible approver / governance evidence record
Status: graph-007 compatibility diagnostics evidence recorded
Authority level: active roadmap planning record; not implementation authorization by itself
Scope: KAH docs roadmap only
Related docs: `README.md`, `specs.md`, `compatibility.md`
Decision summary: add `graph — Command-managed workflow graph` as PR-candidate roadmap epic, complete read-only `graph-002` validation/explanation, complete `graph-003` semantic diff/proposal records, complete `graph-004` initial graph init, complete `graph-005` approval-gated apply, complete `graph-006` non-authoritative visualization export, complete `graph-007` compatibility diagnostics/no-direct-YAML-fallback capability evidence, and mark the deleted `docs/TODO-ALIGN.md` pointer stale.
Evidence/source paths: governance evidence record in kanban task `t_2fb00394`
Stale/conflict markers: `docs/TODO-ALIGN.md` is deleted in the current working tree and is not active authority; the `kah graph` shorthand remains candidate until capabilities/help prove it.
Open questions: `graph-008+` implementation details must be refined one graph task at a time with capability/help evidence.
Next record action: keep graph compatibility diagnostics aligned with release evidence without widening graph export into generated-artifact authority or `kah graph` alias behavior.
