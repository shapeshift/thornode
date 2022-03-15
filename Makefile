include Makefile.cicd
.PHONY: build test tools export healthcheck run-mocknet build-mocknet stop-mocknet ps-mocknet reset-mocknet logs-mocknet

module = gitlab.com/thorchain/thornode
GOBIN?=${GOPATH}/bin
NOW=$(shell date +'%Y-%m-%d_%T')
COMMIT:=$(shell git log -1 --format='%H')
VERSION:=$(shell cat version)
TAG?=testnet
IMAGE?=registry.gitlab.com/thorchain/thornode
TEST_DIR?="./..."
# native coin denom string

ldflags = -X gitlab.com/thorchain/thornode/constants.Version=$(VERSION) \
		  -X gitlab.com/thorchain/thornode/constants.GitCommit=$(COMMIT) \
		  -X gitlab.com/thorchain/thornode/constants.BuildTime=${NOW} \
		  -X github.com/cosmos/cosmos-sdk/version.Name=THORChain \
		  -X github.com/cosmos/cosmos-sdk/version.AppName=thornode \
		  -X github.com/cosmos/cosmos-sdk/version.Version=$(VERSION) \
		  -X github.com/cosmos/cosmos-sdk/version.Commit=$(COMMIT) \
		  -X github.com/cosmos/cosmos-sdk/version.BuildTags=$(TAG)

BUILD_FLAGS := -ldflags '$(ldflags)' -tags ${TAG}
TEST_BUILD_FLAGS := -parallel 1 -tags mocknet

BINARIES=./cmd/thornode ./cmd/bifrost ./tools/generate

all: lint install

protob:
	@sh scripts/protocgen.sh

build: protob
	go build ${BUILD_FLAGS} ${BINARIES}

install: go.sum protob
	go install ${BUILD_FLAGS} ${BINARIES}

tools:
	go install -tags ${TAG} ./tools/generate
	go install -tags ${TAG} ./tools/extract
	go install -tags ${TAG} ./tools/pubkey2address

go.sum: go.mod
	@echo "--> Ensure dependencies have not been modified"
	go mod verify

test-coverage:
	@go test ${TEST_BUILD_FLAGS} -v -coverprofile coverage.out ${TEST_DIR}

coverage-report: test-coverage
	@go tool cover -html=cover.txt

clear:
	clear

test:
	@CGO_ENABLED=0 go test ${TEST_BUILD_FLAGS} ${TEST_DIR}

test-race:
	@go test -race ${TEST_BUILD_FLAGS} ${TEST_DIR}

test-watch: clear
	@gow -c test ${TEST_BUILD_FLAGS} ${TEST_DIR}

format:
	@git ls-files '*.go' | grep -v -e 'pb.go$$' -e '^docs/' | xargs gofumpt -w

lint-pre: protob
	@git ls-files '*.go' | grep -v -e 'pb.go$$' -e '^docs/' | xargs gofumpt -d
	@test -z "$(shell git ls-files '*.go' | grep -v -e 'pb.go$$' -e '^docs/' | xargs gofumpt -l)"
	@go mod verify

lint-handlers:
	@./scripts/lint-handlers.bash

lint-erc20s:
	@./scripts/lint-erc20s.bash

lint: lint-pre lint-handlers lint-erc20s
	@go run tools/analyze/main.go ./common/... ./constants/... ./x/...
ifdef CI_PROJECT_ID
	trunk check --no-progress --monitor=false --upstream origin/develop
else
	trunk check --upstream origin/develop
endif

clean:
	rm -rf ~/.thor*
	rm -f ${GOBIN}/{generate,thornode,bifrost}

.envrc: install
	@generate -t MASTER > .envrc
	@generate -t POOL >> .envrc

extract: tools
	@extract -f "${FILE}" -p "${PASSWORD}" -t ${TYPE}

# updates our tss dependency
tss:
	go get gitlab.com/thorchain/tss/go-tss

pull:
	docker pull ${IMAGE}:mocknet
	docker pull registry.gitlab.com/thorchain/midgard
	docker pull registry.gitlab.com/thorchain/bepswap/bepswap-web-ui
	docker pull registry.gitlab.com/thorchain/bepswap/mock-binance
	docker pull registry.gitlab.com/thorchain/ethereum-mock

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
