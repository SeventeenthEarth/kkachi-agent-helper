# kkachi-agent-helper Specs

Date: 2026-04-30
Owner: Gongmyeong
Status: initial source of truth

## 1. Purpose

`kkachi-agent-helper` is the deterministic command-line helper for the Kkachi software delivery harness. It owns local project state, run artifacts, locks, schema validation, event logging, and initialization of Kkachi project scaffolding. It does not plan work, choose a coding backend, review code, or act as an intelligence layer.

The helper exists so Hermes team members and external coding CLIs can operate through a repeatable, auditable workflow without relying on chat memory or prompt claims as the source of truth.

## 2. Repository role

Kkachi is split into three independently versioned repositories:

| Repository | Responsibility |
|---|---|
| `kkachi-agent-bridge` | Runtime integration with external AI coding CLIs. |
| `kkachi-agent-helper` | Deterministic state, artifact, schema, lock, and bootstrap tooling. |
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
    config.schema.json
    status.schema.json
    event.schema.json
    run-metadata.schema.json
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
      gate-reports/
        <gate>.json
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

The helper must not override the commander's backend choice. It may fail a gate if backend evidence was declared required but canonical backend artifacts are missing from `required_artifacts`, or if the selected backend record is missing, malformed, stale, or marked unsupported for the declared execution mode. `selected-cli.json` passes only with an object containing non-empty `version`, `status`, `backend_type`, `adapter_type`, and `source_ledger_ref`, plus a declared `caveats` array of strings; `status` must be `supported` or `degraded`. `capability-check.md` and `bridge-events.md` require `Status: complete`; the capability check must mention the selected backend and adapter, and bridge events must include non-empty behavior evidence. `bridge-session-snapshot.json` must match the selected backend/adapter, declare a non-empty `session_id`, `state`, and `lifecycle_class`, and have `open_pendings: 0`.

## 10. CLI surface

Initial command groups:

```text
kkachi-agent-helper project init \
  --project-name kkachi-agent-bridge \
  --stack go \
  --repo-path "$PWD" \
  --commander Gongmyeong \
  --redteam Macho \
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
```

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

The schema selector accepts canonical embedded names (`config`, `status`, `event`, `run-metadata`, `selected-cli`, `bridge-session-snapshot`) or canonical project-local schema paths under `.kkachi/schemas/`. Project-local schema paths are identity-checked, but validation remains embedded-registry-backed so a relaxed local schema cannot make invalid helper state pass.

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

KAH no longer exposes an `install` command. Hermes skill installation is handled by Hermes native skill tooling, while KAH initializes the project-local files that KHS/Hermes skills use as their deterministic working contract.

`project init` is a one-shot bootstrap command that requires explicit project parameters. It creates existing helper state plus:

- `.kkachi/project-overlay.yaml`
- `docs/kkachi-docs-map.yaml`

`project init --force` is a reconfiguration command, not a destructive reset. It rewrites `.kkachi/config.yaml`, `.kkachi/project-overlay.yaml`, `docs/kkachi-docs-map.yaml`, and schema copies from the supplied parameters, preserves `status.json`, `events.jsonl`, `.kkachi/runs/**`, run metadata, artifacts, and gate history, and appends `project.reconfigured`.

Command UX rules:

- `--json` emits machine-readable output and no decorative text.
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

### `diagnostics export`

`pilot-002` introduces `diagnostics export [--run <run_id-or-prefix>] [--output <repo-relative-path>]` as a support-safe diagnostic bundle. The command is deterministic and does not append events, take locks, recover state, or repair `.kkachi/`. Without `--output`, it writes the JSON bundle to stdout. With `--output`, it writes one new repository-confined JSON file and prints a compact summary unless `--json` is used.

The bundle includes redacted project config, status, event entries, project-local schema versions, run-local gate reports, and a selected support artifact set for the requested run. If `--run` is omitted, the active run is used when one is recorded in `status.json`; otherwise the bundle contains project-level diagnostics only. Selected artifacts are intentionally narrower than the full artifact tree: `run-metadata.json`, intake classification, backend evidence files, test/verification/docs-update evidence, and `final-report.md`.

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

## 17. Open decisions

The following items remain open until roadmap tasks close them:

- run id format;
- exact lock stale detection policy;
- whether helper exports a library API in addition to the CLI;
- whether bridge capability registry validation is direct or delegated to `kkachi-hermes-skills` assets;
- authoritative bridge and skills version sources for enforced compatibility beyond `compat.required_helper`.
