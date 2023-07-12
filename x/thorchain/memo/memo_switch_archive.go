package thorchain

import (
	"errors"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

func ParseSwitchMemoV1(ctx cosmos.Context, keeper keeper.Keeper, parts []string) (SwitchMemo, error) {
	if len(parts) < 2 {
		return SwitchMemo{}, errors.New("not enough parameters")
	}
	var destination common.Address
	var err error
	if keeper == nil {
		destination, err = common.NewAddress(parts[1])
	} else {
		destination, err = FetchAddress(ctx, keeper, parts[1], common.THORChain)
	}
	if err != nil {
		return SwitchMemo{}, err
	}
	if destination.IsEmpty() {
		return SwitchMemo{}, errors.New("address cannot be empty")
	}
	return NewSwitchMemo(destination), nil
}
