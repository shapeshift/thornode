//go:build stagenet
// +build stagenet

// For internal testing and mockneting
package constants

func init() {
	int64Overrides = map[ConstantName]int64{
		ChurnInterval:               432000,
		MinRunePoolDepth:            1_00000000,
		MinimumBondInRune:           200_000_00000000,
		PoolCycle:                   720,
		EmissionCurve:               8,
		StopFundYggdrasil:           1,
		YggFundLimit:                0,
		NumberOfNewNodesPerChurn:    4,
		MintSynths:                  1,
		BurnSynths:                  1,
		KillSwitchStart:             1,
		KillSwitchDuration:          1,
		MaxRuneSupply:               500_000_000_00000000,
		FullImpLossProtectionBlocks: 0,
	}
}
