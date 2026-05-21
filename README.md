# kkachi-agent-helper

`kkachi-agent-helper` is the deterministic local CLI helper for Kkachi project state, run artifacts, locks, schemas, events, diagnostics, and project bootstrap scaffolding. It stays local-first and scriptable: it does not choose a backend, plan work, review code, call network services, or store secrets.

The current implementation covers `corex-001` through `corex-005`, `runwf-001` through `runwf-004`, `gates-001` through `gates-005`, `packg-001` through `packg-004`, `pilot-001` through `pilot-005`, and `align-001` through `align-008`.

## Source of truth

- [Specs](docs/specs.md) — canonical behavior and schema contracts.
- [Roadmap](docs/roadmap.md) — delivery order and task scope.
- [Compatibility matrix](docs/compatibility.md) — helper/bridge/skills version contract.
- [Release notes template](docs/release-notes-template.md) — release note format and verification checklist.

## Quickstart

```sh
# Install the latest tagged release globally.
go install github.com/SeventeenthEarth/kkachi-agent-helper@latest

# Or install a specific release.
go install github.com/SeventeenthEarth/kkachi-agent-helper@v0.1.0

# Ensure Go's binary directory is on PATH if needed.
export PATH="$(go env GOPATH)/bin:$PATH"

# Initialize helper state in a git repository.
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
  --sot-policy existing_sot_basis
kkachi-agent-helper project doctor

# Create and prepare a local run. Copy the run_id from the JSON output.
kkachi-agent-helper run create \
  --title 'Pilot readiness dry run' \
  --work-path A_development_execution \
  --work-mode standard \
  --urgency normal \
  --sot-policy existing_sot_basis \
  --execution-mode production_write \
  --backend-evidence not_applicable \
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
go install github.com/SeventeenthEarth/kkachi-agent-helper@latest
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

- `go install github.com/SeventeenthEarth/kkachi-agent-helper@latest` installs the tagged helper to Go's binary directory (`GOBIN`, or `$(go env GOPATH)/bin`).
- `make build` writes `bin/kkachi-agent-helper` from the root installable package.
- `make PREFIX="$HOME/.local" install-local` installs the built helper to `$PREFIX/bin/kkachi-agent-helper`.
- `make VERSION=0.1.0 release` writes release artifacts to `dist/`:
  - `dist/kkachi-agent-helper_0.1.0_<goos>_<goarch>`
  - `dist/kkachi-agent-helper_0.1.0_<goos>_<goarch>.tar.gz`
  - `dist/SHA256SUMS`
- Tagged `go install ...@v0.1.0` builds derive the helper version from Go module build info; local `make build` still defaults to `0.0.0-dev`.

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
kkachi-agent-helper capabilities --json
kkachi-agent-helper help
kkachi-agent-helper --help
kkachi-agent-helper [--json] <command>
```

`capabilities --json` is the stable machine-readable command-surface report for KHS activation checks. It includes helper build info, the embedded project schema version, supported command groups, compatibility flags such as artifact mutation, phase-plan support, and approval records, and explicit omitted surfaces such as the removed `install` command.

Help is project-independent and exits `0`. Use `kkachi-agent-helper <command> --help`, supported subcommand topics such as `kkachi-agent-helper project init --help` and `kkachi-agent-helper run create --help`, or `kkachi-agent-helper help <command> [subcommand]` for required arguments, options, and JSON behavior. Implemented command groups have group help pages, including `schema`, `event`, `lock`, `phase-plan`, and `approval`. `--json` with help emits structured help JSON; compatibility automation should still prefer `capabilities --json`.

Project state:

```sh
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
  --sot-policy existing_sot_basis [--force] [--json]
kkachi-agent-helper project status [--json]
kkachi-agent-helper project doctor [--json]
```

Events:

```sh
kkachi-agent-helper event append <event_type> --run <run_id> --payload '<json-object>' [--json]
```

Runs:

```sh
kkachi-agent-helper run create --title <title> --work-path <A_development_execution|B_discovery_shaping> --work-mode <standard|light> --urgency <normal|urgent|critical> --sot-policy <existing_sot_basis|minimal_sot_before_code|full_sot_before_code> --execution-mode <production_write|adapter_qa|readiness_hardening|research|verification|docs_only> --commander <profile> [--backend-evidence <auto|required|not_applicable>] [--task-id <id>] [--redteam <profile>] [--json]
kkachi-agent-helper run list [--json]
kkachi-agent-helper run show <run_id> [--json]
kkachi-agent-helper run activate <run_id> [--json]
kkachi-agent-helper run close <run_id> [--json]
kkachi-agent-helper run abort <run_id> [--json]
```

`--backend-evidence` lets KHS declare backend evidence independently of `execution_mode`: `auto` requires backend evidence for `adapter_qa` and treats other modes as not applicable; `required` makes canonical bridge evidence mandatory even for `production_write`; `not_applicable` preserves direct/non-KAB runs.

Artifacts and gates:

```sh
kkachi-agent-helper artifact init <run_id> [--json]
kkachi-agent-helper artifact list <run_id> [--json]
kkachi-agent-helper artifact validate <run_id> [--gate intake] [--json]
kkachi-agent-helper artifact write <run_id> <artifact_path> --from <repo-relative-file> [--json]
kkachi-agent-helper artifact append <run_id> <artifact_path> --from <repo-relative-file> [--json]
kkachi-agent-helper artifact set-status <run_id> <artifact_path> --status <pending|complete|not_applicable> [--reason <text>] [--json]
kkachi-agent-helper gate check <run_id> <intake|sot|roadmap|plan|backend|implementation|review|verification|docs|final> [--json]
kkachi-agent-helper gate final <run_id> [--json]
```

Backend JSON evidence is schema-owned. Write `selected-cli.json` and `bridge-session-snapshot.json` with `artifact write`; do not run generic `artifact set-status ... --status complete` on those files. KAH rejects that with `artifact_status_not_applicable` so fields such as `selected-cli.json.status=supported|degraded` are not overwritten. Use `gate check backend` to validate backend evidence completion.

Phase plans:

```sh
kkachi-agent-helper phase-plan init <run_id> [--json]
kkachi-agent-helper phase-plan show <run_id> [--json]
kkachi-agent-helper phase-plan set <run_id> <phase-id> --status <status> [--evidence <path>] [--reason <text>] [--approval-required true|false] [--json]
kkachi-agent-helper phase-plan validate <run_id> [--final] [--json]
```

Approvals:

```sh
kkachi-agent-helper approval request <run_id> --phase <phase-id> --reason <reason> [--evidence <ref>] [--json]
kkachi-agent-helper approval record <run_id> --phase <phase-id> --decision <approved|rejected> --by <approver> --evidence <ref> [--reason <reason>] [--json]
kkachi-agent-helper approval show <run_id> [--phase <phase-id>] [--json]
```

Schemas and migrations:

```sh
kkachi-agent-helper schema validate <file> --schema <config|status|event|run-metadata|selected-cli|bridge-session-snapshot> [--json]
kkachi-agent-helper schema export [--schema <name>|--all] [--dry-run] [--json]
kkachi-agent-helper schema migrate --from <version> --to <version> [--dry-run] [--json]
```

Locks:

```sh
kkachi-agent-helper lock recover <active-run|project-write|all> --reason <text> [--run <run_id>] [--json]
```

Project bootstrap is handled by `project init`; KAH no longer provides a local `install` command for Hermes skills or templates. Hermes skill installation belongs to the Hermes native skill system.

Diagnostics:

```sh
kkachi-agent-helper diagnostics export [--run <run_id>] [--output <repo-relative-path>] [--json]
```

## KHS/KAH compatibility contract

KAH owns deterministic helper state after KHS or a user chooses to apply the Kkachi workflow. It validates declared state, artifacts, schemas, gates, events, locks, diagnostics, command surfaces, phase plans, backend evidence, and approval records. KHS owns workflow policy: whether to trigger the workflow, phase applicability and ordering, checklist normalization, backend-use decisions, and any commander reasoning. KAH must not promote itself into planner, backend selector, reviewer, Hermes skill installer, or workflow-policy owner.

KHS `main` may install KAH with `go install github.com/SeventeenthEarth/kkachi-agent-helper@latest`, but activation should fail closed against `kkachi-agent-helper capabilities --json` rather than relying on patch-version guesses. KHS release tags should publish the tested/recommended KAH versions used for reproducible releases, while still allowing `@latest` when the required command-surface capabilities are present.

Project bootstrap remains `project init` / `project init --force`: KAH creates or reconfigures helper-managed project state, schemas, overlays, and docs maps, but never installs Hermes/KHS skill content. Hermes skill installation belongs to Hermes native tooling.

Capability records follow the same boundary. KAH owns project-local `.kkachi/` persistence, evidence, and audit surfaces for accepted capability snapshots and reports, but it does not discover backend-native inventories and does not make a KHS semantic catalog callable. KAB owns raw backend-native discovery/verification; KHS owns workflow/prompt/semantic guidance; Blue command owns final active prompt selection.

## Operational notes

- `.kkachi/config.yaml`, `.kkachi/project-overlay.yaml`, `docs/kkachi-docs-map.yaml`, `.kkachi/status.json`, `.kkachi/events.jsonl`, `.kkachi/schemas/*.schema.json`, `.kkachi/capabilities/...`, and `.kkachi/runs/<run_id>/...` are the local helper state and evidence surfaces.
- Planned capability cache layout is `.kkachi/capabilities/current.json`, `snapshots/<id>.json`, `reports/<id>.json`, `fingerprints/<id>.json`, and `drift/<id>.json`, plus run-local `capability-snapshot.json` / `capability-check.md`. These are cache/evidence records, not backend-native inventory SOT.
- Mutating commands fail closed when `status.last_event_id` and the event log tail diverge.
- `project status`, `project doctor`, `artifact list`, and diagnostics stdout export are read-only.
- `gate check` records deterministic pass/fail/blocked results in run metadata, project status, events, and run-local gate reports.
- `artifact write`, `artifact append`, and markdown `artifact set-status` safely mutate canonical run artifacts with atomic writes and audit events; direct file reads remain compatible during migration. Schema-owned backend JSON artifacts reject generic lifecycle `set-status` and must be rewritten with valid JSON evidence.
- `phase-plan` stores and validates KHS-declared `phase-plan.yaml` state only; KHS owns phase applicability and workflow policy. `--approval-required true` makes final phase-plan validation require an approved KAH approval record for that phase.
- `approval` records KHS-declared approval requests and decisions as strict events; KAH does not decide whether approval is needed.
- `diagnostics export` redacts token-like values and exports only a selected support-safe artifact set.
- Canonical exit codes are `0` success, `1` internal failure, `2` usage/unsupported command state, `3` fail-closed state or validation problems, and `4` missing repository root.
