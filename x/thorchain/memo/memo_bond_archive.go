package thorchain

import (
	"fmt"

	"gitlab.com/thorchain/thornode/common/cosmos"
)

func ParseBondMemoV1(parts []string) (BondMemo, error) {
	additional := cosmos.AccAddress{}
	if len(parts) < 2 {
		return BondMemo{}, fmt.Errorf("not enough parameters")
	}
	addr, err := cosmos.AccAddressFromBech32(parts[1])
	if err != nil {
		return BondMemo{}, fmt.Errorf("%s is an invalid thorchain address: %w", parts[1], err)
	}
	return NewBondMemo(addr, additional), nil
}
