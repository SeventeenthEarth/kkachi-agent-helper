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
- Examples and release artifacts must stay local and secret-free. Do not publish tokens, API keys, bearer headers, bridge session secrets, or production repository paths in docs, bundles, or release notes.
