package terra

import (
	"fmt"

	ctypes "github.com/cosmos/cosmos-sdk/types"
	"gitlab.com/thorchain/thornode/common"
)

const NumDecimals = 6

func sdkCoinToCommonCoin(c ctypes.Coin) (common.Coin, error) {
	// c.Denom[1:] => Ignore the first character, "u", for most Cosmos assets
	name := fmt.Sprintf("%s.%s", common.TERRAChain.String(), c.Denom[1:])
	asset, err := common.NewAsset(name)
	if err != nil {
		return common.Coin{}, fmt.Errorf("failed to create asset (%s): %w", c.Denom, err)
	}

	coin := common.NewCoin(asset, ctypes.NewUintFromBigInt(c.Amount.BigInt()))
	coin.Decimals = int64(NumDecimals)

	return coin, nil
}
