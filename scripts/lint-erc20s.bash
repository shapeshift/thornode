#!/bin/bash
set -euo pipefail

git show origin/develop:bifrost/pkg/chainclients/ethereum/token_list.json |
  jq -r '.tokens[] | .address | ascii_downcase' | sort -n | uniq -u >/tmp/orig_erc20_token_list.txt

jq -r '.tokens[] | .address | ascii_downcase' <bifrost/pkg/chainclients/ethereum/token_list.json |
  uniq -u >/tmp/modified_erc20_token_list.txt

cat /tmp/orig_erc20_token_list.txt /tmp/modified_erc20_token_list.txt |
  sort -n | uniq -d >/tmp/union_erc20_token_list.txt

diff /tmp/orig_erc20_token_list.txt /tmp/union_erc20_token_list.txt || exit 1

echo "OK"
