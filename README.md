# kkachi-agent-helper

`kkachi-agent-helper` is the deterministic local CLI helper for Kkachi project state, run artifacts, locks, schemas, events, and install scaffolding.

The current implementation is the `corex-001` foundation: repository layout, Go toolchain, command shell, version output, JSON error contract, and verification commands. It intentionally does not create or mutate `.kkachi/` project state yet.

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
- `make test-e2e` is reserved for real external-system scenarios. `corex-001` has no registered external-system scenario yet, so the lane reports that explicitly.
- `make test` runs `test-prepare`, `test-unit`, `test-int`, and `test-e2e` sequentially.

## CLI examples

```sh
kkachi-agent-helper --version
kkachi-agent-helper version --json
kkachi-agent-helper project init
```

For `corex-001`, command groups such as `project`, `run`, `artifact`, `gate`, `event`, `schema`, and `install` are reserved placeholders. They return deterministic `not_implemented` errors until their roadmap tasks add real behavior.
