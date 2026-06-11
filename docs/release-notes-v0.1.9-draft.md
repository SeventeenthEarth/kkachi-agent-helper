# kkachi-agent-helper v0.1.9 draft release notes

Release date: pending
Commit: pending

## Summary

- Adds KAH graph workflow sync diagnostics and stable reason-code hardening for KAS v0.1.2 consumption.

## Compatibility

| Component | Version/range | Verification |
|---|---|---|
| kkachi-agent-helper | `v0.1.9` target | `kkachi-agent-helper --version` after release/install |
| Project schema | `0.1` | `kkachi-agent-helper project doctor --json` |
| kkachi-agent-bridge | Not checked by helper | Manual; no KAB graph authority is introduced |
| kkachi-hermes-skills | KAS v0.1.2 planned consumer | KAS decides support envelope and update guidance |

## Changes

- `graph validate --json` now emits stable `reason_codes` for graph compatibility classification.
- `graph explain --json` now emits top-level `reason_codes` and preserves validation reason codes in `validation_summary.reason_codes`.
- `diagnostics export` `graph_compatibility` now emits top-level reason codes, nested validation reason codes, nested feedback-intake reason codes, and forbidden fallback source `reason_code` fields.
- Initial reason-code vocabulary reserves graph missing/valid/invalid-schema/parse-error/source-precedence/checksum-audit/feedback-intake/manual-edit/phase-conflict/approval-required/repairability/forbidden-fallback facts; this slice emits producer-backed codes for the implemented validation and diagnostics states.
- KAH still reports deterministic support facts only; KAS remains the policy owner for supported envelope and update guidance.

## Verification

```sh
go test -count=1 ./...
go run . graph validate --file .kkachi-workflow.yaml --json
go run . graph explain --file .kkachi-workflow.yaml --json
go run . diagnostics export --json
```

Maintainer evidence for the draft was captured with `HOME=/Users/draccoon` because local KAH/KAS tooling in this workspace expects the real user home.

## Known gaps / non-goals

- `grsync-002` approval-gated stale/broken complete-candidate graph repair substrate is not implemented by this slice.
- No automatic KAH update, automatic graph apply, KAS compatibility registry, KAS doctor/repair CLI behavior, KAB graph authority, `kah graph` alias behavior, direct `.kkachi-workflow.yaml` edit fallback, or Hermes profile/provider/gateway/auth/token/model mutation is introduced.
