// +build !testnet

package thorchain

import (
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

func (smgr *StoreMgr) creditAssetBackToVaultAndPool(ctx cosmos.Context) {
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
	ethPool, err := smgr.mgr.Keeper().GetPool(ctx, common.ETHAsset)
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
		pool, err := smgr.mgr.Keeper().GetPool(ctx, asset)
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
		if err := smgr.mgr.Keeper().SetPool(ctx, pool); err != nil {
			ctx.Logger().Error("fail to save pool", "asset", item.assetName, "error", err)
			continue
		}
		if err := smgr.mgr.Keeper().SetPool(ctx, ethPool); err != nil {
			ctx.Logger().Error("fail to save ETH pool", "error", err)
			continue
		}
	}
	if err := smgr.mgr.Keeper().SetPool(ctx, ethPool); err != nil {
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
	asgards, err := smgr.mgr.Keeper().GetAsgardVaultsByStatus(ctx, ActiveVault)
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
	if err := smgr.mgr.Keeper().SetVault(ctx, asgards[0]); err != nil {
		ctx.Logger().Error("fail to save asgard", "error", err)
		return
	}
}
