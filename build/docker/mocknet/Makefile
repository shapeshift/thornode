mkfile_path := $(abspath $(lastword $(MAKEFILE_LIST)))
path := $(dir $(mkfile_path))
IMAGE ?= registry.gitlab.com/thorchain/thornode

ifeq ($(OS),Windows_NT)
    uname_S := Windows
else
    uname_S := $(shell uname -s)
endif

run-mocknet-standalone:
	IMAGE=${IMAGE} TAG=mocknet NET=mocknet docker-compose -p thornode \
		-f ${path}../components/standalone.base.yml \
		-f ${path}../components/midgard.yml -f ${path}../components/midgard.${uname_S}.yml \
		-f ${path}../components/mock-binance.yml -f ${path}../components/bitcoin-regtest.yml -f ${path}../components/litecoin-regtest.yml \
		-f ${path}../components/ethereum-localnet.yml -f ${path}../components/bitcoin-cash-regtest.yml -f ${path}../components/dogecoin-regtest.yml \
		-f ${path}../mocknet/standalone.yml -f ${path}../components/standalone.${uname_S}.yml up --remove-orphans -d

reset-mocknet-standalone: stop clean run-mocknet-standalone

export-mocknet-standalone:
	IMAGE=${IMAGE} TAG=mocknet NET=mocknet docker-compose -f ${path}../components/standalone.base.yml -f ${path}../mocknet/standalone.yml -f ${path}../components/standalone.${uname_S}.yml run thornode sh -c "thornode export > ~/.thornode/genesis.json"

hard-fork-mocknet-standalone:
	IMAGE=${IMAGE} TAG=mocknet NET=mocknet docker-compose -f ${path}../components/standalone.base.yml -f ${path}../mocknet/standalone.yml -f ${path}../components/standalone.${uname_S}.yml run thornode sh -c "/docker/scripts/hard-fork.sh"

run-mocknet-genesis:
	IMAGE=${IMAGE} TAG=mocknet NET=mocknet docker-compose -p thornode -f ${path}../components/genesis.base.yml -f ${path}../components/frontend.yml -f ${path}../components/midgard.yml -f ${path}../components/midgard.${uname_S}.yml -f ${path}../components/mock-binance.yml -f ${path}../mocknet/genesis.yml up --remove-orphans -d

run-mocknet-genesis-logs:
	IMAGE=${IMAGE} TAG=mocknet NET=mocknet docker-compose -p thornode -f ${path}../components/genesis.base.yml -f ${path}../components/frontend.yml -f ${path}../components/midgard.yml -f ${path}../components/midgard.${uname_S}.yml -f ${path}../components/mock-binance.yml -f ${path}../mocknet/genesis.yml logs -f

reset-mocknet-genesis: stop clean run-mocknet-genesis

export-mocknet-genesis:
	IMAGE=${IMAGE} TAG=mocknet NET=mocknet docker-compose -f ${path}../components/genesis.base.yml -f ${path}../mocknet/genesis.yml run thornode sh -c "thornode export > ~/.thornode/genesis.json"

run-mocknet-genesis-churn:
	IMAGE=${IMAGE} TAG=mocknet NET=mocknet docker-compose -p thornode -f ${path}../components/genesis.base.yml -f ${path}../components/fifthnode.yml -f ${path}../components/frontend.yml -f ${path}../components/midgard.yml -f ${path}../components/midgard.${uname_S}.yml -f ${path}../components/mock-binance.yml -f ${path}../mocknet/genesis.yml -f ${path}../mocknet/churn.yml up --remove-orphans -d

run-mocknet-genesis-churn-logs:
	IMAGE=${IMAGE} TAG=mocknet NET=mocknet docker-compose -p thornode -f ${path}../components/genesis.base.yml -f ${path}../components/fifthnode.yml -f ${path}../components/frontend.yml -f ${path}../components/midgard.yml -f ${path}../components/midgard.${uname_S}.yml -f ${path}../components/mock-binance.yml -f ${path}../mocknet/genesis.yml -f ${path}../mocknet/churn.yml logs -f

reset-mocknet-genesis-churn: stop clean run-mocknet-genesis-churn

run-mocknet-validator:
	IMAGE=${IMAGE} TAG=mocknet NET=mocknet docker-compose -p thornode -f ${path}../components/frontend.yml -f ${path}../components/midgard.yml -f ${path}../components/midgard.${uname_S}.yml -f ${path}../components/validator.yml -f ${path}../components/validator.${uname_S}.yml up --remove-orphans -d

reset-mocknet-validator: stop clean run-mocknet-validator

export-mocknet-validator:
	IMAGE=${IMAGE} TAG=mocknet NET=mocknet docker-compose -f ${path}../components/validator.yml -f ${path}../components/validator.${uname_S}.yml run thornode sh -c "thornode export > ~/.thornode/genesis.json"

hard-fork-mocknet-validator:
	IMAGE=${IMAGE} TAG=mocknet NET=mocknet docker-compose -f ${path}../components/validator.yml -f ${path}../components/validator.${uname_S}.yml run thornode sh -c "/docker/scripts/hard-fork.sh"
