#!/bin/sh
set -eu

project_root="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

binary="$tmpdir/kkachi-agent-helper"
(cd "$project_root" && go build -o "$binary" ./cmd/kkachi-agent-helper)

run_scenario() {
  name="$1"
  script="$2"

  printf 'e2e %-32s ... ' "$name"
  "$script" "$binary"
  printf 'PASS\n'
}

run_scenario "corex-003 project init" "$project_root/tests/e2e/project-init.sh"
