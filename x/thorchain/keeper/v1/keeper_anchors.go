package keeperv1

import (
	"fmt"
	"strings"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

func (k KVStore) GetAnchors(ctx cosmos.Context, asset common.Asset) []common.Asset {
	if asset.GetChain().IsTHORChain() {
		assets := make([]common.Asset, 0)
		pools, err := k.GetPools(ctx)
		if err != nil {
			ctx.Logger().Error("unable to fetch pools for anchor", "error", err)
			return assets
		}
		for _, pool := range pools {
			mimirKey := fmt.Sprintf("TorAnchor-%s", pool.Asset.String())
			mimirKey = strings.ReplaceAll(mimirKey, ".", "-")
			val, err := k.GetMimir(ctx, mimirKey)
			if err != nil {
				ctx.Logger().Error("unable to fetch pool for anchor", "mimir", mimirKey, "error", err)
				continue
			}
			if val > 0 {
				assets = append(assets, pool.Asset)
			}
		}
		return assets
	}
	return []common.Asset{asset.GetLayer1Asset()}
}

func (k KVStore) DollarInRune(ctx cosmos.Context) cosmos.Uint {
	// check for mimir override
	dollarInRune, err := k.GetMimir(ctx, "DollarInRune")
	if err == nil && dollarInRune > 0 {
		return cosmos.NewUint(uint64(dollarInRune))
	}

	usdAssets := k.GetAnchors(ctx, common.TOR)

	return k.AnchorMedian(ctx, usdAssets)
}

func (k KVStore) AnchorMedian(ctx cosmos.Context, assets []common.Asset) cosmos.Uint {
	p := make([]cosmos.Uint, 0)
	for _, asset := range assets {
		if k.IsGlobalTradingHalted(ctx) || k.IsChainTradingHalted(ctx, asset.Chain) {
			continue
		}
		pool, err := k.GetPool(ctx, asset)
		if err != nil {
			ctx.Logger().Error("fail to get usd pool", "asset", asset.String(), "error", err)
			continue
		}
		if !pool.IsAvailable() {
			continue
		}
		// value := common.GetUncappedShare(pool.BalanceAsset, pool.BalanceRune, cosmos.NewUint(common.One))
		value := pool.RuneValueInAsset(cosmos.NewUint(constants.DollarMulti * common.One))

		if !value.IsZero() {
			p = append(p, value)
		}
	}
	return common.GetMedianUint(p)
}
