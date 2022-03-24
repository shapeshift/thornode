//go:build !testnet && !mocknet && !stagenet
// +build !testnet,!mocknet,!stagenet

package ethereum

import (
	"gitlab.com/thorchain/thornode/common"
)

var whitelistSmartContractAddres = []common.Address{
	// XRUNE
	common.Address(`0x69fa0feE221AD11012BAb0FdB45d444D3D2Ce71c`),
	// THORSwap Faucet
	common.Address(`0xB73B8E66196f2AF0762833304e3f15dB2e8Df0c3`),
}
