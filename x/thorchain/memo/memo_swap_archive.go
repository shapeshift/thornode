package thorchain

import (
	"fmt"
	"strconv"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
	"gitlab.com/thorchain/thornode/x/thorchain/types"
)

func ParseSwapMemoV1(ctx cosmos.Context, keeper keeper.Keeper, asset common.Asset, parts []string) (SwapMemo, error) {
	var err error
	var order types.OrderType
	if len(parts) < 2 {
		return SwapMemo{}, fmt.Errorf("not enough parameters")
	}
	// DESTADDR can be empty , if it is empty , it will swap to the sender address
	destination := common.NoAddress
	affAddr := common.NoAddress
	affPts := cosmos.ZeroUint()
	if len(parts) > 2 {
		if len(parts[2]) > 0 {
			if keeper == nil {
				destination, err = common.NewAddress(parts[2])
			} else {
				destination, err = FetchAddress(ctx, keeper, parts[2], asset.Chain)
			}
			if err != nil {
				return SwapMemo{}, err
			}
		}
	}
	// price limit can be empty , when it is empty , there is no price protection
	slip := cosmos.ZeroUint()
	if len(parts) > 3 && len(parts[3]) > 0 {
		amount, err := cosmos.ParseUint(parts[3])
		if err != nil {
			return SwapMemo{}, fmt.Errorf("swap price limit:%s is invalid", parts[3])
		}
		slip = amount
	}

	if len(parts) > 5 && len(parts[4]) > 0 && len(parts[5]) > 0 {
		if keeper == nil {
			affAddr, err = common.NewAddress(parts[4])
		} else {
			affAddr, err = FetchAddress(ctx, keeper, parts[4], common.THORChain)
		}
		if err != nil {
			return SwapMemo{}, err
		}
		pts, err := strconv.ParseUint(parts[5], 10, 64)
		if err != nil {
			return SwapMemo{}, err
		}
		affPts = cosmos.NewUint(pts)
	}

	return NewSwapMemo(asset, destination, slip, affAddr, affPts, "", "", cosmos.ZeroUint(), order), nil
}
