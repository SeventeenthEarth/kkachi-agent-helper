# KAH docs archive index

Date: 2026-05-22
Owner: KAH documentation archive
Confirming role: Responsible approver / governance evidence record
Status: docs index; workflow graph init/validation/explanation/diff/proposal/apply/export evidence present
Authority level: reading guide for KAH docs; graph export records are implemented evidence and generated-artifact boundaries
Scope: `kkachi-agent-helper/docs` only
Related docs: `docs/specs.md`, `docs/roadmap.md`, `docs/compatibility.md`
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
| `docs/sot/external-feedback-intake.md` | Configurable `EXTERNAL_FEEDBACK_INTAKE` implementation plan | KAH documentation and implementation planning; responsible review before implementation | Planning SOT plus graph-009 checklist; not final release support claim |
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

- `.kkachi-workflow.yaml` is documented as project-level workflow graph state with implemented init, validation/explanation, semantic diff, proposal records, approval-gated apply, compatibility diagnostics, and read-only `EXTERNAL_FEEDBACK_INTAKE` bounds projection.
- `.kkachi/config.yaml` remains helper runtime/configuration only.
- `.kkachi/runs/<run_id>/phase-plan.yaml` remains run-local execution state/evidence and is not deprecated.
- Kkachi v2 `.kkachi/config/workflows/` is outside KAH/KHS graph scope and must not be used as fallback graph authority.
- `kkachi-agent-helper graph init`, `graph validate`, `graph explain`, `graph diff`, `graph propose`, `graph apply`, and `graph export` are implemented; `kah graph` remains planned/candidate shorthand unless alias evidence exists.
- Graph behavior authority now lives in `docs/specs.md`; KHS/KAH graph activation and fallback guidance lives in `docs/compatibility.md`.
- `docs/sot/external-feedback-intake.md` is the planning SOT for future final configurable `EXTERNAL_FEEDBACK_INTAKE` support and tracks completed graph-009 read-only schema support; it does not claim graph-011 activation support.

## Stale/conflict markers

- Older wording that treats `phase-plan.yaml` as the whole workflow SOT must be read narrowly as run-local execution state for one KHS run.
- Older root-level kkachi config YAML/JSON graph phrasing, if encountered, is superseded for this planning SOT by `.kkachi-workflow.yaml`.
- `docs/TODO-ALIGN.md` is deleted in the current working tree and must not be treated as an active roadmap authority.

## Open questions

- The `.kkachi-workflow.yaml` schema is implemented for init/validation/explanation/diff/proposal/apply records and graph compatibility diagnostics; export is implemented as non-authoritative generated artifacts only.
- The real command name is `kkachi-agent-helper graph`; alias policy for `kah graph` remains unimplemented and current binary evidence must be checked before use.
- Generated graph exports are implemented as non-authoritative artifacts; future graph slices should not promote export output into graph authority.

## Next record action

Use future `docs/roadmap.md` graph slices for separately scoped release or KHS consumption work. Do not widen graph export into generated-artifact authority or alias behavior.
