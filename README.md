# kkachi-agent-helper

`kkachi-agent-helper` is the deterministic local CLI helper for Kkachi project state, run artifacts, locks, schemas, events, diagnostics, and project bootstrap scaffolding. It stays local-first and scriptable: it does not choose a backend, plan work, review code, call network services, or store secrets.

The current implementation covers `corex-001` through `corex-005`, `runwf-001` through `runwf-004`, `gates-001` through `gates-005`, `packg-001` through `packg-004`, `pilot-001` through `pilot-005`, `align-001` through `align-008`, `graph-001` through `graph-012`, `token-001`/`token-002`, MAR evidence, policy-promotion evidence, and DESIGN-004/DESIGN-005 design-evidence schema/artifact/gate support.

## Source of truth

- [Specs](docs/specs.md) — canonical behavior and schema contracts, including `.kkachi-workflow.yaml` command and schema behavior.
- [Roadmap](docs/roadmap.md) — delivery order and task scope.
- [Compatibility matrix](docs/compatibility.md) — helper/bridge/skills version contract, including KHS/KAH graph activation and fallback rules.
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
  --commander responsible-approver \
  --redteam required-reviewer \
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
  --commander responsible-approver \
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
make install
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
- `make install` installs the local checkout to Go's binary directory (`GOBIN`, or `$(go env GOPATH)/bin`) with the Makefile version metadata.
- `make VERSION=0.1.14 release` writes release artifacts to `dist/`:
  - `dist/kkachi-agent-helper_0.1.14_<goos>_<goarch>`
  - `dist/kkachi-agent-helper_0.1.14_<goos>_<goarch>.tar.gz`
  - `dist/SHA256SUMS`
- Tagged `go install ...@v0.1.14` builds derive the helper version from Go module build info; local `make build` defaults to `0.1.14` for this release and can still be overridden with `VERSION=<version>`.

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

`capabilities --json` is the stable machine-readable command-surface report for KHS activation checks. It includes helper build info, the embedded project schema version, supported command groups including `project probe-toolchain`, compatibility flags such as artifact mutation, phase-plan support, approval records, read-only workflow graph support, workflow graph init/apply/export/diagnostics support, explicit no-direct-YAML-fallback graph support, configurable feedback-intake graph support, task-DAG schema validation, workflow instance state, workflow catalog diagnostics, workflow catalog proposal/apply support, workflow final-gate integration, KAS node-contract registry evidence, strict workflow transition ledger/order verification, token-economy evidence gate support, and explicit omitted surfaces such as the removed `install` command.

Help is project-independent and exits `0`. Use `kkachi-agent-helper <command> --help`, supported subcommand topics such as `kkachi-agent-helper project init --help` and `kkachi-agent-helper run create --help`, or `kkachi-agent-helper help <command> [subcommand]` for required arguments, options, and JSON behavior. Implemented command groups have group help pages, including `schema`, `event`, `lock`, `phase-plan`, `approval`, and `graph`. `--json` with help emits structured help JSON; compatibility automation should still prefer `capabilities --json`.

Project state:

```sh
kkachi-agent-helper project init \
  --project-name kkachi-agent-bridge \
  --stack go \
  --repo-path "$PWD" \
  --commander responsible-approver \
  --redteam required-reviewer \
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
kkachi-agent-helper project probe-toolchain --json [--project-root <path>]
```

`project probe-toolchain --json` emits stable `kah.toolchain_probe.v1` helper/project facts for KAS toolchain generation. It reports helper command/version/path, canonical project root, `.kkachi/` presence, initialized-state and workflow-graph presence facts, doctor status/reason codes, and `no_write.guaranteed=true` with `write_count=0`. It is read-only for initialized and uninitialized project directories and never creates `.kkachi/`, events, locks, graphs, schemas, runs, or `.kkachi/toolchain.yaml`.

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
kkachi-agent-helper gate check <run_id> <intake|sot|roadmap|plan|backend|implementation|review|verification|docs|final|token-economy> [--json]
kkachi-agent-helper gate final <run_id> [--json]
```

Schema-owned JSON evidence must be written with `artifact write`; do not run generic `artifact set-status ... --status complete` on `selected-cli.json`, `bridge-session-snapshot.json`, `token-economy-evidence.json`, `multi-agent-review/status.json`, `policy-promotion-evidence.json`, or `design-evidence.json`. KAH rejects that with `artifact_status_not_applicable` so semantic JSON fields are not overwritten.

`gate check <run_id> token-economy` is the deterministic token-001 evidence gate. It is active only for `task_id=token-001`; other tasks emit `not_applicable`. For token-001 runs, it validates the canonical `token-economy-evidence.json` artifact with schema version `token001.v1`, repository-confined evidence refs, optional `sha256:<64hex>` checksums, and marker checks. The gate emits only `pass`, `fail`, or `not_applicable`; invalid or missing evidence fails closed with exit code `3`.

DESIGN-004 provides canonical `design-evidence.json` bootstrap for `DESIGN-*` runs and an embedded/exported `design-evidence` schema (`design004.v1`). DESIGN-005 adds `gate check <run_id> design-evidence`, diagnostics `design_evidence` readback, and `gate final` integration when `design-evidence.json` is declared in the run manifest. KAH validates deterministic shape and evidence presence only: Teal applicability booleans, KAS waiver metadata fields, explicit non-UI skip reasons, required Teal plan/spec/fidelity/screenshot/verification evidence, evidence ref path shape, optional `sha256:<64hex>` checksums, and required `not_applicable` reasons. KAH does not classify UI, select Teal owners, judge design quality, score screenshots, approve waivers, activate KAB, or mutate runtime/provider/auth/profile/model settings.

DESIGN-006 cross-repo compatibility fixtures read KAS declarations for `kkachi_non_ui_skip`, `kkachi_teal_lane_non_ui_skip`, `sudal_ui_required`, and `doksuri_ui_required` and map them into KAH `design-evidence` schema/gate expectations when the KAS fixture is available through `KAS_DESIGN006_SCENARIOS` or a sibling checkout. These fixtures prove deterministic agreement only; they do not authorize installed binary readiness, downstream Sudal/Doksuri UI implementation, Teal owner assignment, or substitution of color review, MAR, backend evidence, or helper notes for required Teal verdicts.

Phase plans:

```sh
kkachi-agent-helper phase-plan init <run_id> [--json]
kkachi-agent-helper phase-plan show <run_id> [--json]
kkachi-agent-helper phase-plan set <run_id> <phase-id> --status <status> [--evidence <path>] [--reason <text>] [--approval-required true|false] [--json]
kkachi-agent-helper phase-plan validate <run_id> [--final] [--json]
```

`phase-plan validate` requires a valid `.kkachi-workflow.yaml` `feedback_intake` declaration before accepting feedback phase rows. Round 1 request/handle rows are required, optional continuation rounds 2 through 5 are allowed only when declared in the run-local phase plan, request/handle rows must be paired, and round 6+ fails closed. Without a valid graph feedback policy, KAH reports a validation failure rather than falling back to legacy `1..3` bounds. With `--final`, declared feedback rows follow the same terminal-state and completed-evidence checks as the required phase spine.

Approvals:

```sh
kkachi-agent-helper approval request <run_id> --phase <phase-id> --reason <reason> [--evidence <ref>] [--json]
kkachi-agent-helper approval record <run_id> --phase <phase-id> --decision <approved|rejected> --by <approver> --evidence <ref> [--reason <reason>] [--json]
kkachi-agent-helper approval show <run_id> [--phase <phase-id>] [--json]
```

Workflow graph:

```sh
kkachi-agent-helper graph init --from-template <khs-default|repo-relative-template.yaml> [--output .kkachi-workflow.yaml] [--json]
kkachi-agent-helper graph validate [--file .kkachi-workflow.yaml] [--json]
kkachi-agent-helper graph explain [--file .kkachi-workflow.yaml] [--json]
kkachi-agent-helper graph diff --from <repo-relative-graph> --to <repo-relative-graph> [--semantic] [--json]
kkachi-agent-helper graph propose --candidate-file <repo-relative-candidate-graph> --reason <text> [--json]
kkachi-agent-helper graph propose --patch <repo-relative-candidate-graph> --reason <text> [--json]  # legacy alias
kkachi-agent-helper graph apply --proposal <proposal-id> --approval <evidence-ref> [--json]
kkachi-agent-helper graph export --format mermaid|plantuml [--output <path>] [--json]
```

`graph init` writes the initial `.kkachi-workflow.yaml` only when no graph exists, using built-in `khs-default` or an explicit repository-relative YAML template path. `graph validate`, `graph explain`, `graph diff`, and `graph export` do not write graph state. `graph validate` and `graph explain` also project a top-level `feedback_intake` declaration when present, using `policy: "EXTERNAL_FEEDBACK_INTAKE"`, `schema_version: "external-feedback-intake/v1"`, `min_rounds: 1`, `max_rounds: 5`, `required_rounds: [1]`, and `optional_rounds: [2,3,4,5]`; stale `max3`/`1..3`, missing, duplicate, unsupported, conflicting, or round 6+ declarations fail closed. `phase-plan validate` consumes a valid project graph feedback policy for run-local feedback round bounds. Stale-only `max3`/`1..3` graph state can be migrated only by recording an explicit `graph propose --candidate-file` evidence record whose valid candidate declares canonical `1..5` bounds and changes no other graph semantics, then applying it with `graph apply --approval <evidence-ref>`. `graph propose` records `.kkachi/graph/proposals/gprop-*.json` evidence and a `graph.proposal_recorded` event for a complete candidate workflow graph supplied through `--candidate-file`; legacy `--patch` remains accepted for compatibility but is not a partial patch DSL. Proposal recording does not apply changes to `.kkachi-workflow.yaml`. `approval_required=false` in proposal output means the semantic diff did not trigger graph approval policy; `graph apply` still requires `--approval <evidence-ref>` so the apply event records an explicit approval or audit evidence reference. `graph apply` verifies proposal/base/candidate checksums fail-closed, writes `.kkachi-workflow.yaml` atomically, stamps `last_applied_event_id`, and appends `graph.applied`; KAH records the supplied evidence reference but does not decide approval policy. `graph export` renders Mermaid or PlantUML generated artifacts with `authoritative: false` and the source checksum; exports never become workflow graph source of truth.

Task-DAG workflows and catalog promotion:

```sh
kkachi-agent-helper workflow validate --file <workflow.yaml> [--json]
kkachi-agent-helper workflow explain --file <workflow.yaml> [--json]
kkachi-agent-helper workflow catalog validate --file <catalog.yaml> [--node-contract-registry <registry.yaml>] [--json]
kkachi-agent-helper workflow catalog explain --file <catalog.yaml> [--workflow-id <id>] [--node-contract-registry <registry.yaml>] [--json]
kkachi-agent-helper workflow catalog propose --packet <kas-promote-packet.json> --reason <text> [--json]
kkachi-agent-helper workflow catalog apply --proposal <proposal-id> --approval <evidence-ref> --proposal-hash sha256:<64hex> [--json]
```

`workflow catalog propose` records KAH-owned proposal evidence under `.kkachi/workflow-catalog/proposals/` from a KAS WFLOW-009 packet. It validates the supplied workflow DAG, catalog, and node-contract registry content, records target paths, base and candidate checksums, changed paths, diagnostics, conflicts, no-write posture, source packet reference, and the canonical proposal hash, but does not write target workflow files. `workflow catalog apply` requires both explicit approval evidence and the exact `--proposal-hash`; when the source KAS packet carries an approval hash, the approval evidence must bind to that hash. Apply rechecks proposal identity, source hash binding, base drift, candidate checksums, and candidate validity before backup/write/audit, then writes only repository-confined project-local workflow catalog targets and appends `workflow_catalog.applied`. KAH validates, proposes, applies, backs up, and audits only; KAS remains responsible for promotion policy, workflow semantic preference, selector behavior, trigger policy, agent assignment, backend execution, and approval semantics. Installed binaries that do not advertise `workflow_catalog_proposal_apply=true` in `capabilities --json` must be treated as lacking this support.

Schemas and migrations:

```sh
kkachi-agent-helper schema validate <file> --schema <config|status|event|run-metadata|selected-cli|bridge-session-snapshot|token-economy-evidence|multi-agent-review-evidence|policy-promotion-evidence|design-evidence> [--json]
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

Diagnostics bundles include top-level `graph_compatibility` evidence with graph support status, `.kkachi-workflow.yaml` validation state, nested `feedback_intake` status/effective bounds/repair issues, forbidden fallback sources, and `no_direct_yaml_fallback: true`. Missing or invalid graph state is reported inside the support-safe bundle so KHS can fail closed when graph support is required; diagnostics export itself still succeeds unless the export path or run selection is invalid.

Diagnostics bundles for runs with workflow instances include `workflow_transition_order` evidence. The result reports whether `.kkachi/runs/<run_id>/workflow-instance.json` is coherently backed by `.kkachi/events.jsonl` transition events and uses bounded diagnostics for malformed payloads, unknown nodes, workflow mismatches, revision gaps/stale revisions, dependency-order violations, complete-without-start, succeeded-node restart, and instance/event mismatches.

## KHS/KAH compatibility contract

KAH owns deterministic helper state after KHS or a user chooses to apply the Kkachi workflow. It validates declared state, artifacts, schemas, gates, events, locks, diagnostics, command surfaces, phase plans, backend evidence, and approval records. KHS owns workflow policy: whether to trigger the workflow, phase applicability and ordering, checklist normalization, backend-use decisions, and any commander reasoning. KAH must not promote itself into planner, backend selector, reviewer, Hermes skill installer, or workflow-policy owner.

KHS `main` may install KAH with `go install github.com/SeventeenthEarth/kkachi-agent-helper@latest`, but activation should fail closed against the effective binary's `kkachi-agent-helper capabilities --json` rather than relying on patch-version guesses or source checkout evidence. KHS release tags should publish the tested/recommended KAH versions used for reproducible releases, while still allowing `@latest` when the required command-surface capabilities are present in the binary that will actually run.

Project bootstrap remains `project init` / `project init --force`: KAH creates or reconfigures helper-managed project state, schemas, overlays, and docs maps, but never installs Hermes/KHS skill content. Hermes skill installation belongs to Hermes native tooling.

Workflow graph support is implemented for `graph init`, `graph validate`, `graph explain`, `graph diff`, `graph propose`, `graph apply`, `graph export`, graph compatibility diagnostics, configurable feedback-intake activation evidence, graph-policy-driven phase-plan feedback round validation, and opt-in declarative workflow checks for CodeGraph evidence plus repository `.gitignore` hygiene. Workflow catalog proposal/apply support is implemented for explicit KAS WFLOW-009 promotion packets only and is activation-gated by the effective binary's `workflow_catalog_proposal_apply=true` capability flag. `.kkachi-workflow.yaml` is the project-level graph file for initialized/validated/explained/diffed/applied graph state; graph proposals are helper-managed evidence until approval-gated apply records `graph.applied`; graph exports are generated visualization artifacts only. KHS chooses workflow policy/templates/candidate graph content and any approval semantics; KAH validates declared graph/catalog data, stores proposal/apply evidence, records the supplied approval/audit reference, and validates run-local feedback rows against a valid graph feedback policy. KHS must require both `workflow_graph_configurable_feedback_intake=true` in `capabilities --json` and `graph_compatibility.feedback_intake.status=="pass"` in diagnostics before activating configurable feedback intake. KHS must fail closed instead of silently editing YAML or using generated diagrams, `.kkachi/config.yaml`, stale `.kkachi/` state, KHS defaults, or Kkachi v2 `.kkachi/config/workflows/` as fallback graph authority.

Capability records follow the same boundary. KAH owns project-local `.kkachi/` persistence, evidence, and audit surfaces for accepted capability snapshots and reports, but it does not discover backend-native inventories and does not make a KHS semantic catalog callable. KAB owns raw backend-native discovery/verification; KHS owns workflow/prompt/semantic guidance; the responsible operator owns final active prompt selection.

## Operational notes

- `.kkachi-workflow.yaml` is the project-level workflow graph file initialized, validated, explained, and diffed by the `graph` command surface; `graph propose --candidate-file` stores complete-candidate proposal evidence without applying graph changes, while legacy `--patch` remains a compatibility alias.
- `.kkachi/config.yaml`, `.kkachi/project-overlay.yaml`, `docs/kkachi-docs-map.yaml`, `.kkachi/status.json`, `.kkachi/events.jsonl`, `.kkachi/schemas/*.schema.json`, `.kkachi/capabilities/...`, and `.kkachi/runs/<run_id>/...` are the local helper state and evidence surfaces.
- Planned capability cache layout is `.kkachi/capabilities/current.json`, `snapshots/<id>.json`, `reports/<id>.json`, `fingerprints/<id>.json`, and `drift/<id>.json`, plus run-local `capability-snapshot.json` / `capability-check.md`. These are cache/evidence records, not backend-native inventory SOT.
- Mutating commands fail closed when `status.last_event_id` and the event log tail diverge.
- `project status`, `project doctor`, `project probe-toolchain`, `artifact list`, and diagnostics stdout export are read-only.
- `gate check` records deterministic pass/fail/blocked results in run metadata, project status, events, and run-local gate reports.
- `artifact write`, `artifact append`, and markdown `artifact set-status` safely mutate canonical run artifacts with atomic writes and audit events; direct file reads remain compatible during migration. Schema-owned backend JSON artifacts reject generic lifecycle `set-status` and must be rewritten with valid JSON evidence.
- `phase-plan` stores and validates KHS-declared `phase-plan.yaml` state only; KHS owns phase applicability and workflow policy. `--approval-required true` makes final phase-plan validation require an approved KAH approval record for that phase.
- `approval` records KHS-declared approval requests and decisions as strict events; KAH does not decide whether approval is needed.
- `diagnostics export` redacts token-like values, exports only a selected support-safe artifact set, and includes `graph_compatibility` so KHS can fail closed without directly parsing fallback YAML.
- Canonical exit codes are `0` success, `1` internal failure, `2` usage/unsupported command state, `3` fail-closed state or validation problems, and `4` missing repository root.
