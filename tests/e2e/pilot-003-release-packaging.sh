#!/bin/sh
set -eu

# The first argument is the already-built helper from the common harness; release
# packaging intentionally rebuilds through the public Make targets to verify the
# documented adoption path rather than that shared test binary.
_ignored_binary="$1"
project_root="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

dist="$tmpdir/dist"
prefix="$tmpdir/prefix"
goos="$(go env GOOS)"
goarch="$(go env GOARCH)"
name="kkachi-agent-helper_0.1.0_${goos}_${goarch}"

if VERSION='0.1.0"bad' \
  COMMIT=e2e \
  BUILD_DATE=2026-01-01T00:00:00Z \
  DIST_DIR="$tmpdir/invalid-dist" \
  GOOS="$goos" \
  GOARCH="$goarch" \
  "$project_root/scripts/build-release.sh" >"$tmpdir/invalid-release.out" 2>"$tmpdir/invalid-release.err"; then
  echo "release unexpectedly accepted an unsafe VERSION value" >&2
  exit 1
fi
grep -F 'error: VERSION' "$tmpdir/invalid-release.err" >/dev/null

make -C "$project_root" \
  VERSION=0.1.0 \
  COMMIT=e2e \
  BUILD_DATE=2026-01-01T00:00:00Z \
  DIST_DIR="$dist" \
  GOOS="$goos" \
  GOARCH="$goarch" \
  release >"$tmpdir/release.out"

artifact="$dist/$name"
archive="$artifact.tar.gz"
checksums="$dist/SHA256SUMS"

[ -x "$artifact" ]
[ -s "$archive" ]
[ -s "$checksums" ]

grep -F "$name" "$checksums" >/dev/null
if command -v shasum >/dev/null 2>&1; then
  (cd "$dist" && shasum -a 256 -c SHA256SUMS >/dev/null)
else
  (cd "$dist" && sha256sum -c SHA256SUMS >/dev/null)
fi

printf 'tamper' >> "$artifact"
if command -v shasum >/dev/null 2>&1; then
  if (cd "$dist" && shasum -a 256 -c SHA256SUMS >/dev/null 2>&1); then
    echo "checksum verification unexpectedly passed after artifact tamper" >&2
    exit 1
  fi
else
  if (cd "$dist" && sha256sum -c SHA256SUMS >/dev/null 2>&1); then
    echo "checksum verification unexpectedly passed after artifact tamper" >&2
    exit 1
  fi
fi
make -C "$project_root" \
  VERSION=0.1.0 \
  COMMIT=e2e \
  BUILD_DATE=2026-01-01T00:00:00Z \
  DIST_DIR="$dist" \
  GOOS="$goos" \
  GOARCH="$goarch" \
  release >"$tmpdir/release-repair.out"
if command -v shasum >/dev/null 2>&1; then
  (cd "$dist" && shasum -a 256 -c SHA256SUMS >/dev/null)
else
  (cd "$dist" && sha256sum -c SHA256SUMS >/dev/null)
fi

alt_goarch="amd64"
if [ "$goarch" = "amd64" ]; then
  alt_goarch="arm64"
fi
alt_name="kkachi-agent-helper_0.1.0_${goos}_${alt_goarch}"
make -C "$project_root" \
  VERSION=0.1.0 \
  COMMIT=e2e \
  BUILD_DATE=2026-01-01T00:00:00Z \
  DIST_DIR="$dist" \
  GOOS="$goos" \
  GOARCH="$alt_goarch" \
  release >"$tmpdir/release-alt.out"
grep -F "$name" "$checksums" >/dev/null
grep -F "$alt_name" "$checksums" >/dev/null
if command -v shasum >/dev/null 2>&1; then
  (cd "$dist" && shasum -a 256 -c SHA256SUMS >/dev/null)
else
  (cd "$dist" && sha256sum -c SHA256SUMS >/dev/null)
fi

extract="$tmpdir/extract"
mkdir -p "$extract"
tar -xzf "$archive" -C "$extract"
[ -x "$extract/bin/kkachi-agent-helper" ]
[ -s "$extract/README.md" ]
[ -s "$extract/docs/specs.md" ]
[ -s "$extract/docs/roadmap.md" ]
[ -s "$extract/docs/compatibility.md" ]
[ -s "$extract/docs/release-notes-template.md" ]
[ -s "$extract/RELEASE-MANIFEST.json" ]

python3 - "$extract/RELEASE-MANIFEST.json" "$goos" "$goarch" <<'PY'
import json
import sys
from pathlib import Path

manifest = json.loads(Path(sys.argv[1]).read_text())
expected_docs = [
    "README.md",
    "docs/roadmap.md",
    "docs/specs.md",
    "docs/compatibility.md",
    "docs/release-notes-template.md",
]
assert manifest["manifest_version"] == "1", manifest
assert manifest["name"] == "kkachi-agent-helper", manifest
assert manifest["version"] == "0.1.0", manifest
assert manifest["commit"] == "e2e", manifest
assert manifest["build_date"] == "2026-01-01T00:00:00Z", manifest
assert manifest["goos"] == sys.argv[2], manifest
assert manifest["goarch"] == sys.argv[3], manifest
assert manifest["binary"] == "bin/kkachi-agent-helper", manifest
assert manifest["docs"] == expected_docs, manifest
PY

version_json="$($artifact version --json)"
printf '%s' "$version_json" | grep -F '"version":"0.1.0"' >/dev/null
printf '%s' "$version_json" | grep -F '"commit":"e2e"' >/dev/null
printf '%s' "$version_json" | grep -F '"build_date":"2026-01-01T00:00:00Z"' >/dev/null

make -C "$project_root" \
  VERSION=0.1.0 \
  COMMIT=e2e \
  BUILD_DATE=2026-01-01T00:00:00Z \
  PREFIX="$prefix" \
  install-local >"$tmpdir/install-local.out"

installed="$prefix/bin/kkachi-agent-helper"
[ -x "$installed" ]
installed_json="$($installed version --json)"
printf '%s' "$installed_json" | grep -F '"version":"0.1.0"' >/dev/null
printf '%s' "$installed_json" | grep -F '"commit":"e2e"' >/dev/null
printf '%s' "$installed_json" | grep -F '"build_date":"2026-01-01T00:00:00Z"' >/dev/null
