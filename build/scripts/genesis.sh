#!/bin/sh

. $(dirname "$0")/core.sh

SIGNER_NAME="${SIGNER_NAME:=thorchain}"
SIGNER_PASSWD="${SIGNER_PASSWD:=password}"
NODES="${NODES:=1}"
SEED="${SEED:=thor-daemon}" # the hostname of the master node

# find or generate our BNB address
gen_bnb_address
ADDRESS=$(cat ~/.bond/address.txt)

# create thorchain user, if it doesn't already
thorcli keys show $SIGNER_NAME
if [ $? -gt 0 ]; then
    if [ "$SIGNER_SEED_PHRASE" != "" ]; then
        printf "$SIGNER_PASSWD\n$SIGNER_SEED_PHRASE\n" | thorcli keys add $SIGNER_NAME --recover
    else
        printf $SIGNER_PASSWD | thorcli --trace keys add $SIGNER_NAME
    fi
fi

VALIDATOR=$(thord tendermint show-validator)
NODE_ADDRESS=$(thorcli keys show thorchain -a)
NODE_PUB_KEY=$(thorcli keys show thorchain -p)
VERSION=$(fetch_version)

mkdir -p /tmp/shared

if [ "$SEED" = "$(hostname)" ]; then
    echo "I AM THE SEED NODE"
    thord tendermint show-node-id > /tmp/shared/node.txt
fi

# write node account data to json file in shared directory
echo "$NODE_ADDRESS $VALIDATOR $NODE_PUB_KEY $VERSION $ADDRESS" > /tmp/shared/node_$NODE_ADDRESS.json

# wait until THORNode have the correct number of nodes in our directory before continuing
while [ "$(ls -1 /tmp/shared/node_*.json | wc -l | tr -d '[:space:]')" != "$NODES" ]; do
    sleep 1
done

if [ "$SEED" = "$(hostname)" ]; then
    if [ ! -f ~/.thord/config/genesis.json ]; then
        # get a list of addresses (thor bech32)
        ADDRS=""
        for f in /tmp/shared/node_*.json; do
            ADDRS="$ADDRS,$(cat $f | awk '{print $1}')"
        done
        init_chain $(echo "$ADDRS" | sed -e 's/^,*//')

        if [ ! -z ${VAULT_PUBKEY+x} ]; then
            PUBKEYS=""
            for f in /tmp/shared/node_*.json; do
                PUBKEYS="$PUBKEYS,$(cat $f | awk '{print $3}')"
            done
            add_vault $VAULT_PUBKEY $(echo "$PUBKEYS" | sed -e 's/^,*//')
        fi

        NODE_IP_ADDRESS=$(curl -s http://whatismyip.akamai.com/)

        # add node accounts to genesis file
        for f in /tmp/shared/node_*.json; do
            if [ ! -z ${VAULT_PUBKEY+x} ]; then
                add_node_account $(cat $f | awk '{print $1}') $(cat $f | awk '{print $2}') $(cat $f | awk '{print $3}') $(cat $f | awk '{print $4}') $(cat $f | awk '{print $5}') $NODE_IP_ADDRESS $VAULT_PUBKEY
            else
                add_node_account $(cat $f | awk '{print $1}') $(cat $f | awk '{print $2}') $(cat $f | awk '{print $3}') $(cat $f | awk '{print $4}') $(cat $f | awk '{print $5}') $NODE_IP_ADDRESS
            fi
        done

        # add gases
        add_gas_config "BNB.BNB" 37500 30000

        # disable default bank transfer, and opt to use our own custom one
        disable_bank_send

        # for mocknet, add heimdall balances
        echo "NET: $NET"
        if [ "$NET" == "mocknet" ]; then
            echo "setting up accounts ...."
            add_account thor1j08ys4ct2hzzc2hcz6h2hgrvlmsjynaw02vym4 50000000000
            add_account thor1zupk5lmc84r2dh738a9g3zscavannjy3h4s0hw 150000000001
            add_account thor1qqnde7kqe5sf96j6zf8jpzwr44dh4gkdftjnal 50900000000

            reserve 40000000000000
        else
            reserve 22000000000000000
        fi

        cat ~/.thord/config/genesis.json
        thord validate-genesis
    fi
fi

# setup peer connection
if [ "$SEED" != "$(hostname)" ]; then
    if [ ! -f ~/.thord/config/genesis.json ]; then
        echo "I AM NOT THE SEED"

        init_chain $NODE_ADDRESS
        fetch_genesis $SEED
        NODE_ID=$(fetch_node_id $SEED)
        echo "NODE ID: $NODE_ID"
        peer_list $NODE_ID $SEED

        cat ~/.thord/config/genesis.json
    fi
fi

exec "$@"
