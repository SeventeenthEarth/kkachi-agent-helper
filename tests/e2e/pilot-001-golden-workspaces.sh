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
project_root="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
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

assert_not_contains() {
  file="$1"
  pattern="$2"
  label="$3"
  if grep -Fq "$pattern" "$file"; then
    echo "$label unexpectedly contained pattern: $pattern" >&2
    echo "--- $label ---" >&2
    cat "$file" >&2
    exit 1
  fi
}

make_repo() {
  name="$1"
  repo="$tmpdir/$name"
  mkdir -p "$repo/.git"
  (cd "$repo" && pwd -P)
}

json_field() {
  file="$1"
  field="$2"
  python3 - "$file" "$field" <<'PY'
import json, sys
with open(sys.argv[1], encoding='utf-8') as f:
    value = json.load(f)
for part in sys.argv[2].split('.'):
    value = value[part]
print(value)
PY
}

write_complete_artifact() {
  repo="$1"
  run_id="$2"
  path="$3"
  body="$4"
  mkdir -p "$(dirname "$repo/.kkachi/runs/$run_id/$path")"
  printf '%s\n' "$body" > "$repo/.kkachi/runs/$run_id/$path"
}

copy_golden_workspace() {
  fixture="$1"
  repo="$2"
  fixture_path="$project_root/tests/e2e/golden-workspaces/pilot-001/$fixture/.kkachi"
  if [ ! -d "$fixture_path" ]; then
    echo "missing golden workspace fixture: $fixture" >&2
    exit 1
  fi
  cp -R "$fixture_path" "$repo/.kkachi"
}

# Successful black-box CLI flow against a temporary repository.
success_repo="$(make_repo success-repo)"
(cd "$success_repo" && "$helper" project init --json > "$tmpdir/success-init.json" 2> "$tmpdir/success-init.err")
(cd "$success_repo" && "$helper" run create --title "Pilot 001 E2E success" --work-path A_development_execution --work-mode standard --urgency normal --sot-policy existing_sot_basis --execution-mode production_write --commander Gongmyeong --task-id pilot-001 --json > "$tmpdir/success-run-create.json" 2> "$tmpdir/success-run-create.err")
run_id="$(json_field "$tmpdir/success-run-create.json" run_id)"
(cd "$success_repo" && "$helper" artifact init "$run_id" --json > "$tmpdir/success-artifact-init.json" 2> "$tmpdir/success-artifact-init.err")
(cd "$success_repo" && "$helper" artifact list "$run_id" --json > "$tmpdir/success-artifact-list.json" 2> "$tmpdir/success-artifact-list.err")
assert_contains "$tmpdir/success-artifact-list.json" '"run_id":"'"$run_id"'"' "success artifact list JSON"
assert_contains "$tmpdir/success-artifact-list.json" '"path":"intake-classification.md","required":true,"exists":true' "success artifact list JSON"

write_complete_artifact "$success_repo" "$run_id" intake-classification.md "Status: complete
Work Path: A_development_execution
Work Mode: standard
SOT Policy: existing_sot_basis
Urgency: normal"
write_complete_artifact "$success_repo" "$run_id" sot-basis.md "Status: complete
Source: docs/specs.md"
write_complete_artifact "$success_repo" "$run_id" roadmap-update.md "Status: complete
Trace: docs/roadmap.md pilot-001"
write_complete_artifact "$success_repo" "$run_id" acceptance-criteria.md "Status: complete
Criteria: black-box CLI success and failure flows pass"
write_complete_artifact "$success_repo" "$run_id" plan.md "Status: complete
Plan: exercise public CLI surfaces only"
write_complete_artifact "$success_repo" "$run_id" checklist.md "Status: complete
- [x] successful final gate
- [x] failure scenarios covered"
printf 'diff --git a/file b/file\n+pilot-001 e2e evidence\n' > "$success_repo/.kkachi/runs/$run_id/diff.patch"
write_complete_artifact "$success_repo" "$run_id" impl-log.md "Status: complete
Implementation: pilot-001 harness verified"
write_complete_artifact "$success_repo" "$run_id" review.md "Status: complete
Review: harness assertions passed"
write_complete_artifact "$success_repo" "$run_id" redteam/impl-review.md "Status: complete
Review: no implementation blockers"
write_complete_artifact "$success_repo" "$run_id" redteam/test-review.md "Status: complete
Review: test coverage accepted"
write_complete_artifact "$success_repo" "$run_id" redteam/final-gate-review.md "Status: complete
Review: final gate ready"
write_complete_artifact "$success_repo" "$run_id" test-log.md "Status: complete
Tests: pilot-001 e2e harness"
write_complete_artifact "$success_repo" "$run_id" verification.md "Status: complete
Verdict: pass"
write_complete_artifact "$success_repo" "$run_id" docs-update.md "Status: complete
Changed Docs: docs/roadmap.md"
write_complete_artifact "$success_repo" "$run_id" final-report.md "Status: complete
Report: pilot-001 black-box flow complete"

(cd "$success_repo" && "$helper" artifact validate "$run_id" --gate intake --json > "$tmpdir/success-artifact-validate.json" 2> "$tmpdir/success-artifact-validate.err")
assert_contains "$tmpdir/success-artifact-validate.json" '"gate":"intake"' "success artifact validate JSON"
assert_contains "$tmpdir/success-artifact-validate.json" '"status":"pass"' "success artifact validate JSON"

for gate in intake sot roadmap plan implementation review verification docs; do
  (cd "$success_repo" && "$helper" gate check "$run_id" "$gate" --json > "$tmpdir/success-gate-$gate.json" 2> "$tmpdir/success-gate-$gate.err")
  assert_contains "$tmpdir/success-gate-$gate.json" '"status":"pass"' "success $gate gate JSON"
  assert_contains "$tmpdir/success-gate-$gate.json" '"report_path":".kkachi/runs/'"$run_id"'/gate-reports/'"$gate"'.json"' "success $gate gate JSON"
done
(cd "$success_repo" && "$helper" gate final "$run_id" --json > "$tmpdir/success-final.json" 2> "$tmpdir/success-final.err")
assert_contains "$tmpdir/success-final.json" '"gate":"final"' "success final gate JSON"
assert_contains "$tmpdir/success-final.json" '"status":"pass"' "success final gate JSON"
assert_contains "$success_repo/.kkachi/runs/$run_id/gate-reports/final.json" '"status": "pass"' "success final gate report"

# Failure: missing required artifacts should fail closed through the gate engine.
missing_repo="$(make_repo missing-artifacts-repo)"
(cd "$missing_repo" && "$helper" project init --json > "$tmpdir/missing-init.json" 2> "$tmpdir/missing-init.err")
(cd "$missing_repo" && "$helper" run create --title "Pilot 001 missing artifacts" --work-path A_development_execution --work-mode standard --urgency normal --sot-policy existing_sot_basis --execution-mode production_write --commander Gongmyeong --task-id pilot-001 --json > "$tmpdir/missing-run-create.json" 2> "$tmpdir/missing-run-create.err")
missing_run_id="$(json_field "$tmpdir/missing-run-create.json" run_id)"
(cd "$missing_repo" && "$helper" artifact init "$missing_run_id" --json > "$tmpdir/missing-artifact-init.json" 2> "$tmpdir/missing-artifact-init.err")
: > "$missing_repo/.kkachi/runs/$missing_run_id/acceptance-criteria.md"
if (cd "$missing_repo" && "$helper" gate check "$missing_run_id" plan --json > "$tmpdir/missing-plan.json" 2> "$tmpdir/missing-plan.err"); then
  echo "plan gate unexpectedly passed with missing artifacts" >&2
  exit 1
fi
assert_contains "$tmpdir/missing-plan.json" '"status":"fail"' "missing artifacts plan gate JSON"
assert_contains "$tmpdir/missing-plan.json" '"missing_evidence":[' "missing artifacts plan gate JSON"
assert_contains "$tmpdir/missing-plan.json" 'acceptance-criteria.md' "missing artifacts plan gate JSON"

# Failure: ambiguous run prefixes must fail closed.
ambiguous_repo="$(make_repo ambiguous-run-repo)"
(cd "$ambiguous_repo" && "$helper" project init --json > "$tmpdir/ambiguous-init.json" 2> "$tmpdir/ambiguous-init.err")
(cd "$ambiguous_repo" && "$helper" run create --title "Pilot 001 ambiguous A" --work-path A_development_execution --work-mode standard --urgency normal --sot-policy existing_sot_basis --execution-mode production_write --commander Gongmyeong --task-id pilot-001 --json > "$tmpdir/ambiguous-a.json" 2> "$tmpdir/ambiguous-a.err")
(cd "$ambiguous_repo" && "$helper" run create --title "Pilot 001 ambiguous B" --work-path A_development_execution --work-mode standard --urgency normal --sot-policy existing_sot_basis --execution-mode production_write --commander Gongmyeong --task-id pilot-001 --json > "$tmpdir/ambiguous-b.json" 2> "$tmpdir/ambiguous-b.err")
if (cd "$ambiguous_repo" && "$helper" run show run --json > "$tmpdir/ambiguous-show.json" 2> "$tmpdir/ambiguous-show.err"); then
  echo "ambiguous run prefix unexpectedly resolved" >&2
  exit 1
fi
assert_contains "$tmpdir/ambiguous-show.err" '"code":"run_id_ambiguous"' "ambiguous run stderr"

# Failure: fresh project write lock should block mutating commands without appending events.
lock_repo="$(make_repo lock-conflict-repo)"
(cd "$lock_repo" && "$helper" project init --json > "$tmpdir/lock-init.json" 2> "$tmpdir/lock-init.err")
hostname="$(hostname 2>/dev/null || printf unknown)"
created_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
python3 - "$lock_repo/.kkachi/project_write.lock" "$$" "$hostname" "$created_at" <<'PY'
import json, sys
path, pid, hostname, created_at = sys.argv[1:]
with open(path, 'w', encoding='utf-8') as f:
    json.dump({"version":"0.1","lock_name":"project_write","owner_pid":int(pid),"hostname":hostname,"command":"pilot-001 lock conflict","created_at":created_at}, f)
    f.write("\n")
PY
if (cd "$lock_repo" && "$helper" event append artifact.written --run run-pilot --payload '{}' --json > "$tmpdir/lock-event.json" 2> "$tmpdir/lock-event.err"); then
  echo "event append unexpectedly succeeded under project_write lock" >&2
  exit 1
fi
assert_contains "$tmpdir/lock-event.err" '"code":"lock_conflict"' "lock conflict stderr"
# The command failed above; this separately proves the failed mutation left no
# event-log side effect before returning lock_conflict.
event_count="$(wc -l < "$lock_repo/.kkachi/events.jsonl" | tr -d ' ')"
if [ "$event_count" != "1" ]; then
  echo "lock conflict appended events unexpectedly: $event_count" >&2
  cat "$lock_repo/.kkachi/events.jsonl" >&2
  exit 1
fi

# Failure: unsafe paths and bad JSON must fail through public CLI errors.
safety_repo="$(make_repo safety-repo)"
(cd "$safety_repo" && "$helper" project init --json > "$tmpdir/safety-init.json" 2> "$tmpdir/safety-init.err")
printf '{}\n' > "$tmpdir/outside-status.json"
if (cd "$safety_repo" && "$helper" schema validate ../outside-status.json --schema status --json > "$tmpdir/unsafe-path.json" 2> "$tmpdir/unsafe-path.err"); then
  echo "unsafe schema path unexpectedly succeeded" >&2
  exit 1
fi
assert_contains "$tmpdir/unsafe-path.err" '"code":"path_escape"' "unsafe path stderr"
if (cd "$safety_repo" && "$helper" event append artifact.written --run run-pilot --payload '{' --json > "$tmpdir/bad-json.json" 2> "$tmpdir/bad-json.err"); then
  echo "bad JSON payload unexpectedly succeeded" >&2
  exit 1
fi
assert_contains "$tmpdir/bad-json.err" '"code":"payload_invalid_json"' "bad JSON stderr"
assert_not_contains "$safety_repo/.kkachi/events.jsonl" 'run-pilot' "bad JSON events"

# Failure: copy golden workspace fixtures and validate fail-closed diagnostics through the binary.
golden_repo="$(make_repo golden-schema-repo)"
copy_golden_workspace schema-mismatch "$golden_repo"
if (cd "$golden_repo" && "$helper" schema validate .kkachi/status.json --schema status --json > "$tmpdir/golden-schema.json" 2> "$tmpdir/golden-schema.err"); then
  echo "schema mismatch golden workspace unexpectedly passed" >&2
  exit 1
fi
assert_contains "$tmpdir/golden-schema.json" '"status":"fail"' "schema mismatch golden JSON"
assert_contains "$tmpdir/golden-schema.json" '"name":"project_id"' "schema mismatch golden JSON"
assert_contains "$tmpdir/golden-schema.json" '"name":"updated_at"' "schema mismatch golden JSON"

golden_mismatch_repo="$(make_repo golden-status-event-mismatch-repo)"
copy_golden_workspace status-event-mismatch "$golden_mismatch_repo"
if (cd "$golden_mismatch_repo" && "$helper" project doctor --json > "$tmpdir/golden-mismatch-doctor.json" 2> "$tmpdir/golden-mismatch-doctor.err"); then
  echo "status/event mismatch golden workspace unexpectedly passed doctor" >&2
  exit 1
fi
assert_contains "$tmpdir/golden-mismatch-doctor.json" '"health":"fail"' "status/event mismatch doctor JSON"
assert_contains "$tmpdir/golden-mismatch-doctor.json" '"name":"coherence"' "status/event mismatch doctor JSON"
assert_contains "$tmpdir/golden-mismatch-doctor.json" '"expected":"evt-000002"' "status/event mismatch doctor JSON"
assert_contains "$tmpdir/golden-mismatch-doctor.json" '"actual":"evt-000001"' "status/event mismatch doctor JSON"

golden_invalid_events_repo="$(make_repo golden-invalid-events-repo)"
copy_golden_workspace invalid-events-jsonl "$golden_invalid_events_repo"
if (cd "$golden_invalid_events_repo" && "$helper" project doctor --json > "$tmpdir/golden-invalid-events-doctor.json" 2> "$tmpdir/golden-invalid-events-doctor.err"); then
  echo "invalid events golden workspace unexpectedly passed doctor" >&2
  exit 1
fi
assert_contains "$tmpdir/golden-invalid-events-doctor.json" '"health":"fail"' "invalid events doctor JSON"
assert_contains "$tmpdir/golden-invalid-events-doctor.json" '"name":"events"' "invalid events doctor JSON"
assert_contains "$tmpdir/golden-invalid-events-doctor.json" 'invalid JSON' "invalid events doctor JSON"
