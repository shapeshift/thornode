# New Chain Integrations

## Process

Integrating a new chain into THORChain is inherently a risky process. THORChain inherits the risks (and value) from each chain it connects. Node operators take on risk and cost by pulling in a new chain daemon to run with their node. Therefore, THORChain should aim to add only those chains whose benifits cleary out-weigh the risks.

### Phase I: Public Proposal and Data Gathering

1. **Proposal of a New Chain:** New chain is proposed in #propose-a-chain, and a new channel created under “Community Chains” in Discord. This is an informal proposal, and should loosely follow the template under [Chain Proposal Template](#chain-proposal-template).

2. **Node Operator Discussion:**

   1. NO comments are gathered using make relay — we’ll make a dedicated push to gather NO comments in a comment window
   2. General public comments as usual in the various channels
   3. A Gitlab Issue should be made, and high-level pros/cons gathered there

3. **Node Mimir Vote:** Prompt Node Operators to vote on `Halt<Proposed-Chain>Chain=1` view Node Mimir. If a 50% consensus is reached then development of the chain client can be started.

### Phase II: Development, Testing, and Auditing

4. **Chain Client Development Period:** Community devs of the Proposed Chain build the Bifrost Chain Client, and open a PR to [`thornode`](https://gitlab.com/thorchain/thornode) (referencing the Gitlab issue created in the discussion phase), [`node-launcher`](https://gitlab.com/thorchain/node-launcher), and the [`smoke-test`](https://gitlab.com/thorchain/heimdall) repo.

   1. All PRs should meet the public requirements set forth in [High-Level Software Requirements](#high-level-software-requirements).

5. **Stagenet Merge/Baking Period:** Community devs are incentivized to test all necessary functionality as it relates to the new chain integration. Any chain on stagenet that is to be considered for Mainnet will have to go through a defined baking/hardening process set forth in [Stagenet Baking / Hardening Requirements](#stagenet-bakinghardening-requirements).

6. **Chain Client Audit:** An expert of the chain (that is not the author) must independently review the chain client and sign off on the safety and validity of its implementation, especially considering this [list](#chain-client-implementation-considerations). The final audit must be included in the chain client Pull Request under `bifrost/pkg/chainclients/<chain-name>`.

### Phase III: Mainnet Release

The following steps will be performed by the core team and Nine Realms for the final rollout of the chain.

7. **Admin Mimir:** Halt the new chain and disable trading until rollout is complete.

8. **Daemon Release and Sync:** Announcement will be made to NOs to `make install` in order to start the sync process for the new chain daemon.

9. **Enable Bifrost Scanning:** The final `node-launcher` PR will be merged, and NOs instructed to perform a final `make install` to enable Bifrost scanning.

10. **Admin Mimir:** Unhalt the chain to enable Bifrost scanning.

11. **Admin Mimir:** Enable trading once nodes have scanned to the tip on the new chain.

## Requirements and Guidelines

### Chain Proposal Template

```yaml
Chain Name:
Chain Type: EVM/UTXO/Cosmos/Other
Hardware Requirements: Memory and Storage
Year started:
Market Cap:
CoinMarketCap Rank:
24hr Volume:
Current DEX Integrations:
Other relevant dApps:
Number of previous hard forks:
```

### High-level Software Requirements

A new Chain Integration must include a pull request to [`thornode`](https://gitlab.com/thorchain/thornode) (referencing the Gitlab issue created in the discussion phase), [`node-launcher`](https://gitlab.com/thorchain/node-launcher), and the [`smoke-test`](https://gitlab.com/thorchain/heimdall) repo.

#### Thornode PR Requirements

1. Ensure a "mocknet" (local development network) service for the chain daemon is be added (`build/docker/docker-compose.yml`).
2. Ensure **70% or greater** unit test coverage.
3. Ensure a `<chain>_DISABLED` environment variable is respected in the Bifrost initialization script at `build/scripts/bifrost.sh`.
4. Lead a live walkthrough (PR author) with the core team, Nine Realms, and any other interested community members. During the walkthrough the author must be able to speak to the questions in [Chain Client Implementation Considerations](#chain-client-implementation-considerations).

#### Node Launcher PR Requirements

There should be 3 PRs in the node-launcher repo - the first to add the Docker image for the chain daemon, the second to add the service, the third to enable scanning in Bifrost. The first must be merged first so that hashes from the image builds may be pinned in the second.

1. **Image PR**
   1. Add a Dockerfile at `ci/images/<chain>/Dockerfile`.
   2. Ensure all source versions in the Dockerfile are pinned to a specific git hash.
2. **Services PR**
   1. Use an existing chain directory as a template for the new chain daemon configuration, reference the PR for the last added chain.
   2. Ensure the resource request sizes for the daemon are slightly over-provisioned (~20%) to the average expected utilization under normal operation.
   3. Extend the `get_node_service` function in `scripts/core.sh` with the service so that it is available for the standard make targets.
   4. Extend the `deploy_fullnode` function in `scripts/core.sh` with `--set <daemon-name>.enabled=false` in both the diff and install commands.
   5. Ensure the `<chain>_DISABLED` environment variable is used to disable the chain via a variable in `bifrost/values.yaml`.
3. **Enable PR**
   1. Update `bifrost/values.yaml` to enable the chain.

#### Hemidall PR Requirements

...

## Chain Client Implementation Considerations

1. Can an inbound transaction be "spoofed" - i.e. can the Chain Client be tricked into thinking value was transferred into THORChain, when it actually was not?
2. Does the chain client properly whitelist valid assets and reject invalid assets?
3. Does the chain client properly convert asset values to/from the 8 decimal point standard of thornode?
4. Is gas reporting deterministic? Every Bifrost must agree, or THORChain will not reach consensus.
5. Does the chain client properly report solvency of Asgard vaults?

## Stagenet Baking/Hardening Requirements

1. **Functionality to be tested:**
   - Swapping to/from the asset
   - Adding/withdrawing assets on the chain
   - Minting/burning synths
   - Registering a thorname for the chain
   - Vault funding
   - Vault churning
   - Inbound addresses returned correctly
   - Insolvency on the chain halts the chain
   - Unauthorised tx on the chain (double-spend) halts the chain
   - Chain client does not sign outbound when `HaltSigning<Chain>` is enabled
1. **Usage Requirements:**
   - 100 inbound transactions on stagenet
   - 100 outbound transactions on stagenet
   - 1000 RUNE of aggregate swap volume on stagenet
   - 100 RUNE of aggregate add liquidity transactions on stagenet
   - 100 RUNE of aggregate withdraw liquidity transactions on stagenet
1. **Permutation Brainstorm:**
   - There must be a brainstorming meeting where we try to think of various permutations of transactions to be tested (gas types, addr formats, return values, etc)
