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
