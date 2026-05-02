# kkachi-agent-helper

`kkachi-agent-helper` is the deterministic local CLI helper for Kkachi project state, run artifacts, locks, schemas, events, and install scaffolding.

The current implementation covers the `corex-001` through `corex-005` foundations, `runwf-001` through `runwf-004`, and `gates-001` through `gates-005`: repository layout, Go toolchain, command shell, version output, repo-root discovery, safe repository-relative path handling, symlink escape rejection, canonical exit codes, structured human/JSON errors, verification commands, safe `.kkachi/` project initialization, atomic state writes, append-only event handling, `last_event_id` coherence checks, read-only `project status` / `project doctor` diagnostics, run metadata lifecycle commands, one-active-write lock enforcement, stale-lock diagnostics, explicit recorded lock recovery, artifact manifest initialization, artifact listing, read-only intake validation, the mutating `gate check` readiness surface, deterministic SOT/roadmap/plan pre-implementation gates, the manifest-driven backend evidence gate, implementation/review/verification/docs/final gates, run-local gate reports, and fixture-backed gate regression coverage.

## Source of truth

- [Specs](docs/specs.md)
- [Roadmap](docs/roadmap.md)

## Build and verify

```sh
make build
make test-prepare
make test-unit
make test-int
make test-e2e
make test
make check
```

The built binary is written to `bin/kkachi-agent-helper`.

Test lanes are intentionally split:

- `make test-prepare` runs formatting and static preparation checks.
- `make test-unit` runs single-file/unit-level tests without external systems.
- `make test-int` runs multi-component integration tests without external systems.
- `make test-e2e` runs local end-to-end scenarios. For `runwf-001`, `runwf-003`, `runwf-004`, and `gates-001` through `gates-005`, it builds the helper, initializes a temporary project, verifies generated `.kkachi/` state, runs `project status` and `project doctor`, creates a run, initializes/lists/validates canonical artifacts, verifies required artifact metadata, checks validation is read-only, runs `gate check intake`, `sot`, `roadmap`, `plan`, and adapter QA `backend`, verifies `gate.passed` plus status/metadata gate summaries and run-local gate reports, activates/closes the run, appends an event, checks `last_event_id` coherence, verifies doctor reports incoherent state, and checks overwrite refusal. For `runwf-002`, it verifies fresh lock conflicts, stale-lock diagnostics, explicit recovery, `lock.recovered` event recording, lock removal, and post-recovery mutation success.
- `make test` runs `test-prepare`, `test-unit`, `test-int`, and `test-e2e` sequentially.

## CLI examples

```sh
kkachi-agent-helper --version
kkachi-agent-helper version --json
kkachi-agent-helper project init
kkachi-agent-helper project init --json
kkachi-agent-helper project status
kkachi-agent-helper project status --json
kkachi-agent-helper project doctor
kkachi-agent-helper project doctor --json
kkachi-agent-helper event append artifact.written --run run-abc --payload '{"path":"impl-log.md"}' --json
kkachi-agent-helper run create --title 'Run workflow metadata' --work-path A_development_execution --work-mode standard --urgency normal --sot-policy existing_sot_basis --execution-mode production_write --commander Gongmyeong --task-id runwf-001 --json
kkachi-agent-helper run list
kkachi-agent-helper run show <run_id> --json
kkachi-agent-helper run activate <run_id> --json
kkachi-agent-helper run close <run_id> --json
kkachi-agent-helper run abort <run_id> --json
kkachi-agent-helper artifact init <run_id> --json
kkachi-agent-helper artifact list <run_id> --json
kkachi-agent-helper artifact validate <run_id> --gate intake --json
kkachi-agent-helper gate check <run_id> intake --json
kkachi-agent-helper gate check <run_id> plan --json
kkachi-agent-helper gate check <run_id> backend --json
kkachi-agent-helper lock recover project-write --reason 'confirmed stale helper process' --json
```

For `corex-004`, `project init` creates `.kkachi/config.yaml`, `.kkachi/status.json`, `.kkachi/events.jsonl`, and the initial `.kkachi/schemas/*.schema.json` files using atomic new-file writes. It allows existing empty helper directories but refuses to overwrite any helper-managed file.

`event append` appends one JSONL event, allocates the next `evt-000001`-style id, and atomically advances `status.last_event_id`. It fails closed if the status file and event log tail are incoherent. CLI payloads are capped at 256 KiB; larger evidence should be written to artifacts and referenced from the event payload.

For `corex-005`, `project status` and `project doctor` are read-only. They do not repair `.kkachi/`, append events, create locks, or rewrite status. `project status` summarizes root path, health, project identity, active run fields, `last_event_id`, event-log tail/count, `updated_at`, gate summary, and issues. `project doctor` reports pass/warn/fail checks for config, status, events, canonical paths, schema availability, lock files, and status/event coherence. Present lock files are warnings; malformed files, unsafe paths, schema problems, and coherence mismatches fail closed.

`runwf-001` implements `run create`, `run list`, `run show`, `run activate`, `run close`, and `run abort`. Run ids use `run-YYYYMMDDTHHMMSSZ-<12hex>`. Full ids resolve exactly; prefixes resolve only when unique, and missing or ambiguous prefixes fail closed. `run create` records `.kkachi/runs/<run_id>/run-metadata.json` with `state: "created"`, empty `required_artifacts`, empty `gate_state`, and a `run.created` event. `run activate` only accepts `created` runs and sets `status.active_run_id` / `status.active_run_state` with `run.activated`. `run close` and `run abort` only accept `created` or `active` runs, clear active status fields when they target the active run, and append `run.closed` / `run.aborted`. `artifact init <run_id>` remains the boundary that populates run artifacts after run creation.

`runwf-003` implements `artifact init <run_id>` and `artifact list <run_id>`. The artifact manifest is derived from run work path, work mode, execution mode, and red-team assignment using canonical artifact names from `docs/specs.md`. `artifact init` creates baseline non-empty run files under `.kkachi/runs/<run_id>/`, updates `run-metadata.json.required_artifacts`, and appends an `artifact.written` event. Existing non-empty artifacts are preserved; existing empty artifacts are reinitialized with baseline content. `artifact list` is read-only and reports every canonical artifact path with required/present/empty/byte status.

`runwf-004` implements read-only `artifact validate <run_id> [--gate intake]`. It validates manifest coherence, completed `intake-classification.md` fields, Path A/B SOT-policy eligibility, urgency metadata, Light-mode reason recording, and the required `Status: not_applicable` / `Reason: ...` not-applicable format for later gates. Validation reports exit `0` for pass and exit `3` with structured failed checks for validation failures without mutating `.kkachi/`.

`gates-001` implements `gate check <run_id> <gate>`. It is a mutating readiness check: it records the result in `run-metadata.json.gate_state`, updates `status.json.gate_summary`, appends `gate.passed`, `gate.failed`, or `gate.checked`, and writes `.kkachi/runs/<run_id>/gate-reports/<gate>.json`. The `intake` gate reuses the deterministic intake validation rules from `artifact validate`. `gates-002` implements the pre-implementation `sot`, `roadmap`, and `plan` gates: Path A requires completed `sot-basis.md`, Path B requires completed `sot-update.md`, roadmap passes through `task_id` trace or completed/not-applicable `roadmap-update.md`, and plan requires completed `acceptance-criteria.md`, `plan.md`, and `checklist.md`. `gates-003` implements `backend` as a manifest-driven evidence gate: when `run-metadata.json.required_artifacts` includes backend artifacts, it validates `selected-cli.json`, `capability-check.md`, `bridge-session-snapshot.json`, and `bridge-events.md`; otherwise it records a not-applicable pass tied to the run manifest. `gates-004` implements `implementation`, `review`, `verification`, `docs`, and `final`/`gate final`. `gates-005` preserves the latest report path in metadata/status and locks behavior with valid/invalid Path A/B Standard/Light fixtures, including malformed evidence and missing artifacts. Passing gates exit `0`; failed or blocked gate checks exit `3`; unknown gate names are usage errors.

`runwf-002` serializes helper-state writes with `.kkachi/project_write.lock` and run lifecycle transitions with `.kkachi/active_run.lock`. Locks are created atomically, contain owner pid, hostname, command, optional run id, and timestamp metadata, and are released only when the recorded identity still matches. Fresh locks make mutating commands fail closed with `lock_conflict`; stale locks fail with `lock_stale_recovery_required` until an operator runs `lock recover <active-run|project-write|all> --reason <text> [--run <run_id>]`. Recovery refuses malformed or fresh locks, appends a `lock.recovered` event before removing stale locks, and advances `status.last_event_id`. `project doctor` remains read-only and reports absent locks as pass, fresh/stale readable locks as warnings, and malformed, unreadable, non-regular, or path-unsafe lock files as failures.

Other command groups such as `schema` and `install`, plus later `project` subcommands, remain reserved placeholders. Repo-bound command groups first require a discoverable Git or `.kkachi` repository root, then return deterministic `not_implemented` errors until their roadmap tasks add real behavior.

Error output is stable for both humans and scripts:

- Human errors include `error`, optional structured fields, `exit_code`, and `hint`.
- JSON errors are emitted as `{"error": ...}` without decorative text when `--json` is present.
- Canonical exit codes are `0` for success, including doctor/status reports with only warnings; `1` for internal failures; `2` for usage, unsupported arguments, or unsupported command state; `3` for fail-closed state problems such as malformed files, unsafe paths, schema failures, or status/event coherence mismatches; and `4` for missing repository roots.
