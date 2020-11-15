package types

import (
	"fmt"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

// MaxWithdrawBasisPoints basis points for withdrawals
const MaxWithdrawBasisPoints = 10_000

// MsgWithdrawLiquidity is used to withdraw
type MsgWithdrawLiquidity struct {
	Tx              common.Tx         `json:"tx"`
	RuneAddress     common.Address    `json:"rune_address"`     // it should be the rune address
	BasisPoints     cosmos.Uint       `json:"basis_points"`     // withdraw basis points
	Asset           common.Asset      `json:"asset"`            // asset asset asset
	WithdrawalAsset common.Asset      `json:"withdrawal_asset"` // asset to be withdrawn
	Signer          cosmos.AccAddress `json:"signer"`
}

// NewMsgWithdrawLiquidity is a constructor function for MsgWithdrawLiquidity
func NewMsgWithdrawLiquidity(tx common.Tx, runeAddress common.Address, withdrawBasisPoints cosmos.Uint, asset, withdrawalAsset common.Asset, signer cosmos.AccAddress) MsgWithdrawLiquidity {
	return MsgWithdrawLiquidity{
		Tx:              tx,
		RuneAddress:     runeAddress,
		BasisPoints:     withdrawBasisPoints,
		Asset:           asset,
		WithdrawalAsset: withdrawalAsset,
		Signer:          signer,
	}
}

// Route should return the route key of the module
func (msg MsgWithdrawLiquidity) Route() string { return RouterKey }

// Type should return the action
func (msg MsgWithdrawLiquidity) Type() string { return "withdraw" }

// ValidateBasic runs stateless checks on the message
func (msg MsgWithdrawLiquidity) ValidateBasic() error {
	if msg.Signer.Empty() {
		return cosmos.ErrInvalidAddress(msg.Signer.String())
	}
	if err := msg.Tx.Valid(); err != nil {
		return cosmos.ErrUnknownRequest(err.Error())
	}
	if msg.Asset.IsEmpty() {
		return cosmos.ErrUnknownRequest("pool asset cannot be empty")
	}
	if msg.RuneAddress.IsEmpty() {
		return cosmos.ErrUnknownRequest("address cannot be empty")
	}
	if !msg.RuneAddress.IsChain(common.RuneAsset().Chain) {
		return cosmos.ErrUnknownRequest(fmt.Sprintf("address must be a %s address", common.RuneAsset().Chain))
	}
	if msg.BasisPoints.IsZero() {
		return cosmos.ErrUnknownRequest("basis points can't be zero")
	}
	if msg.BasisPoints.GT(cosmos.NewUint(MaxWithdrawBasisPoints)) {
		return cosmos.ErrUnknownRequest("basis points is larger than maximum withdraw basis points")
	}
	if !msg.WithdrawalAsset.IsEmpty() && !msg.WithdrawalAsset.IsRune() && !msg.WithdrawalAsset.Equals(msg.Asset) {
		return cosmos.ErrUnknownRequest("withdrawal asset must be empty, rune, or pool asset")
	}
	return nil
}

// GetSignBytes encodes the message for signing
func (msg MsgWithdrawLiquidity) GetSignBytes() []byte {
	return cosmos.MustSortJSON(ModuleCdc.MustMarshalJSON(msg))
}

// GetSigners defines whose signature is required
func (msg MsgWithdrawLiquidity) GetSigners() []cosmos.AccAddress {
	return []cosmos.AccAddress{msg.Signer}
}
