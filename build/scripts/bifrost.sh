#!/bin/sh

set -o pipefail

CHAIN_API="${CHAIN_API:=127.0.0.1:1317}"

. "$(dirname "$0")/core.sh"
"$(dirname "$0")/wait-for-thorchain-api.sh" $CHAIN_API

create_thor_user "$SIGNER_NAME" "$SIGNER_PASSWD" "$SIGNER_SEED_PHRASE"

# TODO: Refactor this logic into the config package.
if [ -n "$PEER" ]; then
  OLD_IFS=$IFS
  IFS=","
  SEED_LIST=""
  for SEED in $PEER; do
    # check if we have a hostname we extract the IP
    if ! expr "$SEED" : '[0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*$' >/dev/null; then
      SEED=$(host "$SEED" | awk '{print $4}')
    fi
    SEED_ID=$(curl -m 10 -sL --fail "http://$SEED:6040/p2pid") || continue
    SEED="/ip4/$SEED/tcp/5040/ipfs/$SEED_ID"
    if [ -z "$SEED_LIST" ]; then
      SEED_LIST="${SEED}"
    else
      SEED_LIST="${SEED_LIST},${SEED}"
    fi
  done
  IFS=$OLD_IFS
  PEER=$SEED_LIST
fi

# dynamically set external ip if mocknet and unset
if [ "$NET" = "mocknet" ] && [ -z "$EXTERNAL_IP" ]; then
  EXTERNAL_IP=$(hostname -i)
fi

export SIGNER_NAME SIGNER_PASSWD
exec "$@"
