#!/bin/sh

source $(dirname "$0")/core.sh

PEER="${PEER:=none}" # the hostname of a seed node
PEER_API="${PEER_API:=$PEER}" # the hostname of a seed node API if different
SIGNER_NAME="${SIGNER_NAME:=thorchain}"
SIGNER_PASSWD="${SIGNER_PASSWD:=password}"
BINANCE=${BINANCE:=$PEER}

if [ ! -f ~/.thord/config/genesis.json ]; then
    if [[ "$PEER" == "none" ]]; then
        echo "Missing PEER"
        exit 1
    fi

    thorcli keys show $SIGNER_NAME
    if [ $? -gt 0 ]; then
        if [ "$SIGNER_SEED_PHRASE" != "" ]; then
            printf "$SIGNER_PASSWD\n$SIGNER_SEED_PHRASE\n" | thorcli keys add $SIGNER_NAME --recover
        else
            printf $SIGNER_PASSWD | thorcli --trace keys add $SIGNER_NAME
        fi
    fi

    NODE_ADDRESS=$(thorcli keys show $SIGNER_NAME -a)
    init_chain $NODE_ADDRESS

    fetch_genesis $PEER

    NODE_ID=$(fetch_node_id $PEER)
    peer_list $NODE_ID $PEER

    if [[ "$NET" == "mocknet" ]]; then
        # create a binance wallet and bond/register
        gen_bnb_address
        ADDRESS=$(cat ~/.bond/address.txt)

        # send bond transaction to mock binance
        $(dirname "$0")/mock-bond.sh $BINANCE $ADDRESS $NODE_ADDRESS $PEER_API

        sleep 15 # wait for thorchain to register the new node account

        # set node keys
        until echo $SIGNER_PASSWD | thorcli tx thorchain set-node-keys $(thorcli keys show thorchain --pubkey) $(thorcli keys show thorchain --pubkey) $(thord tendermint show-validator) --node tcp://$PEER:26657 --from $SIGNER_NAME --yes; do
          sleep 5
        done

        # add IP address
        sleep 5 # wait for thorchain to commit a block

        until echo $SIGNER_PASSWD | thorcli tx thorchain set-ip-address $(curl -s http://whatismyip.akamai.com) --node tcp://$PEER:26657 --from $SIGNER_NAME --yes; do
          sleep 5
        done

    elif [[ "$NET" == "testnet" ]]; then
        # create a binance wallet
        gen_bnb_address
        ADDRESS=$(cat ~/.bond/address.txt)
    else
        echo "YOUR NODE ADDRESS: $NODE_ADDRESS . Send your bond with this as your address."
    fi

fi

exec "$@"
