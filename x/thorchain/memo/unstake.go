package thorchain

import (
	"fmt"

	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/x/thorchain/types"
)

type UnstakeMemo struct {
	MemoBase
	Amount          cosmos.Uint
	WithdrawalAsset common.Asset
}

func (m UnstakeMemo) GetAmount() cosmos.Uint           { return m.Amount }
func (m UnstakeMemo) GetWithdrawalAsset() common.Asset { return m.WithdrawalAsset }

func NewUnstakeMemo(asset common.Asset, amt cosmos.Uint, withdrawalAsset common.Asset) UnstakeMemo {
	return UnstakeMemo{
		MemoBase:        MemoBase{TxType: TxUnstake, Asset: asset},
		Amount:          amt,
		WithdrawalAsset: withdrawalAsset,
	}
}

func ParseUnstakeMemo(asset common.Asset, parts []string) (UnstakeMemo, error) {
	var err error
	if len(parts) < 2 {
		return UnstakeMemo{}, fmt.Errorf("not enough parameters")
	}
	withdrawalBasisPts := cosmos.ZeroUint()
	withdrawalAsset := common.EmptyAsset
	if len(parts) > 2 {
		withdrawalBasisPts, err = cosmos.ParseUint(parts[2])
		if err != nil {
			return UnstakeMemo{}, err
		}
		if withdrawalBasisPts.IsZero() || withdrawalBasisPts.GT(cosmos.NewUint(types.MaxUnstakeBasisPoints)) {
			return UnstakeMemo{}, fmt.Errorf("withdraw amount %s is invalid", parts[2])
		}
	}
	if len(parts) > 3 {
		withdrawalAsset, err = common.NewAsset(parts[3])
		if err != nil {
			return UnstakeMemo{}, err
		}
	}
	return NewUnstakeMemo(asset, withdrawalBasisPts, withdrawalAsset), nil
}
