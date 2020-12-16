package types

import (
	"fmt"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

// MsgDeposit defines a MsgDeposit message
type MsgDeposit struct {
	Coins  common.Coins      `json:"coins"`
	Memo   string            `json:"memo"`
	Signer cosmos.AccAddress `json:"signer"`
}

// NewMsgDeposit is a constructor function for NewMsgDeposit
func NewMsgDeposit(coins common.Coins, memo string, signer cosmos.AccAddress) MsgDeposit {
	return MsgDeposit{
		Coins:  coins,
		Memo:   memo,
		Signer: signer,
	}
}

// Route should return the route key of the module
func (msg MsgDeposit) Route() string { return RouterKey }

// Type should return the action
func (msg MsgDeposit) Type() string { return "deposit" }

// ValidateBasic runs stateless checks on the message
func (msg MsgDeposit) ValidateBasic() error {
	if msg.Signer.Empty() {
		return cosmos.ErrInvalidAddress(msg.Signer.String())
	}
	if err := msg.Coins.Valid(); err != nil {
		return cosmos.ErrUnknownRequest(err.Error())
	}
	for _, coin := range msg.Coins {
		if !coin.IsNative() {
			return cosmos.ErrUnknownRequest("all coins must be native to THORChain")
		}
	}
	if len([]byte(msg.Memo)) > 150 {
		err := fmt.Errorf("memo must not exceed 150 bytes: %d", len([]byte(msg.Memo)))
		return cosmos.ErrUnknownRequest(err.Error())
	}
	return nil
}

// GetSignBytes encodes the message for signing
func (msg MsgDeposit) GetSignBytes() []byte {
	return cosmos.MustSortJSON(ModuleCdc.MustMarshalJSON(msg))
}

// GetSigners defines whose signature is required
func (msg MsgDeposit) GetSigners() []cosmos.AccAddress {
	return []cosmos.AccAddress{msg.Signer}
}
