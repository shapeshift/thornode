#!/bin/sh

set -o pipefail

PORT_P2P=26656
PORT_RPC=26657
[ "$NET" = "mainnet" ] && PORT_P2P=27146 && PORT_RPC=27147
[ "$NET" = "stagenet" ] && PORT_P2P=27146 && PORT_RPC=27147

# adds an account node into the genesis file
add_node_account() {
  NODE_ADDRESS=$1
  VALIDATOR=$2
  NODE_PUB_KEY=$3
  VERSION=$4
  BOND_ADDRESS=$5
  NODE_PUB_KEY_ED25519=$6
  IP_ADDRESS=$7
  MEMBERSHIP=$8
  jq --arg IP_ADDRESS "$IP_ADDRESS" --arg VERSION "$VERSION" --arg BOND_ADDRESS "$BOND_ADDRESS" --arg VALIDATOR "$VALIDATOR" --arg NODE_ADDRESS "$NODE_ADDRESS" --arg NODE_PUB_KEY "$NODE_PUB_KEY" --arg NODE_PUB_KEY_ED25519 "$NODE_PUB_KEY_ED25519" '.app_state.thorchain.node_accounts += [{"node_address": $NODE_ADDRESS, "version": $VERSION, "ip_address": $IP_ADDRESS, "status": "Active","bond":"100000000", "active_block_height": "0", "bond_address":$BOND_ADDRESS, "signer_membership": [], "validator_cons_pub_key":$VALIDATOR, "pub_key_set":{"secp256k1":$NODE_PUB_KEY,"ed25519":$NODE_PUB_KEY_ED25519}}]' <~/.thornode/config/genesis.json >/tmp/genesis.json
  mv /tmp/genesis.json ~/.thornode/config/genesis.json
  if [ -n "$MEMBERSHIP" ]; then
    jq --arg MEMBERSHIP "$MEMBERSHIP" '.app_state.thorchain.node_accounts[-1].signer_membership += [$MEMBERSHIP]' ~/.thornode/config/genesis.json >/tmp/genesis.json
    mv /tmp/genesis.json ~/.thornode/config/genesis.json
  fi
}

reserve() {
  jq --arg RESERVE "$1" '.app_state.thorchain.reserve = $RESERVE' <~/.thornode/config/genesis.json >/tmp/genesis.json
  mv /tmp/genesis.json ~/.thornode/config/genesis.json
}

disable_bank_send() {
  jq '.app_state.bank.params.default_send_enabled = false' <~/.thornode/config/genesis.json >/tmp/genesis.json
  mv /tmp/genesis.json ~/.thornode/config/genesis.json

  jq '.app_state.transfer.params.send_enabled = false' <~/.thornode/config/genesis.json >/tmp/genesis.json
  mv /tmp/genesis.json ~/.thornode/config/genesis.json
}

# inits a thorchain with a comman separate list of usernames
init_chain() {
  OLD_IFS=IFS
  IFS=","

  echo "Init chain"
  thornode init local --chain-id "$CHAIN_ID"
  echo "$SIGNER_PASSWD" | thornode keys list --keyring-backend file

  for user in "$@"; do # iterate over our list of comma separated users "alice,jack"
    thornode add-genesis-account "$user" 100000000rune
  done

  IFS=OLD_IFS

  # thornode config chain-id thorchain
  # thornode config output json
  # thornode config indent true
  # thornode config trust-node true
}

peer_list() {
  PEERUSER="$1@$2:$PORT_P2P"
  PEERSISTENT_PEER_TARGET='persistent_peers = ""'
  sed -i -e "s/$PEERSISTENT_PEER_TARGET/persistent_peers = \"$PEERUSER\"/g" ~/.thornode/config/config.toml
}

block_time() {
  sed -i -e "s/timeout_commit = \"5s\"/timeout_commit = \"$1\"/g" ~/.thornode/config/config.toml
}

seeds_list() {
  EXPECTED_NETWORK=$(echo "$@" | awk '{print $NF}')
  OLD_IFS=$IFS
  IFS=","
  SEED_LIST=""
  for SEED in $1; do
    RESULT=$(curl -sL --fail -m 2 "$SEED:$PORT_RPC/status") || continue
    NODE_ID=$(echo "$RESULT" | jq -r .result.node_info.id)
    NETWORK=$(echo "$RESULT" | jq -r .result.node_info.network)
    # make sure the seeds are on the same network
    if [ "$NETWORK" = "$EXPECTED_NETWORK" ]; then
      SEED="$NODE_ID@$SEED:$PORT_P2P"
      if [ -z "$SEED_LIST" ]; then
        SEED_LIST=$SEED
      else
        SEED_LIST="$SEED_LIST,$SEED"
      fi
    fi
  done
  IFS=$OLD_IFS
  sed -i -e "s/seeds =.*/seeds = \"$SEED_LIST\"/g" ~/.thornode/config/config.toml
}

enable_internal_traffic() {
  ADDR='addr_book_strict = true'
  ADDR_STRICT_FALSE='addr_book_strict = false'
  sed -i -e "s/$ADDR/$ADDR_STRICT_FALSE/g" ~/.thornode/config/config.toml
}

external_address() {
  IP=$1
  NET=$2
  ADDR="$IP:$PORT_P2P"
  sed -i -e "s/external_address =.*/external_address = \"$ADDR\"/g" ~/.thornode/config/config.toml
}

enable_telemetry() {
  sed -i -e "s/prometheus = false/prometheus = true/g" ~/.thornode/config/config.toml
  sed -i -e "s/enabled = false/enabled = true/g" ~/.thornode/config/app.toml
  sed -i -e "s/prometheus-retention-time = 0/prometheus-retention-time = 600/g" ~/.thornode/config/app.toml
}

set_minimum_gas() {
  sed -i -e 's/minimum-gas-prices = ""/minimum-gas-prices = "0rune"/g' ~/.thornode/config/app.toml
}

set_eth_contract() {
  jq --arg CONTRACT "$1" '.app_state.thorchain.chain_contracts = [{"chain": "ETH", "router": $CONTRACT}]' ~/.thornode/config/genesis.json >/tmp/genesis.json
  mv /tmp/genesis.json ~/.thornode/config/genesis.json
}

fetch_genesis() {
  echo "Fetching genesis from $1:$PORT_RPC"
  until curl -s "$1:$PORT_RPC" &>/dev/null; do
    sleep 3
  done
  curl -s "$1:$PORT_RPC/genesis" | jq .result.genesis >~/.thornode/config/genesis.json
  thornode validate-genesis --trace
  cat ~/.thornode/config/genesis.json
}

fetch_genesis_from_seeds() {
  OLD_IFS=$IFS
  IFS=","
  SEED_LIST=""
  for SEED in $1; do
    echo "Fetching genesis from seed $SEED"
    curl -sL --fail -m 10 "$SEED:$PORT_RPC/genesis" | jq .result.genesis >~/.thornode/config/genesis.json || continue
    thornode validate-genesis
    cat ~/.thornode/config/genesis.json
    break
  done
  IFS=$OLD_IFS
}

fetch_node_id() {
  until curl -s "$1:$PORT_RPC" &>/dev/null; do
    sleep 3
  done
  curl -s "$1:$PORT_RPC/status" | jq -r .result.node_info.id
}

set_node_keys() {
  SIGNER_NAME="$1"
  SIGNER_PASSWD="$2"
  PEER="$3"
  NODE_PUB_KEY="$(echo "$SIGNER_PASSWD" | thornode keys show thorchain --pubkey --keyring-backend file | thornode pubkey)"
  NODE_PUB_KEY_ED25519="$(printf "%s\n" "$SIGNER_PASSWD" | thornode ed25519)"
  VALIDATOR="$(thornode tendermint show-validator | thornode pubkey --bech cons)"
  echo "Setting THORNode keys"
  printf "%s\n%s\n" "$SIGNER_PASSWD" "$SIGNER_PASSWD" | thornode tx thorchain set-node-keys "$NODE_PUB_KEY" "$NODE_PUB_KEY_ED25519" "$VALIDATOR" --node "tcp://$PEER:$PORT_RPC" --from "$SIGNER_NAME" --yes
}

set_ip_address() {
  SIGNER_NAME="$1"
  SIGNER_PASSWD="$2"
  PEER="$3"
  NODE_IP_ADDRESS="${4:-$(curl -s http://whatismyip.akamai.com)}"
  echo "Setting THORNode IP address $NODE_IP_ADDRESS"
  printf "%s\n%s\n" "$SIGNER_PASSWD" "$SIGNER_PASSWD" | thornode tx thorchain set-ip-address "$NODE_IP_ADDRESS" --node "tcp://$PEER:$PORT_RPC" --from "$SIGNER_NAME" --yes
}

fetch_version() {
  thornode query thorchain version --output json | jq -r .version
}

create_thor_user() {
  SIGNER_NAME="$1"
  SIGNER_PASSWD="$2"
  SIGNER_SEED_PHRASE="$3"

  echo "Checking if THORNode Thor '$SIGNER_NAME' account exists"
  echo "$SIGNER_PASSWD" | thornode keys show "$SIGNER_NAME" --keyring-backend file &>/dev/null
  # shellcheck disable=SC2181
  if [ $? -ne 0 ]; then
    echo "Creating THORNode Thor '$SIGNER_NAME' account"
    if [ -n "$SIGNER_SEED_PHRASE" ]; then
      printf "%s\n%s\n%s\n" "$SIGNER_SEED_PHRASE" "$SIGNER_PASSWD" "$SIGNER_PASSWD" | thornode keys --keyring-backend file add "$SIGNER_NAME" --recover
    else
      sig_pw=$(printf "%s\n%s\n" "$SIGNER_PASSWD" "$SIGNER_PASSWD")
      RESULT=$(echo "$sig_pw" | thornode keys --keyring-backend file add "$SIGNER_NAME" --output json 2>&1)
      SIGNER_SEED_PHRASE=$(echo "$RESULT" | jq -r '.mnemonic')
    fi
  fi
  NODE_PUB_KEY_ED25519=$(printf "%s\n%s\n" "$SIGNER_PASSWD" "$SIGNER_SEED_PHRASE" | thornode ed25519)
}
