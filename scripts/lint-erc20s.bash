#!/bin/bash

curl -s https://gitlab.com/thorchain/thornode/-/raw/develop/bifrost/pkg/chainclients/ethereum/token_list.json | jq -r '.tokens[] | .address | ascii_downcase' | sort -n | uniq -u >/tmp/orig_erc20_token_list.txt

jq -r '.tokens[] | .address | ascii_downcase' <bifrost/pkg/chainclients/ethereum/token_list.json | uniq -u >/tmp/modified_erc20_token_list.txt

cat /tmp/orig_erc20_token_list.txt /tmp/modified_erc20_token_list.txt | sort -n | uniq -d >/tmp/union_erc20_token_list.txt

if diff /tmp/orig_erc20_token_list.txt /tmp/union_erc20_token_list.txt; then
	echo "OK"
else
	exit 1
fi
