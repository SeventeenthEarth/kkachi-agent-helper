# kkachi-agent-helper

`kkachi-agent-helper` is the deterministic local CLI helper for Kkachi project state, run artifacts, locks, schemas, events, and install scaffolding.

The current implementation covers the `corex-001` through `corex-004` foundations: repository layout, Go toolchain, command shell, version output, repo-root discovery, safe repository-relative path handling, symlink escape rejection, canonical exit codes, structured human/JSON errors, verification commands, safe `.kkachi/` project initialization, atomic state writes, append-only event handling, and `last_event_id` coherence checks.

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
- `make test-e2e` runs local end-to-end scenarios. For `corex-004`, it builds the helper, initializes a temporary project, verifies generated `.kkachi/` state, appends an event, checks `last_event_id` coherence, and checks overwrite refusal.
- `make test` runs `test-prepare`, `test-unit`, `test-int`, and `test-e2e` sequentially.

## CLI examples

```sh
kkachi-agent-helper --version
kkachi-agent-helper version --json
kkachi-agent-helper project init
kkachi-agent-helper project init --json
kkachi-agent-helper event append artifact.written --run run-abc --payload '{"path":"impl-log.md"}' --json
```

For `corex-004`, `project init` creates `.kkachi/config.yaml`, `.kkachi/status.json`, `.kkachi/events.jsonl`, and the initial `.kkachi/schemas/*.schema.json` files using atomic new-file writes. It allows existing empty helper directories but refuses to overwrite any helper-managed file.

`event append` appends one JSONL event, allocates the next `evt-000001`-style id, and atomically advances `status.last_event_id`. It fails closed if the status file and event log tail are incoherent. CLI payloads are capped at 256 KiB; larger evidence should be written to artifacts and referenced from the event payload.

Other command groups such as `run`, `artifact`, `gate`, `schema`, and `install`, plus later `project` subcommands, remain reserved placeholders. Repo-bound command groups first require a discoverable Git or `.kkachi` repository root, then return deterministic `not_implemented` errors until their roadmap tasks add real behavior.

Error output is stable for both humans and scripts:

- Human errors include `error`, optional structured fields, `exit_code`, and `hint`.
- JSON errors are emitted as `{"error": ...}` without decorative text when `--json` is present.
- Canonical exit codes are `0` for success, `1` for internal failures, `2` for usage or unsupported command state, `3` for path-safety failures, and `4` for missing repository roots.
