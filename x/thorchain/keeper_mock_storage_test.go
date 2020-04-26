package thorchain

import (
	"errors"

	"github.com/blang/semver"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/constants"

	"gitlab.com/thorchain/thornode/x/thorchain/types"
)

// MockPoolStorage implements PoolStorage interface, thus THORNode can mock the error cases
type MockPoolStorage struct {
	KVStoreDummy
}

func (mps MockPoolStorage) PoolExist(ctx sdk.Context, asset common.Asset) bool {
	if asset.Equals(common.Asset{Chain: common.BNBChain, Symbol: "NOTEXIST", Ticker: "NOTEXIST"}) {
		return false
	}
	return true
}

func (mps MockPoolStorage) GetPool(ctx sdk.Context, asset common.Asset) (types.Pool, error) {
	if asset.Equals(common.Asset{Chain: common.BNBChain, Symbol: "NOTEXIST", Ticker: "NOTEXIST"}) {
		return types.Pool{}, nil
	} else {
		return types.Pool{
			BalanceRune:  sdk.NewUint(100).MulUint64(common.One),
			BalanceAsset: sdk.NewUint(100).MulUint64(common.One),
			PoolUnits:    sdk.NewUint(100).MulUint64(common.One),
			Status:       types.Enabled,
			Asset:        asset,
		}, nil
	}
}

func (mps MockPoolStorage) SetPool(ctx sdk.Context, ps types.Pool) error { return nil }

func (mps MockPoolStorage) GetPoolStaker(ctx sdk.Context, asset common.Asset) (types.PoolStaker, error) {
	if asset.Equals(common.Asset{Chain: common.BNBChain, Symbol: "NOTEXISTSTICKER", Ticker: "NOTEXISTSTICKER"}) {
		return types.PoolStaker{}, errors.New("you asked for it")
	}
	return types.NewPoolStaker(asset, sdk.NewUint(100)), nil
}

func (mps MockPoolStorage) SetPoolStaker(ctx sdk.Context, ps types.PoolStaker) {}

func (mps MockPoolStorage) AddToLiquidityFees(ctx sdk.Context, asset common.Asset, fs sdk.Uint) error {
	return nil
}

func (mps MockPoolStorage) GetLowestActiveVersion(ctx sdk.Context) semver.Version {
	return constants.SWVersion
}

func (mps MockPoolStorage) AddFeeToReserve(ctx sdk.Context, fee sdk.Uint) error { return nil }
func (mps MockPoolStorage) UpsertEvent(ctx sdk.Context, event Event) error {
	return nil
}

func (mps MockPoolStorage) GetGas(ctx sdk.Context, _ common.Asset) ([]sdk.Uint, error) {
	return []sdk.Uint{sdk.NewUint(37500), sdk.NewUint(30000)}, nil
}
