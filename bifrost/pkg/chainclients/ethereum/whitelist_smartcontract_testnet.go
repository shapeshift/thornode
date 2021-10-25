//go:build testnet || mocknet
// +build testnet mocknet

package ethereum

import (
	"gitlab.com/thorchain/thornode/common"
)

var (
	whitelistSmartContractAddres = []common.Address{
		// THORSwap Faucet
		common.Address(`0x83b0c5136790dDf6cA8D3fb3d220C757e0a91fBe`),
	}
)
