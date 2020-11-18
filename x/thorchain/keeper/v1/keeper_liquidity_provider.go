package keeperv1

import (
	"fmt"
	"strings"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper/types"
)

// GetLiquidityProviderIterator iterate liquidity providers
func (k KVStore) GetLiquidityProviderIterator(ctx cosmos.Context, asset common.Asset) cosmos.Iterator {
	key := k.GetKey(ctx, prefixLiquidityProvider, LiquidityProvider{Asset: asset}.Key())
	return k.getIterator(ctx, types.DbPrefix(key))
}

func (k KVStore) GetTotalSupply(ctx cosmos.Context, asset common.Asset) cosmos.Uint {
	supplier := k.Supply().GetSupply(ctx)
	nativeDenom := asset.Native()
	for _, coin := range supplier.GetTotal() {
		if coin.Denom == nativeDenom {
			return cosmos.NewUint(coin.Amount.Uint64())
		}
	}
	return cosmos.ZeroUint()
}

// GetLiquidityProvider retrieve liquidity provider from the data store
func (k KVStore) GetLiquidityProvider(ctx cosmos.Context, asset common.Asset, addr common.Address) (LiquidityProvider, error) {
	record := LiquidityProvider{
		Asset:        asset,
		RuneAddress:  addr,
		Units:        cosmos.ZeroUint(),
		PendingRune:  cosmos.ZeroUint(),
		PendingAsset: cosmos.ZeroUint(),
	}
	if !addr.IsChain(common.RuneAsset().Chain) {
		record.AssetAddress = addr
		record.RuneAddress = common.NoAddress
	}

	_, err := k.get(ctx, k.GetKey(ctx, prefixLiquidityProvider, record.Key()), &record)
	if err != nil {
		return record, err
	}

	if addr.IsChain(common.THORChain) {
		accAddr, err := addr.AccAddress()
		if err != nil {
			return record, err
		}
		record.Units = k.GetLiquidityProviderBalance(ctx, asset.LiquidityAsset(), accAddr)
	}

	return record, nil
}

// SetLiquidityProvider save the liquidity provider to kv store
func (k KVStore) SetLiquidityProvider(ctx cosmos.Context, lp LiquidityProvider) {
	if !lp.RuneAddress.IsEmpty() && lp.RuneAddress.IsChain(common.THORChain) {
		lp.Units = cosmos.ZeroUint()
	}

	k.set(ctx, k.GetKey(ctx, prefixLiquidityProvider, lp.Key()), lp)
}

// RemoveLiquidityProvider remove the liquidity provider to kv store
func (k KVStore) RemoveLiquidityProvider(ctx cosmos.Context, lp LiquidityProvider) {
	k.del(ctx, k.GetKey(ctx, prefixLiquidityProvider, lp.Key()))
}

func (k KVStore) GetLiquidityProviderBalance(ctx cosmos.Context, asset common.Asset, addr cosmos.AccAddress) cosmos.Uint {
	bank := k.CoinKeeper()
	nativeDenom := strings.ToLower(asset.Symbol.String())
	for _, coin := range bank.GetCoins(ctx, addr) {
		if coin.Denom == nativeDenom {
			return cosmos.NewUint(coin.Amount.Uint64())
		}
	}
	return cosmos.ZeroUint()
}

func (k KVStore) AddOwnership(ctx cosmos.Context, coin common.Coin, addr cosmos.AccAddress) error {
	supplier := k.Supply()
	coinToMint, err := coin.Native()
	if err != nil {
		return fmt.Errorf("fail to parse coins: %w", err)
	}
	coinsToMint := cosmos.Coins{coinToMint}
	err = supplier.MintCoins(ctx, ModuleName, coinsToMint)
	if err != nil {
		return fmt.Errorf("fail to mint assets: %w", err)
	}
	if err := supplier.SendCoinsFromModuleToAccount(ctx, ModuleName, addr, coinsToMint); err != nil {
		return fmt.Errorf("fail to send newly minted token asset to node address(%s), %s: %w", addr, coinsToMint, err)
	}

	return nil
}

func (k KVStore) RemoveOwnership(ctx cosmos.Context, coin common.Coin, addr cosmos.AccAddress) error {
	supplier := k.Supply()
	coinToBurn, err := coin.Native()
	if err != nil {
		return fmt.Errorf("fail to parse coins: %w", err)
	}
	coinsToBurn := cosmos.Coins{coinToBurn}

	if err := supplier.SendCoinsFromAccountToModule(ctx, addr, ModuleName, coinsToBurn); err != nil {
		return fmt.Errorf("fail to remove burned asset from node address(%s), %s: %w", addr, coinsToBurn, err)
	}
	err = supplier.BurnCoins(ctx, ModuleName, coinsToBurn)
	if err != nil {
		return fmt.Errorf("fail to burn assets: %w", err)
	}

	return nil
}
