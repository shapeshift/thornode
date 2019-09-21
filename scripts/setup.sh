#!/bin/sh

set -x
set -e

while true; do

  make install
  ssd init local --chain-id sschain

  echo "password" | sscli keys add jack
  echo "password" | sscli keys add alice

  ssd add-genesis-account $(sscli keys show jack -a) 1000bep,100000000stake
  ssd add-genesis-account $(sscli keys show alice -a) 1000bep,100000000stake

  sscli config chain-id sschain
  sscli config output json
  sscli config indent true
  sscli config trust-node true

  echo "password" | ssd gentx --name jack
  ssd collect-gentxs

  # add jack as a trusted account
  cat ~/.ssd/config/genesis.json | jq ".app_state.swapservice.trust_accounts[0] = {\"signer_address\": \"bnb1lejrrtta9cgr49fuh7ktu3sddhe0ff7wenlYYY\", \"admin_address\": \"bnb1lejrrtta9cgr49fuh7ktu3sddhe0ff7wenlYYY\", \"observer_address\": \"$(sscli keys show jack -a)\"}" > /tmp/genesis.json
  mv /tmp/genesis.json ~/.ssd/config/genesis.json

  ssd validate-genesis

  break

done
