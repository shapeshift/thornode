package thorchain

import (
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
	"gitlab.com/thorchain/thornode/x/thorchain/types"
)

type HelperSuite struct{}

var _ = Suite(&HelperSuite{})

type TestRefundBondKeeper struct {
	keeper.KVStoreDummy
	ygg    Vault
	pool   Pool
	na     NodeAccount
	vaults Vaults
}

func (k *TestRefundBondKeeper) GetAsgardVaultsByStatus(_ cosmos.Context, _ VaultStatus) (Vaults, error) {
	return k.vaults, nil
}

func (k *TestRefundBondKeeper) VaultExists(_ cosmos.Context, pk common.PubKey) bool {
	return true
}

func (k *TestRefundBondKeeper) GetVault(_ cosmos.Context, pk common.PubKey) (Vault, error) {
	if k.ygg.PubKey.Equals(pk) {
		return k.ygg, nil
	}
	return Vault{}, kaboom
}

func (k *TestRefundBondKeeper) GetLeastSecure(ctx cosmos.Context, vaults Vaults, signingTransPeriod int64) Vault {
	return vaults[0]
}

func (k *TestRefundBondKeeper) GetPool(_ cosmos.Context, asset common.Asset) (Pool, error) {
	if k.pool.Asset.Equals(asset) {
		return k.pool, nil
	}
	return NewPool(), kaboom
}

func (k *TestRefundBondKeeper) SetNodeAccount(_ cosmos.Context, na NodeAccount) error {
	k.na = na
	return nil
}

func (k *TestRefundBondKeeper) SetPool(_ cosmos.Context, p Pool) error {
	if k.pool.Asset.Equals(p.Asset) {
		k.pool = p
		return nil
	}
	return kaboom
}

func (k *TestRefundBondKeeper) DeleteVault(_ cosmos.Context, key common.PubKey) error {
	if k.ygg.PubKey.Equals(key) {
		k.ygg = NewVault(1, InactiveVault, AsgardVault, GetRandomPubKey(), common.Chains{common.BNBChain})
	}
	return nil
}

func (s *HelperSuite) TestSubsidizePoolWithSlashBond(c *C) {
	ctx, k := setupKeeperForTest(c)
	ygg := GetRandomVault()
	c.Assert(subsidizePoolWithSlashBond(ctx, k, ygg, cosmos.NewUint(100*common.One), cosmos.ZeroUint()), IsNil)
	poolBNB := NewPool()
	poolBNB.Asset = common.BNBAsset
	poolBNB.BalanceRune = cosmos.NewUint(100 * common.One)
	poolBNB.BalanceAsset = cosmos.NewUint(100 * common.One)
	poolBNB.Status = PoolAvailable
	c.Assert(k.SetPool(ctx, poolBNB), IsNil)

	poolTCAN := NewPool()
	tCanAsset, err := common.NewAsset("BNB.TCAN-014")
	c.Assert(err, IsNil)
	poolTCAN.Asset = tCanAsset
	poolTCAN.BalanceRune = cosmos.NewUint(200 * common.One)
	poolTCAN.BalanceAsset = cosmos.NewUint(200 * common.One)
	poolTCAN.Status = PoolAvailable
	c.Assert(k.SetPool(ctx, poolTCAN), IsNil)

	poolBTC := NewPool()
	poolBTC.Asset = common.BTCAsset
	poolBTC.BalanceAsset = cosmos.NewUint(300 * common.One)
	poolBTC.BalanceRune = cosmos.NewUint(300 * common.One)
	poolBTC.Status = PoolAvailable
	c.Assert(k.SetPool(ctx, poolBTC), IsNil)
	ygg.Type = YggdrasilVault
	ygg.Coins = common.Coins{
		common.NewCoin(common.RuneAsset(), cosmos.NewUint(1*common.One)),
		common.NewCoin(common.BNBAsset, cosmos.NewUint(1*common.One)),            // 1
		common.NewCoin(tCanAsset, cosmos.NewUint(common.One).QuoUint64(2)),       // 0.5 TCAN
		common.NewCoin(common.BTCAsset, cosmos.NewUint(common.One).QuoUint64(4)), // 0.25 BTC
	}
	totalRuneLeft, err := getTotalYggValueInRune(ctx, k, ygg)
	c.Assert(err, IsNil)

	totalRuneStolen := ygg.GetCoin(common.RuneAsset()).Amount
	slashAmt := totalRuneLeft.MulUint64(3).QuoUint64(2)
	c.Assert(subsidizePoolWithSlashBond(ctx, k, ygg, totalRuneLeft, slashAmt), IsNil)

	slashAmt = common.SafeSub(slashAmt, totalRuneStolen)
	totalRuneLeft = common.SafeSub(totalRuneLeft, totalRuneStolen)

	amountBNBForBNBPool := slashAmt.Mul(poolBNB.AssetValueInRune(cosmos.NewUint(common.One))).Quo(totalRuneLeft)
	runeBNB := poolBNB.BalanceRune.Add(amountBNBForBNBPool)
	bnbPoolAsset := poolBNB.BalanceAsset.Sub(cosmos.NewUint(common.One))
	poolBNB, err = k.GetPool(ctx, common.BNBAsset)
	c.Assert(err, IsNil)
	c.Assert(poolBNB.BalanceRune.Equal(runeBNB), Equals, true)
	c.Assert(poolBNB.BalanceAsset.Equal(bnbPoolAsset), Equals, true)
	amountRuneForTCANPool := slashAmt.Mul(poolTCAN.AssetValueInRune(cosmos.NewUint(common.One).QuoUint64(2))).Quo(totalRuneLeft)
	runeTCAN := poolTCAN.BalanceRune.Add(amountRuneForTCANPool)
	tcanPoolAsset := poolTCAN.BalanceAsset.Sub(cosmos.NewUint(common.One).QuoUint64(2))
	poolTCAN, err = k.GetPool(ctx, tCanAsset)
	c.Assert(err, IsNil)
	c.Assert(poolTCAN.BalanceRune.Equal(runeTCAN), Equals, true)
	c.Assert(poolTCAN.BalanceAsset.Equal(tcanPoolAsset), Equals, true)
	amountRuneForBTCPool := slashAmt.Mul(poolBTC.AssetValueInRune(cosmos.NewUint(common.One).QuoUint64(4))).Quo(totalRuneLeft)
	runeBTC := poolBTC.BalanceRune.Add(amountRuneForBTCPool)
	btcPoolAsset := poolBTC.BalanceAsset.Sub(cosmos.NewUint(common.One).QuoUint64(4))
	poolBTC, err = k.GetPool(ctx, common.BTCAsset)
	c.Assert(err, IsNil)
	c.Assert(poolBTC.BalanceRune.Equal(runeBTC), Equals, true)
	c.Assert(poolBTC.BalanceAsset.Equal(btcPoolAsset), Equals, true)

	ygg1 := GetRandomVault()
	ygg1.Type = YggdrasilVault
	ygg1.Coins = common.Coins{
		common.NewCoin(tCanAsset, cosmos.NewUint(common.One*2)),       // 2 TCAN
		common.NewCoin(common.BTCAsset, cosmos.NewUint(common.One*4)), // 4 BTC
	}
	totalRuneLeft, err = getTotalYggValueInRune(ctx, k, ygg1)
	c.Assert(err, IsNil)
	slashAmt = cosmos.NewUint(100 * common.One)
	c.Assert(subsidizePoolWithSlashBond(ctx, k, ygg1, totalRuneLeft, slashAmt), IsNil)
	amountRuneForTCANPool = slashAmt.Mul(poolTCAN.AssetValueInRune(cosmos.NewUint(common.One * 2))).Quo(totalRuneLeft)
	runeTCAN = poolTCAN.BalanceRune.Add(amountRuneForTCANPool)
	poolTCAN, err = k.GetPool(ctx, tCanAsset)
	c.Assert(err, IsNil)
	c.Assert(poolTCAN.BalanceRune.Equal(runeTCAN), Equals, true)
	amountRuneForBTCPool = slashAmt.Mul(poolBTC.AssetValueInRune(cosmos.NewUint(common.One * 4))).Quo(totalRuneLeft)
	runeBTC = poolBTC.BalanceRune.Add(amountRuneForBTCPool)
	poolBTC, err = k.GetPool(ctx, common.BTCAsset)
	c.Assert(err, IsNil)
	c.Assert(poolBTC.BalanceRune.Equal(runeBTC), Equals, true)
}

func (s *HelperSuite) TestRefundBondError(c *C) {
	ctx, _ := setupKeeperForTest(c)
	// active node should not refund bond
	pk := GetRandomPubKey()
	na := GetRandomNodeAccount(NodeActive)
	na.PubKeySet.Secp256k1 = pk
	na.Bond = cosmos.NewUint(100 * common.One)
	mgr := NewDummyMgr()
	tx := GetRandomTx()
	keeper1 := &TestRefundBondKeeper{}
	c.Assert(refundBond(ctx, tx, cosmos.ZeroUint(), &na, keeper1, mgr), IsNil)

	// fail to get vault should return an error
	na.UpdateStatus(NodeStandby, common.BlockHeight(ctx))
	keeper1.na = na
	c.Assert(refundBond(ctx, tx, cosmos.ZeroUint(), &na, keeper1, mgr), NotNil)

	// if the vault is not a yggdrasil pool , it should return an error
	ygg := NewVault(common.BlockHeight(ctx), ActiveVault, AsgardVault, pk, common.Chains{common.BNBChain})
	ygg.Coins = common.Coins{}
	keeper1.ygg = ygg
	c.Assert(refundBond(ctx, tx, cosmos.ZeroUint(), &na, keeper1, mgr), NotNil)

	// fail to get pool should fail
	ygg = NewVault(common.BlockHeight(ctx), ActiveVault, YggdrasilVault, pk, common.Chains{common.BNBChain})
	ygg.Coins = common.Coins{
		common.NewCoin(common.RuneAsset(), cosmos.NewUint(27*common.One)),
		common.NewCoin(common.BNBAsset, cosmos.NewUint(27*common.One)),
	}
	keeper1.ygg = ygg
	c.Assert(refundBond(ctx, tx, cosmos.ZeroUint(), &na, keeper1, mgr), NotNil)

	// when ygg asset in RUNE is more then bond , thorchain should slash the node account with all their bond
	keeper1.pool = Pool{
		Asset:        common.BNBAsset,
		BalanceRune:  cosmos.NewUint(1024 * common.One),
		BalanceAsset: cosmos.NewUint(167 * common.One),
	}
	c.Assert(refundBond(ctx, tx, cosmos.ZeroUint(), &na, keeper1, mgr), IsNil)
	// make sure no tx has been generated for refund
	items, err := mgr.TxOutStore().GetOutboundItems(ctx)
	c.Assert(err, IsNil)
	c.Check(items, HasLen, 0)
}

func (s *HelperSuite) TestRefundBondHappyPath(c *C) {
	ctx, _ := setupKeeperForTest(c)
	na := GetRandomNodeAccount(NodeActive)
	na.Bond = cosmos.NewUint(12098 * common.One)
	mgr := NewDummyMgr()
	pk := GetRandomPubKey()
	na.PubKeySet.Secp256k1 = pk
	ygg := NewVault(common.BlockHeight(ctx), ActiveVault, YggdrasilVault, pk, common.Chains{common.BNBChain})

	ygg.Coins = common.Coins{
		common.NewCoin(common.RuneAsset(), cosmos.NewUint(3946*common.One)),
		common.NewCoin(common.BNBAsset, cosmos.NewUint(27*common.One)),
	}
	keeper := &TestRefundBondKeeper{
		pool: Pool{
			Asset:        common.BNBAsset,
			BalanceRune:  cosmos.NewUint(23789 * common.One),
			BalanceAsset: cosmos.NewUint(167 * common.One),
		},
		ygg:    ygg,
		vaults: Vaults{GetRandomVault()},
	}
	na.Status = NodeStandby
	tx := GetRandomTx()
	yggAssetInRune, err := getTotalYggValueInRune(ctx, keeper, ygg)
	c.Assert(err, IsNil)
	err = refundBond(ctx, tx, cosmos.ZeroUint(), &na, keeper, mgr)
	c.Assert(err, IsNil)
	slashAmt := yggAssetInRune.MulUint64(3).QuoUint64(2)
	items, err := mgr.TxOutStore().GetOutboundItems(ctx)
	c.Assert(err, IsNil)
	c.Assert(items, HasLen, 0)
	p, err := keeper.GetPool(ctx, common.BNBAsset)
	c.Assert(err, IsNil)
	expectedPoolRune := cosmos.NewUint(23789 * common.One).Sub(cosmos.NewUint(3946 * common.One)).Add(slashAmt)
	c.Assert(p.BalanceRune.Equal(expectedPoolRune), Equals, true, Commentf("expect %s however we got %s", expectedPoolRune, p.BalanceRune))
	expectedPoolBNB := cosmos.NewUint(167 * common.One).Sub(cosmos.NewUint(27 * common.One))
	c.Assert(p.BalanceAsset.Equal(expectedPoolBNB), Equals, true, Commentf("expected BNB in pool %s , however we got %s", expectedPoolBNB, p.BalanceAsset))
}

func (s *HelperSuite) TestEnableNextPool(c *C) {
	var err error
	ctx, k := setupKeeperForTest(c)
	eventMgr := NewDummyEventMgr()
	c.Assert(err, IsNil)
	pool := NewPool()
	pool.Asset = common.BNBAsset
	pool.Status = PoolAvailable
	pool.BalanceRune = cosmos.NewUint(100 * common.One)
	pool.BalanceAsset = cosmos.NewUint(100 * common.One)
	c.Assert(k.SetPool(ctx, pool), IsNil)

	pool = NewPool()
	pool.Asset = common.BTCAsset
	pool.Status = PoolStaged
	pool.BalanceRune = cosmos.NewUint(50 * common.One)
	pool.BalanceAsset = cosmos.NewUint(50 * common.One)
	c.Assert(k.SetPool(ctx, pool), IsNil)

	ethAsset, err := common.NewAsset("ETH.ETH")
	c.Assert(err, IsNil)
	pool = NewPool()
	pool.Asset = ethAsset
	pool.Status = PoolStaged
	pool.BalanceRune = cosmos.NewUint(40 * common.One)
	pool.BalanceAsset = cosmos.NewUint(40 * common.One)
	c.Assert(k.SetPool(ctx, pool), IsNil)

	xmrAsset, err := common.NewAsset("XMR.XMR")
	c.Assert(err, IsNil)
	pool = NewPool()
	pool.Asset = xmrAsset
	pool.Status = PoolStaged
	pool.BalanceRune = cosmos.NewUint(40 * common.One)
	pool.BalanceAsset = cosmos.NewUint(0 * common.One)
	c.Assert(k.SetPool(ctx, pool), IsNil)

	// usdAsset
	usdAsset, err := common.NewAsset("BNB.TUSDB")
	c.Assert(err, IsNil)
	pool = NewPool()
	pool.Asset = usdAsset
	pool.Status = PoolStaged
	pool.BalanceRune = cosmos.NewUint(140 * common.One)
	pool.BalanceAsset = cosmos.NewUint(0 * common.One)
	c.Assert(k.SetPool(ctx, pool), IsNil)
	// should enable BTC
	c.Assert(cyclePools(ctx, 100, 1, 0, k, eventMgr), IsNil)
	pool, err = k.GetPool(ctx, common.BTCAsset)
	c.Check(pool.Status, Equals, PoolAvailable)

	// should enable ETH
	c.Assert(cyclePools(ctx, 100, 1, 0, k, eventMgr), IsNil)
	pool, err = k.GetPool(ctx, ethAsset)
	c.Check(pool.Status, Equals, PoolAvailable)

	// should NOT enable XMR, since it has no assets
	c.Assert(cyclePools(ctx, 100, 1, 10*common.One, k, eventMgr), IsNil)
	pool, err = k.GetPool(ctx, xmrAsset)
	c.Assert(pool.IsEmpty(), Equals, false)
	c.Check(pool.Status, Equals, PoolStaged)
	c.Check(pool.BalanceRune.Uint64(), Equals, uint64(30*common.One))
}

func (s *HelperSuite) TestAbandonPool(c *C) {
	ctx, k := setupKeeperForTest(c)
	eventMgr := NewDummyEventMgr()
	usdAsset, err := common.NewAsset("BNB.TUSDB")
	c.Assert(err, IsNil)
	pool := NewPool()
	pool.Asset = usdAsset
	pool.Status = PoolStaged
	pool.BalanceRune = cosmos.NewUint(100 * common.One)
	pool.BalanceAsset = cosmos.NewUint(100 * common.One)
	c.Assert(k.SetPool(ctx, pool), IsNil)

	vault := GetRandomVault()
	vault.Coins = common.Coins{
		common.NewCoin(usdAsset, cosmos.NewUint(100*common.One)),
		common.NewCoin(common.BNBAsset, cosmos.NewUint(100*common.One)),
	}
	c.Assert(k.SetVault(ctx, vault), IsNil)

	runeAddr := GetRandomRUNEAddress()
	bnbAddr := GetRandomBNBAddress()
	lp := LiquidityProvider{
		Asset:        usdAsset,
		RuneAddress:  runeAddr,
		AssetAddress: bnbAddr,
		Units:        cosmos.ZeroUint(),
		PendingRune:  cosmos.ZeroUint(),
		PendingAsset: cosmos.ZeroUint(),
	}
	k.SetLiquidityProvider(ctx, lp)
	accAddr, err := runeAddr.AccAddress()
	c.Assert(err, IsNil)
	c.Assert(k.AddOwnership(ctx, common.NewCoin(usdAsset.LiquidityAsset(), cosmos.NewUint(1000)), accAddr), IsNil)
	c.Assert(k.GetLiquidityProviderBalance(ctx, usdAsset.LiquidityAsset(), accAddr).Uint64(), Equals, uint64(1000))

	// cycle pools
	c.Assert(cyclePools(ctx, 100, 1, 100*common.One, k, eventMgr), IsNil)

	// check pool was deleted
	pool, err = k.GetPool(ctx, usdAsset)
	c.Assert(pool.BalanceRune.IsZero(), Equals, true)
	c.Assert(pool.BalanceAsset.IsZero(), Equals, true)

	// check vault remove pool asset
	vault, err = k.GetVault(ctx, vault.PubKey)
	c.Assert(err, IsNil)
	c.Assert(vault.HasAsset(usdAsset), Equals, false)
	c.Assert(vault.CoinLength(), Equals, 1)

	// check that liquidity provider got removed
	count := 0
	iterator := k.GetLiquidityProviderIterator(ctx, usdAsset)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		count++
	}
	c.Assert(count, Equals, 0)

	// check that pool ownership tokens are removed
	c.Assert(k.GetLiquidityProviderBalance(ctx, usdAsset.LiquidityAsset(), accAddr).IsZero(), Equals, true)
}

type addGasFeesKeeperHelper struct {
	keeper.Keeper
	errGetNetwork bool
	errSetNetwork bool
	errGetPool    bool
	errSetPool    bool
	errSetEvent   bool
}

func newAddGasFeesKeeperHelper(keeper keeper.Keeper) *addGasFeesKeeperHelper {
	return &addGasFeesKeeperHelper{
		Keeper: keeper,
	}
}

func (h *addGasFeesKeeperHelper) GetNetwork(ctx cosmos.Context) (Network, error) {
	if h.errGetNetwork {
		return Network{}, kaboom
	}
	return h.Keeper.GetNetwork(ctx)
}

func (h *addGasFeesKeeperHelper) SetNetwork(ctx cosmos.Context, data Network) error {
	if h.errSetNetwork {
		return kaboom
	}
	return h.Keeper.SetNetwork(ctx, data)
}

func (h *addGasFeesKeeperHelper) SetPool(ctx cosmos.Context, pool Pool) error {
	if h.errSetPool {
		return kaboom
	}
	return h.Keeper.SetPool(ctx, pool)
}

func (h *addGasFeesKeeperHelper) GetPool(ctx cosmos.Context, asset common.Asset) (Pool, error) {
	if h.errGetPool {
		return Pool{}, kaboom
	}
	return h.Keeper.GetPool(ctx, asset)
}

type addGasFeeTestHelper struct {
	ctx        cosmos.Context
	k          *addGasFeesKeeperHelper
	na         NodeAccount
	gasManager GasManager
}

func newAddGasFeeTestHelper(c *C) addGasFeeTestHelper {
	ctx, k := setupKeeperForTest(c)
	keeper := newAddGasFeesKeeperHelper(k)
	pool := NewPool()
	pool.Asset = common.BNBAsset
	pool.BalanceAsset = cosmos.NewUint(100 * common.One)
	pool.BalanceRune = cosmos.NewUint(100 * common.One)
	pool.Status = PoolAvailable
	c.Assert(k.SetPool(ctx, pool), IsNil)

	poolBTC := NewPool()
	poolBTC.Asset = common.BTCAsset
	poolBTC.BalanceAsset = cosmos.NewUint(100 * common.One)
	poolBTC.BalanceRune = cosmos.NewUint(100 * common.One)
	poolBTC.Status = PoolAvailable
	c.Assert(k.SetPool(ctx, poolBTC), IsNil)

	na := GetRandomNodeAccount(NodeActive)
	c.Assert(k.SetNodeAccount(ctx, na), IsNil)
	yggVault := NewVault(common.BlockHeight(ctx), ActiveVault, YggdrasilVault, na.PubKeySet.Secp256k1, common.Chains{common.BNBChain})
	c.Assert(k.SetVault(ctx, yggVault), IsNil)
	version := constants.SWVersion
	constAccessor := constants.GetConstantValues(version)
	return addGasFeeTestHelper{
		ctx:        ctx,
		k:          keeper,
		na:         na,
		gasManager: NewGasMgrV1(constAccessor, keeper),
	}
}

func (s *HelperSuite) TestAddGasFees(c *C) {
	testCases := []struct {
		name        string
		txCreator   func(helper addGasFeeTestHelper) ObservedTx
		runner      func(helper addGasFeeTestHelper, tx ObservedTx) error
		expectError bool
		validator   func(helper addGasFeeTestHelper, c *C)
	}{
		{
			name: "empty Gas should just return nil",
			txCreator: func(helper addGasFeeTestHelper) ObservedTx {
				return GetRandomObservedTx()
			},

			expectError: false,
		},
		{
			name: "normal BNB gas",
			txCreator: func(helper addGasFeeTestHelper) ObservedTx {
				tx := ObservedTx{
					Tx: common.Tx{
						ID:          GetRandomTxHash(),
						Chain:       common.BNBChain,
						FromAddress: GetRandomBNBAddress(),
						ToAddress:   GetRandomBNBAddress(),
						Coins: common.Coins{
							common.NewCoin(common.BNBAsset, cosmos.NewUint(5*common.One)),
							common.NewCoin(common.RuneAsset(), cosmos.NewUint(8*common.One)),
						},
						Gas: common.Gas{
							common.NewCoin(common.BNBAsset, BNBGasFeeSingleton[0].Amount),
						},
						Memo: "",
					},
					Status:         types.Done,
					OutHashes:      nil,
					BlockHeight:    common.BlockHeight(helper.ctx),
					Signers:        []cosmos.AccAddress{helper.na.NodeAddress},
					ObservedPubKey: helper.na.PubKeySet.Secp256k1,
				}
				return tx
			},
			runner: func(helper addGasFeeTestHelper, tx ObservedTx) error {
				return AddGasFees(helper.ctx, helper.k, tx, helper.gasManager)
			},
			expectError: false,
			validator: func(helper addGasFeeTestHelper, c *C) {
				expected := common.NewCoin(common.BNBAsset, BNBGasFeeSingleton[0].Amount)
				c.Assert(helper.gasManager.GetGas(), HasLen, 1)
				c.Assert(helper.gasManager.GetGas()[0].Equals(expected), Equals, true)
			},
		},
		{
			name: "normal BTC gas",
			txCreator: func(helper addGasFeeTestHelper) ObservedTx {
				tx := ObservedTx{
					Tx: common.Tx{
						ID:          GetRandomTxHash(),
						Chain:       common.BTCChain,
						FromAddress: GetRandomBTCAddress(),
						ToAddress:   GetRandomBTCAddress(),
						Coins: common.Coins{
							common.NewCoin(common.BTCAsset, cosmos.NewUint(5*common.One)),
						},
						Gas: common.Gas{
							common.NewCoin(common.BTCAsset, cosmos.NewUint(2000)),
						},
						Memo: "",
					},
					Status:         types.Done,
					OutHashes:      nil,
					BlockHeight:    common.BlockHeight(helper.ctx),
					Signers:        []cosmos.AccAddress{helper.na.NodeAddress},
					ObservedPubKey: helper.na.PubKeySet.Secp256k1,
				}
				return tx
			},
			runner: func(helper addGasFeeTestHelper, tx ObservedTx) error {
				return AddGasFees(helper.ctx, helper.k, tx, helper.gasManager)
			},
			expectError: false,
			validator: func(helper addGasFeeTestHelper, c *C) {
				expected := common.NewCoin(common.BTCAsset, cosmos.NewUint(2000))
				c.Assert(helper.gasManager.GetGas(), HasLen, 1)
				c.Assert(helper.gasManager.GetGas()[0].Equals(expected), Equals, true)
			},
		},
	}
	for _, tc := range testCases {
		helper := newAddGasFeeTestHelper(c)
		tx := tc.txCreator(helper)
		var err error
		if tc.runner == nil {
			err = AddGasFees(helper.ctx, helper.k, tx, helper.gasManager)
		} else {
			err = tc.runner(helper, tx)
		}

		if err != nil && !tc.expectError {
			c.Errorf("test case: %s,didn't expect error however it got : %s", tc.name, err)
			c.FailNow()
		}
		if err == nil && tc.expectError {
			c.Errorf("test case: %s, expect error however it didn't", tc.name)
			c.FailNow()
		}
		if !tc.expectError && tc.validator != nil {
			tc.validator(helper, c)
			continue
		}
	}
}
