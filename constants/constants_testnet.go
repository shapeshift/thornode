// +build testnet

// For Public TestNet
package constants

func init() {
	int64Overrides = map[ConstantName]int64{
		NewPoolCycle:         17280,
		DesireValidatorSet:   12,
		RotatePerBlockHeight: 17280,
		BadValidatorRate:     17280,
		OldValidatorRate:     17280,
		MinimumBondInRune:    100_000_000, // 1 rune
		StakeLockUpBlocks:    0,
		CliTxCost:            0,
	}
	boolOverrides = map[ConstantName]bool{
		StrictBondStakeRatio: false,
	}
}
