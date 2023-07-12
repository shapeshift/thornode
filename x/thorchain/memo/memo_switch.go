package thorchain

import (
	"github.com/blang/semver"
	"gitlab.com/thorchain/thornode/common"
)

type SwitchMemo struct {
	MemoBase
	Destination common.Address
}

func (m SwitchMemo) GetDestination() common.Address {
	return m.Destination
}

func NewSwitchMemo(addr common.Address) SwitchMemo {
	return SwitchMemo{
		MemoBase:    MemoBase{TxType: TxSwitch},
		Destination: addr,
	}
}

func (p *parser) ParseSwitchMemo() (SwitchMemo, error) {
	switch {
	case p.version.GTE(semver.MustParse("1.116.0")):
		return p.ParseSwitchMemoV116()
	default:
		return ParseSwitchMemoV1(p.ctx, p.keeper, p.parts)
	}
}

func (p *parser) ParseSwitchMemoV116() (SwitchMemo, error) {
	destination := p.getAddressWithKeeper(1, true, common.NoAddress, common.THORChain)
	return NewSwitchMemo(destination), p.Error()
}
