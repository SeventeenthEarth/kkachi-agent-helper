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

(cd "$repo" && "$helper" project init --json > "$tmpdir/init.json" 2> "$tmpdir/init.err")

required_files="
.kkachi/config.yaml
.kkachi/status.json
.kkachi/events.jsonl
.kkachi/schemas/config.schema.json
.kkachi/schemas/status.schema.json
.kkachi/schemas/event.schema.json
.kkachi/schemas/run-metadata.schema.json
.kkachi/schemas/selected-cli.schema.json
.kkachi/schemas/bridge-session-snapshot.schema.json
.kkachi/schemas/install-manifest.schema.json
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

(cd "$repo" && "$helper" schema validate .kkachi/config.yaml --schema config --json > "$tmpdir/schema-config.json" 2> "$tmpdir/schema-config.err")
(cd "$repo" && "$helper" schema validate .kkachi/status.json --schema .kkachi/schemas/status.schema.json --json > "$tmpdir/schema-status.json" 2> "$tmpdir/schema-status.err")
(cd "$repo" && "$helper" schema validate .kkachi/events.jsonl --schema event --json > "$tmpdir/schema-events.json" 2> "$tmpdir/schema-events.err")
assert_contains "$tmpdir/schema-config.json" '"schema":"config"' "schema config JSON"
assert_contains "$tmpdir/schema-config.json" '"status":"pass"' "schema config JSON"
assert_contains "$tmpdir/schema-status.json" '"schema":"status"' "schema status JSON"
assert_contains "$tmpdir/schema-status.json" '"status":"pass"' "schema status JSON"
assert_contains "$tmpdir/schema-events.json" '"schema":"event"' "schema events JSON"
assert_contains "$tmpdir/schema-events.json" '"status":"pass"' "schema events JSON"


install_source="$repo/fixtures/install-source"
mkdir -p "$install_source/skills/create" "$install_source/skills/update" "$install_source/skills/preserve" "$repo/.codex/skills/update" "$repo/.codex/skills/preserve"
printf '%s\ncreate\n' '<!-- kkachi-agent-helper:managed -->' > "$install_source/skills/create/SKILL.md"
printf '%s\nnew\n' '<!-- kkachi-agent-helper:managed -->' > "$install_source/skills/update/SKILL.md"
printf '%s\nupstream\n' '<!-- kkachi-agent-helper:managed -->' > "$install_source/skills/preserve/SKILL.md"
printf '%s\nold\n' '<!-- kkachi-agent-helper:managed -->' > "$repo/.codex/skills/update/SKILL.md"
printf 'user custom\n' > "$repo/.codex/skills/preserve/SKILL.md"
python3 - "$install_source" <<'PY_INSTALL'
import hashlib, json, pathlib, sys
root = pathlib.Path(sys.argv[1])
items = []
for source, target in [
    ("skills/create/SKILL.md", ".codex/skills/create/SKILL.md"),
    ("skills/update/SKILL.md", ".codex/skills/update/SKILL.md"),
    ("skills/preserve/SKILL.md", ".codex/skills/preserve/SKILL.md"),
]:
    data = (root / source).read_bytes()
    items.append({"source": source, "target": target, "sha256": hashlib.sha256(data).hexdigest(), "owner_marker": "<!-- kkachi-agent-helper:managed -->"})
manifest = {"version": "0.1", "kind": "skills", "package": {"name": "kkachi-e2e-pack", "version": "0.1.0"}, "compat": {"required_helper": ">=0.1.0", "required_bridge": ">=0.1.0", "required_skills": ">=0.1.0"}, "items": items}
(root / "kkachi-install-manifest.json").write_text(json.dumps(manifest, indent=2) + "\n")
PY_INSTALL
install_events_before="$tmpdir/install-events-before.jsonl"
cp "$repo/.kkachi/events.jsonl" "$install_events_before"
(cd "$repo" && "$helper" install skills --source "$install_source" --dry-run --json > "$tmpdir/install-dry-run.json" 2> "$tmpdir/install-dry-run.err")
assert_contains "$tmpdir/install-dry-run.json" '"dry_run":true' "install dry-run JSON"
assert_contains "$tmpdir/install-dry-run.json" '"kind":"skills"' "install dry-run JSON"
assert_contains "$tmpdir/install-dry-run.json" '"create":1' "install dry-run JSON"
assert_contains "$tmpdir/install-dry-run.json" '"update":1' "install dry-run JSON"
assert_contains "$tmpdir/install-dry-run.json" '"preserve":1' "install dry-run JSON"
assert_contains "$tmpdir/install-dry-run.json" '"target":".codex/skills/create/SKILL.md"' "install dry-run JSON"
assert_contains "$repo/.codex/skills/update/SKILL.md" 'old' "install dry-run target preservation"
assert_contains "$repo/.codex/skills/preserve/SKILL.md" 'user custom' "install dry-run user-owned preservation"
if [ -e "$repo/.codex/skills/create/SKILL.md" ]; then
  echo "install dry-run unexpectedly created target" >&2
  exit 1
fi
if ! cmp -s "$install_events_before" "$repo/.kkachi/events.jsonl"; then
  echo "install dry-run unexpectedly mutated repo events" >&2
  diff -u "$install_events_before" "$repo/.kkachi/events.jsonl" >&2 || true
  exit 1
fi
if (cd "$repo" && "$helper" install skills --source "$install_source" --json > "$tmpdir/install-real.json" 2> "$tmpdir/install-real.err"); then
  echo "non-dry-run install unexpectedly succeeded" >&2
  exit 1
fi
assert_contains "$tmpdir/install-real.err" '"code":"install_real_not_implemented"' "install real error"
if (cd "$repo" && "$helper" install skills --source "$repo/fixtures/missing-install-source" --dry-run --json > "$tmpdir/install-missing-source.json" 2> "$tmpdir/install-missing-source.err"); then
  echo "missing source install unexpectedly succeeded" >&2
  exit 1
fi
assert_contains "$tmpdir/install-missing-source.err" '"code":"install_source_invalid"' "install missing source error"
(cd "$repo" && "$helper" schema validate fixtures/install-source/kkachi-install-manifest.json --schema install-manifest --json > "$tmpdir/install-manifest-schema.json" 2> "$tmpdir/install-manifest-schema.err")
assert_contains "$tmpdir/install-manifest-schema.json" '"schema":"install-manifest"' "install manifest schema JSON"
assert_contains "$tmpdir/install-manifest-schema.json" '"status":"pass"' "install manifest schema JSON"

export_repo="$tmpdir/export-repo"
mkdir -p "$export_repo/.git"
export_repo="$(cd "$export_repo" && pwd -P)"
(cd "$export_repo" && "$helper" project init --json > "$tmpdir/export-init.json" 2> "$tmpdir/export-init.err")
printf '%s\n' '{"$id":"https://kkachi.local/schemas/status.schema.json","version":"0.1"}' > "$export_repo/.kkachi/schemas/status.schema.json"
(cd "$export_repo" && "$helper" schema export --all --dry-run --json > "$tmpdir/schema-export-dry-run.json" 2> "$tmpdir/schema-export-dry-run.err")
assert_contains "$tmpdir/schema-export-dry-run.json" '"dry_run":true' "schema export dry-run JSON"
assert_contains "$tmpdir/schema-export-dry-run.json" '"would_write":[".kkachi/schemas/status.schema.json"]' "schema export dry-run JSON"
(cd "$export_repo" && "$helper" schema export --all --json > "$tmpdir/schema-export.json" 2> "$tmpdir/schema-export.err")
assert_contains "$tmpdir/schema-export.json" '"written":[".kkachi/schemas/status.schema.json"]' "schema export JSON"
assert_contains "$tmpdir/schema-export.json" '"event_id":"evt-000002"' "schema export JSON"
assert_contains "$export_repo/.kkachi/events.jsonl" '"type":"schema.exported"' "schema export events"
(cd "$export_repo" && "$helper" schema export --all --json > "$tmpdir/schema-export-idempotent.json" 2> "$tmpdir/schema-export-idempotent.err")
assert_contains "$tmpdir/schema-export-idempotent.json" '"written":null' "schema export idempotent JSON"
if grep -Fq '"event_id"' "$tmpdir/schema-export-idempotent.json"; then
  echo "idempotent schema export unexpectedly recorded an event" >&2
  cat "$tmpdir/schema-export-idempotent.json" >&2
  exit 1
fi

(cd "$export_repo" && "$helper" schema migrate --from 0.1 --to 0.1 --dry-run --json > "$tmpdir/schema-migrate-dry-run.json" 2> "$tmpdir/schema-migrate-dry-run.err")
assert_contains "$tmpdir/schema-migrate-dry-run.json" '"dry_run":true' "schema migrate dry-run JSON"
assert_contains "$tmpdir/schema-migrate-dry-run.json" '"from_version":"0.1"' "schema migrate dry-run JSON"
assert_contains "$tmpdir/schema-migrate-dry-run.json" '"to_version":"0.1"' "schema migrate dry-run JSON"
assert_contains "$tmpdir/schema-migrate-dry-run.json" '"would_backup":[' "schema migrate dry-run JSON"
if grep -Fq '"event_id"' "$tmpdir/schema-migrate-dry-run.json"; then
  echo "dry-run schema migrate unexpectedly recorded an event" >&2
  cat "$tmpdir/schema-migrate-dry-run.json" >&2
  exit 1
fi

(cd "$export_repo" && "$helper" schema migrate --from 0.1 --to 0.1 --json > "$tmpdir/schema-migrate.json" 2> "$tmpdir/schema-migrate.err")
assert_contains "$tmpdir/schema-migrate.json" '"dry_run":false' "schema migrate JSON"
assert_contains "$tmpdir/schema-migrate.json" '"migration":"noop-0.1-to-0.1"' "schema migrate JSON"
assert_contains "$tmpdir/schema-migrate.json" '"event_id":"evt-000003"' "schema migrate JSON"
assert_contains "$tmpdir/schema-migrate.json" '"backup_path":".kkachi/backups/schema-migrations/' "schema migrate JSON"
assert_contains "$export_repo/.kkachi/events.jsonl" '"type":"schema.migrated"' "schema migrate events"
backup_path="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["backup_path"])' "$tmpdir/schema-migrate.json")"
if [ ! -f "$export_repo/$backup_path/.kkachi/status.json" ]; then
  echo "schema migration backup missing status.json at $backup_path" >&2
  exit 1
fi

if (cd "$export_repo" && "$helper" schema migrate --from 9.9 --to 0.1 --json > "$tmpdir/schema-migrate-unknown.json" 2> "$tmpdir/schema-migrate-unknown.err"); then
  echo "schema migrate accepted unknown source version" >&2
  exit 1
fi
assert_contains "$tmpdir/schema-migrate-unknown.err" '"code":"schema_migration_unknown_source_version"' "schema migrate unknown source error"

(cd "$export_repo" && "$helper" run create --title "Packg e2e metadata" --work-path A_development_execution --work-mode standard --urgency normal --sot-policy existing_sot_basis --execution-mode production_write --commander Gongmyeong --task-id packg-002 --json > "$tmpdir/schema-migrate-run-create.json" 2> "$tmpdir/schema-migrate-run-create.err")
migrate_run_id="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["run_id"])' "$tmpdir/schema-migrate-run-create.json")"
(cd "$export_repo" && "$helper" schema migrate --from 0.1 --to 0.1 --json > "$tmpdir/schema-migrate-with-run.json" 2> "$tmpdir/schema-migrate-with-run.err")
run_backup_path="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["backup_path"])' "$tmpdir/schema-migrate-with-run.json")"
if [ ! -f "$export_repo/$run_backup_path/.kkachi/runs/$migrate_run_id/run-metadata.json" ]; then
  echo "schema migration backup missing run-metadata.json at $run_backup_path" >&2
  exit 1
fi
assert_contains "$export_repo/$run_backup_path/.kkachi/runs/$migrate_run_id/run-metadata.json" '"task_id": "packg-002"' "schema migration run metadata backup"

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

(cd "$repo" && "$helper" run create --title "Run workflow metadata" --work-path A_development_execution --work-mode standard --urgency normal --sot-policy existing_sot_basis --execution-mode production_write --commander Gongmyeong --task-id runwf-001 --json > "$tmpdir/run-create.json" 2> "$tmpdir/run-create.err")
run_id="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["run_id"])' "$tmpdir/run-create.json")"
case "$run_id" in
  run-????????T??????Z-????????????) ;;
  *) echo "unexpected run id: $run_id" >&2; cat "$tmpdir/run-create.json" >&2; exit 1 ;;
esac
assert_contains "$tmpdir/run-create.json" '"state":"created"' "run create JSON"
assert_contains "$tmpdir/run-create.json" '"event_id":"evt-000002"' "run create JSON"
assert_contains "$tmpdir/run-create.json" '"task_id":"runwf-001"' "run create JSON"
assert_contains "$tmpdir/run-create.json" '"required_artifacts":[]' "run create JSON"
assert_contains "$repo/.kkachi/runs/$run_id/run-metadata.json" '"state": "created"' "run-metadata.json"
assert_contains "$repo/.kkachi/events.jsonl" '"type":"run.created"' "events.jsonl after run create"

(cd "$repo" && "$helper" run list --json > "$tmpdir/run-list.json" 2> "$tmpdir/run-list.err")
assert_contains "$tmpdir/run-list.json" '"run_id":"'"$run_id"'"' "run list JSON"
assert_contains "$tmpdir/run-list.json" '"state":"created"' "run list JSON"

run_prefix="$(printf '%s' "$run_id" | cut -c1-24)"
(cd "$repo" && "$helper" run show "$run_prefix" --json > "$tmpdir/run-show.json" 2> "$tmpdir/run-show.err")
assert_contains "$tmpdir/run-show.json" '"run_id":"'"$run_id"'"' "run show JSON"
assert_contains "$tmpdir/run-show.json" '"gate_state":{}' "run show JSON"

(cd "$repo" && "$helper" artifact init "$run_id" --json > "$tmpdir/artifact-init.json" 2> "$tmpdir/artifact-init.err")
assert_contains "$tmpdir/artifact-init.json" '"event_id":"evt-000003"' "artifact init JSON"
assert_contains "$tmpdir/artifact-init.json" '"path":"intake-classification.md"' "artifact init JSON"
assert_contains "$tmpdir/artifact-init.json" '"required_artifacts":[' "artifact init JSON"
assert_contains "$repo/.kkachi/events.jsonl" '"type":"artifact.written"' "events.jsonl after artifact init"
assert_contains "$repo/.kkachi/runs/$run_id/run-metadata.json" '"required_artifacts": [' "run-metadata.json after artifact init"
assert_contains "$repo/.kkachi/runs/$run_id/run-metadata.json" '"diff.patch"' "run-metadata.json after artifact init"
for artifact in intake-classification.md sot-basis.md acceptance-criteria.md plan.md checklist.md diff.patch impl-log.md test-log.md verification.md docs-update.md final-report.md redteam/final-gate-review.md; do
  if [ ! -s "$repo/.kkachi/runs/$run_id/$artifact" ]; then
    echo "missing or empty initialized artifact: $artifact" >&2
    exit 1
  fi
done
(cd "$repo" && "$helper" artifact list "$run_prefix" --json > "$tmpdir/artifact-list.json" 2> "$tmpdir/artifact-list.err")
assert_contains "$tmpdir/artifact-list.json" '"run_id":"'"$run_id"'"' "artifact list JSON"
assert_contains "$tmpdir/artifact-list.json" '"path":"intake-classification.md","required":true,"exists":true' "artifact list JSON"

if (cd "$repo" && "$helper" artifact validate "$run_id" --json > "$tmpdir/artifact-validate-pending.json" 2> "$tmpdir/artifact-validate-pending.err"); then
  echo "artifact validate succeeded with baseline pending intake" >&2
  exit 1
fi
assert_contains "$tmpdir/artifact-validate-pending.json" '"status":"fail"' "pending artifact validate JSON"
assert_contains "$tmpdir/artifact-validate-pending.json" '"name":"intake_status"' "pending artifact validate JSON"

cat > "$repo/.kkachi/runs/$run_id/intake-classification.md" <<EOF_INTAKE
# intake-classification.md

Status: complete
Work Path: A_development_execution
Work Mode: standard
SOT Policy: existing_sot_basis
Urgency: normal
EOF_INTAKE

(cd "$repo" && "$helper" artifact validate "$run_prefix" --gate intake --json > "$tmpdir/artifact-validate.json" 2> "$tmpdir/artifact-validate.err")
assert_contains "$tmpdir/artifact-validate.json" '"run_id":"'"$run_id"'"' "artifact validate JSON"
assert_contains "$tmpdir/artifact-validate.json" '"gate":"intake"' "artifact validate JSON"
assert_contains "$tmpdir/artifact-validate.json" '"status":"pass"' "artifact validate JSON"
assert_contains "$tmpdir/artifact-validate.json" '"name":"required_artifacts","status":"pass"' "artifact validate JSON"
event_count_after_validate="$(wc -l < "$repo/.kkachi/events.jsonl" | tr -d ' ')"
if [ "$event_count_after_validate" != "3" ]; then
  echo "events.jsonl line count after artifact validate = $event_count_after_validate, want 3" >&2
  cat "$repo/.kkachi/events.jsonl" >&2
  exit 1
fi

(cd "$repo" && "$helper" gate check "$run_prefix" intake --json > "$tmpdir/gate-check.json" 2> "$tmpdir/gate-check.err")
assert_contains "$tmpdir/gate-check.json" '"run_id":"'"$run_id"'"' "gate check JSON"
assert_contains "$tmpdir/gate-check.json" '"gate":"intake"' "gate check JSON"
assert_contains "$tmpdir/gate-check.json" '"status":"pass"' "gate check JSON"
assert_contains "$tmpdir/gate-check.json" '"event_id":"evt-000004"' "gate check JSON"
assert_contains "$tmpdir/gate-check.json" '"missing_evidence":[]' "gate check JSON"
assert_contains "$tmpdir/gate-check.json" '"report_path":".kkachi/runs/'"$run_id"'/gate-reports/intake.json"' "gate check JSON"
assert_contains "$repo/.kkachi/runs/$run_id/gate-reports/intake.json" '"gate": "intake"' "intake gate report"
assert_contains "$repo/.kkachi/runs/$run_id/gate-reports/intake.json" '"status": "pass"' "intake gate report"
assert_contains "$repo/.kkachi/runs/$run_id/gate-reports/intake.json" '"event_id": "evt-000004"' "intake gate report"
assert_contains "$repo/.kkachi/events.jsonl" '"type":"gate.passed"' "events.jsonl after gate check"
assert_contains "$repo/.kkachi/status.json" '"gate_summary": {' "status.json after gate check"
assert_contains "$repo/.kkachi/status.json" '"intake": {' "status.json after gate check"
assert_contains "$repo/.kkachi/status.json" '"event_id": "evt-000004"' "status.json after gate check"
assert_contains "$repo/.kkachi/runs/$run_id/run-metadata.json" '"gate_state": {' "run-metadata.json after gate check"
assert_contains "$repo/.kkachi/runs/$run_id/run-metadata.json" '"status": "pass"' "run-metadata.json after gate check"
assert_contains "$repo/.kkachi/runs/$run_id/run-metadata.json" '"report_path": ".kkachi/runs/'"$run_id"'/gate-reports/intake.json"' "run-metadata.json after gate check"

event_count_after_gate="$(wc -l < "$repo/.kkachi/events.jsonl" | tr -d ' ')"
if [ "$event_count_after_gate" != "4" ]; then
  echo "events.jsonl line count after gate check = $event_count_after_gate, want 4" >&2
  cat "$repo/.kkachi/events.jsonl" >&2
  exit 1
fi

cat > "$repo/.kkachi/runs/$run_id/sot-basis.md" <<EOF_SOT
# sot-basis.md

Status: complete
Source: docs/specs.md
EOF_SOT
cat > "$repo/.kkachi/runs/$run_id/acceptance-criteria.md" <<EOF_ACCEPTANCE
# acceptance-criteria.md

Status: complete
Criteria: deterministic pre-implementation gates pass/fail from artifacts
EOF_ACCEPTANCE
cat > "$repo/.kkachi/runs/$run_id/plan.md" <<EOF_PLAN
# plan.md

Status: complete
Plan: implement gates-002 SOT, roadmap, and plan checks
EOF_PLAN
cat > "$repo/.kkachi/runs/$run_id/checklist.md" <<EOF_CHECKLIST
# checklist.md

Status: complete
- [x] SOT gate
- [x] Roadmap gate
- [x] Plan gate
EOF_CHECKLIST

(cd "$repo" && "$helper" gate check "$run_id" sot --json > "$tmpdir/gate-sot.json" 2> "$tmpdir/gate-sot.err")
assert_contains "$tmpdir/gate-sot.json" '"gate":"sot"' "SOT gate JSON"
assert_contains "$tmpdir/gate-sot.json" '"status":"pass"' "SOT gate JSON"
assert_contains "$tmpdir/gate-sot.json" '"event_id":"evt-000005"' "SOT gate JSON"
assert_contains "$tmpdir/gate-sot.json" '"name":"sot_basis","status":"pass"' "SOT gate JSON"
assert_contains "$repo/.kkachi/runs/$run_id/gate-reports/sot.json" '"event_id": "evt-000005"' "SOT gate report"

(cd "$repo" && "$helper" gate check "$run_id" roadmap --json > "$tmpdir/gate-roadmap.json" 2> "$tmpdir/gate-roadmap.err")
assert_contains "$tmpdir/gate-roadmap.json" '"gate":"roadmap"' "roadmap gate JSON"
assert_contains "$tmpdir/gate-roadmap.json" '"status":"pass"' "roadmap gate JSON"
assert_contains "$tmpdir/gate-roadmap.json" '"event_id":"evt-000006"' "roadmap gate JSON"
assert_contains "$tmpdir/gate-roadmap.json" '"name":"roadmap_trace","status":"pass"' "roadmap gate JSON"

(cd "$repo" && "$helper" gate check "$run_id" plan --json > "$tmpdir/gate-plan.json" 2> "$tmpdir/gate-plan.err")
assert_contains "$tmpdir/gate-plan.json" '"gate":"plan"' "plan gate JSON"
assert_contains "$tmpdir/gate-plan.json" '"status":"pass"' "plan gate JSON"
assert_contains "$tmpdir/gate-plan.json" '"event_id":"evt-000007"' "plan gate JSON"
assert_contains "$tmpdir/gate-plan.json" '"name":"checklist_artifact","status":"pass"' "plan gate JSON"
assert_contains "$repo/.kkachi/runs/$run_id/gate-reports/plan.json" '"event_id": "evt-000007"' "plan gate report"

(cd "$repo" && "$helper" run activate "$run_id" --json > "$tmpdir/run-activate.json" 2> "$tmpdir/run-activate.err")
assert_contains "$tmpdir/run-activate.json" '"state":"active"' "run activate JSON"
assert_contains "$tmpdir/run-activate.json" '"event_id":"evt-000008"' "run activate JSON"
assert_contains "$repo/.kkachi/status.json" '"active_run_id": "'"$run_id"'"' "status.json after run activate"
assert_contains "$repo/.kkachi/status.json" '"active_run_state": "active"' "status.json after run activate"

(cd "$repo" && "$helper" run close "$run_id" --json > "$tmpdir/run-close.json" 2> "$tmpdir/run-close.err")
assert_contains "$tmpdir/run-close.json" '"state":"closed"' "run close JSON"
assert_contains "$tmpdir/run-close.json" '"event_id":"evt-000009"' "run close JSON"
assert_contains "$repo/.kkachi/status.json" '"active_run_id": null' "status.json after run close"
assert_contains "$repo/.kkachi/status.json" '"active_run_state": null' "status.json after run close"
assert_contains "$repo/.kkachi/events.jsonl" '"type":"run.closed"' "events.jsonl after run close"

for schema in "$repo"/.kkachi/schemas/*.schema.json; do
  assert_contains "$schema" '"$schema": "https://json-schema.org/draft/2020-12/schema"' "$schema"
  assert_contains "$schema" '"required": [' "$schema"
  assert_contains "$schema" '"version"' "$schema"
done

(cd "$repo" && "$helper" event append artifact.written --run run-abc --payload '{"path":"impl-log.md"}' --json > "$tmpdir/event.json" 2> "$tmpdir/event.err")

assert_contains "$tmpdir/event.json" '"event_id":"evt-000010"' "event append JSON"
assert_contains "$tmpdir/event.json" '"previous_id":"evt-000009"' "event append JSON"
assert_contains "$repo/.kkachi/status.json" '"last_event_id": "evt-000010"' "status.json after event append"
assert_contains "$repo/.kkachi/events.jsonl" '"event_id":"evt-000010"' "events.jsonl after event append"
assert_contains "$repo/.kkachi/events.jsonl" '"type":"artifact.written"' "events.jsonl after event append"
assert_contains "$repo/.kkachi/events.jsonl" '"run_id":"run-abc"' "events.jsonl after event append"

event_count_after_append="$(wc -l < "$repo/.kkachi/events.jsonl" | tr -d ' ')"
if [ "$event_count_after_append" != "10" ]; then
  echo "events.jsonl line count after append = $event_count_after_append, want 10" >&2
  cat "$repo/.kkachi/events.jsonl" >&2
  exit 1
fi

(cd "$repo" && "$helper" run create --title 'Adapter QA backend gate' --work-path A_development_execution --work-mode standard --urgency normal --sot-policy existing_sot_basis --execution-mode adapter_qa --commander Gongmyeong --task-id gates-003 --json > "$tmpdir/adapter-run.json" 2> "$tmpdir/adapter-run.err")
adapter_run_id="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["run_id"])' "$tmpdir/adapter-run.json")"
if [ -z "$adapter_run_id" ]; then
  echo "could not parse adapter QA run id" >&2
  cat "$tmpdir/adapter-run.json" >&2
  exit 1
fi
assert_contains "$tmpdir/adapter-run.json" '"event_id":"evt-000011"' "adapter run create JSON"
(cd "$repo" && "$helper" artifact init "$adapter_run_id" --json > "$tmpdir/adapter-artifact-init.json" 2> "$tmpdir/adapter-artifact-init.err")
assert_contains "$tmpdir/adapter-artifact-init.json" '"event_id":"evt-000012"' "adapter artifact init JSON"
assert_contains "$repo/.kkachi/runs/$adapter_run_id/run-metadata.json" '"selected-cli.json"' "adapter run metadata"
if (cd "$repo" && "$helper" gate check "$adapter_run_id" backend --json > "$tmpdir/adapter-backend-pending.json" 2> "$tmpdir/adapter-backend-pending.err"); then
  echo "backend gate succeeded with baseline pending evidence" >&2
  cat "$tmpdir/adapter-backend-pending.json" >&2
  exit 1
fi
assert_contains "$tmpdir/adapter-backend-pending.json" '"gate":"backend"' "adapter pending backend gate JSON"
assert_contains "$tmpdir/adapter-backend-pending.json" '"status":"fail"' "adapter pending backend gate JSON"
assert_contains "$tmpdir/adapter-backend-pending.json" '"event_id":"evt-000013"' "adapter pending backend gate JSON"
assert_contains "$tmpdir/adapter-backend-pending.json" '"name":"selected_cli","status":"fail"' "adapter pending backend gate JSON"
cat > "$repo/.kkachi/runs/$adapter_run_id/selected-cli.json" <<'EOF_SELECTED_CLI'
{
  "version": "0.1",
  "status": "supported",
  "backend_type": "codex",
  "adapter_type": "openai-codex",
  "source_ledger_ref": "docs/ledger.md#codex",
  "caveats": []
}
EOF_SELECTED_CLI
cat > "$repo/.kkachi/runs/$adapter_run_id/capability-check.md" <<'EOF_CAPABILITY'
# capability-check.md

Status: complete
Backend Type: codex
Adapter Type: openai-codex
Capability: thread resume checked
EOF_CAPABILITY
cat > "$repo/.kkachi/runs/$adapter_run_id/bridge-session-snapshot.json" <<'EOF_BRIDGE_SNAPSHOT'
{
  "session_id": "session-123",
  "backend_type": "codex",
  "adapter_type": "openai-codex",
  "state": "running",
  "lifecycle_class": "interactive",
  "open_pendings": 0
}
EOF_BRIDGE_SNAPSHOT
cat > "$repo/.kkachi/runs/$adapter_run_id/bridge-events.md" <<'EOF_BRIDGE_EVENTS'
# bridge-events.md

Status: complete
Event: bridge opened a codex session and emitted output
EOF_BRIDGE_EVENTS
(cd "$repo" && "$helper" gate check "$adapter_run_id" backend --json > "$tmpdir/adapter-backend-pass.json" 2> "$tmpdir/adapter-backend-pass.err")
assert_contains "$tmpdir/adapter-backend-pass.json" '"gate":"backend"' "adapter passing backend gate JSON"
assert_contains "$tmpdir/adapter-backend-pass.json" '"status":"pass"' "adapter passing backend gate JSON"
assert_contains "$tmpdir/adapter-backend-pass.json" '"event_id":"evt-000014"' "adapter passing backend gate JSON"
assert_contains "$tmpdir/adapter-backend-pass.json" '"name":"bridge_events","status":"pass"' "adapter passing backend gate JSON"
assert_contains "$repo/.kkachi/runs/$adapter_run_id/gate-reports/backend.json" '"event_id": "evt-000014"' "adapter backend gate report"
assert_contains "$repo/.kkachi/events.jsonl" '"type":"gate.failed"' "events.jsonl after adapter pending backend gate"
assert_contains "$repo/.kkachi/events.jsonl" '"type":"gate.passed"' "events.jsonl after adapter passing backend gate"
assert_contains "$repo/.kkachi/status.json" '"backend": {' "status.json after adapter backend gate"
assert_contains "$repo/.kkachi/runs/$adapter_run_id/run-metadata.json" '"backend": {' "adapter run metadata after backend gate"

cat >> "$repo/.kkachi/events.jsonl" <<'EOF_CRASH'
{"version":"0.1","event_id":"evt-000015","occurred_at":"2026-04-30T03:00:00Z","run_id":"run-abc","type":"run.created","actor":"helper","payload":{}}
EOF_CRASH

if (cd "$repo" && "$helper" event append artifact.written --run run-abc --payload '{}' --json > "$tmpdir/mismatch.json" 2> "$tmpdir/mismatch.err"); then
  echo "event append succeeded despite last_event_id mismatch" >&2
  exit 1
fi

assert_contains "$tmpdir/mismatch.err" '"code":"last_event_id_mismatch"' "mismatch stderr"
assert_contains "$tmpdir/mismatch.err" '"exit_code":3' "mismatch stderr"
assert_contains "$repo/.kkachi/status.json" '"last_event_id": "evt-000014"' "status.json after refused mismatch append"

if (cd "$repo" && "$helper" project doctor --json > "$tmpdir/mismatch-doctor.json" 2> "$tmpdir/mismatch-doctor.err"); then
  echo "project doctor succeeded despite last_event_id mismatch" >&2
  exit 1
fi
assert_contains "$tmpdir/mismatch-doctor.json" '"health":"fail"' "mismatch doctor JSON"
assert_contains "$tmpdir/mismatch-doctor.json" '"name":"coherence"' "mismatch doctor JSON"
assert_contains "$tmpdir/mismatch-doctor.json" '"expected":"evt-000015"' "mismatch doctor JSON"
assert_contains "$tmpdir/mismatch-doctor.json" '"actual":"evt-000014"' "mismatch doctor JSON"

event_count_after_refused_append="$(wc -l < "$repo/.kkachi/events.jsonl" | tr -d ' ')"
if [ "$event_count_after_refused_append" != "15" ]; then
  echo "events.jsonl line count after refused append = $event_count_after_refused_append, want 15" >&2
  cat "$repo/.kkachi/events.jsonl" >&2
  exit 1
fi

if (cd "$repo" && "$helper" project init --json > "$tmpdir/retry.json" 2> "$tmpdir/retry.err"); then
  echo "second project init succeeded unexpectedly" >&2
  exit 1
fi

assert_contains "$tmpdir/retry.err" '"code":"helper_state_exists"' "retry stderr"
assert_contains "$tmpdir/retry.err" '"exit_code":3' "retry stderr"
