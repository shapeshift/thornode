.PHONY: build test tools export healthcheck run-mocknet build-mocknet stop-mocknet ps-mocknet reset-mocknet logs-mocknet openapi

# compiler flags
NOW=$(shell date +'%Y-%m-%d_%T')
COMMIT:=$(shell git log -1 --format='%H')
VERSION:=$(shell cat version)
TAG?=testnet
ldflags = -X gitlab.com/thorchain/thornode/constants.Version=$(VERSION) \
		  -X gitlab.com/thorchain/thornode/constants.GitCommit=$(COMMIT) \
		  -X gitlab.com/thorchain/thornode/constants.BuildTime=${NOW} \
		  -X github.com/cosmos/cosmos-sdk/version.Name=THORChain \
		  -X github.com/cosmos/cosmos-sdk/version.AppName=thornode \
		  -X github.com/cosmos/cosmos-sdk/version.Version=$(VERSION) \
		  -X github.com/cosmos/cosmos-sdk/version.Commit=$(COMMIT) \
		  -X github.com/cosmos/cosmos-sdk/version.BuildTags=$(TAG)

# golang settings
TEST_DIR?="./..."
BUILD_FLAGS := -ldflags '$(ldflags)' -tags ${TAG}
TEST_BUILD_FLAGS := -parallel=1 -tags=mocknet
GOBIN?=${GOPATH}/bin
BINARIES=./cmd/thornode ./cmd/bifrost ./tools/generate

# image build settings
BRANCH?=$(shell git rev-parse --abbrev-ref HEAD | sed -e 's/master/mocknet/g')
GITREF=$(shell git rev-parse --short HEAD)
BUILDTAG?=$(shell git rev-parse --abbrev-ref HEAD | sed -e 's/master/mocknet/g;s/develop/mocknet/g;s/testnet-multichain/testnet/g')
ifdef CI_COMMIT_BRANCH # pull branch name from CI, if available
	BRANCH=$(shell echo ${CI_COMMIT_BRANCH} | sed 's/master/mocknet/g')
	BUILDTAG=$(shell echo ${CI_COMMIT_BRANCH} | sed -e 's/master/mocknet/g;s/develop/mocknet/g;s/testnet-multichain/testnet/g')
endif

all: lint install

# ------------------------------ Generate ------------------------------

protob:
	@./scripts/protocgen.sh

protob-docker:
	@docker run --rm -v $(shell pwd):/app -w /app \
		registry.gitlab.com/thorchain/thornode:builder-v1@sha256:6a7fb4e4ba636ca8ae6b7db93ae8838a8393ddbc8dfc2b99eb706fb18f50d635 \
		make protob

openapi:
	@docker run --rm \
		--user $(shell id -u):$(shell id -g) \
		-v $$PWD/openapi:/mnt \
		openapitools/openapi-generator-cli:v6.0.0@sha256:310bd0353c11863c0e51e5cb46035c9e0778d4b9c6fe6a7fc8307b3b41997a35 \
		generate -i /mnt/openapi.yaml -g go -o /mnt/gen
	@rm openapi/gen/go.mod openapi/gen/go.sum

# ------------------------------ Build ------------------------------

build: protob
	go build ${BUILD_FLAGS} ${BINARIES}

install: protob
	go install ${BUILD_FLAGS} ${BINARIES}

tools:
	go install -tags ${TAG} ./tools/generate
	go install -tags ${TAG} ./tools/pubkey2address

# ------------------------------ Housekeeping ------------------------------

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

# ------------------------------ Testing ------------------------------

test-coverage:
	@go test ${TEST_BUILD_FLAGS} -v -coverprofile=coverage.txt -covermode count ${TEST_DIR}
	sed -i '/\.pb\.go:/d' coverage.txt

coverage-report: test-coverage
	@go tool cover -html=coverage.txt

test-coverage-sum:
	@go run gotest.tools/gotestsum --junitfile report.xml --format testname -- ${TEST_BUILD_FLAGS} -v -coverprofile=coverage.txt -covermode count ${TEST_DIR}
	sed -i '/\.pb\.go:/d' coverage.txt
	@GOFLAGS='${TEST_BUILD_FLAGS}' go run github.com/boumenot/gocover-cobertura < coverage.txt > coverage.xml
	@go tool cover -func=coverage.txt
	@go tool cover -html=coverage.txt -o coverage.html

test:
	@CGO_ENABLED=0 go test ${TEST_BUILD_FLAGS} ${TEST_DIR}

test-race:
	@go test -race ${TEST_BUILD_FLAGS} ${TEST_DIR}

test-watch:
	@gow -c test ${TEST_BUILD_FLAGS} ${TEST_DIR}

test-sync-mainnet:
	@TAG=mainnet BRANCH=mainnet $(MAKE) docker-gitlab-build
	@docker run --rm -e CHAIN_ID=thorchain-mainnet-v1 -e NET=mainnet registry.gitlab.com/thorchain/thornode:mainnet

# ------------------------------ Docker Build ------------------------------

docker-gitlab-login:
	docker login -u ${CI_REGISTRY_USER} -p ${CI_REGISTRY_PASSWORD} ${CI_REGISTRY}

docker-gitlab-push:
	./build/docker/semver_tags.sh registry.gitlab.com/thorchain/thornode ${BRANCH} $(shell cat version) \
		| xargs -n1 | grep registry | xargs -n1 docker push
	docker push registry.gitlab.com/thorchain/thornode:${GITREF}

docker-gitlab-build:
	docker build -f build/docker/Dockerfile \
		$(shell sh ./build/docker/semver_tags.sh registry.gitlab.com/thorchain/thornode ${BRANCH} $(shell cat version)) \
		-t registry.gitlab.com/thorchain/thornode:${GITREF} --build-arg TAG=${BUILDTAG} .

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
