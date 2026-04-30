# kkachi-agent-helper

`kkachi-agent-helper` is the deterministic local CLI helper for Kkachi project state, run artifacts, locks, schemas, events, and install scaffolding.

The current implementation covers the `corex-001` through `corex-005` foundations: repository layout, Go toolchain, command shell, version output, repo-root discovery, safe repository-relative path handling, symlink escape rejection, canonical exit codes, structured human/JSON errors, verification commands, safe `.kkachi/` project initialization, atomic state writes, append-only event handling, `last_event_id` coherence checks, and read-only `project status` / `project doctor` diagnostics.

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
- `make test-e2e` runs local end-to-end scenarios. For `corex-005`, it builds the helper, initializes a temporary project, verifies generated `.kkachi/` state, runs `project status` and `project doctor`, appends an event, checks `last_event_id` coherence, verifies doctor reports incoherent state, and checks overwrite refusal.
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
```

For `corex-004`, `project init` creates `.kkachi/config.yaml`, `.kkachi/status.json`, `.kkachi/events.jsonl`, and the initial `.kkachi/schemas/*.schema.json` files using atomic new-file writes. It allows existing empty helper directories but refuses to overwrite any helper-managed file.

`event append` appends one JSONL event, allocates the next `evt-000001`-style id, and atomically advances `status.last_event_id`. It fails closed if the status file and event log tail are incoherent. CLI payloads are capped at 256 KiB; larger evidence should be written to artifacts and referenced from the event payload.

For `corex-005`, `project status` and `project doctor` are read-only. They do not repair `.kkachi/`, append events, create locks, or rewrite status. `project status` summarizes root path, health, project identity, active run fields, `last_event_id`, event-log tail/count, `updated_at`, gate summary, and issues. `project doctor` reports pass/warn/fail checks for config, status, events, canonical paths, schema availability, lock files, and status/event coherence. Present lock files are warnings; malformed files, unsafe paths, schema problems, and coherence mismatches fail closed.

Other command groups such as `run`, `artifact`, `gate`, `schema`, and `install`, plus later `project` subcommands, remain reserved placeholders. Repo-bound command groups first require a discoverable Git or `.kkachi` repository root, then return deterministic `not_implemented` errors until their roadmap tasks add real behavior.

Error output is stable for both humans and scripts:

- Human errors include `error`, optional structured fields, `exit_code`, and `hint`.
- JSON errors are emitted as `{"error": ...}` without decorative text when `--json` is present.
- Canonical exit codes are `0` for success, including doctor/status reports with only warnings; `1` for internal failures; `2` for usage, unsupported arguments, or unsupported command state; `3` for fail-closed state problems such as malformed files, unsafe paths, schema failures, or status/event coherence mismatches; and `4` for missing repository roots.
