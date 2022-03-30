package constants

// NewConstantValue010 get new instance of ConstantValue010
func NewConstantValue010() *ConstantVals {
	return &ConstantVals{
		int64values: map[ConstantName]int64{
			EmissionCurve:                 6,
			BlocksPerYear:                 5256000,
			IncentiveCurve:                100,                // configures incentive pendulum
			OutboundTransactionFee:        2_000000,           // A 0.02 Rune fee on all swaps and withdrawals
			NativeTransactionFee:          2_000000,           // A 0.02 Rune fee on all on chain txs
			PoolCycle:                     43200,              // Make a pool available every 3 days
			StagedPoolCost:                10_00000000,        // amount of rune to take from a staged pool on every pool cycle
			MinRunePoolDepth:              10000_00000000,     // minimum rune pool depth to be an available pool
			MaxAvailablePools:             100,                // maximum number of available pools
			MinimumNodesForYggdrasil:      6,                  // No yggdrasil pools if THORNode have less than 6 active nodes
			MinimumNodesForBFT:            4,                  // Minimum node count to keep network running. Below this, Ragnarök is performed.
			DesiredValidatorSet:           100,                // desire validator set
			AsgardSize:                    40,                 // desired node operators in an asgard vault
			FundMigrationInterval:         360,                // number of blocks THORNode will attempt to move funds from a retiring vault to an active one
			ChurnInterval:                 43200,              // How many blocks THORNode try to rotate validators
			ChurnRetryInterval:            720,                // How many blocks until we retry a churn (only if we haven't had a successful churn in ChurnInterval blocks
			BadValidatorRedline:           3,                  // redline multiplier to find a multitude of bad actors
			BadValidatorRate:              43200,              // rate to mark a validator to be rotated out for bad behavior
			OldValidatorRate:              43200,              // rate to mark a validator to be rotated out for age
			LowBondValidatorRate:          43200,              // rate to mark a validator to be rotated out for low bond
			LackOfObservationPenalty:      2,                  // add two slash point for each block where a node does not observe
			SigningTransactionPeriod:      300,                // how many blocks before a request to sign a tx by yggdrasil pool, is counted as delinquent.
			DoubleSignMaxAge:              24,                 // number of blocks to limit double signing a block
			MinimumBondInRune:             1_000_000_00000000, // 1 million rune
			MaxBondProviders:              6,                  // maximum number of bond providers
			MaxOutboundAttempts:           0,                  // maximum retries to reschedule a transaction
			PauseOnSlashThreshold:         100_00000000,       // number of rune to pause the network on the event a vault is slash for theft
			FailKeygenSlashPoints:         720,                // slash for 720 blocks , which equals 1 hour
			FailKeysignSlashPoints:        2,                  // slash for 2 blocks
			LiquidityLockUpBlocks:         0,                  // the number of blocks LP can withdraw after their liquidity
			ObserveSlashPoints:            1,                  // the number of slashpoints for making an observation (redeems later if observation reaches consensus
			ObservationDelayFlexibility:   10,                 // number of blocks of flexibility for a validator to get their slash points taken off for making an observation
			YggFundLimit:                  50,                 // percentage of the amount of funds a ygg vault is allowed to have.
			YggFundRetry:                  1000,               // number of blocks before retrying to fund a yggdrasil vault
			JailTimeKeygen:                720 * 6,            // blocks a node account is jailed for failing to keygen. DO NOT drop below tss timeout
			JailTimeKeysign:               60,                 // blocks a node account is jailed for failing to keysign. DO NOT drop below tss timeout
			NodePauseChainBlocks:          720,                // number of blocks that a node can pause/resume a global chain halt
			NodeOperatorFee:               500,                // Node operator fee
			MinSwapsPerBlock:              10,                 // process all swaps if queue is less than this number
			MaxSwapsPerBlock:              100,                // max swaps to process per block
			VirtualMultSynths:             2,                  // pool depth multiplier for synthetic swaps
			MaxSynthPerAssetDepth:         3300,               // percentage (in basis points) of how many synths are allowed relative to asset depth of the related pool
			MinSlashPointsForBadValidator: 100,                // The minimum slash point
			FullImpLossProtectionBlocks:   1440000,            // number of blocks before a liquidity provider gets 100% impermanent loss protection
			MinTxOutVolumeThreshold:       1000_00000000,      // total txout volume (in rune) a block needs to have to slow outbound transactions
			TxOutDelayRate:                25_00000000,        // outbound rune per block rate for scheduled transactions (excluding native assets)
			TxOutDelayMax:                 17280,              // max number of blocks a transaction can be delayed
			MaxTxOutOffset:                720,                // max blocks to offset a txout into a future block
			TNSRegisterFee:                10_00000000,        // registration fee for new THORName
			TNSFeeOnSale:                  1000,               // fee for TNS sale in basis points
			TNSFeePerBlock:                20,                 // per block cost for TNS, in rune
			PermittedSolvencyGap:          100,                // the setting is in basis points
			ValidatorMaxRewardRatio:       1,                  // the ratio to MinimumBondInRune at which validators stop receiving rewards proportional to their bond
			PoolDepthForYggFundingMin:     500_000_00000000,   // the minimum pool depth in RUNE required for ygg funding
		},
		boolValues: map[ConstantName]bool{
			StrictBondLiquidityRatio: true,
		},
		stringValues: map[ConstantName]string{
			DefaultPoolStatus: "Staged",
		},
	}
}
