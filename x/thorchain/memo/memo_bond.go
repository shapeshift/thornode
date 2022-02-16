package thorchain

import (
	"fmt"

	cosmos "gitlab.com/thorchain/thornode/common/cosmos"

	"github.com/blang/semver"
)

type BondMemo struct {
	MemoBase
	NodeAddress         cosmos.AccAddress
	BondProviderAddress cosmos.AccAddress
}

func (m BondMemo) GetAccAddress() cosmos.AccAddress { return m.NodeAddress }

func NewBondMemo(addr, additional cosmos.AccAddress) BondMemo {
	return BondMemo{
		MemoBase:            MemoBase{TxType: TxBond},
		NodeAddress:         addr,
		BondProviderAddress: additional,
	}
}

func ParseBondMemo(version semver.Version, parts []string) (BondMemo, error) {
	if version.GTE(semver.MustParse("0.81.0")) {
		return ParseBondMemoV81(parts)
	}
	return ParseBondMemoV1(parts)
}

func ParseBondMemoV81(parts []string) (BondMemo, error) {
	additional := cosmos.AccAddress{}
	if len(parts) < 2 {
		return BondMemo{}, fmt.Errorf("not enough parameters")
	}
	addr, err := cosmos.AccAddressFromBech32(parts[1])
	if err != nil {
		return BondMemo{}, fmt.Errorf("%s is an invalid thorchain address: %w", parts[1], err)
	}
	if len(parts) >= 3 {
		additional, err = cosmos.AccAddressFromBech32(parts[2])
		if err != nil {
			return BondMemo{}, fmt.Errorf("%s is an invalid thorchain address: %w", parts[2], err)
		}
	}
	return NewBondMemo(addr, additional), nil
}
