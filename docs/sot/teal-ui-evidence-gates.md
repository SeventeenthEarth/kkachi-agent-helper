# DESIGN Teal UI evidence gates for KAH

Status: DESIGN-004 implementation SOT for KAH design evidence schema/artifact bootstrap; DESIGN-005 gate/diagnostic behavior remains deferred
Owner: KAH deterministic helper layer
Source evidence: `/Users/draccoon/Workspace/Hermes/17thHermes/40_outputs/projects/kkachi/2026-06-21-kas-kah-teal-ui-workflow-sot.md`
Upstream KAS SOT: `kkachi-hermes-skills/docs/sot/teal-ui-workflow-policy.md`

## Purpose

This document registers KAH's deterministic helper responsibilities for the `DESIGN` epic. KAS owns Teal workflow policy and role contracts. KAH owns only evidence shape, artifact bootstrap, schemas, gate checks, diagnostics, and final-gate integration where declared.

KAH must not decide whether a design is good, select Teal owners, waive Teal, judge screenshots subjectively, choose workflows, run reviewers, or replace KAS/Blue/Teal authority. KAH records deterministic facts and fails closed when required declared evidence is missing or malformed.

DESIGN-004 implements only the canonical `design-evidence.json` artifact bootstrap and embedded/exportable `design-evidence` schema (`design004.v1`). KAH validates deterministic JSON shape only in this task; DESIGN-005 remains responsible for any fail-closed Teal gate, diagnostic, or final-gate behavior.

## KAH evidence fields

KAH companion work should support deterministic fields such as:

```yaml
project_has_teal_lane: true|false
ui_ux_change: true|false
ui_ux_classification_owner: string|null
teal_required: true|false
teal_skip_reason: string|null
teal_owner: string|null
teal_waiver_approved: true|false
teal_waiver_approval_ref: string
teal_waiver_scope: string|null
teal_waiver_expires_at: string
required_when_teal_required: []
missing_required_status: string
teal_design_discussion_ref: string|null
plan_design_spec_ref: string|null
plan_reference_image_refs: []
expected_state_screenshot_list: []
fidelity_criteria_ref: string|null
teal_plan_verdict: string|null
teal_plan_verdict_evidence_ref: string|null
final_screenshot_refs: []
teal_design_verification_verdict: string|null
design_verification_evidence_ref: string|null
teal_color_review_verdict: string|null
teal_color_review_evidence_ref: string|null
```

## Gate posture

If `teal_required=true`, KAH must fail closed when declared required design-plan verdicts, design spec refs, fidelity criteria, final screenshot refs, design-verification verdicts, or authorized waiver evidence are missing or malformed.

If `teal_required=false`, KAH should validate that a concrete `teal_skip_reason` or authorized waiver evidence exists and report not-applicable/pass only through deterministic shape checks.

DESIGN-004 schema validation rejects malformed `design-evidence.json` shape as ordinary schema validation. It does not register a `design` gate, update diagnostics, or integrate with `gate final`; those fail-closed checks are deferred to DESIGN-005 and must not be replaced with warning-only behavior.

## Sequential task order

주군 selected a sequential seven-task roadmap. Do not parallelize these tasks. Execute by task id order:

1. `DESIGN-001` — KAS docs/SOT adoption and shared roadmap registration.
2. `DESIGN-002` — KAS Teal applicability and node contract semantics.
3. `DESIGN-003` — KAS workflow selector/materializer and skill guidance.
4. `DESIGN-004` — KAH design evidence schema and artifact bootstrap.
5. `DESIGN-005` — KAH fail-closed gate and diagnostics support.
6. `DESIGN-006` — Cross-repo compatibility examples and proof fixtures.
7. `DESIGN-007` — Full verification, docs map, Red/Orange/Gray/Teal review, and Blue closeout.

Small cross-edits are allowed only for field-name alignment, docs links, fixture stubs, or compatibility examples. New KAS policy behavior or KAH validation behavior must remain in the owning repository's task.

## Boundaries

This planning SOT does not authorize helper code behavior, schema export claims, installed binary readiness, profile skill updates, runtime/provider/auth/token/gateway/model mutation, KAB activation, release, push, or applying Teal to non-UI Kkachi repository work. Implementation status requires task-specific code/docs/tests, KAS compatibility evidence, and official review gates.
