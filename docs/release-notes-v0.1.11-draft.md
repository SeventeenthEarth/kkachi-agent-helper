# kkachi-agent-helper v0.1.11 draft release notes

Release date: 2026-06-18
Commit: pending

## Summary

- Raises the helper source/build default version to `0.1.11` so local source builds and toolchain installs report `kkachi-agent-helper 0.1.11`.
- Carries MAREV-002 multi-agent-review evidence gate and schema support for KAS MAR outputs.

## Compatibility

| Component | Version/range | Verification |
|---|---|---|
| kkachi-agent-helper | `v0.1.11` target | `kkachi-agent-helper --version` after release/install |
| Project schema | `0.1` | `kkachi-agent-helper schema validate` / `project doctor --json` |
| kkachi-agent-skills | KAS `v0.1.5` MAR role-first consumer | KAS validates/produces MAR evidence; KAH mechanically checks canonical MAR artifacts |
| kkachi-agent-bridge | Not checked by helper | Manual; no KAB runtime authority is introduced |

## Changes

- `--version` / `version --json` report `0.1.11` for source-default builds.
- `Makefile` default `VERSION` is `0.1.11`; callers can still override `VERSION=<version>` for release artifact tests or explicit dev builds.
- MAREV-002 support includes `multi-agent-review/status.json`, `multi-agent-review-evidence` schema validation, `multi-agent-review` gate checks, and final-gate integration for MAR-required runs.

## Verification

```sh
git diff --check
go run . --version
go run . version --json
make test
go test ./... -count=1
```

## Known gaps / non-goals

- This release does not choose reviewers, adjudicate MAR findings, execute providers, mutate KAS/KAB configuration, or move any existing published tag.
- KAS remains the policy/producer layer; KAH validates only deterministic recorded evidence.
