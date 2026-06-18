# POLPR helper evidence and workflow-alignment planning SOT

Date: 2026-06-18
Owner: KAH deterministic helper layer
Confirming role: Responsible approver / KAS Blue command with Red, Orange, and project-Gray review evidence
Status: planning SOT for KAH companion work; not implemented helper behavior by itself
Authority level: KAH-side planning authority for shared POLPR companion tasks
Scope: KAH roadmap/docs registration, helper default phase-plan support, deterministic evidence/gate fields, diagnostics, and tests required to support KAS POLPR without moving policy ownership into KAH
Source evidence: `/Users/draccoon/Workspace/Hermes/17thHermes/40_outputs/projects/kkachi/2026-06-14-kas-policy-promotion-candidates.md`
Upstream KAS SOT: `kkachi-hermes-skills/docs/sot/policy-promotion-governance-contract.md`
Epic: `POLPR` — policy-promotion helper evidence alignment

## Purpose

The KAS POLPR epic promotes accepted workflow-governance, review, test-layering, docs-update, and agent-instruction policy into KAS. KAH's role is narrower: provide deterministic helper support and evidence surfaces where those KAS policies require machine-checkable run artifacts, default phase-plan scaffolding, gate/report fields, diagnostics, or tests.

KAH must not decide KAS workflow policy, choose reviewers, adjudicate MAR findings, decide whether a project should use the default graph, author prompts, mutate profile skills, or replace Kanban/color-review authority. KAH reports deterministic facts and validates declared artifacts fail-closed.

## Companion task prefix

- KAH epic slug: `POLPR`
- Shared cross-repo PR/task ids: `POLPR-001` through `POLPR-008`
- KAH-owned slices: `POLPR-002`, `POLPR-005`, and `POLPR-007`
- Upstream KAS epic: `POLPR`

KAH follows the upstream approved shared-numbering strategy for this tightly coupled companion work, but it does not make shared numbering the default for other multi-repository projects. Shared single-epic numbering requires explicit responsible-approver authorization and should be avoided for large repositories, independent product/release lifecycles, or broad projects whose components need separate ownership. If shared numbering is not explicitly approved, KAH companion work should use independent per-repository epics and task sequences.

## KAH companion principles

1. **Policy remains KAS-owned:** KAH implements or documents only deterministic state, schema, validation, evidence, diagnostics, and tests.
2. **MAR-only naming support:** KAH default phase-plan/example/test wording must not preserve `GLM Octo` review as an active KAS/KAH review lane. Any independent-review phase name used by KAH defaults should align with `mar-review` when the default KAS workflow requires it.
3. **Configurable default graph:** KAH may initialize or validate a default phase/graph shape supplied by KAS, but custom project graphs remain valid when they satisfy the declared schema/supportability rules.
4. **Evidence, not judgment:** KAH may record document impact map paths, searched surfaces, review artifact refs, test-layer labels, and final-gate presence. KAH does not judge prose quality or review sufficiency beyond declared deterministic fields.
5. **Fail-closed mutation:** Any proposal/apply behavior must require explicit approval evidence, base-state checks, checksum/reason-code evidence, and no silent direct-YAML fallback.

## Companion PR-candidate slices

| Task ID | Title | Status | Work guide | Notes |
|---|---|---|---|---|
| `POLPR-002` | Register POLPR helper SOT and roadmap companion | In Progress | Add this SOT, roadmap entry, docs index/map registration, and cross-link to the KAS `POLPR-001` SOT. | Docs/SOT planning only; no helper behavior claim. |
| `POLPR-005` | Align default phase-plan and MAR naming support | Planned | Update KAH default phase-plan support/tests and related docs so active KAS defaults use `mar-review` instead of `octo-review`, while custom project workflows remain supportability-based. | KAH does not choose MAR providers or adjudicate findings. |
| `POLPR-007` | Add deterministic docs/test/review evidence support if needed | Planned | Add or document machine-checkable evidence fields for document impact maps, project-Gray coverage refs, test-layer labels, failed-test repair ownership, and final stale-status checks when KAS POLPR implementation needs helper validation. | Scope is evidence presence/shape only; KAS owns policy and reviewer meaning. |

## Impact map baseline

KAH POLPR work must inspect at least:

- `docs/roadmap.md`, `docs/README.md`, and `docs/kkachi-docs-map.yaml`.
- This SOT and existing KAH SOTs that name graph/default phase-plan, MAR evidence, token-economy evidence, final gates, or compatibility boundaries.
- `docs/specs.md` and `docs/compatibility.md` only when helper behavior or release-facing support changes.
- `internal/project/phase_plan.go`, `internal/project/phase_plan_test.go`, gate/evidence tests, fixtures, and e2e coverage when implementation changes default phases or evidence validation.
- The upstream KAS POLPR SOT and candidate note for policy language, without treating the candidate note as implemented helper behavior.

## Acceptance criteria for POLPR-002

- This SOT exists and states KAH's deterministic-helper boundary for POLPR and the opt-in nature of shared cross-repository numbering.
- KAH `docs/roadmap.md` registers `POLPR` in delivery order and active roadmap with KAH-owned slices `POLPR-002`, `POLPR-005`, and `POLPR-007`.
- KAH `docs/README.md` and `docs/kkachi-docs-map.yaml` reference this SOT.
- Cross-links to the KAS POLPR SOT and source candidate note are present.
- Verification includes docs readback, docs-map YAML parse, `git diff --check`, and repository test command or explicit blocker/degraded reason.

## Deferrals and non-goals

- No KAH behavior, command, schema, gate, diagnostics, release, install, or runtime state change is completed by `POLPR-002`.
- No provider execution, reviewer choice, MAR adjudication, model voting, automatic Red/Kanban routing, profile mutation, KAB session control, auth/token/provider/gateway/model mutation, warning-only MAR gate, or automatic graph apply is authorized.
- No KAH policy ownership over KAS docs-update, agent-instruction lifecycle, test taxonomy, review governance, or default workflow selection is introduced.

## Next action

After `POLPR-002` docs/SOT registration is accepted, implement companion changes only when a KAS POLPR slice requires deterministic helper support and the KAS plan/impact map names the exact KAH surface to update.
