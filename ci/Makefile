
all: builder runner

builder:
	docker build -t registry.gitlab.com/thorchain/thornode:builder-v1 -f Dockerfile.builder .

runner:
	docker build -t registry.gitlab.com/thorchain/thornode:runner-base-v1 -f Dockerfile.runner .

push-builder:
	docker push registry.gitlab.com/thorchain/thornode:builder-v1

push-runner:
	docker push registry.gitlab.com/thorchain/thornode:runner-base-v1
