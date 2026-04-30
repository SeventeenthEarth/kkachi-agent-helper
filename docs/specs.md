# kkachi-agent-helper Specs

Date: 2026-04-30
Owner: Gongmyeong
Status: initial source of truth

## 1. Purpose

`kkachi-agent-helper` is the deterministic command-line helper for the Kkachi software delivery harness. It owns local project state, run artifacts, locks, schema validation, event logging, and installation of Kkachi project scaffolding. It does not plan work, choose a coding backend, review code, or act as an intelligence layer.

The helper exists so Hermes team members and external coding CLIs can operate through a repeatable, auditable workflow without relying on chat memory or prompt claims as the source of truth.

## 2. Repository role

Kkachi is split into three independently versioned repositories:

| Repository | Responsibility |
|---|---|
| `kkachi-agent-bridge` | Runtime integration with external AI coding CLIs. |
| `kkachi-agent-helper` | Deterministic state, artifact, schema, lock, and install tooling. |
| `kkachi-hermes-skills` | Hermes phase skills, orchestration skills, templates, registries, and evaluation assets. |

`kkachi-agent-helper` must stay small, local-first, scriptable, and safe to call from agents, shell scripts, and future UI surfaces.

## 3. Non-goals

The helper must not:

- decide which external backend is best for a task;
- generate implementation plans using model reasoning;
- replace Hermes skills or project overlays;
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
.kkachi/
  config.yaml
  status.json
  events.jsonl
  active_run.lock
  project_write.lock
  schemas/
    status.schema.json
    run-metadata.schema.json
    event.schema.json
    selected-cli.schema.json
    bridge-session-snapshot.schema.json
  runs/
    <run_id>/
      run-metadata.json
      intake-classification.md
      sot-basis.md
      task-brief.md
      acceptance-criteria.md
      plan.md
      checklist.md
      selected-cli.json
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
```

Light mode may use the same artifact names with shorter content or explicit not-applicable records. It must not introduce an incompatible artifact schema without a versioned migration.

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
| `active_run_state` | string or null | `draft`, `ready`, `running`, `blocked`, `review`, `verified`, `closed`, or `aborted`. |
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
- `schema.migrated`

## 7. Run metadata

`run-metadata.json` records how the work is classified and who owns it.

Minimum fields:

| Field | Allowed values or shape |
|---|---|
| `version` | schema version |
| `run_id` | helper-generated id |
| `task_id` | roadmap task id or null |
| `title` | short title |
| `work_path` | `A_development_execution` or `B_discovery_shaping` |
| `work_mode` | `standard` or `light` |
| `urgency` | `normal`, `urgent`, or `critical` |
| `sot_policy` | `existing_sot_basis`, `minimal_sot_before_code`, or `full_sot_before_code` |
| `execution_mode` | `production_write`, `adapter_qa`, `readiness_hardening`, `research`, `verification`, or `docs_only` |
| `commander` | assigned team-member profile name |
| `redteam` | assigned red-team profile name or null |
| `created_at` | ISO timestamp |
| `state` | run lifecycle state |
| `required_artifacts` | list of artifact paths required by gates |
| `gate_state` | per-gate pass/fail/block summary |

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

When a run uses `kkachi-agent-bridge`, helper validation covers artifact shape only. The commander owns the reasoning.

Required files:

| Artifact | Helper responsibility |
|---|---|
| `selected-cli.json` | Validate required fields, supported status values, source ledger reference, and declared caveats. |
| `capability-check.md` | Validate presence and link to selected backend identity. |
| `bridge-session-snapshot.json` | Validate session identity fields such as `session_id`, `backend_type`, `adapter_type`, state, lifecycle class, and open pendings. |
| `bridge-events.md` | Validate presence when backend behavior matters. |

The helper must not override the commander's backend choice. It may fail a gate if the selected backend record is missing, malformed, stale, or marked unsupported for the declared execution mode.

## 10. CLI surface

Initial command groups:

```text
kkachi-agent-helper project init
kkachi-agent-helper project doctor
kkachi-agent-helper project status [--json]
kkachi-agent-helper run create
kkachi-agent-helper run activate <run_id>
kkachi-agent-helper run close <run_id>
kkachi-agent-helper run abort <run_id>
kkachi-agent-helper artifact init <run_id>
kkachi-agent-helper artifact list <run_id> [--json]
kkachi-agent-helper artifact validate <run_id> [--gate <gate>]
kkachi-agent-helper gate check <run_id> <gate>
kkachi-agent-helper gate final <run_id>
kkachi-agent-helper event append <type> --run <run_id> --payload <json>
kkachi-agent-helper schema validate <file> --schema <schema>
kkachi-agent-helper schema migrate --from <version> --to <version>
kkachi-agent-helper install skills --source <path-or-version>
kkachi-agent-helper install templates --source <path-or-version>
```

Command UX rules:

- `--json` emits machine-readable output and no decorative text.
- Non-zero exit means the requested action did not succeed.
- Validation failures include path, field, expected value, actual value, and remediation hint.
- Mutating commands append an event unless the command fails before mutation.
- `event append` is itself the primitive append-only event mutation; it fails if status and event-log tail ids are incoherent.
- `event append` keeps payloads compact: CLI payload input is limited to 256 KiB and serialized JSONL event lines are limited to 1 MiB. Larger evidence belongs in run artifacts.
- Event run ids may be omitted/null; when present, they must be printable, newline-free strings. Full run id syntax is defined by later run workflow tasks.
- State-file creation and replacement use atomic temp-file writes with durable sync before publish where the host filesystem supports it.
- Commands must reject absolute paths, paths escaping the repository root, and ambiguous run ids.

## 11. Locking

Default lock files:

| Lock | Purpose |
|---|---|
| `.kkachi/active_run.lock` | Prevent conflicting active-run transitions. |
| `.kkachi/project_write.lock` | Enforce one active write lane per project. |

Lock requirements:

- Use atomic create where possible.
- Record owner process id, hostname, command, run id, and timestamp.
- Provide `doctor` diagnostics for stale locks.
- Provide explicit recovery commands rather than silent lock removal.
- Do not allow forced unlock without an event record.

## 12. Schema and migration policy

- Schemas live embedded in the binary and may also be copied under `.kkachi/schemas/` for transparency.
- `project init` writes project-local minimal JSON Schema draft 2020-12 copies for the canonical schema names. These copies require `version` and remain intentionally permissive until `packg-001` adds the full registry and validator surface.
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
| `plan` | `acceptance-criteria.md`, `plan.md`, `checklist.md`. |
| `backend` | bridge evidence artifacts when bridge is used. |
| `implementation` | `diff.patch`, `impl-log.md`, optional `cli-output.md`. |
| `review` | `review.md` and required red-team artifacts. |
| `verification` | `test-log.md`, `verification.md`, pass/fail verdict. |
| `docs` | `docs-update.md` and changed docs list or no-change reason. |
| `final` | all required gates pass, no open blockers, `final-report.md` exists. |

## 14. Install and project initialization

`project init` creates `.kkachi/`, default config, schemas, status, and event log. It must not overwrite existing helper state without an explicit migration or reset command.

Initial `project init` defaults:

- `project.name` is derived from the repository basename as a slug.
- `status.project_id` uses `kkachi-project-<project-slug>-<random-hex>`.
- `status.last_event_id` is `evt-000001`.
- `.kkachi/events.jsonl` contains exactly one initial `project.initialized` JSONL record.
- `.kkachi/schemas/` contains minimal local schema copies for status, run metadata, event, selected CLI, and bridge session snapshot.

Skill and template installation should support:

- local path source for development;
- versioned package source later;
- manifest with checksums;
- dry-run preview;
- managed block replacement for helper-owned files only;
- preservation of user-owned files.

## 15. Testing standard

Minimum implementation test layers:

| Layer | Required coverage |
|---|---|
| Unit | schema validation, path safety, id generation, lock behavior, gate rules. |
| Integration | project init, run create/activate/close, event append, schema migration. |
| Local E2E | User-visible CLI flows such as project init success, generated state files, JSON output, and unsafe overwrite refusal. |
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

## 17. Open decisions

The following items remain open until roadmap tasks close them:

- implementation language and packaging strategy;
- exact schema syntax and validator library;
- run id format;
- exact config schema;
- exact lock stale detection policy;
- skill/template package manifest format;
- whether helper exports a library API in addition to the CLI;
- whether bridge capability registry validation is direct or delegated to `kkachi-hermes-skills` assets;
- release versioning and compatibility guarantees.
