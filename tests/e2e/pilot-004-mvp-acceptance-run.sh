#!/bin/sh
set -eu

if [ "$#" -ne 1 ]; then
  echo "usage: $0 <kkachi-agent-helper-binary>" >&2
  exit 2
fi

if ! command -v python3 >/dev/null 2>&1; then
  echo "error: python3 required but not found" >&2
  exit 2
fi

helper="$1"
tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

assert_contains() {
  file="$1"
  pattern="$2"
  label="$3"
  if ! grep -Fq "$pattern" "$file"; then
    echo "$label did not contain expected pattern: $pattern" >&2
    echo "--- $label ---" >&2
    cat "$file" >&2
    exit 1
  fi
}

assert_empty() {
  file="$1"
  label="$2"
  if [ -s "$file" ]; then
    echo "$label unexpectedly wrote stderr" >&2
    echo "--- $label stderr ---" >&2
    cat "$file" >&2
    exit 1
  fi
}

json_field() {
  file="$1"
  field="$2"
  python3 - "$file" "$field" <<'PYJSON'
import json
import sys

try:
    with open(sys.argv[1], encoding='utf-8') as f:
        value = json.load(f)
    for part in sys.argv[2].split('.'):
        value = value[part]
except (OSError, json.JSONDecodeError, KeyError, TypeError) as exc:
    print(f"error: cannot read JSON field {sys.argv[2]!r} from {sys.argv[1]}: {exc}", file=sys.stderr)
    sys.exit(1)
if value is None:
    print(f"error: JSON field {sys.argv[2]!r} in {sys.argv[1]} is null", file=sys.stderr)
    sys.exit(1)
print(value)
PYJSON
}

assert_json_field() {
  file="$1"
  field="$2"
  expected="$3"
  label="$4"
  actual="$(json_field "$file" "$field")"
  if [ "$actual" != "$expected" ]; then
    echo "$label JSON field $field = $actual, want $expected" >&2
    echo "--- $label JSON ---" >&2
    cat "$file" >&2
    exit 1
  fi
}

assert_run_id() {
  run_id="$1"
  python3 - "$run_id" <<'PYRUNID'
import re
import sys

run_id = sys.argv[1]
if not re.fullmatch(r"run-\d{8}T\d{6}Z-[0-9a-f]{12}", run_id):
    print(f"error: unsafe or invalid run_id: {run_id}", file=sys.stderr)
    sys.exit(1)
PYRUNID
}

write_artifact() {
  repo="$1"
  run_id="$2"
  path="$3"
  body="$4"
  mkdir -p "$(dirname "$repo/.kkachi/runs/$run_id/$path")"
  printf '%s\n' "$body" > "$repo/.kkachi/runs/$run_id/$path"
}

repo="$tmpdir/pilot-004-repo"
mkdir -p "$repo/.git"

(cd "$repo" && "$helper" project init --json > "$tmpdir/init.json" 2> "$tmpdir/init.err")
assert_empty "$tmpdir/init.err" "project init"
(cd "$repo" && "$helper" run create \
  --title "Pilot 004 MVP acceptance run" \
  --work-path A_development_execution \
  --work-mode standard \
  --urgency normal \
  --sot-policy existing_sot_basis \
  --execution-mode adapter_qa \
  --commander Gongmyeong \
  --task-id pilot-004 \
  --redteam Haneul \
  --json > "$tmpdir/run-create.json" 2> "$tmpdir/run-create.err")
assert_empty "$tmpdir/run-create.err" "run create"
run_id="$(json_field "$tmpdir/run-create.json" run_id)"
assert_run_id "$run_id"

(cd "$repo" && "$helper" run activate "$run_id" --json > "$tmpdir/run-activate.json" 2> "$tmpdir/run-activate.err")
assert_empty "$tmpdir/run-activate.err" "run activate"
assert_json_field "$tmpdir/run-activate.json" run_id "$run_id" "run activate"
assert_json_field "$tmpdir/run-activate.json" state active "run activate"
(cd "$repo" && "$helper" artifact init "$run_id" --json > "$tmpdir/artifact-init.json" 2> "$tmpdir/artifact-init.err")
assert_empty "$tmpdir/artifact-init.err" "artifact init"
(cd "$repo" && "$helper" project status --json > "$tmpdir/status-active.json" 2> "$tmpdir/status-active.err")
assert_empty "$tmpdir/status-active.err" "active project status"
assert_json_field "$tmpdir/status-active.json" active_run_id "$run_id" "active project status"
assert_json_field "$tmpdir/status-active.json" active_run_state active "active project status"

write_artifact "$repo" "$run_id" intake-classification.md "Status: complete
Work Path: A_development_execution
Work Mode: standard
SOT Policy: existing_sot_basis
Urgency: normal
Acceptance Evidence: pilot-004 local MVP run is classified as adapter QA."
write_artifact "$repo" "$run_id" sot-basis.md "Status: complete
Source: docs/specs.md
Basis: helper CLI behavior is already specified for run, artifact, gate, and diagnostics commands."
write_artifact "$repo" "$run_id" roadmap-update.md "Status: complete
Trace: docs/roadmap.md pilot-004"
write_artifact "$repo" "$run_id" acceptance-criteria.md "Status: complete
Criteria: status, events, artifacts, bridge evidence, verification, docs decision, gate reports, diagnostics bundle, and final report are preserved."
write_artifact "$repo" "$run_id" plan.md "Status: complete
Plan: execute public helper commands against a temporary git repository and preserve deterministic acceptance evidence."
write_artifact "$repo" "$run_id" checklist.md "Status: complete
- [x] adapter QA bridge evidence recorded
- [x] required gates pass
- [x] diagnostics bundle exported
- [x] final report preserved"
write_artifact "$repo" "$run_id" task-brief.md "Status: complete
Brief: pilot-004 proves MVP readiness discipline through one complete helper-managed run."
write_artifact "$repo" "$run_id" prompt.md "Status: complete
Prompt: execute pilot-004 according to docs/roadmap.md and docs/specs.md."
write_artifact "$repo" "$run_id" context-pack.md "Status: complete
Context: docs/specs.md gate contracts and docs/roadmap.md pilot epic."

cat > "$repo/.kkachi/runs/$run_id/selected-cli.json" <<'JSON'
{
  "version": "0.1",
  "status": "supported",
  "backend_type": "codex",
  "adapter_type": "openai-codex",
  "source_ledger_ref": "pilot-004-local-ledger",
  "caveats": []
}
JSON
write_artifact "$repo" "$run_id" capability-check.md "Status: complete
Backend Type: codex
Adapter Type: openai-codex
Capability: local helper acceptance workflow can preserve bridge-shaped evidence without choosing a backend."
cat > "$repo/.kkachi/runs/$run_id/bridge-session-snapshot.json" <<'JSON'
{
  "version": "0.1",
  "session_id": "pilot-004-local-session",
  "backend_type": "codex",
  "adapter_type": "openai-codex",
  "state": "closed",
  "lifecycle_class": "local_acceptance",
  "open_pendings": 0
}
JSON
write_artifact "$repo" "$run_id" bridge-events.md "Status: complete
Event: selected codex/openai-codex evidence was recorded for the local acceptance run.
Event: bridge-shaped session snapshot closed with open_pendings 0."
write_artifact "$repo" "$run_id" cli-output.md "Status: complete
Output: project init, run create, run activate, artifact init, gate checks, gate final, and diagnostics export completed."

write_artifact "$repo" "$run_id" diff.patch "diff --git a/.kkachi/evidence b/.kkachi/evidence
+pilot-004 acceptance evidence preserved through helper artifacts"
write_artifact "$repo" "$run_id" impl-log.md "Status: complete
Implementation: pilot-004 acceptance run exercised helper-managed evidence capture without source mutation in the temporary target repo."
write_artifact "$repo" "$run_id" review.md "Status: complete
Review: acceptance evidence is complete for helper responsibility boundaries."
write_artifact "$repo" "$run_id" redteam/plan-review.md "Status: complete
Review: plan evidence matches docs/roadmap.md pilot-004."
write_artifact "$repo" "$run_id" redteam/shaping-review.md "Status: complete
Review: no shaping blocker for the MVP helper acceptance run."
write_artifact "$repo" "$run_id" redteam/qa-review.md "Status: complete
Review: adapter QA bridge evidence is shape-valid and identity-consistent."
write_artifact "$repo" "$run_id" redteam/final-gate-review.md "Status: complete
Review: final gate is ready after all required gate reports are generated."
write_artifact "$repo" "$run_id" test-log.md "Status: complete
Tests: pilot-004 MVP acceptance E2E scenario executed against a temporary repository."
write_artifact "$repo" "$run_id" verification.md "Status: complete
Verdict: pass
Evidence: all required pilot-004 gates passed and diagnostics were exported."
write_artifact "$repo" "$run_id" docs-update.md "Status: complete
No Change Reason: the temporary pilot run records runtime evidence; repository docs are updated by the harness change, not inside the target run."
write_artifact "$repo" "$run_id" final-report.md "Status: complete
Report: pilot-004 preserved project status, event history, run artifacts, bridge evidence, verification verdict, docs-update decision, gate reports, diagnostics bundle, and this final report."

for gate in intake sot roadmap plan backend implementation review verification docs; do
  (cd "$repo" && "$helper" gate check "$run_id" "$gate" --json > "$tmpdir/gate-$gate.json" 2> "$tmpdir/gate-$gate.err")
  assert_empty "$tmpdir/gate-$gate.err" "$gate gate"
  assert_contains "$tmpdir/gate-$gate.json" '"status":"pass"' "$gate gate JSON"
  assert_contains "$tmpdir/gate-$gate.json" '"report_path":".kkachi/runs/'"$run_id"'/gate-reports/'"$gate"'.json"' "$gate gate JSON"
done

(cd "$repo" && "$helper" gate final "$run_id" --json > "$tmpdir/gate-final.json" 2> "$tmpdir/gate-final.err")
assert_empty "$tmpdir/gate-final.err" "final gate"
assert_contains "$tmpdir/gate-final.json" '"gate":"final"' "final gate JSON"
assert_contains "$tmpdir/gate-final.json" '"status":"pass"' "final gate JSON"
assert_contains "$repo/.kkachi/runs/$run_id/gate-reports/final.json" '"status": "pass"' "final gate report"

(cd "$repo" && "$helper" project status --json > "$tmpdir/status-gated.json" 2> "$tmpdir/status-gated.err")
assert_empty "$tmpdir/status-gated.err" "gated project status"
assert_contains "$tmpdir/status-gated.json" '"gate_summary"' "gated project status JSON"
assert_contains "$tmpdir/status-gated.json" '"final"' "gated project status JSON"
assert_contains "$repo/.kkachi/events.jsonl" '"type":"gate.passed"' "event log"
assert_contains "$repo/.kkachi/events.jsonl" '"gate":"backend"' "event log"
assert_contains "$repo/.kkachi/events.jsonl" '"gate":"final"' "event log"

(cd "$repo" && "$helper" diagnostics export --run "$run_id" --output diagnostics/pilot-004.json > "$tmpdir/diagnostics-human.txt" 2> "$tmpdir/diagnostics.err")
assert_empty "$tmpdir/diagnostics.err" "diagnostics export"
assert_contains "$tmpdir/diagnostics-human.txt" 'diagnostics bundle exported: diagnostics/pilot-004.json' "diagnostics human output"
assert_contains "$repo/diagnostics/pilot-004.json" '"run_id": "'"$run_id"'"' "diagnostics bundle"
assert_contains "$repo/diagnostics/pilot-004.json" '"gate_reports": [' "diagnostics bundle"
assert_contains "$repo/diagnostics/pilot-004.json" 'gate-reports/final.json' "diagnostics bundle"
assert_contains "$repo/diagnostics/pilot-004.json" 'selected-cli.json' "diagnostics bundle"
assert_contains "$repo/diagnostics/pilot-004.json" 'bridge-session-snapshot.json' "diagnostics bundle"
assert_contains "$repo/diagnostics/pilot-004.json" 'verification.md' "diagnostics bundle"
assert_contains "$repo/diagnostics/pilot-004.json" 'docs-update.md' "diagnostics bundle"
assert_contains "$repo/diagnostics/pilot-004.json" 'final-report.md' "diagnostics bundle"

(cd "$repo" && "$helper" run close "$run_id" --json > "$tmpdir/run-close.json" 2> "$tmpdir/run-close.err")
assert_empty "$tmpdir/run-close.err" "run close"
assert_json_field "$tmpdir/run-close.json" run_id "$run_id" "run close"
assert_json_field "$tmpdir/run-close.json" state closed "run close"
(cd "$repo" && "$helper" project status --json > "$tmpdir/status-closed.json" 2> "$tmpdir/status-closed.err")
assert_empty "$tmpdir/status-closed.err" "closed project status"
assert_contains "$tmpdir/status-closed.json" '"active_run_id":null' "closed project status JSON"
assert_contains "$repo/.kkachi/events.jsonl" '"type":"run.closed"' "event log"
