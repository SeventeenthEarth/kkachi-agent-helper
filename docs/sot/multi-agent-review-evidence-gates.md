# Multi-Agent Review evidence gates

Date: 2026-06-17
Owner: KAH deterministic helper layer
Status: planning SOT; implementation pending
Authority level: KAH-side planning authority for MAR evidence capture and future deterministic gates; not implemented helper behavior by itself
Upstream KAS SOT: KAS `docs/sot/multi-agent-review-policy.md`
Source SOT: `/Users/draccoon/Workspace/Hermes/17thHermes/40_outputs/projects/kkachi/2026-06-16-kkachi-multi-agent-review-mar-sot.md`
Scope: MAR evidence artifact layout, deterministic validation expectations, final-gate posture, and KAH/KAS/KAB boundaries

## Purpose

This document records the KAH-side planning contract for Kkachi Multi-Agent Review (MAR). MAR itself is a KAS-owned review policy and script surface. KAH's role is deterministic evidence preservation and gate validation only.

This SOT does not implement helper behavior. `MAREV-001` records this planning contract. Actual KAH code, schemas, artifact validators, diagnostics, or final-gate behavior must be implemented and verified under `MAREV-002` or later.

## Task split

| Task | Scope | Completion claim allowed |
|---|---|---|
| `MAREV-001` | KAH planning SOT, docs index, roadmap row, and KAS/KAH boundary record. | Planning authority only; no helper behavior. |
| `MAREV-002` | Deterministic KAH implementation for MAR artifact/gate/schema support if selected, including provider-attempt, retry, alternate-provider approval, and waiver evidence checks. | Implemented helper behavior only after code, tests, docs, review, and final gate evidence. |
| `MAREV-003` | Optional diagnostics/release integration if MAREV-002 needs release-facing compatibility evidence. | Release/compatibility behavior only after implemented evidence. |

## Boundary

KAH must not:

- choose reviewer models;
- generate review prompts;
- run reviewer CLIs;
- parse findings with model reasoning;
- adjudicate findings;
- decide whether Red or premium reviewers are semantically needed;
- mutate source code, tests, provider config, auth, tokens, secrets, gateway state, Hermes profiles, or KAB runtime state;
- turn degraded reviewer coverage into warning-only pass.

KAH may, once implemented, deterministically validate whether declared MAR artifacts exist, are structurally valid, are safe repository-local paths, have expected status vocabulary, and satisfy final-gate freshness/completeness rules.

## Planned artifact layout

Recommended run-local layout:

```text
.kkachi/runs/<run_id>/multi-agent-review/
  request.yaml
  doctor.json
  input-bundle.md
  diff.patch
  prompt/
    zcode-glm-5-2-sot-risk.md
    kimi-k2-7-trace.md
    antigravity-gemini-architecture.md
  raw/
    zcode-glm-5-2.out.md
    kimi-k2-7.out.md
    antigravity-gemini.out.md
    zcode-glm-5-2.err.txt
    kimi-k2-7.err.txt
    antigravity-gemini.err.txt
  parsed/
    zcode-glm-5-2.findings.json
    kimi-k2-7.findings.json
    antigravity-gemini.findings.json
  attempts/
    provider-attempts.json
    retry-attempts.json          # only when retries are attempted
    alternate-approvals.yaml     # only when an alternate reviewer is approved
    waiver-evidence.yaml         # only when 주군 waives failed coverage
  status.json
  provider-report.md
  merge-pack.md
  blue-disposition.md
  red-adjudication.md        # only when triggered
  premium-request.md         # only when needed
  premium-result/            # only after approval
```

KAH implementation may refine exact filenames only through a versioned schema and docs update. Until MAREV implementation exists, this layout is planning guidance consumed by KAS artifacts, not a helper-enforced manifest.

## Planned deterministic checks

A future MAREV implementation should check only mechanical facts, such as:

- `request.yaml` exists and names the run id, task id, repository root, requested reviewer set, scope, and read-only posture;
- `doctor.json` records provider availability/validation status without secrets;
- `provider-attempts.json` records an attempted entry for every required default reviewer, including reviewer id, provider lane, model label, attempt status, exit category, deterministic reason code when failed, redacted raw/stderr path references, and mutation-guard reference;
- failed default reviewer coverage is not converted to clean `PASS` unless a linked retry succeeds, an explicitly approved alternate-provider attempt succeeds, or `waiver-evidence.yaml` records 주군's waiver with waived reviewer id, reason, accepted residual risk, timestamp, and approval reference;
- `retry-attempts.json`, when present, links every retry to the original failed attempt and records bounded retry count, reason, result, and evidence paths;
- `alternate-approvals.yaml`, when present, identifies the failed default reviewer replaced, approved alternate provider/model, approval reference, reason, and successful attempt evidence; KAH validates only that the approval/evidence exists, not whether the alternate provider was semantically preferable;
- `waiver-evidence.yaml`, when present, identifies the waived reviewer failure, 주군 approval reference, accepted residual risk, and any required Blue/Red disposition references;
- `status.json` uses an accepted MAR status value: `PASS`, `PASS_WITH_FINDINGS`, `REQUEST_CHANGES`, `BLOCKED`, `DEGRADED`, or `FAILED`;
- default reviewer coverage semantics are represented explicitly, including failed/unavailable providers;
- all raw/parsed/prompt/attempt paths referenced by status files are repository-confined and present when claimed complete;
- degraded coverage includes a Blue reason;
- insufficient coverage or triggered risk categories require `red-adjudication.md` before final completion;
- premium reviewer artifacts exist only with a `premium-request.md` / approval reference when required by policy;
- before/after mutation guard evidence is present when the KAS script claims read-only review;
- final Blue disposition exists before a final gate accepts MAR as satisfied.

KAH must not judge whether a finding is correct, choose whether a failed provider should be retried, select an alternate provider, or approve a waiver. It can only check that Blue, Red, or 주군 recorded the required disposition/approval evidence for findings and provider-coverage decisions declared by the KAS MAR package.

## Gate semantics target

If implemented, MAR-aware gates should use fail-closed statuses:

| Condition | Gate posture |
|---|---|
| No MAR required for the active task | `not_applicable` with reason |
| MAR required but request/status artifacts missing | `fail` |
| Required default reviewer missing an attempted entry | `fail` |
| Failed default reviewer lacks deterministic reason code or redacted evidence reference | `fail` |
| Failed default reviewer has no successful linked retry, approved alternate-provider success, or explicit 주군 waiver evidence | `fail` or `blocked`, not pass |
| Alternate provider used without explicit approval evidence | `fail` |
| Waiver lacks waived reviewer, approval reference, or accepted residual-risk record | `fail` |
| All default reviewers failed | `fail` or `blocked`, not pass |
| One default reviewer succeeded on nontrivial development | `fail` until Red adjudication exists |
| Degraded coverage without Blue reason | `fail` |
| Blocker/high authority-boundary finding without Red adjudication | `fail` |
| Premium review used without approval evidence | `fail` |
| KAS script claims read-only but mutation guard evidence is missing or dirty | `fail` |
| Complete coverage and required dispositions exist | `pass` |

## Evidence freshness

A future implementation should bind MAR artifacts to the same repository state reviewed by Blue:

- repository root;
- git head or explicit dirty-worktree snapshot reference;
- input bundle checksum;
- diff snapshot checksum when a diff is reviewed;
- MAR status timestamp;
- Blue disposition timestamp;
- Red adjudication timestamp when applicable.

If the reviewed diff/input bundle is stale relative to the final gate target, KAH should fail closed or require a refreshed MAR record.

## Relationship to existing KAH gates

MAREV implementation should be additive. It must not replace existing review, verification, docs, token-economy, graph, workflow, or final gates unless a later reviewed task explicitly changes those gates. For initial adoption, KAS may record MAR artifacts without KAH enforcing them, as long as reports do not claim helper-enforced MAR behavior.

## Deferrals

Deferred unless separately approved: KAH execution of reviewer providers, subjective review quality judgment, automatic Red routing, automatic Kanban assignment, premium-review approval decisions, alternate-provider selection decisions, waiver approval decisions, provider retry policy decisions, KAB session control, auth/token/provider/gateway/model mutation, warning-only advisory states, and treating MAR as a replacement for required team color review gates.
