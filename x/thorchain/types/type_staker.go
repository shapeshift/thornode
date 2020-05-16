package types

import (
	"errors"
	"fmt"

	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
)

type Staker struct {
	Asset             common.Asset   `json:"asset"`
	RuneAddress       common.Address `json:"rune_address"`
	AssetAddress      common.Address `json:"asset_address"`
	LastStakeHeight   int64          `json:"last_stake"`
	LastUnStakeHeight int64          `json:"last_unstake"`
	Units             cosmos.Uint    `json:"units"`
	PendingRune       cosmos.Uint    `json:"pending_rune"` // number of rune coins
}

func (staker Staker) IsValid() error {
	if staker.LastStakeHeight == 0 {
		return errors.New("last stake height cannot be empty")
	}
	if staker.RuneAddress.IsEmpty() {
		return errors.New("rune address cannot be empty")
	}
	if staker.AssetAddress.IsEmpty() {
		return errors.New("asset address cannot be empty")
	}
	return nil
}

func (staker Staker) Key() string {
	return fmt.Sprintf(
		"%s/%s",
		staker.Asset.String(),
		staker.RuneAddress.String(),
	)
}
