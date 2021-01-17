package types

import (
	"errors"
	"fmt"
)

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

// Key return a string which can be used to identify lp
func (m *LiquidityProvider) Key() string {
	return fmt.Sprintf("%s/%s", m.Asset.String(), m.RuneAddress.String())
}
