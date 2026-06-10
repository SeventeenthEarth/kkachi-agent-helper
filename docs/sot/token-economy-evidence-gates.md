# KAH token-economy evidence gates SOT

Date: 2026-06-09
Owner: KAH deterministic helper layer
Confirming role: Red `t_ba846dc4`, Orange `t_6d420a08`, and Gray `t_f5896baa` planning review accepted the original token-001 scope with no blocking findings; TOKEN-007 through TOKEN-010 / token-002 extension review accepted by Red `t_8726167f`, Orange `t_c037afad`, and focused Gray re-review `t_148f5ff0` after Gray `t_32211704` requested traceability fixes
Status: accepted SOT for the KAH side of KAS token-economy work; token-001 deterministic evidence gate is implemented; no release, KAS install/update/uninstall, KAB activation, Hermes runtime change, profile mutation, or auth/token/gateway/provider/model mutation is authorized by this document alone
Authority level: accepted planning source of truth for future KAH mechanical evidence gates supporting KAS token-economy and English-output work
Scope: `kkachi-agent-helper` roadmap, specs, schemas, gate/artifact/diagnostics planning, and future deterministic tests. KAS owns workflow policy, prompt contracts, lifecycle semantics, and operator-facing language; KAB owns backend bridge/session control; Hermes runtime remains stock.
Related docs: `docs/README.md`, `docs/roadmap.md`, `docs/specs.md`, `docs/compatibility.md`, KAS `docs/sot/token-economy-and-agent-instruction-contract.md`
Evidence/source paths: KAS accepted SOT `kkachi-hermes-skills/docs/sot/token-economy-and-agent-instruction-contract.md`; KAS roadmap TOKEN-001 completed by commit `26b97dc`; 주군 direction on 2026-06-09 to register the KAH development item and create this KAH-side SOT before TOKEN-002 development; Red review `t_ba846dc4` ACCEPT; Orange review `t_6d420a08` ACCEPT; Gray review `t_f5896baa` ACCEPT; TOKEN-007 through TOKEN-010 / token-002 extension review Red `t_8726167f` ACCEPT, Orange `t_c037afad` ACCEPT, Gray `t_32211704` REQUEST_CHANGES on acceptance/evidence traceability resolved by focused Gray re-review `t_148f5ff0` ACCEPT.

## 1. Decision summary

KAH needs two bounded development items for the KAS token-economy workstream. The first adds deterministic evidence support for compact English output, artifact-first reporting, repo-local agent-instruction evidence, and project KAS lifecycle evidence. The second adds deterministic evidence support for KAS verification profiles, no-agent runner artifacts, reversible evidence summaries, compact review bundles, no-agent fan-in watcher terminal reports, and change-aware verification matrix evidence.

Both KAH items are dependent on KAS producing stable evidence shapes. KAH must not become the policy owner or prose judge. Its future implementation must report only machine-checkable `pass`, `fail`, or `not_applicable` outcomes.

## 2. Layer boundary

| Layer | Owns for this workstream | Must not own |
|---|---|---|
| KAS | Token-economy policy, English product-output contract, prompt/template/lifecycle semantics, project-specific KAS install/update/uninstall planning | KAH gate internals or KAB runtime session control |
| KAH | Deterministic artifact existence, schema, marker, approval-reference, checksum/path, and gate evidence checks | Policy judgment, subjective language-quality scoring, summarization, warning-only advisory states, KAS lifecycle management |
| KAB | Backend bridge/session/control evidence when selected by KAS policy | KAS policy decisions or KAH validation semantics |
| Hermes runtime | Stock profile/tool/session substrate | Runtime fork or context-pruning requirement for this workstream |

## 3. Future KAH gate candidates

The future KAH PRs should add bounded deterministic evidence surfaces. The exact command shape may be implemented as built-in gates, graph-declared checks, diagnostics evidence, or small schema/artifact additions, but they must satisfy the following constraints.

### 3.1 Required mechanical checks for `token-001` when in scope

KAH may check that required artifacts or fields exist and match deterministic values, including:

- task class is recorded, or a `not_applicable` reason exists;
- compact English output policy acknowledgment exists in a phase plan, prompt artifact, or accepted run artifact;
- detailed output is referenced by artifact path when details are required;
- `AGENTS.md` and/or `CLAUDE.md` evidence exists, or a `not_applicable` reason is recorded;
- KAS managed block markers, or accepted no-marker migration evidence, exist when agent-instruction management is in scope;
- final report uses an approved compact-summary/artifact-pointer shape, without KAH judging prose quality;
- project KAS lifecycle work records dry-run evidence, approval/apply reference, profile/role labels, target paths, manifest/checksum fields, backup paths, lifecycle verb, and doctor verdict when lifecycle work is in scope;
- explicit approval evidence exists before any claim of broad KAB, Hermes runtime, profile, gateway, provider, model, auth, or token mutation.

### 3.2 Required mechanical checks for `token-002` when in scope

KAH may check that required artifacts or fields exist and match deterministic values, including:

- selected verification profile id, selected gate id, command, timeout, applicability, exit code, duration, log path, checksum, and status;
- no-agent runner full log artifact references and bounded failure excerpt fields when command execution fails;
- compact phase packet schema fields such as run id, task id, phase, status, changed paths, verification summaries, blockers, artifact paths, checksums, and next phase/action;
- artifact-first retrieval evidence, including referenced path existence and checksum match where checksums are declared;
- required markers or artifact references for compression-forbidden evidence classes declared by KAS;
- compact review bundle fields for task id, run id, acceptance reference, diff artifact/checksum, changed paths, verification summaries, forbidden scope, requested verdict, role verdicts, and finding dispositions;
- no-agent watcher terminal reports with compact terminal status and artifact pointers when watcher evidence is in scope;
- changed-path classification evidence with rule id, changed paths, selected verification, scoped/skipped verification, skip reason, and final aggregate preservation status.

KAH must not choose verification profiles, decide whether verification may be skipped, judge review quality, summarize logs, operate watcher policy, replace Kanban/color review evidence, or activate KAB/Hermes runtime behavior.

### Required result vocabulary

KAH results for this workstream are restricted to:

- `pass` — all required deterministic evidence is present and valid;
- `fail` — required deterministic evidence is missing, malformed, unsafe, stale, or contradictory;
- `not_applicable` — the task class or phase plan states this evidence is out of scope and records a deterministic reason.

Warning-only states are out of scope. If evidence matters to a gate, absence or invalidity must fail closed. If evidence does not matter to the current task, it must be `not_applicable` with a reason.

## 4. Explicit non-goals

This KAH workstream must not:

- parse or score natural-language quality;
- decide whether output is “good English” beyond deterministic marker/schema checks;
- summarize backend output, test logs, watcher output, or KAS prompts;
- install, update, repair, or uninstall KAS skills;
- mutate Hermes profiles, gateways, providers, models, tokens, auth, or runtime configuration;
- choose or activate KAB backends;
- replace KAS policy, color review, or 주군 approvals;
- introduce hidden fallback from missing KAS evidence to direct YAML/file edits.

## 5. Acceptance criteria for the KAH development items

A future implementation PR is not complete until it provides:

1. an implemented deterministic evidence surface documented in `docs/specs.md` and `docs/compatibility.md` if it becomes release-facing;
2. fixture or unit coverage for `pass`, `fail`, and `not_applicable` cases;
3. path-safety and schema validation for any new artifact fields or files;
4. compatibility/capabilities evidence if KAS must activate the surface conditionally;
5. no KAH behavior that manages KAS lifecycle or evaluates subjective language quality;
6. repository verification evidence, including the KAH `make test` gate;
7. color review evidence for KAH/KAS boundary preservation before merge.

## 6. Roadmap registration

The roadmap registrations for this SOT are `token-001` and `token-002` under the `token` epic in `docs/roadmap.md`.

`token-001` is an implemented KAH evidence gate for compact English output, artifact-first detail references, repo-local agent-instruction evidence, project KAS lifecycle evidence, and explicit mutation-approval evidence. The implementation is the `token-economy` gate over canonical `token-economy-evidence.json` with schema version `token001.v1`.

`token-002` is a planned KAH implementation item for verification profile evidence, no-agent runner artifacts, reversible evidence summaries, compact review bundles, no-agent watcher terminal reports, and change-aware verification matrix evidence.

This SOT records token-001 as implemented by KAH and only registers/constrains token-002 for future implementation.

## 7. Promotion rule

This document may be promoted from candidate to accepted planning SOT after Red, Orange, and Gray color review accept the roadmap/SOT registration with no blocking findings. Promotion may update only the confirming role, status, and evidence/source paths unless a reviewer requires content changes.
