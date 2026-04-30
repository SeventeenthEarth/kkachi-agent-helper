#!/bin/sh
set -eu

if [ "$#" -ne 1 ]; then
  echo "usage: $0 <kkachi-agent-helper-binary>" >&2
  exit 2
fi

helper="$1"
tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

repo="$tmpdir/repo"
mkdir -p "$repo/.git"

(cd "$repo" && "$helper" project init --json > "$tmpdir/init.json" 2> "$tmpdir/init.err")

required_files="
.kkachi/config.yaml
.kkachi/status.json
.kkachi/events.jsonl
.kkachi/schemas/status.schema.json
.kkachi/schemas/run-metadata.schema.json
.kkachi/schemas/event.schema.json
.kkachi/schemas/selected-cli.schema.json
.kkachi/schemas/bridge-session-snapshot.schema.json
"

for relative in $required_files; do
  if [ ! -f "$repo/$relative" ]; then
    echo "missing expected file: $relative" >&2
    exit 1
  fi
done

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

assert_contains "$tmpdir/init.json" '"root_path":"'"$repo"'"' "init JSON"
assert_contains "$tmpdir/init.json" '"project_name":"repo"' "init JSON"
assert_contains "$tmpdir/init.json" '"initial_event_id":"evt-000001"' "init JSON"

(cd "$repo" && "$helper" project status --json > "$tmpdir/status.json" 2> "$tmpdir/status.err")
(cd "$repo" && "$helper" project doctor --json > "$tmpdir/doctor.json" 2> "$tmpdir/doctor.err")
assert_contains "$tmpdir/status.json" '"health":"ok"' "status JSON"
assert_contains "$tmpdir/status.json" '"event_tail_id":"evt-000001"' "status JSON"
assert_contains "$tmpdir/status.json" '"event_count":1' "status JSON"
assert_contains "$tmpdir/doctor.json" '"health":"ok"' "doctor JSON"
assert_contains "$tmpdir/doctor.json" '"failed":0' "doctor JSON"

assert_contains "$repo/.kkachi/config.yaml" 'version: "0.1"' "config.yaml"
assert_contains "$repo/.kkachi/config.yaml" 'name: "repo"' "config.yaml"
assert_contains "$repo/.kkachi/config.yaml" 'root_policy: "repository_confined_no_symlink_escape"' "config.yaml"
assert_contains "$repo/.kkachi/config.yaml" 'run_root: ".kkachi/runs"' "config.yaml"
assert_contains "$repo/.kkachi/config.yaml" 'status_file: ".kkachi/status.json"' "config.yaml"
assert_contains "$repo/.kkachi/config.yaml" 'events_file: ".kkachi/events.jsonl"' "config.yaml"
assert_contains "$repo/.kkachi/config.yaml" 'one_active_write_run: true' "config.yaml"
assert_contains "$repo/.kkachi/config.yaml" 'mode: "both"' "config.yaml"

assert_contains "$repo/.kkachi/status.json" '"version": "0.1"' "status.json"
assert_contains "$repo/.kkachi/status.json" '"project_id": "kkachi-project-repo-' "status.json"
assert_contains "$repo/.kkachi/status.json" '"active_run_id": null' "status.json"
assert_contains "$repo/.kkachi/status.json" '"active_run_state": null' "status.json"
assert_contains "$repo/.kkachi/status.json" '"last_event_id": "evt-000001"' "status.json"
assert_contains "$repo/.kkachi/status.json" '"gate_summary": {}' "status.json"

event_count="$(wc -l < "$repo/.kkachi/events.jsonl" | tr -d ' ')"
if [ "$event_count" != "1" ]; then
  echo "events.jsonl line count = $event_count, want 1" >&2
  cat "$repo/.kkachi/events.jsonl" >&2
  exit 1
fi
assert_contains "$repo/.kkachi/events.jsonl" '"version":"0.1"' "events.jsonl"
assert_contains "$repo/.kkachi/events.jsonl" '"event_id":"evt-000001"' "events.jsonl"
assert_contains "$repo/.kkachi/events.jsonl" '"run_id":null' "events.jsonl"
assert_contains "$repo/.kkachi/events.jsonl" '"type":"project.initialized"' "events.jsonl"
assert_contains "$repo/.kkachi/events.jsonl" '"actor":"helper"' "events.jsonl"
assert_contains "$repo/.kkachi/events.jsonl" '"project_id":"kkachi-project-repo-' "events.jsonl"
assert_contains "$repo/.kkachi/events.jsonl" '"project_name":"repo"' "events.jsonl"

for schema in "$repo"/.kkachi/schemas/*.schema.json; do
  assert_contains "$schema" '"$schema": "https://json-schema.org/draft/2020-12/schema"' "$schema"
  assert_contains "$schema" '"required": [' "$schema"
  assert_contains "$schema" '"version"' "$schema"
done

(cd "$repo" && "$helper" event append artifact.written --run run-abc --payload '{"path":"impl-log.md"}' --json > "$tmpdir/event.json" 2> "$tmpdir/event.err")

assert_contains "$tmpdir/event.json" '"event_id":"evt-000002"' "event append JSON"
assert_contains "$tmpdir/event.json" '"previous_id":"evt-000001"' "event append JSON"
assert_contains "$repo/.kkachi/status.json" '"last_event_id": "evt-000002"' "status.json after event append"
assert_contains "$repo/.kkachi/events.jsonl" '"event_id":"evt-000002"' "events.jsonl after event append"
assert_contains "$repo/.kkachi/events.jsonl" '"type":"artifact.written"' "events.jsonl after event append"
assert_contains "$repo/.kkachi/events.jsonl" '"run_id":"run-abc"' "events.jsonl after event append"

event_count_after_append="$(wc -l < "$repo/.kkachi/events.jsonl" | tr -d ' ')"
if [ "$event_count_after_append" != "2" ]; then
  echo "events.jsonl line count after append = $event_count_after_append, want 2" >&2
  cat "$repo/.kkachi/events.jsonl" >&2
  exit 1
fi

cat >> "$repo/.kkachi/events.jsonl" <<'EOF_CRASH'
{"version":"0.1","event_id":"evt-000003","occurred_at":"2026-04-30T03:00:00Z","run_id":"run-abc","type":"run.created","actor":"helper","payload":{}}
EOF_CRASH

if (cd "$repo" && "$helper" event append artifact.written --run run-abc --payload '{}' --json > "$tmpdir/mismatch.json" 2> "$tmpdir/mismatch.err"); then
  echo "event append succeeded despite last_event_id mismatch" >&2
  exit 1
fi

assert_contains "$tmpdir/mismatch.err" '"code":"last_event_id_mismatch"' "mismatch stderr"
assert_contains "$tmpdir/mismatch.err" '"exit_code":3' "mismatch stderr"
assert_contains "$repo/.kkachi/status.json" '"last_event_id": "evt-000002"' "status.json after refused mismatch append"

if (cd "$repo" && "$helper" project doctor --json > "$tmpdir/mismatch-doctor.json" 2> "$tmpdir/mismatch-doctor.err"); then
  echo "project doctor succeeded despite last_event_id mismatch" >&2
  exit 1
fi
assert_contains "$tmpdir/mismatch-doctor.json" '"health":"fail"' "mismatch doctor JSON"
assert_contains "$tmpdir/mismatch-doctor.json" '"name":"coherence"' "mismatch doctor JSON"
assert_contains "$tmpdir/mismatch-doctor.json" '"expected":"evt-000003"' "mismatch doctor JSON"
assert_contains "$tmpdir/mismatch-doctor.json" '"actual":"evt-000002"' "mismatch doctor JSON"

event_count_after_refused_append="$(wc -l < "$repo/.kkachi/events.jsonl" | tr -d ' ')"
if [ "$event_count_after_refused_append" != "3" ]; then
  echo "events.jsonl line count after refused append = $event_count_after_refused_append, want 3" >&2
  cat "$repo/.kkachi/events.jsonl" >&2
  exit 1
fi

if (cd "$repo" && "$helper" project init --json > "$tmpdir/retry.json" 2> "$tmpdir/retry.err"); then
  echo "second project init succeeded unexpectedly" >&2
  exit 1
fi

assert_contains "$tmpdir/retry.err" '"code":"helper_state_exists"' "retry stderr"
assert_contains "$tmpdir/retry.err" '"exit_code":3' "retry stderr"
