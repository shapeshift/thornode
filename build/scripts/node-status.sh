#!/bin/sh

set -o pipefail

format_1e8 () {
  printf "%.2f\n" $(jq -n $1/100000000 2> /dev/null) 2> /dev/null | sed ':a;s/\B[0-9]\{3\}\>/,&/;ta'
}

format_int () {
  printf "%.0f\n" $1 2> /dev/null | sed ':a;s/\B[0-9]\{3\}\>/,&/;ta'
}

API=http://thor-api:1317
THOR_DAEMON_SERVICE_PORT_RPC="${THOR_DAEMON_SERVICE_PORT_RPC:=26657}"
BINANCE_DAEMON_SERVICE_PORT_RPC="${BINANCE_DAEMON_SERVICE_PORT_RPC:=26657}"
BITCOIN_DAEMON_SERVICE_PORT_RPC="${BITCOIN_DAEMON_SERVICE_PORT_RPC:=18443}"

ADDRESS=$(echo $SIGNER_PASSWD | thorcli keys show $SIGNER_NAME -a)
JSON=$(curl -sL --fail -m 10 $API/thorchain/node/$ADDRESS)

IP=$(echo $JSON | jq -r ".ip_address")
VERSION=$(echo $JSON | jq -r ".version")
BOND=$(echo $JSON | jq -r ".bond")
REWARDS=$(echo $JSON | jq -r ".current_award")
SLASH=$(echo $JSON | jq -r ".slash_points")
STATUS=$(echo $JSON | jq -r ".status")
PREFLIGHT=$(echo $JSON | jq -r  ".preflight_status")

if [ "$VALIDATOR" == "true" ]; then
  # calculate BNB chain sync progress
  [ "$NET" = "mainnet" ] && BNB_PEER=dataseed1.binance.org || BNB_PEER=data-seed-pre-0-s1.binance.org
  BNB_HEIGHT=$(curl -sL --fail -m 10 $BNB_PEER/status | jq -r ".result.sync_info.latest_block_height")
  BNB_SYNC_HEIGHT=$(curl -sL --fail -m 10 binance-daemon:$BINANCE_DAEMON_SERVICE_PORT_RPC/status | jq -r ".result.sync_info.index_height")
  BNB_PROGRESS=$(printf "%.3f%%" $(jq -n $BNB_SYNC_HEIGHT/$BNB_HEIGHT*100 2> /dev/null) 2> /dev/null) || BNB_PROGRESS=Error

  # calculate BTC chain sync progress
  BTC_RESULT=$(curl -sL --fail -m 10 --data-binary '{"jsonrpc": "1.0", "id": "node-status", "method": "getblockchaininfo", "params": []}' -H 'content-type: text/plain;' http://thorchain:password@bitcoin-daemon:$BITCOIN_DAEMON_SERVICE_PORT_RPC)
  BTC_HEIGHT=$(echo $BTC_RESULT | jq -r ".result.headers")
  BTC_SYNC_HEIGHT=$(echo $BTC_RESULT | jq -r ".result.blocks")
  BTC_PROGRESS=$(echo $BTC_RESULT | jq -r ".result.verificationprogress")
  BTC_PROGRESS=$(printf "%.3f%%" $(jq -n $BTC_PROGRESS*100 2> /dev/null) 2> /dev/null) || BTC_PROGRESS=Error
fi

# calculate THOR chain sync progress
THOR_SYNC_HEIGHT=$(curl -sL --fail -m 10 localhost:$THOR_DAEMON_SERVICE_PORT_RPC/status | jq -r ".result.sync_info.latest_block_height")
if [ "$PEER" != "" ]; then
  THOR_HEIGHT=$(curl -sL --fail -m 10 $PEER:$THOR_DAEMON_SERVICE_PORT_RPC/status | jq -r ".result.sync_info.latest_block_height")
elif [ "$SEEDS" != "" ]; then
  OLD_IFS=$IFS
  IFS=","
  for PEER in $SEEDS
  do
    THOR_HEIGHT=$(curl -sL --fail -m 10 $PEER:$THOR_DAEMON_SERVICE_PORT_RPC/status | jq -r ".result.sync_info.latest_block_height") || continue
    break
  done
  IFS=$OLD_IFS
else
  THOR_HEIGHT=$THOR_SYNC_HEIGHT
fi
THOR_PROGRESS=$(printf "%.3f%%" $(jq -n $THOR_SYNC_HEIGHT/$THOR_HEIGHT*100 2> /dev/null) 2> /dev/null) || THOR_PROGRESS=Error


cat << "EOF"
 ________ ______  ___  _  __        __
/_  __/ // / __ \/ _ \/ |/ /__  ___/ /__
 / / / _  / /_/ / , _/    / _ \/ _  / -_)
/_/ /_//_/\____/_/|_/_/|_/\___/\_,_/\__/
EOF
echo

if [ "$VALIDATOR" == "true" ]; then
  echo "ADDRESS     $ADDRESS"
  echo "IP          $IP"
  echo "VERSION     $VERSION"
  echo "STATUS      $STATUS"
  echo "BOND        $(format_1e8 $BOND)"
  echo "REWARDS     $(format_1e8 $REWARDS)"
  echo "SLASH       $(format_int $SLASH)"
  echo "PREFLIGHT   "$PREFLIGHT
fi

echo
echo "API         http://$IP:1317/thorchain/doc"
echo "RPC         http://$IP:$THOR_DAEMON_SERVICE_PORT_RPC"
echo "MIDGARD     http://$IP:8080/v1/doc"

echo
printf "%-11s %-10s %-10s\n" CHAIN SYNC BLOCKS
printf "%-11s %-10s %-10s\n" THORChain "$THOR_PROGRESS" "$(format_int $THOR_SYNC_HEIGHT)/$(format_int $THOR_HEIGHT)"
[ "$VALIDATOR" == "true" ] && printf "%-11s %-10s %-10s\n" Binance "$BNB_PROGRESS" "$(format_int $BNB_SYNC_HEIGHT)/$(format_int $BNB_HEIGHT)"
[ "$VALIDATOR" == "true" ] && printf "%-11s %-10s %-10s\n" Bitcoin "$BTC_PROGRESS" "$(format_int $BTC_SYNC_HEIGHT)/$(format_int $BTC_HEIGHT)"
exit 0
