include mocknet/Makefile
.PHONY: build

TAG ?= mocknet
IMAGE ?= registry.gitlab.com/thorchain/thornode

build:
	docker build -t ${IMAGE}:${TAG} --build-arg TAG=${TAG} -f Dockerfile ../..

clean:
	@rm -rf /tmp/genesis
	@rm -rf ~/.thor*
	@rm -rf /tmp/shared
	@mkdir /tmp/genesis
	@mkdir /tmp/shared
	@mkdir ~/.thord
	@mkdir ~/.thornode
	@mkdir ~/.thornode/standalone
	@mkdir ~/.thornode/tss
	@mkdir ~/.thornode/genesis_1
	@mkdir ~/.thornode/genesis_2
	@mkdir ~/.thornode/genesis_3
	@mkdir ~/.thornode/genesis_4
	@mkdir ~/.thornode/validator
	@mkdir ~/.thornode/fullnode

clean-validator:
	@rm -rf ~/.thornode/validator
	@mkdir -p ~/.thornode/validator

clean-fullnode:
	@rm -rf ~/.thornode/fullnode
	@mkdir -p ~/.thornode/fullnode

stop:
	BUILD_FLAG="" docker-compose -p thornode -f ${path}../components/standalone.base.yml down --remove-orphans

run-validator:
	docker-compose -f validator.yml --project-name validator up --force-recreate -d

reset-validator: clean-validator run-validator

run-fullnode:
	docker-compose -f fullnode.yml --project-name validator up --force-recreate -d

reset-fullnode: clean-fullnode run-fullnode

run-mainnet-binance:
	@mkdir -p ${HOME}/.bnbchaind && chown 1000:1000 ${HOME}/.bnbchaind
	@mkdir -p ${HOME}/.bnbcli && chown 1000:1000 ${HOME}/.bnbcli
	docker-compose -p thornode -f ${path}../components/binance-mainnet.yml up --force-recreate --remove-orphans -d

clean-mainnet-binance:
	@rm -rf ${HOME}/.bnbchaind
	@rm -rf ${HOME}/.bnbcli

reset-mainnet-binance: stop clean-mainnet-binance run-mainnet-binance
