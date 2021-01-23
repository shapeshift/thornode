package thorchain

import (
	"strconv"

	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
)

type AddLiquidityMemo struct {
	MemoBase
	Address              common.Address
	AffiliateAddress     common.Address
	AffiliateBasisPoints cosmos.Uint
}

func (m AddLiquidityMemo) GetDestination() common.Address { return m.Address }

func NewAddLiquidityMemo(asset common.Asset, addr common.Address, affAddr common.Address, affPts cosmos.Uint) AddLiquidityMemo {
	return AddLiquidityMemo{
		MemoBase:             MemoBase{TxType: TxAdd, Asset: asset},
		Address:              addr,
		AffiliateAddress:     affAddr,
		AffiliateBasisPoints: affPts,
	}
}

func ParseAddLiquidityMemo(asset common.Asset, parts []string) (AddLiquidityMemo, error) {
	var err error
	addr := common.NoAddress
	affAddr := common.NoAddress
	affPts := cosmos.ZeroUint()
	if len(parts) >= 3 && len(parts[2]) > 0 {
		addr, err = common.NewAddress(parts[2])
		if err != nil {
			return AddLiquidityMemo{}, err
		}
	}

	if len(parts) >= 4 && len(parts[3]) > 0 && len(parts[4]) > 0 {
		affAddr, err = common.NewAddress(parts[3])
		if err != nil {
			return AddLiquidityMemo{}, err
		}
		pts, err := strconv.ParseUint(parts[4], 10, 64)
		if err != nil {
			return AddLiquidityMemo{}, err
		}
		affPts = cosmos.NewUint(pts)
	}
	return NewAddLiquidityMemo(asset, addr, affAddr, affPts), nil
}
