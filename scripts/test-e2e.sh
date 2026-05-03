#!/bin/sh
set -eu

project_root="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

binary="$tmpdir/kkachi-agent-helper"
(cd "$project_root" && go build -ldflags "-X main.version=0.1.0" -o "$binary" ./cmd/kkachi-agent-helper)

run_scenario() {
  name="$1"
  script="$2"

  printf 'e2e %-32s ... ' "$name"
  "$script" "$binary"
  printf 'PASS\n'
}

run_scenario "runwf-001/003 lifecycle artifacts" "$project_root/tests/e2e/project-init.sh"
run_scenario "runwf-002 lock recovery" "$project_root/tests/e2e/runwf-002-locks.sh"
