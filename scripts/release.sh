#!/usr/bin/env bash
# Tag every publishable module of this multi-module repo for a release.
#
# Go resolves a nested module from tags prefixed by its directory, so v2.0.0 of
# github.com/rodrigocelsius/go-nitro/pkg/v2 is the git tag "pkg/v2.0.0"
# (the root module is plain "v2.0.0"). All internal requires already point to
# the same version, so tag everything for the same release.
#
#   scripts/release.sh v2.0.0          # dry run: prints the tags
#   scripts/release.sh v2.0.0 --push   # creates and pushes them
set -euo pipefail

VERSION="${1:?usage: release.sh vX.Y.Z [--push]}"
PUSH="${2:-}"
[[ "$VERSION" =~ ^v2\.[0-9]+\.[0-9]+(-[0-9A-Za-z.]+)?$ ]] || { echo "version must be v2.x.y (module paths end in /v2)"; exit 1; }

cd "$(git rev-parse --show-toplevel)"
[[ -z "$(git status --porcelain)" ]] || { echo "working tree not clean"; exit 1; }

# Publishable modules: everything except examples, testdata and codegen fixtures.
mapfile -t DIRS < <(find . -name go.mod \
  -not -path './examples/*' -not -path '*/testdata/*' \
  -not -path '*/routetestdata*' -not -path '*/proto_nested/*' \
  -printf '%h\n' | sed 's|^\./||' | sort)

# Every internal require must already be at $VERSION.
# Only each module's own go.mod (not recursive: test fixtures pin placeholders).
bad=$(for d in "${DIRS[@]}"; do grep -hE 'go-nitro/.*v2 v[0-9]' "$d/go.mod"; done 2>/dev/null | grep -vE " $VERSION( |$)" || true)
[[ -z "$bad" ]] || { echo "internal requires not at $VERSION:"; echo "$bad"; exit 1; }

# Binaries installed with `go install …@version` (no replace allowed in their
# go.mod) are tagged in a second phase, once the libraries they require exist.
CLIS=(cmd/nitrogen)
is_cli() { local x; for x in "${CLIS[@]}"; do [[ "$1" == "$x" ]] && return 0; done; return 1; }

tag_dir() {
  local d="$1" tag
  if [[ "$d" == "." ]]; then tag="$VERSION"; else tag="$d/$VERSION"; fi
  if [[ "$PUSH" == "--push" ]]; then
    git tag -a "$tag" -m "$tag"
  else
    echo "would tag: $tag"
  fi
}

# Phase 1: libraries.
for d in "${DIRS[@]}"; do
  is_cli "$d" || tag_dir "$d"
done
[[ "$PUSH" == "--push" ]] && git push origin --tags

# Phase 2: CLIs — resolve go.sum against the just-published libraries.
for d in "${CLIS[@]}"; do
  [[ -f "$d/go.mod" ]] || continue
  if [[ "$PUSH" == "--push" ]]; then
    (cd "$d" && GOWORK=off GOPROXY=direct GONOSUMDB=github.com/rodrigocelsius/go-nitro GOFLAGS=-mod=mod go mod tidy)
    if [[ -n "$(git status --porcelain "$d")" ]]; then
      git add "$d/go.mod" "$d/go.sum"
      git commit -m "chore(release): pin $d dependencies to $VERSION"
      git push origin HEAD
    fi
  fi
  tag_dir "$d"
done
[[ "$PUSH" == "--push" ]] && git push origin --tags && echo "pushed ${#DIRS[@]} tags"
exit 0
