//go:build !testnet && !mocknet
// +build !testnet,!mocknet

package ethereum

import (
	"gitlab.com/thorchain/thornode/common"
)

var (
	whitelistSmartContractAddres = []common.Address{
		// XRUNE
		common.Address(`0x69fa0feE221AD11012BAb0FdB45d444D3D2Ce71c`),
		// THORSwap Faucet
		common.Address(`0x3F02745BADeAe8738104931cfD864d33FDb52310`),
	}
)
