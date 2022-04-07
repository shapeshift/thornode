#!/bin/bash

# set -ex
echo "Linting handlers file"

handlers=$(ls -1 x/thorchain/handler_*)

for f in $handlers; do
  if [[ $f == *"_test"* ]]; then
    continue
  fi
  if [[ $f == *"_archive"* ]]; then
    continue
  fi
  base=$(echo "$f" | awk -F "." '{ print $1 }')
  archive="$base"_archive.go
  files="$f"
  if [[ -f $archive ]]; then
    files+=" $archive"
  fi

  # trunk-ignore(shellcheck/SC2086): word split is desired behavior
  validate_init=$(grep " validateV" $files | awk '{ print $4 }' | awk -F "(" '{ print $1 }')
  validate_call=$(grep "h.validateV" "$f" | awk '{ print $2 }' | awk -F "(" '{ print $1 }' | awk -F "." '{ print $2 }')

  # trunk-ignore(shellcheck/SC2086): word split is desired behavior
  handler_init=$(grep " handleV" $files | awk '{ print $4 }' | awk -F "(" '{ print $1 }')
  handler_call=$(grep "h.handleV" "$f" | awk '{ print $2 }' | awk -F "(" '{ print $1 }' | awk -F "." '{ print $2 }')

  missing=$(echo -e "$validate_init\n$validate_call\n$handler_init\n$handler_call" | sort -n | uniq -u)

  if [[ -n $missing ]]; then
    echo "Handler: $f... Not OK"
    echo "$missing"
    exit 1
  fi
  echo "Handler: $f... OK"
done
