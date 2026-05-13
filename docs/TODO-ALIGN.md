# TODO-ALIGN — KHS/KAH Integration Alignment

Date: 2026-05-12
Status: Source of truth for the `align` roadmap epic
Roadmap epic: `align — KHS/KAH integration alignment`
Related projects: `kkachi-hermes-skills` (KHS), `kkachi-agent-bridge` (KAB)

## Purpose

This document is the task-level SOT for the KAH `align` epic. `docs/roadmap.md` tracks the task ids, delivery order, and status; this file owns the detailed scope, acceptance criteria, and implementation notes for the eight `align-*` tasks.

The goal is to let KHS consume KAH `@latest` safely while preserving ownership boundaries:

- KAB owns backend session control and backend plan/runtime evidence.
- KHS owns workflow policy, phase applicability, and normalized `checklist.md` generation.
- KAH owns deterministic state, artifacts, schemas, gates, events, locks, diagnostics, and command-surface compatibility checks.

## Boundary contract

KAH must not:

- decide whether a user request should trigger KHS;
- choose backend lanes or external AI CLIs;
- parse KAB planner intent semantically;
- require KAB-specific planner sections such as `KHS Checklist Seed`;
- decide phase applicability from task semantics;
- plan or implement software changes;
- install Hermes skills;
- judge commander reasoning beyond deterministic artifact validation.

KAH may:

- store and validate KHS-declared state;
- enforce artifact and gate completeness;
- fail closed when declared requirements are missing;
- validate artifact shape, required fields, status markers, and evidence links;
- expose command-surface capabilities for KHS `@latest` compatibility checks;
- include phase-plan, approvals, backend evidence, and artifact status in diagnostics.

## Delivery order

1. `align-001` — Plan/checklist ownership contract
2. `align-002` — Declared backend evidence requirement
3. `align-003` — Command-surface capabilities report
4. `align-004` — Standard help UX
5. `align-005` — Phase-plan validation and diagnostics
6. `align-006` — Deterministic artifact mutation commands
7. `align-007` — Approval record surface
8. `align-008` — KHS/KAH compatibility contract docs

## Task details

### align-001 — Plan/checklist ownership contract

Status: Completed

#### Problem

KHS treats `plan.md` and `checklist.md` as mandatory pre-implementation artifacts. KHS should recover from a missing or malformed KAB `KHS Checklist Seed` when the plan text is usable by deriving a normalized `checklist.md` from plan text, task contract, acceptance criteria, phase contract, expected evidence, and gate requirements.

KAH should validate the artifacts KHS writes, not the original KAB planner format.

#### Scope

- Document that KHS owns `checklist.md` normalization.
- Document that KAH does not parse or require KAB-specific planner sections such as `KHS Checklist Seed`.
- Protect the existing `plan` gate contract: completed `acceptance-criteria.md`, `plan.md`, and `checklist.md` are required.
- Keep KAH validation deterministic: mandatory artifacts must exist, be non-empty, and be marked complete.

#### Acceptance criteria

- KAH docs state that KHS owns checklist normalization.
- `gate check <run_id> plan` requires completed `acceptance-criteria.md`, `plan.md`, and `checklist.md`.
- Tests cover missing, empty, incomplete, and complete plan artifacts.
- No implementation depends on KAB-specific planner text sections.
- KHS can rely on this contract across compatible KAH `@latest` updates.

#### Completion notes

- `docs/specs.md` and `docs/compatibility.md` now state that KHS owns normalized `checklist.md` generation while KAH validates completed artifacts only.
- Unit regressions cover missing, empty, pending, and complete plan artifacts, plus `plan.md` cases with and without `KHS Checklist Seed`.
- No KAH command parses or requires KAB-specific planner seed sections for the `plan` gate.

### align-002 — Declared backend evidence requirement

Status: Planned

#### Problem

KAH's backend gate is manifest-driven, which is correct, but backend evidence is currently tied too closely to execution mode. KHS can run KAB-backed `production_write` work, where backend evidence must be required even though `execution_mode` remains `production_write`.

Execution mode and KAB usage must be separate concepts.

#### Scope

- Add an explicit KHS-declared backend evidence requirement, for example `backend_evidence: required` in run metadata and/or `run create ... --backend-evidence required`.
- Preserve direct/non-KAB runs where backend evidence is not applicable.
- When backend evidence is required, include canonical backend artifacts in `required_artifacts`:
  - `selected-cli.json`
  - `capability-check.md`
  - `bridge-session-snapshot.json`
  - `bridge-events.md`
- Make the backend gate fail closed until required backend artifacts are complete and valid.

#### Acceptance criteria

- A KAB-backed `production_write` run can explicitly require backend evidence.
- A direct/non-KAB `production_write` run can keep backend evidence not applicable.
- `artifact init` records required backend artifacts when the run requires them.
- `gate check <run_id> backend` fails closed when required backend evidence is missing or incomplete.
- KAH does not choose or override the backend; it validates declared artifact shape and completion only.

### align-003 — Command-surface capabilities report

Status: Planned

#### Problem

KHS `main` should use KAH `@latest` where possible, but it needs capability-based compatibility checks rather than fragile patch-version pinning.

#### Scope

- Add `kkachi-agent-helper capabilities --json`.
- Include helper version and project schema version.
- Include stable booleans/statuses for command groups and compatibility-relevant features.
- Make missing, deprecated, optional, and intentionally omitted surfaces explicit, including the removed `install` command.

#### Acceptance criteria

- `capabilities --json` exits `0` on a healthy binary.
- Output includes helper version and project schema version.
- Output lets KHS determine whether project init, run lifecycle, artifact init/list/validate, gates, backend evidence requirements, phase-plan support, approval records, diagnostics, and omitted install behavior are available.
- Output is stable enough for KHS activation checks.

### align-004 — Standard help UX

Status: Planned

#### Problem

Current command discovery is awkward for humans and automation because common help invocations return usage errors.

#### Scope

Support stable help output for:

```text
kkachi-agent-helper help
kkachi-agent-helper --help
kkachi-agent-helper project --help
kkachi-agent-helper project init --help
kkachi-agent-helper run --help
kkachi-agent-helper run create --help
kkachi-agent-helper artifact --help
kkachi-agent-helper gate --help
kkachi-agent-helper diagnostics --help
kkachi-agent-helper phase-plan --help
```

#### Acceptance criteria

- Help commands exit `0`.
- Help output lists required arguments and options.
- JSON mode behavior is documented; structured JSON help is preferred, but a clear documented non-JSON behavior is acceptable for the first PR.
- Existing command errors remain structured and deterministic.

### align-005 — Phase-plan validation and diagnostics

Status: Planned

#### Problem

KHS treats `phase-plan.yaml` as the workflow source of truth for a run. KAH metadata such as `work_path`, `work_mode`, and `execution_mode` remains helper classification metadata and must not decide which KHS phases execute.

KHS can write `phase-plan.yaml` directly today, but KAH lacks a deterministic command surface to validate, report, or include that state in diagnostics.

#### Scope

- Add deterministic support for KHS-declared phase plans via commands such as `phase-plan init`, `phase-plan show`, `phase-plan set`, and `phase-plan validate`, or an equivalent KAH-managed representation.
- Validate declared structure and completeness only.
- Require explicit reasons for skipped/not-applicable phases.
- Include phase-plan evidence in diagnostics export.

#### Deterministic validations

- Required phase rows are present.
- Skipped or not-applicable phase rows include reasons.
- Feedback rounds stay within the configured range, currently 1 to 3.
- Code-change runs include optimize evidence or an explicit skip reason.
- `ask`, `request-feedback-1`, and `handle-feedback-1` are represented even when they produce no actionable question or feedback.
- Final verification confirms required phase rows have terminal states and evidence links.

#### Acceptance criteria

- KHS can initialize and update phase state through KAH without direct mutation of helper-managed metadata.
- KAH validates declared phase-plan structure and completeness deterministically.
- KAH does not choose phases, reorder phases intelligently, choose backends, or infer user intent.
- Diagnostics export includes `phase-plan.yaml` or its KAH-managed equivalent.

### align-006 — Deterministic artifact mutation commands

Status: Planned

#### Problem

KHS currently writes major artifacts directly. Direct writes work, but they bypass KAH-controlled safe path enforcement, audit events, atomic writes, and artifact status updates.

This matters most for pre-start preservation of `plan.md` and repeated updates to `checklist.md`, `phase-plan.yaml`, and backend event artifacts.

#### Scope

Add deterministic artifact mutation commands such as:

```bash
kkachi-agent-helper artifact write <run_id> plan.md --from <file> --json
kkachi-agent-helper artifact write <run_id> checklist.md --from <file> --json
kkachi-agent-helper artifact append <run_id> bridge-events.md --from <file> --json
kkachi-agent-helper artifact set-status <run_id> checklist.md --status complete --reason <reason> --json
```

#### Acceptance criteria

- Artifact write commands refuse unsafe paths and unknown unmanaged locations.
- Writes are atomic or fail closed.
- KAH records an event for artifact write/update operations.
- KAH can distinguish canonical artifacts from KHS supplemental artifacts.
- Existing direct file compatibility remains possible during migration.

### align-007 — Approval record surface

Status: Planned

#### Problem

KHS may auto-start low-risk work, but high-risk or ambiguous cases require explicit master approval. KHS can record this manually today, but KAH-native approval records make diagnostics and final verification cleaner.

#### Scope

Add deterministic approval commands or a documented strict event schema, for example:

```bash
kkachi-agent-helper approval request <run_id> --phase implement --reason <reason> --json
kkachi-agent-helper approval record <run_id> --phase implement --decision approved --by master --evidence <artifact-or-message-ref> --json
kkachi-agent-helper approval show <run_id> --json
```

A strict wrapper around `event append` is acceptable if KAH should avoid a larger top-level command surface initially.

#### Acceptance criteria

- KHS can record approval requests and decisions with phase, reason, decision, approver, timestamp, and evidence reference.
- Approval records are included in diagnostics export.
- KAH does not decide whether approval is needed; it records the declaration and decision.
- Final verification can check approvals when the phase plan says they were required.

### align-008 — KHS/KAH compatibility contract docs

Status: Planned

#### Problem

After the compatibility surfaces land, README/specs/compatibility docs must clearly state the KHS/KAH contract so KHS can use KAH `@latest` while preserving reproducibility through tested/recommended versions.

#### Scope

Document that:

- KAH does not install Hermes skills.
- KAH does not decide whether KHS should trigger.
- KAH project bootstrap is through `project init` / `project init --force`.
- KHS `main` may verify KAH by `@latest` plus command-surface capabilities.
- KHS release tags may publish tested/recommended KAH versions for reproducibility.
- KAH owns deterministic state only after KHS or the user chooses to apply the Kkachi workflow.
- KAH may validate KHS-declared `phase-plan.yaml` but must not decide phase applicability intelligently.

#### Acceptance criteria

- README/specs/compatibility docs describe the KHS/KAH boundary consistently.
- Docs reference `capabilities --json` as the preferred KHS compatibility check once implemented.
- Docs preserve the `project init` bootstrap contract and no-install-command boundary.
- Docs do not promote KAH into workflow-policy, planner, backend-selection, or Hermes-skill-install ownership.

## Verification expectations

Each task should include tests at the lowest effective level:

- Unit tests for deterministic validators and manifest/status changes.
- CLI tests for command parsing, JSON output, and exit codes.
- Integration/e2e coverage when behavior crosses command boundaries or run artifact state.
- Docs/spec updates for every public command or compatibility contract change.

Before marking any `align-*` task complete, verify the changed behavior with relevant local commands and update both this file and `docs/roadmap.md` status/notes.
