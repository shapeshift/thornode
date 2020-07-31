package thorchain

import (
	"fmt"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

func migrateStoreV2(ctx cosmos.Context, keeper keeper.Keeper) error {
	pools, err := keeper.GetPools(ctx)
	if err != nil {
		return fmt.Errorf("fail to get pool:%w", err)
	}
	for _, p := range pools {
		if p.Asset.Symbol.IsMiniToken() {
			// remove pool
			keeper.RemovePool(ctx, p.Asset)
		}
	}

	// remove the coin from all our vault (asgard and yggdrasil)
	iter := keeper.GetVaultIterator(ctx)
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var vault Vault
		if err := keeper.Cdc().UnmarshalBinaryBare(iter.Value(), &vault); err != nil {
			ctx.Logger().Error("fail to unmarshal vault", "error", err)
		}

		if vault.Coins.IsEmpty() {
			continue
		}

		coinsToRemove := common.Coins{}
		for _, c := range vault.Coins {
			if c.Asset.Symbol.IsMiniToken() {
				coinsToRemove = append(coinsToRemove, c)
			}
		}
		if coinsToRemove.IsEmpty() {
			continue
		}

		vault.SubFunds(coinsToRemove)
		if err := keeper.SetVault(ctx, vault); err != nil {
			ctx.Logger().Error("fail to save vault", "error", err)
		}
	}

	return nil
}
