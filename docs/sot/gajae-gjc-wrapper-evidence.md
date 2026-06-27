# GAJAE — GJC wrapper and evidence contract SOT

Date: 2026-06-26
Owner: KAH deterministic helper layer
Confirming role: Responsible approver / governance evidence record
Status: GAJAE-002 Completed for the KAH source-side GJC evidence wrapper MVP; GAJAE-004 source-side ralplan/callback evidence pilot support is completed for source scope; this document does not authorize install/release/runtime activation by itself
Authority level: KAH-side SOT for deterministic GJC wrapper commands, GJC/KAT artifact refs, async callback evidence, and watcher-compatible status surfaces
Scope: `kkachi-agent-helper` source behavior only; paired KAS packet/gate authority is `kkachi-hermes-skills/docs/sot/gajae-delegated-execution-contract.md`
Related docs: `docs/roadmap.md`, `docs/specs.md`, `docs/compatibility.md`, `docs/sot/multi-agent-review-evidence-gates.md`, `docs/sot/strict-workflow-enforcement.md`, KAS `docs/sot/gajae-delegated-execution-contract.md`
External evidence: `/Users/draccoon/Workspace/Hermes/17thHermes/40_outputs/team/hwangchung/kkachi/2026-06-23-kas-kah-kat-gjc-execution-sot.md`, `/Users/draccoon/Workspace/Hermes/17thHermes/50_health/team/hwangchung/backups/gjc-delegated-execution-pilot-20260625/report.md`

## 1. Decision

`GAJAE` is the shared KAS/KAH epic for delegated Gajae Code (`gjc`) execution. KAH's role is to provide a thin deterministic wrapper around GJC and KAT so KAS can delegate planning/implementation without hand-written shell snippets, hidden environment assumptions, or Hermes-visible polling loops.

As of GAJAE-004 source-side work, the KAH source tree contains the completed `gjc` start/status wrapper plus source-side pilot evidence for ralplan receipt preservation, callback idempotency metadata, and KAS-supplied plan-lock hashes. This SOT records the intended and current helper boundary; it does not by itself authorize live runtime activation, KAB activation, provider/auth/token/gateway/model/profile mutation, install/release/push, or any transfer of KAS plan/review/MAR/final authority into KAH.

As of GAJAE-005 source-side work, the KAH source tree also contains bounded
KAT attachment evidence handling for `ultragoal_ready` candidate runs. This is
source behavior only: it records KAT status, summary, raw-log refs and hashes,
extractor status, command exit code, attachment status, and run-id linkage. It
does not authorize real async GJC `ultragoal`, live KAT execution, install,
release, runtime activation, or acceptance authority.

KAH must not become the policy or reasoning layer. It records facts, artifact references, hashes, process states, callbacks, and deterministic gate evidence. KAS/Blue/color review decide plan acceptance, implementation acceptance, MAR disposition, and final completion.

## 2. Pilot-verified wrapper requirements

The GAJAE pilot and the 2026-06-27 `/tmp/kkachi-gjc` scratch verification proved these KAH wrapper requirements and current live-pilot blockers:

1. GJC called from Hermes may read the Hermes profile home unless the wrapper normalizes execution to the real user home. The wrapper must set or otherwise resolve `HOME=/Users/draccoon` for the current operator environment.
2. Non-interactive GJC commands require an explicit `GJC_SESSION_ID` or equivalent session id. The wrapper must create, persist, and reuse this id.
3. Native `gjc ralplan --write` returns artifact path, stage, stage number, SHA-256, and created-at metadata that KAH should capture, but GJC 0.7.3 requires KAH to provide stage/stage-number/artifact or equivalent native inputs; KAH must not call live GJC with `--packet` alone.
4. Native `gjc ultragoal` writes `brief.md`, `goals.json`, `ledger.jsonl`, and exposes `status --json`, but GJC 0.7.3 requires `--brief`, `--brief-file`, or `--from-stdin`; KAH must derive that brief input from the KAS packet rather than passing `--packet` alone.
5. GJC can call Hermes Kanban CLI from foreground and background processes. KAH should still record callback intent, idempotency key, result, and notification metadata instead of relying on transient stdout.
6. KAT writes Kkachi run-id test artifacts under `.kkachi/runs/<run_id>/artifacts/test/` when invoked with global `--run-id` before `run`; GAJAE-009 KAH behavior normalizes raw KAT v0.1.0 status/summary/raw-log refs into bindable factual KAH KAT evidence without requiring a KAT-emitted compatibility snapshot.
7. KAT extractor statuses `degraded` and `no_match` are valid evidence states but do not override command exit code or KAS acceptance decisions.
8. Same-thread Discord wake-up requires explicit channel/thread metadata or an origin-bound watcher; KAH should preserve metadata/evidence but not decide notification routing policy.

## 3. KAH command surface

GAJAE should add these KAH commands or approved equivalents:

```bash
kkachi-agent-helper gjc start-deep-interview --run <run_id> --task <task_id> --packet <path> --json
kkachi-agent-helper gjc start-ralplan --run <run_id> --task <task_id> --packet <path> --json
kkachi-agent-helper gjc start-ultragoal --run <run_id> --task <task_id> --packet <path> --json
kkachi-agent-helper gjc status --run <run_id> --json
kkachi-agent-helper gjc attach-kat-evidence --run <run_id> --kat-status <path> --kat-status-hash sha256:<hash> --json
kkachi-agent-helper gjc callback-kanban --run <run_id> --task <task_id> --status <status> --json
kkachi-agent-helper gjc lock-plan --run <run_id> --accepted-plan-hash <sha256> --approval-ref <ref> --json
```

The exact CLI may change during implementation, but the wrapper must preserve these responsibilities:

- environment normalization;
- GJC session persistence;
- process start/status capture;
- artifact ref/hash capture;
- KAT evidence attachment;
- callback idempotency;
- source-side plan-lock hash evidence;
- watcher-compatible compact JSON.

For GAJAE-005, `attach-kat-evidence` must attach only run-local KAT evidence
for the same run id after `start-ultragoal` records `ultragoal_ready`. The
persisted KAT section records `run_id`, `status_ref`, `source_status_ref`,
`summary_ref`, `summary_md_ref`, `raw_log_ref`, `status_hash`,
`raw_log_hash`, `source_status_hash`, `extractor_status`, `command_exit_code`,
and `attachment_status`. It may route
the current required actor toward review, but it must not approve review,
waive findings, satisfy MAR, or mark final completion.

For the GAJAE-003 packet-reference contract, `packet_ref` is the canonical KAS input packet evidence stored in KAH status: the selected run-local packet path plus SHA-256. KAH validates `packet_ref` mechanically as repository-confined, run-local, readable, regular-file evidence with a matching hash before status is consumed. Missing, cross-run, unsafe, non-regular, unreadable, or hash-drifted `packet_ref` evidence fails closed with recovery guidance to regenerate or repair the KAS packet before consuming GJC status.

For GJC candidate output, `artifact_refs` remains the canonical GJC JSON field. KAH may accept `artifacts` as a bounded compatibility alias when `artifact_refs` is absent, but KAH-persisted status evidence must continue to write `artifact_refs`.

For GAJAE-004, `ralplan_ready` requires a run-local plan artifact reference
under `.kkachi/runs/<run_id>/artifacts/plan/` with a matching SHA-256 hash.
KAH may record `lock_status: locked`, `accepted_plan_hash`, and `approval_ref`
only when KAS/Blue/color approval evidence supplies the accepted hash. KAH does
not decide acceptance. If the locked plan artifact drifts, status consumption
fails closed with plan-conflict evidence requirements.

## 4. GJC delegation ledger intent

KAH should store a run-local ledger similar to:

```yaml
schema_version: kah.gajae_gjc_delegation.v1
run_id: <run_id>
task_id: <kanban-or-roadmap-task-id>
status: planning | plan_ready | plan_review | plan_locked | executing | review_ready | fixing | final_ready | blocked | failed | cancelled
current_required_actor: gjc | kas | color | mar | user | kat | none
current_wait_reason: null
real_user_home: /Users/draccoon
gjc:
  command: gjc
  version: <version>
  session_id: <gjc-session-id>
  process_id: <local-process-id-or-null>
  last_status_path: .kkachi/runs/<run_id>/artifacts/gjc/status.json
  last_status_hash: sha256:<hash>
packet_ref:
  path: .kkachi/runs/<run_id>/artifacts/gjc/<packet>.yaml
  sha256: sha256:<hash>
plan:
  artifact: .kkachi/runs/<run_id>/artifacts/plan/gjc-plan.md
  artifact_hash: sha256:<hash>
  lock_status: unlocked | locked | conflict_reported
ultragoal:
  brief: .kkachi/runs/<run_id>/artifacts/gjc/brief.md
  goals: .kkachi/runs/<run_id>/artifacts/gjc/goals.json
  ledger: .kkachi/runs/<run_id>/artifacts/gjc/ledger.jsonl
kat:
  status_json: .kkachi/runs/<run_id>/artifacts/test/<id>.status.json
  summary_json: .kkachi/runs/<run_id>/artifacts/test/<id>.summary.json
  summary_md: .kkachi/runs/<run_id>/artifacts/test/<id>.summary.md
  raw_log: .kkachi/runs/<run_id>/artifacts/test/<id>.raw.log
  status_hash: sha256:<hash>
callback:
  task_id: <kanban-task-id>
  idempotency_key: <key>
  last_callback_status: pending | delivered | failed
  notification_context_ref: <platform-chat-thread-or-origin-ref>
  source_status_hash: sha256:<hash>
  last_notified_hash: sha256:<hash>
  same_thread_wake_claim: false
```

The ledger is evidence, not policy authority. KAS interprets it.

## 5. KAT attachment contract

KAH recognizes KAT v0.1.0 evidence produced by:

```bash
kkachi-agent-tester --run-id <run_id> run --lane <lane> -- <command...>
```

For GAJAE-009, `attach-kat-evidence` accepts the run-local KAT status JSON and status hash as the minimum operator input. KAH normalizes the factual v0.1.0 fields `summary_path`, `summary_sha256`, `raw_log_path`, `raw_log_sha256`, `extractor_status`, and `exit_code` into the persisted KAH `kat` evidence section, derives the sibling Markdown summary ref from `<summary>.json -> <summary>.md`, and binds the attachment to the pre-attachment GJC `status_hash` as `source_status_hash`. KAT `status_hash` remains KAT self-integrity metadata only; KAH must not treat it as the GJC source status hash.

KAH must reject or report incomplete evidence when the expected files are missing, unreadable, outside the project/run root, symlinked/non-regular, checksum-mismatched, or impossible to map to the current run.

KAH must not treat `extractor_status=degraded` or `extractor_status=no_match` as command success. These states are parser/rule-quality evidence only.

KAH must fail closed when KAT attachment evidence is missing, unsafe, escaping,
cross-run, unreadable, non-regular, hashless, checksum-mismatched, malformed,
run-id-mismatched, uses unsupported factual statuses, or claims self-approval,
review approval, waiver approval, MAR acceptance, or final acceptance.

## 6. Callback and watcher policy

KAH must preserve enough evidence for no-agent watchers and Kanban callbacks to resume the correct Kkachi context:

- run id;
- task id;
- expected status transition;
- callback idempotency key;
- source status hash;
- comment/complete result;
- notification target or origin reference when available;
- notification status (`metadata_recorded_no_wake_claim` or `no_wake_claim`);
- wake evidence status (`missing_watcher_evidence` until explicit origin/thread metadata and watcher proof exist);
- last-notified hash to avoid repeat notifications.

Callbacks may report that a plan or implementation bundle is ready. They must not mark Kkachi plan approval, review acceptance, MAR acceptance, same-thread wake readiness, or final completion by themselves. Metadata-only callbacks are factual notification evidence and must remain `no-wake-claim` until a separately verified watcher/origin/thread evidence path exists.

## 7. Shared task sequence and KAH ownership

GAJAE uses shared logical task ids across KAS and KAH. KAH tasks require KAH-local tests, docs, review, and final gates even when the logical task is cross-repo.

| Task ID | Title | KAH scope | Status |
|---|---|---|---|
| GAJAE-001 | Register GAJAE SOTs and roadmap sequence | Add this SOT, KAS companion cross-link, roadmap/docs index/docs-map entries. | Completed |
| GAJAE-002 | Implement KAH GJC wrapper MVP | Add `gjc` command group with environment/session normalization and read-only/status-safe start/status behavior. | Completed |
| GAJAE-003 | Add GJC packet/template and artifact-reference contract | Shared logical task with KAS physical packet-template scope and KAH physical packet-ref/artifact-ref preservation scope. KAH validates `packet_ref` as KAS input packet evidence and preserves GJC `artifact_refs` as candidate output evidence without interpreting policy. | Completed |
| GAJAE-004 | Async ralplan callback pilot | Source-side support for ralplan receipt/status, callback idempotency evidence, no-wake-claim metadata, and KAS-supplied plan-lock hash recording for plan review. | Completed |
| GAJAE-005 | Async ultragoal + KAT evidence pilot | Start ultragoal async, attach KAT run-id evidence, and expose review-ready status. | Source-side pilot |
| GAJAE-006 | Watcher/callback closeout | Productize idempotent callback/watcher status surfaces and docs/compatibility notes. | Completed |
| GAJAE-007 | Real GJC `ralplan` adapter | Convert KAS packet `native_ralplan_input.stage`, `.stage_n`, and `.artifact` fields into GJC 0.7.3 native `ralplan --write` flags and persist candidate plan refs/hashes without KAH plan acceptance. | Completed |
| GAJAE-008 | Real GJC `ultragoal` adapter | Convert KAS packet `native_ultragoal_input.brief` into run-local `native_input_ref` evidence, invoke native `ultragoal create-goals --brief-file`, and persist implementation-candidate refs/hashes without review/MAR/final acceptance. | Completed |
| GAJAE-009 | KAT evidence normalization / attach adapter | Normalize KAT v0.1.0 factual artifacts into KAH-bindable KAT evidence from status/summary/raw-log refs without requiring KAT source changes. | Completed |
| GAJAE-010 | Contract docs and skill guidance update | Update KAS/KAH/KAT repo docs plus Hermes skill references after the real adapter contracts settle. | Planned |

## 7.1. GAJAE-007..010 pilot-unblock task scope

- `GAJAE-007`: KAH code changes for the real GJC 0.7.3 ralplan invocation contract. Done criteria include live-compatible native input construction, run-local candidate plan refs/hashes, plan-lock compatibility, and fail-closed behavior for missing/malformed native inputs.
- `GAJAE-008`: KAH code changes for the real GJC 0.7.3 ultragoal invocation contract. Done criteria include KAS-derived brief generation as run-local `native_input_ref`, native `ultragoal create-goals --brief-file` invocation, run-local implementation-candidate refs/hashes, and no authority claims beyond `ultragoal_ready`.
- `GAJAE-009`: KAH/KAT evidence contract changes. Done criteria include a normalized KAT evidence snapshot or KAT-emitted bindable status, pre-attachment GJC status-hash binding, factual command/extractor evidence preservation, and fail-closed rejection of approval/final/review/MAR/waiver claims.
- `GAJAE-010`: documentation and skill-reference closeout after the adapter contracts settle. Verification is recorded inside each task's done criteria; do not create a separate verification-only task.

## 8. GAJAE-001 acceptance criteria

GAJAE-001 is complete only when:

1. KAH and KAS SOT docs exist and cross-link each other.
2. KAH and KAS roadmaps register the GAJAE epic and shared task sequence.
3. KAH docs index and docs map include the new SOT.
4. KAS docs index and docs map include the KAS companion SOT.
5. Docs verification passes in both repositories, or blockers are recorded explicitly.
6. Red/Orange/Gray review accepts the planning SOTs, or 주군 explicitly waives that review for docs-only registration. Completed evidence: Red `t_4cbf4624`, Orange `t_18dccb4c`, Gray initial `t_bbb1af05`, Gray focused re-review `t_c6ba0567`, and Blue synthesis `t_6be5b0e5` accepted the docs-only planning registration after traceability fixes; synthesis artifact: `/Users/draccoon/Workspace/Hermes/17thHermes/50_health/team/hwangchung/backups/gajae-001-color-review-20260626/blue-synthesis.md`.
7. No helper command behavior, schema, gate, installed binary, release, push, KAB activation, profile mutation, or provider/auth/gateway/model mutation is claimed.

## 9. Deferrals

GAJAE does not authorize KAH to add:

- KAS plan acceptance, review adjudication, MAR disposition, or final acceptance logic;
- GJC self-approval or warning-only final states;
- automatic fallback to another backend when GJC fails;
- KAB runtime/session control unless a separate KAB task is approved;
- provider/auth/token/gateway/model mutation;
- broad profile mutation, install, release, push, or runtime activation;
- same-thread Discord wake claims before callback metadata and watcher evidence are implemented and verified.
