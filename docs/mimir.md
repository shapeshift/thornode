# Mimir Abilities

## Tx Out

`OutboundTransactionFee`: Amount of rune to withhold on all outbound transactions (1e8 notation)

### Scheduled Outbound

`MaxTxOutOffset`: Max number of blocks a scheduled outbound transaction can be delayed
`MinTxOutVolumeThreshold`: Quantity of outbound value (in 1e8 rune) in a block before its considered "full" and additional value is pushed into the next block
`TxOutDelayMax`: Maximum number of blocks a scheduled transaction can be delayed
`TxOutDelayRate`: Rate of which scheduled transactions are delayed

## Swapping

`HaltTrading`: Pause all trading
`Halt<chain>Trading`: Pause trading on a specific chain
`MaxSwapsPerBlock`: Artificial limit on the number of swaps that a single block with process
`MinSwapsPerBlock`: Process all swaps if the queue is equal to or smaller than this number

### Synths

`MaxSynthPerAssetDepth`: The amount of synths allowed per pool relative to the pool depth
`BurnSynths`: Enable/Disable burning synths
`MintSynths`: Enable/Disable minting synths
`VirtualMultSynths`: The amount of increase the pool depths for calculating swap fees of synths

## LP Management

`PauseLP`: Pauses the ability for LPs to add/remove liquidity
`PauseLP<chain>`: Pauses the ability for LPs to add/remove liquidity, per chain
`MaximumLiquidityRune`: Max rune capped on the pools

### Impermanet Loss Protection

`FullImpLossProtectionBlocks`: Number of blocks before an LP gets full imp loss protection
`ILP-DISABLED-<asset>`: Enable/Disable imp loss protection per asset

## Chain Management

`HaltChainGlobal`: Pause all chains (chain clients)
`Halt<chain>Chain`: Pause a specific blockchain via mimir or detected double-spend
`SolvencyHalt<chain>Chain`: Solvency checker auto halts chain. Chain will be auto un-halted once solvency is regained
`NodePauseChainGlobal`: Inidividual node controlled means to pause all chains
`NodePauseChainBlocks`: Number of block a node operator can pause/resume the chains for

### Solvency Checker

`StopSolvencyCheck`: Enable/Disable Solvency Checker
`StopSolvencyCheck<chain>`: Enable/Disable Solvency Checker, per chain
`PermittedSolvencyGap`: The amount of funds permitted to be "insolvent". This gives the network a little bit of "wiggle room" for margin of error

## Node Management

`MaximumBondInRune`: Sets an upper cap on how much a node can bond
`MinimumBondInRune`: Sets a lower bound on bond for a node to be considered to be churned in

### Yggdrasil Management

`YggFundLimit`: Funding limit for yggdrasil vaults (percentage)
`YggFundRetry`: Number of blocks to wait before attempting to fund a yggdrasil again
`StopFundYggdrasil`: Enable/Disable yggdrasil funding

## Churning

`AsgardSize`: Defines the number of members to an Asgard vault
`MinSlashPointsForBadValidator`: Min quantity of slash points needed to be considered "bad" and be marked for churn out
`BondLockupPeriod`: Lockout period that a node must wait before being allowed to unbond
`ChurnInterval`: Number of blocks between each churn
`HaltChurning`: Pause churning
`DesiredValidatorSet`: Max number of validators
`FundMigrationInterval`: Number of blocks between attempts to migrate funds between asgard vaults during a migration
`NumberOfNewNodesPerChurn`: Number of targeted additional nodes added to the validator set each churn
`MaxNodeToChurnOutForLowVersion`: Max number of validators to churn out for low version each churn

## Economics

`EmissionCurve`: How quickly rune is emitted from the reserve in block rewards
`IncentiveCurve`: The split between nodes and LPs while the balance is optimal
`MaxAvailablePools`: Maximum number of pools allowed on the network. Gas pools are excluded from this
`MinRunePoolDepth`: Minimum number of rune to be considered to become active
`PoolCycle`: Number of blocks the network will churn the pools (add/remove new available pools)
`StagedPoolCost`: Number of rune (1e8 notation) that a stage pool is deducted on each pool cycle.

## Miscellaneous

`DollarInRune`: Manual override of amount of rune in a dollar. Used for metrics data collection
`THORNames`: Enable/Disable THORNames
`TNSRegisterFee`: TNS registration fee of new names
`TNSFeePerBlock`: TNS cost per block to retain ownership of a name
`ArtificialRagnarokBlockHeight`: Triggers a chain shutodwn and ragnarok
`NativeTransactionFee`: The rune fee for a native transaction (gas cost in 1e8 notation)

### Router Upgrading (DO NOT TOUCH!)

`MimirRecallFund`: Recalls Chain funds, typically used for router upgrades only
`MimirUpgradeContract`: Upgrades contract, typically used for router upgrades only
