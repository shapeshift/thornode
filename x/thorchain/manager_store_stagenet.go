//go:build stagenet
// +build stagenet

package thorchain

import (
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

func creditAssetBackToVaultAndPool(ctx cosmos.Context, mgr Manager) {
}

func purgeETHOutboundQueue(ctx cosmos.Context, mgr Manager) {
}

func correctAsgardVaultBalanceV61(ctx cosmos.Context, mgr Manager, asgardPubKey common.PubKey) {
}

func migrateStoreV80(ctx cosmos.Context, mgr Manager) {
	// Set decimals of asgard vaults
	vaults, err := mgr.Keeper().GetAsgardVaults(ctx)
	if err != nil {
		ctx.Logger().Error("unable to get vaults: %w", err)
	}

	luna, _ := common.NewAsset("TERRA.LUNA")
	ust, _ := common.NewAsset("TERRA.USD")

	for _, vault := range vaults {
		var vaultCoins common.Coins
		for _, c := range vault.Coins {
			if c.Asset.Equals(luna) || c.Asset.Equals(ust) {
				c.Decimals = 6
			}
			vaultCoins = append(vaultCoins, c)
		}
		vault.Coins = vaultCoins
		err := mgr.Keeper().SetVault(ctx, vault)
		if err != nil {
			ctx.Logger().Error("unable to set vaults: %w", err)
		}
	}
}

func migrateStoreV81(ctx cosmos.Context, mgr Manager) {
	removeTransactions(ctx, mgr,
		"9C255224D38282D2A2DC20B1B29D642CFA8E1D5180840F17A31279B5EE1527DA",
		"CD684106D927B6B18C02BD390723C71EF2C625C050A7C5A264931DBB480DF0B1",
		"9F0EA38348F4E5DEFDAC0A72E5A1D343E782329C76E4DABB36BAF2418F186657",
		"565189B306D6D1921D33101EDD024035F80F11E13411133F4134934A9CE34AD6")

	usd, err := common.NewAsset("TERRA.USD")
	if err != nil {
		ctx.Logger().Error("unable to create TERRA.USD asset: %w", err)
		return
	}

	ust, _ := common.NewAsset("TERRA.UST")
	if err != nil {
		ctx.Logger().Error("unable to create TERRA.UST asset: %w", err)
		return
	}

	// Update TERRA.USD pool
	pools, err := mgr.Keeper().GetPools(ctx)
	if err != nil {
		ctx.Logger().Error("unable to get pools: %w", err)
	}

	for _, pool := range pools {
		if pool.Asset.Equals(usd) {
			pool.Asset = ust
			err := mgr.Keeper().SetPool(ctx, pool)
			if err != nil {
				ctx.Logger().Error("unable to set pool: %w", err)
				return
			}
		}
	}

	// Update TERRA.USD vaults
	vaults, err := mgr.Keeper().GetAsgardVaults(ctx)
	if err != nil {
		ctx.Logger().Error("unable to get vaults: %w", err)
		return
	}

	for _, vault := range vaults {
		var vaultCoins common.Coins
		for _, c := range vault.Coins {
			if c.Asset.Equals(usd) {
				c.Asset = ust
			}
			vaultCoins = append(vaultCoins, c)
		}
		vault.Coins = vaultCoins
		err := mgr.Keeper().SetVault(ctx, vault)
		if err != nil {
			ctx.Logger().Error("unable to set vaults: %w", err)
		}
	}
}
