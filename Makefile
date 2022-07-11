include Makefile.cicd
.PHONY: build test tools export healthcheck run-mocknet build-mocknet stop-mocknet ps-mocknet reset-mocknet logs-mocknet openapi

module = gitlab.com/thorchain/thornode
GOBIN?=${GOPATH}/bin
NOW=$(shell date +'%Y-%m-%d_%T')
COMMIT:=$(shell git log -1 --format='%H')
VERSION:=$(shell cat version)
TAG?=testnet
TEST_DIR?="./..."

SMOKE_DOCKER_OPTS = --network=host --rm -e RUNE=THOR.RUNE -e LOGLEVEL=INFO -e PYTHONPATH=/app -v ${PWD}/test/smoke:/app -w /app

ldflags = -X gitlab.com/thorchain/thornode/constants.Version=$(VERSION) \
		  -X gitlab.com/thorchain/thornode/constants.GitCommit=$(COMMIT) \
		  -X gitlab.com/thorchain/thornode/constants.BuildTime=${NOW} \
		  -X github.com/cosmos/cosmos-sdk/version.Name=THORChain \
		  -X github.com/cosmos/cosmos-sdk/version.AppName=thornode \
		  -X github.com/cosmos/cosmos-sdk/version.Version=$(VERSION) \
		  -X github.com/cosmos/cosmos-sdk/version.Commit=$(COMMIT) \
		  -X github.com/cosmos/cosmos-sdk/version.BuildTags=$(TAG)

BUILD_FLAGS := -ldflags '$(ldflags)' -tags ${TAG}
TEST_BUILD_FLAGS := -parallel=1 -tags=mocknet

BINARIES=./cmd/thornode ./cmd/bifrost ./tools/generate

all: lint install

protob:
	@./scripts/protocgen.sh

protob-docker:
	@docker run --rm -v $(shell pwd):/app -w /app registry.gitlab.com/thorchain/thornode:builder-v1@sha256:6a7fb4e4ba636ca8ae6b7db93ae8838a8393ddbc8dfc2b99eb706fb18f50d635 make protob

openapi:
	@docker run --rm \
		--user $(shell id -u):$(shell id -g) \
		-v $$PWD/openapi:/mnt \
		openapitools/openapi-generator-cli:v6.0.0@sha256:310bd0353c11863c0e51e5cb46035c9e0778d4b9c6fe6a7fc8307b3b41997a35 \
		generate -i /mnt/openapi.yaml -g go -o /mnt/gen
	@rm openapi/gen/go.mod openapi/gen/go.sum

build: protob
	go build ${BUILD_FLAGS} ${BINARIES}

install: protob
	go install ${BUILD_FLAGS} ${BINARIES}

tools:
	go install -tags ${TAG} ./tools/generate
	go install -tags ${TAG} ./tools/pubkey2address

go.sum: go.mod
	@echo "--> Ensure dependencies have not been modified"
	go mod verify

test-coverage:
	@go test ${TEST_BUILD_FLAGS} -v -coverprofile=coverage.txt -covermode count ${TEST_DIR}
	sed -i '/\.pb\.go:/d' coverage.txt
	@docker run ${SMOKE_DOCKER_OPTS} ${IMAGE_NAME} coverage run -m unittest tests/test_*

coverage-report: test-coverage
	@go tool cover -html=coverage.txt
	@docker run ${DOCKER_OPTS} ${IMAGE_NAME} coverage report -m

test-coverage-sum:
	@go run gotest.tools/gotestsum --junitfile report.xml --format testname -- ${TEST_BUILD_FLAGS} -v -coverprofile=coverage.txt -covermode count ${TEST_DIR}
	sed -i '/\.pb\.go:/d' coverage.txt
	@GOFLAGS='${TEST_BUILD_FLAGS}' go run github.com/boumenot/gocover-cobertura < coverage.txt > coverage.xml
	@go tool cover -func=coverage.txt
	@go tool cover -html=coverage.txt -o coverage.html

test:
	@CGO_ENABLED=0 go test ${TEST_BUILD_FLAGS} ${TEST_DIR}
	@docker run ${SMOKE_DOCKER_OPTS} -e EXPORT=${EXPORT} -e EXPORT_EVENTS=${EXPORT_EVENTS} ${IMAGE_NAME} python -m unittest tests/test_*

test-race:
	@go test -race ${TEST_BUILD_FLAGS} ${TEST_DIR}

test-watch:
	@gow -c test ${TEST_BUILD_FLAGS} ${TEST_DIR}

format:
	@git ls-files '*.go' | grep -v -e '^docs/' | xargs gofumpt -w

lint:
	@./scripts/lint.sh
	@go run tools/analyze/main.go ./common/... ./constants/... ./x/...
ifdef CI_MERGE_REQUEST_TARGET_BRANCH_NAME
	./scripts/trunk check --no-progress --monitor=false --upstream origin/$(CI_MERGE_REQUEST_TARGET_BRANCH_NAME)
else
ifndef CI_PROJECT_ID
	./scripts/trunk check --no-fix --upstream origin/develop
endif
endif

clean:
	rm -rf ~/.thor*
	rm -f ${GOBIN}/{generate,thornode,bifrost}

# updates our tss dependency
tss:
	go get gitlab.com/thorchain/tss/go-tss

# ------------------------------ Smoke Tests ------------------------------

smoke: reset-mocknet
	@docker build -f test/smoke/Dockerfile -t registry.gitlab.com/thorchain/thornode:smoke test/smoke
	docker run ${SMOKE_DOCKER_OPTS} \
		-e BLOCK_SCANNER_BACKOFF=${BLOCK_SCANNER_BACKOFF} \
		registry.gitlab.com/thorchain/thornode:smoke \
		python scripts/smoke.py --fast-fail=True

# ------------------------------ Single Node Mocknet ------------------------------

cli-mocknet:
	@docker-compose -f build/docker/docker-compose.yml run --rm cli

run-mocknet:
	@docker-compose -f build/docker/docker-compose.yml --profile mocknet --profile midgard up -d

stop-mocknet:
	@docker-compose -f build/docker/docker-compose.yml --profile mocknet --profile midgard down -v

build-mocknet:
	@docker-compose -f build/docker/docker-compose.yml --profile mocknet --profile midgard build

ps-mocknet:
	@docker-compose -f build/docker/docker-compose.yml --profile mocknet --profile midgard images
	@docker-compose -f build/docker/docker-compose.yml --profile mocknet --profile midgard ps

logs-mocknet:
	@docker-compose -f build/docker/docker-compose.yml --profile mocknet logs -f thornode bifrost

reset-mocknet: stop-mocknet run-mocknet

# ------------------------------ Multi Node Mocknet ------------------------------

run-mocknet-cluster:
	@docker-compose -f build/docker/docker-compose.yml --profile mocknet-cluster --profile midgard up -d

stop-mocknet-cluster:
	@docker-compose -f build/docker/docker-compose.yml --profile mocknet-cluster --profile midgard down -v

build-mocknet-cluster:
	@docker-compose -f build/docker/docker-compose.yml --profile mocknet-cluster --profile midgard build

ps-mocknet-cluster:
	@docker-compose -f build/docker/docker-compose.yml --profile mocknet-cluster --profile midgard images
	@docker-compose -f build/docker/docker-compose.yml --profile mocknet-cluster --profile midgard ps

reset-mocknet-cluster: stop-mocknet-cluster build-mocknet-cluster run-mocknet-cluster
