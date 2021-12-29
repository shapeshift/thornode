package thorchain

import (
	"encoding/json"
	"os"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/x/thorchain/types"
	. "gopkg.in/check.v1"
)

type StoreManagerTestSuite struct{}

var _ = Suite(&StoreManagerTestSuite{})

func (s *StoreManagerTestSuite) TestMigrateStoreV55(c *C) {
	ctx, mgr := setupManagerForTest(c)
	storeMgr := newStoreMgr(mgr)
	keeper := storeMgr.mgr.Keeper()
	assetToAdjust, err := common.NewAsset("BNB.USDT-6D8")
	c.Assert(err, IsNil)
	pool := NewPool()
	pool.Asset = assetToAdjust
	c.Assert(keeper.SetPool(ctx, pool), IsNil)
	migrateStoreV55(ctx, mgr)
	newPool, err := keeper.GetPool(ctx, assetToAdjust)
	c.Assert(err, IsNil)
	c.Assert(newPool.BalanceAsset.Equal(cosmos.NewUint(900000000)), Equals, true)
}

func (s *StoreManagerTestSuite) TestMigrateStoreV58(c *C) {
	SetupConfigForTest()
	ctx, mgr := setupManagerForTest(c)
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
	migrateStoreV58(ctx, mgr)
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
	storeMgr := newStoreMgr(mgr)
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

	migrateStoreV58Refund(ctx, mgr)
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
	storeMgr := newStoreMgr(mgr)
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
	removeTransactions(ctx, mgr, inTxID.String(), inTxID1.String(), inTxID2.String())
	for _, txID := range allTxIDs {
		voterAfter, err := storeMgr.mgr.Keeper().GetObservedTxInVoter(ctx, txID)
		c.Assert(err, IsNil)
		txAfter := voterAfter.GetTx(NodeAccounts{})
		c.Assert(txAfter.IsDone(len(voterAfter.Actions)), Equals, true)
	}
}
func (s *StoreManagerTestSuite) TestCreditBackToVaultAndPool(c *C) {
	ctx, mgr := setupManagerForTest(c)
	storeMgr := newStoreMgr(mgr)
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
	creditAssetBackToVaultAndPool(ctx, mgr)

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
	txOut := NewTxOut(ctx.BlockHeight())
	ethAddr, err := GetRandomPubKey().GetAddress(common.ETHChain)
	c.Assert(err, IsNil)
	blockHeight := common.BlockHeight(ctx)
	erc20Asset, err := common.NewAsset("ETH.USDC-0XA0B86991C6218B36C1D19D4A2E9EB0CE3606EB48")
	c.Assert(err, IsNil)
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
		{
			Chain:       common.ETHChain,
			ToAddress:   ethAddr,
			VaultPubKey: GetRandomPubKey(),
			Coin: common.Coin{
				Asset:    erc20Asset,
				Amount:   cosmos.NewUint(1024),
				Decimals: 0,
			},
			Memo:    NewOutboundMemo(GetRandomTxHash()).String(),
			GasRate: 88,
			InHash:  GetRandomTxHash(),
		},
	}
	c.Assert(mgr.Keeper().SetTxOut(ctx, txOut), IsNil)
	ctx = ctx.WithBlockHeight(ctx.BlockHeight() + 100)
	purgeETHOutboundQueue(ctx, mgr)
	txOutAfter, err := mgr.Keeper().GetTxOut(ctx, blockHeight)
	c.Assert(err, IsNil)
	c.Assert(txOutAfter.TxArray[0].OutHash.IsEmpty(), Equals, false)
	c.Assert(txOutAfter.TxArray[1].OutHash.IsEmpty(), Equals, true)
	c.Assert(txOutAfter.TxArray[2].OutHash.IsEmpty(), Equals, true)
	c.Assert(txOutAfter.TxArray[3].OutHash.IsEmpty(), Equals, true)
	c.Assert(txOutAfter.TxArray[3].ToAddress.String(), Equals, temporaryUSDTHolder)
}

func (s *StoreManagerTestSuite) TestCorrectAsgardVaultBalance(c *C) {
	ctx, mgr := setupManagerForTest(c)
	SetupConfigForTest()
	ctx = ctx.WithBlockHeight(1024)
	vault := NewVault(ctx.BlockHeight(), ActiveVault, AsgardVault, GetRandomPubKey(), []string{
		common.BTCChain.String(),
		common.ETHChain.String(),
		common.BNBChain.String(),
		common.BCHChain.String(),
		common.LTCChain.String(),
	}, nil)
	c.Assert(mgr.Keeper().SetVault(ctx, vault), IsNil)
	correctAsgardVaultBalanceV61(ctx, mgr, vault.PubKey)
	afterVault, err := mgr.Keeper().GetVault(ctx, vault.PubKey)
	c.Assert(err, IsNil)
	expectedAssets := []struct {
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
	for _, item := range expectedAssets {
		asset, err := common.NewAsset(item.name)
		if err != nil {
			ctx.Logger().Error("fail to parse asset", "asset", item.name, "error", err)
			continue
		}
		coin := common.NewCoin(asset, item.amount)
		c.Assert(afterVault.Coins.Contains(coin), Equals, true)
	}
}

func (s *StoreManagerTestSuite) TestRefundBinanceTx(c *C) {
	c.Assert(os.Setenv("NET", "mainnet"), IsNil)
	ctx, mgr := setupManagerForTest(c)
	SetupConfigForTest()
	config := cosmos.GetConfig()
	config.SetBech32PrefixForAccount("thor", "thorpub")
	ctx = ctx.WithBlockHeight(1024)
	pubKey, err := common.NewPubKey(`thorpub1addwnpepqdr4386mnkqyqzpqlydtat0k82f8xvkfwzh4xtjc84cuaqmwx5vjvgnf6v5`)
	c.Assert(err, IsNil)
	vault := NewVault(ctx.BlockHeight(), ActiveVault, AsgardVault, pubKey, []string{
		common.BTCChain.String(),
		common.ETHChain.String(),
		common.BNBChain.String(),
		common.BCHChain.String(),
		common.LTCChain.String(),
	}, nil)

	rawAsgardCoins := `[
            {
                "asset": "BNB.AVA-645",
                "amount": "4744823210393"
            },
            {
                "asset": "BNB.BNB",
                "amount": "1195436423971"
            },
            {
                "asset": "BNB.BTCB-1DE",
                "amount": "4379958779"
            },
            {
                "asset": "BNB.BUSD-BD1",
                "amount": "541359868907707"
            },
            {
                "asset": "BNB.CAS-167",
                "amount": "0"
            },
            {
                "asset": "BNB.ETH-1C9",
                "amount": "52467800398"
            },
            {
                "asset": "BNB.TWT-8C2",
                "amount": "41233825418279"
            },
            {
                "asset": "BCH.BCH",
                "amount": "91883387729"
            },
            {
                "asset": "LTC.LTC",
                "amount": "541616600355"
            },
            {
                "asset": "BTC.BTC",
                "amount": "31029602086"
            },
            {
                "asset": "ETH.ETH",
                "amount": "112291405"
            },
            {
                "asset": "BNB.FTM-A64",
                "amount": "1363812542681"
            },
            {
                "asset": "BNB.BAT-07A",
                "amount": "117628878079"
            },
            {
                "asset": "BNB.USDT-6D8",
                "amount": "45309922539"
            },
            {
                "asset": "ETH.ALCX-0XDBDB4D16EDA451D0503B854CF79D55697F90C8DF",
                "amount": "107457767154"
            },
            {
                "asset": "ETH.SUSHI-0X6B3595068778DD592E39A122F4F5A5CF09C90FE2",
                "amount": "6238104630646"
            },
            {
                "asset": "ETH.USDC-0XA0B86991C6218B36C1D19D4A2E9EB0CE3606EB48",
                "amount": "185885061012400",
                "decimals": 6
            },
            {
                "asset": "ETH.YFI-0X0BC529C00C6401AEF6D220BE8C6EA1667F6AD93E",
                "amount": "768934424"
            },
            {
                "asset": "ETH.RAZE-0X5EAA69B29F99C84FE5DE8200340B4E9B4AB38EAC",
                "amount": "34234116974195"
            },
            {
                "asset": "ETH.CREAM-0X2BA592F78DB6436527729929AAF6C908497CB200",
                "amount": "44893217485"
            },
            {
                "asset": "ETH.KYL-0X67B6D479C7BB412C54E03DCA8E1BC6740CE6B99C",
                "amount": "148648933061840"
            },
            {
                "asset": "ETH.ALPHA-0XA1FAA113CBE53436DF28FF0AEE54275C13B40975",
                "amount": "12935153461169"
            },
            {
                "asset": "ETH.HEGIC-0X584BC13C7D411C00C01A62E8019472DE68768430",
                "amount": "49670952325086"
            },
            {
                "asset": "ETH.HOT-0X6C6EE5E31D828DE241282B9606C8E98EA48526E2",
                "amount": "2347933226238794"
            },
            {
                "asset": "ETH.VIU-0X519475B31653E46D20CD09F9FDCF3B12BDACB4F5",
                "amount": "6025838"
            },
            {
                "asset": "ETH.DODO-0X43DFC4159D86F3A37A5A4B3D4580B888AD7D4DDD",
                "amount": "17695835201961"
            },
            {
                "asset": "ETH.DNA-0XEF6344DE1FCFC5F48C30234C16C1389E8CDC572C",
                "amount": "0"
            },
            {
                "asset": "BNB.RUNE-B1A",
                "amount": "974882206399614"
            },
            {
                "asset": "ETH.PERP-0XBC396689893D065F41BC2C6ECBEE5E0085233447",
                "amount": "2006916325197"
            },
            {
                "asset": "ETH.SNX-0XC011A73EE8576FB46F5E1C5751CA3B9FE0AF2A6F",
                "amount": "863948962240"
            },
            {
                "asset": "ETH.RUNE-0X3155BA85D5F96B2D030A4966AF206230E46849CB",
                "amount": "39514577925536"
            },
            {
                "asset": "ETH.AAVE-0X7FC66500C84A76AD7E9C93437BFC5AC33E2DDAE9",
                "amount": "48307773434"
            },
            {
                "asset": "ETH.USDT-0XDAC17F958D2EE523A2206206994597C13D831EC7",
                "amount": "110036265191500",
                "decimals": 6
            },
            {
                "asset": "ETH.XRUNE-0X69FA0FEE221AD11012BAB0FDB45D444D3D2CE71C",
                "amount": "2318606453983113"
            }
        ]`
	var coins common.Coins
	if err := json.Unmarshal([]byte(rawAsgardCoins), &coins); err != nil {
		panic(err)
	}
	vault.AddFunds(coins)
	c.Assert(mgr.Keeper().SetVault(ctx, vault), IsNil)

	refunds := []struct {
		hash   string
		addr   string
		amount float64
		asset  string
	}{
		{"7F6EC99B8F82F36501DCCDDBF54E9BBEAA51A8604CD39E9635027711967B2DF9", "bnb1h05y4pe6heq2g8u6955kchrdst7s8gxspg3u2s", 628, "BNB.RUNE-B1A"},
		{"2EF95165FDAABBF72CA4EE10DBAEF825EE06C280B8B2098AB55A89B922105C57", "bnb1uresxjchzzffvdvjgkf2su7kcupx5uycc58ygs", 34.166, "BNB.RUNE-B1A"},
		{"1D8DE0EC576D7C9A485ECA74401560CBA65870F9E07DF814257B24C0EC8165FC", "bnb1kcay57v22f6gn5kf5vsnstxv8dfnaz2gecckeg", 47.477, "BNB.RUNE-B1A"},
		{"EA6627C95264B150DB729D2BFBD9C52B91300A5118C404F519244C07E9A0A4C1", "bnb1zxtn3xt5prfd7w3st53jjnkeuexkyn7uh8a24c", 126.10821, "BNB.RUNE-B1A"},
		{"6FFFF126170C6B4A144121DD5E5E9A195B9355F083C5494C4C9BF2DA855B9739", "bnb1kcay57v22f6gn5kf5vsnstxv8dfnaz2gecckeg", 251.082, "BNB.RUNE-B1A"},
		{"37E64B6DF40541E9C77107C29BBE3A03BD051EA072ED8CBDC2CC51CF3AD2BF59", "bnb1rvzk2wkn6f8k447s63tf6yuk52tef4m0t79vn0", 50.84107, "BNB.RUNE-B1A"},
		{"B1D6CC11BC34E680362E26CDE8628444222D5AEA30520EF1A38BF3B1BB63C246", "bnb1km5j6s27w5zr9677p3nydv7gq8xemh53g6keeq", 100, "BNB.RUNE-B1A"},
		{"B57C28C425EFA75560C32CA6EBB9DC2BE86EE118837B33840008023808CD3583", "bnb1ljkguffu227zwvz29quzn7aj6a3wdf3nc4rffz", 641.09822, "BNB.RUNE-B1A"},
		{"4030559AA45D78422296581D6C352FA90B51F913DF216E953B0B044F5107A17E", "bnb1zptk7ftjmessmw06q2qg4te2d2cd54973wthye", 2.846, "BNB.RUNE-B1A"},
		{"2C6FA678407506A4CC8F8C8CC1EC07511C6228120B963A2FDDD4A22E67611AA6", "bnb1zptk7ftjmessmw06q2qg4te2d2cd54973wthye", 0.106, "BNB.RUNE-B1A"},
		{"A77AE682E0244D3203A31BF2AAD9DE317742B0D38BED40162E093EA5B3411233", "bnb1n63mqfexeqj5rdc08ww4jyu6ys5k3s2h8n95tj", 0.04983093, "BNB.BNB"},
		{"189BD06D18EE322D5662BF6AD5F18AF44286FD71985EE8408B91726EF59F29A0", "bnb1q3wkg6p2plf88342qtxe943qsnrhacp7f2f364", 498.446, "BNB.RUNE-B1A"},
		{"EAF6B55E6718BA0E0E81F425DB34E4CA6E9C36D3CE16562362DF2FC57FD3E65E", "bnb1mfz43ul3njltlgfhn976l88a2fd0uvcm2xyl6p", 1, "BNB.BUSD-BD1"},
		{"9D3C914F4049101410A86298E471E5C3C113DF45568087B362A2376C7575663B", "bnb1ukfhdgy9c0ws42eqt786q82ept9ht6fag88t6u", 0.01, "BNB.BNB"},
		{"2243A8F3938031C485506955D1F5170EB3D7C6226444AE37F122D29194CB9208", "bnb173mra2em8zx8t5frf8t39jdmh0fgtcgnauea9g", 249.9875878, "BNB.BUSD-BD1"},
		{"E4546065A5E1F7D0D69951E3990FE9333CCFB34AA14FFAE4A6387F080C15F7E4", "bnb173mra2em8zx8t5frf8t39jdmh0fgtcgnauea9g", 249.9993278, "BNB.BUSD-BD1"},
		{"E3A23F59F02AF1F2FA312A92565C28DCB468133A164719DF573B7CA6D8791138", "bnb173mra2em8zx8t5frf8t39jdmh0fgtcgnauea9g", 129.9999985, "BNB.BUSD-BD1"},
		{"61537E93C783E38E1F66A9C830F25D38414CC5A82DEAD6AB353F9F724C7E3C6F", "bnb173mra2em8zx8t5frf8t39jdmh0fgtcgnauea9g", 249.9875878, "BNB.BUSD-BD1"},
		{"62DC14CB8617786AC9A67E4C0BAD9322883AF4CEFF4568458AC3577D46CD01C2", "bnb173mra2em8zx8t5frf8t39jdmh0fgtcgnauea9g", 119.9999999, "BNB.BUSD-BD1"},
		{"05FADCD5E58368F242F11A664527B85E439381F851E2E4977C94455502E3B785", "bnb173mra2em8zx8t5frf8t39jdmh0fgtcgnauea9g", 249.9993278, "BNB.BUSD-BD1"},
		{"F1ADEDDEFC52659FBE5A473A008E1D4D796278B9B08A1B6EFFC16DB504E031F3", "bnb1e0ynwm4qyvpqzdv7hx20trssjx3cx6jpcv9rm8", 0.10178468, "BNB.BNB"},
		{"E72C7D0E789164558B19372D60C4E75314E5BF6BB7A0515F9F63837E09AD2211", "bnb12d0p6s47l5tr7xs0qedu7v300dc0glc3vcstx7", 0.2991625, "BNB.BNB"},
		{"8EA505016F51A3784246E625D782121CF7B1CDFD6D9761CCF257A16C7E469386", "bnb1s9hznv329sa44hd3fqc2tkt34mwn9lsfydvl3l", 29.7763598, "BNB.RUNE-B1A"},
		{"52EE48D2AB26E3019D94468B05D55093FA067CC8DAE161621C8F7C16DA1F7125", "bnb1ps98jc7ahv26ystenqhuh24fmxyv7t9zzc85xh", 17.5, "BNB.RUNE-B1A"},
		{"E2B18B54AD30B0EF66DF430E3F640585EE6006BBC04461AA787D9F8FA3776DD5", "bnb12ahp8vpwhe8yw86n33clsvy5z2vgq8fl0pgcfy", 24.4632, "BNB.RUNE-B1A"},
		{"117B04B214A4F97CF4C911B0150B01A8AF0AF56FA2EF919700A556ACD2C20523", "bnb1vukqle2pzckhd5jpxm8vp3yw8l2wzz9rwg9st4", 59.95595267, "BNB.BUSD-BD1"},
		{"047C6698FB19A0CCD4404AD29CB074B787418B64C0502C8BCE210AF8ADD51213", "bnb19w5yazzc9cmfcjf7fm35u5l7t5npgu673936ht", 156.1715529, "BNB.RUNE-B1A"},
		{"26FB54195FDA0D4200A3510AD7939FF8DE02DAFDB7F4FB250B51BA3A773CE48D", "bnb1afwzv54gff0e62t7wx9hl9eng4qk0ajqfgh62g", 5609.330992, "BNB.RUNE-B1A"},
		{"30B5E211A1C6DBC1B7C939490B2A19605192472BB71C3C21CE7EA960C40808DD", "bnb1n8v8grqlq22h70s87g2rc9dhp3mddughpx8c6j", 0.1498125, "BNB.BNB"},
		{"0F8157A253B9231C860DFB4F71EEE0D3553C7A2DFA4D4C9D89AC5F33EE78B65C", "bnb1n8v8grqlq22h70s87g2rc9dhp3mddughpx8c6j", 514.2599, "BNB.RUNE-B1A"},
		{"952976E7AF4F9017CEBC2FC0C37F2F815A85CE5B67D1E485405D89197EA4F760", "bnb14a3sppfhumq64fvfveatfd5gqfg6ggpjr9q9xm", 0.108, "BNB.BNB"},
		{"0779BD5785C0E27EA0FBC39965C21182A57697F5091CC14E50E6FA3FF418E5FC", "bnb1524d9an2cm8zhts5zrs9auty8dnke4cnc2x72s", 14.266, "BNB.RUNE-B1A"},
		{"138493B9E8240C134C3FBE4FE2C2DB3334990E88145B7BEC7E91B8887011F750", "bnb12d0p6s47l5tr7xs0qedu7v300dc0glc3vcstx7", 3.9490056, "BNB.RUNE-B1A"},
		{"4A4C867E21F8041B920BE0B8C18F3A7981EE4C4B5813936A3831E4FFA0DD25AE", "bnb1chv7uxr79hs7nuj48s2dfh3eeyvw9gnp7qpqdv", 80, "BNB.RUNE-B1A"},
		{"57F4E20E2A0AA6548DB10988B2EA68EBFD4384B7D3EC99F9A17E2B20E666598F", "bnb1en9lzcakhnudhdqpk5t7dqh3atncfu36mj4g7z", 100.03134, "BNB.RUNE-B1A"},
		{"933818BA5A4F27CA3309D53B22CA5AD26277AE11EC135773CD4AF1F3BB4F2FA9", "bnb1agx9jgj54e3f3cyfun2wlv9yty0hxdm89gad7v", 252.6842, "BNB.RUNE-B1A"},
		{"9CA45BFC2E42A24B2779A626574481A5127157DBC81D417AC8F4FD024817976D", "bnb1dht3y9gm200n9h7pjlynl74t3kfdqg55qtux5r", 11.1, "BNB.RUNE-B1A"},
		{"686EDE578BA46FFA9EFEAC9E1667DB95F535E31D14CE0BAFBEA798442672A103", "bnb1jw620r89kk8ueg0wpfmra2lhh3c3473jjuz6x2", 173.21, "BNB.RUNE-B1A"},
		{"18C65B412960B2482F38196390080AD87F9236ECA85A67C59A7ED211590544CE", "bnb1dyysfz2ecs20p33spvmk36fuyhqje9dac04299", 5.957, "BNB.RUNE-B1A"},
		{"6B8FD3FCCE475476B7FEA886BDE950DF107A12A9A27B6F32AA338F52782AF2F2", "bnb1ejn27zjq8zehjpvqs0856845xk00ypu62g5tuy", 400.526055, "BNB.RUNE-B1A"},
		{"DA0C266AE094F1F75142057633EBEE2A99389351DE9A647780C7E96FC10C2898", "bnb1qseefdrdskdy0lhgd9jns9tt0l280xtftrgvpg", 36.002, "BNB.RUNE-B1A"},
		{"FF0D24AA6E58BA3A53C2BB14D6260734F4D6E38DD7579EFFAEA5F586897309EB", "bnb1eqxsxe0yqnrq68k6quunqfaalqwqyg52yykut8", 100.5736732, "BNB.RUNE-B1A"},
		{"48D5DCCCE9743306DAD1BE66E2A5E87D315F4E4F760225E775E4F40F841C8667", "bnb1cxx3l6j50pkcmse4rxeg7lu5rtyjn0flfl7da9", 15.702, "BNB.RUNE-B1A"},
		{"24B6EDE1841115D2BE287AE2A7756D9C406593A555BB7CD29B6D1EE456003B48", "bnb1eqfyhkumpzxplsdjjudj302nktj320jj4x0fcd", 760.861, "BNB.RUNE-B1A"},
		{"E3D464210A41A874DEEC525FAE791C0AD22326124CFD1B9118F768AD93B98B80", "bnb1dp409mu2kc5frgkytnn7lxrzc5855e9hdmucun", 0.5, "BNB.RUNE-B1A"},
		{"28F3B39ACFD2AD0DDFCFA8C53F193515F7B0E3C9DC9B12974D0588A54DC3DF19", "bnb1vrugu3yq4s5zal29v6sn9pavpfjwzwp3sezjp4", 30001.73045, "BNB.RUNE-B1A"},
		{"805D99B5876E7CCECEF3C70FB8FF71CC8B6E888DACF311D35CE8A75CED87827D", "bnb1pt38g0xddfrkufv554dexuq9a98f843t580cnr", 16.553353, "BNB.RUNE-B1A"},
		{"2EF665B8A35FBF224431CFFC2D5E454262F9B30B78F63A7D92E9AB5302B6E902", "bnb1gufh8sp5kaqsetmwg9j6x5rfp6gzxkk6hngzdp", 543.0727504, "BNB.RUNE-B1A"},
		{"34C91917D01158E335736C37A8944A56B8FFD7F222C416F3A4A175369CA98268", "bnb1pqmlr4mewzl72w6r26mmnhqfzgwhk0cu3se6la", 1096.223315, "BNB.RUNE-B1A"},
		{"0169142021C2A82F608872404C02F04513AA61BE4F422D641ADEC70D47384AF9", "bnb129f9dvzkla80v90vgt7e5r84gvea26c9cz5pyd", 2.9, "BNB.RUNE-B1A"},
		{"BF8A81481F56A0C95009B6CB021A52B8F4F19AF6FBE9CC3EA5CE0F6E600C2DA1", "bnb1chnqh4vvf0hqyymkcfcrf4grq88f9ks3yr9v2h", 1, "BNB.BUSD-BD1"},
		{"4E5EDBED7DD3515065E92842AD8E89121AB05F77F57B275C2AE49E96B0EB7815", "bnb19424lc6n639lspmshjcqa3e4g3xcdfhlmvtyhv", 99.9, "BNB.RUNE-B1A"},
		{"906B60C8BD5ECF2578438C3165130DA90BD240D638E45278C211A9B0DAD39101", "bnb1h7rdk6pe9l2w0mt64m5axjvtylrave4f87g9nr", 8.4433734, "BNB.RUNE-B1A"},
		{"AA12CB4BC2D0DAF928C220E9E9AC614FFA1A5DD6AF958CBD16269E8E891B8E23", "bnb145yyrd783ye42c699qhp0l9ke7vq6n4j7kcgvd", 1441.785562, "BNB.RUNE-B1A"},
		{"E6F49096971DAA96791FC1101333B4F0DF887D023F410ECE411CF1DB6FE321C1", "bnb1c8vl0r5ue53awz408ems6r3pvk04t7uenwt48w", 10, "BNB.RUNE-B1A"},
		{"E3A0F6E465B793EA4144FAE42A5985FAEBFBD17780A4CF147C24B2FDC2D9E1F5", "bnb13xrnj2e7hpae4tnksm9yxqznk9m95ujk0pv6gn", 3, "BNB.RUNE-B1A"},
		{"B139AB86B9B67EB4F729C6300EB136E22B6CFF92D1F0CF98284D0E8569E28034", "bnb176heu734unjw5hjw84tqulmha2fpuxaxlynclu", 76.20612, "BNB.RUNE-B1A"},
		{"D2EA3A93323546ACC71632393DF6AC986119752B52FFE422207060FC001BB226", "bnb1a8vapazge7g6z3fgh7f6m384n7p3ak6snln9kh", 0.83965422, "BNB.ETH-1C9"},
		{"032DB75290EDEE1B96EFCEADB3C32F9C0E97E059123782DD6AF98CDBA9F2C7BC", "bnb1yvj0p2j994u7qet9qd2fzgv0smr8098m6e5f0w", 40.665, "BNB.AVA-645"},
		{"B5BA7BE324095F4B079313175D10AD7822885CE3EBF4D81C883B214B2DD01E2B", "bnb164jc0pxcyu8gnkd2u022z3mtst2e5cxfpt76sm", 14, "BNB.RUNE-B1A"},
		{"F536F680C72BFCFD339FC43442F2F23445BA9C2201CB9B1A176BF006E02C8B5D", "bnb1xehlqphf90r7cca8s28kmnzvfkl4vsq2aeg4h7", 61.499, "BNB.RUNE-B1A"},
		{"BC7FC7CD884A2A4E46A8064D98213D348598504C637FA65D109072D1884CC572", "bnb1weh8yy8ywrhuc9nf9h2dymr5h059wlua2a97vy", 0.20983857, "BNB.ETH-1C9"},
		{"A906A663E7357CF0250845117D0665D1F1E2239071C92CA215B2D2907DD00573", "bnb1p9zneqlme5tmatkyzxg3tczer4rxwxhgqnyhmf", 14.249292, "BNB.RUNE-B1A"},
		{"CDA2C4DC7DE5124DE69407784A87B09814042F170C04056EDBE6E18EF0128BF1", "bnb1y86p3erdsuukhdelpf8vq3auzjseuzfen348cl", 0.10047994, "BNB.BNB"},
		{"BA99E1FEA1627924651CCEF154C82D03F4FD62E1A8BEB9DC757478456A534D6E", "bnb1tk05fcy0zsg3atxtp24s70cpjnps0h7vrk9kg2", 319.9599277, "BNB.BUSD-BD1"},
		{"4B3EC98C9BA3B0D7CB7B68862DB32A138930ED157736C5B006D9A1E5B581E3D9", "bnb1ydu26ssyx48x258qawtrrgwhpj28mhqhake57p", 1, "BNB.BNB"},
		{"9A457583D2BBC0B75BDDECB0CFD2C0DE5BFFB19C2415AF28631735B4A65D5C08", "bnb1u2rd2vkpdr24lw9mddkw7k2v0nna94vnfsuvfs", 35.824113, "BNB.RUNE-B1A"},
		{"2793CD3D0DA1220AB568F70CD88808F3DC0AE2B91A816D66CC6FC55D636701A3", "bnb1f8ht8rp7dk0uxshx8ckjpxn9zgsphxlklwluxc", 1, "BNB.BNB"},
		{"88E52E21C9A5ECF2AF7663B6CC827EE31CD4B7C3F201A38EA3E843A660686165", "bnb14e2e2jn7cjxca6ys63w9jtachf2c7xx50jsnw8", 5, "BNB.RUNE-B1A"},
		{"899B355C8051370F669FD4FAA3F921DE2CE10362CE6C8598248A8FC0687187BE", "bnb159wrhm5gt94fgkzldp8zmfgnlxa365k03zzsfm", 1.0911, "BNB.BNB"},
		{"E3555213FA7B8A3ECE24B77266E4757A7137B007EE5C7CAC7794580846D611CA", "bnb15ktsdv6w5lp973vduxukgpg7zdtg8r2hvws00c", 8.852112, "BNB.RUNE-B1A"},
		{"7C00E88FC775A0D5356767D89FBBC2888F4A07237B17BC4524012532117817E1", "bnb1vf3ljyga82sewznuc9lemvm2t4335wtm2xrxn5", 1143.246025, "BNB.RUNE-B1A"},
		{"B63C5591635CD671B7D442A5FD202A74DDDB0428096AEEAC1D8476E652788D22", "bnb1panzh7z479dfwj7j5dxkfa6hpze3j3gd8wykcj", 4000, "BNB.BUSD-BD1"},
		{"87B7544A4B8521BF2183BAB99AE68F01E4A4AFDEE5172B15B1D4D2C9E377F679", "bnb1amyth8xm9lr3l7h20zaxl5xw0taxz8d5y2zxlv", 218.315439, "BNB.RUNE-B1A"},
		{"C465E3EAF174BECB1E88402017E540FD0001B503ABA4C0606ABBE2EE03200AEB", "bnb18hpy44al65skdkenuh4ns47u8nf24fw2dtk8uq", 0.05113845, "BNB.BNB"},
		{"A2F9102F98BAB1A936E63CA4E7792B840487A6812A80E551E0F7BE5D82DE64E4", "bnb1rv89nkw2x5ksvhf6jtqwqpke4qhh7jmudpvqmj", 7606.5, "BNB.BUSD-BD1"},
		{"E5FB928D24028185FCF94777B312A2B1D9EB85241C116BFE43626D3541747ABB", "bnb1rv89nkw2x5ksvhf6jtqwqpke4qhh7jmudpvqmj", 4757.85, "BNB.BUSD-BD1"},
		{"70D0C9206E47C7A2FA3C5AD1E2BBC1B3F0A90BC535028FF32A5AA9EE71047734", "bnb1rv89nkw2x5ksvhf6jtqwqpke4qhh7jmudpvqmj", 31170.8404, "BNB.BUSD-BD1"},
		{"BA182566510AB2A7799075AA6078992058ECFFEE599BB4AFC7032CDBEEF64EA7", "bnb1f0kxq029tgac6zhx83q4gppsn857rqvwdzsw76", 1000, "BNB.BUSD-BD1"},
		{"72E92F8BB2FE9B26F4FAF4751D69FBEE76782F200149A805AC00DDC0843935F6", "bnb1m8wuq0kqq5cju704fhp94eth436v6zpvfl0ylw", 500, "BNB.BUSD-BD1"},
		{"68A009E3CCAB8A8725E15A36E308D972A74DB785ED4B8D966C0FFC445CF827E8", "bnb1rv89nkw2x5ksvhf6jtqwqpke4qhh7jmudpvqmj", 1019.749, "BNB.BUSD-BD1"},
		{"62F46CEF75ED79C78350E4905AE22361FB80CDC1DEFB047F91E6CA242C84174A", "bnb1rv89nkw2x5ksvhf6jtqwqpke4qhh7jmudpvqmj", 4935, "BNB.BUSD-BD1"},
		{"5126B3858B6296E810FD061D40F9D5A3D3915D46D7757E2120AADFF4B3EDD349", "bnb1rv89nkw2x5ksvhf6jtqwqpke4qhh7jmudpvqmj", 8077.5, "BNB.BUSD-BD1"},
		{"ADF763E3A0075C3E272CF8E0B32BD77F21C528C3996E0FBF085DB9D03F555421", "bnb1rv89nkw2x5ksvhf6jtqwqpke4qhh7jmudpvqmj", 4719.15, "BNB.BUSD-BD1"},
		{"C90A60BE87EFB7E340C2446A64A0702933525CCE0F5084E1A2C0323D98D9B84A", "bnb1rgwulut04q6ef85tcvk76ey5yld52slaplwwa9", 54.06594774, "BNB.BUSD-BD1"},
		{"BA9E117D6E40B9ADB257FF6A4B06B467DF82F47FF93D55191E0BDCB0004C5309", "bnb1x4xuxzeyckpctkhp5g425w3wf7y8m3w7z57s0f", 388.373211, "BNB.RUNE-B1A"},
		{"0E6E05E73D7DD5A74FE9F125748B2D24908154B75661FC4248B3B3901DB423C4", "bnb12xufpsysr6r0tankhqdaams9zee0tzxy006qns", 5, "BNB.RUNE-B1A"},
		{"E71642CAC325B733A3581F31F1DA64A52AA5AC590980E5630EF80BE1D906AECF", "bnb137xvx6ceqpxe34xr46y3ax350fr2q4zymw5l94", 2500, "BNB.BUSD-BD1"},
		{"A4F17152DDB1473AE3341311F256171A532558711B70467AB606040E570A6F46", "bnb1rv89nkw2x5ksvhf6jtqwqpke4qhh7jmudpvqmj", 30376.57296, "BNB.BUSD-BD1"},
	}
	// setup pools
	for _, item := range []string{"BNB.BNB", "BNB.BUSD-BD1", "BNB.ETH-1C9", "BNB.AVA-645"} {
		asset, err := common.NewAsset(item)
		c.Assert(err, IsNil)
		p := NewPool()
		p.Asset = asset
		p.Status = PoolAvailable
		p.BalanceAsset = cosmos.NewUint(common.One)
		p.BalanceRune = cosmos.NewUint(common.One)
		c.Assert(mgr.Keeper().SetPool(ctx, p), IsNil)
	}
	// create all tx voter
	for _, item := range refunds {
		hash, err := common.NewTxID(item.hash)
		if err != nil {
			ctx.Logger().Error("fail to parse hash", "hash", item.hash, "error", err)
			continue
		}
		fromAddr, err := pubKey.GetAddress(common.BNBChain)
		c.Assert(err, IsNil)
		addr, err := common.NewAddress(item.addr)
		c.Assert(err, IsNil)
		asset, err := common.NewAsset(item.asset)
		c.Assert(err, IsNil)
		coin := common.NewCoin(asset, cosmos.NewUint(uint64(item.amount*common.One)))
		tx := NewObservedTx(common.NewTx(hash, fromAddr, addr, common.NewCoins(coin),
			BNBGasFeeSingleton, "swap:BNB.BNB:bnb137xvx6ceqpxe34xr46y3ax350fr2q4zymw5l94"), common.BlockHeight(ctx), pubKey, common.BlockHeight(ctx))
		voter := NewObservedTxVoter(hash, []ObservedTx{tx})
		mgr.Keeper().SetObservedTxInVoter(ctx, voter)
	}
	refundBNBTransactionsV71(ctx, mgr)
	txOut, err := mgr.Keeper().GetTxOut(ctx, common.BlockHeight(ctx))
	c.Assert(err, IsNil)
	c.Assert(txOut.TxArray, HasLen, 89)
	refundHandler := NewRefundHandler(mgr)
	count := 0
	for idx, item := range refunds {
		if idx%2 == 0 {
			count++
			continue
		}
		hash, err := common.NewTxID(item.hash)
		if err != nil {
			ctx.Logger().Error("fail to parse hash", "hash", item.hash, "error", err)
			continue
		}
		fromAddr, err := pubKey.GetAddress(common.BNBChain)
		c.Assert(err, IsNil)
		addr, err := common.NewAddress(item.addr)
		c.Assert(err, IsNil)
		asset, err := common.NewAsset(item.asset)
		c.Assert(err, IsNil)
		coin := common.NewCoin(asset, cosmos.NewUint(uint64(item.amount*common.One)))
		tx := NewObservedTx(common.NewTx(hash, fromAddr, addr, common.NewCoins(coin),
			BNBGasFeeSingleton, NewRefundMemo(hash).String()), common.BlockHeight(ctx), pubKey, common.BlockHeight(ctx))
		refundMsg := types.NewMsgRefundTx(tx, hash, GetRandomBech32Addr())
		result, err := refundHandler.handle(ctx, *refundMsg)
		c.Assert(err, IsNil)
		c.Assert(result, NotNil)
	}
	c.Assert(os.Setenv("NET", "mainnet"), IsNil)
	// make sure the transaction can be rescheduled
	blockHeight := ctx.BlockHeight()
	ctx = ctx.WithBlockHeight(blockHeight + 300)
	c.Assert(mgr.Slasher().LackSigning(ctx, mgr.ConstAccessor, mgr), IsNil)
	txOut, err = mgr.Keeper().GetTxOut(ctx, common.BlockHeight(ctx))
	c.Assert(err, IsNil)
	c.Assert(txOut.TxArray, HasLen, count)
}

func (s *StoreManagerTestSuite) TestMigrateStoreV77(c *C) {
	c.Assert(os.Setenv("NET", "mainnet"), IsNil)
	ctx, mgr := setupManagerForTest(c)
	SetupConfigForTest()
	config := cosmos.GetConfig()
	config.SetBech32PrefixForAccount("thor", "thorpub")
	ctx = ctx.WithBlockHeight(1024)
	pubKey, err := common.NewPubKey(`thorpub1addwnpepq0myn4whrj7qfrzc647dju7rgtjc5punucxwvfut56mghuzxakq37e8ev4y`)
	c.Assert(err, IsNil)
	vault := NewVault(ctx.BlockHeight(), ActiveVault, AsgardVault, pubKey, []string{
		common.BTCChain.String(),
		common.ETHChain.String(),
		common.BNBChain.String(),
		common.BCHChain.String(),
		common.LTCChain.String(),
	}, nil)
	currentAsgardCoins := `[
            {
                "asset": "BNB.AVA-645",
                "amount": "2995505849358"
            },
            {
                "asset": "BNB.BNB",
                "amount": "446168460458"
            },
            {
                "asset": "BNB.BTCB-1DE",
                "amount": "3344094234"
            },
            {
                "asset": "BNB.BUSD-BD1",
                "amount": "549442431405611"
            },
            {
                "asset": "BNB.ETH-1C9",
                "amount": "31517072382"
            },
            {
                "asset": "BNB.TWT-8C2",
                "amount": "32445423761396"
            },
            {
                "asset": "BCH.BCH",
                "amount": "10533997912"
            },
            {
                "asset": "LTC.LTC",
                "amount": "122091616350"
            },
            {
                "asset": "BTC.BTC",
                "amount": "8024004386"
            },
            {
                "asset": "ETH.AAVE-0X7FC66500C84A76AD7E9C93437BFC5AC33E2DDAE9",
                "amount": "101784699275"
            },
            {
                "asset": "ETH.ALCX-0XDBDB4D16EDA451D0503B854CF79D55697F90C8DF",
                "amount": "220288455892"
            },
            {
                "asset": "ETH.ALPHA-0XA1FAA113CBE53436DF28FF0AEE54275C13B40975",
                "amount": "8876436709598"
            },
            {
                "asset": "ETH.CREAM-0X2BA592F78DB6436527729929AAF6C908497CB200",
                "amount": "127403642055"
            },
            {
                "asset": "ETH.DODO-0X43DFC4159D86F3A37A5A4B3D4580B888AD7D4DDD",
                "amount": "45791169720467"
            },
            {
                "asset": "ETH.ETH",
                "amount": "173222928384"
            },
            {
                "asset": "ETH.FOX-0XC770EEFAD204B5180DF6A14EE197D99D808EE52D",
                "amount": "133464519904774"
            },
            {
                "asset": "ETH.HEGIC-0X584BC13C7D411C00C01A62E8019472DE68768430",
                "amount": "37053912256444"
            },
            {
                "asset": "ETH.HOT-0X6C6EE5E31D828DE241282B9606C8E98EA48526E2",
                "amount": "2363210550686297"
            },
            {
                "asset": "ETH.KYL-0X67B6D479C7BB412C54E03DCA8E1BC6740CE6B99C",
                "amount": "226762492196091"
            },
            {
                "asset": "ETH.PERP-0XBC396689893D065F41BC2C6ECBEE5E0085233447",
                "amount": "1984309486311"
            },
            {
                "asset": "ETH.RAZE-0X5EAA69B29F99C84FE5DE8200340B4E9B4AB38EAC",
                "amount": "70873303136020"
            },
            {
                "asset": "ETH.SNX-0XC011A73EE8576FB46F5E1C5751CA3B9FE0AF2A6F",
                "amount": "1614417707351"
            },
            {
                "asset": "ETH.SUSHI-0X6B3595068778DD592E39A122F4F5A5CF09C90FE2",
                "amount": "6577015859010"
            },
            {
                "asset": "ETH.THOR-0XA5F2211B9B8170F694421F2046281775E8468044",
                "amount": "621970955052727"
            },
            {
                "asset": "ETH.USDC-0XA0B86991C6218B36C1D19D4A2E9EB0CE3606EB48",
                "amount": "120319012267800",
                "decimals": 6
            },
            {
                "asset": "ETH.USDT-0XDAC17F958D2EE523A2206206994597C13D831EC7",
                "amount": "50141176666100",
                "decimals": 6
            },
            {
                "asset": "ETH.XRUNE-0X69FA0FEE221AD11012BAB0FDB45D444D3D2CE71C",
                "amount": "1818497213416249"
            },
            {
                "asset": "ETH.YFI-0X0BC529C00C6401AEF6D220BE8C6EA1667F6AD93E",
                "amount": "553045668"
            },
            {
                "asset": "BNB.BAT-07A",
                "amount": "42339777240"
            },
            {
                "asset": "BNB.FRM-DE7",
                "amount": "1934787663777"
            },
            {
                "asset": "BNB.ADA-9F4",
                "amount": "406635606461"
            },
            {
                "asset": "BNB.USDT-6D8",
                "amount": "15727577814"
            },
            {
                "asset": "BNB.FTM-A64",
                "amount": "9955579837"
            },
            {
                "asset": "ETH.VIU-0X519475B31653E46D20CD09F9FDCF3B12BDACB4F5",
                "amount": "2052271"
            },
            {
                "asset": "ETH.RUNE-0X3155BA85D5F96B2D030A4966AF206230E46849CB",
                "amount": "967714581654"
            },
            {
                "asset": "BNB.RUNE-B1A",
                "amount": "4552442470536"
            }
        ]`
	var coins common.Coins
	if err := json.Unmarshal([]byte(currentAsgardCoins), &coins); err != nil {
		panic(err)
	}
	vault.AddFunds(coins)
	c.Assert(mgr.Keeper().SetVault(ctx, vault), IsNil)
	changesToVaultAndPools := []struct {
		asset  string
		amount cosmos.Uint
	}{
		{asset: "ETH.YFI-0X0BC529C00C6401AEF6D220BE8C6EA1667F6AD93E", amount: cosmos.NewUint(10015767)},
		{asset: "ETH.USDT-0XDAC17F958D2EE523A2206206994597C13D831EC7", amount: cosmos.NewUint(555652034700)},
		{asset: "ETH.XRUNE-0X69FA0FEE221AD11012BAB0FDB45D444D3D2CE71C", amount: cosmos.NewUint(43906122817485)},
		{asset: "ETH.USDC-0XA0B86991C6218B36C1D19D4A2E9EB0CE3606EB48", amount: cosmos.NewUint(2456761120600)},
		{asset: "ETH.ALCX-0XDBDB4D16EDA451D0503B854CF79D55697F90C8DF", amount: cosmos.NewUint(3290908800)},
		{asset: "ETH.CREAM-0X2BA592F78DB6436527729929AAF6C908497CB200", amount: cosmos.NewUint(1238645952)},
		{asset: "ETH.SNX-0XC011A73EE8576FB46F5E1C5751CA3B9FE0AF2A6F", amount: cosmos.NewUint(21225979435)},
		{asset: "ETH.DODO-0X43DFC4159D86F3A37A5A4B3D4580B888AD7D4DDD", amount: cosmos.NewUint(201067634531)},
		{asset: "ETH.PERP-0XBC396689893D065F41BC2C6ECBEE5E0085233447", amount: cosmos.NewUint(43419172120)},
		{asset: "ETH.SUSHI-0X6B3595068778DD592E39A122F4F5A5CF09C90FE2", amount: cosmos.NewUint(74548758317)},
		{asset: "ETH.RAZE-0X5EAA69B29F99C84FE5DE8200340B4E9B4AB38EAC", amount: cosmos.NewUint(722328715369)},
		{asset: "ETH.KYL-0X67B6D479C7BB412C54E03DCA8E1BC6740CE6B99C", amount: cosmos.NewUint(4450083484193)},
		{asset: "ETH.ALPHA-0XA1FAA113CBE53436DF28FF0AEE54275C13B40975", amount: cosmos.NewUint(318113978753)},
		{asset: "ETH.HOT-0X6C6EE5E31D828DE241282B9606C8E98EA48526E2", amount: cosmos.NewUint(60699215576820)},
		{asset: "ETH.HEGIC-0X584BC13C7D411C00C01A62E8019472DE68768430", amount: cosmos.NewUint(1694523495579)},
		{asset: "ETH.AAVE-0X7FC66500C84A76AD7E9C93437BFC5AC33E2DDAE9", amount: cosmos.NewUint(1073207831)},
	}
	// setup pools
	for _, item := range changesToVaultAndPools {
		asset, err := common.NewAsset(item.asset)
		c.Assert(err, IsNil)
		p := NewPool()
		p.Asset = asset
		p.Status = PoolAvailable
		p.BalanceAsset = cosmos.NewUint(common.One)
		p.BalanceRune = cosmos.NewUint(common.One)
		c.Assert(mgr.Keeper().SetPool(ctx, p), IsNil)
	}
	migrateStoreV77(ctx, mgr)

	vaultAfter, err := mgr.Keeper().GetVault(ctx, pubKey)
	c.Assert(err, IsNil)
	vaultAfter.SubFunds(vault.Coins)
	vaultAfterCoins := vaultAfter.Coins

	for _, item := range changesToVaultAndPools {
		asset, err := common.NewAsset(item.asset)
		c.Assert(err, IsNil)
		c.Assert(vaultAfterCoins.Contains(common.NewCoin(asset, item.amount)), Equals, true)
		p, err := mgr.Keeper().GetPool(ctx, asset)
		c.Assert(err, IsNil)
		c.Assert(p.BalanceAsset.Sub(cosmos.NewUint(common.One)).Equal(item.amount), Equals, true)
	}
}
