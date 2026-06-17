# KAH docs archive index

Date: 2026-05-22
Owner: KAH documentation archive
Confirming role: Responsible approver / governance evidence record
Status: docs index; workflow graph init/validation/explanation/diff/proposal/apply/export, graph compatibility reason-code diagnostics, approval-gated complete-candidate repair substrate, and phase-plan feedback-bound validation evidence present
Authority level: reading guide for KAH docs; graph export records are implemented evidence and generated-artifact boundaries
Scope: `kkachi-agent-helper/docs` only
Related docs: `docs/specs.md`, `docs/roadmap.md`, `docs/compatibility.md`, `docs/sot/graph-workflow-sync-diagnostics-and-repair.md`, `docs/sot/task-dag-state-machine.md`, `docs/sot/token-economy-evidence-gates.md`
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
| `docs/sot/multi-agent-review-evidence-gates.md` | KAH-side planning SOT for MAR evidence capture and future gates | KAH deterministic helper layer with KAS MAR dependency | Planning authority for MAR artifact layout, deterministic validation targets, and final-gate posture; `MAREV-001` is docs-only and helper behavior requires `MAREV-002` or later implementation evidence |
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
- Multi-Agent Review evidence-gate planning lives in `docs/sot/multi-agent-review-evidence-gates.md`; `MAREV-001` is the docs/SOT record only, and actual helper artifact/gate/schema support belongs to `MAREV-002` or later.

## Stale/conflict markers

- Older wording that treats `phase-plan.yaml` as the whole workflow SOT must be read narrowly as run-local execution state for one KHS run.
- Older root-level kkachi config YAML/JSON graph phrasing, if encountered, is superseded for this planning SOT by `.kkachi-workflow.yaml`.
- `docs/TODO-ALIGN.md` is deleted in the current working tree and must not be treated as an active roadmap authority.

## Open questions

- The `.kkachi-workflow.yaml` schema is implemented for init/validation/explanation/diff/proposal/apply records and graph compatibility diagnostics with stable reason codes; export is implemented as non-authoritative generated artifacts only.
- The real command name is `kkachi-agent-helper graph`; alias policy for `kah graph` remains unimplemented and current binary evidence must be checked before use.
- Generated graph exports are implemented as non-authoritative artifacts; future graph slices should not promote export output into graph authority.

## Next record action

Use future `docs/roadmap.md` graph slices for separately scoped release or KHS consumption work. After `grsync-002`, remaining graph workflow sync expansions stay bounded by `docs/sot/graph-workflow-sync-diagnostics-and-repair.md`; do not widen graph export into generated-artifact authority, KAS compatibility registry behavior, automatic apply/update behavior, or alias behavior.
