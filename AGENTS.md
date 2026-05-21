# AGENTS.md

Guidance for coding agents working in this repository.

`kkachi-agent-helper` is a deterministic local CLI helper for Kkachi project state, run artifacts, locks, schemas, events, diagnostics, gates, and project bootstrap scaffolding. It must stay local-first, scriptable, and small. It does not choose a backend, plan work, review code, call network services, install Hermes/KHS skill content, or store secrets.

## Source Of Truth

- Start with `docs/specs.md` for behavior, schema, and boundary contracts.
- Use `docs/roadmap.md` for delivery order, task IDs, and PR-sized scope.
- Use `docs/compatibility.md` for KHS/KAH compatibility expectations.
- Use `README.md` for user-facing command examples and supported workflows.
- Do not treat stale or deleted TODO files as authority when the roadmap or specs say they are stale.

## Language

- Use English by default for code, comments, docs, commit messages, test names, identifiers, branch names, artifact content, and internal reasoning artifacts.
- Use Korean only when reporting status, results, blockers, or summaries directly to the user, unless the user explicitly asks for another language.
- When using Korean to report to the user, use a polite tone and address the user as `마스터`.
- Preserve the existing language of user-facing product copy or quoted source text unless the task asks to change it.

## Think Before Coding

Do not assume. Do not hide confusion. Surface tradeoffs.

Before implementing:

- State assumptions explicitly when they affect the implementation.
- If multiple interpretations exist, present them instead of silently choosing.
- If a simpler approach exists, say so and prefer it.
- Push back when a requested change would move KAH beyond deterministic helper ownership.
- If something material is unclear, stop, name what is unclear, and ask.

For trivial, reversible, low-risk work, use judgment and proceed without turning this into ceremony.

## Simplicity First

Write the minimum code that solves the requested problem.

- Do not add features beyond what was asked.
- Do not add abstractions for single-use code.
- Do not add flexibility or configurability that is not required by the specs, roadmap task, or user request.
- Do not add error handling for impossible scenarios.
- If a change grows large and could be smaller, simplify before finishing.
- Prefer deletion and reuse over new layers.

KAH is intentionally deterministic. Keep reasoning, planning, backend choice, and review intelligence outside this helper.

## Surgical Changes

Touch only what the task requires.

- Do not improve adjacent code, comments, or formatting just because you see it.
- Do not refactor unrelated code.
- Match the existing Go, test, docs, and CLI-output style.
- If unrelated dead code or drift is found, mention it instead of deleting it.
- Remove imports, variables, files, and helpers made unused by your own changes.
- Do not remove pre-existing dead code unless explicitly asked.

Every changed line should trace directly to the user's request, the active roadmap task, or a required verification fix.

## Git And Worktree Safety

- Never revert existing user changes.
- Do not touch unrelated dirty files.
- Never run `git add`, `git commit`, or `git push`.

## Goal-Driven Execution

Turn tasks into verifiable goals.

For multi-step work, use a brief plan shaped like:

```text
1. [Step] -> verify: [check]
2. [Step] -> verify: [check]
3. [Step] -> verify: [check]
```

Examples:

- "Add validation" means add tests for invalid inputs, then make them pass.
- "Fix the bug" means reproduce it with a focused test, then make it pass.
- "Refactor X" means confirm behavior before and after the refactor.

Loop until the stated success criteria are proven or a real blocker remains.

## Project Boundaries

Preserve these KAH boundaries:

- KAH owns deterministic local state, artifacts, schemas, gates, events, locks, diagnostics, command-surface validation, and project bootstrap state.
- KHS owns workflow policy, phase applicability, checklist normalization, backend-use decisions, semantic guidance, and release recommendations.
- KAB owns backend-native discovery, bridge sessions, and external CLI runtime integration.
- KAH must fail closed for missing required evidence, invalid schemas, unsafe paths, lock conflicts, unsupported versions, ambiguous run resolution, and stale status/event coherence.
- KAH must not mutate user source files except explicitly managed Kkachi scaffold, state, evidence, schema, or diagnostic files.
- KAH must not require network access for the default local workflow.

Keep examples local and secret-free. Never place tokens, bearer headers, API keys, passwords, production paths, or private bridge payloads in `.kkachi/` files, diagnostics bundles, release notes, docs examples, fixtures, or test output.

## Implementation Rules

- Prefer existing package boundaries and helpers before introducing new ones.
- Keep CLI behavior deterministic with stable human output, JSON output, and exit behavior.
- Preserve append-only event semantics unless a spec-backed migration says otherwise.
- Use atomic writes and path-safety helpers for state mutation paths.
- Keep schema-owned JSON fields schema-owned; do not overwrite semantic status fields through generic artifact lifecycle status.
- Do not add dependencies without an explicit request or a spec-backed need.
- Keep docs updates in sync with command behavior when user-facing contracts change.

## Docs Sync

- When CLI behavior, JSON schemas, gate contracts, or artifact contracts change, update the affected parts of `README.md`, `docs/specs.md`, and `docs/compatibility.md`.
- Even for docs-only changes, do not create or preserve stale source-of-truth content.

## Verification

Run the narrowest meaningful verification first, then broaden when risk or shared contracts justify it.

Before completing any task, always run `make test-prepare` so formatting, `go vet`, and lint checks have run. If `make test-prepare` fails, fix the failure and rerun it until it passes before claiming the task is complete.

## Testing Scope

- The default verification floor is `make test-prepare`.
- If CLI behavior, schemas, gates, artifacts, or locks change, also run the relevant unit, integration, or E2E tests.
- If the public command surface changes, include help, capabilities, and docs verification.

Common commands:

```sh
make test-prepare
make test-unit
make test-int
make test-e2e
make test
make check
```

- `make test-prepare` runs formatting and static checks.
- `make test-unit` runs package and unit tests.
- `make test-int` runs integration-tagged tests.
- `make test-e2e` runs local black-box CLI scenarios.
- `make check` is the full build and verification lane.

Before claiming completion, report what changed, what verification ran, and any known remaining risk or untested gap.
