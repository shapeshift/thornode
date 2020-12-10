# Help to add new chain client


## Run new chain daemon in the local development environment

This repository uses docker-compose to run a local development stack of THORChain.
While running in development mode, THORChain is run on `mocknet`.

To add a new chain, you will need to add a new chain daemon service using a docker image.
The docker-compose services are specified in the directory `build/docker/components/`.

For an example, you can take a look at Bitcoin Core chain daemon service added in `build/docker/components/bitcoin-regstest.yml`.
You can see here, the Bitcoin Core daemon is run on `regtest` mode, used here as a mock for bitcoin along side other mock services in THORChain `mocknet`.

You will need to run you new chain daemon as some kind of mock version of it so you can run tests easily against it.
If you chain does not offer a "mock" version of the daemon, you might have to write one yourself, like Binance-mock here https://gitlab.com/thorchain/bepswap/mock-binance.

Once you have a "mock" version of your chain you can run through a docker container,
you will nee dto create a new component in the directory `build/docker/components/` like `mynewchain-mock.yml`.

The project uses Makefiles to run the stack, to add the new service in the deployed local development stack,
you need to update the mocknet Makefile here `build/docker/mocknet/Makefile`, again a good example of what to add
exactly is taking a look at "bitcoin-regtest".


## Update Bifrost to connect to and observe the new chain daemon

### Bifrost Configuration

Bifrost is the service responsible to observe chains and relay information to THORCHain.
We need to add a new configuration within Bifrost to tell it to observe a new chain.

Bifrost configuration is of JSON format, here is an example:

{
    "thorchain": {
        "chain_id": "thorchain",
        "chain_host": "thor-api:1317",
        "chain_rpc": "thor-daemon:26657",
        "signer_name": "thorchain"
    },
    "metrics": {
        "enabled": true
    },
    "chains": [
      {
        "chain_id": "BNB",
        "rpc_host": "http://binance-daemon:26657",
        "block_scanner": {
          "rpc_host": "http://binance-daemon:26657",
          "enforce_block_height": false,
          "block_scan_processors": 1,
          "block_height_discover_back_off": "0.3s",
          "block_retry_interval": "10s",
          "chain_id": "BNB",
          "http_request_timeout": "30s",
          "http_request_read_timeout": "30s",
          "http_request_write_timeout": "30s",
          "max_http_request_retry": 10,
          "start_block_height": 0,
          "db_path": "/var/data/bifrost/observer/"
        }
      },
      {
        "chain_id": "BTC",
        "rpc_host": "bitcoin-daemon:18332",
        "username": "thorchain",
        "password": "password",
        "http_post_mode": 1,
        "disable_tls": 1,
        "block_scanner": {
          "rpc_host": "bitcoin-daemon:18332",
          "enforce_block_height": false,
          "block_scan_processors": 1,
          "block_height_discover_back_off": "5s",
          "block_retry_interval": "10s",
          "chain_id": "BTC",
          "http_request_timeout": "30s",
          "http_request_read_timeout": "30s",
          "http_request_write_timeout": "30s",
          "max_http_request_retry": 10,
          "start_block_height": 1895640,
          "db_path": "/var/data/bifrost/observer/"
        }
      }
    ],
    "tss": {
        "bootstrap_peers": [],
        "rendezvous": "asgard",
        "external_ip": "1.2.3.4",
        "p2p_port": 5040,
        "info_address": ":6040"
    },
    "signer": {
      "signer_db_path": "/var/data/bifrost/signer/",
      "block_scanner": {
        "rpc_host": "thor-daemon:26657",
        "start_block_height": 1,
        "enforce_block_height": false,
        "block_scan_processors": 1,
        "block_height_discover_back_off": "5s",
        "block_retry_interval": "10s",
        "start_block_height": 0,
        "db_path": "/var/data/bifrost/signer/",
        "scheme": "http"
      }
    }

The configuration that needs to be updated here is the `chains` one where we declare what chain bifrost will observe.

To add the new chain in Bifrost configuration, you need to update the file generating
Bifrost configuration here `build/scripts/bifrost.sh`.
Again a good example to follow is BTC or ETH.
Environment variables are used to pass down configuration to services.

### Bifrost environment variables

We need to then update in different places to pass down the chain host configuration.
For an example on what to change and where we can take a look at the environment variable BTC_HOST:

```
BTC_HOST: ${BTC_HOST:-bitcoin-regtest:18443}
```

Here is the list of files where you need to add the configuration like NEWCHAIN_HOST:

```
build/docker/components/standalone.Linux.yml
build/docker/components/standalone.base.yml
build/docker/components/validator.yml
build/docker/components/validator.Linux.yml
build/scripts/bifrost.sh
```
