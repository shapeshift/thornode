package types

import cosmos "gitlab.com/thorchain/thornode/common/cosmos"

// MsgMimir defines a no op message
type MsgMimir struct {
	Key    string            `json:"key"`
	Value  int64             `json:"value"`
	Signer cosmos.AccAddress `json:"signer"`
}

// NewMsgMimir is a constructor function for MsgMimir
func NewMsgMimir(key string, value int64, signer cosmos.AccAddress) MsgMimir {
	return MsgMimir{
		Key:    key,
		Value:  value,
		Signer: signer,
	}
}

// Route should return the pooldata of the module
func (msg MsgMimir) Route() string { return RouterKey }

// Type should return the action
func (msg MsgMimir) Type() string { return "set_mimir_attr" }

// ValidateBasic runs stateless checks on the message
func (msg MsgMimir) ValidateBasic() cosmos.Error {
	if msg.Key == "" {
		return cosmos.ErrUnknownRequest("key cannot be empty")
	}
	if msg.Signer.Empty() {
		return cosmos.ErrInvalidAddress(msg.Signer.String())
	}
	return nil
}

// GetSignBytes encodes the message for signing
func (msg MsgMimir) GetSignBytes() []byte {
	return cosmos.MustSortJSON(ModuleCdc.MustMarshalJSON(msg))
}

// GetSigners defines whose signature is required
func (msg MsgMimir) GetSigners() []cosmos.AccAddress {
	return []cosmos.AccAddress{msg.Signer}
}
