package thorchain

import (
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain/types"
	. "gopkg.in/check.v1"
)

type StoreManagerTestSuite struct{}

var _ = Suite(&StoreManagerTestSuite{})

func (s *StoreManagerTestSuite) TestMigrateStoreV55(c *C) {
	ctx, mgr := setupManagerForTest(c)
	storeMgr := NewStoreMgr(mgr)
	keeper := storeMgr.mgr.Keeper()
	version := GetCurrentVersion()
	constantAccessor := constants.GetConstantValues(version)
	assetToAdjust, err := common.NewAsset("BNB.USDT-6D8")
	c.Assert(err, IsNil)
	pool := NewPool()
	pool.Asset = assetToAdjust
	c.Assert(keeper.SetPool(ctx, pool), IsNil)
	storeMgr.migrateStoreV55(ctx, version, constantAccessor)
	newPool, err := keeper.GetPool(ctx, assetToAdjust)
	c.Assert(err, IsNil)
	c.Assert(newPool.BalanceAsset.Equal(cosmos.NewUint(900000000)), Equals, true)
}
func (s *StoreManagerTestSuite) TestMigrateStoreV58(c *C) {
	SetupConfigForTest()
	ctx, mgr := setupManagerForTest(c)
	storeMgr := NewStoreMgr(mgr)
	version := GetCurrentVersion()
	constantAccessor := constants.GetConstantValues(version)
	pubKey, err := common.NewPubKey("tthorpub1addwnpepqg65km6vfflrlymsjhrnmn4w58d2d36h977pcu3aqp6dxee2yf88yg0z3v4")
	c.Assert(err, IsNil)
	retiredVault := NewVault(1024, types.VaultStatus_InactiveVault, AsgardVault, pubKey, []string{
		common.BTCChain.String(),
		common.BNBChain.String(),
		common.ETHChain.String(),
		common.LTCChain.String(),
		common.BCHChain.String(),
	}, nil)
	c.Assert(mgr.Keeper().SetVault(ctx, retiredVault), IsNil)

	retiringPubKey, err := common.NewPubKey("tthorpub1addwnpepqfz98sx54jpv3f95qfg39zkx500avc6tr0d8ww0lv283yu3ucgq3g9y9njj")
	c.Assert(err, IsNil)
	retiringVault := NewVault(1024, types.VaultStatus_RetiringVault, AsgardVault, retiringPubKey, []string{
		common.BTCChain.String(),
		common.BNBChain.String(),
		common.ETHChain.String(),
		common.LTCChain.String(),
		common.BCHChain.String(),
	}, nil)
	inputs := []struct {
		assetName string
		amount    cosmos.Uint
	}{
		{"ETH.ETH", cosmos.NewUint(2357498870)},
		{"ETH.DAI-0XAD6D458402F60FD3BD25163575031ACDCE07538D", cosmos.NewUint(380250359282)},
		{"ETH.MARS-0X9465DC5A988957CB56BE398D1F05A66F65170361", cosmos.NewUint(5747652585)},
		{"ETH.REP-0X6FD34013CDD2905D8D27B0ADAD5B97B2345CF2B8", cosmos.NewUint(1556246434)},
		{"ETH.UNI-0X71D82EB6A5051CFF99582F4CDF2AE9CD402A4882", cosmos.NewUint(548635445548)},
		{"ETH.USDT-0XA3910454BF2CB59B8B3A401589A3BACC5CA42306", cosmos.NewUint(10987963700)},
		{"ETH.XEENUS-0X7E0480CA9FD50EB7A3855CF53C347A1B4D6A2FF5", cosmos.NewUint(25626085749)},
		{"ETH.XRUNE-0X0FE3ECD525D16FA09AA1FF177014DE5304C835E2", cosmos.NewUint(3550865535196787)},
		{"ETH.XRUNE-0X8626DB1A4F9F3E1002EEB9A4F3C6D391436FFC23", cosmos.NewUint(16098175953548)},
		{"ETH.ZRX-0XE4C6182EA459E63B8F1BE7C428381994CCC2D49C", cosmos.NewUint(6732474974)},
		{"ETH.XRUNE-0XD9B37D046C543EB0E5E3EC27B86609E31DA205D7", cosmos.NewUint(10000000000)},
	}
	var coinsToSubtract common.Coins
	for _, item := range inputs {
		asset, err := common.NewAsset(item.assetName)
		if err != nil {
			ctx.Logger().Error("fail to parse asset", "error", err, "asset name", item.assetName)
			continue
		}
		coinsToSubtract = append(coinsToSubtract, common.NewCoin(asset, item.amount))
	}
	retiringVault.AddFunds(coinsToSubtract)
	c.Assert(mgr.Keeper().SetVault(ctx, retiringVault), IsNil)
	c.Assert(retiredVault.HasFunds(), Equals, false)
	storeMgr.migrateStoreV58(ctx, version, constantAccessor)
	vaultAfter, err := mgr.Keeper().GetVault(ctx, pubKey)
	c.Assert(err, IsNil)
	c.Assert(vaultAfter.Status.String(), Equals, RetiringVault.String())
	c.Assert(vaultAfter.HasFunds(), Equals, true)
	vaultAfter1, err := mgr.Keeper().GetVault(ctx, retiringPubKey)
	c.Assert(err, IsNil)
	c.Assert(vaultAfter1.HasFunds(), Equals, false)
}

func (s *StoreManagerTestSuite) TestMigrateStoreV58Refund(c *C) {
	ctx, mgr := setupManagerForTest(c)
	storeMgr := NewStoreMgr(mgr)
	vault := NewVault(1024, ActiveVault, AsgardVault, GetRandomPubKey(), []string{
		common.ETHChain.String(),
	}, nil)
	mkrToken, err := common.NewAsset("ETH.MKR-0X9F8F72AA9304C8B593D555F12EF6589CC3A579A2")
	c.Assert(err, IsNil)
	linkToken, err := common.NewAsset("ETH.LINK-0X514910771AF9CA656AF840DFF83E8264ECF986CA")
	c.Assert(err, IsNil)
	c.Assert(storeMgr.mgr.Keeper().SaveNetworkFee(ctx, common.ETHChain, NetworkFee{
		common.ETHChain, 80000, 30,
	}), IsNil)
	coins := common.NewCoins(
		common.NewCoin(mkrToken, cosmos.NewUint(60000000)),
		common.NewCoin(linkToken, cosmos.NewUint(140112242412)),
		common.NewCoin(common.ETHAsset, cosmos.NewUint(common.One)),
	)
	vault.AddFunds(coins)
	c.Assert(storeMgr.mgr.Keeper().SetVault(ctx, vault), IsNil)

	inTxID, err := common.NewTxID("6232075D4C63A69CDC8B65157A1737CBC4C1DA979BAA7E6F8B6B9F20A38388CA")
	c.Assert(err, IsNil)
	tx := common.NewTx(inTxID,
		"thor1vtklmng59728j5mzx0n0an4sek4kdcctxcgapk",
		"thor1g98cy3n9mmjrpn0sxmn63lztelera37n8n67c0",
		common.NewCoins(common.NewCoin(common.RuneNative, cosmos.NewUint(12200000000))),
		common.Gas{
			common.NewCoin(common.RuneNative, cosmos.NewUint(2000000)),
		}, "=:BNB.BNB:bnb136ns6lfw4zs5hg4n85vdthaad7hq5m4gtkgf23:242781111")
	observedTx := NewObservedTx(tx, 1281323, vault.PubKey, 1281323)
	voter := NewObservedTxVoter(inTxID, []ObservedTx{
		observedTx,
	})
	voter.Actions = []TxOutItem{
		{
			Chain:       common.BNBChain,
			ToAddress:   "bnb136ns6lfw4zs5hg4n85vdthaad7hq5m4gtkgf23",
			VaultPubKey: vault.PubKey,
			Coin:        common.NewCoin(common.BNBAsset, cosmos.NewUint(251892902)),
			Memo:        "OUT:6232075D4C63A69CDC8B65157A1737CBC4C1DA979BAA7E6F8B6B9F20A38388CA",
			InHash:      "6232075D4C63A69CDC8B65157A1737CBC4C1DA979BAA7E6F8B6B9F20A38388CA",
		},
	}
	voter.Tx = voter.Txs[0]
	storeMgr.mgr.Keeper().SetObservedTxInVoter(ctx, voter)

	inTxID1, err := common.NewTxID("05A7C2FD035FEC62BB957465BC80970177EBD80605138B7C4709333F118E7338")
	c.Assert(err, IsNil)
	tx1 := common.NewTx(inTxID1,
		"thor1dw4mztad7dxqx2aw3jlyatjdqcf30z3lzpjl3u",
		"thor1g98cy3n9mmjrpn0sxmn63lztelera37n8n67c0",
		common.NewCoins(common.NewCoin(common.RuneNative, cosmos.NewUint(34000000))),
		common.Gas{
			common.NewCoin(common.RuneNative, cosmos.NewUint(2000000)),
		}, "=:BNB.BNB:bnb136ns6lfw4zs5hg4n85vdthaad7hq5m4gtkgf23:633444")
	observedTx1 := NewObservedTx(tx1, 1261290, vault.PubKey, 1261290)
	voter1 := NewObservedTxVoter(inTxID, []ObservedTx{
		observedTx1,
	})
	voter.Actions = []TxOutItem{
		{
			Chain:       common.BNBChain,
			ToAddress:   "bnb136ns6lfw4zs5hg4n85vdthaad7hq5m4gtkgf23",
			VaultPubKey: vault.PubKey,
			Coin:        common.NewCoin(common.BNBAsset, cosmos.NewUint(707275)),
			Memo:        "OUT:05A7C2FD035FEC62BB957465BC80970177EBD80605138B7C4709333F118E7338",
			InHash:      "05A7C2FD035FEC62BB957465BC80970177EBD80605138B7C4709333F118E7338",
		},
	}
	voter1.Tx = voter1.Txs[0]
	storeMgr.mgr.Keeper().SetObservedTxInVoter(ctx, voter1)

	version := GetCurrentVersion()
	constantAccessor := constants.GetConstantValues(version)
	storeMgr.migrateStoreV58Refund(ctx, version, constantAccessor)
	afterVault, err := storeMgr.mgr.Keeper().GetVault(ctx, vault.PubKey)
	c.Assert(err, IsNil)
	c.Assert(afterVault.HasFunds(), Equals, true)
	items, err := mgr.TxOutStore().GetOutboundItems(ctx)
	c.Assert(err, IsNil)
	c.Assert(items, HasLen, 2)
	c.Assert(items[0].ToAddress.String(), Equals, "0x340b94e5369cEDe551a117960c75547eA84eAEdE")
	c.Assert(items[0].Coin.Asset.String(), Equals, "ETH.MKR-0X9F8F72AA9304C8B593D555F12EF6589CC3A579A2")
	c.Assert(items[0].Coin.Amount.Equal(cosmos.NewUint(60000000)), Equals, true)
	c.Assert(items[0].InHash.String(), Equals, "F8165C9D888C1ABD51EDDDF8B3DA9C8BCF6CDE4CACDCA15F3DF2D176332DCDD7")
	c.Assert(items[1].ToAddress.String(), Equals, "0x0749405611B77f94311576C6e80FAe69CfcCa41A")
	c.Assert(items[1].Coin.Asset.String(), Equals, "ETH.LINK-0X514910771AF9CA656AF840DFF83E8264ECF986CA")
	c.Assert(items[1].Coin.Amount.Equal(cosmos.NewUint(140112242412)), Equals, true)
	c.Assert(items[1].InHash.String(), Equals, "B8489F8A5BDFD39C899CC1987EB32E81490580F2FB6426CD4BC710E45C20B721")

	voterAfter, err := storeMgr.mgr.Keeper().GetObservedTxInVoter(ctx, inTxID)
	c.Assert(err, IsNil)
	txAfter := voterAfter.GetTx(NodeAccounts{})
	c.Assert(txAfter.IsDone(len(voterAfter.Actions)), Equals, true)

	voterAfter1, err := storeMgr.mgr.Keeper().GetObservedTxInVoter(ctx, inTxID1)
	c.Assert(err, IsNil)
	txAfter1 := voterAfter.GetTx(NodeAccounts{})
	c.Assert(txAfter1.IsDone(len(voterAfter1.Actions)), Equals, true)
}

func (s *StoreManagerTestSuite) TestRemoveTransactions(c *C) {
	ctx, mgr := setupManagerForTest(c)
	storeMgr := NewStoreMgr(mgr)
	vault := NewVault(1024, ActiveVault, AsgardVault, GetRandomPubKey(), []string{
		common.ETHChain.String(),
	}, nil)

	c.Assert(storeMgr.mgr.Keeper().SaveNetworkFee(ctx, common.ETHChain, NetworkFee{
		common.ETHChain, 80000, 30,
	}), IsNil)

	inTxID, err := common.NewTxID("BC68035CE2C8A2C549604FF7DB59E07931F39040758B138190338FA697338DB3")
	c.Assert(err, IsNil)
	tx := common.NewTx(inTxID,
		"0x3a196410a0f5facd08fd7880a4b8551cd085c031",
		"0xf56cBa49337A624E94042e325Ad6Bc864436E370",
		common.NewCoins(common.NewCoin(common.ETHAsset, cosmos.NewUint(200*common.One))),
		common.Gas{
			common.NewCoin(common.RuneNative, cosmos.NewUint(2000000)),
		}, "SWAP:ETH.AAVE-0x7Fc66500c84A76Ad7e9c93437bFc5Ac33E2DDaE9")
	observedTx := NewObservedTx(tx, 1281323, vault.PubKey, 1281323)
	voter := NewObservedTxVoter(inTxID, []ObservedTx{
		observedTx,
	})
	aaveAsset, _ := common.NewAsset("ETH.AAVE-0x7Fc66500c84A76Ad7e9c93437bFc5Ac33E2DDaE9")
	voter.Actions = []TxOutItem{
		{
			Chain:       common.ETHChain,
			ToAddress:   "0x3a196410a0f5facd08fd7880a4b8551cd085c031",
			VaultPubKey: vault.PubKey,
			Coin:        common.NewCoin(aaveAsset, cosmos.NewUint(1422136902)),
			Memo:        "OUT:BC68035CE2C8A2C549604FF7DB59E07931F39040758B138190338FA697338DB3",
			InHash:      "BC68035CE2C8A2C549604FF7DB59E07931F39040758B138190338FA697338DB3",
		},
		{
			Chain:       common.ETHChain,
			ToAddress:   "0x3a196410a0f5facd08fd7880a4b8551cd085c031",
			VaultPubKey: vault.PubKey,
			Coin:        common.NewCoin(aaveAsset, cosmos.NewUint(1330195098)),
			Memo:        "OUT:BC68035CE2C8A2C549604FF7DB59E07931F39040758B138190338FA697338DB3",
			InHash:      "BC68035CE2C8A2C549604FF7DB59E07931F39040758B138190338FA697338DB3",
		},
	}
	voter.Tx = voter.Txs[0]
	storeMgr.mgr.Keeper().SetObservedTxInVoter(ctx, voter)
	hegicAsset, _ := common.NewAsset("ETH.HEGIC-0x584bC13c7D411c00c01A62e8019472dE68768430")
	inTxID1, err := common.NewTxID("5DA19C1C5C2F6BBDF9D4FB0E6FF16A0DF6D6D7FE1F8E95CA755E5B3C6AADA458")
	c.Assert(err, IsNil)
	tx1 := common.NewTx(inTxID1,
		"0x3a196410a0f5facd08fd7880a4b8551cd085c031",
		"0xf56cBa49337A624E94042e325Ad6Bc864436E370",
		common.NewCoins(common.NewCoin(common.ETHAsset, cosmos.NewUint(200*common.One))),
		common.Gas{
			common.NewCoin(common.RuneNative, cosmos.NewUint(2000000)),
		}, "SWAP:ETH.HEGIC-0x584bC13c7D411c00c01A62e8019472dE68768430")
	observedTx1 := NewObservedTx(tx1, 1281323, vault.PubKey, 1281323)
	voter1 := NewObservedTxVoter(inTxID1, []ObservedTx{
		observedTx1,
	})
	voter1.Actions = []TxOutItem{
		{
			Chain:       common.ETHChain,
			ToAddress:   "0x3a196410a0f5facd08fd7880a4b8551cd085c031",
			VaultPubKey: vault.PubKey,
			Coin:        common.NewCoin(hegicAsset, cosmos.NewUint(3083783295390)),
			Memo:        "OUT:5DA19C1C5C2F6BBDF9D4FB0E6FF16A0DF6D6D7FE1F8E95CA755E5B3C6AADA458",
			InHash:      "5DA19C1C5C2F6BBDF9D4FB0E6FF16A0DF6D6D7FE1F8E95CA755E5B3C6AADA458",
		},
		{
			Chain:       common.ETHChain,
			ToAddress:   "0x3a196410a0f5facd08fd7880a4b8551cd085c031",
			VaultPubKey: vault.PubKey,
			Coin:        common.NewCoin(hegicAsset, cosmos.NewUint(2481151780248)),
			Memo:        "OUT:5DA19C1C5C2F6BBDF9D4FB0E6FF16A0DF6D6D7FE1F8E95CA755E5B3C6AADA458",
			InHash:      "5DA19C1C5C2F6BBDF9D4FB0E6FF16A0DF6D6D7FE1F8E95CA755E5B3C6AADA458",
		},
	}
	voter1.Tx = voter.Txs[0]
	storeMgr.mgr.Keeper().SetObservedTxInVoter(ctx, voter1)

	inTxID2, err := common.NewTxID("D58D1EF6D6E49EB99D0524128C16115893396FD05877EF4856FCE474B5BA09A7")
	c.Assert(err, IsNil)
	tx2 := common.NewTx(inTxID2,
		"0x3a196410a0f5facd08fd7880a4b8551cd085c031",
		"0xf56cBa49337A624E94042e325Ad6Bc864436E370",
		common.NewCoins(common.NewCoin(common.ETHAsset, cosmos.NewUint(150005145000))),
		common.Gas{
			common.NewCoin(common.RuneNative, cosmos.NewUint(2000000)),
		}, "SWAP:ETH.ETH")
	observedTx2 := NewObservedTx(tx2, 1281323, vault.PubKey, 1281323)
	voter2 := NewObservedTxVoter(inTxID2, []ObservedTx{
		observedTx2,
	})
	voter2.Actions = []TxOutItem{
		{
			Chain:       common.ETHChain,
			ToAddress:   "0x3a196410a0f5facd08fd7880a4b8551cd085c031",
			VaultPubKey: vault.PubKey,
			Coin:        common.NewCoin(hegicAsset, cosmos.NewUint(150003465000)),
			Memo:        "REFUND:D58D1EF6D6E49EB99D0524128C16115893396FD05877EF4856FCE474B5BA09A7",
			InHash:      "D58D1EF6D6E49EB99D0524128C16115893396FD05877EF4856FCE474B5BA09A7",
		},
	}
	voter2.Tx = voter.Txs[0]
	storeMgr.mgr.Keeper().SetObservedTxInVoter(ctx, voter2)

	allTxIDs := []common.TxID{
		inTxID, inTxID1, inTxID2,
	}
	storeMgr.removeTransactions(ctx, inTxID.String(), inTxID1.String(), inTxID2.String())
	for _, txID := range allTxIDs {
		voterAfter, err := storeMgr.mgr.Keeper().GetObservedTxInVoter(ctx, txID)
		c.Assert(err, IsNil)
		txAfter := voterAfter.GetTx(NodeAccounts{})
		c.Assert(txAfter.IsDone(len(voterAfter.Actions)), Equals, true)
	}
}
func (s *StoreManagerTestSuite) TestCreditBackToVaultAndPool(c *C) {
	ctx, mgr := setupManagerForTest(c)
	storeMgr := NewStoreMgr(mgr)
	vault := NewVault(1024, ActiveVault, AsgardVault, GetRandomPubKey(), []string{
		common.ETHChain.String(),
	}, nil)
	vault.AddFunds(common.NewCoins(
		common.NewCoin(common.ETHAsset, cosmos.NewUint(1339348406651)),
	))
	c.Assert(storeMgr.mgr.Keeper().SetVault(ctx, vault), IsNil)
	c.Assert(storeMgr.mgr.Keeper().SaveNetworkFee(ctx, common.ETHChain, NetworkFee{
		common.ETHChain, 80000, 30,
	}), IsNil)

	assetsForTest := map[string][2]cosmos.Uint{
		"ETH.ETH": {cosmos.NewUint(1006103676112), cosmos.NewUint(70306321826847)},
		"ETH.AAVE-0X7FC66500C84A76AD7E9C93437BFC5AC33E2DDAE9":  {cosmos.NewUint(64528597541), cosmos.NewUint(8356937728557)},
		"ETH.HEGIC-0X584BC13C7D411C00C01A62E8019472DE68768430": {cosmos.NewUint(115332831324359), cosmos.NewUint(5823770298376)},
		"ETH.YFI-0X0BC529C00C6401AEF6D220BE8C6EA1667F6AD93E":   {cosmos.NewUint(837597381), cosmos.NewUint(43408026406467)},
		"ETH.SUSHI-0X6B3595068778DD592E39A122F4F5A5CF09C90FE2": {cosmos.NewUint(6227149236123), cosmos.NewUint(47444039861851)},
		"ETH.KYL-0X67B6D479C7BB412C54E03DCA8E1BC6740CE6B99C":   {cosmos.NewUint(244452777487314), cosmos.NewUint(17322656702203)},
		"ETH.DODO-0X43DFC4159D86F3A37A5A4B3D4580B888AD7D4DDD":  {cosmos.NewUint(36302299741717), cosmos.NewUint(17608104987305)},
	}

	for k, item := range assetsForTest {
		asset, err := common.NewAsset(k)
		c.Assert(err, IsNil)
		pool, err := mgr.Keeper().GetPool(ctx, asset)
		c.Assert(err, IsNil)
		pool.Asset = asset
		pool.Status = PoolAvailable
		pool.BalanceAsset = item[0]
		pool.BalanceRune = item[1]
		c.Assert(mgr.Keeper().SetPool(ctx, pool), IsNil)
	}
	storeMgr.creditAssetBackToVaultAndPool(ctx)

	vaultAfter, err := mgr.Keeper().GetVault(ctx, vault.PubKey)
	c.Assert(err, IsNil)
	c.Assert(vaultAfter.GetCoin(common.ETHAsset).Amount.Equal(cosmos.NewUint(43465695171)), Equals, true)
	ethPoolAfter, err := mgr.Keeper().GetPool(ctx, common.ETHAsset)
	c.Assert(err, IsNil)
	c.Assert(ethPoolAfter.BalanceAsset.Equal(cosmos.NewUint(76228226137)), Equals, true)
	c.Assert(ethPoolAfter.BalanceRune.GT(cosmos.NewUint(70306321826847)), Equals, true)
}
func (s *StoreManagerTestSuite) TestMigrateStoreV61(c *C) {
	ctx, mgr := setupManagerForTest(c)
	ctx = ctx.WithBlockHeight(1024)
	storeMgr := NewStoreMgr(mgr)
	txOut := NewTxOut(ctx.BlockHeight())
	ethAddr, err := GetRandomPubKey().GetAddress(common.ETHChain)
	c.Assert(err, IsNil)
	blockHeight := common.BlockHeight(ctx)
	txOut.TxArray = []TxOutItem{
		{
			Chain:       common.ETHChain,
			ToAddress:   ethAddr,
			VaultPubKey: GetRandomPubKey(),
			Coin: common.Coin{
				Asset:    common.ETHAsset,
				Amount:   cosmos.NewUint(1024),
				Decimals: 0,
			},
			Memo:    NewOutboundMemo(GetRandomTxHash()).String(),
			GasRate: 88,
			InHash:  GetRandomTxHash(),
		},
		{
			Chain:       common.BNBChain,
			ToAddress:   GetRandomBNBAddress(),
			VaultPubKey: GetRandomPubKey(),
			Coin: common.Coin{
				Asset:    common.BNBAsset,
				Amount:   cosmos.NewUint(1024),
				Decimals: 0,
			},
			Memo:    NewOutboundMemo(GetRandomTxHash()).String(),
			GasRate: 88,
			InHash:  GetRandomTxHash(),
		},
		{
			Chain:       common.ETHChain,
			ToAddress:   ethAddr,
			VaultPubKey: GetRandomPubKey(),
			Coin: common.Coin{
				Asset:    common.ETHAsset,
				Amount:   cosmos.NewUint(1024),
				Decimals: 0,
			},
			Memo:    NewRefundMemo(GetRandomTxHash()).String(),
			GasRate: 88,
			InHash:  GetRandomTxHash(),
		},
	}
	c.Assert(mgr.Keeper().SetTxOut(ctx, txOut), IsNil)
	ctx = ctx.WithBlockHeight(ctx.BlockHeight() + 100)
	storeMgr.migrateStoreV61(ctx, GetCurrentVersion(), mgr.ConstAccessor)
	txOutAfter, err := mgr.Keeper().GetTxOut(ctx, blockHeight)
	c.Assert(err, IsNil)
	c.Assert(txOutAfter.TxArray[0].OutHash.IsEmpty(), Equals, false)
	c.Assert(txOutAfter.TxArray[1].OutHash.IsEmpty(), Equals, true)
	c.Assert(txOutAfter.TxArray[2].OutHash.IsEmpty(), Equals, true)
}
