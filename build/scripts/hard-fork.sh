set -ex

HARDFORK_BLOCK_HEIGHT="${HARDFORK_BLOCK_HEIGHT:--1}"
DATE=$(date +%s)

# backup first
cp -r ~/.thornode/config ~/.thornode/config.$DATE.bak
cp -r ~/.thornode/data ~/.thornode/data.$DATE.bak

# export genesis file
thornode export --height $HARDFORK_BLOCK_HEIGHT > thorchain_genesis_export.$DATE.json

# delete existing data files
rm -rf ~/.thornode/data/application.db ~/.thornode/data/blockstore.db ~/.thornode/data/cs.wal ~/.thornode/data/evidence.db ~/.thornode/data/snapshots ~/.thornode/data/state.db ~/.thornode/data/tx_index.db ~/.thornode/data/priv_validator_state.json

# delete old genesis file
rm -rf ~/.thornode/config/genesis.json

# if we need to run some command to change the state
echo '{
  "height": "0",
  "round": 0,
  "step": 0
}' > ~/.thornode/data/priv_validator_state.json

# copied exported genesis file to the config directory
cp thorchain_genesis_export.$DATE.json ~/.thornode/config/genesis.json