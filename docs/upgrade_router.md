<!-- markdownlint-disable MD024 -->

# What is router

On EVM based chain , bifrost rely on Router to emit correct events to determinate what had happened , all inbound/outbound transactions go through a smart contract, we call it Router. Current Router is on V4.
So far we only have Router on ETH chain , but it might deploy to other chains in the future , for simplicity , Let's assume it is ETH chain.

Router contract hold all ERC20 assets , but not ETH. ETH will be sent to asgard address directly.

# Where is Router code?

https://gitlab.com/thorchain/ethereum/eth-router , if you need to make changes to this router , please raise PR in [this repository](https://gitlab.com/thorchain/ethereum/eth-router)

# How to upgrade Router?

**Note:** Newer version router need to be compatible with old router.

## What you can do?

- You can add new functions, new events

## What you can't do?

- Don't change existing function signature , Don't add parameter , don't remove parameter , don't change return value etc.
- Don't change events , don't add new fields , don't remove fields

## Router upgrade procedure

Before router upgrade , make sure you already make relevant changes in thornode repo.

- New router has been deployed , and the router address has been updated. `ethOldRouter` is your current router address, `ethNewRouter` is your new router address
  - [Mocknet](https://gitlab.com/thorchain/thornode/-/blob/develop/x/thorchain/router_upgrade_info_mocknet.go)
  - [Testnet](https://gitlab.com/thorchain/thornode/-/blob/develop/x/thorchain/router_upgrade_info_testnet.go)
  - [Stagenet](https://gitlab.com/thorchain/thornode/-/blob/develop/x/thorchain/router_upgrade_info_stagenet.go)
  - [Chaosnet](https://gitlab.com/thorchain/thornode/-/blob/develop/x/thorchain/router_upgrade_info.go)

Before upgrade , make sure the network is healthy , all active nodes / standby nodes are online. If some nodes are not healthy , bifrost are not online it will cause the node's vault in a bad state

## Detail upgrade procedure

1. Set admin mimir `ChurnInterval` -> `432000` to stop churn
2. Set admin mimir `StopFundYggdrasil` -> `1` , to stop funding yggdrasil
3. Set admin mimir `StopSolvencyCheckETH` -> `1` to stop Solvency checker on ETH chain , this will make sure the migration fund will not cause solvency checker to halt the chain
4. Set admin mimir `MimirRecallFund` -> `1` , this will trigger the network to recall all yggdrasil fund on ETH chain
5. Wait for a few minutes , until all yggdrasil vault return all the ERC20 tokens , Confirm that all yggdrasil vaults have return funds back by checking `/thorchain/vaults/yggdrasil` endpoint , make sure ERC20 token balance of all yggdrasil vaults are zero.
6. Set admin mimir `MimirUpgradeContract` -> `1` to update the router on yggdrasil vault , make sure router contract has been updated on all yggdrasil vaults
7. Set admin mimir `ChurnInterval` -> `43200`
8. Wait a churn to kick off , and make sure funds have been migrated from older router to new router. And vault retired successfully
9. Set admin mimir `StopFundYggdrasil` -> `0` to start fund yggdrasil vaults again
10. Set admin mimir `StopSolvencyCheckETH` -> `0` to resume solvency checker on ETH chain
