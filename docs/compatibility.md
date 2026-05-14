# Kkachi compatibility matrix

This matrix records the local-only compatibility contract for `kkachi-agent-helper` releases. Keep it in sync with `docs/specs.md`, the project bootstrap contract, and release notes.

## Version matrix

| Helper version | Supported project schema | Required `kkachi-agent-bridge` | Required `kkachi-hermes-skills` | Notes |
|---|---|---|---|---|
| `0.1.x` | `0.1` | Not checked by helper | Hermes native install | MVP helper release line. KAH project bootstrap is handled by `project init`; Hermes skill content is installed outside KAH through Hermes native tooling. |
| `0.0.0-dev` | Development only | Not checked by helper | Hermes native install | Local development builds are for local bootstrap/testing only. Build with `make VERSION=0.1.0 build` or `make VERSION=0.1.0 release` for release validation. |

## Compatibility rules

- Helper state schemas are versioned by the project-local `.kkachi/config.yaml` and embedded schema registry.
- `project init` owns KAH project bootstrap files: `.kkachi/config.yaml`, `.kkachi/project-overlay.yaml`, `docs/kkachi-docs-map.yaml`, status, events, and schema copies.
- `project init --force` reconfigures bootstrap files without deleting status, events, runs, artifacts, or gate history.
- KAH does not install KHS/Hermes skill content. Use Hermes native skill installation for skill packages.
- KHS owns normalized `checklist.md` generation. KAH's `plan` gate requires completed `acceptance-criteria.md`, `plan.md`, and `checklist.md`, but it does not parse or require KAB planner-only sections such as `KHS Checklist Seed`.
- KHS declares backend evidence requirements through `run create --backend-evidence auto|required|not_applicable`; KAH stores the resolved `backend_evidence` value, initializes required backend artifacts only when declared required, and validates artifact shape/completion without choosing or overriding the backend.
- KHS may write canonical run artifacts through `artifact write`, `artifact append`, and `artifact set-status` to get KAH path-safety checks, atomic file replacement, and `artifact.written` audit events. Direct artifact file compatibility remains available during migration; this release intentionally rejects unmanaged supplemental artifact paths.
- KHS may store declared workflow state in `.kkachi/runs/<run_id>/phase-plan.yaml` through `phase-plan init/show/set/validate`. KAH validates declared structure, reasons, feedback bounds, final evidence links, and approval records for rows marked `approval_required: true` only; KHS owns phase applicability, ordering policy, and workflow decisions.
- KHS may record high-risk phase approvals with `approval request`, `approval record`, and `approval show`; KAH stores the declaration/decision and never decides when approval is required.
- KHS should prefer `kkachi-agent-helper capabilities --json` for `@latest` activation checks. The report exposes helper build info, embedded project schema version, supported command groups, compatibility flags including artifact mutation and phase-plan support, supported approval records, and explicitly omitted surfaces such as the removed `install` command.
- Examples and release artifacts must stay local and secret-free. Do not publish tokens, API keys, bearer headers, bridge session secrets, or production repository paths in docs, bundles, or release notes.

## Capabilities schema evolution

`capabilities_schema_version` versions the `capabilities --json` contract independently from helper release version and project state schema version.

- Additive changes that preserve existing field names, JSON types, and meanings may keep the same capabilities schema version. Examples: adding a new command group, subcommand, compatibility flag, or deprecated/omitted surface entry.
- Breaking changes must bump `capabilities_schema_version`. Examples: removing, renaming, or changing the JSON type of an existing field; changing the meaning of an existing compatibility flag; changing a previously supported command surface to unsupported without first exposing a deprecation/omission signal.
- Surfaces planned for removal should first appear in `deprecated_surfaces` or `omitted_surfaces` so KHS can fail closed or adjust activation checks before a breaking version.
