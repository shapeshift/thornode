package thorchain

import (
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
	"gitlab.com/thorchain/thornode/x/thorchain/types"
)

type TxOutStoreV65Suite struct{}

var _ = Suite(&TxOutStoreV65Suite{})

func (s TxOutStoreV65Suite) TestAddGasFees(c *C) {
	ctx, mgr := setupManagerForTest(c)
	tx := GetRandomObservedTx()

	version := GetCurrentVersion()
	constAccessor := constants.GetConstantValues(version)
	mgr.gasMgr = newGasMgrV1(constAccessor, mgr.Keeper())
	err := addGasFees(ctx, mgr, tx)
	c.Assert(err, IsNil)
	c.Assert(mgr.GasMgr().GetGas(), HasLen, 1)
}

func (s TxOutStoreV65Suite) TestEndBlock(c *C) {
	w := getHandlerTestWrapper(c, 1, true, true)
	txOutStore := newTxOutStorageV65(w.keeper, w.mgr.GetConstants(), w.mgr.EventMgr(), w.mgr.GasMgr())

	item := TxOutItem{
		Chain:     common.BNBChain,
		ToAddress: GetRandomBNBAddress(),
		InHash:    GetRandomTxHash(),
		Coin:      common.NewCoin(common.BNBAsset, cosmos.NewUint(20*common.One)),
	}
	err := txOutStore.UnSafeAddTxOutItem(w.ctx, w.mgr, item)
	c.Assert(err, IsNil)

	c.Assert(txOutStore.EndBlock(w.ctx, w.mgr), IsNil)

	items, err := txOutStore.GetOutboundItems(w.ctx)
	c.Assert(err, IsNil)
	c.Assert(items, HasLen, 1)
	c.Check(items[0].GasRate, Equals, int64(56250))
	c.Assert(items[0].MaxGas, HasLen, 1)
	c.Check(items[0].MaxGas[0].Asset.Equals(common.BNBAsset), Equals, true)
	c.Check(items[0].MaxGas[0].Amount.Uint64(), Equals, uint64(37500))
}

func (s TxOutStoreV65Suite) TestAddOutTxItem(c *C) {
	w := getHandlerTestWrapper(c, 1, true, true)
	vault := GetRandomVault()
	vault.Coins = common.Coins{
		common.NewCoin(common.RuneAsset(), cosmos.NewUint(10000*common.One)),
		common.NewCoin(common.BNBAsset, cosmos.NewUint(10000*common.One)),
	}
	w.keeper.SetVault(w.ctx, vault)

	acc1 := GetRandomNodeAccount(NodeActive)
	acc2 := GetRandomNodeAccount(NodeActive)
	acc3 := GetRandomNodeAccount(NodeActive)
	c.Assert(w.keeper.SetNodeAccount(w.ctx, acc1), IsNil)
	c.Assert(w.keeper.SetNodeAccount(w.ctx, acc2), IsNil)
	c.Assert(w.keeper.SetNodeAccount(w.ctx, acc3), IsNil)

	ygg := NewVault(common.BlockHeight(w.ctx), ActiveVault, YggdrasilVault, acc1.PubKeySet.Secp256k1, common.Chains{common.BNBChain}.Strings(), []ChainContract{})
	ygg.AddFunds(
		common.Coins{
			common.NewCoin(common.BNBAsset, cosmos.NewUint(40*common.One)),
			common.NewCoin(common.BCHAsset, cosmos.NewUint(40*common.One)),
		},
	)
	c.Assert(w.keeper.SetVault(w.ctx, ygg), IsNil)

	ygg = NewVault(common.BlockHeight(w.ctx), ActiveVault, YggdrasilVault, acc2.PubKeySet.Secp256k1, common.Chains{common.BNBChain}.Strings(), []ChainContract{})
	ygg.AddFunds(
		common.Coins{
			common.NewCoin(common.BNBAsset, cosmos.NewUint(50*common.One)),
			common.NewCoin(common.BCHAsset, cosmos.NewUint(40*common.One)),
		},
	)
	c.Assert(w.keeper.SetVault(w.ctx, ygg), IsNil)

	ygg = NewVault(common.BlockHeight(w.ctx), ActiveVault, YggdrasilVault, acc3.PubKeySet.Secp256k1, common.Chains{common.BNBChain}.Strings(), []ChainContract{})
	ygg.AddFunds(
		common.Coins{
			common.NewCoin(common.BNBAsset, cosmos.NewUint(100*common.One)),
			common.NewCoin(common.BCHAsset, cosmos.NewUint(40*common.One)),
		},
	)
	c.Assert(w.keeper.SetVault(w.ctx, ygg), IsNil)

	// Create voter
	inTxID := GetRandomTxHash()
	voter := NewObservedTxVoter(inTxID, ObservedTxs{
		ObservedTx{
			Tx:             GetRandomTx(),
			Status:         types.Status_incomplete,
			BlockHeight:    1,
			Signers:        []string{w.activeNodeAccount.NodeAddress.String(), acc1.NodeAddress.String(), acc2.NodeAddress.String()},
			KeysignMs:      0,
			FinaliseHeight: 1,
		},
	})
	w.keeper.SetObservedTxInVoter(w.ctx, voter)

	// Should get acc2. Acc3 hasn't signed and acc2 is the highest value
	item := TxOutItem{
		Chain:     common.BNBChain,
		ToAddress: GetRandomBNBAddress(),
		InHash:    inTxID,
		Coin:      common.NewCoin(common.BNBAsset, cosmos.NewUint(20*common.One)),
	}
	txOutStore := newTxOutStorageV65(w.keeper, w.mgr.GetConstants(), w.mgr.EventMgr(), w.mgr.GasMgr())
	ok, err := txOutStore.TryAddTxOutItem(w.ctx, w.mgr, item)
	c.Assert(err, IsNil)
	c.Assert(ok, Equals, true)
	msgs, err := txOutStore.GetOutboundItems(w.ctx)
	c.Assert(err, IsNil)
	c.Assert(msgs, HasLen, 1)
	c.Assert(msgs[0].VaultPubKey.String(), Equals, acc2.PubKeySet.Secp256k1.String())
	c.Assert(msgs[0].Coin.Amount.Equal(cosmos.NewUint(1999887500)), Equals, true, Commentf("%d", msgs[0].Coin.Amount.Uint64()))
	// Should get acc1. Acc3 hasn't signed and acc1 now has the highest amount
	// of coin.
	item = TxOutItem{
		Chain:     common.BNBChain,
		ToAddress: GetRandomBNBAddress(),
		InHash:    inTxID,
		Coin:      common.NewCoin(common.BNBAsset, cosmos.NewUint(20*common.One)),
	}
	txOutStore.ClearOutboundItems(w.ctx)
	success, err := txOutStore.TryAddTxOutItem(w.ctx, w.mgr, item)
	c.Assert(success, Equals, true)
	c.Assert(err, IsNil)
	msgs, err = txOutStore.GetOutboundItems(w.ctx)
	c.Assert(err, IsNil)
	c.Assert(msgs, HasLen, 1)
	c.Assert(msgs[0].VaultPubKey.String(), Equals, acc2.PubKeySet.Secp256k1.String())

	item = TxOutItem{
		Chain:     common.BNBChain,
		ToAddress: GetRandomBNBAddress(),
		InHash:    inTxID,
		Coin:      common.NewCoin(common.BNBAsset, cosmos.NewUint(1000*common.One)),
	}
	txOutStore.ClearOutboundItems(w.ctx)
	success, err = txOutStore.TryAddTxOutItem(w.ctx, w.mgr, item)
	c.Assert(err, IsNil)
	c.Assert(success, Equals, true)
	msgs, err = txOutStore.GetOutboundItems(w.ctx)
	c.Assert(err, IsNil)
	c.Assert(msgs, HasLen, 3)
	c.Check(msgs[0].VaultPubKey.String(), Equals, acc2.PubKeySet.Secp256k1.String())
	c.Check(msgs[1].VaultPubKey.String(), Equals, acc1.PubKeySet.Secp256k1.String())
	c.Check(msgs[2].VaultPubKey.String(), Equals, vault.PubKey.String())

	item = TxOutItem{
		Chain:     common.BCHChain,
		ToAddress: "1EFJFJm7Y9mTVsCBXA9PKuRuzjgrdBe4rR",
		InHash:    inTxID,
		Coin:      common.NewCoin(common.BCHAsset, cosmos.NewUint(20*common.One)),
		MaxGas: common.Gas{
			common.NewCoin(common.BCHAsset, cosmos.NewUint(10000)),
		},
	}
	txOutStore.ClearOutboundItems(w.ctx)
	result, err := txOutStore.TryAddTxOutItem(w.ctx, w.mgr, item)
	c.Assert(result, Equals, true)
	c.Assert(err, IsNil)
	msgs, err = txOutStore.GetOutboundItems(w.ctx)
	// this should be a mocknet address
	c.Assert(msgs[0].ToAddress.String(), Equals, "qzg5mkh7rkw3y8kw47l3rrnvhmenvctmd5yg6hxe64")
}

func (s TxOutStoreV65Suite) TestAddOutTxItemNotEnoughForFee(c *C) {
	w := getHandlerTestWrapper(c, 1, true, true)
	vault := GetRandomVault()
	vault.Coins = common.Coins{
		common.NewCoin(common.RuneAsset(), cosmos.NewUint(10000*common.One)),
		common.NewCoin(common.BNBAsset, cosmos.NewUint(10000*common.One)),
	}
	w.keeper.SetVault(w.ctx, vault)

	acc1 := GetRandomNodeAccount(NodeActive)
	acc2 := GetRandomNodeAccount(NodeActive)
	acc3 := GetRandomNodeAccount(NodeActive)
	c.Assert(w.keeper.SetNodeAccount(w.ctx, acc1), IsNil)
	c.Assert(w.keeper.SetNodeAccount(w.ctx, acc2), IsNil)
	c.Assert(w.keeper.SetNodeAccount(w.ctx, acc3), IsNil)

	ygg := NewVault(common.BlockHeight(w.ctx), ActiveVault, YggdrasilVault, acc1.PubKeySet.Secp256k1, common.Chains{common.BNBChain}.Strings(), []ChainContract{})
	ygg.AddFunds(
		common.Coins{
			common.NewCoin(common.BNBAsset, cosmos.NewUint(40*common.One)),
		},
	)
	c.Assert(w.keeper.SetVault(w.ctx, ygg), IsNil)

	ygg = NewVault(common.BlockHeight(w.ctx), ActiveVault, YggdrasilVault, acc2.PubKeySet.Secp256k1, common.Chains{common.BNBChain}.Strings(), []ChainContract{})
	ygg.AddFunds(
		common.Coins{
			common.NewCoin(common.BNBAsset, cosmos.NewUint(50*common.One)),
		},
	)
	c.Assert(w.keeper.SetVault(w.ctx, ygg), IsNil)

	ygg = NewVault(common.BlockHeight(w.ctx), ActiveVault, YggdrasilVault, acc3.PubKeySet.Secp256k1, common.Chains{common.BNBChain}.Strings(), []ChainContract{})
	ygg.AddFunds(
		common.Coins{
			common.NewCoin(common.BNBAsset, cosmos.NewUint(100*common.One)),
		},
	)
	c.Assert(w.keeper.SetVault(w.ctx, ygg), IsNil)

	// Create voter
	inTxID := GetRandomTxHash()
	voter := NewObservedTxVoter(inTxID, ObservedTxs{
		ObservedTx{
			Tx:             GetRandomTx(),
			Status:         types.Status_incomplete,
			BlockHeight:    1,
			Signers:        []string{w.activeNodeAccount.NodeAddress.String(), acc1.NodeAddress.String(), acc2.NodeAddress.String()},
			KeysignMs:      0,
			FinaliseHeight: 1,
		},
	})
	w.keeper.SetObservedTxInVoter(w.ctx, voter)

	item := TxOutItem{
		Chain:     common.BNBChain,
		ToAddress: GetRandomBNBAddress(),
		InHash:    inTxID,
		Coin:      common.NewCoin(common.BNBAsset, cosmos.NewUint(30000)),
	}
	txOutStore := newTxOutStorageV65(w.keeper, w.mgr.GetConstants(), w.mgr.EventMgr(), w.mgr.GasMgr())
	ok, err := txOutStore.TryAddTxOutItem(w.ctx, w.mgr, item)
	c.Assert(err, NotNil)
	c.Assert(err, Equals, ErrNotEnoughToPayFee)
	c.Assert(ok, Equals, false)
	msgs, err := txOutStore.GetOutboundItems(w.ctx)
	c.Assert(err, IsNil)
	c.Assert(msgs, HasLen, 0)
}

func (s TxOutStoreV65Suite) TestAddOutTxItemWithoutBFT(c *C) {
	w := getHandlerTestWrapper(c, 1, true, true)
	vault := GetRandomVault()
	vault.Coins = common.Coins{
		common.NewCoin(common.BNBAsset, cosmos.NewUint(100*common.One)),
	}
	w.keeper.SetVault(w.ctx, vault)

	inTxID := GetRandomTxHash()
	item := TxOutItem{
		Chain:     common.BNBChain,
		ToAddress: GetRandomBNBAddress(),
		InHash:    inTxID,
		Coin:      common.NewCoin(common.BNBAsset, cosmos.NewUint(20*common.One)),
	}
	txOutStore := newTxOutStorageV65(w.keeper, w.mgr.GetConstants(), w.mgr.EventMgr(), w.mgr.GasMgr())
	success, err := txOutStore.TryAddTxOutItem(w.ctx, w.mgr, item)
	c.Assert(err, IsNil)
	c.Assert(success, Equals, true)
	msgs, err := txOutStore.GetOutboundItems(w.ctx)
	c.Assert(err, IsNil)
	c.Assert(msgs, HasLen, 1)
	c.Assert(msgs[0].Coin.Amount.Equal(cosmos.NewUint(1999887500)), Equals, true, Commentf("%d", msgs[0].Coin.Amount.Uint64()))
}

func (s TxOutStoreV65Suite) TestAddOutTxItemDeductMaxGasFromYggdrasil(c *C) {
	w := getHandlerTestWrapper(c, 1, true, true)
	vault := GetRandomVault()
	vault.Coins = common.Coins{
		common.NewCoin(common.RuneAsset(), cosmos.NewUint(10000*common.One)),
		common.NewCoin(common.BNBAsset, cosmos.NewUint(10000*common.One)),
	}
	w.keeper.SetVault(w.ctx, vault)

	acc1 := GetRandomNodeAccount(NodeActive)
	acc2 := GetRandomNodeAccount(NodeActive)
	acc3 := GetRandomNodeAccount(NodeActive)
	c.Assert(w.keeper.SetNodeAccount(w.ctx, acc1), IsNil)
	c.Assert(w.keeper.SetNodeAccount(w.ctx, acc2), IsNil)
	c.Assert(w.keeper.SetNodeAccount(w.ctx, acc3), IsNil)

	ygg := NewVault(common.BlockHeight(w.ctx), ActiveVault, YggdrasilVault, acc1.PubKeySet.Secp256k1, common.Chains{common.BNBChain}.Strings(), []ChainContract{})
	ygg.AddFunds(
		common.Coins{
			common.NewCoin(common.BNBAsset, cosmos.NewUint(11*common.One)),
		},
	)
	c.Assert(w.keeper.SetVault(w.ctx, ygg), IsNil)

	ygg = NewVault(common.BlockHeight(w.ctx), ActiveVault, YggdrasilVault, acc2.PubKeySet.Secp256k1, common.Chains{common.BNBChain}.Strings(), []ChainContract{})
	ygg.AddFunds(
		common.Coins{
			common.NewCoin(common.BNBAsset, cosmos.NewUint(50*common.One)),
		},
	)
	c.Assert(w.keeper.SetVault(w.ctx, ygg), IsNil)

	ygg = NewVault(common.BlockHeight(w.ctx), ActiveVault, YggdrasilVault, acc3.PubKeySet.Secp256k1, common.Chains{common.BNBChain}.Strings(), []ChainContract{})
	ygg.AddFunds(
		common.Coins{
			common.NewCoin(common.BNBAsset, cosmos.NewUint(100*common.One)),
		},
	)
	c.Assert(w.keeper.SetVault(w.ctx, ygg), IsNil)

	// Create voter
	inTxID := GetRandomTxHash()
	voter := NewObservedTxVoter(inTxID, ObservedTxs{
		ObservedTx{
			Tx:             GetRandomTx(),
			Status:         types.Status_incomplete,
			BlockHeight:    1,
			Signers:        []string{w.activeNodeAccount.NodeAddress.String(), acc1.NodeAddress.String(), acc2.NodeAddress.String()},
			KeysignMs:      0,
			FinaliseHeight: 1,
		},
	})
	w.keeper.SetObservedTxInVoter(w.ctx, voter)

	// Should get acc2. Acc3 hasn't signed and acc2 is the highest value
	item := TxOutItem{
		Chain:     common.BNBChain,
		ToAddress: GetRandomBNBAddress(),
		InHash:    inTxID,
		Coin:      common.NewCoin(common.BNBAsset, cosmos.NewUint(3900000000)),
		MaxGas: common.Gas{
			common.NewCoin(common.BNBAsset, cosmos.NewUint(100000000)),
		},
	}
	txOutStore := newTxOutStorageV65(w.keeper, w.mgr.GetConstants(), w.mgr.EventMgr(), w.mgr.GasMgr())
	ok, err := txOutStore.TryAddTxOutItem(w.ctx, w.mgr, item)
	c.Assert(err, IsNil)
	c.Assert(ok, Equals, true)
	msgs, err := txOutStore.GetOutboundItems(w.ctx)
	c.Assert(err, IsNil)
	c.Assert(msgs, HasLen, 1)

	item1 := TxOutItem{
		Chain:     common.BNBChain,
		ToAddress: GetRandomBNBAddress(),
		InHash:    inTxID,
		Coin:      common.NewCoin(common.BNBAsset, cosmos.NewUint(1000000000)),
		MaxGas: common.Gas{
			common.NewCoin(common.BNBAsset, cosmos.NewUint(7500)),
		},
	}
	ok, err = txOutStore.TryAddTxOutItem(w.ctx, w.mgr, item1)
	c.Assert(err, IsNil)
	c.Assert(ok, Equals, true)
	msgs, err = txOutStore.GetOutboundItems(w.ctx)
	c.Assert(err, IsNil)
	c.Assert(msgs, HasLen, 2)
	c.Assert(msgs[1].VaultPubKey.Equals(acc1.PubKeySet.Secp256k1), Equals, true)
}

type TestCalcKeeper struct {
	keeper.KVStoreDummy
	value map[int64]cosmos.Uint
	mimir map[string]int64
}

func (k *TestCalcKeeper) GetPool(ctx cosmos.Context, asset common.Asset) (types.Pool, error) {
	pool := NewPool()
	pool.Asset = asset
	pool.BalanceRune = cosmos.NewUint(90527581399649)
	pool.BalanceAsset = cosmos.NewUint(1402011488988)
	return pool, nil
}

func (k *TestCalcKeeper) GetMimir(ctx cosmos.Context, key string) (int64, error) {
	return k.mimir[key], nil
}

func (k *TestCalcKeeper) GetTxOutValue(ctx cosmos.Context, height int64) (cosmos.Uint, error) {
	val, ok := k.value[height]
	if !ok {
		return cosmos.ZeroUint(), nil
	}
	return val, nil
}

func (s TxOutStoreV65Suite) TestcalcTxOutHeight(c *C) {
	keeper := &TestCalcKeeper{
		value: make(map[int64]cosmos.Uint, 0),
		mimir: make(map[string]int64, 0),
	}

	keeper.mimir["MinTxOutVolumeThreshold"] = 25_00000000
	keeper.mimir["TxOutDelayRate"] = 25_00000000
	keeper.mimir["MaxTxOutOffset"] = 720
	keeper.mimir["TxOutDelayMax"] = 17280

	addValue := func(h int64, v cosmos.Uint) {
		if _, ok := keeper.value[h]; !ok {
			keeper.value[h] = cosmos.ZeroUint()
		}
		keeper.value[h] = keeper.value[h].Add(v)
	}

	ctx, _ := setupManagerForTest(c)

	txout := TxOutStorageV65{keeper: keeper}

	toi := TxOutItem{
		Coin: common.NewCoin(common.BNBAsset, cosmos.NewUint(50*common.One)),
		Memo: "OUT:nomnomnom",
	}
	pool, _ := keeper.GetPool(ctx, common.BNBAsset)
	value := pool.AssetValueInRune(toi.Coin.Amount)

	targetBlock, err := txout.calcTxOutHeight(ctx, toi)
	c.Assert(err, IsNil)
	c.Check(targetBlock, Equals, int64(147))
	addValue(targetBlock, value)

	targetBlock, err = txout.calcTxOutHeight(ctx, toi)
	c.Assert(err, IsNil)
	c.Check(targetBlock, Equals, int64(148))
	addValue(targetBlock, value)

	toi.Coin.Amount = cosmos.NewUint(50000 * common.One)
	targetBlock, err = txout.calcTxOutHeight(ctx, toi)
	c.Assert(err, IsNil)
	c.Check(targetBlock, Equals, int64(738))
	addValue(targetBlock, value)
}
