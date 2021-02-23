// +build testnet

// For Public TestNet
package constants

func init() {
	int64Overrides = map[ConstantName]int64{
		PoolCycle:             1000,
		MinRunePoolDepth:      100_00000000,
		AsgardSize:            5,
		DesiredValidatorSet:   30,
		ChurnInterval:         240,
		BadValidatorRate:      17280,
		OldValidatorRate:      17280,
		MinimumBondInRune:     10000_00000000, // 1 rune
		LiquidityLockUpBlocks: 0,
		CliTxCost:             0,
		StagedPoolCost:        10_00000000,
	}
	boolOverrides = map[ConstantName]bool{
		StrictBondLiquidityRatio: false,
	}
}
