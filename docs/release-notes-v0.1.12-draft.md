# kkachi-agent-helper v0.1.12 draft release notes

Release date: 2026-06-20
Commit: pending

## Summary

- Raises the helper source/build default version to `0.1.12` so local source builds and toolchain installs report `kkachi-agent-helper 0.1.12`.
- Carries STRICT deterministic workflow/final-gate validation support for KAS strict workflow execution evidence.

## Compatibility

| Component | Version/range | Verification |
|---|---|---|
| kkachi-agent-helper | `v0.1.12` target | `kkachi-agent-helper --version` after release/install |
| Project schema | `0.1` | `kkachi-agent-helper schema validate` / `project doctor --json` |
| kkachi-agent-skills | KAS `v0.1.6` STRICT/MARTL consumer | KAS produces STRICT/MARTL evidence; KAH mechanically validates deterministic run/gate artifacts |
| kkachi-agent-bridge | Not checked by helper | Manual; no KAB runtime authority is introduced |

## Changes

- `--version` / `version --json` report `0.1.12` for source-default builds.
- `Makefile` default `VERSION` is `0.1.12`; callers can still override `VERSION=<version>` for release artifact tests or explicit dev builds.
- STRICT support includes workflow-managed final gate enforcement, workflow transition order verification, and workflow phase projection validation evidence used by KAS `v0.1.6`.

## Verification

```sh
git diff --check
go run . --version
go run . version --json
make test
go test ./... -count=1
```

## Known gaps / non-goals

- This release does not choose backends, adjudicate reviewer findings, mutate KAS/KAB configuration, install profiles, or move any existing published tag.
- KAS remains the policy/producer layer; KAH validates only deterministic recorded evidence.
