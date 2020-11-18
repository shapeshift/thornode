package types

import (
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

// MsgAddLiquidity defines a AddLiquidity message
type MsgAddLiquidity struct {
	Tx           common.Tx         `json:"tx"`
	Asset        common.Asset      `json:"asset"`         // ticker means the asset
	AssetAmount  cosmos.Uint       `json:"asset_amt"`     // the amount of asset add
	RuneAmount   cosmos.Uint       `json:"rune"`          // the amount of rune add
	RuneAddress  common.Address    `json:"rune_address"`  // liqudity provider rune address
	AssetAddress common.Address    `json:"asset_address"` // liqudity provider asset address
	Signer       cosmos.AccAddress `json:"signer"`
}

// NewMsgAddLiquidity is a constructor function for MsgAddLiquidity
func NewMsgAddLiquidity(tx common.Tx, asset common.Asset, r, amount cosmos.Uint, runeAddr, assetAddr common.Address, signer cosmos.AccAddress) MsgAddLiquidity {
	return MsgAddLiquidity{
		Tx:           tx,
		Asset:        asset,
		AssetAmount:  amount,
		RuneAmount:   r,
		RuneAddress:  runeAddr,
		AssetAddress: assetAddr,
		Signer:       signer,
	}
}

// Route should return the route key of the module
func (msg MsgAddLiquidity) Route() string { return RouterKey }

// Type should return the action
func (msg MsgAddLiquidity) Type() string { return "add_liquidity" }

// ValidateBasic runs stateless checks on the message
func (msg MsgAddLiquidity) ValidateBasic() error {
	if msg.Signer.Empty() {
		return cosmos.ErrInvalidAddress(msg.Signer.String())
	}
	if msg.Asset.IsEmpty() {
		return cosmos.ErrUnknownRequest("add liquidity asset cannot be empty")
	}
	if err := msg.Tx.Valid(); err != nil {
		return cosmos.ErrUnknownRequest(err.Error())
	}
	if msg.Asset.IsEmpty() {
		return cosmos.ErrUnknownRequest("unable to determine the intended pool for this add liquidity")
	}
	// There is no dedicate pool for RUNE ,because every pool will have RUNE , that's by design
	if msg.Asset.IsRune() {
		return cosmos.ErrUnknownRequest("invalid pool asset")
	}
	// test scenario we get two coins, but none are rune, invalid liquidity provider
	if len(msg.Tx.Coins) == 2 && (msg.AssetAmount.IsZero() || msg.RuneAmount.IsZero()) {
		return cosmos.ErrUnknownRequest("did not find both coins")
	}
	if len(msg.Tx.Coins) > 2 {
		return cosmos.ErrUnknownRequest("not expecting more than two coins in adding liquidity")
	}
	if msg.RuneAmount.IsZero() && msg.AssetAmount.IsZero() {
		return cosmos.ErrUnknownRequest("rune and asset amounts cannot both be empty")
	}
	if msg.RuneAddress.IsEmpty() && msg.AssetAddress.IsEmpty() {
		return cosmos.ErrUnknownRequest("rune address and asset address cannot be empty")
	}
	return nil
}

// GetSignBytes encodes the message for signing
func (msg MsgAddLiquidity) GetSignBytes() []byte {
	return cosmos.MustSortJSON(ModuleCdc.MustMarshalJSON(msg))
}

// GetSigners defines whose signature is required
func (msg MsgAddLiquidity) GetSigners() []cosmos.AccAddress {
	return []cosmos.AccAddress{msg.Signer}
}
