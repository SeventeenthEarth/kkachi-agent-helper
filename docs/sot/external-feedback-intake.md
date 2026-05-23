# Configurable external feedback intake planning SOT

Date: 2026-05-24
Owner: KAH documentation and implementation planning
Confirming role: Responsible technical and risk reviewers before implementation
Status: planning SOT
Authority level: planning authority; not implementation evidence
Scope: KAH-only configurable external feedback intake support
Related docs: `docs/specs.md`, `docs/roadmap.md`, `docs/compatibility.md`

## Purpose

This document is the planning source of truth for replacing the stale fixed feedback range with graph-declared `EXTERNAL_FEEDBACK_INTAKE` bounds.

The intended policy is:

- `min_rounds=1`.
- `max_rounds=5`.
- Round 1 is required.
- Rounds 2 through 5 are optional continuation rounds.
- Five loops are never mandatory.

This document does not claim implemented support. KAH may advertise configurable external feedback intake support only after `graph-009` graph schema handling, `graph-010` phase-plan validation, and `graph-011` migration/proposal handling, diagnostics/capabilities, permanent docs, and tests are aligned.

## Authority model

`.kkachi-workflow.yaml` is the project workflow graph and policy source after KAH validation and audit evidence confirm it. It may declare `EXTERNAL_FEEDBACK_INTAKE` bounds for KHS to consume when KHS chooses or generates run-local workflow state.

`.kkachi/config.yaml` is helper runtime configuration only. It must not be used as fallback authority for feedback bounds.

`.kkachi/runs/<run_id>/phase-plan.yaml` is run-local execution state. It records the realized feedback phase rows for a specific run after KHS or the operator has selected policy for that run.

Run-local evidence belongs in phase-plan rows, KAH events, canonical feedback artifacts, checklist/report output, and diagnostics. Live counters such as current round, performed count, remaining rounds, current feedback availability, or per-run continuation decisions must not be stored as project graph policy.

If graph policy, helper config, run-local phase plan, checklist, events, reports, or feedback artifacts conflict, KAH must fail closed and report the authoritative graph source, the conflicting artifact path, the stale or unsupported value, and a repair or confirmation action.

## graph-008: Docs contract and roadmap

### Goal

Lock the normative contract before code changes so later implementation can stay narrow and reviewable.

### Do

- Update `docs/specs.md` with the planned feedback intake graph policy contract, authority model, fail-closed behavior, and run-local artifact expectations.
- Update `docs/compatibility.md` with KHS/KAH compatibility expectations for configurable external feedback intake.
- Update `docs/roadmap.md` with task rows or notes for the remaining implementation slices.
- Cross-reference this SOT from permanent docs where it helps future implementers find the plan.
- Mark stale fixed `1..3` wording as stale or planned-to-change when it is still describing current implemented behavior.

### Do not

- Do not edit Go code, embedded schemas, command behavior, capabilities, diagnostics, or generated templates.
- Do not claim configurable feedback intake is implemented.
- Do not describe `kah graph` shorthand as implemented unless the binary advertises that alias through help/capabilities evidence.
- Do not modify KHS templates, KHS registries, KAB runtime behavior, or backend discovery behavior.

### Approach

Use `docs/specs.md` as the behavior SOT and `docs/compatibility.md` as the activation/fallback SOT. Keep `graph-008` docs explicit that fixed `1..3` validation remains the current implementation until later graph tasks change it.

### Implementation notes

- Prefer a compact specs addition over duplicating this entire planning document.
- Preserve existing graph boundaries: KHS chooses workflow policy; KAH validates declared graph and run-local state.
- Make all command names match current evidence: use `kkachi-agent-helper graph ...` for implemented command surfaces and reserve `kah graph ...` for planned/candidate wording.

### Checklist

- [x] `docs/specs.md` records intended `min_rounds=1`, `max_rounds=5`, required round 1, and optional rounds 2..5.
- [x] `docs/specs.md` states that current implementation may still reject round 4 until `graph-010` lands.
- [x] `docs/compatibility.md` states KHS must fail closed until KAH advertises final support.
- [x] `docs/roadmap.md` lists `graph-008`, `graph-009`, `graph-010`, and `graph-011` as reviewable task candidates.
- [x] Stale `1..3` references are either preserved as current implementation facts or marked as planned-to-change.
- [x] KHS/KAH/KAB ownership boundaries remain explicit.
- [x] `kah graph` remains candidate unless binary evidence proves otherwise.

### Acceptance criteria

`graph-008` is complete when the docs explain the intended contract without overclaiming implementation, and `graph-009` through `graph-011` can be executed without reopening basic authority decisions.

## graph-009: Graph schema and read-only validation

### Goal

Make graph validation understand feedback bounds without advertising full end-to-end support.

### Do

- Extend the workflow graph parser/schema validation to recognize schema-owned `EXTERNAL_FEEDBACK_INTAKE` bounds.
- Validate `min_rounds`, `max_rounds`, required round declarations, optional continuation bounds, schema/version markers, and policy identity.
- Reject missing required fields, unknown or unsupported schema versions, duplicate declarations, conflicting declarations, stale fixed `1..3` or `max3` assumptions, and round 6 or higher.
- Include feedback-bound information in read-only `graph validate`, `graph explain`, and semantic `graph diff` output where appropriate.
- Add focused unit and CLI/integration tests for graph validation and projection behavior.

### Do not

- Do not generate run-local phase-plan rows.
- Do not change phase-plan validation yet except where tests need fixtures for graph-only behavior.
- Do not mutate graph state outside the existing proposal/apply paths.
- Do not enable final configurable feedback intake capability flags.
- Do not invent defaults when bounds are absent.

### Approach

Add graph support as a read-only schema/projection layer first. The graph can report whether a candidate policy is valid, stale, missing, or conflicting, but KAH should still fail closed for consumers that require full feedback-intake support until later graph tasks land.

### Implementation notes

- Keep deterministic problem codes and human/JSON output stable.
- Preserve unknown fields only when needed for migration diagnostics; unknown fields must never make validation pass.
- Add semantic diff risk flags when feedback bounds change, but leave migration/apply policy completion to `graph-011`.
- Keep `.kkachi/config.yaml`, generated diagrams, stale `.kkachi/` state, KHS defaults, Kkachi v2 workflow config, and KAB runtime state out of graph authority.

### Checklist

- [ ] `min_rounds >= 1` is validated.
- [ ] `max_rounds >= min_rounds` is validated.
- [ ] Current intended `max_rounds <= 5` is validated.
- [ ] Round 1 is required for the intended policy.
- [ ] Rounds 2 through 5 are represented as optional continuation, not mandatory.
- [ ] Round 6 and higher fail closed.
- [ ] Missing bounds fail closed when graph-managed feedback bounds are required.
- [ ] Unknown or unsupported schema versions fail closed.
- [ ] Duplicate and conflicting declarations fail closed.
- [ ] Stale `1..3` or `max3` declarations are reported as stale, not silently accepted.
- [ ] `graph validate/explain/diff` tests cover min1/max5, stale max3, missing bounds, unknown versions, duplicate declarations, conflicting declarations, and round 6 rejection.

### Acceptance criteria

`graph-009` is complete when graph read-only surfaces can validate and explain feedback bounds deterministically, while capabilities and compatibility docs still make clear that full support is not advertised.

## graph-010: Phase-plan generation and validation

### Goal

Replace fixed `1..3` phase-plan validation with policy-driven bounds while preserving KHS ownership of phase applicability and optional continuation decisions.

### Do

- Update `internal/project/phase_plan.go` so feedback round validation is driven by graph/policy bounds when available.
- Preserve paired `request-feedback-N` and `handle-feedback-N` validation.
- Require round 1 when the effective policy requires `min_rounds=1`.
- Allow rounds 2 through 5 only when declared for the run.
- Reject round 6 and higher.
- Keep final validation checks for terminal status, evidence links, skipped/not-applicable reasons, and approval records.
- Add tests for phase-plan bounds, pairing, final validation, and graph-vs-run conflict cases.

### Do not

- Do not store live counters in `.kkachi-workflow.yaml`.
- Do not infer whether external feedback is useful or available.
- Do not make optional rounds mandatory.
- Do not let `.kkachi/config.yaml`, generated diagrams, KHS defaults, stale runtime state, Kkachi v2 workflow config, or KAB runtime state serve as fallback policy.
- Do not widen phase-plan behavior into KHS checklist wording or template semantics.

### Approach

Keep phase-plan as run-local declared state. KAH validates the rows KHS or the operator declared; it does not choose whether optional continuation rounds should exist. When graph policy and run-local rows disagree, KAH fails closed with precise conflict evidence.

### Implementation notes

- Round 1 remains the minimum required pair for the intended policy.
- Optional rounds may be absent without failure unless KHS declared them for that run or a final gate requires explicit skipped/not-applicable rows.
- If optional rows are predeclared, skipped/not-applicable rows require non-empty reasons.
- Completed feedback rows must keep evidence links scoped to the matching round.
- Event payloads and reports should avoid stale `1..3` language once `graph-010` changes runtime behavior.

### Checklist

- [ ] `request-feedback-4` and `handle-feedback-4` can pass when declared under a valid max5 policy.
- [ ] `request-feedback-6` or `handle-feedback-6` fails.
- [ ] Missing required round 1 fails when graph policy requires it.
- [ ] Unpaired request/handle rows fail.
- [ ] Optional rounds 2 through 5 are not required by default.
- [ ] Skipped/not-applicable optional rows require reasons when declared.
- [ ] Completed rows require evidence links under final validation.
- [ ] Stale checklist/report/event claims such as `max3`, `1..3`, or round 4 out-of-range conflict with graph max5 and fail closed.

### Acceptance criteria

`graph-010` is complete when run-local phase plans validate against effective feedback bounds, paired rows remain enforced, optional rounds remain optional, and stale graph-vs-run claims fail closed with actionable diagnostics.

## graph-011: Migration, diagnostics, capabilities, and final docs

### Goal

Make configurable external feedback intake consumable by KHS only after schema, command, phase-plan, tests, diagnostics/capabilities, and permanent docs are aligned.

### Do

- Add graph diff/proposal risk flags for feedback-bound changes where not already complete.
- Add migration/proposal support for known stale `1..3` or `max3` assumptions when a validated candidate graph declares `min_rounds=1` and `max_rounds=5`.
- Add diagnostics evidence for effective feedback bounds, stale/missing/invalid states, forbidden fallback sources, and graph-vs-run conflicts.
- Add the final compatibility/capability flag that KHS can use for fail-closed activation.
- Update README, `docs/specs.md`, `docs/compatibility.md`, and docs-contract tests to state implemented support only after the implementation is complete.
- Add CLI/integration/E2E coverage for final support behavior.

### Do not

- Do not treat `diagnostics export` failure as the normal missing-graph signal; keep graph compatibility diagnostics support-safe.
- Do not advertise support before all required behavior and tests are present.
- Do not silently accept stale generated artifacts.
- Do not use direct YAML editing as the normal migration path.
- Do not include KHS template or registry changes in this KAH task.

### Approach

Finish the support loop last: migration evidence, diagnostics, capabilities, and docs should advertise only what the code can prove. KHS activation must rely on `capabilities --json` and diagnostics, not command help text or partial implementation.

### Implementation notes

- Capabilities JSON is the machine contract; help text is supplemental.
- `graph_compatibility` should report missing or invalid graph state inside diagnostics without making diagnostics export fail for that reason.
- Proposal/apply checks must remain checksum and approval/audit evidence driven.
- Migration from stale assumptions must require explicit proposal or audit evidence and must not silently repair user state.

### Checklist

- [ ] Migration reports stale `1..3` or `max3` before repair.
- [ ] Migration requires explicit graph proposal or audit evidence.
- [ ] Proposal/diff output flags feedback-bound changes as risk-bearing.
- [ ] Diagnostics report effective min/max bounds when valid.
- [ ] Diagnostics report missing, invalid, stale, unsupported, or conflicting bounds without treating those reports as diagnostics export failures.
- [ ] `capabilities --json` includes the final configurable feedback intake support flag.
- [ ] README, specs, compatibility docs, and roadmap are aligned with implemented behavior.
- [ ] Docs-contract tests prove public docs do not overclaim support.
- [ ] Focused unit, integration, E2E, `go vet ./...`, and `make test-prepare` pass.

### Acceptance criteria

`graph-011` is complete when KHS can reliably detect configurable external feedback intake support through capabilities and diagnostics, stale projects require explicit migration/proposal evidence, and permanent docs match tested behavior.

## Global non-goals

- No KHS template, registry, skill, or checklist wording edits in this KAH implementation plan.
- No KAB runtime implementation, backend transport, or backend discovery behavior.
- No direct YAML fallback as a normal path.
- No hidden defaults when feedback bounds are absent, stale, unknown, unsupported, or conflicting.
- No mandatory five-loop workflow.
- No project graph live counters.
- No product concept that depends on named agents or color-team authority.

## Final support rule

KAH may advertise configurable external feedback intake support only after `graph-011` completes. Before that point, intermediate graph tasks may add docs, validation, or internal behavior, but KHS activation must fail closed unless the effective binary advertises the final support flag and diagnostics prove the required graph/run state.

## Verification guidance

For the SOT creation task:

- Run `git diff --check`.
- Run a docs grep to confirm `docs/sot/external-feedback-intake.md` is discoverable from `docs/README.md`.

For implementation graph tasks:

- Run focused graph and phase-plan tests before broader integration tests.
- Run integration and E2E tests when public command surfaces, diagnostics, capabilities, docs-contract behavior, or user-visible output changes.
- Run `make test-prepare` before claiming final completion for any implementation graph task.
