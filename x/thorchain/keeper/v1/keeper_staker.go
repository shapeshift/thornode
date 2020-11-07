package keeperv1

import (
	"fmt"
	"strings"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper/types"
)

// GetStakerIterator iterate stakers
func (k KVStore) GetStakerIterator(ctx cosmos.Context, asset common.Asset) cosmos.Iterator {
	key := k.GetKey(ctx, prefixStaker, Staker{Asset: asset}.Key())
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

// GetStaker retrieve staker from the data store
func (k KVStore) GetStaker(ctx cosmos.Context, asset common.Asset, addr common.Address) (Staker, error) {
	record := Staker{
		Asset:       asset,
		RuneAddress: addr,
		Units:       cosmos.ZeroUint(),
		PendingRune: cosmos.ZeroUint(),
	}
	_, err := k.get(ctx, k.GetKey(ctx, prefixStaker, record.Key()), &record)
	if err != nil {
		return record, err
	}

	accAddr, err := addr.AccAddress()
	if err != nil {
		return record, err
	}
	record.Units = k.GetStakerBalance(ctx, asset.LiquidityAsset(), accAddr)

	return record, err
}

// SetStaker save the staker to kv store
func (k KVStore) SetStaker(ctx cosmos.Context, staker Staker) {
	k.set(ctx, k.GetKey(ctx, prefixStaker, staker.Key()), staker)
}

// RemoveStaker remove the staker to kv store
func (k KVStore) RemoveStaker(ctx cosmos.Context, staker Staker) {
	k.del(ctx, k.GetKey(ctx, prefixStaker, staker.Key()))
}

func (k KVStore) GetStakerBalance(ctx cosmos.Context, asset common.Asset, addr cosmos.AccAddress) cosmos.Uint {
	bank := k.CoinKeeper()
	nativeDenom := strings.ToLower(asset.Symbol.String())
	for _, coin := range bank.GetCoins(ctx, addr) {
		if coin.Denom == nativeDenom {
			return cosmos.NewUint(coin.Amount.Uint64())
		}
	}
	return cosmos.ZeroUint()
}

func (k KVStore) AddStake(ctx cosmos.Context, coin common.Coin, addr cosmos.AccAddress) error {
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

func (k KVStore) RemoveStake(ctx cosmos.Context, coin common.Coin, addr cosmos.AccAddress) error {
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
