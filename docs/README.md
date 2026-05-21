# KAH docs archive index

Date: 2026-05-21
Owner: KAH documentation archive
Confirming role: Hwangchung / KHC Blue commander
Status: candidate docs index pending Blue confirmation
Authority level: reading guide for KAH docs after confirmation; Gray edits remain candidate until Blue confirms
Scope: `kkachi-agent-helper/docs` only
Related docs: `docs/specs.md`, `docs/sot/workflow-graph.md`, `docs/roadmap.md`, `docs/compatibility.md`
Evidence/source paths:
- `/Users/draccoon/.hermes/kanban/workspaces/t_81f61495/hwangchung-final-kah-khs-graph-docs-plan.md`
- Kanban task `t_2fb00394`

## Purpose

This directory is the project archive for `kkachi-agent-helper` docs. It separates implemented helper behavior, candidate SOT updates, compatibility records, and roadmap planning so future Kkachi agents can reconstruct which file has authority before acting.

## Authority ladder

| Path | Meaning | Owner / confirming role | Authority |
|---|---|---|---|
| `docs/specs.md` | Current KAH helper behavior SOT | KAH owner, confirmed by Hwangchung for Kkachi command use | Authoritative for implemented/helper behavior unless a narrower confirmed `docs/sot/*` file supersedes a section |
| `docs/sot/workflow-graph.md` | Candidate SOT/spec for `.kkachi-workflow.yaml` and planned `kah graph` support | KHS proposes policy/templates; KAH validates/writes/applies; Hwangchung confirms before normative use | Candidate SOT only until implementation and Blue confirmation exist |
| `docs/roadmap.md` | Active KAH delivery roadmap | KAH owner / Blue direction | Planning authority; not implementation authorization by itself |
| `docs/compatibility.md` | Release-facing KHS/KAH compatibility contract | KAH/KHS integration owners | Compatibility matrix and activation guidance |
| `docs/release-notes-template.md` | Release note template | KAH release owner | Template only |
| `docs/.omx/` if present | Tool/runtime/agent state | Tooling | Non-authoritative; never a KAH docs SOT |

## Status vocabulary

| Status | Meaning |
|---|---|
| `source of truth` | Confirmed current authority for its stated scope |
| `candidate SOT` | Proposed normative record pending confirmation and/or implementation evidence |
| `planned/candidate` | Roadmap or proposed command surface, not proven implemented |
| `historical` | Preserved context; not current authority by itself |
| `stale` | Known to conflict with newer evidence or decisions; preserve with marker rather than silently delete |
| `superseded` | Replaced by a named newer authority |

## Decision summary

- `.kkachi-workflow.yaml` is documented as planned project-level workflow graph state, not as implemented KAH behavior today.
- `.kkachi/config.yaml` remains helper runtime/configuration only.
- `.kkachi/runs/<run_id>/phase-plan.yaml` remains run-local execution state/evidence and is not deprecated.
- Kkachi v2 `.kkachi/config/workflows/` is outside KAH/KHS graph scope and must not be used as fallback graph authority.
- `kah graph` is planned/candidate shorthand unless KAH capabilities/help evidence proves the command or alias exists.

## Stale/conflict markers

- Older wording that treats `phase-plan.yaml` as the whole workflow SOT must be read narrowly as run-local execution state for one KHS run.
- Older `.kkachi-config.yaml` or `.kkachi-config.json` graph phrasing, if encountered, is superseded for this candidate plan by `.kkachi-workflow.yaml`.
- `docs/TODO-ALIGN.md` is deleted in the current working tree and must not be treated as an active roadmap authority.

## Open questions

- Exact `.kkachi-workflow.yaml` schema remains an outline until implementation tasks close.
- The real command name and alias policy for `kah graph` remain unimplemented; current binary evidence must be checked before use.
- Blue confirmation is still required before this Gray docs update becomes final project authority.

## Next record action

After Hwangchung review, either promote the workflow graph docs from candidate SOT to confirmed SOT, or mark the rejected/deferred portions with explicit reasons and successor paths.
