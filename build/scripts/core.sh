#!/usr/bin/env bash
# adds an account node into the genesis file
add_node_account () {
    NODE_ADDRESS=$1
    VALIDATOR=$2
    NODE_PUB_KEY=$3
    VERSION=$4
    BOND_ADDRESS=$5
    IP_ADDRESS=$6
    MEMBERSHIP=$7
    jq --arg IP_ADDRESS $IP_ADDRESS --arg VERSION $VERSION --arg BOND_ADDRESS "$BOND_ADDRESS" --arg VALIDATOR "$VALIDATOR" --arg NODE_ADDRESS "$NODE_ADDRESS" --arg NODE_PUB_KEY "$NODE_PUB_KEY" '.app_state.thorchain.node_accounts += [{"node_address": $NODE_ADDRESS, "version": $VERSION, "ip_address": $IP_ADDRESS, "status":"active", "active_block_height": "0", "bond_address":$BOND_ADDRESS, "signer_membership": [], "validator_cons_pub_key":$VALIDATOR, "pub_key_set":{"secp256k1":$NODE_PUB_KEY,"ed25519":$NODE_PUB_KEY}}]' <~/.thord/config/genesis.json >/tmp/genesis.json
    mv /tmp/genesis.json ~/.thord/config/genesis.json
    if [ ! -z "$MEMBERSHIP" ]; then
        jq --arg MEMBERSHIP "$MEMBERSHIP" '.app_state.thorchain.node_accounts[-1].signer_membership += [$MEMBERSHIP]' ~/.thord/config/genesis.json > /tmp/genesis.json
        mv /tmp/genesis.json ~/.thord/config/genesis.json
    fi
}

add_last_event_id () {
    echo "Adding last event id $1"
    jq --arg ID $1 '.app_state.thorchain.last_event_id = $ID' ~/.thord/config/genesis.json > /tmp/genesis.json
    mv /tmp/genesis.json ~/.thord/config/genesis.json
}

add_gas_config () {
    asset=$1
    shift

    # add asset to gas
    jq --argjson path "[\"app_state\", \"thorchain\", \"gas\", \"$asset\"]" 'getpath($path) = []' ~/.thord/config/genesis.json > /tmp/genesis.json
    mv /tmp/genesis.json ~/.thord/config/genesis.json

    for unit in $@; do
        jq --argjson path "[\"app_state\", \"thorchain\", \"gas\", \"$asset\"]" --arg unit "$unit" 'getpath($path) += [$unit]' ~/.thord/config/genesis.json > /tmp/genesis.json
        mv /tmp/genesis.json ~/.thord/config/genesis.json
    done
}

reserve () {
    jq --arg RESERVE $1 '.app_state.thorchain.reserve = $RESERVE' <~/.thord/config/genesis.json >/tmp/genesis.json
    mv /tmp/genesis.json ~/.thord/config/genesis.json
}


disable_bank_send () {
    jq '.app_state.bank.send_enabled = false' <~/.thord/config/genesis.json >/tmp/genesis.json
    mv /tmp/genesis.json ~/.thord/config/genesis.json
}

add_account () {
    jq --arg ADDRESS $1 --arg ASSET $2 --arg AMOUNT $3 '.app_state.auth.accounts += [{
        "type": "cosmos-sdk/Account",
        "value": {
          "address": $ADDRESS,
          "coins": [
            {
              "denom": $ASSET,
              "amount": $AMOUNT
            }
          ],
          "public_key": "",
          "account_number": 0,
          "sequence": 0
        }
    }]' <~/.thord/config/genesis.json >/tmp/genesis.json
    mv /tmp/genesis.json ~/.thord/config/genesis.json
}

add_vault () {
    POOL_PUBKEY=$1; shift

    jq --arg POOL_PUBKEY "$POOL_PUBKEY" '.app_state.thorchain.vaults += [{"block_height": "0", "pub_key": $POOL_PUBKEY, "coins":[], "type": "asgard", "status":"active", "status_since": "0", "membership":[]}]' <~/.thord/config/genesis.json >/tmp/genesis.json
    mv /tmp/genesis.json ~/.thord/config/genesis.json

    export IFS=","
    for pubkey in $@; do # iterate over our list of comma separated pubkeys
        jq --arg PUBKEY "$pubkey" '.app_state.thorchain.vaults[0].membership += [$PUBKEY]' ~/.thord/config/genesis.json > /tmp/genesis.json
        mv /tmp/genesis.json ~/.thord/config/genesis.json
    done
}

# inits a thorchain with a comman separate list of usernames
init_chain () {
    export IFS=","

    thord init local --chain-id thorchain
    echo $SIGNER_PASSWD | thorcli keys list

    for user in $@; do # iterate over our list of comma separated users "alice,jack"
        thord add-genesis-account $user 1000thor
    done

    thorcli config chain-id thorchain
    thorcli config output json
    thorcli config indent true
    thorcli config trust-node true
}

peer_list () {
    PEERUSER="$1@$2:26656"
    ADDR='addr_book_strict = true'
    ADDR_STRICT_FALSE='addr_book_strict = false'
    PEERSISTENT_PEER_TARGET='persistent_peers = ""'

    sed -i -e "s/$ADDR/$ADDR_STRICT_FALSE/g" ~/.thord/config/config.toml
    sed -i -e "s/$PEERSISTENT_PEER_TARGET/persistent_peers = \"$PEERUSER\"/g" ~/.thord/config/config.toml
}

enable_telemetry () {
    sed -i -e "s/prometheus = false/prometheus = true/g" ~/.thord/config/config.toml
}

gen_bnb_address () {
    if [ ! -f ~/.bond/private_key.txt ]; then
        echo "GENERATING BNB ADDRESSES"
        mkdir -p ~/.bond
        # because the generate command can get API rate limited, THORNode may need to retry
        n=0
        until [ $n -ge 60 ]; do
            generate > /tmp/bnb && break
            n=$[$n+1]
            sleep 1
        done
        ADDRESS=$(cat /tmp/bnb | grep MASTER= | awk -F= '{print $NF}')
        echo $ADDRESS > ~/.bond/address.txt
        BINANCE_PRIVATE_KEY=$(cat /tmp/bnb | grep MASTER_KEY= | awk -F= '{print $NF}')
        echo $BINANCE_PRIVATE_KEY > /root/.bond/private_key.txt
        PUBKEY=$(cat /tmp/bnb | grep MASTER_PUBKEY= | awk -F= '{print $NF}')
        echo $PUBKEY > /root/.bond/pubkey.txt
        MNEMONIC=$(cat /tmp/bnb | grep MASTER_MNEMONIC= | awk -F= '{print $NF}')
        echo $MNEMONIC > /root/.bond/mnemonic.txt
    fi
}



fetch_genesis () {
    until curl -s "$1:26657" > /dev/null; do
        sleep 3
    done
    curl -s $1:26657/genesis | jq .result.genesis > ~/.thord/config/genesis.json
    thord validate-genesis
    cat ~/.thord/config/genesis.json
}

fetch_node_id () {
    until curl -s "$1:26657" > /dev/null; do
        sleep 3
    done
    curl -s $1:26657/status | jq -r .result.node_info.id
}

set_node_keys () {
  SIGNER_NAME=$1
  SIGNER_PASSWD=$2
  PEER=$3
  NODE_PUB_KEY=$(echo $SIGNER_PASSWD | thorcli keys show thorchain --pubkey)
  VALIDATOR=$(thord tendermint show-validator)
  printf "$SIGNER_PASSWD\n$SIGNER_PASSWD\n" | thorcli tx thorchain set-node-keys $NODE_PUB_KEY $NODE_PUB_KEY $VALIDATOR --node tcp://$PEER:26657 --from $SIGNER_NAME --yes
}

set_ip_address () {
  SIGNER_NAME=$1
  SIGNER_PASSWD=$2
  PEER=$3
  printf "$SIGNER_PASSWD\n$SIGNER_PASSWD\n" | thorcli tx thorchain set-ip-address $(curl -s http://whatismyip.akamai.com) --node tcp://$PEER:26657 --from $SIGNER_NAME --yes
}

fetch_version () {
    thorcli query thorchain version --chain-id thorchain --trust-node --output json | jq -r .version
}
