# THORNode Docker

## Fullnode

The default image will start a fullnode:

```bash
docker run -e NET=mainnet registry.gitlab.com/thorchain/thornode:chaosnet-multichain
```

Since this image tag contains the latest version of THORNode, the node can auto update by simply placing this in a loop to re-pull the image on exit:

```bash
while true; do
  docker pull registry.gitlab.com/thorchain/thornode:chaosnet-multichain
  docker run -e NET=mainnet registry.gitlab.com/thorchain/thornode:chaosnet-multichain
do
```

The above commands also apply to `testnet` and `stagenet` by simply using the respective image (in these cases `-e NET=...` is not required):

```code
testnet  => registry.gitlab.com/thorchain/thornode:testnet
stagenet => registry.gitlab.com/thorchain/thornode:stagenet
```

## Validator

Officially supported deployments of THORNode validators require a working understanding of Kubernetes and related infrastructure. See the [Cluster Launcher](https://gitlab.com/thorchain/devops/cluster-launcher) repo for cluster Terraform resources, and the [Node Launcher](https://gitlab.com/thorchain/devops/node-launcher) repo for deployment utilities which internally leveraging Helm.

## Mocknet

The development environment leverages Docker Compose V2 to create a mock network - this is included in the latest version of Docker Desktop for Mac and Windows, and can be added as a plugin on Linux by following the instructions [here](https://docs.docker.com/compose/cli-command/#installing-compose-v2).

The mocknet configuration is vanilla, leveraging Docker Compose profiles which can be combined at user discretion. The following profiles exist:

```code
thornode => thornode only
bifrost  => bifrost and thornode dependency
midgard  => midgard and thornode dependency
mocknet  => all mocknet dependencies
```

Example commands are provided below for those less familiar with Docker Compose features:

```bash
# start a mocknet with all dependencies
docker compose --profile mocknet up -d

# multiple profiles are supported, start a mocknet and midgard
docker compose --profile mocknet --profile midgard up -d

# check running services
docker compose ps

# tail the logs of all services
docker compose logs -f

# tail the logs of only thornode and bifrost
docker compose logs -f thornode bifrost

# enter a shell in the thornode container
docker compose exec thornode sh

# copy a file from the thornode container
docker compose cp thornode:/root/.thornode/config/genesis.json .

# rebuild all buildable services (thornode and bifrost)
docker compose build

# export thornode genesis
docker compose stop thornode
docker compose run thornode -- thornode export
docker compose start thornode

# hard fork thornode
docker compose stop thornode
docker compose run /docker/scripts/hard-fork.sh

# stop mocknet services
docker compose --profile mocknet down

# clear mocknet docker volumes
docker compose --profile mocknet down -v
```
