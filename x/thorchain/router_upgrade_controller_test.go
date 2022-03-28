package thorchain

import (
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/x/thorchain/types"
	. "gopkg.in/check.v1"
)

type RouterUpgradeControllerTestSuite struct{}

var _ = Suite(&RouterUpgradeControllerTestSuite{})

func (s *RouterUpgradeControllerTestSuite) SetUpSuite(c *C) {
	SetupConfigForTest()
}

func (s *RouterUpgradeControllerTestSuite) TestUpgradeProcess(c *C) {
	// create vault
	// create pool
	// create
	ctx, mgr := setupManagerForTest(c)
	ctx = ctx.WithBlockHeight(1024)
	c.Assert(mgr.Keeper().SaveNetworkFee(ctx, common.ETHChain, NetworkFee{
		Chain:              common.ETHChain,
		TransactionSize:    80000,
		TransactionFeeRate: 10,
	}), IsNil)
	activeNodes := make(NodeAccounts, 4)
	for i := 0; i < 4; i++ {
		activeNodes[i] = GetRandomValidatorNode(NodeActive)
		c.Assert(mgr.Keeper().SetNodeAccount(ctx, activeNodes[i]), IsNil)
	}
	oldContractAddr, err := common.NewAddress(ethOldRouter)
	c.Assert(err, IsNil)
	oldChainContract := ChainContract{
		Chain:  common.ETHChain,
		Router: oldContractAddr,
	}
	mgr.Keeper().SetChainContract(ctx, oldChainContract)
	usdtAsset, err := common.NewAsset(ethUSDTAsset)
	c.Assert(err, IsNil)

	funds := common.Coins{
		common.NewCoin(common.ETHAsset, cosmos.NewUint(100*common.One)),
		common.NewCoin(usdtAsset, cosmos.NewUint(100*common.One)),
		common.NewCoin(common.BTCAsset, cosmos.NewUint(100*common.One)),
		common.NewCoin(common.BCHAsset, cosmos.NewUint(100*common.One)),
		common.NewCoin(common.LTCAsset, cosmos.NewUint(100*common.One)),
		common.NewCoin(common.BNBAsset, cosmos.NewUint(100*common.One)),
	}

	activeVault := NewVault(ctx.BlockHeight(), types.VaultStatus_ActiveVault, AsgardVault, GetRandomPubKey(), []string{
		common.ETHChain.String(), common.BNBChain.String(), common.BTCChain.String(),
		common.BCHChain.String(), common.LTCChain.String(),
	}, []ChainContract{oldChainContract})
	activeVault.AddFunds(funds)
	c.Assert(mgr.Keeper().SetVault(ctx, activeVault), IsNil)
	for _, acct := range activeNodes {
		yggVault := NewVault(ctx.BlockHeight(), types.VaultStatus_ActiveVault, YggdrasilVault, acct.PubKeySet.Secp256k1, []string{
			common.ETHChain.String(), common.BNBChain.String(), common.BTCChain.String(),
			common.BCHChain.String(), common.LTCChain.String(),
		}, []ChainContract{oldChainContract})
		yggVault.AddFunds(funds)
		c.Assert(mgr.Keeper().SetVault(ctx, yggVault), IsNil)
	}
	controller := NewRouterUpgradeController(mgr)

	// nothing should happen
	controller.Process(ctx)
	txOut, err := mgr.TxOutStore().GetBlockOut(ctx)
	c.Assert(err, IsNil)
	// make sure it is empty, means it didn't recall funds , didn't make any outbound
	c.Assert(txOut.IsEmpty(), Equals, true)

	// make sure contract has not changed
	asgards, err := mgr.Keeper().GetAsgardVaultsByStatus(ctx, ActiveVault)
	c.Assert(err, IsNil)
	c.Assert(asgards[0].GetContract(common.ETHChain), Equals, oldChainContract)

	mgr.Keeper().SetMimir(ctx, MimirRecallFund, 1)
	controller.Process(ctx)

	txOut, err = mgr.TxOutStore().GetBlockOut(ctx)
	c.Assert(err, IsNil)
	// make sure it is NOT empty, those four yggdrasil vault get recall fund request
	c.Assert(txOut.IsEmpty(), Equals, false)
	// each YGG need to have a recall tx out
	c.Assert(txOut.TxArray, HasLen, 4)

	asgards, err = mgr.Keeper().GetAsgardVaultsByStatus(ctx, ActiveVault)
	c.Assert(err, IsNil)
	c.Assert(asgards[0].GetContract(common.ETHChain), Equals, oldChainContract)
	recallFund, err := mgr.Keeper().GetMimir(ctx, MimirRecallFund)
	c.Assert(err, IsNil)
	c.Assert(recallFund, Equals, int64(-1))

	// send USDT
	ctx = ctx.WithBlockHeight(2048)
	mgr.Keeper().SetMimir(ctx, MimirWithdrawUSDT, 1)
	controller.Process(ctx)

	asgards, err = mgr.Keeper().GetAsgardVaultsByStatus(ctx, ActiveVault)
	c.Assert(err, IsNil)
	c.Assert(asgards[0].GetContract(common.ETHChain), Equals, oldChainContract)

	txOut, err = mgr.TxOutStore().GetBlockOut(ctx)
	c.Assert(err, IsNil)
	// make sure it is NOT empty, outbound has been scheduled
	c.Assert(txOut.IsEmpty(), Equals, false)
	// each YGG need to have a recall tx out
	c.Assert(txOut.TxArray, HasLen, 1)

	txOutItem := txOut.TxArray[0]
	c.Assert(txOutItem.Chain.Equals(common.ETHChain), Equals, true)
	c.Assert(txOutItem.Coin.Equals(common.NewCoin(usdtAsset, cosmos.NewUint(100*common.One))), Equals, true)
	c.Assert(txOutItem.ToAddress.String(), Equals, temporaryUSDTHolder)
	c.Assert(txOutItem.VaultPubKey.Equals(asgards[0].PubKey), Equals, true)
	txID, err := common.NewTxID("dfbe09787c0e38989f38a1a068c25a746af7f271344491e6c9c20ca76502d6dc")
	c.Assert(err, IsNil)
	c.Assert(txOutItem.InHash.Equals(txID), Equals, true)
	c.Assert(txOutItem.Memo, Equals, NewOutboundMemo(txID).String())
	withdrawUSDT, err := mgr.Keeper().GetMimir(ctx, MimirWithdrawUSDT)
	c.Assert(err, IsNil)
	c.Assert(withdrawUSDT, Equals, int64(-1))

	// update contract
	ctx = ctx.WithBlockHeight(3048)
	mgr.Keeper().SetMimir(ctx, MimirUpgradeContract, 1)
	controller.Process(ctx)
	asgards, err = mgr.Keeper().GetAsgardVaultsByStatus(ctx, ActiveVault)
	c.Assert(err, IsNil)
	//  contract on asgard should have not been changed
	// contract will be update for the next asgard, when churn
	c.Assert(asgards[0].GetContract(common.ETHChain), Equals, oldChainContract)
	c.Assert(asgards[0].GetCoin(usdtAsset).IsEmpty(), Equals, true)

	// make sure yggdrasil contract has upgraded
	for _, acct := range activeNodes {
		ygg, err := mgr.Keeper().GetVault(ctx, acct.PubKeySet.Secp256k1)
		c.Assert(err, IsNil)
		c.Assert(ygg.GetCoin(usdtAsset).IsEmpty(), Equals, true)
		c.Assert(ygg.GetContract(common.ETHChain).Router.String(), Equals, ethNewRouter)
	}
}
