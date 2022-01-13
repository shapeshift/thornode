#!/bin/bash

# set -ex
echo "Linting handlers file"

handlers=$(ls -1 x/thorchain/handler_*)

for f in $handlers; do
  if [[ "$f" == *"_test"* ]]; then
    continue
  fi
  if [[ "$f" == *"_archive"* ]]; then
    continue
  fi
  base=$(echo "$f" | awk -F "." '{ print $1 }')
  archive="$base"_archive.go
  validate_init=$(grep " validateV" "$f" "$archive" | awk '{ print $4 }' | awk -F "(" '{ print $1 }')
  validate_call=$(grep "h.validateV" "$f" | awk '{ print $2 }' | awk -F "(" '{ print $1 }' | awk -F "." '{ print $2 }')

  handler_init=$(grep " handleV" "$f" "$archive" | awk '{ print $4 }' | awk -F "(" '{ print $1 }')
  handler_call=$(grep "h.handleV" "$f" | awk '{ print $2 }' | awk -F "(" '{ print $1 }' | awk -F "." '{ print $2 }')

  missing=$(echo -e "$validate_init\n$validate_call\n$handler_init\n$handler_call" | sort -n | uniq -u)

  if [[ -n "$missing" ]]; then
    echo "Handler: $f... Not OK"
    echo "$missing"
    exit 1
  fi
  echo "Handler: $f... OK"
done
