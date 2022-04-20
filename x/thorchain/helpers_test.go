package thorchain

import (
	"fmt"
	"strings"

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
	return Vault{}, errKaboom
}

func (k *TestRefundBondKeeper) GetLeastSecure(ctx cosmos.Context, vaults Vaults, signingTransPeriod int64) Vault {
	return vaults[0]
}

func (k *TestRefundBondKeeper) GetPool(_ cosmos.Context, asset common.Asset) (Pool, error) {
	if k.pool.Asset.Equals(asset) {
		return k.pool, nil
	}
	return NewPool(), errKaboom
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
	return errKaboom
}

func (k *TestRefundBondKeeper) DeleteVault(_ cosmos.Context, key common.PubKey) error {
	if k.ygg.PubKey.Equals(key) {
		k.ygg = NewVault(1, InactiveVault, AsgardVault, GetRandomPubKey(), common.Chains{common.BNBChain}.Strings(), []ChainContract{})
	}
	return nil
}

func (k *TestRefundBondKeeper) SetVault(ctx cosmos.Context, vault Vault) error {
	if k.ygg.PubKey.Equals(vault.PubKey) {
		k.ygg = vault
	}
	return nil
}

func (k *TestRefundBondKeeper) SetBondProviders(ctx cosmos.Context, _ BondProviders) error {
	return nil
}

func (k *TestRefundBondKeeper) GetBondProviders(ctx cosmos.Context, add cosmos.AccAddress) (BondProviders, error) {
	return BondProviders{}, nil
}

func (s *HelperSuite) TestSubsidizePoolWithSlashBond(c *C) {
	ctx, mgr := setupManagerForTest(c)
	ygg := GetRandomVault()
	c.Assert(subsidizePoolWithSlashBond(ctx, ygg, cosmos.NewUint(100*common.One), cosmos.ZeroUint(), mgr), IsNil)
	poolBNB := NewPool()
	poolBNB.Asset = common.BNBAsset
	poolBNB.BalanceRune = cosmos.NewUint(100 * common.One)
	poolBNB.BalanceAsset = cosmos.NewUint(100 * common.One)
	poolBNB.Status = PoolAvailable
	c.Assert(mgr.Keeper().SetPool(ctx, poolBNB), IsNil)

	poolTCAN := NewPool()
	tCanAsset, err := common.NewAsset("BNB.TCAN-014")
	c.Assert(err, IsNil)
	poolTCAN.Asset = tCanAsset
	poolTCAN.BalanceRune = cosmos.NewUint(200 * common.One)
	poolTCAN.BalanceAsset = cosmos.NewUint(200 * common.One)
	poolTCAN.Status = PoolAvailable
	c.Assert(mgr.Keeper().SetPool(ctx, poolTCAN), IsNil)

	poolBTC := NewPool()
	poolBTC.Asset = common.BTCAsset
	poolBTC.BalanceAsset = cosmos.NewUint(300 * common.One)
	poolBTC.BalanceRune = cosmos.NewUint(300 * common.One)
	poolBTC.Status = PoolAvailable
	c.Assert(mgr.Keeper().SetPool(ctx, poolBTC), IsNil)
	ygg.Type = YggdrasilVault
	ygg.Coins = common.Coins{
		common.NewCoin(common.RuneAsset(), cosmos.NewUint(1*common.One)),
		common.NewCoin(common.BNBAsset, cosmos.NewUint(1*common.One)),            // 1
		common.NewCoin(tCanAsset, cosmos.NewUint(common.One).QuoUint64(2)),       // 0.5 TCAN
		common.NewCoin(common.BTCAsset, cosmos.NewUint(common.One).QuoUint64(4)), // 0.25 BTC
	}
	totalRuneLeft, err := getTotalYggValueInRune(ctx, mgr.Keeper(), ygg)
	c.Assert(err, IsNil)

	totalRuneStolen := ygg.GetCoin(common.RuneAsset()).Amount
	slashAmt := totalRuneLeft.MulUint64(3).QuoUint64(2)
	c.Assert(subsidizePoolWithSlashBond(ctx, ygg, totalRuneLeft, slashAmt, mgr), IsNil)

	slashAmt = common.SafeSub(slashAmt, totalRuneStolen)
	totalRuneLeft = common.SafeSub(totalRuneLeft, totalRuneStolen)

	amountBNBForBNBPool := slashAmt.Mul(poolBNB.AssetValueInRune(cosmos.NewUint(common.One))).Quo(totalRuneLeft)
	runeBNB := poolBNB.BalanceRune.Add(amountBNBForBNBPool)
	bnbPoolAsset := poolBNB.BalanceAsset.Sub(cosmos.NewUint(common.One))
	poolBNB, err = mgr.Keeper().GetPool(ctx, common.BNBAsset)
	c.Assert(err, IsNil)
	c.Assert(poolBNB.BalanceRune.Equal(runeBNB), Equals, true)
	c.Assert(poolBNB.BalanceAsset.Equal(bnbPoolAsset), Equals, true)
	amountRuneForTCANPool := slashAmt.Mul(poolTCAN.AssetValueInRune(cosmos.NewUint(common.One).QuoUint64(2))).Quo(totalRuneLeft)
	runeTCAN := poolTCAN.BalanceRune.Add(amountRuneForTCANPool)
	tcanPoolAsset := poolTCAN.BalanceAsset.Sub(cosmos.NewUint(common.One).QuoUint64(2))
	poolTCAN, err = mgr.Keeper().GetPool(ctx, tCanAsset)
	c.Assert(err, IsNil)
	c.Assert(poolTCAN.BalanceRune.Equal(runeTCAN), Equals, true)
	c.Assert(poolTCAN.BalanceAsset.Equal(tcanPoolAsset), Equals, true)
	amountRuneForBTCPool := slashAmt.Mul(poolBTC.AssetValueInRune(cosmos.NewUint(common.One).QuoUint64(4))).Quo(totalRuneLeft)
	runeBTC := poolBTC.BalanceRune.Add(amountRuneForBTCPool)
	btcPoolAsset := poolBTC.BalanceAsset.Sub(cosmos.NewUint(common.One).QuoUint64(4))
	poolBTC, err = mgr.Keeper().GetPool(ctx, common.BTCAsset)
	c.Assert(err, IsNil)
	c.Assert(poolBTC.BalanceRune.Equal(runeBTC), Equals, true)
	c.Assert(poolBTC.BalanceAsset.Equal(btcPoolAsset), Equals, true)

	ygg1 := GetRandomVault()
	ygg1.Type = YggdrasilVault
	ygg1.Coins = common.Coins{
		common.NewCoin(tCanAsset, cosmos.NewUint(common.One*2)),       // 2 TCAN
		common.NewCoin(common.BTCAsset, cosmos.NewUint(common.One*4)), // 4 BTC
	}
	totalRuneLeft, err = getTotalYggValueInRune(ctx, mgr.Keeper(), ygg1)
	c.Assert(err, IsNil)
	slashAmt = cosmos.NewUint(100 * common.One)
	c.Assert(subsidizePoolWithSlashBond(ctx, ygg1, totalRuneLeft, slashAmt, mgr), IsNil)
	amountRuneForTCANPool = slashAmt.Mul(poolTCAN.AssetValueInRune(cosmos.NewUint(common.One * 2))).Quo(totalRuneLeft)
	runeTCAN = poolTCAN.BalanceRune.Add(amountRuneForTCANPool)
	poolTCAN, err = mgr.Keeper().GetPool(ctx, tCanAsset)
	c.Assert(err, IsNil)
	c.Assert(poolTCAN.BalanceRune.Equal(runeTCAN), Equals, true)
	amountRuneForBTCPool = slashAmt.Mul(poolBTC.AssetValueInRune(cosmos.NewUint(common.One * 4))).Quo(totalRuneLeft)
	runeBTC = poolBTC.BalanceRune.Add(amountRuneForBTCPool)
	poolBTC, err = mgr.Keeper().GetPool(ctx, common.BTCAsset)
	c.Assert(err, IsNil)
	c.Assert(poolBTC.BalanceRune.Equal(runeBTC), Equals, true)

	ygg2 := GetRandomVault()
	ygg2.Type = YggdrasilVault
	ygg2.Coins = common.Coins{
		common.NewCoin(common.RuneAsset(), cosmos.NewUint(2*common.One)),
		common.NewCoin(tCanAsset, cosmos.NewUint(0)),
	}
	totalRuneLeft, err = getTotalYggValueInRune(ctx, mgr.Keeper(), ygg2)
	c.Assert(err, IsNil)
	slashAmt = cosmos.NewUint(2 * common.One)
	c.Assert(subsidizePoolWithSlashBond(ctx, ygg2, totalRuneLeft, slashAmt, mgr), IsNil)
}

func (s *HelperSuite) TestPausedLP(c *C) {
	ctx, mgr := setupManagerForTest(c)

	c.Check(isLPPaused(ctx, common.BNBChain, mgr), Equals, false)
	c.Check(isLPPaused(ctx, common.BTCChain, mgr), Equals, false)

	mgr.Keeper().SetMimir(ctx, "PauseLPBTC", 1)
	c.Check(isLPPaused(ctx, common.BTCChain, mgr), Equals, true)

	mgr.Keeper().SetMimir(ctx, "PauseLP", 1)
	c.Check(isLPPaused(ctx, common.BNBChain, mgr), Equals, true)
}

func (s *HelperSuite) TestRefundBondError(c *C) {
	ctx, _ := setupKeeperForTest(c)
	// active node should not refund bond
	pk := GetRandomPubKey()
	na := GetRandomValidatorNode(NodeActive)
	na.PubKeySet.Secp256k1 = pk
	na.Bond = cosmos.NewUint(100 * common.One)
	tx := GetRandomTx()
	tx.FromAddress = GetRandomTHORAddress()
	keeper1 := &TestRefundBondKeeper{}
	mgr := NewDummyMgrWithKeeper(keeper1)
	c.Assert(refundBond(ctx, tx, na.NodeAddress, cosmos.ZeroUint(), &na, mgr), IsNil)

	// fail to get vault should return an error
	na.UpdateStatus(NodeStandby, common.BlockHeight(ctx))
	keeper1.na = na
	mgr.K = keeper1
	c.Assert(refundBond(ctx, tx, na.NodeAddress, cosmos.ZeroUint(), &na, mgr), NotNil)

	// if the vault is not a yggdrasil pool , it should return an error
	ygg := NewVault(common.BlockHeight(ctx), ActiveVault, AsgardVault, pk, common.Chains{common.BNBChain}.Strings(), []ChainContract{})
	ygg.Coins = common.Coins{}
	keeper1.ygg = ygg
	mgr.K = keeper1
	c.Assert(refundBond(ctx, tx, na.NodeAddress, cosmos.ZeroUint(), &na, mgr), NotNil)

	// fail to get pool should fail
	ygg = NewVault(common.BlockHeight(ctx), ActiveVault, YggdrasilVault, pk, common.Chains{common.BNBChain}.Strings(), []ChainContract{})
	ygg.Coins = common.Coins{
		common.NewCoin(common.RuneAsset(), cosmos.NewUint(27*common.One)),
		common.NewCoin(common.BNBAsset, cosmos.NewUint(27*common.One)),
	}
	keeper1.ygg = ygg
	mgr.K = keeper1
	c.Assert(refundBond(ctx, tx, na.NodeAddress, cosmos.ZeroUint(), &na, mgr), NotNil)

	// when ygg asset in RUNE is more then bond , thorchain should slash the node account with all their bond
	keeper1.pool = Pool{
		Asset:        common.BNBAsset,
		BalanceRune:  cosmos.NewUint(1024 * common.One),
		BalanceAsset: cosmos.NewUint(167 * common.One),
	}
	mgr.K = keeper1
	c.Assert(refundBond(ctx, tx, na.NodeAddress, cosmos.ZeroUint(), &na, mgr), IsNil)
	// make sure no tx has been generated for refund
	items, err := mgr.TxOutStore().GetOutboundItems(ctx)
	c.Assert(err, IsNil)
	c.Check(items, HasLen, 0)
}

func (s *HelperSuite) TestRefundBondHappyPath(c *C) {
	ctx, _ := setupKeeperForTest(c)
	na := GetRandomValidatorNode(NodeActive)
	na.Bond = cosmos.NewUint(12098 * common.One)
	pk := GetRandomPubKey()
	na.PubKeySet.Secp256k1 = pk
	ygg := NewVault(common.BlockHeight(ctx), ActiveVault, YggdrasilVault, pk, common.Chains{common.BNBChain}.Strings(), []ChainContract{})

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
	mgr := NewDummyMgrWithKeeper(keeper)
	tx := GetRandomTx()
	tx.FromAddress, _ = common.NewAddress(na.BondAddress.String())
	yggAssetInRune, err := getTotalYggValueInRune(ctx, keeper, ygg)
	c.Assert(err, IsNil)
	err = refundBond(ctx, tx, na.NodeAddress, cosmos.ZeroUint(), &na, mgr)
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

func (s *HelperSuite) TestRefundBondDisableRequestToLeaveNode(c *C) {
	ctx, _ := setupKeeperForTest(c)
	na := GetRandomValidatorNode(NodeActive)
	na.Bond = cosmos.NewUint(12098 * common.One)
	pk := GetRandomPubKey()
	na.PubKeySet.Secp256k1 = pk
	ygg := NewVault(common.BlockHeight(ctx), ActiveVault, YggdrasilVault, pk, common.Chains{common.BNBChain}.Strings(), []ChainContract{})

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
	na.RequestedToLeave = true
	mgr := NewDummyMgrWithKeeper(keeper)
	tx := GetRandomTx()
	yggAssetInRune, err := getTotalYggValueInRune(ctx, keeper, ygg)
	c.Assert(err, IsNil)
	err = refundBond(ctx, tx, na.NodeAddress, cosmos.ZeroUint(), &na, mgr)
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
	c.Assert(keeper.na.Status == NodeDisabled, Equals, true)
}

func (s *HelperSuite) TestEnableNextPool(c *C) {
	var err error
	ctx, k := setupKeeperForTest(c)
	mgr := NewDummyMgrWithKeeper(k)
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
	c.Assert(cyclePools(ctx, 100, 1, 0, mgr), IsNil)
	pool, err = k.GetPool(ctx, common.BTCAsset)
	c.Assert(err, IsNil)
	c.Check(pool.Status, Equals, PoolAvailable)

	// should enable ETH
	c.Assert(cyclePools(ctx, 100, 1, 0, mgr), IsNil)
	pool, err = k.GetPool(ctx, ethAsset)
	c.Assert(err, IsNil)
	c.Check(pool.Status, Equals, PoolAvailable)

	// should NOT enable XMR, since it has no assets
	c.Assert(cyclePools(ctx, 100, 1, 10*common.One, mgr), IsNil)
	pool, err = k.GetPool(ctx, xmrAsset)
	c.Assert(err, IsNil)
	c.Assert(pool.IsEmpty(), Equals, false)
	c.Check(pool.Status, Equals, PoolStaged)
	c.Check(pool.BalanceRune.Uint64(), Equals, uint64(30*common.One))
}

func (s *HelperSuite) TestAbandonPool(c *C) {
	ctx, k := setupKeeperForTest(c)
	mgr := NewDummyMgrWithKeeper(k)
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

	// cycle pools
	c.Assert(cyclePools(ctx, 100, 1, 100*common.One, mgr), IsNil)

	// check pool was deleted
	pool, err = k.GetPool(ctx, usdAsset)
	c.Assert(err, IsNil)
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
}

func (s *HelperSuite) TestDollarInRune(c *C) {
	ctx, k := setupKeeperForTest(c)
	mgr := NewDummyMgrWithKeeper(k)
	busd, err := common.NewAsset("BNB.BUSD-BD1")
	c.Assert(err, IsNil)
	pool := NewPool()
	pool.Asset = busd
	pool.Status = PoolAvailable
	pool.BalanceRune = cosmos.NewUint(85515078103667)
	pool.BalanceAsset = cosmos.NewUint(709802235538353)
	c.Assert(k.SetPool(ctx, pool), IsNil)

	runeUSDPrice := telem(DollarInRune(ctx, mgr))
	c.Assert(runeUSDPrice, Equals, float32(0.12047733))
}

func (s *HelperSuite) TestTelem(c *C) {
	value := cosmos.NewUint(12047733)
	c.Assert(value.Uint64(), Equals, uint64(12047733))
	c.Assert(telem(value), Equals, float32(0.12047733))
}

type addGasFeesKeeperHelper struct {
	keeper.Keeper
	errGetNetwork bool
	errSetNetwork bool
	errGetPool    bool
	errSetPool    bool
}

func newAddGasFeesKeeperHelper(keeper keeper.Keeper) *addGasFeesKeeperHelper {
	return &addGasFeesKeeperHelper{
		Keeper: keeper,
	}
}

func (h *addGasFeesKeeperHelper) GetNetwork(ctx cosmos.Context) (Network, error) {
	if h.errGetNetwork {
		return Network{}, errKaboom
	}
	return h.Keeper.GetNetwork(ctx)
}

func (h *addGasFeesKeeperHelper) SetNetwork(ctx cosmos.Context, data Network) error {
	if h.errSetNetwork {
		return errKaboom
	}
	return h.Keeper.SetNetwork(ctx, data)
}

func (h *addGasFeesKeeperHelper) SetPool(ctx cosmos.Context, pool Pool) error {
	if h.errSetPool {
		return errKaboom
	}
	return h.Keeper.SetPool(ctx, pool)
}

func (h *addGasFeesKeeperHelper) GetPool(ctx cosmos.Context, asset common.Asset) (Pool, error) {
	if h.errGetPool {
		return Pool{}, errKaboom
	}
	return h.Keeper.GetPool(ctx, asset)
}

type addGasFeeTestHelper struct {
	ctx cosmos.Context
	na  NodeAccount
	mgr Manager
}

func newAddGasFeeTestHelper(c *C) addGasFeeTestHelper {
	ctx, mgr := setupManagerForTest(c)
	keeper := newAddGasFeesKeeperHelper(mgr.Keeper())
	mgr.K = keeper
	pool := NewPool()
	pool.Asset = common.BNBAsset
	pool.BalanceAsset = cosmos.NewUint(100 * common.One)
	pool.BalanceRune = cosmos.NewUint(100 * common.One)
	pool.Status = PoolAvailable
	c.Assert(mgr.Keeper().SetPool(ctx, pool), IsNil)

	poolBTC := NewPool()
	poolBTC.Asset = common.BTCAsset
	poolBTC.BalanceAsset = cosmos.NewUint(100 * common.One)
	poolBTC.BalanceRune = cosmos.NewUint(100 * common.One)
	poolBTC.Status = PoolAvailable
	c.Assert(mgr.Keeper().SetPool(ctx, poolBTC), IsNil)

	na := GetRandomValidatorNode(NodeActive)
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, na), IsNil)
	yggVault := NewVault(common.BlockHeight(ctx), ActiveVault, YggdrasilVault, na.PubKeySet.Secp256k1, common.Chains{common.BNBChain}.Strings(), []ChainContract{})
	c.Assert(mgr.Keeper().SetVault(ctx, yggVault), IsNil)
	version := GetCurrentVersion()
	constAccessor := constants.GetConstantValues(version)
	mgr.gasMgr = newGasMgrV81(constAccessor, keeper)
	return addGasFeeTestHelper{
		ctx: ctx,
		mgr: mgr,
		na:  na,
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
					Status:         types.Status_done,
					OutHashes:      nil,
					BlockHeight:    common.BlockHeight(helper.ctx),
					Signers:        []string{helper.na.NodeAddress.String()},
					ObservedPubKey: helper.na.PubKeySet.Secp256k1,
				}
				return tx
			},
			runner: func(helper addGasFeeTestHelper, tx ObservedTx) error {
				return addGasFees(helper.ctx, helper.mgr, tx)
			},
			expectError: false,
			validator: func(helper addGasFeeTestHelper, c *C) {
				expected := common.NewCoin(common.BNBAsset, BNBGasFeeSingleton[0].Amount)
				c.Assert(helper.mgr.GasMgr().GetGas(), HasLen, 1)
				c.Assert(helper.mgr.GasMgr().GetGas()[0].Equals(expected), Equals, true)
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
					Status:         types.Status_done,
					OutHashes:      nil,
					BlockHeight:    common.BlockHeight(helper.ctx),
					Signers:        []string{helper.na.NodeAddress.String()},
					ObservedPubKey: helper.na.PubKeySet.Secp256k1,
				}
				return tx
			},
			runner: func(helper addGasFeeTestHelper, tx ObservedTx) error {
				return addGasFees(helper.ctx, helper.mgr, tx)
			},
			expectError: false,
			validator: func(helper addGasFeeTestHelper, c *C) {
				expected := common.NewCoin(common.BTCAsset, cosmos.NewUint(2000))
				c.Assert(helper.mgr.GasMgr().GetGas(), HasLen, 1)
				c.Assert(helper.mgr.GasMgr().GetGas()[0].Equals(expected), Equals, true)
			},
		},
	}
	for _, tc := range testCases {
		helper := newAddGasFeeTestHelper(c)
		tx := tc.txCreator(helper)
		var err error
		if tc.runner == nil {
			err = addGasFees(helper.ctx, helper.mgr, tx)
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

func (s *HelperSuite) TestEmitPoolStageCostEvent(c *C) {
	ctx, mgr := setupManagerForTest(c)
	emitPoolBalanceChangedEvent(ctx,
		NewPoolMod(common.BTCAsset, cosmos.NewUint(1000), false, cosmos.ZeroUint(), false), "test", mgr)
	found := false
	for _, e := range ctx.EventManager().Events() {
		if strings.EqualFold(e.Type, types.PoolBalanceChangeEventType) {
			found = true
			break
		}
	}
	c.Assert(found, Equals, true)
}

func (s *HelperSuite) TestIsTradingHalt(c *C) {
	ctx, mgr := setupManagerForTest(c)
	txID := GetRandomTxHash()
	tx := common.NewTx(txID, GetRandomBTCAddress(), GetRandomBTCAddress(), common.NewCoins(common.NewCoin(common.BTCAsset, cosmos.NewUint(100))), common.Gas{
		common.NewCoin(common.BTCAsset, cosmos.NewUint(100)),
	}, "swap:BNB.BNB:"+GetRandomBNBAddress().String())
	memo, err := ParseMemoWithTHORNames(ctx, mgr.Keeper(), tx.Memo)
	c.Assert(err, IsNil)
	m, err := getMsgSwapFromMemo(memo.(SwapMemo), NewObservedTx(tx, common.BlockHeight(ctx), GetRandomPubKey(), common.BlockHeight(ctx)), GetRandomBech32Addr())
	c.Assert(err, IsNil)

	txAddLiquidity := common.NewTx(txID, GetRandomBTCAddress(), GetRandomBTCAddress(), common.NewCoins(common.NewCoin(common.BTCAsset, cosmos.NewUint(100))), common.Gas{
		common.NewCoin(common.BTCAsset, cosmos.NewUint(100)),
	}, "add:BTC.BTC:"+GetRandomTHORAddress().String())
	memoAddExternal, err := ParseMemoWithTHORNames(ctx, mgr.Keeper(), txAddLiquidity.Memo)
	c.Assert(err, IsNil)
	mAddExternal, err := getMsgAddLiquidityFromMemo(ctx,
		memoAddExternal.(AddLiquidityMemo),
		NewObservedTx(txAddLiquidity, common.BlockHeight(ctx), GetRandomPubKey(), common.BlockHeight(ctx)),
		GetRandomBech32Addr())

	c.Assert(err, IsNil)
	txAddRUNE := common.NewTx(txID, GetRandomTHORAddress(), GetRandomTHORAddress(), common.NewCoins(common.NewCoin(common.RuneNative, cosmos.NewUint(100))), common.Gas{
		common.NewCoin(common.RuneNative, cosmos.NewUint(100)),
	}, "add:BTC.BTC:"+GetRandomBTCAddress().String())
	memoAddRUNE, err := ParseMemoWithTHORNames(ctx, mgr.Keeper(), txAddRUNE.Memo)
	c.Assert(err, IsNil)
	mAddRUNE, err := getMsgAddLiquidityFromMemo(ctx,
		memoAddRUNE.(AddLiquidityMemo),
		NewObservedTx(txAddRUNE, common.BlockHeight(ctx), GetRandomPubKey(), common.BlockHeight(ctx)),
		GetRandomBech32Addr())
	c.Assert(err, IsNil)

	mgr.Keeper().SetTHORName(ctx, THORName{
		Name:              "testtest",
		ExpireBlockHeight: common.BlockHeight(ctx) + 1024,
		Owner:             GetRandomBech32Addr(),
		PreferredAsset:    common.BNBAsset,
		Aliases: []THORNameAlias{
			{
				Chain:   common.BNBChain,
				Address: GetRandomBNBAddress(),
			},
		},
	})
	txWithThorname := common.NewTx(txID, GetRandomBTCAddress(), GetRandomBTCAddress(), common.NewCoins(common.NewCoin(common.BTCAsset, cosmos.NewUint(100))), common.Gas{
		common.NewCoin(common.BTCAsset, cosmos.NewUint(100)),
	}, "swap:BNB.BNB:testtest")
	memoWithThorname, err := ParseMemoWithTHORNames(ctx, mgr.Keeper(), txWithThorname.Memo)
	c.Assert(err, IsNil)
	mWithThorname, err := getMsgSwapFromMemo(memoWithThorname.(SwapMemo), NewObservedTx(txWithThorname, common.BlockHeight(ctx), GetRandomPubKey(), common.BlockHeight(ctx)), GetRandomBech32Addr())
	c.Assert(err, IsNil)

	txSynth := common.NewTx(txID, GetRandomTHORAddress(), GetRandomTHORAddress(),
		common.NewCoins(common.NewCoin(common.BNBAsset.GetSyntheticAsset(), cosmos.NewUint(100))),
		common.Gas{common.NewCoin(common.BNBAsset, cosmos.NewUint(100))},
		"swap:ETH.ETH:"+GetRandomTHORAddress().String())
	memoRedeemSynth, err := ParseMemoWithTHORNames(ctx, mgr.Keeper(), txSynth.Memo)
	c.Assert(err, IsNil)
	mRedeemSynth, err := getMsgSwapFromMemo(memoRedeemSynth.(SwapMemo), NewObservedTx(txSynth, common.BlockHeight(ctx), GetRandomPubKey(), common.BlockHeight(ctx)), GetRandomBech32Addr())
	c.Assert(err, IsNil)

	c.Assert(isTradingHalt(ctx, m, mgr), Equals, false)
	c.Assert(isTradingHalt(ctx, mAddExternal, mgr), Equals, false)
	c.Assert(isTradingHalt(ctx, mAddRUNE, mgr), Equals, false)
	c.Assert(isTradingHalt(ctx, mWithThorname, mgr), Equals, false)
	c.Assert(isTradingHalt(ctx, mRedeemSynth, mgr), Equals, false)

	mgr.Keeper().SetMimir(ctx, "HaltTrading", 1)
	c.Assert(isTradingHalt(ctx, m, mgr), Equals, true)
	c.Assert(isTradingHalt(ctx, mAddExternal, mgr), Equals, true)
	c.Assert(isTradingHalt(ctx, mAddRUNE, mgr), Equals, true)
	c.Assert(isTradingHalt(ctx, mWithThorname, mgr), Equals, true)
	c.Assert(isTradingHalt(ctx, mRedeemSynth, mgr), Equals, true)
	c.Assert(mgr.Keeper().DeleteMimir(ctx, "HaltTrading"), IsNil)

	mgr.Keeper().SetMimir(ctx, "HaltBNBTrading", 1)
	c.Assert(isTradingHalt(ctx, m, mgr), Equals, true)
	c.Assert(isTradingHalt(ctx, mAddExternal, mgr), Equals, false)
	c.Assert(isTradingHalt(ctx, mAddRUNE, mgr), Equals, false)
	c.Assert(isTradingHalt(ctx, mWithThorname, mgr), Equals, true)
	c.Assert(isTradingHalt(ctx, mRedeemSynth, mgr), Equals, true)
	c.Assert(mgr.Keeper().DeleteMimir(ctx, "HaltBNBTrading"), IsNil)

	mgr.Keeper().SetMimir(ctx, "HaltBTCTrading", 1)
	c.Assert(isTradingHalt(ctx, m, mgr), Equals, true)
	c.Assert(isTradingHalt(ctx, mAddExternal, mgr), Equals, true)
	c.Assert(isTradingHalt(ctx, mAddRUNE, mgr), Equals, true)
	c.Assert(isTradingHalt(ctx, mWithThorname, mgr), Equals, true)
	c.Assert(isTradingHalt(ctx, mRedeemSynth, mgr), Equals, false)
	c.Assert(mgr.Keeper().DeleteMimir(ctx, "HaltBTCTrading"), IsNil)

	mgr.Keeper().SetMimir(ctx, "SolvencyHaltBTCChain", 1)
	c.Assert(isTradingHalt(ctx, m, mgr), Equals, true)
	c.Assert(isTradingHalt(ctx, mAddExternal, mgr), Equals, true)
	c.Assert(isTradingHalt(ctx, mAddRUNE, mgr), Equals, true)
	c.Assert(isTradingHalt(ctx, mWithThorname, mgr), Equals, true)
	c.Assert(isTradingHalt(ctx, mRedeemSynth, mgr), Equals, false)
	c.Assert(mgr.Keeper().DeleteMimir(ctx, "SolvencyHaltBTCChain"), IsNil)
}

func (s *HelperSuite) TestUpdateTxOutGas(c *C) {
	ctx, mgr := setupManagerForTest(c)

	// Create ObservedVoter and add a TxOut
	txVoter := GetRandomObservedTxVoter()
	txOut := GetRandomTxOutItem()
	txVoter.Actions = append(txVoter.Actions, txOut)
	mgr.Keeper().SetObservedTxInVoter(ctx, txVoter)

	// Try to set new gas, should return error as TxOut InHash doesn't match
	newGas := common.Gas{common.NewCoin(common.LUNAAsset, cosmos.NewUint(2000000))}
	err := updateTxOutGas(ctx, mgr.K, txOut, newGas)
	c.Assert(err.Error(), Equals, fmt.Sprintf("Fail to find tx out in ObservedTxVoter %s", txOut.InHash))

	// Update TxOut InHash to match, should update gas
	txOut.InHash = txVoter.TxID
	txVoter.Actions[1] = txOut
	mgr.Keeper().SetObservedTxInVoter(ctx, txVoter)

	// Err should be Nil
	err = updateTxOutGas(ctx, mgr.K, txOut, newGas)
	c.Assert(err, IsNil)

	// Keeper should have updated gas of TxOut in Actions
	txVoter, err = mgr.Keeper().GetObservedTxInVoter(ctx, txVoter.TxID)
	c.Assert(err, IsNil)

	didUpdate := false
	for _, item := range txVoter.Actions {
		if item.Equals(txOut) && item.MaxGas.Equals(newGas) {
			didUpdate = true
			break
		}
	}

	c.Assert(didUpdate, Equals, true)
}
