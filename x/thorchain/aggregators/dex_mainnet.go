//go:build !testnet && !mocknet && !stagenet
// +build !testnet,!mocknet,!stagenet

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
		{common.ETHChain, `0x1C0Ee4030f771a1BB8f72C86150730d063f6b3ff`},
		// TSAggregatorUniswapV3 3000
		{common.ETHChain, `0x96ab925EFb957069507894CD941F40734f0288ad`},
		// TSAggregatorUniswapV3 10000
		{common.ETHChain, `0xE308B9562de7689B2d31C76a41649933F38ab761`},
		// TSAggregator2LegUniswapV2 USDC
		{common.ETHChain, `0x3660dE6C56cFD31998397652941ECe42118375DA`},
		// TSAggregator SUSHIswap
		{common.ETHChain, `0x0F2CD5dF82959e00BE7AfeeF8245900FC4414199`},
	}
}
