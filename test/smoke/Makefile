include Makefile.cicd
IMAGE_NAME?=registry.gitlab.com/thorchain/heimdall
LOGLEVEL?=INFO
RUNE?=THOR.RUNE
DOCKER_OPTS = --network=host --rm -e RUNE=${RUNE} -e LOGLEVEL=${LOGLEVEL} -e PYTHONPATH=/app -v ${PWD}:/app -w /app

clean:
	rm *.pyc

build:
	@docker pull ${IMAGE_NAME} || true
	@docker build --cache-from ${IMAGE_NAME} -t ${IMAGE_NAME} .

proto-gen:
	@scripts/proto-gen.sh

lint:
	@docker run --rm -v ${PWD}:/app pipelinecomponents/flake8:latest flake8 --exclude ./thornode_proto

format:
	@docker run --rm -v ${PWD}:/app cytopia/black /app

test:
	@docker run ${DOCKER_OPTS} -e EXPORT=${EXPORT} -e EXPORT_EVENTS=${EXPORT_EVENTS} ${IMAGE_NAME} python -m unittest tests/test_*

test-coverage:
	@docker run ${DOCKER_OPTS} -e EXPORT=${EXPORT} -e EXPORT_EVENTS=${EXPORT_EVENTS} ${IMAGE_NAME} coverage run -m unittest tests/test_*

test-coverage-report:
	@docker run ${DOCKER_OPTS} -e EXPORT=${EXPORT} -e EXPORT_EVENTS=${EXPORT_EVENTS} ${IMAGE_NAME} coverage report -m

test-watch:
	@PYTHONPATH=${PWD} ptw tests/test_*

benchmark-provision:
	@docker run ${DOCKER_OPTS} ${IMAGE_NAME} python scripts/benchmark.py --tx-type=add --num=${NUM}

benchmark-swap:
	@docker run ${DOCKER_OPTS} ${IMAGE_NAME} python scripts/benchmark.py --tx-type=swap --num=${NUM}

smoke:
	@docker run ${DOCKER_OPTS} -e BLOCK_SCANNER_BACKOFF=${BLOCK_SCANNER_BACKOFF} ${IMAGE_NAME} python scripts/smoke.py --fast-fail=True

bootstrap:
	@docker run ${DOCKER_OPTS} -e BLOCK_SCANNER_BACKOFF=${BLOCK_SCANNER_BACKOFF} ${IMAGE_NAME} python scripts/smoke.py --bootstrap-only=True

kube-smoke:
	@kubectl replace --force -f kube/smoke.yml

kube-benchmark-provision:
	@sed -e 's|NUM|${NUM}|g' kube/benchmark-provision.yml | kubectl replace --force -f -

kube-benchmark-swap:
	@sed -e 's|NUM|${NUM}|g' kube/benchmark-swap.yml | kubectl replace --force -f -

health:
	@docker run ${DOCKER_OPTS} ${IMAGE_NAME} python scripts/health.py

health-mainnet:
	@docker run ${DOCKER_OPTS} ${IMAGE_NAME} python scripts/health.py --binance-api=https://dex.binance.org --thorchain=http://3.65.216.254:1317 --midgard=https://midgard.thorchain.info --margin-err=0.001

health-testnet:
	@docker run ${DOCKER_OPTS} ${IMAGE_NAME} python scripts/health.py --binance-api=https://testnet-dex.binance.org --thorchain=http://18.159.71.230:1317 --midgard=https://testnet.midgard.thorchain.info --margin-err=0

bitcoin-reorg:
	@docker run ${DOCKER_OPTS} ${IMAGE_NAME} python scripts/smoke.py --fast-fail=True --bitcoin-reorg=True

ethereum-reorg:
	@docker run ${DOCKER_OPTS} ${IMAGE_NAME} python scripts/smoke.py --fast-fail=True --ethereum-reorg=True

shell:
	@docker run ${DOCKER_OPTS} -it ${IMAGE_NAME} sh

.PHONY: build lint format test test-watch health smoke shell
