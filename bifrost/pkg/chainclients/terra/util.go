package terra

import (
	"fmt"
	"math/big"
	"strings"

	ctypes "github.com/cosmos/cosmos-sdk/types"
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

const ThorchainDecimals = 8

var WhitelistAssets = map[string]int{"uluna": 6, "uusd": 6}

func fromCosmosToThorchain(c cosmos.Coin) common.Coin {
	name := fmt.Sprintf("%s.%s", common.TERRAChain.String(), c.Denom[1:])
	asset, _ := common.NewAsset(name)

	decimals, exists := WhitelistAssets[c.Denom]
	if !exists {
		return common.NewCoin(asset, ctypes.Uint(c.Amount))
	}

	amount := c.Amount.BigInt()
	var exp big.Int
	// Decimals are more than native THORChain, so divide...
	if decimals > ThorchainDecimals {
		decimalDiff := int64(decimals - ThorchainDecimals)
		amount.Quo(amount, exp.Exp(big.NewInt(10), big.NewInt(int64(decimalDiff)), nil))
	} else if decimals < ThorchainDecimals {
		// Decimals are less than native THORChain, so multiply...
		decimalDiff := int64(ThorchainDecimals - decimals)
		amount.Mul(amount, exp.Exp(big.NewInt(10), big.NewInt(int64(decimalDiff)), nil))
	}
	return common.NewCoin(asset, ctypes.NewUintFromBigInt(amount))
}

func fromThorchainToCosmos(coin common.Coin) cosmos.Coin {
	denom := fmt.Sprintf("u%s", strings.ToLower(coin.Asset.Symbol.String()))
	decimals := WhitelistAssets[denom]

	amount := coin.Amount.BigInt()
	var exp big.Int
	if decimals > ThorchainDecimals {
		decimalDiff := int64(decimals - ThorchainDecimals)
		amount.Mul(amount, exp.Exp(big.NewInt(10), big.NewInt(int64(decimalDiff)), nil))
	} else if decimals < ThorchainDecimals {
		// Decimals are less than native THORChain, so multiply...
		decimalDiff := int64(ThorchainDecimals - decimals)
		amount.Quo(amount, exp.Exp(big.NewInt(10), big.NewInt(int64(decimalDiff)), nil))
	}
	return cosmos.NewCoin(denom, ctypes.NewIntFromBigInt(amount))
}
