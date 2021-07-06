package thorchain

import (
	"fmt"
	"strconv"

	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

type SwapMemo struct {
	MemoBase
	Destination          common.Address
	SlipLimit            cosmos.Uint
	AffiliateAddress     common.Address
	AffiliateBasisPoints cosmos.Uint
}

func (m SwapMemo) GetDestination() common.Address       { return m.Destination }
func (m SwapMemo) GetSlipLimit() cosmos.Uint            { return m.SlipLimit }
func (m SwapMemo) GetAffiliateAddress() common.Address  { return m.AffiliateAddress }
func (m SwapMemo) GetAffiliateBasisPoints() cosmos.Uint { return m.AffiliateBasisPoints }

func NewSwapMemo(asset common.Asset, dest common.Address, slip cosmos.Uint, affAddr common.Address, affPts cosmos.Uint) SwapMemo {
	return SwapMemo{
		MemoBase:             MemoBase{TxType: TxSwap, Asset: asset},
		Destination:          dest,
		SlipLimit:            slip,
		AffiliateAddress:     affAddr,
		AffiliateBasisPoints: affPts,
	}
}

func ParseSwapMemo(ctx cosmos.Context, keeper keeper.Keeper, asset common.Asset, parts []string) (SwapMemo, error) {
	var err error
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
	return NewSwapMemo(asset, destination, slip, affAddr, affPts), nil
}
