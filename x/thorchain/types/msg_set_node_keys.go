package types

import (
	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
)

// MsgSetNodeKeys defines a MsgSetNodeKeys message
type MsgSetNodeKeys struct {
	PubKeySetSet        common.PubKeySet  `json:"pub_key_set"`
	ValidatorConsPubKey string            `json:"validator_cons_pub_key"`
	Signer              cosmos.AccAddress `json:"signer"`
}

// NewMsgSetNodeKeys is a constructor function for NewMsgAddNodeKeys
func NewMsgSetNodeKeys(nodePubKeySet common.PubKeySet, validatorConsPubKey string, signer cosmos.AccAddress) MsgSetNodeKeys {
	return MsgSetNodeKeys{
		PubKeySetSet:        nodePubKeySet,
		ValidatorConsPubKey: validatorConsPubKey,
		Signer:              signer,
	}
}

// Route should return the cmname of the module
func (msg MsgSetNodeKeys) Route() string { return RouterKey }

// Type should return the action
func (msg MsgSetNodeKeys) Type() string { return "set_node_keys" }

// ValidateBasic runs stateless checks on the message
func (msg MsgSetNodeKeys) ValidateBasic() error {
	if msg.Signer.Empty() {
		return cosmos.ErrInvalidAddress(msg.Signer.String())
	}
	if len(msg.ValidatorConsPubKey) == 0 {
		return cosmos.ErrUnknownRequest("validator consensus pubkey cannot be empty")
	}
	if msg.PubKeySetSet.IsEmpty() {
		return cosmos.ErrUnknownRequest("node pub keys cannot be empty")
	}
	return nil
}

// GetSignBytes encodes the message for signing
func (msg MsgSetNodeKeys) GetSignBytes() []byte {
	return cosmos.MustSortJSON(ModuleCdc.MustMarshalJSON(msg))
}

// GetSigners defines whose signature is required
func (msg MsgSetNodeKeys) GetSigners() []cosmos.AccAddress {
	return []cosmos.AccAddress{msg.Signer}
}
