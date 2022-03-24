//go:build stagenet
// +build stagenet

package ethereum

import (
	"gitlab.com/thorchain/thornode/common"
)

var whitelistSmartContractAddres = []common.Address{
	// XRUNE
	common.Address(`0x69fa0feE221AD11012BAb0FdB45d444D3D2Ce71c`),
	// THORSwap Faucet
	common.Address(`0xB73B8E66196f2AF0762833304e3f15dB2e8Df0c3`),
	// THORSwap Wrapped Uniswap V2 Router
	common.Address(`0x1e181df53d07b698c6a58ca6308ab5d827f116e1`),
	// THORSwap Wrapped Uniswap V3 Router
	common.Address(`0x2f8aedd149afbdb5206ecaf8b1a3abb9186c8053`),
}
