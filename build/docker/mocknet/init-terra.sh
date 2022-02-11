#!/bin/sh

rm /root/.terra/config/*
cd /root/.terra/config

wget https://raw.githubusercontent.com/terra-money/LocalTerra/main/config/app.toml
wget https://raw.githubusercontent.com/terra-money/LocalTerra/main/config/client.toml
wget https://raw.githubusercontent.com/terra-money/LocalTerra/main/config/config.toml
wget https://raw.githubusercontent.com/terra-money/LocalTerra/main/config/genesis.json
wget https://raw.githubusercontent.com/terra-money/LocalTerra/main/config/node_key.json
wget https://raw.githubusercontent.com/terra-money/LocalTerra/main/config/priv_validator_key.json

terrad start
