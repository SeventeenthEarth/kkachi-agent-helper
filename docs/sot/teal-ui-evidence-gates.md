# DESIGN Teal UI evidence gates for KAH

Status: planning SOT for the KAH companion side of the `DESIGN` shared KAS/KAH epic; not implementation evidence
Owner: KAH deterministic helper layer
Source evidence: `/Users/draccoon/Workspace/Hermes/17thHermes/40_outputs/projects/kkachi/2026-06-21-kas-kah-teal-ui-workflow-sot.md`
Upstream KAS SOT: `kkachi-hermes-skills/docs/sot/teal-ui-workflow-policy.md`

## Purpose

This document registers KAH's deterministic helper responsibilities for the `DESIGN` epic. KAS owns Teal workflow policy and role contracts. KAH owns only evidence shape, artifact bootstrap, schemas, gate checks, diagnostics, and final-gate integration where declared.

KAH must not decide whether a design is good, select Teal owners, waive Teal, judge screenshots subjectively, choose workflows, run reviewers, or replace KAS/Blue/Teal authority. KAH records deterministic facts and fails closed when required declared evidence is missing or malformed.

## KAH evidence fields

KAH companion work should support deterministic fields such as:

```yaml
project_has_teal_lane: true|false
ui_ux_change: true|false
ui_ux_classification_owner: string|null
teal_required: true|false
teal_skip_reason: string|null
teal_owner: string|null
teal_waiver_ref: string|null
teal_waiver_approvers: []
teal_waiver_reason: string|null
teal_waiver_scope: string|null
teal_waiver_residual_risk: string|null
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
