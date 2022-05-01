//go:build !testnet && !mocknet && !stagenet
// +build !testnet,!mocknet,!stagenet

package ethereum

import (
	"gitlab.com/thorchain/thornode/common"
)

var whitelistSmartContractAddress = []common.Address{
	// XRUNE
	common.Address(`0x69fa0feE221AD11012BAb0FdB45d444D3D2Ce71c`),
	// THORSwap Faucet
	common.Address(`0xB73B8E66196f2AF0762833304e3f15dB2e8Df0c3`),
	// TSAggregatorGeneric
	common.Address(`0xd31f7e39afECEc4855fecc51b693F9A0Cec49fd2`),
	// TSAggregatorUniswapV2
	common.Address(`0x7C38b8B2efF28511ECc14a621e263857Fb5771d3`),
	// TSAggregatorUniswapV3 500
	common.Address(`0x1C0Ee4030f771a1BB8f72C86150730d063f6b3ff`),
	// TSAggregatorUniswapV3 3000
	common.Address(`0x96ab925EFb957069507894CD941F40734f0288ad`),
	// TSAggregatorUniswapV3 10000
	common.Address(`0xE308B9562de7689B2d31C76a41649933F38ab761`),
	// TSAggregator2LegUniswapV2 USDC
	common.Address(`0x3660dE6C56cFD31998397652941ECe42118375DA`),
	// TSAggregator SUSHIswap
	common.Address(`0x0F2CD5dF82959e00BE7AfeeF8245900FC4414199`),
}
