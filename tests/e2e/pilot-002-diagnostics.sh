#!/bin/sh
set -eu

if [ "$#" -ne 1 ]; then
  echo "usage: $0 <kkachi-agent-helper-binary>" >&2
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

repo="$tmpdir/repo"
mkdir -p "$repo/.git"
secret='sk-abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ123456'

(cd "$repo" && "$helper" project init --json > "$tmpdir/init.json" 2> "$tmpdir/init.err")
(cd "$repo" && "$helper" run create --title "Pilot 002 diagnostics" --work-path A_development_execution --work-mode standard --urgency normal --sot-policy existing_sot_basis --execution-mode adapter_qa --commander Gongmyeong --task-id pilot-002 --json > "$tmpdir/run.json" 2> "$tmpdir/run.err")
run_id="$(json_field "$tmpdir/run.json" run_id)"
(cd "$repo" && "$helper" artifact init "$run_id" --json > "$tmpdir/artifact-init.json" 2> "$tmpdir/artifact-init.err")
printf '{"version":"0.1","status":"pending","api_token":"%s"}\n' "$secret" > "$repo/.kkachi/runs/$run_id/selected-cli.json"
(cd "$repo" && "$helper" event append diagnostic.secret --run "$run_id" --payload '{"access_token":"'$secret'"}' --json > "$tmpdir/event.json" 2> "$tmpdir/event.err")
if (cd "$repo" && "$helper" gate check "$run_id" intake --json > "$tmpdir/intake.json" 2> "$tmpdir/intake.err"); then
  echo "intake gate unexpectedly passed with pending artifacts" >&2
  exit 1
fi

(cd "$repo" && "$helper" diagnostics export --run "$run_id" --json > "$tmpdir/diagnostics.json" 2> "$tmpdir/diagnostics.err")
assert_contains "$tmpdir/diagnostics.json" '"version":"0.1"' "diagnostics JSON"
assert_contains "$tmpdir/diagnostics.json" '"schema_versions":' "diagnostics JSON"
assert_contains "$tmpdir/diagnostics.json" '"gate_reports":' "diagnostics JSON"
assert_contains "$tmpdir/diagnostics.json" '"selected_artifacts":' "diagnostics JSON"
assert_contains "$tmpdir/diagnostics.json" '"api_token":"[REDACTED]"' "diagnostics JSON"
assert_not_contains "$tmpdir/diagnostics.json" "$secret" "diagnostics JSON"

(cd "$repo" && "$helper" diagnostics export --run "$run_id" --output diagnostics/pilot-002.json > "$tmpdir/diagnostics-human.txt" 2> "$tmpdir/diagnostics-output.err")
assert_contains "$tmpdir/diagnostics-human.txt" 'diagnostics bundle exported: diagnostics/pilot-002.json' "diagnostics human output"
assert_contains "$repo/diagnostics/pilot-002.json" '"run_id": "'"$run_id"'"' "written diagnostics bundle"
assert_contains "$repo/diagnostics/pilot-002.json" '"api_token": "[REDACTED]"' "written diagnostics bundle"
assert_not_contains "$repo/diagnostics/pilot-002.json" "$secret" "written diagnostics bundle"

if (cd "$repo" && "$helper" diagnostics export --output ../api_token="$secret" --json > "$tmpdir/redact-stdout.json" 2> "$tmpdir/redact-stderr.json"); then
  echo "unsafe diagnostics output unexpectedly succeeded" >&2
  exit 1
fi
assert_contains "$tmpdir/redact-stderr.json" '"code":"path_escape"' "redacted diagnostics error"
assert_contains "$tmpdir/redact-stderr.json" '[REDACTED]' "redacted diagnostics error"
assert_not_contains "$tmpdir/redact-stderr.json" "$secret" "redacted diagnostics error"
