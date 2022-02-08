//go:build testnet || mocknet
// +build testnet mocknet

package ethereum

import (
	"gitlab.com/thorchain/thornode/common"
)

var whitelistSmartContractAddres = []common.Address{
	// THORSwap Faucet
	common.Address(`0x83b0c5136790dDf6cA8D3fb3d220C757e0a91fBe`),
	// aggregator for uniswap v2
	common.Address(`0x69ba883Af416fF5501D54D5e27A1f497fBD97156`),
	// aggregator for uniswap v3
	common.Address(`0x3b7DbdD635B99cEa39D3d95Dbd0217F05e55B212`),
	// aggregator for sushiswap
	common.Address(`0x7fD9bd7A2Cab44820DD2874859E461640F04542D`),
}
