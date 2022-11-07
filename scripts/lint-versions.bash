#!/bin/bash

set -euo pipefail

# This script compares versioned functions to the develop branch to ensure no logic
# changes in historical versions that would cause consensus failure.

VERSION=$(awk -F. '{ print $2 }' version)
CI_MERGE_REQUEST_TITLE=${CI_MERGE_REQUEST_TITLE:-}

go run tools/versioned-functions/main.go --version="$VERSION" >/tmp/versioned-fns-current
git checkout origin/develop
git checkout - -- tools
go run tools/versioned-functions/main.go --version="$VERSION" >/tmp/versioned-fns-develop
git checkout -

if ! diff -u -F '^func' -I '^//' --color=always /tmp/versioned-fns-develop /tmp/versioned-fns-current; then
  echo "Detected change in versioned function."
  if [[ $CI_MERGE_REQUEST_TITLE == *"#unsafe"* ]]; then
    echo "Merge request is marked unsafe."
  else
    echo 'Correct the change, add a new versioned function, or add "#unsafe" to the PR description.'
    exit 1
  fi
fi