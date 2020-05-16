package types

import (
	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
)

// MsgSetStakeData defines a SetStakeData message
type MsgSetStakeData struct {
	Tx           common.Tx         `json:"tx"`
	Asset        common.Asset      `json:"asset"`         // ticker means the asset
	AssetAmount  cosmos.Uint       `json:"asset_amt"`     // the amount of asset stake
	RuneAmount   cosmos.Uint       `json:"rune"`          // the amount of rune stake
	RuneAddress  common.Address    `json:"rune_address"`  // staker's rune address
	AssetAddress common.Address    `json:"asset_address"` // staker's asset address
	Signer       cosmos.AccAddress `json:"signer"`
}

// NewMsgSetStakeData is a constructor function for MsgSetStakeData
func NewMsgSetStakeData(tx common.Tx, asset common.Asset, r, amount cosmos.Uint, runeAddr, assetAddr common.Address, signer cosmos.AccAddress) MsgSetStakeData {
	return MsgSetStakeData{
		Tx:           tx,
		Asset:        asset,
		AssetAmount:  amount,
		RuneAmount:   r,
		RuneAddress:  runeAddr,
		AssetAddress: assetAddr,
		Signer:       signer,
	}
}

// Route should return the pooldata of the module
func (msg MsgSetStakeData) Route() string { return RouterKey }

// Type should return the action
func (msg MsgSetStakeData) Type() string { return "set_stakedata" }

// ValidateBasic runs stateless checks on the message
func (msg MsgSetStakeData) ValidateBasic() cosmos.Error {
	if msg.Signer.Empty() {
		return cosmos.ErrInvalidAddress(msg.Signer.String())
	}
	if msg.Asset.IsEmpty() {
		return cosmos.ErrUnknownRequest("Stake asset cannot be empty")
	}
	if err := msg.Tx.IsValid(); err != nil {
		return cosmos.ErrUnknownRequest(err.Error())
	}
	if msg.RuneAddress.IsEmpty() {
		return cosmos.ErrUnknownRequest("rune address cannot be empty")
	}
	if !msg.Asset.Chain.IsBNB() {
		if msg.AssetAddress.IsEmpty() {
			return cosmos.ErrUnknownRequest("asset address cannot be empty")
		}
	}
	return nil
}

// GetSignBytes encodes the message for signing
func (msg MsgSetStakeData) GetSignBytes() []byte {
	return cosmos.MustSortJSON(ModuleCdc.MustMarshalJSON(msg))
}

// GetSigners defines whose signature is required
func (msg MsgSetStakeData) GetSigners() []cosmos.AccAddress {
	return []cosmos.AccAddress{msg.Signer}
}
