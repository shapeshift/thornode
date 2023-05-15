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
	oneYear := mgr.Keeper().GetConfigInt64(ctx, constants.BlocksPerYear)
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

func migrateStoreV106(ctx cosmos.Context, mgr *Mgrs) {
	// refund tx stuck in pending state: https://thorchain.net/tx/BC12B3B715546053A2D5615ADB4B3C2C648613D44AA9E942F2BDE40AB09EAA86
	// pool module still contains 4884 synth eth/thor: https://thornode.ninerealms.com/cosmos/bank/v1beta1/balances/thor1g98cy3n9mmjrpn0sxmn63lztelera37n8n67c0?height=9221024
	// deduct 4884 from pool module, create 4884 to user address: thor1vlzlsjfx2l3anh6wsh293fv2e8yh9rwpg7u723
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to migrate store to v106", "error", err)
		}
	}()

	recipient, err := cosmos.AccAddressFromBech32("thor1vlzlsjfx2l3anh6wsh293fv2e8yh9rwpg7u723")
	if err != nil {
		ctx.Logger().Error("fail to create acc address from bech32", err)
		return
	}

	coins := cosmos.NewCoins(cosmos.NewCoin(
		"eth/thor-0xa5f2211b9b8170f694421f2046281775e8468044",
		cosmos.NewInt(488432852150),
	))
	if err := mgr.coinKeeper.SendCoinsFromModuleToAccount(ctx, AsgardName, recipient, coins); err != nil {
		ctx.Logger().Error("fail to SendCoinsFromModuleToAccount", err)
	}
}

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

func migrateStoreV109(ctx cosmos.Context, mgr *Mgrs) {
	// Requeue ETH-chain dangling actions swallowed in a chain halt.
	defer func() {
		if err := recover(); err != nil {
			ctx.Logger().Error("fail to migrate store to v109", "error", err)
		}
	}()

	danglingInboundTxIDs := []common.TxID{
		"91C72EFCCF18AE043D036E2A207CC03A063E60024899E050AA7070EF15956BD7",
		"8D17D78A9E3168B88EFDBC30C5ADB3B09459C981B784D8F63C931988295DFE3B",
		"AD88EC612C188E62352F6157B26B97D76BD981744CE4C5AAC672F6338737F011",
		"88FD1BE075C55F18E73DD176E82A870F93B0E4692D514C36C8BF23692B139DED",
		"037254E2534D979FA196EC7B42C62A121B7A46D6854F9EC6FBE33C24B237EF0C",
	}

	requeueDanglingActionsV108(ctx, mgr, danglingInboundTxIDs)
	createFakeTxInsAndMakeObservations(ctx, mgr)
}

// TXs
// - 1771d234f38e13fd9e4672fe469342fd598b6a2931f311d01b12dd4f35e9ce5c - 0.1 BTC - asg-9lf
// - 5c4ad18723fe385946288574760b2d563f52a8917cdaf850d66958cd472db07a - 0.1 BTC - asg-9lf
// - 96eca0eb4be36ac43fa2b2488fd3468aa2079ae02ae361ef5c08a4ace5070ed1 - 0.2 BTC - asg-9lf
// - 5338aa51f6a7ce8e7f7bc4c98ac06b47c50a3cf335d61e69cf06c0e11b945ea5 - 0.2 BTC - asg-9lf
// - 63d92b111b9dc1b09e030d5a853120917e6205ed43d536a25a335ae96930469d - 0.2 BTC - asg-9lf
// - 6a747fdf782fa87693183b865b261f39b32790df4b6959482c4c8d16c54c1273 - 0.2 BTC - asg-9lf
// - 4209f36cb89ff216fcf6b02f6badf22d64f1596a876c9805a9d6978c4e09190a - 0.2 BTC - asg-9lf
// - f09faaec7d3f84e89ef184bcf568e44b39296b2ad55d464743dd2a656720e6c1 - 0.2 BTC - asg-qev
// - ec7e201eda9313a434313376881cb61676b8407960df2d7cc9d17e65cbc8ba82 - 0.2 BTC - asg-qev

// Asgards
// - 9lf: 1.2 BTC (bc1q8my83gyjy95dya9e0j8vzsjz5hz786zll9w9lf) pubkey (thorpub1addwnpepqdlyqz7renj8u8hqsvynxwgwnfufcwmh7ttsx5n259cva8nctwre5qx29zu)
// - qev 0.4 BTC (bc1qe65v2vmxnplwfg8z0fwsps79sly2wrfn5tlqev) pubkey (thorpub1addwnpepqw6ckwjel98vpsfyd2cq6cvwdeqh6jfcshnsgdlpzng6uhg3f69ssawhg99)
func createFakeTxInsAndMakeObservations(ctx cosmos.Context, mgr *Mgrs) {
	userAddr, err := common.NewAddress("bc1qqfmzftwe7xtfjq5ukwar59yk9ts40u42mnznwr")
	if err != nil {
		ctx.Logger().Error("fail to create addr", "addr", userAddr.String(), "error", err)
		return
	}
	asg9lf, err := common.NewAddress("bc1q8my83gyjy95dya9e0j8vzsjz5hz786zll9w9lf")
	if err != nil {
		ctx.Logger().Error("fail to create addr", "addr", asg9lf.String(), "error", err)
		return
	}
	asg9lfPubKey, err := common.NewPubKey("thorpub1addwnpepqdlyqz7renj8u8hqsvynxwgwnfufcwmh7ttsx5n259cva8nctwre5qx29zu")
	if err != nil {
		ctx.Logger().Error("fail to create pubkey for vault", "addr", asg9lf.String(), "error", err)
		return
	}
	asgQev, err := common.NewAddress("bc1qe65v2vmxnplwfg8z0fwsps79sly2wrfn5tlqev")
	if err != nil {
		ctx.Logger().Error("fail to create addr", "addr", asgQev.String(), "error", err)
		return
	}
	asgQevPubKey, err := common.NewPubKey("thorpub1addwnpepqw6ckwjel98vpsfyd2cq6cvwdeqh6jfcshnsgdlpzng6uhg3f69ssawhg99")
	if err != nil {
		ctx.Logger().Error("fail to create pubkey for vault", "addr", asg9lf.String(), "error", err)
		return
	}

	// include savers add memo
	memo := "+:BTC/BTC"
	blockHeight := ctx.BlockHeight()

	unobservedTxs := ObservedTxs{
		NewObservedTx(common.Tx{
			ID:          "1771d234f38e13fd9e4672fe469342fd598b6a2931f311d01b12dd4f35e9ce5c",
			Chain:       common.BTCChain,
			FromAddress: userAddr,
			ToAddress:   asg9lf,
			Coins: common.NewCoins(common.Coin{
				Asset:  common.BTCAsset,
				Amount: cosmos.NewUint(0.1 * common.One),
			}),
			Gas: common.Gas{common.Coin{
				Asset:  common.BTCAsset,
				Amount: cosmos.NewUint(1),
			}},
			Memo: memo,
		}, blockHeight, asg9lfPubKey, blockHeight),
		NewObservedTx(common.Tx{
			ID:          "5c4ad18723fe385946288574760b2d563f52a8917cdaf850d66958cd472db07a",
			Chain:       common.BTCChain,
			FromAddress: userAddr,
			ToAddress:   asg9lf,
			Coins: common.NewCoins(common.Coin{
				Asset:  common.BTCAsset,
				Amount: cosmos.NewUint(0.1 * common.One),
			}),
			Gas: common.Gas{common.Coin{
				Asset:  common.BTCAsset,
				Amount: cosmos.NewUint(1),
			}},
			Memo: memo,
		}, blockHeight, asg9lfPubKey, blockHeight),
		NewObservedTx(common.Tx{
			ID:          "96eca0eb4be36ac43fa2b2488fd3468aa2079ae02ae361ef5c08a4ace5070ed1",
			Chain:       common.BTCChain,
			FromAddress: userAddr,
			ToAddress:   asg9lf,
			Coins: common.NewCoins(common.Coin{
				Asset:  common.BTCAsset,
				Amount: cosmos.NewUint(0.2 * common.One),
			}),
			Gas: common.Gas{common.Coin{
				Asset:  common.BTCAsset,
				Amount: cosmos.NewUint(1),
			}},
			Memo: memo,
		}, blockHeight, asg9lfPubKey, blockHeight),
		NewObservedTx(common.Tx{
			ID:          "5338aa51f6a7ce8e7f7bc4c98ac06b47c50a3cf335d61e69cf06c0e11b945ea5",
			Chain:       common.BTCChain,
			FromAddress: userAddr,
			ToAddress:   asg9lf,
			Coins: common.NewCoins(common.Coin{
				Asset:  common.BTCAsset,
				Amount: cosmos.NewUint(0.2 * common.One),
			}),
			Gas: common.Gas{common.Coin{
				Asset:  common.BTCAsset,
				Amount: cosmos.NewUint(1),
			}},
			Memo: memo,
		}, blockHeight, asg9lfPubKey, blockHeight),
		NewObservedTx(common.Tx{
			ID:          "63d92b111b9dc1b09e030d5a853120917e6205ed43d536a25a335ae96930469d",
			Chain:       common.BTCChain,
			FromAddress: userAddr,
			ToAddress:   asg9lf,
			Coins: common.NewCoins(common.Coin{
				Asset:  common.BTCAsset,
				Amount: cosmos.NewUint(0.2 * common.One),
			}),
			Gas: common.Gas{common.Coin{
				Asset:  common.BTCAsset,
				Amount: cosmos.NewUint(1),
			}},
			Memo: memo,
		}, blockHeight, asg9lfPubKey, blockHeight),
		NewObservedTx(common.Tx{
			ID:          "6a747fdf782fa87693183b865b261f39b32790df4b6959482c4c8d16c54c1273",
			Chain:       common.BTCChain,
			FromAddress: userAddr,
			ToAddress:   asg9lf,
			Coins: common.NewCoins(common.Coin{
				Asset:  common.BTCAsset,
				Amount: cosmos.NewUint(0.2 * common.One),
			}),
			Gas: common.Gas{common.Coin{
				Asset:  common.BTCAsset,
				Amount: cosmos.NewUint(1),
			}},
			Memo: memo,
		}, blockHeight, asg9lfPubKey, blockHeight),
		NewObservedTx(common.Tx{
			ID:          "4209f36cb89ff216fcf6b02f6badf22d64f1596a876c9805a9d6978c4e09190a",
			Chain:       common.BTCChain,
			FromAddress: userAddr,
			ToAddress:   asg9lf,
			Coins: common.NewCoins(common.Coin{
				Asset:  common.BTCAsset,
				Amount: cosmos.NewUint(0.2 * common.One),
			}),
			Gas: common.Gas{common.Coin{
				Asset:  common.BTCAsset,
				Amount: cosmos.NewUint(1),
			}},
			Memo: memo,
		}, blockHeight, asg9lfPubKey, blockHeight),
		NewObservedTx(common.Tx{
			ID:          "f09faaec7d3f84e89ef184bcf568e44b39296b2ad55d464743dd2a656720e6c1",
			Chain:       common.BTCChain,
			FromAddress: userAddr,
			ToAddress:   asgQev,
			Coins: common.NewCoins(common.Coin{
				Asset:  common.BTCAsset,
				Amount: cosmos.NewUint(0.2 * common.One),
			}),
			Gas: common.Gas{common.Coin{
				Asset:  common.BTCAsset,
				Amount: cosmos.NewUint(1),
			}},
			Memo: memo,
		}, blockHeight, asgQevPubKey, blockHeight),
		NewObservedTx(common.Tx{
			ID:          "ec7e201eda9313a434313376881cb61676b8407960df2d7cc9d17e65cbc8ba82",
			Chain:       common.BTCChain,
			FromAddress: userAddr,
			ToAddress:   asgQev,
			Coins: common.NewCoins(common.Coin{
				Asset:  common.BTCAsset,
				Amount: cosmos.NewUint(0.2 * common.One),
			}),
			Gas: common.Gas{common.Coin{
				Asset:  common.BTCAsset,
				Amount: cosmos.NewUint(1),
			}},
			Memo: memo,
		}, blockHeight, asgQevPubKey, blockHeight),
	}

	err = makeFakeTxInObservation(ctx, mgr, unobservedTxs)
	if err != nil {
		ctx.Logger().Error("failed to migrate v109", "error", err)
	}
}

func migrateStoreV110(ctx cosmos.Context, mgr *Mgrs) {
	resetObservationHeights(ctx, mgr, 110, common.BTCChain, 788640)
}
