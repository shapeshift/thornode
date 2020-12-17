package types

import (
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

// MsgSwap defines a MsgSwap message
type MsgSwap struct {
	Tx                   common.Tx         `json:"tx"`           // request tx
	TargetAsset          common.Asset      `json:"target_asset"` // target asset
	Destination          common.Address    `json:"destination"`  // destination , used for swap and send , the destination address THORNode send it to
	TradeTarget          cosmos.Uint       `json:"trade_target"`
	AffiliateAddress     common.Address    `json:"affiliate_address"`
	AffiliateBasisPoints cosmos.Uint       `json:"affiliate_basis_points"`
	Signer               cosmos.AccAddress `json:"signer"`
}

// NewMsgSwap is a constructor function for MsgSwap
func NewMsgSwap(tx common.Tx, target common.Asset, destination common.Address, tradeTarget cosmos.Uint, affAddr common.Address, affPts cosmos.Uint, signer cosmos.AccAddress) MsgSwap {
	return MsgSwap{
		Tx:                   tx,
		TargetAsset:          target,
		Destination:          destination,
		TradeTarget:          tradeTarget,
		AffiliateAddress:     affAddr,
		AffiliateBasisPoints: affPts,
		Signer:               signer,
	}
}

// Route should return the route key of the module
func (msg MsgSwap) Route() string { return RouterKey }

// Type should return the action
func (msg MsgSwap) Type() string { return "swap" }

// ValidateBasic runs stateless checks on the message
func (msg MsgSwap) ValidateBasic() error {
	if msg.Signer.Empty() {
		return cosmos.ErrInvalidAddress(msg.Signer.String())
	}
	if err := msg.Tx.Valid(); err != nil {
		return cosmos.ErrUnknownRequest(err.Error())
	}
	if msg.TargetAsset.IsEmpty() {
		return cosmos.ErrUnknownRequest("swap Target cannot be empty")
	}
	if len(msg.Tx.Coins) > 1 {
		return cosmos.ErrUnknownRequest("not expecting multiple coins in a swap")
	}
	if msg.Tx.Coins.IsEmpty() {
		return cosmos.ErrUnknownRequest("swap coin cannot be empty")
	}
	for _, coin := range msg.Tx.Coins {
		if coin.Asset.Equals(msg.TargetAsset) {
			return cosmos.ErrUnknownRequest("swap Source and Target cannot be the same.")
		}
	}
	if msg.Destination.IsEmpty() {
		return cosmos.ErrUnknownRequest("swap Destination cannot be empty")
	}
	if msg.AffiliateAddress.IsEmpty() && !msg.AffiliateBasisPoints.IsZero() {
		return cosmos.ErrUnknownRequest("swap affiliate address is empty while affiliate basis points is non-zero")
	}
	if !msg.Destination.IsChain(msg.TargetAsset.Chain) && !msg.Destination.IsChain(common.THORChain) {
		return cosmos.ErrUnknownRequest("swap destination address is not the same chain as the target asset")
	}
	if !msg.AffiliateAddress.IsEmpty() && !msg.AffiliateAddress.IsChain(common.THORChain) {
		return cosmos.ErrUnknownRequest("swap affiliate address must be a THOR address")
	}
	return nil
}

// GetSignBytes encodes the message for signing
func (msg MsgSwap) GetSignBytes() []byte {
	return cosmos.MustSortJSON(ModuleCdc.MustMarshalJSON(msg))
}

// GetSigners defines whose signature is required
func (msg MsgSwap) GetSigners() []cosmos.AccAddress {
	return []cosmos.AccAddress{msg.Signer}
}
