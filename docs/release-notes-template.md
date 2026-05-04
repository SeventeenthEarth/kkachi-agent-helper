# kkachi-agent-helper <version> release notes

Release date: <YYYY-MM-DD>
Commit: <git-sha>

## Summary

- <one-line user-facing release outcome>

## Compatibility

| Component | Version/range | Verification |
|---|---|---|
| kkachi-agent-helper | `<version>` | `kkachi-agent-helper version --json` |
| Project schema | `0.1` | `kkachi-agent-helper schema validate .kkachi/status.json --schema status --json` |
| kkachi-agent-bridge | <range or manual note> | Manual; helper reports `not_checked` |
| kkachi-hermes-skills | <range or manual note> | Manual; helper reports `not_checked` |

## Artifacts

- `dist/kkachi-agent-helper_<version>_<goos>_<goarch>`
- `dist/kkachi-agent-helper_<version>_<goos>_<goarch>.tar.gz`
- `dist/SHA256SUMS`
- `RELEASE-MANIFEST.json` inside the tarball, with `manifest_version: "1"`

## Checksums

Paste the relevant `dist/SHA256SUMS` entries here.

```text
<sha256>  <artifact>
```

## Local install

```sh
make VERSION=<version> build
make PREFIX="$HOME/.local" install-local
kkachi-agent-helper version --json
```

## Verification

```sh
make test-prepare
make test-unit
make test-int
make test-e2e
make VERSION=<version> release
make VERSION=<version> PREFIX="$HOME/.local" install-local
kkachi-agent-helper version --json
```

## Known gaps

- `compat.required_bridge` and `compat.required_skills` are not enforced by the helper yet.
- <other release-specific gaps>
