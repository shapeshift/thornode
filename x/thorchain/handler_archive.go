package thorchain

import (
	"strings"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

func fuzzyAssetMatchV1(ctx cosmos.Context, keeper keeper.Keeper, asset common.Asset) common.Asset {
	// if its already an exact match, return it immediately
	if keeper.PoolExist(ctx, asset) {
		return asset
	}

	matches := make(Pools, 0)

	iterator := keeper.GetPoolIterator(ctx)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		var pool Pool
		if err := keeper.Cdc().Unmarshal(iterator.Value(), &pool); err != nil {
			ctx.Logger().Error("fail to fetch pool", "asset", asset, "err", err)
			continue
		}

		// check chain match
		if !asset.Chain.Equals(pool.Asset.Chain) {
			continue
		}

		// check ticker match
		if !asset.Ticker.Equals(pool.Asset.Ticker) {
			continue
		}

		// check symbol
		parts := strings.Split(asset.Symbol.String(), "-")
		// check if no symbol given (ie "USDT" or "USDT-")
		if len(parts) < 2 || strings.EqualFold(parts[1], "") {
			matches = append(matches, pool)
			continue
		}

		if strings.HasSuffix(strings.ToLower(pool.Asset.Symbol.String()), strings.ToLower(parts[1])) {
			matches = append(matches, pool)
			continue
		}
	}

	// if we found no matches, return the argument given
	if len(matches) == 0 {
		return asset
	}

	// find the deepest pool
	winner := NewPool()
	for _, pool := range matches {
		if winner.BalanceRune.LT(pool.BalanceRune) {
			winner = pool
		}
	}

	return winner.Asset
}
