# KAH graph workflow sync diagnostics and repair SOT

Date: 2026-06-11
Owner: KAH deterministic helper layer
Confirming role: Responsible approver / governance evidence record pending
Status: planning SOT for KAH v0.1.9 graph workflow sync support; not implemented behavior until roadmap tasks pass evidence and release gates
Authority level: KAH-side planning authority for graph compatibility diagnostics and approval-gated repair substrate
Scope: `kkachi-agent-helper` docs, diagnostics, graph command JSON contracts, deterministic repair/apply safety; no KAS policy selection, KAB runtime behavior, Hermes profile mutation, auth/token/provider/gateway/model change, or network dependency
Related docs: `docs/specs.md`, `docs/compatibility.md`, `docs/roadmap.md`, KAS `docs/sot/graph-workflow-sync-compatibility.md`, KAS `docs/sot/workflow-graph-integration.md`
Evidence/source paths:
- Master direction in 17번째 지구 Discord thread on 2026-06-11: KAS should know its supported KAH version; KAS/KAH should periodically confirm `.kkachi-workflow.yaml` supportability; old KAH should trigger update guidance; latest KAH with old/broken graph should repair through supported KAH mechanics; user-custom graphs should be judged by supportability, not by default-template equality.

## Decision summary

KAH v0.1.9 will provide the deterministic substrate needed for KAS graph workflow sync without becoming the workflow-policy owner. KAH must expose stable graph diagnostics, reason codes, source-precedence evidence, and approval-gated repair/apply mechanics so KAS can decide whether to recommend a KAH update, accept a user-custom graph, propose a graph repair, or fail closed.

KAH owns only machine-checkable facts: graph parse/schema validity, source precedence, audit/checksum consistency, feedback-intake validity, graph command support, proposal/apply records, backups, events, and diagnostics. KAH must not decide whether a project should use the KAS default template, whether a user-custom graph is semantically preferred, which KAS version should be installed, which backend should execute work, or whether policy changes are approved.

## Release target

- Target helper release: `kkachi-agent-helper v0.1.9`.
- KAS target consumer: `kkachi-agent-skills v0.1.2`.
- KAS v0.1.2 compatibility metadata is expected to set KAH `min_required`, `recommended`, and `tested` to `0.1.9` for graph workflow sync.
- KAH v0.1.9 must be released before KAS v0.1.2 can claim tested graph workflow sync support.

## Required KAH behavior

### 1. Stable compatibility diagnostics

KAH graph diagnostics, `graph validate --json`, and `graph explain --json` must expose stable machine-readable fields sufficient for KAS to classify project graph state. The exact JSON shape is implementation-owned by KAH, but it must preserve these concepts as stable contract fields or reason codes:

- effective KAH version and graph capability support;
- graph file path and presence state;
- graph schema version;
- validation status;
- source authority status;
- checksum/audit evidence status;
- feedback-intake status;
- stale/broken/manual-edit/conflict reason codes;
- whether KAH can safely create a proposal for a complete candidate graph;
- whether apply is blocked on explicit approval evidence;
- forbidden fallback sources detected, including `.kkachi/config.yaml`, generated diagrams, stale `.kkachi/` runtime state, KAS defaults, and Kkachi v2 `.kkachi/config/workflows/`.

Diagnostics should remain deterministic and local-only. Invalid graph input should still return structured diagnostics when possible rather than only an opaque parse error.

### 2. Reason-code vocabulary

KAH should provide stable reason codes for KAS consumption. The initial vocabulary should cover at least:

- `graph_missing`
- `graph_valid`
- `graph_invalid_schema`
- `graph_parse_error`
- `graph_source_precedence_violation`
- `graph_checksum_mismatch`
- `graph_audit_missing`
- `graph_feedback_intake_missing`
- `graph_feedback_intake_stale_bounds`
- `graph_feedback_intake_invalid`
- `graph_manual_edit_unverified`
- `graph_conflicts_with_phase_plan`
- `graph_apply_requires_approval`
- `graph_repair_candidate_supported`
- `graph_repair_candidate_unsupported`
- `forbidden_fallback_source_detected`

KAH can add codes when needed, but KAS-facing removals or meaning changes require compatibility review.

### 3. Approval-gated repair/apply substrate

KAH must support stale/broken graph repair only through proposal/apply evidence. It must not introduce a direct YAML edit fallback or partial patch DSL.

Required repair posture:

1. A repair proposal references a complete candidate graph.
2. The proposal records base graph state, even when the base is missing, invalid, stale, or checksum-mismatched.
3. The proposal records semantic diff or equivalent deterministic comparison where possible.
4. Apply requires an explicit `--approval <evidence-ref>` or equivalent approval evidence.
5. Apply rechecks current graph state against the proposal base and fails closed on drift.
6. Apply writes atomically, records checksum/version/audit events, and preserves a recovery/backup reference when replacing a stale or broken graph.
7. Apply validates the candidate graph using current KAH schema and source-precedence rules.
8. Apply does not decide KAS policy or approval sufficiency beyond requiring a non-empty evidence reference and recording it.

### 4. User-custom graph boundary

KAH must not classify a valid custom `.kkachi-workflow.yaml` as wrong merely because it differs from a KAS default template. KAH should report deterministic support facts. KAS decides whether the custom graph is inside the supported KAS/KAH envelope.

### 5. Periodic check support

KAH should keep diagnostics safe for periodic read-only checks from KAS, CI, cron, or no-agent runners. Periodic checks may validate, explain, and export diagnostics. They must not apply graph changes without explicit approval evidence.

## Non-goals

- No KAS compatibility registry implementation in KAH.
- No KAS default-template policy decision in KAH.
- No KAB graph policy authority.
- No Hermes profile, skill install, provider, model, gateway, token, or auth mutation.
- No network dependency for graph diagnostics or repair.
- No automatic apply from periodic checks.
- No merge/fallback with Kkachi v2 `.kkachi/config/workflows/`.

## Roadmap slice mapping

This SOT maps to two KAH PR-candidate tasks:

1. `grsync-001` — graph compatibility diagnostics and reason-code hardening.
2. `grsync-002` — approval-gated stale/broken graph repair substrate.

Both tasks must update `docs/specs.md`, `docs/compatibility.md`, `docs/README.md`, and release notes when implementation lands. This SOT alone is not an implementation claim.

## Acceptance gates before KAH v0.1.9 release

- KAH exposes deterministic graph compatibility diagnostics and reason codes.
- KAH validates/explains valid, custom-supported, stale, broken, missing, manual-edit, and checksum-mismatch graph fixtures.
- KAH proposal/apply repair path is approval-gated and drift-safe.
- Direct `.kkachi-workflow.yaml` edit fallback remains forbidden.
- `make test` and relevant graph/diagnostics tests pass.
- Docs/specs/compatibility/roadmap/release notes match the implemented behavior.
- Red/Orange/Gray review gates are resolved if the active Kkachi run requires them.
