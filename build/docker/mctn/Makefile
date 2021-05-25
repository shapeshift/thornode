mkfile_path := $(abspath $(lastword $(MAKEFILE_LIST)))
path := $(dir $(mkfile_path))
IMAGE ?= registry.gitlab.com/thorchain/thornode
TAG = testnet
SEEDS ?= $(shell curl -sL testnet-seed.thorchain.info | jq -r '.[]' | paste -sd, -)

ifeq ($(OS),Windows_NT)
    uname_S := Windows
else
    uname_S := $(shell uname -s)
endif

run-mctn-fullnode:
	IMAGE=${IMAGE} TAG=testnet NET=testnet SEEDS=${SEEDS} docker-compose -p thornode \
		-f ${path}../components/fullnode.yml -f ${path}../components/fullnode.${uname_S}.yml up --remove-orphans -d

reset-mctn-fullnode: stop clean run-mctn-fullnode
