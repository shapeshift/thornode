package types

import (
	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
)

// Network keep track of reserve , reward and bond
type Network struct {
	BondRewardRune cosmos.Uint `json:"bond_reward_rune"` // The total amount of awarded rune for bonders
	TotalBondUnits cosmos.Uint `json:"total_bond_units"` // Total amount of bond units
	TotalReserve   cosmos.Uint `json:"total_reserve"`    // Total amount of reserves (in rune)
}

// NewNetwork create a new instance Network it is empty though
func NewNetwork() Network {
	return Network{
		BondRewardRune: cosmos.ZeroUint(),
		TotalBondUnits: cosmos.ZeroUint(),
		TotalReserve:   cosmos.ZeroUint(),
	}
}

// CalcNodeRewards calculate node rewards
func (v Network) CalcNodeRewards(nodeUnits cosmos.Uint) cosmos.Uint {
	return common.GetShare(nodeUnits, v.TotalBondUnits, v.BondRewardRune)
}
