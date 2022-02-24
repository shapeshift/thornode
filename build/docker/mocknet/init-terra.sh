#!/bin/sh

rm /root/.terra/config/*
cd /root/.terra/config || exit

wget https://raw.githubusercontent.com/terra-money/LocalTerra/main/config/app.toml
wget https://raw.githubusercontent.com/terra-money/LocalTerra/main/config/client.toml
wget https://raw.githubusercontent.com/terra-money/LocalTerra/main/config/config.toml
wget https://raw.githubusercontent.com/terra-money/LocalTerra/main/config/genesis.json
wget https://raw.githubusercontent.com/terra-money/LocalTerra/main/config/node_key.json
wget https://raw.githubusercontent.com/terra-money/LocalTerra/main/config/priv_validator_key.json

if [ -n "$BLOCK_TIME" ]; then
	sed -E -i "/timeout_(propose|prevote|precommit|commit)/s/[0-9]+m?s/$BLOCK_TIME/" /root/.terra/config/config.toml
fi

terrad start --pruning=custom --pruning-keep-recent=100 --pruning-interval=100 --pruning-keep-every=100
