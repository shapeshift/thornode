package types

import (
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

var _ cosmos.Msg = &MsgAddLiquidity{}

// NewMsgAddLiquidity is a constructor function for MsgAddLiquidity
func NewMsgAddLiquidity(tx common.Tx, asset common.Asset, r, amount cosmos.Uint, runeAddr, assetAddr common.Address, signer cosmos.AccAddress) *MsgAddLiquidity {
	return &MsgAddLiquidity{
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
func (m *MsgAddLiquidity) Route() string { return RouterKey }

// Type should return the action
func (m MsgAddLiquidity) Type() string { return "add_liquidity" }

// ValidateBasic runs stateless checks on the message
func (m *MsgAddLiquidity) ValidateBasic() error {
	if m.Signer.Empty() {
		return cosmos.ErrInvalidAddress(m.Signer.String())
	}
	if m.Asset.IsEmpty() {
		return cosmos.ErrUnknownRequest("add liquidity asset cannot be empty")
	}
	if err := m.Tx.Valid(); err != nil {
		return cosmos.ErrUnknownRequest(err.Error())
	}
	if m.Asset.IsEmpty() {
		return cosmos.ErrUnknownRequest("unable to determine the intended pool for this add liquidity")
	}
	// There is no dedicate pool for RUNE ,because every pool will have RUNE , that's by design
	if m.Asset.IsRune() {
		return cosmos.ErrUnknownRequest("invalid pool asset")
	}
	// test scenario we get two coins, but none are rune, invalid liquidity provider
	if len(m.Tx.Coins) == 2 && (m.AssetAmount.IsZero() || m.RuneAmount.IsZero()) {
		return cosmos.ErrUnknownRequest("did not find both coins")
	}
	if len(m.Tx.Coins) > 2 {
		return cosmos.ErrUnknownRequest("not expecting more than two coins in adding liquidity")
	}
	if m.RuneAmount.IsZero() && m.AssetAmount.IsZero() {
		return cosmos.ErrUnknownRequest("rune and asset amounts cannot both be empty")
	}
	if m.RuneAddress.IsEmpty() && m.AssetAddress.IsEmpty() {
		return cosmos.ErrUnknownRequest("rune address and asset address cannot be empty")
	}
	return nil
}

// GetSignBytes encodes the message for signing
func (m *MsgAddLiquidity) GetSignBytes() []byte {
	return cosmos.MustSortJSON(ModuleCdc.MustMarshalJSON(m))
}

// GetSigners defines whose signature is required
func (m *MsgAddLiquidity) GetSigners() []cosmos.AccAddress {
	return []cosmos.AccAddress{m.Signer}
}
