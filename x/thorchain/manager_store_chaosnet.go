//go:build !testnet && !stagenet && !mocknet
// +build !testnet,!stagenet,!mocknet

package thorchain

import (
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

func migrateStoreV86(ctx cosmos.Context, mgr *Mgrs) {}

func importPreRegistrationTHORNames(ctx cosmos.Context, mgr Manager) error {
	oneYear := fetchConfigInt64(ctx, mgr, constants.BlocksPerYear)
	names, err := getPreRegisterTHORNames(ctx, ctx.BlockHeight()+oneYear)
	if err != nil {
		return err
	}

	for _, name := range names {
		mgr.Keeper().SetTHORName(ctx, name)
	}
	return nil
}

func migrateStoreV88(ctx cosmos.Context, mgr Manager) {
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to migrate store to v88", "error", err)
		}
	}()

	err := importPreRegistrationTHORNames(ctx, mgr)
	if err != nil {
		ctx.Logger().Error("fail to migrate store to v88", "error", err)
	}
}

// no op
func migrateStoreV102(ctx cosmos.Context, mgr Manager) {}

func migrateStoreV103(ctx cosmos.Context, mgr *Mgrs) {
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to migrate store to v102", "error", err)
		}
	}()

	// MAINNET REFUND
	// A user sent two 4,500 RUNE swap out txs (to USDT), but the external asset matching had a conflict and the outbounds were dropped. Txs:

	// https://viewblock.io/thorchain/tx/B07A6B1B40ADBA2E404D9BCE1BEF6EDE6F70AD135E83806E4F4B6863CF637D0B
	// https://viewblock.io/thorchain/tx/4795A3C036322493A9692B5D44E7D4FF29C3E2C1E848637184E98FE8B05FD06E

	// The below methodology was tested on Stagenet, results are documented here: https://gitlab.com/thorchain/thornode/-/merge_requests/2596#note_1216814315

	// The RUNE was swapped to ETH, but the outbound swap out was dropped by Bifrost. This means RUNE was added, ETH was removed from
	// the pool. This must be reversed and the RUNE sent back to the user.
	// So:
	// 1. Credit the total ETH amount back the pool, this ETH is already in the pool since the outbound was dropped.
	// 2. Deduct the RUNE balance from the ETH pool, this will be sent back to the user.
	// 3. Send the user RUNE from Asgard.
	//
	// Note: the Asgard vault does not need to be credited the ETH since the outbound was never sent, thus never observed (which
	// is where Vault funds are subtracted)

	firstSwapOut := DroppedSwapOutTx{
		inboundHash: "B07A6B1B40ADBA2E404D9BCE1BEF6EDE6F70AD135E83806E4F4B6863CF637D0B",
		gasAsset:    common.ETHAsset,
	}

	err := refundDroppedSwapOutFromRUNE(ctx, mgr, firstSwapOut)
	if err != nil {
		ctx.Logger().Error("fail to migrate store to v103 refund failed", "error", err, "tx hash", "B07A6B1B40ADBA2E404D9BCE1BEF6EDE6F70AD135E83806E4F4B6863CF637D0B")
	}

	secondSwapOut := DroppedSwapOutTx{
		inboundHash: "4795A3C036322493A9692B5D44E7D4FF29C3E2C1E848637184E98FE8B05FD06E",
		gasAsset:    common.ETHAsset,
	}

	err = refundDroppedSwapOutFromRUNE(ctx, mgr, secondSwapOut)
	if err != nil {
		ctx.Logger().Error("fail to migrate store to v103 refund failed", "error", err, "tx hash", "4795A3C036322493A9692B5D44E7D4FF29C3E2C1E848637184E98FE8B05FD06E")
	}
}
