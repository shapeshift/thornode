//go:build testnet || mocknet
// +build testnet mocknet

package ethereum

import (
	"gitlab.com/thorchain/thornode/common"
)

var (
	whitelistSmartContractAddres = []common.Address{
		// THORSwap Faucet
		common.Address(`0xDCA48722d7feb6a82b11a69c25b1037dE2e2e5C5`),
	}
)
