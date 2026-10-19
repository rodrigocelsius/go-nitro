# Releasing

Nitro is a multi-module repository. Each module has its own `go.mod` whose
path ends in `/v2` (Go requires the major-version suffix for `v2.x` tags).

| Module | Path | Git tag |
|---|---|---|
| root | `github.com/rodrigocelsius/go-nitro/v2` | `v2.0.0` |
| facade | `github.com/rodrigocelsius/go-nitro/pkg/v2` | `pkg/v2.0.0` |
| any other | `github.com/rodrigocelsius/go-nitro/<dir>/v2` | `<dir>/v2.0.0` |

## Steps

1. Update `CHANGELOG.md` and make sure CI is green.
2. Internal `require` lines must already be at the release version (they are
   `v2.0.0` today; bump them with `find . -name go.mod | xargs sed -i ...`).
3. `scripts/release.sh v2.0.0` (dry run) → `scripts/release.sh v2.0.0 --push`.
4. Verify from a clean directory:
   ```bash
   mkdir /tmp/t && cd /tmp/t && go mod init t
   go get github.com/rodrigocelsius/go-nitro/pkg/v2@v2.0.0
   go install github.com/rodrigocelsius/go-nitro/cmd/nitrogen/v2@v2.0.0
   ```

`replace` directives in each `go.mod` only serve local development; consumers
ignore them and resolve everything from tags.

**CLI (`cmd/nitrogen`)** is the exception: `go install …@version` rejects a
`go.mod` with `replace`, so the CLI uses `cmd/nitrogen/go.work` for local
development instead. `release.sh` tags libraries first, then runs
`go mod tidy` for the CLI against the published tags (committing go.sum when it
changes) and tags `cmd/nitrogen/vX.Y.Z` last.
