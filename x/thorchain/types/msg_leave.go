package types

import (
	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
)

// MsgLeave when an operator don't want to be a validator anymore
type MsgLeave struct {
	Tx     common.Tx         `json:"tx"`
	Signer cosmos.AccAddress `json:"signer"`
}

// NewMsgLeave create a new instance of MsgLeave
func NewMsgLeave(tx common.Tx, signer cosmos.AccAddress) MsgLeave {
	return MsgLeave{
		Tx:     tx,
		Signer: signer,
	}
}

// Route should return the router key of the module
func (msg MsgLeave) Route() string { return RouterKey }

// Type should return the action
func (msg MsgLeave) Type() string { return "validator_leave" }

// ValidateBasic runs stateless checks on the message
func (msg MsgLeave) ValidateBasic() cosmos.Error {
	if msg.Tx.FromAddress.IsEmpty() {
		return cosmos.ErrUnknownRequest("from address cannot be empty")
	}
	if msg.Tx.ID.IsEmpty() {
		return cosmos.ErrUnknownRequest("tx id hash cannot be empty")
	}
	if msg.Signer.Empty() {
		return cosmos.ErrUnknownRequest("signer cannot be empty ")
	}
	return nil
}

// GetSignBytes encodes the message for signing
func (msg MsgLeave) GetSignBytes() []byte {
	return cosmos.MustSortJSON(ModuleCdc.MustMarshalJSON(msg))
}

// GetSigners defines whose signature is required
func (msg MsgLeave) GetSigners() []cosmos.AccAddress {
	return []cosmos.AccAddress{msg.Signer}
}
