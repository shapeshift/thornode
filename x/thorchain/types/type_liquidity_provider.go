package types

import (
	"errors"
	"fmt"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

// LiquidityProvider is a structure to store the information about a customer
// who provide liquidity in a pool
type LiquidityProvider struct {
	Asset              common.Asset   `json:"asset"`
	RuneAddress        common.Address `json:"rune_address"`
	AssetAddress       common.Address `json:"asset_address"`
	LastAddHeight      int64          `json:"last_add"`
	LastWithdrawHeight int64          `json:"last_withdraw"`
	Units              cosmos.Uint    `json:"units"`
	PendingRune        cosmos.Uint    `json:"pending_rune"`  // number of rune coins
	PendingAsset       cosmos.Uint    `json:"pending_asset"` // number of asset coins
	PendingTxID        common.TxID    `json:"pending_tx_id"`
}

type LiquidityProviders []LiquidityProvider

// Valid check whether lp represent valid information
func (lp LiquidityProvider) Valid() error {
	if lp.LastAddHeight == 0 {
		return errors.New("last add liquidity height cannot be empty")
	}
	if lp.AssetAddress.IsEmpty() && lp.RuneAddress.IsEmpty() {
		return errors.New("asset address and rune address cannot be empty")
	}
	return nil
}

// Key return a string which can be used to identify lp
func (lp LiquidityProvider) Key() string {
	return fmt.Sprintf("%s/%s", lp.Asset.String(), lp.RuneAddress.String())
}
