# BITS-GeoIP

Automated **GeoIP database and sing-box rule-set builder**. Source data comes
from the latest [`Country.mmdb`](https://github.com/Dreamacro/maxmind-geoip)
release (MaxMind GeoLite2 via Dreamacro), plus provider network rule sets from
[`Loyalsoldier/geoip`](https://github.com/Loyalsoldier/geoip).

Generated data targets [sing-box](https://github.com/SagerNet/sing-box) and is
published by [bitscoid/BITS-GeoIP](https://github.com/bitscoid/BITS-GeoIP)
through GitHub Releases plus the `release` and `rule-set` branches.

<p>
  <img src="https://img.shields.io/badge/Platform-Cross--platform-00ADD8?logo=go&logoColor=white" alt="Go" />
  <img src="https://img.shields.io/badge/Generated%20for-sing--box-7C3AED?logo=go&logoColor=white" alt="sing-box" />
  <img src="https://img.shields.io/badge/License-MIT-blue.svg" alt="MIT License" />
</p>

---

## Table of Contents

- [What is generated](#what-is-generated)
- [Variants](#variants)
- [Provider rule sets](#provider-rule-sets)
- [Usage in sing-box](#usage-in-sing-box)
- [Requirements](#requirements)
- [Local usage](#local-usage)
- [Environment variables](#environment-variables)
- [Development](#development)
- [Processing flow](#processing-flow)
- [GitHub Actions](#github-actions)
- [Versioning](#versioning)
- [Related projects](#related-projects)
- [License](#license)

---

## What is generated

| File | Description |
| --- | --- |
| `geoip.db` | **Full** GeoIP database (all countries) in MMDB format. |
| `geoip-min.db` | **Minimal** GeoIP database: `id` only. |
| `geoip.db.sha256sum` | SHA-256 checksum of the full database. |
| `geoip-min.db.sha256sum` | SHA-256 checksum of the minimal database. |
| `rule-set/geoip-<cc>.srs` | sing-box binary rule set per country code. |
| `rule-set/provider-<name>.srs` | Provider rule sets (cloudflare, google, ...). |

`<cc>` uses lowercase ISO 3166-1 alpha-2 country codes, e.g. `id`, `us`, `sg`.

## Variants

| Variant | Contents | Size (approx.) | Typical use |
| --- | --- | --- | --- |
| **Minimal** (`geoip-min.db`) | `id` only | ~246 KB | Bundled in the [BITS Box](https://github.com/Banten-IT-Solutions/BITS-Box) APK; covers the default Indonesian IP bypass rule. |
| **Full** (`geoip.db`) | Every country | ~3.9 MB | When rules reference IPs from other countries. |

## Provider rule sets

Provider networks span multiple countries, so they are kept separate from
country data. They are downloaded from the
[`Loyalsoldier/geoip` SRS release branch](https://github.com/Loyalsoldier/geoip/tree/release/srs)
and currently include:

```text
cloudflare  cloudfront  facebook  fastly  google
netflix     telegram    tor       twitter
```

## Usage in sing-box

Reference country IPs in rules with the `geoip:` prefix:

```json
{
  "rules": [
    { "ip_cidr": ["geoip:id"], "outbound": "direct" },
    { "ip_cidr": ["geoip:cn"], "outbound": "proxy" }
  ]
}
```

Or use the binary rule sets:

```json
{ "rule_set": ["geoip-id"], "outbound": "direct" }
```

with `rule_set` defined pointing to a local `rule-set/geoip-id.srs` file or a
remote URL. Provider rule sets work the same way (`provider-cloudflare.srs`,
etc.).

> **Note:** rules referencing a country code that is missing from the loaded
> database are skipped silently. With the minimal database only `id` is
> available.

## Requirements

- Go version declared in [`go.mod`](go.mod).
- Network access to GitHub Releases.
- GitHub token only when higher API rate limits are needed.

## Local usage

```sh
go run .                        # build latest upstream data
make build                      # compile to ./bits-geoip
./bits-geoip
NO_SKIP=true go run .           # force regeneration (skip already-latest check)
FIXED_RELEASE=<release-tag> go run .   # pin a specific upstream release
```

The generator queries the upstream repo, downloads `Country.mmdb`, generates
the databases and rule sets, then **publishes** them to the destination repo
(GitHub Releases + branches).

## Environment variables

| Variable | Description |
| --- | --- |
| `ACCESS_TOKEN` | GitHub token for authenticated API requests and higher rate limits. |
| `FIXED_RELEASE` | Upstream release tag to build instead of the latest release. |
| `NO_SKIP` | Set to `true` to disable the destination-release skip check. |

Generated databases and rule sets are ignored by Git through [`.gitignore`](.gitignore).

## Development

```sh
make fmt           # Format Go source (gofumpt + gofmt + gci)
make fmt_install   # Install formatting tools
make lint          # Run golangci-lint
make test          # Run Go package tests
make build         # Build ./bits-geoip
make clean         # Remove generated artifacts
```

## Processing flow

1. Query the upstream or pinned GitHub release.
2. Locate and download `Country.mmdb` with timeout and retry handling.
3. Parse IP networks and their registered country codes.
4. Generate `geoip.db` for all countries.
5. Generate `geoip-min.db` for the `id` country only.
6. Generate one `.srs` rule set per country.
7. Download and validate provider `.srs` rule sets.
8. Expose the source release tag to GitHub Actions through `GITHUB_OUTPUT`.

## GitHub Actions

### `build.yaml`

Runs on pushes to `main`. Checks out, installs Go, lints, builds the data with
`NO_SKIP=true`, and uploads databases plus the complete `rule-set/` directory
as a workflow artifact.

### `release.yaml`

Runs **monthly** (cron `0 8 12 * *` — 12th of each month) or through manual
dispatch. It:

1. Lints and builds the generated data.
2. Generates SHA-256 checksum files.
3. Publishes rule sets to branch `rule-set`.
4. Publishes databases and checksums to branch `release`.
5. Keeps the **three latest** GitHub Releases.
6. Publishes databases, checksums, and individual `.srs` rule set files (country + provider) as release assets.

Manual releases can provide an upstream tag through the workflow `tag` input,
or force a rebuild with the `force` input (passes `NO_SKIP=true`).

Published branches:

- [`release`](https://github.com/bitscoid/BITS-GeoIP/tree/release) — databases + checksums.
- [`rule-set`](https://github.com/bitscoid/BITS-GeoIP/tree/rule-set) — sing-box rule sets.

## Versioning

Release tags mirror the upstream `Dreamacro/maxmind-geoip` release tag
(e.g. `20260812`), so a fresh release is only created when upstream publishes
new data. Old releases are pruned to the three most recent.

## Dependency updates

Dependabot configuration is stored in [`.github/dependabot.yml`](.github/dependabot.yml).
It checks Go modules and GitHub Actions weekly.

## Related projects

- [BITS-GeoSite](https://github.com/bitscoid/BITS-GeoSite) — GeoSite database builder.
- [BITS-Box](https://github.com/Banten-IT-Solutions/BITS-Box) — Android client that consumes these assets.
- [sing-box](https://github.com/SagerNet/sing-box) — the target kernel.
- [Dreamacro/maxmind-geoip](https://github.com/Dreamacro/maxmind-geoip) — upstream GeoIP data.
- [Loyalsoldier/geoip](https://github.com/Loyalsoldier/geoip) — provider rule sets.

## License

This project is licensed under the [MIT License](LICENSE).
