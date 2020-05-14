#!/bin/sh

echo "CACHE": $CACHE
SEQNO=$(curl https://testnet-dex.binance.org/api/v1/account/${POOL_ADDRESS} | jq '.sequence | tonumber')
echo $SEQNO
docker build $CACHE \
--build-arg chain_id=thorchain \
--build-arg pool_address=$POOL_ADDRESS \
--build-arg dex_host=testnet-dex.binance.org \
--build-arg db_path=/var/data \
--build-arg rpc_host=data-seed-pre-0-s3.binance.org \
--build-arg chain_host=127.0.0.1:1317 \
--build-arg signer_name=thorchain \
--build-arg signer_passwd=$SIGNER_PASSWD \
--build-arg binance_private_key=$BINANCE_PRIVATE_KEY \
--build-arg binance_test_net=Binance-Chain-Nile \
--build-arg trusted_bnb_address=$TRUSTED_BNB_ADDRESS \
--build-arg net=$NET \
--build-arg seqno=$SEQNO \
-t $1 .
