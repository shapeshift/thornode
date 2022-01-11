//go:build mocknet
// +build mocknet

// For internal testing and mockneting
package constants

func init() {
	int64Overrides = map[ConstantName]int64{
		// ArtificialRagnarokBlockHeight: 200,
		DesiredValidatorSet:       12,
		ChurnInterval:             60, // 5 min
		ChurnRetryInterval:        30,
		BadValidatorRate:          60,          // 5 min
		OldValidatorRate:          60,          // 5 min
		MinimumBondInRune:         100_000_000, // 1 rune
		ValidatorMaxRewardRatio:   3,
		FundMigrationInterval:     40,
		LiquidityLockUpBlocks:     0,
		JailTimeKeygen:            10,
		JailTimeKeysign:           10,
		AsgardSize:                6,
		MinimumNodesForYggdrasil:  4,
		MinTxOutVolumeThreshold:   2000000_00000000,
		TxOutDelayRate:            2000000_00000000,
		PoolDepthForYggFundingMin: 500_000_00000000,
	}
	boolOverrides = map[ConstantName]bool{
		StrictBondLiquidityRatio: false,
	}
	stringOverrides = map[ConstantName]string{
		DefaultPoolStatus: "Available",
	}
}
