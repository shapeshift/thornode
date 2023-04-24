//go:build testnet || mocknet
// +build testnet mocknet

package thorchain

import (
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

// migrateStoreV86 remove all LTC asset from the retiring vault
func migrateStoreV86(ctx cosmos.Context, mgr *Mgrs) {
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to migrate store to v86", "error", err)
		}
	}()
	vaults, err := mgr.Keeper().GetAsgardVaultsByStatus(ctx, RetiringVault)
	if err != nil {
		ctx.Logger().Error("fail to get retiring asgard vaults", "error", err)
		return
	}
	for _, v := range vaults {
		ltcCoin := v.GetCoin(common.LTCAsset)
		v.SubFunds(common.NewCoins(ltcCoin))
		if err := mgr.Keeper().SetVault(ctx, v); err != nil {
			ctx.Logger().Error("fail to save vault", "error", err)
		}
	}
}

func migrateStoreV88(ctx cosmos.Context, mgr Manager) {}

// no op
func migrateStoreV102(ctx cosmos.Context, mgr Manager) {}

// no op
func migrateStoreV103(ctx cosmos.Context, mgr *Mgrs) {}

func migrateStoreV106(ctx cosmos.Context, mgr *Mgrs) {
	// testing for migrateStoreV106 in chaosnet
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to migrate store to v106", "error", err)
		}
	}()

	recipient, err := cosmos.AccAddressFromBech32("tthor1zf3gsk7edzwl9syyefvfhle37cjtql35h6k85m")
	if err != nil {
		ctx.Logger().Error("fail to create acc address from bech32", err)
		return
	}

	coins := cosmos.NewCoins(cosmos.NewCoin(
		"btc/btc",
		cosmos.NewInt(488432852150),
	))
	if err := mgr.coinKeeper.SendCoinsFromModuleToAccount(ctx, AsgardName, recipient, coins); err != nil {
		ctx.Logger().Error("fail to SendCoinsFromModuleToAccount", err)
	}
}

// For v108-requeue.yaml regression test
func migrateStoreV108(ctx cosmos.Context, mgr *Mgrs) {
	// Requeue four BCH.BCH txout (dangling actions) items swallowed in a chain halt.
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to migrate store to v108", "error", err)
		}
	}()

	danglingInboundTxIDs := []common.TxID{
		"5840920B63CDB9A02028ABB844B28F0305C2B37ADA4F936B69EBEFA04E2F826B",
		"BFACE691A12E85083DD2E4E4ADFBE702299DA6F08A98E6B6F7CF95A9D1D71632",
		"395EBDADA6D0975CF4D3F2E2BD7E246037C672C9CAB97DBFB744CC0F2FFABE95",
		"5881692D0522D0D5221A61FC0704B018ED51A6C43475063ADF6AC912D748208D",
	}

	requeueDanglingActionsV108(ctx, mgr, danglingInboundTxIDs)
}

// no op
func migrateStoreV109(ctx cosmos.Context, mgr *Mgrs) {}
