# sing-geoip

Builds GeoIP databases and sing-box rule sets from the latest `Country.mmdb` release published by [`Dreamacro/maxmind-geoip`](https://github.com/Dreamacro/maxmind-geoip).

Generated data targets [`sing-box`](https://github.com/SagerNet/sing-box) and is published through the `SagerNet/sing-geoip` `release` and `rule-set` branches, plus GitHub Releases.

## Generated Files

| File | Description |
| --- | --- |
| `geoip.db` | Complete country GeoIP database in MMDB format. |
| `geoip-id.db` | Indonesia-only GeoIP database. |
| `rule-set/geoip-<CC>.srs` | sing-box binary rule set for country code `<CC>`. |

`<CC>` uses lowercase ISO 3166-1 alpha-2 country codes, for example `id`, `us`, and `sg`.

## Requirements

- Go version declared in [`go.mod`](go.mod)
- Network access to GitHub Releases
- GitHub token only when higher API rate limits are needed

## Local Usage

Build generated data from the latest upstream release:

```bash
go run .
```

Build the executable instead:

```bash
make build
./sing-geoip
```

Force a build even when the destination already contains the upstream release:

```bash
NO_SKIP=true go run .
```

Build from a specific upstream release tag:

```bash
FIXED_RELEASE=<release-tag> go run .
```

## Environment Variables

| Variable | Description |
| --- | --- |
| `ACCESS_TOKEN` | GitHub token used for authenticated API requests and higher rate limits. |
| `FIXED_RELEASE` | Upstream release tag to build instead of the latest release. |
| `NO_SKIP` | Set to `true` to disable the destination-release skip check. |

Generated databases and rule sets are ignored by Git through [`.gitignore`](.gitignore).

## Development Commands

```bash
make fmt          # Format Go source files.
make lint         # Run golangci-lint.
make test         # Run Go package tests.
make build        # Build the sing-geoip executable.
make clean        # Remove generated databases, rule sets, and build artifacts.
```

Install formatting tools when needed:

```bash
make fmt_install
```

## Processing Flow

1. Query the upstream or pinned GitHub release.
2. Locate and download `Country.mmdb` with timeout and retry handling.
3. Parse IP networks and their registered country codes.
4. Generate `geoip.db` for all countries.
5. Generate `geoip-id.db` for Indonesia.
6. Generate one `.srs` rule set per country.
7. Expose the source release tag to GitHub Actions through `GITHUB_OUTPUT`.

## GitHub Actions

### Build workflow

`.github/workflows/build.yaml` runs on pushes to `main` and:

1. Checks out the repository.
2. Installs the Go version from `go.mod`.
3. Runs golangci-lint.
4. Builds `geoip.db` with `NO_SKIP=true`.
5. Uploads `geoip.db` as a workflow artifact.

### Release workflow

`.github/workflows/release.yaml` runs monthly or through manual dispatch. It:

1. Uses the Go version from `go.mod`.
2. Lints and builds the generated data.
3. Generates SHA-256 checksum files.
4. Publishes rule sets to branch `rule-set`.
5. Publishes databases and checksums to branch `release`.
6. Keeps the three latest GitHub Releases.
7. Publishes `geoip.db`, `geoip-id.db`, and their checksums as GitHub Release assets.

Manual releases can provide an upstream tag through the workflow `tag` input.

## Dependency Updates

Dependabot configuration is stored in [`.github/dependabot.yml`](.github/dependabot.yml). It checks Go modules and GitHub Actions weekly.

## License

This project is licensed under the [MIT License](LICENSE).
