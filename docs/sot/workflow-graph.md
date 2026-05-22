# KAH workflow graph SOT

Date: 2026-05-21
Owner: KAH deterministic helper boundary
Confirming role: Responsible approver / governance evidence record
Status: source of truth for `.kkachi-workflow.yaml`; read-only validation/explanation implemented, mutation surfaces planned
Authority level: behavior authority for implemented read-only graph commands and planning authority for future graph mutation commands
Scope: KAH graph docs and read-only command behavior only; no KAB docs, profiles, registries, or gateway changes
Related docs: `../README.md`, `../specs.md`, `../roadmap.md`, `../compatibility.md`, KHS `docs/sot/workflow-graph-integration.md`
Evidence/source paths:
- Governance evidence record in kanban task `t_2fb00394`

## Decision summary

`.kkachi-workflow.yaml` is the project-level workflow graph instance for KAH/KHS coordination. KHS chooses workflow policy, templates, phase applicability, and proposal content. KAH currently validates and explains graph state read-only. Later roadmap tasks add diff, proposal records, approved writes/applies, and deterministic audit evidence. KAH does not decide project policy.

`phase-plan.yaml` remains run-local execution state/evidence for one KHS run. It may be instantiated from, constrained by, or checked against `.kkachi-workflow.yaml`, but it is not deprecated and is not a second project graph file.

## Scope

In scope:

- project-level `.kkachi-workflow.yaml` graph state;
- implemented read-only KAH graph validation/explanation plus planned/candidate diff, proposal, apply, and export behavior;
- source precedence and fail-closed rules;
- relationship between project graph state and run-local KHS phase state;
- KAH/KHS evidence requirements for future graph mutation.

Out of scope:

- KAB backend/session/plan runtime policy;
- Kkachi v2 `.kkachi/config/workflows/` runtime configuration;
- Hermes profile/runtime/gateway settings;
- graph mutation implementation work outside `graph-002`;
- KAH deciding phase policy, review policy, gate policy, backend choice, or external approval, risk review, and operator/product approval rules.

## File authority table

| Path / artifact | Meaning | Owner | Authority |
|---|---|---|---|
| `.kkachi-workflow.yaml` | Project-level workflow graph instance | KHS proposes policy/templates; KAH validates/explains read-only today; future tasks write/apply approved graph changes | Project graph file for implemented read-only validation/explanation; mutation authority requires later audit evidence |
| `.kkachi/config.yaml` | KAH helper runtime/configuration | KAH | Helper config only; never workflow graph SOT |
| `.kkachi/` | Runtime state, evidence, events, locks, schemas, run artifacts | KAH | Runtime/evidence substrate |
| `.kkachi/runs/<run_id>/phase-plan.yaml` | Run-local execution state/evidence for a KHS run | KHS content stored/validated by KAH | Run-local workflow/execution state; not project graph replacement |
| `.kkachi/config/workflows/` | Kkachi v2 workflow runtime config if present | Kkachi v2, not KAH/KHS graph docs | Out of KAH/KHS graph scope; no merge/fallback |
| Mermaid/PlantUML exports | Generated visualization | KAH export command | Non-authoritative artifact only |

## `.kkachi-workflow.yaml` and `phase-plan.yaml` relationship

```text
KHS template/policy/proposal
        |
        v
KAH implemented `kkachi-agent-helper graph validate/explain`; planned diff/propose/apply
        |
        v
project root `.kkachi-workflow.yaml` + KAH audit events
        |
        v
run-local `.kkachi/runs/<run_id>/phase-plan.yaml` + run artifacts/gates
```

Rules:

- `.kkachi-workflow.yaml` records project phases/nodes, edges/dependencies, gate requirements, approval requirements, owners, source references, and managed metadata.
- KHS selects or drafts graph policy through templates and declarative proposals.
- KAH validates and explains graph state deterministically today; future tasks write/apply approved graph state deterministically. It does not decide policy.
- `phase-plan.yaml` records run-local execution state/evidence for one run.
- If project graph, KHS phase policy, and run-local phase state conflict, KAH/KHS fail closed and require responsible role confirmation before work proceeds.

Read-only validation/explanation is implemented and advertised through capabilities/help. Graph mutation remains planned until later capability/help evidence exists.

## Kkachi v2 namespace collision

`.kkachi-workflow.yaml` is the KAH/KHS project-level workflow graph file. If a repository also contains Kkachi v2 `.kkachi/config/workflows/templates/*.json` or `.kkachi/config/workflows/addons/*.json`, those files belong to Kkachi v2 runtime orchestration and are outside KAH/KHS graph authority. KAH must not read them as fallback graph policy, merge them silently, or treat them as equivalent to `.kkachi-workflow.yaml`.

## `graph` command surface

Status: `kkachi-agent-helper graph validate` and `kkachi-agent-helper graph explain` are implemented read-only commands. Other graph commands remain planned/candidate. `kah graph` remains shorthand only; docs must not claim a `kah` shell alias exists until implementation evidence exists.

```text
kkachi-agent-helper graph validate [--file .kkachi-workflow.yaml] [--json]
kkachi-agent-helper graph explain [--file .kkachi-workflow.yaml] [--json]
kah graph init --from-template <template-id-or-path> [--output .kkachi-workflow.yaml] [--json]        # planned shorthand
kah graph diff --from <file-or-ref> --to <file-or-ref> [--semantic] [--json]                         # planned shorthand
kah graph propose --patch <patch-file> --reason <text> [--json]                                      # planned shorthand
kah graph apply --proposal <proposal-id> --approval <evidence-ref> [--json]                          # planned shorthand
kah graph export --format mermaid|plantuml [--output <path>] [--json]                                # planned shorthand
```

Do not document policy-setting surfaces as normal commands. Forbidden examples include workflow subcommands under the `kah` prefix, profile-driven graph initialization, gate-setting commands, review-policy setters, and graph policy setters.

Forbidden examples:

```text
kah workflow ...
kah graph init --profile ...
kah gate set ...
kah review-policy set ...
kah graph set-policy ...
```

## Command classification

| Command | Mutates graph? | Category | Policy mutation? |
|---|---:|---|---:|
| `init --from-template` | yes, only initial graph write or approved replacement | deterministic write from selected template | no |
| `validate` | no | implemented validation | no |
| `explain` | no | implemented operator-readable explanation | no |
| `diff` | no | semantic diff | no |
| `propose` | records proposal, does not apply graph | proposal record | no |
| `apply` | yes, after approval evidence | approval-gated deterministic apply | no |
| `export` | no graph mutation | visualization artifact generation | no |

Policy mutation category is empty. KAH validates and records state; KHS and responsible approvers own policy decisions.

## Source precedence and fail-closed rules

### Graph mutation input precedence

1. Explicit `kah graph apply --proposal <id>` with approval evidence.
2. Explicit `kah graph init --from-template <template-id-or-path>` only when no graph exists, or when replacing through an approved proposal.
3. Explicit `kkachi-agent-helper graph validate/explain --file <path>` for inspection only; inspection does not make the file authoritative. Future diff inspection remains planned.
4. Current `.kkachi-workflow.yaml` on disk only when schema-valid and not in conflict with last KAH audit/checksum evidence.
5. KHS template defaults are proposal/init inputs only, never silent overrides.
6. KAH built-in examples, if any, are examples only and not operational fallback defaults.

### Effective runtime/evidence precedence after graph support lands

1. Applied `.kkachi-workflow.yaml` whose checksum/version matches KAH graph audit evidence.
2. KAH graph proposal/apply/audit records proving how the graph changed.
3. Run-local `phase-plan.yaml` for execution state of a specific run.
4. `.kkachi/config.yaml` for helper config only.
5. Generated Mermaid/PlantUML diagrams for visualization only.

Fail closed when:

- graph-managed workflow is required but `.kkachi-workflow.yaml` is missing;
- `.kkachi-workflow.yaml` is invalid, ambiguous, duplicated, or conflicts with KHS phase policy or run-local `phase-plan.yaml`;
- `.kkachi/config.yaml`, generated diagrams, stale `.kkachi/` state, KHS defaults, or Kkachi v2 `.kkachi/config/workflows/` are used as fallback graph authority;
- direct manual edits lack validation/proposal/apply/audit evidence;
- graph patch changes gates, approvals, review policy, or dependencies without approval evidence;
- KHS asks for imperative KAH policy-setting commands.

## Proposal lifecycle

1. KHS, a responsible approver, or a human drafts a declarative graph patch or selects a KHS template.
2. KAH validates candidate input fail-closed.
3. KAH explains the current effective graph.
4. KAH produces a semantic diff.
5. KAH records a proposal; proposal id/path becomes evidence.
6. Required role approval is recorded by explicit evidence reference.
7. KAH applies the approved proposal atomically.
8. KAH records graph audit events and new checksum/version.
9. KHS stores relevant evidence in run artifacts when the change affects a run.

No direct YAML edit path is normal operation. If a human edits `.kkachi-workflow.yaml` directly, KAH must detect drift or validate it as unmanaged input and require repair/proposal/audit evidence before use.

## Schema v1 outline

This is an outline only, not final implementation schema.
The implemented read-only validator requires each phase to declare `required` explicitly and rejects duplicate top-level sections, duplicate fields, duplicate gate ids, and duplicate approval scopes. Its YAML subset is narrower than full YAML: list rows must use inline `- key: value` form, indentation is spaces-only, string lists are inline, scalars are plain or double-quoted, and anchors, aliases, block scalars, nested maps outside the documented shape, tab indentation, bare `-` list rows, and unquoted inline comments are rejected.

```yaml
version: "workflow-graph/v1"
graph_id: "<stable-project-graph-id>"
metadata:
  project: "<name>"
  created_by: "khs|human|kah"
  managed_by: "kah"
  source_template: "<template-id-or-null>"
  last_applied_event_id: "<event-id-or-null>"
phases:
  - id: "plan"
    title: "Plan"
    owner_layer: "khs"
    required: true
    evidence: ["plan.md"]
edges:
  - from: "plan"
    to: "ask"
gates:
  - id: "pre-implementation"
    requires: ["plan", "ask"]
approvals:
  - scope: "sot-change"
    required_role: "responsible-approver|required-reviewer|external-approver"
proposals:
  policy: "proposal-first"
```

## Human and JSON output expectations

Human output must show effective source, validation status, changed phases/edges/gates/review requirements, pending proposals, failure reason, and remediation/next action.

Required compact JSON fields:

| Command | Required fields |
|---|---|
| `validate --json` | `schema_version`, `status`, `file`, `checksum`, `effective_source`, `errors`, `warnings`, `conflicts`, `next_action` |
| `explain --json` | `schema_version`, `status`, `graph_version`, `effective_source`, `phases`, `edges`, `gates`, `approval_requirements`, `pending_proposals`, `validation_summary`, `next_action` |
| `diff --json` | `schema_version`, `status`, `from`, `to`, `changed_phases`, `changed_edges`, `changed_gates`, `changed_approvals`, `risk_flags`, `requires_approval`, `next_action` |
| `propose --json` | `schema_version`, `status`, `proposal_id`, `proposal_path`, `validation_summary`, `semantic_diff_ref`, `approval_required`, `next_action` |
| `apply --json` | `schema_version`, `status`, `proposal_id`, `approval_ref`, `graph_path`, `new_checksum`, `event_ids`, `next_action` |
| `export --json` | `schema_version`, `status`, `format`, `output_path`, `source_checksum`, `authoritative: false` |

## Mermaid / PlantUML scope

Mermaid and PlantUML exports are generated visualization artifacts only. They do not become graph policy, schema, or source of truth. A later export command must include source checksum and `authoritative: false` in JSON output. Examples may be shown only when labeled non-authoritative.

## Risk review closure coverage

| Required review item | Resolution in this planning SOT |
|---|---|
| MF-1 | `.kkachi-workflow.yaml` is project-level graph state; `phase-plan.yaml` remains run-local execution state/evidence and is not deprecated. |
| MF-2 | Kkachi v2 `.kkachi/config/workflows/` is outside KAH/KHS graph scope; no fallback, merge, or namespace sharing is implied. |
| MF-3 | `kkachi-agent-helper graph validate/explain` are implemented; `kah graph` remains planned/candidate shorthand until alias evidence exists. |
| MF-4 | Mutation input precedence, runtime/evidence precedence, and fail-closed rules are explicit above. |
| MF-5 | Command classification contains zero policy-mutation commands; KAH does not expose policy-setting graph commands in this planning SOT. |

## Stale/conflict markers

- Older wording that treats `phase-plan.yaml` as the full workflow SOT is narrowed to run-local execution state/evidence.
- Existing docs that use the actual binary name `kkachi-agent-helper` remain correct; `kah graph` wording is only planned/candidate shorthand until alias evidence exists.
- Prior root-level kkachi config YAML/JSON graph phrasing, if encountered, is superseded by `.kkachi-workflow.yaml` for this planning SOT.
- Prior role examples that used personal or internal codenames are superseded by generic role placeholders such as `responsible-approver`, `required-reviewer`, and `external-approver`.
- `docs/TODO-ALIGN.md` is not active roadmap authority in the current working tree.

## Open questions

- Event types, proposal storage path, mutation checksum policy, diff/apply/export behavior, and alias policy remain implementation tasks.
- Current implemented command evidence is limited to `kkachi-agent-helper graph validate` and `kkachi-agent-helper graph explain`.

## Next record action

Next implementation work is `graph-003`: semantic graph diff plus proposal record storage. Do not widen read-only `graph-002` into graph mutation.
