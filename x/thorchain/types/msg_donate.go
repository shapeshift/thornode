package types

import (
	"errors"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

// MsgDonate defines a donate message
type MsgDonate struct {
	Asset       common.Asset      `json:"asset"`     // asset of the asset
	AssetAmount cosmos.Uint       `json:"asset_amt"` // the amount of asset
	RuneAmount  cosmos.Uint       `json:"rune_amt"`  // the amount of rune
	Tx          common.Tx         `json:"tx"`
	Signer      cosmos.AccAddress `json:"signer"`
}

// NewMsgDonate is a constructor function for MsgDonate
func NewMsgDonate(tx common.Tx, asset common.Asset, r, amount cosmos.Uint, signer cosmos.AccAddress) MsgDonate {
	return MsgDonate{
		Asset:       asset,
		AssetAmount: amount,
		RuneAmount:  r,
		Tx:          tx,
		Signer:      signer,
	}
}

// Route should return the route key of the module
func (msg MsgDonate) Route() string { return RouterKey }

// Type should return the action
func (msg MsgDonate) Type() string { return "donate" }

// ValidateBasic runs stateless checks on the message
func (msg MsgDonate) ValidateBasic() error {
	if msg.Signer.Empty() {
		return cosmos.ErrInvalidAddress(msg.Signer.String())
	}
	if msg.Asset.IsEmpty() {
		return cosmos.ErrUnknownRequest("donate asset cannot be empty")
	}
	if msg.RuneAmount.IsZero() && msg.AssetAmount.IsZero() {
		return errors.New("rune and asset amount cannot be zero")
	}
	if err := msg.Tx.Valid(); err != nil {
		return cosmos.ErrUnknownRequest(err.Error())
	}
	return nil
}

// GetSignBytes encodes the message for signing
func (msg MsgDonate) GetSignBytes() []byte {
	return cosmos.MustSortJSON(ModuleCdc.MustMarshalJSON(msg))
}

// GetSigners defines whose signature is required
func (msg MsgDonate) GetSigners() []cosmos.AccAddress {
	return []cosmos.AccAddress{msg.Signer}
}
