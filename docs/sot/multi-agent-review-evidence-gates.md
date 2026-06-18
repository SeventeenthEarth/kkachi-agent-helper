# Multi-Agent Review evidence gates

Date: 2026-06-17
Owner: KAH deterministic helper layer
Status: accepted SOT with MAREV-002 source-side implementation evidence; release/install claims still require normal KAH release gates
Authority level: KAH-side source of truth for MAR evidence capture and deterministic gate/schema behavior; KAS remains policy owner
Upstream KAS SOT: KAS `docs/sot/multi-agent-review-policy.md`
Source SOT: `/Users/draccoon/Workspace/Hermes/17thHermes/40_outputs/projects/kkachi/2026-06-16-kkachi-multi-agent-review-mar-sot.md`
Scope: MAR evidence artifact layout, deterministic validation expectations, final-gate posture, and KAH/KAS/KAB boundaries

## Purpose

This document records the KAH-side planning contract for Kkachi Multi-Agent Review (MAR). MAR itself is a KAS-owned review policy and script surface. KAH's role is deterministic evidence preservation and gate validation only.

`MAREV-001` recorded the planning contract. `MAREV-002` implements source-side KAH artifact/gate/schema support for MAR evidence through canonical `multi-agent-review/status.json`, the `multi-agent-review` gate, and the embedded/exported `multi-agent-review-evidence` schema. KAH still does not execute reviewers, select providers, or adjudicate findings.

## Task split

| Task | Scope | Completion claim allowed |
|---|---|---|
| `MAREV-001` | KAH planning SOT, docs index, roadmap row, and KAS/KAH boundary record. | Planning authority only; no helper behavior. |
| `MAREV-002` | Deterministic KAH implementation for MAR artifact/gate/schema support aligned to the upstream KAS `MAR-005` role-first review contract: declared review roles, role coverage, primary/secondary provider attempts, retry/escalation/waiver evidence, Blue disposition, and triggered Red adjudication. | Source-side helper behavior implemented for canonical `multi-agent-review/status.json`, `gate check multi-agent-review`, final-gate integration when MAR is required, and schema validation/export; review/release gates remain separate. |
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

KAH deterministically validates whether declared MAR artifacts exist, are structurally valid, are safe repository-local paths, have expected status vocabulary, and satisfy final-gate freshness/completeness rules.

## MAREV-002 implemented behavior

`MAREV-002` follows upstream KAS `MAR-005` role-first evidence. KAH treats KAS-declared review roles as the validation unit and provider attempts as evidence attached to those roles. The source-side implementation validates the canonical `multi-agent-review/status.json` artifact with schema version `mar-evidence.v1`.

KAH validates mechanical evidence for each KAS-declared required review role, such as `logic`, `security`, `arch`, `cve`, and `test_adequacy`, without deciding which roles or providers are appropriate. The KAS MAR package must declare role policy, required/conditional status, primary/secondary provider candidates, and the final role coverage status. KAH checks that this declaration is present, self-consistent, repository-confined, fresh, and fail-closed when coverage is incomplete.

KAH must not assume `zcode`, `kimi`, or `antigravity` are the permanent validation surface. Those provider lanes may appear as primary/secondary evidence for a role, but role coverage is the stable gate contract. If `MAR-005` changes the exact artifact names or schema, `MAREV-002` should follow the reviewed KAS schema and update this SOT, fixtures, roadmap notes, and gate diagnostics in the same KAH task.

## Artifact layout

Implemented canonical gate artifact plus recommended supporting layout:

```text
.kkachi/runs/<run_id>/multi-agent-review/
  request.yaml
  doctor.json
  input-bundle.md
  diff.patch
  roles/
    role-matrix.json
    role-coverage.json
  prompt/
    logic.primary.md
    security.primary.md
    arch.primary.md
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
    role-attempts.json
    provider-attempts.json       # compatibility/detail view, derived from role attempts when present
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

KAH enforces `multi-agent-review/status.json` as the canonical gate artifact when `task_id` has the `mar-` prefix or the run manifest explicitly requires that artifact. Supporting files remain KAS-produced evidence references. KAH validates referenced paths/checksums/markers mechanically when the status artifact points to them.

## Deterministic checks

MAREV-002 checks only mechanical facts from canonical `multi-agent-review/status.json` and any evidence refs it names, such as:

- the status artifact names the run id, task id, role coverage state, provider attempt records, and Blue disposition reference;
- optional KAS-produced supporting files such as `request.yaml`, `doctor.json`, `role-matrix.json`, `role-coverage.json`, and `role-attempts.json` remain policy/evidence artifacts owned by KAS and are mechanically checked by KAH only when `status.json` references them through repository-confined refs, checksums, or markers;
- provider attempt records include an attempted entry for every covered or waived required review role, including role id, attempted provider lane, model label when available, attempt status, deterministic reason code when failed, redacted raw/stderr path references when required, and mutation-guard reference;
- failed required role coverage is not converted to clean `PASS` unless a linked retry succeeds, an explicitly approved secondary/alternate provider attempt succeeds, or `waiver-evidence.yaml` records 주군's waiver with waived role id, reason, accepted residual risk, timestamp, and approval reference;
- `retry-attempts.json`, when present, links every retry to the original failed attempt and records bounded retry count, reason, result, and evidence paths;
- `alternate-approvals.yaml`, when present, identifies the failed role coverage replaced, approved secondary/alternate provider/model, approval reference, reason, and successful attempt evidence; KAH validates only that the approval/evidence exists, not whether the alternate provider was semantically preferable;
- `waiver-evidence.yaml`, when present, identifies the waived role failure, 주군 approval reference, accepted residual risk, and any required Blue/Red disposition references;
- `status.json` uses an accepted MAR status value: `PASS`, `PASS_WITH_FINDINGS`, `REQUEST_CHANGES`, `BLOCKED`, `DEGRADED`, or `FAILED`;
- review-role coverage semantics are represented explicitly, including failed/unavailable providers for each required role;
- all raw/parsed/prompt/attempt paths referenced by status files are repository-confined and present when claimed complete;
- degraded coverage includes a Blue reason;
- insufficient required-role coverage or triggered risk categories require `red-adjudication.md` before final completion;
- premium reviewer artifacts exist only with a `premium-request.md` / approval reference when required by policy;
- before/after mutation guard evidence is present when the KAS script claims read-only review;
- final Blue disposition exists before a final gate accepts MAR as satisfied.

KAH must not judge whether a finding is correct, choose whether a failed role/provider should be retried, select a secondary/alternate provider, or approve a waiver. It can only check that Blue, Red, or 주군 recorded the required disposition/approval evidence for findings and role-coverage decisions declared by the KAS MAR package.

## Gate semantics target

The implemented MAR-aware gate uses fail-closed statuses:

| Condition | Gate posture |
|---|---|
| No MAR required for the active task | `not_applicable` with reason |
| MAR required but request/status artifacts missing | `fail` |
| Required review role missing a coverage or attempt entry | `fail` |
| Failed required role lacks deterministic reason code or redacted evidence reference | `fail` |
| Failed required role has no successful linked retry, approved secondary/alternate-provider success, or explicit 주군 waiver evidence | `fail` or `blocked`, not pass |
| Alternate provider used without explicit approval evidence | `fail` |
| Waiver lacks waived role, approval reference, or accepted residual-risk record | `fail` |
| All required review roles failed | `fail` or `blocked`, not pass |
| Only partial required-role coverage succeeded on nontrivial development | `fail` until Blue disposition and required Red adjudication exist |
| Degraded coverage without Blue reason | `fail` |
| Blocker/high authority-boundary finding without Red adjudication | `fail` |
| Premium review used without approval evidence | `fail` |
| KAS script claims read-only but mutation guard evidence is missing or dirty | `fail` |
| Complete coverage and required dispositions exist | `pass` |

## Evidence freshness

KAS-produced MAR evidence should bind artifacts to the same repository state reviewed by Blue:

- repository root;
- git head or explicit dirty-worktree snapshot reference;
- input bundle checksum;
- diff snapshot checksum when a diff is reviewed;
- MAR status timestamp;
- Blue disposition timestamp;
- Red adjudication timestamp when applicable.

If the reviewed diff/input bundle is stale relative to the final gate target, KAH should fail closed or require a refreshed MAR record.

## Relationship to existing KAH gates

MAREV implementation is additive. It does not replace existing review, verification, docs, token-economy, graph, workflow, or final gates. The final gate requires `multi-agent-review` when the task id is `mar-*` or the run manifest explicitly requires `multi-agent-review/status.json`; otherwise MAR can remain not applicable.

## Deferrals

Deferred unless separately approved: KAH execution of reviewer providers, subjective review quality judgment, automatic Red routing, automatic Kanban assignment, premium-review approval decisions, alternate-provider selection decisions, waiver approval decisions, provider retry policy decisions, KAB session control, auth/token/provider/gateway/model mutation, warning-only advisory states, and treating MAR as a replacement for required team color review gates.
