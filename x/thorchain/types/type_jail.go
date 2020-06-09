package types

import (
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

type Jail struct {
	NodeAddress   cosmos.AccAddress `json:"node_address"`
	ReleaseHeight int64             `json:"release_height"`
	Reason        string            `json:"reason"`
}

func NewJail(addr cosmos.AccAddress) Jail {
	return Jail{
		NodeAddress: addr,
	}
}

func (j Jail) IsJailed(ctx cosmos.Context) bool {
	return j.ReleaseHeight > common.BlockHeight(ctx)
}
