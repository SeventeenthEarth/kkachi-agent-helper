# KAH docs archive index

Date: 2026-05-21
Owner: KAH documentation archive
Confirming role: Responsible approver / governance evidence record
Status: docs index; read-only workflow graph implementation evidence present and mutation evidence pending
Authority level: reading guide for KAH docs; mutation planning records remain implementation-pending until capability/help evidence exists
Scope: `kkachi-agent-helper/docs` only
Related docs: `docs/specs.md`, `docs/sot/workflow-graph.md`, `docs/roadmap.md`, `docs/compatibility.md`
Evidence/source paths:
- Governance evidence record in kanban task `t_2fb00394`

## Purpose

This directory is the project archive for `kkachi-agent-helper` docs. It separates implemented helper behavior, candidate SOT updates, compatibility records, and roadmap planning so future Kkachi agents can reconstruct which file has authority before acting.

## Authority ladder

| Path | Meaning | Owner / confirming role | Authority |
|---|---|---|---|
| `docs/specs.md` | Current KAH helper behavior SOT | KAH owner; governance approval evidence recorded for Kkachi command use | Authoritative for implemented/helper behavior unless a narrower confirmed `docs/sot/*` file supersedes a section |
| `docs/sot/workflow-graph.md` | SOT/spec for `.kkachi-workflow.yaml` and graph support | KHS proposes policy/templates; KAH validates/explains read-only today; future tasks write/apply approved graph changes | Authority for implemented read-only graph validation/explanation; planning SOT for mutation |
| `docs/roadmap.md` | Active KAH delivery roadmap | KAH owner / responsible approver direction | Planning authority; not implementation authorization by itself |
| `docs/compatibility.md` | Release-facing KHS/KAH compatibility contract | KAH/KHS integration owners | Compatibility matrix and activation guidance |
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

- `.kkachi-workflow.yaml` is documented as project-level workflow graph state with implemented read-only validation/explanation.
- `.kkachi/config.yaml` remains helper runtime/configuration only.
- `.kkachi/runs/<run_id>/phase-plan.yaml` remains run-local execution state/evidence and is not deprecated.
- Kkachi v2 `.kkachi/config/workflows/` is outside KAH/KHS graph scope and must not be used as fallback graph authority.
- `kkachi-agent-helper graph validate` and `kkachi-agent-helper graph explain` are implemented; `kah graph` remains planned/candidate shorthand unless alias evidence exists.

## Stale/conflict markers

- Older wording that treats `phase-plan.yaml` as the whole workflow SOT must be read narrowly as run-local execution state for one KHS run.
- Older root-level kkachi config YAML/JSON graph phrasing, if encountered, is superseded for this planning SOT by `.kkachi-workflow.yaml`.
- `docs/TODO-ALIGN.md` is deleted in the current working tree and must not be treated as an active roadmap authority.

## Open questions

- The read-only `.kkachi-workflow.yaml` schema is implemented for validation/explanation; mutation schema details remain open until later tasks close.
- The real command name is `kkachi-agent-helper graph`; alias policy for `kah graph` remains unimplemented and current binary evidence must be checked before use.
- Graph mutation authority still requires capability/help evidence and any required review gates.

## Next record action

Use `docs/roadmap.md` `graph-003` as the next implementation slice for semantic graph diff and proposal records. Do not widen read-only graph validation/explanation into mutation behavior.
