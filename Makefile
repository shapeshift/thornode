include Makefile.cicd
.PHONY: build test tools export healthcheck

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
TEST_BUILD_FLAGS :=  -tags mocknet

BINARIES=./cmd/thornode ./cmd/bifrost ./tools/generate

# variables default for healthcheck against full stack in docker
CHAIN_API?=localhost:1317
MIDGARD_API?=localhost:8080

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
	@go test ${TEST_BUILD_FLAGS} ${TEST_DIR}

test-watch: clear
	@gow -c test ${TEST_BUILD_FLAGS} ${TEST_DIR}

format:
	@gofumpt -w .

lint-pre: protob
	@gofumpt -d cmd x bifrost common constants tools # for display
	@test -z "$(shell gofumpt -l cmd x bifrost common constants tools)" # cause error
	@go mod verify

lint-handlers:
	@./scripts/lint-handlers.bash

lint: lint-pre lint-managers
	@golangci-lint run --skip-files ".*\\.pb\\.go$$"

lint-verbose: lint-pre lint-managers
	golangci-lint run -v --skip-files ".*\\.pb\\.go$$"

start-daemon:
	thord start --log_level "main:info,state:debug,*:error"

start-rest:
	thorcli rest-server

clean:
	rm -rf ~/.thor*
	rm -f ${GOBIN}/{generate,thorcli,thord,bifrost}

.envrc: install
	@generate -t MASTER > .envrc
	@generate -t POOL >> .envrc

extract: tools
	@extract -f "${FILE}" -p "${PASSWORD}" -t ${TYPE}

# updates our tss dependency
tss:
	go get gitlab.com/thorchain/tss/go-tss

export:
	thord export

pull:
	docker pull ${IMAGE}:mocknet
	docker pull registry.gitlab.com/thorchain/midgard
	docker pull registry.gitlab.com/thorchain/bepswap/bepswap-web-ui
	docker pull registry.gitlab.com/thorchain/bepswap/mock-binance
	docker pull registry.gitlab.com/thorchain/ethereum-mock
