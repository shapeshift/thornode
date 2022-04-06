#!/bin/sh

set -o pipefail

# fullnodes default to mainnet

export THORNODE_API_ENABLE="true"
export SIGNER_NAME="${SIGNER_NAME:=thorchain}"
export SIGNER_PASSWD="${SIGNER_PASSWD:=password}"

. "$(dirname "$0")/core.sh"

CHAIN_ID=${CHAIN_ID:=thorchain}

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

	# enable telemetry through prometheus metrics endpoint
	enable_telemetry

	# enable internal traffic as well
	enable_internal_traffic

	# use external IP if available
	[ -n "$EXTERNAL_IP" ] && external_address "$EXTERNAL_IP" "$NET"
fi

seeds_list "$SEEDS" "$CHAIN_ID"

if [ -z "$*" ]; then
	exec thornode start
else
	exec "$@"
fi
