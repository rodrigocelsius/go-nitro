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
bad=$(grep -rhE 'go-nitro/.*/v2 v[0-9]' --include=go.mod "${DIRS[@]/#/./}" 2>/dev/null | grep -v " $VERSION" || true)
[[ -z "$bad" ]] || { echo "internal requires not at $VERSION:"; echo "$bad"; exit 1; }

for d in "${DIRS[@]}"; do
  if [[ "$d" == "." ]]; then tag="$VERSION"; else tag="$d/$VERSION"; fi
  if [[ "$PUSH" == "--push" ]]; then
    git tag -a "$tag" -m "$tag"
  else
    echo "would tag: $tag"
  fi
done
[[ "$PUSH" == "--push" ]] && git push origin --tags && echo "pushed ${#DIRS[@]} tags"
exit 0
