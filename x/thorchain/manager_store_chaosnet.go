//go:build !testnet && !stagenet
// +build !testnet,!stagenet

package thorchain

import (
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	thorchain "gitlab.com/thorchain/thornode/x/thorchain/memo"
)

func creditAssetBackToVaultAndPool(ctx cosmos.Context, mgr Manager) {
	// These are the asset that is in the outbound queue, which are supposed to go to the attacker address
	// but it will be stopped by bifrost, thus these asset didn't actually leave the network
	// swap these asset back to RUNE , and then credit it back to ETH pool
	input := []struct {
		assetName string
		amount    cosmos.Uint
	}{
		{"ETH.AAVE-0X7FC66500C84A76AD7E9C93437BFC5AC33E2DDAE9", cosmos.NewUint(20912604795)},
		{"ETH.HEGIC-0X584BC13C7D411C00C01A62E8019472DE68768430", cosmos.NewUint(2098087620642)},
		{"ETH.YFI-0X0BC529C00C6401AEF6D220BE8C6EA1667F6AD93E", cosmos.NewUint(555358575)},
		{"ETH.SUSHI-0X6B3595068778DD592E39A122F4F5A5CF09C90FE2", cosmos.NewUint(3571961904132)},
		{"ETH.KYL-0X67B6D479C7BB412C54E03DCA8E1BC6740CE6B99C", cosmos.NewUint(35103388382444)},
		{"ETH.DODO-0X43DFC4159D86F3A37A5A4B3D4580B888AD7D4DDD", cosmos.NewUint(83806675976)},
	}
	ethPool, err := mgr.Keeper().GetPool(ctx, common.ETHAsset)
	if err != nil {
		ctx.Logger().Error("fail to get ETH pool", "error", err)
		return
	}
	// based on calculation , this is the total amount of ETH left in the network after the attack
	ethPool.BalanceAsset = cosmos.NewUint(76228226137)
	for _, item := range input {
		asset, err := common.NewAsset(item.assetName)
		if err != nil {
			ctx.Logger().Error("fail to parse asset", "asset", item.assetName, "error", err)
			continue
		}
		pool, err := mgr.Keeper().GetPool(ctx, asset)
		if err != nil {
			ctx.Logger().Error("fail to get pool", "asset", item.assetName, "error", err)
			continue
		}
		runeAmount := pool.AssetValueInRune(item.amount)
		pool.BalanceAsset = pool.BalanceAsset.Add(item.amount)
		pool.BalanceRune = common.SafeSub(pool.BalanceRune, runeAmount)
		if pool.BalanceRune.IsZero() {
			ctx.Logger().Info("pool balance rune is 0 , skip it", "asset", item.assetName)
			continue
		}
		ethPool.BalanceRune = ethPool.BalanceRune.Add(runeAmount)
		if err := mgr.Keeper().SetPool(ctx, pool); err != nil {
			ctx.Logger().Error("fail to save pool", "asset", item.assetName, "error", err)
			continue
		}
		if err := mgr.Keeper().SetPool(ctx, ethPool); err != nil {
			ctx.Logger().Error("fail to save ETH pool", "error", err)
			continue
		}
	}
	if err := mgr.Keeper().SetPool(ctx, ethPool); err != nil {
		ctx.Logger().Error("fail to save ETH pool", "error", err)
		return
	}
	// correct the asgard ETH balance
	// asgard vault only has 434.656951721660530911 Ether left
	// https://etherscan.io/address/0xf56cba49337a624e94042e325ad6bc864436e370
	// however asgard think it still have 13393.48406651
	// subtract the gap from asgard vault
	ethAmount := cosmos.NewUint(1295882711480)
	ethCoin := common.NewCoin(common.ETHAsset, ethAmount)
	asgards, err := mgr.Keeper().GetAsgardVaultsByStatus(ctx, ActiveVault)
	if err != nil {
		ctx.Logger().Error("fail to get active asgard", "error", err)
		return
	}
	if len(asgards) == 0 {
		ctx.Logger().Info("didn't find any asgard")
		return
	}
	// add all these funds back to asgard
	asgards[0].SubFunds(common.NewCoins(ethCoin))
	if err := mgr.Keeper().SetVault(ctx, asgards[0]); err != nil {
		ctx.Logger().Error("fail to save asgard", "error", err)
		return
	}
}

func purgeETHOutboundQueue(ctx cosmos.Context, mgr Manager) {
	signingTransPeriod := mgr.GetConstants().GetInt64Value(constants.SigningTransactionPeriod)
	if common.BlockHeight(ctx) < signingTransPeriod {
		return
	}
	// temporaryUSDTHolder address is thorchain deployer
	thorchainDeployerAddr, err := common.NewAddress(temporaryUSDTHolder)
	if err != nil {
		ctx.Logger().Error("fail to parse thorchain deployer address", "error", err)
	}
	startHeight := common.BlockHeight(ctx) - signingTransPeriod
	for height := startHeight; height < common.BlockHeight(ctx); height++ {
		txOut, err := mgr.Keeper().GetTxOut(ctx, height)
		if err != nil {
			ctx.Logger().Error("fail to get txout", "height", height, "error", err)
			continue
		}
		changed := false
		for idx, txOutItem := range txOut.TxArray {
			if !txOutItem.Chain.Equals(common.ETHChain) {
				continue
			}
			m, err := ParseMemo(mgr.GetVersion(), txOutItem.Memo)
			if err != nil {
				ctx.Logger().Error("fail to parse memo", "error", err)
				continue
			}
			// do not remove yggdrasil+ and refund tx out item
			switch m.GetType() {
			case TxYggdrasilFund, thorchain.TxRefund:
				continue
			}
			if txOutItem.Coin.Asset.Equals(common.ETHAsset) {
				ctx.Logger().Info("txout item marked as done", "in_hash", txOutItem.InHash, "memo", txOutItem.Memo)
				txOut.TxArray[idx].OutHash = common.BlankTxID
			} else {
				txOut.TxArray[idx].ToAddress = thorchainDeployerAddr
			}
			changed = true
		}
		if changed {
			if err := mgr.Keeper().SetTxOut(ctx, txOut); err != nil {
				ctx.Logger().Error("fail to save tx out", "error", err)
			}
		}
	}
}

func correctAsgardVaultBalanceV61(ctx cosmos.Context, mgr Manager, asgardPubKey common.PubKey) {
	gaps := []struct {
		name   string
		amount cosmos.Uint
	}{
		{"BNB.AVA-645", cosmos.NewUint(11208500000)},
		{"BNB.BNB", cosmos.NewUint(5451406786)},
		{"BNB.BUSD-BD1", cosmos.NewUint(23628616582724)},
		{"BNB.CAS-167", cosmos.NewUint(1534000000000)},
		{"BNB.ETH-1C9", cosmos.NewUint(104959279)},
		{"LTC.LTC", cosmos.NewUint(22892875)},
		{"BTC.BTC", cosmos.NewUint(145030708)},
		{"ETH.SUSHI-0X6B3595068778DD592E39A122F4F5A5CF09C90FE2", cosmos.NewUint(453522456575)},
		{"ETH.USDC-0XA0B86991C6218B36C1D19D4A2E9EB0CE3606EB48", cosmos.NewUint(336236128900)},
		{"ETH.KYL-0X67B6D479C7BB412C54E03DCA8E1BC6740CE6B99C", cosmos.NewUint(178627612214)},
		{"ETH.DODO-0X43DFC4159D86F3A37A5A4B3D4580B888AD7D4DDD", cosmos.NewUint(321108830146)},
		{"BNB.RUNE-B1A", cosmos.NewUint(760601586434)},
		{"ETH.AAVE-0X7FC66500C84A76AD7E9C93437BFC5AC33E2DDAE9", cosmos.NewUint(2600000000)},
		{"ETH.XRUNE-0X69FA0FEE221AD11012BAB0FDB45D444D3D2CE71C", cosmos.NewUint(4475832856439)},
	}
	var coins common.Coins
	for _, item := range gaps {
		asset, err := common.NewAsset(item.name)
		if err != nil {
			ctx.Logger().Error("fail to parse asset", "asset", item.name, "error", err)
			continue
		}
		coins = append(coins, common.NewCoin(asset, item.amount))
	}

	asgards, err := mgr.Keeper().GetAsgardVaultsByStatus(ctx, ActiveVault)
	if err != nil {
		ctx.Logger().Error("fail to get active asgard", "error", err)
		return
	}

	for _, v := range asgards {
		if !v.PubKey.Equals(asgardPubKey) {
			continue
		}
		v.AddFunds(coins)
		if err := mgr.Keeper().SetVault(ctx, v); err != nil {
			ctx.Logger().Error("fail to save asgard", "error", err)
			return
		}
	}
}

func migrateStoreV80(ctx cosmos.Context, mgr Manager) {}

func migrateStoreV86(ctx cosmos.Context, mgr *Mgrs) {
}
