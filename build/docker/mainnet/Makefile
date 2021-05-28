mkfile_path := $(abspath $(lastword $(MAKEFILE_LIST)))
path := $(dir $(mkfile_path))
IMAGE ?= registry.gitlab.com/thorchain/thornode
TAG = mainnet
SEEDS = $(shell curl -sL seed.thorchain.info | jq -r '.[]' | paste -sd, -)

ifeq ($(OS),Windows_NT)
    uname_S := Windows
else
    uname_S := $(shell uname -s)
endif

run-fullnode:
	IMAGE=${IMAGE} TAG=mainnet NET=mainnet SEEDS=${SEEDS} docker-compose -p thornode \
		-f ${path}../components/fullnode.yml -f ${path}../components/fullnode.${uname_S}.yml up --remove-orphans -d

reset-fullnode: stop clean-fullnode run-fullnode

clean-fullnode:
	@rm -rf ~/.thornode/fullnode
	@mkdir -p ~/.thornode/fullnode

stop:
	BUILD_FLAG="" docker-compose -p thornode -f ${path}../components/standalone.base.yml down --remove-orphans
