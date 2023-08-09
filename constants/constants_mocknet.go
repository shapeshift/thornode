//go:build mocknet
// +build mocknet

// For internal testing and mockneting
package constants

import (
	"os"
	"regexp"
	"strconv"
	"strings"
)

func camelToSnakeUpper(s string) string {
	re := regexp.MustCompile(`([a-z])([A-Z])`)
	snake := re.ReplaceAllString(s, `${1}_${2}`)
	return strings.ToUpper(snake)
}

func init() {
	int64Overrides = map[ConstantName]int64{
		// ArtificialRagnarokBlockHeight: 200,
		DesiredValidatorSet:                 12,
		ChurnInterval:                       60, // 5 min
		ChurnRetryInterval:                  30,
		MinimumBondInRune:                   100_000_000, // 1 rune
		ValidatorMaxRewardRatio:             3,
		FundMigrationInterval:               40,
		LiquidityLockUpBlocks:               0,
		MaxRuneSupply:                       500_000_000_00000000,
		JailTimeKeygen:                      10,
		JailTimeKeysign:                     10,
		AsgardSize:                          6,
		StreamingSwapMinBPFee:               100,
		MinimumNodesForYggdrasil:            4,
		VirtualMultSynthsBasisPoints:        20_000,
		MinTxOutVolumeThreshold:             2000000_00000000,
		TxOutDelayRate:                      2000000_00000000,
		PoolDepthForYggFundingMin:           500_000_00000000,
		MaxSynthPerPoolDepth:                3_500,
		MaxSynthsForSaversYield:             5000,
		PauseLoans:                          0,
		AllowWideBlame:                      1,
		TargetOutboundFeeSurplusRune:        10_000_00000000,
		MaxOutboundFeeMultiplierBasisPoints: 20_000,
		MinOutboundFeeMultiplierBasisPoints: 15_000,
		PreferredAssetOutboundFeeMultiplier: 100,
	}
	boolOverrides = map[ConstantName]bool{
		StrictBondLiquidityRatio: false,
	}
	stringOverrides = map[ConstantName]string{
		DefaultPoolStatus: "Available",
	}

	// allow overrides from environment variables in mocknet
	for k := range int64Overrides {
		env := camelToSnakeUpper(k.String())
		if os.Getenv(env) != "" {
			int64Overrides[k], _ = strconv.ParseInt(os.Getenv(env), 10, 64)
		}
	}
	for k := range boolOverrides {
		env := camelToSnakeUpper(k.String())
		if os.Getenv(env) != "" {
			boolOverrides[k], _ = strconv.ParseBool(os.Getenv(env))
		}
	}
	for k := range stringOverrides {
		env := camelToSnakeUpper(k.String())
		if os.Getenv(env) != "" {
			stringOverrides[k] = os.Getenv(env)
		}
	}
}
