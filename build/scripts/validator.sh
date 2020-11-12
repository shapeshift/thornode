#!/bin/sh

set -o pipefail

source $(dirname "$0")/core.sh

SEEDS="${SEEDS:=none}" # the hostname of multiple seeds set as tendermint seeds
PEER="${PEER:=none}" # the hostname of a seed node set as tendermint persistent peer
PEER_API="${PEER_API:=$PEER}" # the hostname of a seed node API if different
SIGNER_NAME="${SIGNER_NAME:=thorchain}"
SIGNER_PASSWD="${SIGNER_PASSWD:=password}"
BINANCE=${BINANCE:=$PEER:26660}

if [ ! -f ~/.thord/config/genesis.json ]; then
    if [[ "$PEER" == "none" && "$SEEDS" == "none" ]]; then
        echo "Missing PEER / SEEDS"
        exit 1
    fi

    # config the keyring to use file backend
    thorcli config keyring-backend file

    # create thorchain user, if it doesn't already
    echo $SIGNER_PASSWD | thorcli keys show $SIGNER_NAME
    if [ $? -gt 0 ]; then
      if [ "$SIGNER_SEED_PHRASE" != "" ]; then
        printf "$SIGNER_SEED_PHRASE\n$SIGNER_PASSWD\n$SIGNER_PASSWD\n" | thorcli keys add $SIGNER_NAME --recover
        NODE_PUB_KEY_ED25519=$(echo "$SIGNER_PASSWD\n$SIGNER_SEED_PHRASE\n" | thorcli ed25519)
      else
        RESULT=$(printf "$SIGNER_PASSWD\n$SIGNER_PASSWD\n" | thorcli keys add $SIGNER_NAME -o json 2>&1 >/dev/null)
        MNEMONIC=$(echo $RESULT|jq -r '.mnemonic')
        NODE_PUB_KEY_ED25519=$(printf "$SIGNER_PASSWD\n$MNEMONIC\n" | thorcli ed25519)
      fi
    fi

    NODE_ADDRESS=$(echo $SIGNER_PASSWD | thorcli keys show $SIGNER_NAME -a)
    init_chain $NODE_ADDRESS

    if [[ "$PEER" != "none" ]]; then
      fetch_genesis $PEER

      # add persistent peer tendermint config
      NODE_ID=$(fetch_node_id $PEER)
      peer_list $NODE_ID $PEER
    fi

    if [[ "$SEEDS" != "none" ]]; then
      fetch_genesis_from_seeds $SEEDS

      # add seeds tendermint config
      seeds_list $SEEDS
    fi

    # enable telemetry through prometheus metrics endpoint
    enable_telemetry

    # enable internal traffic as well
    enable_internal_traffic

    # use external IP if available
    [ ! -z $EXTERNAL_IP ] && external_address $EXTERNAL_IP $NET

    if [[ "$NET" == "mocknet" ]]; then
        # create a binance wallet and bond/register
        gen_bnb_address
        ADDRESS=$(cat ~/.bond/address.txt)

        # send bond transaction to mock binance
        $(dirname "$0")/mock-bond.sh $BINANCE $ADDRESS $NODE_ADDRESS $PEER_API

        # switch the BNB bond to native RUNE
        $(dirname "$0")/mock-switch.sh $BINANCE $ADDRESS $NODE_ADDRESS $PEER_API

        sleep 30 # wait for thorchain to register the new node account
        # send native rune tx
        $(dirname "$0")/native-rune-payload.sh $PEER_API '[{"asset": "RUNE","amount": "100000000000000"}]' "bond:$NODE_ADDRESS" $NODE_ADDRESS tmp.json

        printf "$SIGNER_PASSWD\n$SIGNER_PASSWD\n" | thorcli tx sign tmp.json --node tcp://$PEER:26657 --from $SIGNER_NAME --output-document tmp-signed.json
        printf "$SIGNER_PASSWD\n" | thorcli tx broadcast tmp-signed.json  --node tcp://$PEER:26657 --from $SIGNER_NAME
        # send bond

        NODE_PUB_KEY=$(echo $SIGNER_PASSWD | thorcli keys show thorchain --pubkey)
        VALIDATOR=$(thord tendermint show-validator)

        # set node keys
        until printf "$SIGNER_PASSWD\n$SIGNER_PASSWD\n" | thorcli tx thorchain set-node-keys $NODE_PUB_KEY $NODE_PUB_KEY_ED25519 $VALIDATOR --node tcp://$PEER:26657 --from $SIGNER_NAME --yes; do
          sleep 5
        done

        # add IP address
        sleep 10 # wait for thorchain to commit a block

        NODE_IP_ADDRESS=${EXTERNAL_IP:=$(curl -s http://whatismyip.akamai.com)}
        until printf "$SIGNER_PASSWD\n$SIGNER_PASSWD\n" | thorcli tx thorchain set-ip-address $NODE_IP_ADDRESS --node tcp://$PEER:26657 --from $SIGNER_NAME --yes; do
          sleep 5
        done

        sleep 10 # wait for thorchain to commit a block
        # set node version
        until printf "$SIGNER_PASSWD\n$SIGNER_PASSWD\n" | thorcli tx thorchain set-version --node tcp://$PEER:26657 --from $SIGNER_NAME --yes; do
          sleep 5
        done

    elif [[ "$NET" == "testnet" ]]; then
        # create a binance wallet
        gen_bnb_address
        ADDRESS=$(cat ~/.bond/address.txt)
    else
        echo "YOUR NODE ADDRESS: $NODE_ADDRESS . Send your bond with this as your address."
    fi

else
    # update seeds tendermint config if available
    if [[ "$PEER" != "none" ]]; then
      # add persistent peer tendermint config
      NODE_ID=$(fetch_node_id $PEER)
      peer_list $NODE_ID $PEER
    fi

    if [[ "$SEEDS" != "none" ]]; then
      # add seeds tendermint config
      seeds_list $SEEDS
    fi
fi

(echo $SIGNER_NAME; echo $SIGNER_PASSWD ) | exec "$@"
