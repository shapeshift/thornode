#!/bin/sh

set -ex

DATE=$(date +%s)

# backup first
cp -r ~/.thord/config ~/.thord/config.$DATE.bak
cp -r ~/.thord/data ~/.thord/data.$DATE.bak

# export genesis file
gaiad export > thorchain_genesis_export.$DATE.json

# reset data, unsafe
thord unsafe-reset-all

# copied exported genesis file to the config directory
cp thorchain_genesis_export.$DATE.json ~/.thord/config/genesis.json
