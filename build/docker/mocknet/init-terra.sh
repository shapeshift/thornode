#!/bin/sh

rm /root/.terra/config/*
cd /root/.terra/config || exit

wget https://raw.githubusercontent.com/terra-money/LocalTerra/main/config/app.toml
wget https://raw.githubusercontent.com/terra-money/LocalTerra/main/config/client.toml
wget https://raw.githubusercontent.com/terra-money/LocalTerra/main/config/config.toml
wget https://raw.githubusercontent.com/terra-money/LocalTerra/main/config/genesis.json
wget https://raw.githubusercontent.com/terra-money/LocalTerra/main/config/node_key.json
wget https://raw.githubusercontent.com/terra-money/LocalTerra/main/config/priv_validator_key.json

if [ -n "$TERRA_BLOCK_TIME" ]; then
	sed -E -i "/timeout_(propose|prevote|precommit|commit)/s/[0-9]+m?s/$TERRA_BLOCK_TIME/" /root/.terra/config/config.toml
fi

# disable tax policy to be able to sign UST txs with only LUNA
apk add jq
jq '.app_state.treasury.tax_rate="0.0" | .app_state.treasury.params.tax_policy.rate_min="0.0" | .app_state.treasury.params.tax_policy.rate_max="0.0" | .app_state.treasury.params.tax_policy.change_rate_max="0.0"' genesis.json >temp.json
rm -rf genesis.json
mv temp.json genesis.json

terrad start --pruning=custom --pruning-keep-recent=100 --pruning-interval=100 --pruning-keep-every=100
