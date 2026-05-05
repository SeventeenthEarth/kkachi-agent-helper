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

## Active roadmap

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

## Backlog and review points

- Revisit implementation language and package manager before `corex-001` starts.
- Keep `docs/specs.md` authoritative for helper behavior; this roadmap controls delivery order.
- Keep helper validation deterministic. Backend choice, planning, and review reasoning remain commander/general responsibilities.
- Do not promote helper behavior into shared Kkachi skills until the behavior is implemented, tested, and reflected in the install/package contracts.
- Review this roadmap after each epic; split only tasks that prove too large for one reviewable PR.
