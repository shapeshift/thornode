// +build mocknet

// For internal testing and mockneting
package constants

func init() {
	int64Overrides = map[ConstantName]int64{
		// ArtificialRagnarokBlockHeight: 200,
		DesiredValidatorSet:   12,
		ChurnInterval:         60, // 5 min
		ChurnRetryInterval:    30,
		BadValidatorRate:      60,          // 5 min
		OldValidatorRate:      60,          // 5 min
		MinimumBondInRune:     100_000_000, // 1 rune
		FundMigrationInterval: 60,
		LiquidityLockUpBlocks: 0,
		CliTxCost:             0,
		JailTimeKeygen:        10,
		JailTimeKeysign:       10,
		AsgardSize:            6,
	}
	boolOverrides = map[ConstantName]bool{
		StrictBondLiquidityRatio: false,
	}
	stringOverrides = map[ConstantName]string{
		DefaultPoolStatus: "Available",
	}
}
