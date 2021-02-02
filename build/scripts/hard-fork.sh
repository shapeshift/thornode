#!/bin/sh

set -ex

DATE=$(date +%s)

# backup first
cp -r ~/.thornode/config ~/.thornode/config.$DATE.bak
cp -r ~/.thornode/data ~/.thornode/data.$DATE.bak

# export genesis file
thornode export >thorchain_genesis_export.$DATE.json

# reset data, unsafe
thornode unsafe-reset-all

# copied exported genesis file to the config directory
cp thorchain_genesis_export.$DATE.json ~/.thornode/config/genesis.json

