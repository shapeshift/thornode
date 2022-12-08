#!/bin/sh

set -o pipefail
format_1e8() {
  printf "%.2f\n" "$(jq -n "$1"/100000000 2>/dev/null)" 2>/dev/null | sed ':a;s/\B[0-9]\{3\}\>/,&/;ta'
}

format_int() {
  printf "%.0f\n" "$1" 2>/dev/null | sed ':a;s/\B[0-9]\{3\}\>/,&/;ta'
}

calc_progress() {
  if [ "$1" = "$2" ]; then
    [ "$1" = "0" ] && echo "0.000%" || echo "100.000%"
  elif [ -n "$3" ]; then
    progress="$(echo "scale=6; $3 * 100" | bc 2>/dev/null)" 2>/dev/null && printf "%.3f%%" "$progress" || echo "Error"
  else
    progress="$(echo "scale=6; $1/$2 * 100" | bc 2>/dev/null)" 2>/dev/null && printf "%.3f%%" "$progress" || echo "Error"
  fi
}

API=http://thornode:1317
THORNODE_PORT="${THORNODE_SERVICE_PORT_RPC:-27147}"

BINANCE_ENDPOINT="${BINANCE_HOST:-binance-daemon:${BINANCE_DAEMON_SERVICE_PORT_RPC:-26657}}"
BITCOIN_ENDPOINT="${BTC_HOST:-bitcoin-daemon:${BITCOIN_DAEMON_SERVICE_PORT_RPC:-8332}}"
LITECOIN_ENDPOINT="${LTC_HOST:-litecoin-daemon:${LITECOIN_DAEMON_SERVICE_PORT_RPC:-9332}}"
BITCOIN_CASH_ENDPOINT="${BCH_HOST:-bitcoin-cash-daemon:${BITCOIN_CASH_DAEMON_SERVICE_PORT_RPC:-8332}}"
DOGECOIN_ENDPOINT="${DOGE_HOST:-dogecoin-daemon:${DOGECOIN_DAEMON_SERVICE_PORT_RPC:-22555}}"
ETHEREUM_ENDPOINT="${ETH_HOST:-http://ethereum-daemon:${ETHEREUM_DAEMON_SERVICE_PORT_RPC:-8545}}"
ETHEREUM_BEACON_ENDPOINT=$(echo "$ETHEREUM_ENDPOINT" | sed 's/:[0-9]*$/:3500/g')
GAIA_ENDPOINT="${GAIA_HOST:-http://gaia-daemon:26657}"
AVALANCHE_ENDPOINT="${AVAX_HOST:-http://avalanche-daemon:9650/ext/bc/C/rpc}"

ADDRESS=$(echo "$SIGNER_PASSWD" | thornode keys show "$SIGNER_NAME" -a --keyring-backend file)
JSON=$(curl -sL --fail -m 10 "$API/thorchain/node/$ADDRESS")

IP=$(echo "$JSON" | jq -r ".ip_address")
VERSION=$(echo "$JSON" | jq -r ".version")
BOND=$(echo "$JSON" | jq -r ".total_bond")
REWARDS=$(echo "$JSON" | jq -r ".current_award")
SLASH=$(echo "$JSON" | jq -r ".slash_points")
STATUS=$(echo "$JSON" | jq -r ".status")
PREFLIGHT=$(echo "$JSON" | jq -r ".preflight_status")
[ "$VALIDATOR" = "false" ] && IP=$EXTERNAL_IP

if [ "$VALIDATOR" = "true" ]; then
  # calculate BNB chain sync progress
  if [ "$NET" = "mainnet" ] || [ "$NET" = "stagenet" ]; then # Seeds from https://docs.binance.org/smart-chain/developer/rpc.html
    BNB_PEERS='https://dataseed1.binance.org https://dataseed2.binance.org https://dataseed3.binance.org https://dataseed4.binance.org'
  else
    BNB_PEERS='http://data-seed-pre-0-s1.binance.org http://data-seed-pre-1-s1.binance.org http://data-seed-pre-2-s1.binance.org http://data-seed-pre-0-s3.binance.org http://data-seed-pre-1-s3.binance.org'
  fi
  for BNB_PEER in ${BNB_PEERS}; do
    BNB_HEIGHT=$(curl -sL --fail -m 10 "$BNB_PEER"/status | jq -e -r ".result.sync_info.latest_block_height") || continue
    if [ -z "$BNB_HEIGHT" ]; then continue; fi # Continue if empty height (malformed/bad json reply?)
    break
  done
  BNB_SYNC_HEIGHT=$(curl -sL --fail -m 10 "$BINANCE_ENDPOINT"/status | jq -r ".result.sync_info.index_height")
  BNB_PROGRESS=$(calc_progress "$BNB_SYNC_HEIGHT" "$BNB_HEIGHT")

  # calculate BTC chain sync progress
  BTC_RESULT=$(curl -sL --fail -m 10 --data-binary '{"jsonrpc": "1.0", "id": "node-status", "method": "getblockchaininfo", "params": []}' -H 'content-type: text/plain;' http://thorchain:password@"$BITCOIN_ENDPOINT")
  BTC_HEIGHT=$(echo "$BTC_RESULT" | jq -r ".result.headers")
  BTC_SYNC_HEIGHT=$(echo "$BTC_RESULT" | jq -r ".result.blocks")
  BTC_PROGRESS=$(echo "$BTC_RESULT" | jq -r ".result.verificationprogress")
  BTC_PROGRESS=$(calc_progress "$BTC_SYNC_HEIGHT" "$BTC_HEIGHT" "$BTC_PROGRESS")

  # calculate LTC chain sync progress
  LTC_RESULT=$(curl -sL --fail -m 10 --data-binary '{"jsonrpc": "1.0", "id": "node-status", "method": "getblockchaininfo", "params": []}' -H 'content-type: text/plain;' http://thorchain:password@"$LITECOIN_ENDPOINT")
  LTC_HEIGHT=$(echo "$LTC_RESULT" | jq -r ".result.headers")
  LTC_SYNC_HEIGHT=$(echo "$LTC_RESULT" | jq -r ".result.blocks")
  LTC_PROGRESS=$(echo "$LTC_RESULT" | jq -r ".result.verificationprogress")
  LTC_PROGRESS=$(calc_progress "$LTC_SYNC_HEIGHT" "$LTC_HEIGHT" "$LTC_PROGRESS")

  ETH_RESULT=$(curl -X POST -sL --fail -m 10 --data '{"jsonrpc":"2.0","method":"eth_syncing","params":[],"id":1}' -H 'content-type: application/json' "$ETHEREUM_ENDPOINT")
  if [ "$ETH_RESULT" = '{"jsonrpc":"2.0","id":1,"result":false}' ]; then
    ETH_RESULT=$(curl -X POST -sL --fail -m 10 --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' -H 'content-type: application/json' "$ETHEREUM_ENDPOINT")
    ETH_HEIGHT=$(printf "%.0f" "$(echo "$ETH_RESULT" | jq -r ".result")")
    ETH_SYNC_HEIGHT=$ETH_HEIGHT
    ETH_PROGRESS=$(calc_progress "$ETH_SYNC_HEIGHT" "$ETH_HEIGHT")
  elif [ -n "$ETH_RESULT" ]; then
    ETH_HEIGHT=$(printf "%.0f" "$(echo "$ETH_RESULT" | jq -r ".result.highestBlock")")
    ETH_SYNC_HEIGHT=$(printf "%.0f" "$(echo "$ETH_RESULT" | jq -r ".result.currentBlock")")
  else
    ETH_PROGRESS=Error
  fi

  # calculate ETH chain sync progress
  ETH_BEACON_RESULT=$(curl -sL --fail -m 10 "$ETHEREUM_BEACON_ENDPOINT/eth/v1/node/syncing")
  if [ -n "$ETH_BEACON_RESULT" ]; then
    ETH_BEACON_HEIGHT=$(echo "$ETH_BEACON_RESULT" | jq -r "(.data.head_slot|tonumber)+(.data.sync_distance|tonumber)")
    ETH_BEACON_SYNC_HEIGHT=$(echo "$ETH_BEACON_RESULT" | jq -r ".data.head_slot|tonumber")
    ETH_BEACON_PROGRESS=$(calc_progress "$ETH_BEACON_SYNC_HEIGHT" "$ETH_BEACON_HEIGHT")
  else
    ETH_BEACON_PROGRESS=Error
  fi

  # calculate BCH chain sync progress
  BCH_RESULT=$(curl -sL --fail -m 10 --data-binary '{"jsonrpc": "1.0", "id": "node-status", "method": "getblockchaininfo", "params": []}' -H 'content-type: text/plain;' http://thorchain:password@"$BITCOIN_CASH_ENDPOINT")
  BCH_HEIGHT=$(echo "$BCH_RESULT" | jq -r ".result.headers")
  BCH_SYNC_HEIGHT=$(echo "$BCH_RESULT" | jq -r ".result.blocks")
  BCH_PROGRESS=$(echo "$BCH_RESULT" | jq -r ".result.verificationprogress")
  BCH_PROGRESS=$(calc_progress "$BCH_SYNC_HEIGHT" "$BCH_HEIGHT" "$BCH_PROGRESS")

  # calculate DOGE chain sync progress
  if [ -z "$DOGECOIN_DISABLED" ]; then
    DOGE_RESULT=$(curl -sL --fail -m 10 --data-binary '{"jsonrpc": "1.0", "id": "node-status", "method": "getblockchaininfo", "params": []}' -H 'content-type: text/plain;' http://thorchain:password@"$DOGECOIN_ENDPOINT")
    DOGE_HEIGHT=$(echo "$DOGE_RESULT" | jq -r ".result.headers")
    DOGE_SYNC_HEIGHT=$(echo "$DOGE_RESULT" | jq -r ".result.blocks")
    DOGE_PROGRESS=$(echo "$DOGE_RESULT" | jq -r ".result.verificationprogress")
    DOGE_PROGRESS=$(calc_progress "$DOGE_SYNC_HEIGHT" "$DOGE_HEIGHT" "$DOGE_PROGRESS")
  fi

  # calculate Gaia chain sync progress
  if [ -z "$GAIA_DISABLED" ]; then
    GAIA_HEIGHT=$(curl -sL --fail -m 10 https://api.cosmos.network/blocks/latest | jq -e -r ".block.header.height")
    GAIA_SYNC_HEIGHT=$(curl -sL --fail -m 10 "$GAIA_ENDPOINT/status" | jq -r ".result.sync_info.latest_block_height")
    GAIA_PROGRESS=$(calc_progress "$GAIA_SYNC_HEIGHT" "$GAIA_HEIGHT")
  fi

  # calculate AVAX chain sync progress
  if [ -z "$AVALANCHE_DISABLED" ]; then
    AVAX_HEIGHT_RESULT=$(curl -X POST -sL --fail -m 10 --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' -H 'content-type: application/json' "https://api.avax.network/ext/bc/C/rpc")
    AVAX_SYNC_HEIGHT_RESULT=$(curl -X POST -sL --fail -m 10 --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' -H 'content-type: application/json' "$AVALANCHE_ENDPOINT")
    AVAX_HEIGHT=$(printf "%.0f" "$(echo "$AVAX_HEIGHT_RESULT" | jq -r ".result")")
    if [ -n "$AVAX_SYNC_HEIGHT_RESULT" ]; then
      AVAX_SYNC_HEIGHT=$(printf "%.0f" "$(echo "$AVAX_SYNC_HEIGHT_RESULT" | jq -r ".result")")
    else
      AVAX_SYNC_HEIGHT=0
    fi
    AVAX_PROGRESS=$(calc_progress "$AVAX_SYNC_HEIGHT" "$AVAX_HEIGHT")
  fi
fi

# calculate THOR chain sync progress
THOR_SYNC_HEIGHT=$(curl -sL --fail -m 10 thornode:"$THORNODE_PORT"/status | jq -r ".result.sync_info.latest_block_height")
if [ "$PEER" != "" ]; then
  THOR_HEIGHT=$(curl -sL --fail -m 10 "$PEER:$THORNODE_PORT/status" | jq -r ".result.sync_info.latest_block_height")
elif [ "$SEEDS" != "" ]; then
  OLD_IFS=$IFS
  IFS=","
  for PEER in $SEEDS; do
    THOR_HEIGHT=$(curl -sL --fail -m 10 "$PEER:$THORNODE_PORT/status" | jq -r ".result.sync_info.latest_block_height") || continue
    break
  done
  IFS=$OLD_IFS
elif [ "$NET" = "mainnet" ]; then
  THOR_HEIGHT=$(curl -sL --fail -m 10 https://rpc.ninerealms.com/status | jq -r ".result.sync_info.latest_block_height")
elif [ "$NET" = "stagenet" ]; then
  THOR_HEIGHT=$(curl -sL --fail -m 10 https://stagenet-rpc.ninerealms.com/status | jq -r ".result.sync_info.latest_block_height")
else
  THOR_HEIGHT=$THOR_SYNC_HEIGHT
fi
THOR_PROGRESS=$(printf "%.3f%%" "$(jq -n "$THOR_SYNC_HEIGHT"/"$THOR_HEIGHT"*100 2>/dev/null)" 2>/dev/null) || THOR_PROGRESS=Error

cat <<"EOF"
 ________ ______  ___  _  __        __
/_  __/ // / __ \/ _ \/ |/ /__  ___/ /__
 / / / _  / /_/ / , _/    / _ \/ _  / -_)
/_/ /_//_/\____/_/|_/_/|_/\___/\_,_/\__/
EOF
echo

if [ "$VALIDATOR" = "true" ]; then
  echo "ADDRESS     $ADDRESS"
  echo "IP          $IP"
  echo "VERSION     $VERSION"
  echo "STATUS      $STATUS"
  echo "BOND        $(format_1e8 "$BOND")"
  echo "REWARDS     $(format_1e8 "$REWARDS")"
  echo "SLASH       $(format_int "$SLASH")"
  echo "PREFLIGHT   $PREFLIGHT"
fi

echo
echo "API         http://$IP:1317/thorchain/doc/"
echo "RPC         http://$IP:$THORNODE_PORT"
echo "MIDGARD     http://$IP:8080/v2/doc"

# set defaults to avoid failures in math below
THOR_HEIGHT=${THOR_HEIGHT:=0}
THOR_SYNC_HEIGHT=${THOR_SYNC_HEIGHT:=0}
BNB_HEIGHT=${BNB_HEIGHT:=0}
BNB_SYNC_HEIGHT=${BNB_SYNC_HEIGHT:=0}
BTC_HEIGHT=${BTC_HEIGHT:=0}
BTC_SYNC_HEIGHT=${BTC_SYNC_HEIGHT:=0}
ETH_HEIGHT=${ETH_HEIGHT:=0}
ETH_SYNC_HEIGHT=${ETH_SYNC_HEIGHT:=0}
LTC_HEIGHT=${LTC_HEIGHT:=0}
LTC_SYNC_HEIGHT=${LTC_SYNC_HEIGHT:=0}
BCH_HEIGHT=${BCH_HEIGHT:=0}
BCH_SYNC_HEIGHT=${BCH_SYNC_HEIGHT:=0}
DOGE_HEIGHT=${DOGE_HEIGHT:=0}
DOGE_SYNC_HEIGHT=${DOGE_SYNC_HEIGHT:=0}
GAIA_HEIGHT=${GAIA_HEIGHT:=0}
GAIA_SYNC_HEIGHT=${GAIA_SYNC_HEIGHT:=0}

echo
printf "%-18s %-10s %-14s %-10s\n" CHAIN SYNC BEHIND TIP
printf "%-18s %-10s %-14s %-10s\n" THOR "$THOR_PROGRESS" "$(format_int $((THOR_SYNC_HEIGHT - THOR_HEIGHT)))" "$(format_int "$THOR_HEIGHT")"
[ "$VALIDATOR" = "true" ] && printf "%-18s %-10s %-14s %-10s\n" BNB "$BNB_PROGRESS" "$(format_int $((BNB_SYNC_HEIGHT - BNB_HEIGHT)))" "$(format_int "$BNB_HEIGHT")"
[ "$VALIDATOR" = "true" ] && printf "%-18s %-10s %-14s %-10s\n" BTC "$BTC_PROGRESS" "$(format_int $((BTC_SYNC_HEIGHT - BTC_HEIGHT)))" "$(format_int "$BTC_HEIGHT")"
[ "$VALIDATOR" = "true" ] && printf "%-18s %-10s %-14s %-10s\n" "ETH" "$ETH_PROGRESS" "$(format_int $((ETH_SYNC_HEIGHT - ETH_HEIGHT)))" "$(format_int "$ETH_HEIGHT")"
[ "$VALIDATOR" = "true" ] && printf "%-18s %-10s %-14s %-10s\n" "ETH (beacon slot)" "$ETH_BEACON_PROGRESS" "$(format_int $((ETH_BEACON_SYNC_HEIGHT - ETH_BEACON_HEIGHT)))" "$(format_int "$ETH_BEACON_HEIGHT")"
[ "$VALIDATOR" = "true" ] && printf "%-18s %-10s %-14s %-10s\n" LTC "$LTC_PROGRESS" "$(format_int $((LTC_SYNC_HEIGHT - LTC_HEIGHT)))" "$(format_int "$LTC_HEIGHT")"
[ "$VALIDATOR" = "true" ] && printf "%-18s %-10s %-14s %-10s\n" BCH "$BCH_PROGRESS" "$(format_int $((BCH_SYNC_HEIGHT - BCH_HEIGHT)))" "$(format_int "$BCH_HEIGHT")"
if [ "$VALIDATOR" = "true" ] && [ -z "$DOGECOIN_DISABLED" ]; then
  printf "%-18s %-10s %-14s %-10s\n" DOGE "$DOGE_PROGRESS" "$(format_int $((DOGE_SYNC_HEIGHT - DOGE_HEIGHT)))" "$(format_int "$DOGE_HEIGHT")"
fi
if [ "$VALIDATOR" = "true" ] && [ -z "$GAIA_DISABLED" ]; then
  printf "%-18s %-10s %-14s %-10s\n" GAIA "$GAIA_PROGRESS" "$(format_int $((GAIA_SYNC_HEIGHT - GAIA_HEIGHT)))" "$(format_int "$GAIA_HEIGHT")"
fi
if [ "$VALIDATOR" = "true" ] && [ -z "$AVALANCHE_DISABLED" ]; then
  printf "%-18s %-10s %-14s %-10s\n" AVAX "$AVAX_PROGRESS" "$(format_int $((AVAX_SYNC_HEIGHT - AVAX_HEIGHT)))" "$(format_int "$AVAX_HEIGHT")"
fi
exit 0
