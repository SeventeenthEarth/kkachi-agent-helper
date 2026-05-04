# kkachi-agent-helper

`kkachi-agent-helper` is the deterministic local CLI helper for Kkachi project state, run artifacts, locks, schemas, events, diagnostics, and install scaffolding. It stays local-first and scriptable: it does not choose a backend, plan work, review code, call network services, or store secrets.

The current implementation covers `corex-001` through `corex-005`, `runwf-001` through `runwf-004`, `gates-001` through `gates-005`, `packg-001` through `packg-004`, and `pilot-004`.

## Source of truth

- [Specs](docs/specs.md) — canonical behavior and schema contracts.
- [Roadmap](docs/roadmap.md) — delivery order and task scope.
- [Compatibility matrix](docs/compatibility.md) — helper/bridge/skills version contract.
- [Release notes template](docs/release-notes-template.md) — release note format and verification checklist.

## Quickstart

```sh
# Build a semver helper for install compatibility checks.
make VERSION=0.1.0 build

# Put the helper on PATH for this shell session.
export PATH="$PWD/bin:$PATH"

# Initialize helper state in a git repository.
kkachi-agent-helper project init
kkachi-agent-helper project doctor

# Create and prepare a local run. Copy the run_id from the JSON output.
kkachi-agent-helper run create \
  --title 'Pilot readiness dry run' \
  --work-path A_development_execution \
  --work-mode standard \
  --urgency normal \
  --sot-policy existing_sot_basis \
  --execution-mode production_write \
  --commander Gongmyeong \
  --task-id pilot-003 \
  --json

run_id=<run_id-from-json-output>
kkachi-agent-helper artifact init "$run_id"
kkachi-agent-helper artifact list "$run_id" --json
kkachi-agent-helper diagnostics export --run "$run_id" --output diagnostics/helper-bundle.json
```

All examples are local and secret-free. Do not place tokens, bearer headers, API keys, passwords, production paths, or private bridge payloads in `.kkachi/` files, diagnostics bundles, release notes, or docs examples.

## Build, install, release, and verify

```sh
make build
make PREFIX="$HOME/.local" install-local
make VERSION=0.1.0 release
make test-prepare
make test-unit
make test-int
make test-e2e
make test
make check
```

- `make build` writes `bin/kkachi-agent-helper`.
- `make PREFIX="$HOME/.local" install-local` installs the built helper to `$PREFIX/bin/kkachi-agent-helper`.
- `make VERSION=0.1.0 release` writes release artifacts to `dist/`:
  - `dist/kkachi-agent-helper_0.1.0_<goos>_<goarch>`
  - `dist/kkachi-agent-helper_0.1.0_<goos>_<goarch>.tar.gz`
  - `dist/SHA256SUMS`
- For real `install skills/templates` compatibility checks, build with a semver helper version. The default `0.0.0-dev` intentionally does not satisfy ranges such as `>=0.1.0`.

Test lanes are intentionally split:

- `make test-prepare` runs formatting and static preparation checks.
- `make test-unit` runs package/unit-level Go tests.
- `make test-int` runs tagged integration tests.
- `make test-e2e` runs Go-native local black-box scenarios for project init, lock recovery, golden workspaces, diagnostics export, release packaging, and the pilot acceptance run.
- `make test` runs all lanes sequentially.

## Command reference

Global options:

```sh
kkachi-agent-helper --version
kkachi-agent-helper version --json
kkachi-agent-helper [--json] <command>
```

Project state:

```sh
kkachi-agent-helper project init
kkachi-agent-helper project status [--json]
kkachi-agent-helper project doctor [--json]
```

Events:

```sh
kkachi-agent-helper event append <event_type> --run <run_id> --payload '<json-object>' [--json]
```

Runs:

```sh
kkachi-agent-helper run create --title <title> --work-path <A_development_execution|B_discovery_shaping> --work-mode <standard|light> --urgency <normal|urgent|critical> --sot-policy <existing_sot_basis|minimal_sot_before_code|full_sot_before_code> --execution-mode <production_write|adapter_qa|readiness_hardening|research|verification|docs_only> --commander <profile> [--task-id <id>] [--redteam <profile>] [--json]
kkachi-agent-helper run list [--json]
kkachi-agent-helper run show <run_id> [--json]
kkachi-agent-helper run activate <run_id> [--json]
kkachi-agent-helper run close <run_id> [--json]
kkachi-agent-helper run abort <run_id> [--json]
```

Artifacts and gates:

```sh
kkachi-agent-helper artifact init <run_id> [--json]
kkachi-agent-helper artifact list <run_id> [--json]
kkachi-agent-helper artifact validate <run_id> [--gate intake] [--json]
kkachi-agent-helper gate check <run_id> <intake|sot|roadmap|plan|backend|implementation|review|verification|docs|final> [--json]
kkachi-agent-helper gate final <run_id> [--json]
```

Schemas and migrations:

```sh
kkachi-agent-helper schema validate <file> --schema <config|status|event|run-metadata|selected-cli|bridge-session-snapshot|install-manifest> [--json]
kkachi-agent-helper schema export [--schema <name>|--all] [--dry-run] [--json]
kkachi-agent-helper schema migrate --from <version> --to <version> [--dry-run] [--json]
```

Locks:

```sh
kkachi-agent-helper lock recover <active-run|project-write|all> --reason <text> [--run <run_id>] [--json]
```

Local skill/template install:

```sh
kkachi-agent-helper install skills --source <local-path> [--dry-run|--drift-check] [--json]
kkachi-agent-helper install templates --source <local-path> [--dry-run|--drift-check] [--json]
```

Diagnostics:

```sh
kkachi-agent-helper diagnostics export [--run <run_id>] [--output <repo-relative-path>] [--json]
```

## Operational notes

- `.kkachi/config.yaml`, `.kkachi/status.json`, `.kkachi/events.jsonl`, `.kkachi/schemas/*.schema.json`, and `.kkachi/runs/<run_id>/...` are the local helper state and evidence surfaces.
- Mutating commands fail closed when `status.last_event_id` and the event log tail diverge.
- `project status`, `project doctor`, `artifact list`, install dry-runs, install drift checks, and diagnostics stdout export are read-only.
- `gate check` records deterministic pass/fail/blocked results in run metadata, project status, events, and run-local gate reports.
- `diagnostics export` redacts token-like values and exports only a selected support-safe artifact set.
- Canonical exit codes are `0` success, `1` internal failure, `2` usage/unsupported command state, `3` fail-closed state or validation problems, and `4` missing repository root.
