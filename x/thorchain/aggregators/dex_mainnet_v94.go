//go:build !testnet && !mocknet
// +build !testnet,!mocknet

package aggregators

import (
	"github.com/blang/semver"

	"gitlab.com/thorchain/thornode/common"
)

func DexAggregators(version semver.Version) []Aggregator {
	return []Aggregator{
		// TSAggregatorGeneric
		{common.ETHChain, `0xd31f7e39afECEc4855fecc51b693F9A0Cec49fd2`},
		// TSAggregatorUniswapV2
		{common.ETHChain, `0x7C38b8B2efF28511ECc14a621e263857Fb5771d3`},
		// TSAggregatorUniswapV3 500
		{common.ETHChain, `0x0747c681e5ADa7936Ad915CcfF6cD3bd71DBF121`},
		// TSAggregatorUniswapV3 3000
		{common.ETHChain, `0xd1ea5F7cE9dA98D0bd7B1F4e3E05985E88b1EF10`},
		// TSAggregatorUniswapV3 10000
		{common.ETHChain, `0x94a852F0a21E473078846cf88382dd8d15bD1Dfb`},
		// TSAggregator2LegUniswapV2 USDC
		{common.ETHChain, `0x3660dE6C56cFD31998397652941ECe42118375DA`},
		// TSAggregator SUSHIswap
		{common.ETHChain, `0x0F2CD5dF82959e00BE7AfeeF8245900FC4414199`},
		// RangoThorchainOutputAggUniV2
		{common.ETHChain, `0x2a7813412b8da8d18Ce56FE763B9eb264D8e28a8`},
		// RangoThorchainOutputAggUniV3
		{common.ETHChain, `0xbB8De86F3b041B3C084431dcf3159fE4827c5F0D`},
	}
}
