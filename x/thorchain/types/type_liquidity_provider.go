package types

import (
	"errors"
	"fmt"

	"github.com/cosmos/cosmos-sdk/codec"

	"gitlab.com/thorchain/thornode/common"
)

var _ codec.ProtoMarshaler = &LiquidityProvider{}

// LiquidityProviders a list of liquidity providers
type LiquidityProviders []LiquidityProvider

// Valid check whether lp represent valid information
func (m *LiquidityProvider) Valid() error {
	if m.LastAddHeight == 0 {
		return errors.New("last add liquidity height cannot be empty")
	}
	if m.AssetAddress.IsEmpty() && m.RuneAddress.IsEmpty() {
		return errors.New("asset address and rune address cannot be empty")
	}
	return nil
}

func (lp LiquidityProvider) GetAddress() common.Address {
	if !lp.RuneAddress.IsEmpty() {
		return lp.RuneAddress
	}
	return lp.AssetAddress
}

// Key return a string which can be used to identify lp
func (lp LiquidityProvider) Key() string {
	return fmt.Sprintf("%s/%s", lp.Asset.String(), lp.GetAddress().String())
}
