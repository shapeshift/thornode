package thorchain

import (
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
	addr := common.NoAddress
	var err error
	if len(parts) == 3 {
		addr, err = common.NewAddress(parts[2])
		if err != nil {
			return AddLiquidityMemo{}, err
		}
	}
	return NewAddLiquidityMemo(asset, addr), nil
}
