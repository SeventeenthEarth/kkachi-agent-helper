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
repo="$(cd "$repo" && pwd -P)"

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

lock_path_for() {
  if [ "$1" = "active_run" ]; then
    printf '%s\n' "$repo/.kkachi/active_run.lock"
  else
    printf '%s\n' "$repo/.kkachi/project_write.lock"
  fi
}

assert_event_count() {
  expected="$1"
  label="$2"
  actual="$(wc -l < "$repo/.kkachi/events.jsonl" | tr -d ' ')"
  if [ "$actual" != "$expected" ]; then
    echo "$label: event count = $actual, want $expected" >&2
    cat "$repo/.kkachi/events.jsonl" >&2
    exit 1
  fi
}

write_lock() {
  lock_name="$1"
  owner_pid="$2"
  hostname="$3"
  command="$4"
  created_at="$5"
  run_id="${6:-}"
  lock_path="$(lock_path_for "$lock_name")"
  LOCK_PATH="$lock_path" LOCK_NAME="$lock_name" OWNER_PID="$owner_pid" HOSTNAME="$hostname" COMMAND="$command" CREATED_AT="$created_at" RUN_ID="$run_id" python3 - <<'PY'
import json
import os

payload = {
    "version": "0.1",
    "lock_name": os.environ["LOCK_NAME"],
    "owner_pid": int(os.environ["OWNER_PID"]),
    "hostname": os.environ["HOSTNAME"],
    "command": os.environ["COMMAND"],
    "created_at": os.environ["CREATED_AT"],
}
if os.environ["RUN_ID"]:
    payload["run_id"] = os.environ["RUN_ID"]
with open(os.environ["LOCK_PATH"], "w", encoding="utf-8") as f:
    json.dump(payload, f, indent=2)
    f.write("\n")
PY
}

create_run() {
  title="$1"
  (cd "$repo" && "$helper" run create --title "$title" --work-path A_development_execution --work-mode standard --urgency normal --sot-policy existing_sot_basis --execution-mode production_write --commander Gongmyeong --task-id runwf-002 --json)
}

(cd "$repo" && "$helper" project init --json > "$tmpdir/init.json" 2> "$tmpdir/init.err")

hostname="$(hostname 2>/dev/null || printf unknown)"
fresh_created_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
write_lock project_write "$$" "$hostname" "e2e fresh writer" "$fresh_created_at"
if create_run "Blocked by fresh project lock" > "$tmpdir/fresh-project-create.json" 2> "$tmpdir/fresh-project-create.err"; then
  echo "run create succeeded under fresh project_write lock" >&2
  exit 1
fi
assert_contains "$tmpdir/fresh-project-create.err" '"code":"lock_conflict"' "fresh project-write lock stderr"
assert_event_count 1 "events after refused create"
rm "$(lock_path_for project_write)"

create_run "Run workflow lock owner" > "$tmpdir/run-create.json" 2> "$tmpdir/run-create.err"
run_id="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["run_id"])' "$tmpdir/run-create.json")"

fresh_created_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
write_lock active_run "$$" "$hostname" "e2e active lifecycle" "$fresh_created_at" "$run_id"
if (cd "$repo" && "$helper" run activate "$run_id" --json > "$tmpdir/active-lock.json" 2> "$tmpdir/active-lock.err"); then
  echo "run activate succeeded under fresh active_run lock" >&2
  exit 1
fi
assert_contains "$tmpdir/active-lock.err" '"code":"lock_conflict"' "fresh active-run lock stderr"
assert_contains "$repo/.kkachi/status.json" '"active_run_id": null' "status after active-run lock refusal"
assert_contains "$repo/.kkachi/runs/$run_id/run-metadata.json" '"state": "created"' "metadata after active-run lock refusal"
rm "$(lock_path_for active_run)"

write_lock project_write 999999 "other-host" "e2e stale writer" "2026-04-30T02:33:05Z"
if create_run "Blocked by stale project lock" > "$tmpdir/stale-project-create.json" 2> "$tmpdir/stale-project-create.err"; then
  echo "run create succeeded under stale project_write lock before recovery" >&2
  exit 1
fi
assert_contains "$tmpdir/stale-project-create.err" '"code":"lock_stale_recovery_required"' "stale project-write lock stderr"

(cd "$repo" && "$helper" project doctor --json > "$tmpdir/stale-doctor.json" 2> "$tmpdir/stale-doctor.err")
assert_contains "$tmpdir/stale-doctor.json" '"health":"warning"' "stale doctor JSON"
assert_contains "$tmpdir/stale-doctor.json" 'lock recover' "stale doctor JSON"

(cd "$repo" && "$helper" lock recover project-write --reason "e2e stale recovery" --json > "$tmpdir/recover.json" 2> "$tmpdir/recover.err")
assert_contains "$tmpdir/recover.json" '"lock_name":"project_write"' "lock recover JSON"
if [ -e "$(lock_path_for project_write)" ]; then
  echo "project_write.lock still exists after recovery" >&2
  exit 1
fi
assert_contains "$repo/.kkachi/events.jsonl" '"type":"lock.recovered"' "events after lock recovery"

create_run "After lock recovery" > "$tmpdir/post-recovery-create.json" 2> "$tmpdir/post-recovery-create.err"
assert_contains "$tmpdir/post-recovery-create.json" '"state":"created"' "post-recovery run create JSON"
