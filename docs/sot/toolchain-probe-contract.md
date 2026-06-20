# TOLMR — Toolchain probe contract SOT

Date: 2026-06-20
Owner: KAH deterministic helper layer
Confirming role: Responsible approver / governance evidence record
Status: planning SOT / implementation not authorized by this document alone
Authority level: KAH-side SOT for deterministic read-only project/helper facts consumed by KAS TOLMR
Scope: `kkachi-agent-helper` source behavior only; paired KAS schema/generation authority is `kkachi-hermes-skills/docs/sot/toolchain-local-metadata-registry.md`
Related docs: `docs/roadmap.md`, `docs/specs.md`, `docs/compatibility.md`, KAS `docs/sot/toolchain-local-metadata-registry.md`

## 1. Decision

TOLMR names the shared KAS/KAH work for generated project-local toolchain state:

```text
TOLMR = Toolchain local metadata registry
```

KAH's role is to expose deterministic facts that KAS can consume while generating or validating:

```text
<project>/.kkachi/toolchain.yaml
```

KAH must not own the `.kkachi/toolchain.yaml` schema, KAS policy, KAB adoption stage semantics, MAR role/provider policy, KAS upstream baselines, or legacy profile-skill-state migration. Those are KAS-owned in the paired KAS SOT. KAH provides read-only project/helper facts and no-write evidence.

## 2. Command contract

TOLMR should add this read-only command or an approved equivalent:

```bash
kkachi-agent-helper project probe-toolchain --json [--project-root <path>]
```

The command must:

1. resolve the target project root using existing KAH safe-path rules;
2. report helper command, version, and executable path facts;
3. report whether `.kkachi/` and KAH project state exist;
4. report whether the workflow graph and selected deterministic helper surfaces are present;
5. optionally include a compact `project doctor` summary if it can be computed without writes;
6. emit stable JSON even when the project is not initialized;
7. guarantee no mutation of `.kkachi/`, run state, events, graphs, schemas, locks, or project files;
8. use deterministic status and reason-code fields instead of prose-only errors.

## 3. JSON shape intent

The initial schema should be compact and intentionally fact-only:

```json
{
  "ok": true,
  "schema_version": "kah.toolchain_probe.v1",
  "no_write": {
    "guaranteed": true,
    "write_count": 0
  },
  "kah": {
    "helper_command": "kkachi-agent-helper",
    "version": "0.1.x",
    "binary_path": "/absolute/path/to/kkachi-agent-helper"
  },
  "project": {
    "root": "/absolute/path/to/project",
    "kkachi_dir": "/absolute/path/to/project/.kkachi",
    "kkachi_dir_present": true,
    "project_initialized": true,
    "workflow_graph_present": true
  },
  "doctor": {
    "status": "PASS|WARN|FAIL|UNKNOWN",
    "reason_codes": []
  },
  "diagnostics": []
}
```

Additional fields are allowed only when they are deterministic helper facts and do not move policy ownership into KAH.

## 4. KAH must not decide policy

The probe must not:

- select or change KAS/KAB adoption stage;
- decide Stage 1 versus Stage 2 execution eligibility;
- choose MAR roles, providers, retries, alternates, premium review, or waiver status;
- choose KAS upstream commits or project-specific KAS baselines;
- migrate or write `kas-project-state.yaml`, `kab-adoption-stage.md`, or `.kkachi/toolchain.yaml`;
- infer that KAB execution is allowed from the presence of KAB binaries;
- treat warning/degraded evidence as clean PASS.

KAH may report deterministic capability facts. KAS interprets those facts under its policy.

## 5. Logical task / physical commit rule

TOLMR follows 주군's KAS/KAH cross-repo preference:

```text
one logical task + one acceptance/evidence package + physical repo-specific commits/PRs
```

This does not collapse quality gates. For every TOLMR task touching KAH, the KAH repo must carry its own tests, enhance-test review, AI-slop cleanup/optimize pass, docs-impact check, local diff review, and final verification. Cross-repo KAS-to-KAH evidence is required but cannot replace KAH-local gates.

## 6. Shared task sequence and KAH ownership

| Task ID | Title | KAH scope | Status |
|---|---|---|---|
| TOLMR-001 | Schema and KAH probe contract | Define the KAH probe JSON contract and no-write boundary; pair with KAS schema definition. | Completed |
| TOLMR-002 | Generated toolchain init, doctor, and refresh | Implement the read-only `project probe-toolchain --json` substrate that KAS consumes. | In Review |
| TOLMR-003 | Legacy state migration plus Stage/MAR integration | No KAH behavior by default; provide verification support only if KAS proves a deterministic helper fact is missing. | Planned |
| TOLMR-004 | Cross-repo rollout, evidence, and release readiness | Run KAH-local gates plus cross-repo KAS->KAH toolchain generation evidence; update release/compatibility docs only after implementation evidence. | Planned |

## 7. Verification requirements

KAH completion evidence must include:

- unit and CLI tests for initialized and uninitialized projects;
- no-write proof that the probe creates no `.kkachi/` files, events, locks, graphs, schemas, or run artifacts;
- safe-path/root-resolution tests;
- stable JSON and reason-code tests for missing/unreadable project state;
- capability/help/docs updates only after command behavior exists;
- separate KAH enhance-test and AI-slop cleanup/optimize evidence;
- cross-repo smoke where KAS TOLMR calls the source-built or otherwise selected KAH probe and records the returned facts in generated `.kkachi/toolchain.yaml`.

## 8. Deferrals

TOLMR does not authorize KAH to add:

- write-capable toolchain generation;
- KAS policy, stage, MAR, provider, or backend selection logic;
- KAB runtime activation or session management;
- automatic KAS/KAH binary installation;
- provider/auth/gateway/model mutation;
- warning-only final gates;
- tracked project docs copies of local generated toolchain values.
