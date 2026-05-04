# Kkachi compatibility matrix

This matrix records the local-only compatibility contract for `kkachi-agent-helper` releases. Keep it in sync with `docs/specs.md`, the embedded install manifest schema, and release notes.

## Version matrix

| Helper version | Supported project schema | Required `kkachi-agent-bridge` | Required `kkachi-hermes-skills` | Notes |
|---|---|---|---|---|
| `0.1.x` | `0.1` | Not checked by helper | Not checked by helper | MVP helper release line. `install skills/templates` enforces only `compat.required_helper`; bridge and skills requirements are reported as `not_checked` until authoritative package version sources are added. |
| `0.0.0-dev` | Development only | Not checked by helper | Not checked by helper | Local development builds intentionally do not satisfy semver install compatibility ranges such as `>=0.1.0`. Build with `make VERSION=0.1.0 build` or `make VERSION=0.1.0 release` for real install validation. |

## Compatibility rules

- Helper state schemas are versioned by the project-local `.kkachi/config.yaml` and embedded schema registry.
- Local install packages declare compatibility in `kkachi-install-manifest.json` under `compat.required_helper`, `compat.required_bridge`, and `compat.required_skills`.
- The current helper enforces `compat.required_helper` only. Supported range syntax is an exact `x.y.z` or `>=x.y.z`.
- `compat.required_bridge` and `compat.required_skills` are surfaced as `not_checked`; humans must verify those versions before a pilot release.
- Examples and release artifacts must stay local and secret-free. Do not publish tokens, API keys, bearer headers, bridge session secrets, or production repository paths in docs, bundles, or release notes.
