package thorchain

import (
	"fmt"

	"gitlab.com/thorchain/thornode/common"
)

type AddLiquidityMemo struct {
	MemoBase
	Address common.Address
}

func (m AddLiquidityMemo) GetDestination() common.Address { return m.Address }

func NewAddLiquidityMemo(asset common.Asset, addr common.Address) AddLiquidityMemo {
	return AddLiquidityMemo{
		MemoBase: MemoBase{TxType: TxAdd, Asset: asset},
		Address:  addr,
	}
}

func ParseAddLiquidityMemo(asset common.Asset, parts []string) (AddLiquidityMemo, error) {
	var addr common.Address
	var err error
	if !asset.Chain.Equals(common.RuneAsset().Chain) {
		if len(parts) < 3 {
			// cannot stake into a non THOR-based pool when THORNode don't have an
			// associated address
			return AddLiquidityMemo{}, fmt.Errorf("invalid stake. Cannot stake to a non THOR-based pool without providing an associated address")
		}
		addr, err = common.NewAddress(parts[2])
		if err != nil {
			return AddLiquidityMemo{}, err
		}
	}
	return NewAddLiquidityMemo(asset, addr), nil
}
