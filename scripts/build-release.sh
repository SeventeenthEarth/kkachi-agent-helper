#!/bin/sh
set -eu

project_root="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
version="${VERSION:-0.1.0}"
commit="${COMMIT:-$(git -C "$project_root" rev-parse --short HEAD 2>/dev/null || echo unknown)}"
build_date="${BUILD_DATE:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"
goos="${GOOS:-$(go env GOOS)}"
goarch="${GOARCH:-$(go env GOARCH)}"
dist_dir="${DIST_DIR:-$project_root/dist}"

require_match() {
  field="$1"
  value="$2"
  pattern="$3"
  hint="$4"
  if ! printf '%s' "$value" | grep -Eq "$pattern"; then
    printf 'error: %s %s: %s\n' "$field" "$hint" "$value" >&2
    exit 2
  fi
}

# Values are embedded in ldflags, artifact names, and RELEASE-MANIFEST.json.
# Keep them intentionally narrow so shell quoting and JSON escaping do not become
# part of the release contract.
require_match VERSION "$version" '^[0-9]+[.][0-9]+[.][0-9]+([-+][0-9A-Za-z.-]+)?$' 'must be semver-like x.y.z with optional prerelease/build suffix'
require_match COMMIT "$commit" '^[0-9A-Za-z._-]+$' 'must contain only letters, numbers, dot, underscore, or hyphen'
require_match BUILD_DATE "$build_date" '^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$' 'must be UTC RFC3339 seconds like 2026-01-01T00:00:00Z'
require_match GOOS "$goos" '^[0-9A-Za-z._-]+$' 'must contain only letters, numbers, dot, underscore, or hyphen'
require_match GOARCH "$goarch" '^[0-9A-Za-z._-]+$' 'must contain only letters, numbers, dot, underscore, or hyphen'

name="kkachi-agent-helper_${version}_${goos}_${goarch}"
stage="${dist_dir}/${name}.package"

rm -rf "$stage"
mkdir -p "$dist_dir" "$stage/bin" "$stage/docs"

(
  cd "$project_root"
  GOOS="$goos" GOARCH="$goarch" go build \
    -ldflags "-X main.version=$version -X main.commit=$commit -X main.buildDate=$build_date" \
    -o "$stage/bin/kkachi-agent-helper" \
    ./cmd/kkachi-agent-helper
)

cp "$stage/bin/kkachi-agent-helper" "$dist_dir/$name"
cp "$project_root/README.md" "$stage/README.md"
cp "$project_root/docs/roadmap.md" "$stage/docs/roadmap.md"
cp "$project_root/docs/specs.md" "$stage/docs/specs.md"
cp "$project_root/docs/compatibility.md" "$stage/docs/compatibility.md"
cp "$project_root/docs/release-notes-template.md" "$stage/docs/release-notes-template.md"
cat > "$stage/RELEASE-MANIFEST.json" <<MANIFEST
{
  "manifest_version": "1",
  "name": "kkachi-agent-helper",
  "version": "$version",
  "commit": "$commit",
  "build_date": "$build_date",
  "goos": "$goos",
  "goarch": "$goarch",
  "binary": "bin/kkachi-agent-helper",
  "docs": [
    "README.md",
    "docs/roadmap.md",
    "docs/specs.md",
    "docs/compatibility.md",
    "docs/release-notes-template.md"
  ]
}
MANIFEST

(
  cd "$dist_dir"
  tar -czf "$name.tar.gz" -C "$name.package" .
  checksum_inputs=".SHA256SUMS.inputs.$$"
  find . -maxdepth 1 -type f -name "kkachi-agent-helper_${version}_*" ! -name SHA256SUMS -print | sed 's#^./##' | sort > "$checksum_inputs"
  if [ ! -s "$checksum_inputs" ]; then
    printf 'error: no release artifacts found for version %s\n' "$version" >&2
    exit 1
  fi
  if command -v shasum >/dev/null 2>&1; then
    while IFS= read -r artifact; do
      shasum -a 256 "$artifact"
    done < "$checksum_inputs" > SHA256SUMS
  else
    while IFS= read -r artifact; do
      sha256sum "$artifact"
    done < "$checksum_inputs" > SHA256SUMS
  fi
  rm -f "$checksum_inputs"
)

rm -rf "$stage"
printf 'release artifacts written to %s\n' "$dist_dir"
printf '%s\n' "$dist_dir/$name"
printf '%s\n' "$dist_dir/$name.tar.gz"
printf '%s\n' "$dist_dir/SHA256SUMS"
