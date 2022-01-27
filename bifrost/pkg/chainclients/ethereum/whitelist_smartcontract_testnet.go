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
		// aggregator for uniswap v2
		common.Address(`0x1E181dF53d07B698C6a58Ca6308AB5D827F116e1`),
		// aggregator for uniswap v3
		common.Address(`0x2F8aEdd149AFbDb5206ECaF8b1a3abB9186C8053`),
	}
)
