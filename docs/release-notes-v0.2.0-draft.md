# kkachi-agent-helper v0.2.0 draft release notes

Release date: pending
Commit: pending

## Summary

- Raises the helper source/build default version to `0.2.0` so local source builds and toolchain installs report `kkachi-agent-helper 0.2.0`.
- Carries the GAJAE pilot-prep helper surface as the next bounded release target for KAS consumers.

## Compatibility

| Component | Version/range | Verification |
|---|---|---|
| kkachi-agent-helper | `v0.2.0` target | `kkachi-agent-helper --version` after release/install |
| Project schema | `0.1` | `kkachi-agent-helper schema validate` / `project doctor --json` |
| kkachi-agent-skills | KAS `v0.2.0` pilot-prep consumer | KAS owns `.kkachi/toolchain.yaml`; KAH exposes deterministic project/run/toolchain facts |
| kkachi-agent-bridge | Not checked by helper | Manual; no KAB runtime authority is introduced |

## Changes

- `--version` / `version --json` report `0.2.0` for source-default builds.
- `Makefile` default `VERSION` is `0.2.0`; callers can still override `VERSION=<version>` for release artifact tests or explicit dev builds.
- TOLMR support boundary remains read-only in KAH: KAS owns toolchain metadata writes, stage policy, MAR/provider policy, and legacy import behavior.

## Verification

```sh
git diff --check
go run . --version
go run . version --json
make test
```

## Known gaps / non-goals

- This release does not create or push Git tags by itself.
- This release does not choose backends, adjudicate reviewer findings, mutate KAS/KAB configuration, install profiles, or move any existing published tag.
- KAS remains the policy/producer layer; KAH validates and reports only deterministic recorded evidence.
