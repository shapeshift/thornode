//go:build testnet || mocknet
// +build testnet mocknet

package ethereum

import (
	"gitlab.com/thorchain/thornode/common"
)

var whitelistSmartContractAddress = []common.Address{
	// THORSwap Faucet
	common.Address(`0x83b0c5136790dDf6cA8D3fb3d220C757e0a91fBe`),
	// aggregator for uniswap v2
	common.Address(`0x942c6dA485FD6cEf255853ef83a149d43A73F18a`),
	// aggregator for uniswap v3
	common.Address(`0x7236D46c894Be8Af0C6b26Dd97608E396Db0f339`),
	// aggregator for sushiswap
	common.Address(`0x7fD9bd7A2Cab44820DD2874859E461640F04542D`),
	// generic
	common.Address(`0xDdf2498f9C57A8BB6dc9c75c64ef007E2592A81F`),
}
