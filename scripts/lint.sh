#!/usr/bin/env bash
set -euo pipefail

die() {
  echo "ERR: $*"
  exit 1
}

./scripts/check-build-env.sh

# Check that no .pb.go files were added.
if git ls-files '*.go' | grep -q '.pb.go$'; then
  die "Do not add generated protobuf .pb.go files"
fi

git ls-files '*.go' | grep -v -e '^docs/' | xargs gofumpt -d
if [ -n "$(git ls-files '*.go' | grep -v -e '^docs/' | xargs gofumpt -l)" ]; then
  die "Go formatting errors"
fi
go mod verify

./scripts/lint-handlers.bash

./scripts/lint-managers.bash

./scripts/lint-erc20s.bash
