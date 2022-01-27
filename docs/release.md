## Versioning
THORNode is following semantic version. MAJOR.MINOR.PATCH(0.77.1)

At the moment , Major is always 0, it might become 1 once mainnet reached

Minor version need to update when the network introduce some none backward compatible changes

Patch version , is backward compatible, usually changes only in bifrost

## Prepare for release ?
1) Create a milestone using the release version
2) Tag issues & PRs using the milestone, so we can identify which PR is on which version
3) PRs need to get approved by #thornode-team and #thorsec, once get approved , merge to `develop` branch
4) Once all PRs for a version have been merged , then create a tag from `develop` branch. for example: https://gitlab.com/thorchain/thornode/-/tags/v0.76.0

## Test release candidate
1) Create a 5 nodes mock net , follow [private_mock_chain.md](private_mock_chain.md)
2) Build image from `develop` branch, sanity check the following features work
- [ ] Genesis node start up successfully
- [ ] Bifrost startup correctly , and start to observe all chains 
- [ ] Create pools for BNB/BTC/BCH/LTC/ETH/USDT
- [ ] Add liquidity to BNB/BTC/BCH/LTC/ETH/USDT pools
- [ ] Switch BEP2 RUNE -> Native RUNE
- [ ] Bond new validator
- [ ] Set version
- [ ] Set node keys
- [ ] Set IP Address
- [ ] Churn successful , cluster grow from 1 genesis node to 4 nodes
- [ ] Fund migration successfully
- [ ] Fund yggdrasil successfully
- [ ] Some swaps , RUNE -> BTC , BTC -> BNB etc.
- [ ] Mocknet grow from four nodes -> five nodes , which include keygen , migration
- [ ] Node can leave
- [ ] Yggdrasil return fund successful after churn out

## Create testnet & chaosnet image
### testnet
1) Raise a PR to merge all changes from `develop` branch -> `testnet` branch , once the PR get approved & merged , testnet image should be created automatically by pipeline
   for example: https://gitlab.com/thorchain/thornode/-/pipelines/433627075
2) If you click "build-thornode" pipeline step , you should be able to see the docker image has been build and tagged successfully
```
Successfully built a15350678d3e
Successfully tagged registry.gitlab.com/thorchain/thornode:testnet
Successfully tagged registry.gitlab.com/thorchain/thornode:testnet-0
Successfully tagged registry.gitlab.com/thorchain/thornode:testnet-0.77
Successfully tagged registry.gitlab.com/thorchain/thornode:testnet-0.77.2
Successfully tagged registry.gitlab.com/thorchain/thornode:8be434a
```
### chaosnet
1) Raise a PR to merge all changes from `develop` branch -> `chaosnet-multichain` branch, once the PR get approved & merged, chaosnet image should be created automatically by pipeline
 for example: https://gitlab.com/thorchain/thornode/-/pipelines/433627314
2) Make sure "build-thornode" pipeline step is successful, when you click the step , inside you should be able to see 
```
Successfully built fdfd001f96ba
Successfully tagged registry.gitlab.com/thorchain/thornode:chaosnet-multichain
Successfully tagged registry.gitlab.com/thorchain/thornode:chaosnet-multichain-0
Successfully tagged registry.gitlab.com/thorchain/thornode:chaosnet-multichain-0.77
Successfully tagged registry.gitlab.com/thorchain/thornode:chaosnet-multichain-0.77.2
Successfully tagged registry.gitlab.com/thorchain/thornode:d24c9bd
```
## Release to testnet
node-launcher repository:  https://gitlab.com/thorchain/devops/node-launcher/
### Raise PR in node-launcher
1) Raise PR to release version to testnet , usually just change thornode-stack/testnet.yaml file , to use the new image tag
for example: https://gitlab.com/thorchain/devops/node-launcher/-/merge_requests/390
2) Post the PR to #devops channel , and tag @thornode-team @thorsec @Nine Realms team to approve , it will need at least 4 approvals
3) Start a new validator on testnet , using the new image tag, let it sync from genesis to the tip,  make sure thornode pod doesn't get consensus failure

## Release to chaosnet
### Raise PR in node-launcher
1) Raise PR to release version to testnet , usually just change thornode-stack/testnet.yaml file , to use the new image tag
   for example: https://gitlab.com/thorchain/devops/node-launcher/-/merge_requests/390
2) Post the PR to #devops channel , and tag @thornode-team @thorsec @Nine Realms team to approve , it will need at least 4 approvals
3) Start a new validator on chaosnet , using the new image tag , let it sync from genesis to the tip , make sure thornode pod doesn't get consensus failure.
4) step 3) takes a few days right now , If for some reason, 3) does get consensus failure , then the image can't be released , need to investigate and find out what cause the issue , and fix it

## Pre-release check
1) quickly go through all the PRs in the release.
2) apply the latest changes to a standby node , make sure it works fine

## Release
1) Post release announcement in #thornode-chaosnet or #thornode-testnet
2) For chaosnet release , post the release announcement in telegram #THORNode Announcement as well

