# KAH docs archive index

Date: 2026-05-22
Owner: KAH documentation archive
Confirming role: Responsible approver / governance evidence record
Status: docs index; workflow graph init/validation/explanation/diff/proposal/apply/export, graph compatibility reason-code diagnostics, approval-gated complete-candidate repair substrate, and phase-plan feedback-bound validation evidence present
Authority level: reading guide for KAH docs; graph export records are implemented evidence and generated-artifact boundaries
Scope: `kkachi-agent-helper/docs` only
Related docs: `docs/specs.md`, `docs/roadmap.md`, `docs/compatibility.md`, `docs/sot/graph-workflow-sync-diagnostics-and-repair.md`, `docs/sot/task-dag-state-machine.md`, `docs/sot/token-economy-evidence-gates.md`, `docs/sot/multi-agent-review-evidence-gates.md`, `docs/sot/policy-promotion-helper-evidence.md`, `docs/sot/strict-workflow-enforcement.md`, `docs/sot/toolchain-probe-contract.md`, `docs/sot/teal-ui-evidence-gates.md`, `docs/sot/gajae-gjc-wrapper-evidence.md`
Evidence/source paths:
- Governance evidence record in kanban task `t_2fb00394`

## Purpose

This directory is the project archive for `kkachi-agent-helper` docs. It separates implemented helper behavior, candidate SOT updates, compatibility records, and roadmap planning so future Kkachi agents can reconstruct which file has authority before acting.

## Authority ladder

| Path | Meaning | Owner / confirming role | Authority |
|---|---|---|---|
| `docs/specs.md` | Current KAH helper behavior SOT, including `.kkachi-workflow.yaml` command/schema behavior | KAH owner; governance approval evidence recorded for Kkachi command use | Authoritative for implemented/helper behavior and workflow graph behavior |
| `docs/roadmap.md` | Active KAH delivery roadmap | KAH owner / responsible approver direction | Planning authority; not implementation authorization by itself |
| `docs/compatibility.md` | Release-facing KHS/KAH compatibility contract | KAH/KHS integration owners | Compatibility matrix, activation guidance, and graph fallback rules |
| `docs/sot/graph-workflow-sync-diagnostics-and-repair.md` | KAH-side planning SOT for graph workflow sync diagnostics and approval-gated repair substrate | KAH deterministic helper layer with KAS v0.1.2 dependency | Historical planning authority for KAH v0.1.9 graph diagnostics/repair work; implemented behavior is reflected in specs, compatibility, and release notes |
| `docs/sot/task-dag-state-machine.md` | KAH-side planning SOT for task-DAG workflow state machine support | KAH deterministic helper layer with KAS WFLOW dependency | Planning authority for KAH DAGSM schema validation, workflow instance state, node FSM/order enforcement, ready-node calculation, diagnostics, and final gates; not implemented behavior by itself |
| `docs/sot/token-economy-evidence-gates.md` | KAH-side planning SOT for future token-economy / English-output evidence gates | KAH deterministic helper layer with KAS workstream dependency | Planning authority for future KAH gate work; not implemented behavior by itself |
| `docs/sot/multi-agent-review-evidence-gates.md` | KAH-side SOT for MAR evidence capture and deterministic gate/schema behavior | KAH deterministic helper layer with KAS MAR dependency | Source of truth for MAR artifact layout, deterministic validation targets, and final-gate posture; `MAREV-002` implements source-side helper artifact/gate/schema support while KAS remains policy owner |
| `docs/sot/policy-promotion-helper-evidence.md` | KAH-side POLPR planning SOT | KAH deterministic helper layer with KAS POLPR dependency | Planning authority for KAH companion docs, default phase-plan support, evidence fields, diagnostics, and tests needed by KAS POLPR without moving workflow/review policy ownership into KAH |
| `docs/sot/strict-workflow-enforcement.md` | KAH-side STRICT companion SOT | KAH deterministic helper layer with KAS STRICT dependency | Planning authority for workflow-managed strict final-gate markers, node claim ledger/order verification, and workflow-to-phase projection consistency; KAH does not choose task class, workflow, agent, prompt, or backend |
| `docs/sot/toolchain-probe-contract.md` | KAH-side TOLMR companion SOT | KAH deterministic helper layer with KAS TOLMR dependency | Planning authority for the read-only project/helper fact probe consumed by KAS `.kkachi/toolchain.yaml` generation; KAH does not choose stage, MAR policy, KAS baselines, or write toolchain state |
| `docs/sot/teal-ui-evidence-gates.md` | KAH-side DESIGN companion SOT | KAH deterministic helper layer with KAS DESIGN dependency | Planning authority for deterministic Teal/UI evidence fields, artifact/schema bootstrap, fail-closed gate checks, and diagnostics; KAH does not classify UI, select Teal, waive gates, or judge design quality |
| `docs/sot/gajae-gjc-wrapper-evidence.md` | KAH-side GAJAE companion SOT | KAH deterministic helper layer with KAS GAJAE dependency | Planning authority for the deterministic GJC wrapper, GJC/KAT artifact refs, async status, callback idempotency, and watcher-compatible evidence; KAH does not approve plans, reviews, MAR, or final completion |
| `docs/release-notes-template.md` | Release note template | KAH release owner | Template only |
| `docs/.omx/` if present | Tool/runtime/agent state | Tooling | Non-authoritative; never a KAH docs SOT |

## Status vocabulary

| Status | Meaning |
|---|---|
| `source of truth` | Confirmed current authority for its stated scope |
| `candidate SOT` | Proposed normative record pending confirmation and/or implementation evidence |
| `planning SOT` | Confirmed planning authority that still requires implementation evidence before release behavior claims |
| `planned/candidate` | Roadmap or proposed command surface, not proven implemented |
| `historical` | Preserved context; not current authority by itself |
| `stale` | Known to conflict with newer evidence or decisions; preserve with marker rather than silently delete |
| `superseded` | Replaced by a named newer authority |

## Decision summary

- `.kkachi-workflow.yaml` is documented as project-level workflow graph state with implemented init, validation/explanation, semantic diff, proposal records, approval-gated apply, complete-candidate repair for missing/stale/broken invalid graph states, compatibility diagnostics, `EXTERNAL_FEEDBACK_INTAKE` bounds projection/migration/activation evidence, and phase-plan feedback-bound validation.
- `.kkachi/config.yaml` remains helper runtime/configuration only.
- `.kkachi/runs/<run_id>/phase-plan.yaml` remains run-local execution state/evidence and is not deprecated.
- Kkachi v2 `.kkachi/config/workflows/` is outside KAH/KHS graph scope and must not be used as fallback graph authority.
- `kkachi-agent-helper graph init`, `graph validate`, `graph explain`, `graph diff`, `graph propose`, `graph apply`, and `graph export` are implemented; `kah graph` remains planned/candidate shorthand unless alias evidence exists.
- Graph behavior authority now lives in `docs/specs.md`; KHS/KAH graph activation and fallback guidance lives in `docs/compatibility.md`.
- Configurable `EXTERNAL_FEEDBACK_INTAKE` behavior lives in `docs/specs.md`; activation and fallback guidance lives in `docs/compatibility.md`; graph-009 through graph-011 implementation history lives in `docs/roadmap.md`.
- Graph workflow sync KAH planning lives in `docs/sot/graph-workflow-sync-diagnostics-and-repair.md`; `grsync-001` implements diagnostics/reason-code hardening for KAS v0.1.2 consumption, and `grsync-002` implements approval-gated complete-candidate graph repair substrate while preserving the no-direct-YAML-fallback boundary.
- Task-DAG workflow state-machine planning lives in `docs/sot/task-dag-state-machine.md`; `DAGSM-001` through `DAGSM-003` implement deterministic task-DAG validation, run-local workflow instance state, node FSM/order enforcement, ready-node calculation, multi-DAG catalog diagnostics, explicit catalog create mode, and final gate integration without KAH choosing agents, prompts, or workflow policy.
- Token-economy / English-output KAH evidence-gate planning lives in `docs/sot/token-economy-evidence-gates.md`; `token-001` and `token-002` deterministic evidence validation are implemented, reviewed, and accepted for commit-readiness pending separate 주군 commit/install approval.
- Multi-Agent Review evidence-gate behavior lives in `docs/sot/multi-agent-review-evidence-gates.md`; `MAREV-001` recorded the planning SOT and `MAREV-002` implements source-side canonical `multi-agent-review/status.json`, `multi-agent-review` gate, schema validation/export, and final-gate integration when MAR is required.
- POLPR helper companion planning lives in `docs/sot/policy-promotion-helper-evidence.md`; `POLPR-002` is KAH docs/SOT registration only under the shared cross-repo numbering, and later KAH slices remain limited to deterministic evidence/default phase-plan/docs/test support for KAS-owned policy.
- STRICT helper companion planning lives in `docs/sot/strict-workflow-enforcement.md`; KAH-owned `STRICT-002` is completed in KAH commit `97acd29`, KAS `STRICT-003` is completed in KAS commit `196d8d0`, KAH `STRICT-004` is implemented source-side for node claim ledger/order verification, KAS `STRICT-005` is complete source-side in KAS commit `cb16cae`, and KAH `STRICT-006` is implemented source-side for phase-plan projection consistency. Install/release/push/live activation and effective-runtime claims remain separate approvals/evidence.
- TOLMR helper companion planning lives in `docs/sot/toolchain-probe-contract.md`; KAH will provide read-only current-system/project facts for KAS-generated `.kkachi/toolchain.yaml`, while KAS owns schema, generation, Stage/MAR policy, legacy import, and final interpretation. Cross-repo TOLMR work uses one logical task/evidence package with separate KAH and KAS repo-local quality gates and physical commits.
- DESIGN helper companion planning lives in `docs/sot/teal-ui-evidence-gates.md`; KAH will validate deterministic Teal/UI evidence shape for KAS-selected UI-bearing workflows, fail closed on missing required evidence, and stay out of design judgment/waiver/owner-selection authority. DESIGN work proceeds sequentially through DESIGN-001..007.
- GAJAE helper companion planning lives in `docs/sot/gajae-gjc-wrapper-evidence.md`; KAH wraps GJC execution, preserves GJC/KAT artifact refs and hashes, exposes async/callback status, and stays out of KAS plan/review/MAR/final authority. GAJAE-001..009 completed source-side closeout through KAH-side KAT v0.1.0 evidence normalization; GAJAE-010 remains the follow-on contract docs/skill update task.

## Stale/conflict markers

- Older wording that treats `phase-plan.yaml` as the whole workflow SOT must be read narrowly as run-local execution state for one KHS run.
- Older root-level kkachi config YAML/JSON graph phrasing, if encountered, is superseded for this planning SOT by `.kkachi-workflow.yaml`.
- `docs/TODO-ALIGN.md` is deleted in the current working tree and must not be treated as an active roadmap authority.

## Open questions

- The `.kkachi-workflow.yaml` schema is implemented for init/validation/explanation/diff/proposal/apply records and graph compatibility diagnostics with stable reason codes; export is implemented as non-authoritative generated artifacts only.
- Strict workflow order enforcement is implemented source-side through `STRICT-004` under `docs/sot/strict-workflow-enforcement.md`; effective installed/runtime behavior still depends on the committed build being installed or activated through a separate approval path.
- The real command name is `kkachi-agent-helper graph`; alias policy for `kah graph` remains unimplemented and current binary evidence must be checked before use.
- Generated graph exports are implemented as non-authoritative artifacts; future graph slices should not promote export output into graph authority.

## Next record action

Use future `docs/roadmap.md` graph and STRICT slices for separately scoped release or KHS/KAS consumption work. After `grsync-002`, remaining graph workflow sync expansions stay bounded by `docs/sot/graph-workflow-sync-diagnostics-and-repair.md`; STRICT work stays bounded by `docs/sot/strict-workflow-enforcement.md` and must not widen KAH into task classification, workflow selection, automatic rollback, or alias behavior.
