#!/bin/sh

set -o pipefail

export SIGNER_NAME="${SIGNER_NAME:=thorchain}"
export SIGNER_PASSWD="${SIGNER_PASSWD:=password}"

. "$(dirname "$0")/core.sh"

# auto populate seeds for fullnode if unprovided
if [ -z "$SEEDS" ]; then
  if [ "$NET" = "mainnet" ]; then
    SEEDS=$(curl -s https://seed.thorchain.info/ | jq -r '. | join(",")')
  elif [ "$NET" = "testnet" ]; then
    SEEDS=$(curl -s https://testnet.seed.thorchain.info/ | jq -r '. | join(",")')
  elif [ "$NET" = "stagenet" ]; then
    SEEDS="stagenet-seed.ninerealms.com"
  fi
fi

if [ ! -f ~/.thornode/config/genesis.json ]; then
  init_chain

  fetch_genesis_from_seeds "$SEEDS"
fi

# render tendermint and cosmos configuration files
thornode render-config

exec thornode start
