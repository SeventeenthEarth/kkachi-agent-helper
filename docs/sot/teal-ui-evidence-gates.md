# DESIGN Teal UI evidence gates for KAH

Status: DESIGN-005 source-side implementation SOT for KAH design evidence schema/artifact bootstrap, fail-closed gate checks, diagnostics readback, and final-gate integration
Owner: KAH deterministic helper layer
Source evidence: `/Users/draccoon/Workspace/Hermes/17thHermes/40_outputs/projects/kkachi/2026-06-21-kas-kah-teal-ui-workflow-sot.md`
Upstream KAS SOT: `kkachi-hermes-skills/docs/sot/teal-ui-workflow-policy.md`

## Purpose

This document registers KAH's deterministic helper responsibilities for the `DESIGN` epic. KAS owns Teal workflow policy and role contracts. KAH owns only evidence shape, artifact bootstrap, schemas, gate checks, diagnostics, and final-gate integration where declared.

KAH must not decide whether a design is good, select Teal owners, waive Teal, judge screenshots subjectively, choose workflows, run reviewers, or replace KAS/Blue/Teal authority. KAH records deterministic facts and fails closed when required declared evidence is missing or malformed.

DESIGN-004 implemented the canonical `design-evidence.json` artifact bootstrap and embedded/exportable `design-evidence` schema (`design004.v1`). DESIGN-005 adds the source-side `design-evidence` gate, diagnostics readback, and final-gate integration. KAH still validates deterministic shape and declared evidence presence only.

DESIGN-006 adds cross-repo readback fixtures for KAS-declared scenarios: `kkachi_non_ui_skip`, `kkachi_teal_lane_non_ui_skip`, `sudal_ui_required`, and `doksuri_ui_required`. KAS owns the canonical declarations in `kkachi-hermes-skills/docs/examples/design006-teal-compatibility-scenarios.json`; KAH tests consume those declarations from `KAS_DESIGN006_SCENARIOS` or the sibling KAS checkout when present and prove they map to deterministic `design-evidence` schema/gate expectations without KAH classifying UI or judging design quality.

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

DESIGN-004 schema validation rejects malformed `design-evidence.json` shape as ordinary schema validation. DESIGN-005 registers the built-in `design-evidence` gate, adds diagnostics `design_evidence` readback, and makes `gate final` require a passing/fresh `design-evidence` gate when `design-evidence.json` is declared in the run manifest. Required evidence gaps, unsafe refs, invalid skip/waiver evidence, and warning-only downgrade attempts fail closed.

Stable DESIGN-005 reason codes include `design_evidence_missing`, `design_evidence_schema_invalid`, `design_evidence_ref_unsafe`, `design_evidence_boundary_invalid`, `teal_required_evidence_missing`, `teal_required_plan_verdict_missing`, `teal_required_design_spec_missing`, `teal_required_fidelity_refs_missing`, `teal_required_screenshot_refs_missing`, `teal_required_verification_verdict_missing`, `teal_required_waiver_invalid`, `teal_skip_evidence_missing`, `teal_skip_reason_missing`, `teal_waiver_evidence_invalid`, and `warning_only_fallback_forbidden`.

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

This SOT does not authorize installed binary readiness, profile skill updates, runtime/provider/auth/token/gateway/model mutation, KAB activation, release, push, KAS policy mutation, or applying Teal to non-UI Kkachi repository work. Final closure still requires task-specific implementation evidence, KAS compatibility evidence, and official review gates.
