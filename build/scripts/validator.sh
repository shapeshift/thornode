#!/bin/sh

set -o pipefail

. "$(dirname "$0")/core.sh"

SEEDS="${SEEDS:=none}"        # the hostname of multiple seeds set as tendermint seeds
PEER="${PEER:=none}"          # the hostname of a seed node set as tendermint persistent peer
PEER_API="${PEER_API:=$PEER}" # the hostname of a seed node API if different
SIGNER_NAME="${SIGNER_NAME:=thorchain}"
SIGNER_PASSWD="${SIGNER_PASSWD:=password}"
BINANCE=${BINANCE:=$PEER:26660}
CHAIN_ID=${CHAIN_ID:=thorchain}

if [ ! -f ~/.thornode/config/genesis.json ]; then
	echo "Setting THORNode as Validator node"
	if [ "$PEER" = "none" ] && [ "$SEEDS" = "none" ]; then
		echo "Missing PEER / SEEDS"
		exit 1
	fi

	create_thor_user "$SIGNER_NAME" "$SIGNER_PASSWD" "$SIGNER_SEED_PHRASE"

	NODE_ADDRESS=$(echo "$SIGNER_PASSWD" | thornode keys show "$SIGNER_NAME" -a --keyring-backend file)
	init_chain "$NODE_ADDRESS"

	if [ "$PEER" != "none" ]; then
		fetch_genesis $PEER
	fi

	if [ "$SEEDS" != "none" ]; then
		fetch_genesis_from_seeds $SEEDS

		# add seeds tendermint config
		seeds_list $SEEDS $CHAIN_ID
	fi

	# enable telemetry through prometheus metrics endpoint
	enable_telemetry

	# set the minimum gas to 0 rune
	set_minimum_gas

	# enable internal traffic as well
	enable_internal_traffic

	# use external IP if available
	[ -n "$EXTERNAL_IP" ] && external_address "$EXTERNAL_IP" "$NET"

	if [ "$NET" = "mocknet" ]; then
		# create a binance wallet and bond/register
		gen_bnb_address
		ADDRESS=$(cat ~/.bond/address.txt)

		# switch the BNB bond to native RUNE
		"$(dirname "$0")/mock/switch.sh" $BINANCE "$ADDRESS" "$NODE_ADDRESS" $PEER

		sleep 30 # wait for thorchain to register the new node account

		printf "%s\n" "$SIGNER_PASSWD" | thornode tx thorchain deposit 100000000000000 RUNE "bond:$NODE_ADDRESS" --node tcp://$PEER:26657 --from "$SIGNER_NAME" --keyring-backend=file --chain-id "$CHAIN_ID" --yes

		# send bond

		sleep 10 # wait for thorchain to commit a block , otherwise it get the wrong sequence number

		NODE_PUB_KEY=$(echo "$SIGNER_PASSWD" | thornode keys show thorchain --pubkey --keyring-backend=file | thornode pubkey)
		VALIDATOR=$(thornode tendermint show-validator | thornode pubkey --bech cons)

		# set node keys
		until printf "%s\n" "$SIGNER_PASSWD" | thornode tx thorchain set-node-keys "$NODE_PUB_KEY" "$NODE_PUB_KEY_ED25519" "$VALIDATOR" --node tcp://$PEER:26657 --from "$SIGNER_NAME" --keyring-backend=file --chain-id "$CHAIN_ID" --yes; do
			sleep 5
		done

		# add IP address
		sleep 10 # wait for thorchain to commit a block

		NODE_IP_ADDRESS=${EXTERNAL_IP:=$(curl -s http://whatismyip.akamai.com)}
		until printf "%s\n" "$SIGNER_PASSWD" | thornode tx thorchain set-ip-address "$NODE_IP_ADDRESS" --node tcp://$PEER:26657 --from "$SIGNER_NAME" --keyring-backend=file --chain-id "$CHAIN_ID" --yes; do
			sleep 5
		done

		# use external IP if available
		[ -n "$EXTERNAL_IP" ] && external_address "$EXTERNAL_IP" "$NET"

		sleep 10 # wait for thorchain to commit a block
		# set node version
		until printf "%s\n" "$SIGNER_PASSWD" | thornode tx thorchain set-version --node tcp://$PEER:26657 --from "$SIGNER_NAME" --keyring-backend=file --chain-id "$CHAIN_ID" --yes; do
			sleep 5
		done

	elif [ "$NET" = "testnet" ]; then
		# create a binance wallet
		gen_bnb_address
		ADDRESS=$(cat ~/.bond/address.txt)
	else
		echo "Your THORNode address: $NODE_ADDRESS"
		echo "Send your bond to that address"
	fi

else

	if [ "$SEEDS" != "none" ]; then
		# add seeds tendermint config
		seeds_list $SEEDS $CHAIN_ID
	fi
fi

export SIGNER_NAME
export SIGNER_PASSWD
exec "$@"
