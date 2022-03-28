package thorchain

import (
	"errors"

	. "gopkg.in/check.v1"

	"github.com/blang/semver"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

type NetworkManagerV76TestSuite struct{}

var _ = Suite(&NetworkManagerV76TestSuite{})

func (s *NetworkManagerV76TestSuite) SetUpSuite(c *C) {
	SetupConfigForTest()
}

type TestRagnarokChainKeeper struct {
	keeper.KVStoreDummy
	activeVault Vault
	retireVault Vault
	yggVault    Vault
	pools       Pools
	lps         LiquidityProviders
	na          NodeAccount
	err         error
}

func (k *TestRagnarokChainKeeper) ListValidatorsWithBond(_ cosmos.Context) (NodeAccounts, error) {
	return NodeAccounts{k.na}, k.err
}

func (k *TestRagnarokChainKeeper) ListActiveValidators(_ cosmos.Context) (NodeAccounts, error) {
	return NodeAccounts{k.na}, k.err
}

func (k *TestRagnarokChainKeeper) GetNodeAccount(ctx cosmos.Context, signer cosmos.AccAddress) (NodeAccount, error) {
	if k.na.NodeAddress.Equals(signer) {
		return k.na, nil
	}
	return NodeAccount{}, nil
}

func (k *TestRagnarokChainKeeper) GetAsgardVaultsByStatus(_ cosmos.Context, vt VaultStatus) (Vaults, error) {
	if vt == ActiveVault {
		return Vaults{k.activeVault}, k.err
	}
	return Vaults{k.retireVault}, k.err
}

func (k *TestRagnarokChainKeeper) VaultExists(_ cosmos.Context, _ common.PubKey) bool {
	return true
}

func (k *TestRagnarokChainKeeper) GetVault(_ cosmos.Context, _ common.PubKey) (Vault, error) {
	return k.yggVault, k.err
}

func (k *TestRagnarokChainKeeper) GetMostSecure(ctx cosmos.Context, vaults Vaults, signingTransPeriod int64) Vault {
	return vaults[0]
}

func (k *TestRagnarokChainKeeper) GetLeastSecure(ctx cosmos.Context, vaults Vaults, signingTransPeriod int64) Vault {
	return vaults[0]
}

func (k *TestRagnarokChainKeeper) GetPools(_ cosmos.Context) (Pools, error) {
	return k.pools, k.err
}

func (k *TestRagnarokChainKeeper) GetPool(_ cosmos.Context, asset common.Asset) (Pool, error) {
	for _, pool := range k.pools {
		if pool.Asset.Equals(asset) {
			return pool, nil
		}
	}
	return Pool{}, errors.New("pool not found")
}

func (k *TestRagnarokChainKeeper) SetPool(_ cosmos.Context, pool Pool) error {
	for i, p := range k.pools {
		if p.Asset.Equals(pool.Asset) {
			k.pools[i] = pool
		}
	}
	return k.err
}

func (k *TestRagnarokChainKeeper) PoolExist(_ cosmos.Context, _ common.Asset) bool {
	return true
}

func (k *TestRagnarokChainKeeper) GetLiquidityProviderIterator(ctx cosmos.Context, _ common.Asset) cosmos.Iterator {
	cdc := makeTestCodec()
	iter := keeper.NewDummyIterator()
	for _, lp := range k.lps {
		iter.AddItem([]byte("key"), cdc.MustMarshal(lp))
	}
	return iter
}

func (k *TestRagnarokChainKeeper) AddOwnership(ctx cosmos.Context, coin common.Coin, addr cosmos.AccAddress) error {
	lp, _ := common.NewAddress(addr.String())
	for i, skr := range k.lps {
		if lp.Equals(skr.RuneAddress) {
			k.lps[i].Units = k.lps[i].Units.Add(coin.Amount)
		}
	}
	return nil
}

func (k *TestRagnarokChainKeeper) RemoveOwnership(ctx cosmos.Context, coin common.Coin, addr cosmos.AccAddress) error {
	lp, _ := common.NewAddress(addr.String())
	for i, skr := range k.lps {
		if lp.Equals(skr.RuneAddress) {
			k.lps[i].Units = k.lps[i].Units.Sub(coin.Amount)
		}
	}
	return nil
}

func (k *TestRagnarokChainKeeper) GetLiquidityProvider(_ cosmos.Context, asset common.Asset, addr common.Address) (LiquidityProvider, error) {
	if asset.Equals(common.BTCAsset) {
		for i, lp := range k.lps {
			if addr.Equals(lp.RuneAddress) {
				return k.lps[i], k.err
			}
		}
	}
	return LiquidityProvider{}, k.err
}

func (k *TestRagnarokChainKeeper) SetLiquidityProvider(_ cosmos.Context, lp LiquidityProvider) {
	for i, skr := range k.lps {
		if lp.RuneAddress.Equals(skr.RuneAddress) {
			lp.Units = k.lps[i].Units
			k.lps[i] = lp
		}
	}
}

func (k *TestRagnarokChainKeeper) RemoveLiquidityProvider(_ cosmos.Context, lp LiquidityProvider) {
	for i, skr := range k.lps {
		if lp.RuneAddress.Equals(skr.RuneAddress) {
			k.lps[i] = lp
		}
	}
}

func (k *TestRagnarokChainKeeper) GetGas(_ cosmos.Context, _ common.Asset) ([]cosmos.Uint, error) {
	return []cosmos.Uint{cosmos.NewUint(10)}, k.err
}

func (k *TestRagnarokChainKeeper) GetLowestActiveVersion(_ cosmos.Context) semver.Version {
	return GetCurrentVersion()
}

func (k *TestRagnarokChainKeeper) AddFeeToReserve(_ cosmos.Context, _ cosmos.Uint) error {
	return k.err
}

func (k *TestRagnarokChainKeeper) IsActiveObserver(_ cosmos.Context, _ cosmos.AccAddress) bool {
	return true
}

func (s *NetworkManagerV76TestSuite) TestRagnarokChain(c *C) {
	ctx, _ := setupKeeperForTest(c)
	ctx = ctx.WithBlockHeight(100000)
	ver := GetCurrentVersion()
	constAccessor := constants.GetConstantValues(ver)

	activeVault := GetRandomVault()
	activeVault.StatusSince = common.BlockHeight(ctx) - 10
	activeVault.Coins = common.Coins{
		common.NewCoin(common.BNBAsset, cosmos.NewUint(100*common.One)),
	}
	retireVault := GetRandomVault()
	retireVault.Chains = common.Chains{common.BNBChain, common.BTCChain}.Strings()
	yggVault := GetRandomVault()
	yggVault.Type = YggdrasilVault
	yggVault.Coins = common.Coins{
		common.NewCoin(common.BTCAsset, cosmos.NewUint(3*common.One)),
		common.NewCoin(common.RuneAsset(), cosmos.NewUint(300*common.One)),
	}

	btcPool := NewPool()
	btcPool.Asset = common.BTCAsset
	btcPool.BalanceRune = cosmos.NewUint(1000 * common.One)
	btcPool.BalanceAsset = cosmos.NewUint(10 * common.One)
	btcPool.LPUnits = cosmos.NewUint(1600)

	bnbPool := NewPool()
	bnbPool.Asset = common.BNBAsset
	bnbPool.BalanceRune = cosmos.NewUint(1000 * common.One)
	bnbPool.BalanceAsset = cosmos.NewUint(10 * common.One)
	bnbPool.LPUnits = cosmos.NewUint(1600)

	addr := GetRandomRUNEAddress()
	lps := LiquidityProviders{
		{
			RuneAddress:       addr,
			AssetAddress:      GetRandomBTCAddress(),
			LastAddHeight:     5,
			Units:             btcPool.LPUnits.QuoUint64(2),
			PendingRune:       cosmos.ZeroUint(),
			PendingAsset:      cosmos.ZeroUint(),
			AssetDepositValue: cosmos.ZeroUint(),
			RuneDepositValue:  cosmos.ZeroUint(),
		},
		{
			RuneAddress:       GetRandomRUNEAddress(),
			AssetAddress:      GetRandomBTCAddress(),
			LastAddHeight:     10,
			Units:             btcPool.LPUnits.QuoUint64(2),
			PendingRune:       cosmos.ZeroUint(),
			PendingAsset:      cosmos.ZeroUint(),
			AssetDepositValue: cosmos.ZeroUint(),
			RuneDepositValue:  cosmos.ZeroUint(),
		},
	}

	keeper := &TestRagnarokChainKeeper{
		na:          GetRandomValidatorNode(NodeActive),
		activeVault: activeVault,
		retireVault: retireVault,
		yggVault:    yggVault,
		pools:       Pools{bnbPool, btcPool},
		lps:         lps,
	}

	mgr := NewDummyMgrWithKeeper(keeper)

	vaultMgr := newNetworkMgrV76(keeper, mgr.TxOutStore(), mgr.EventMgr())

	// the first round should just recall yggdrasil fund
	err := vaultMgr.manageChains(ctx, mgr, constAccessor)
	c.Assert(err, IsNil)
	c.Check(keeper.pools[1].Asset.Equals(common.BTCAsset), Equals, true)
	c.Check(keeper.pools[1].LPUnits.IsZero(), Equals, false, Commentf("%d\n", keeper.pools[1].LPUnits.Uint64()))
	c.Check(keeper.pools[0].LPUnits.Equal(cosmos.NewUint(1600)), Equals, true)
	for _, skr := range keeper.lps {
		c.Check(skr.Units.IsZero(), Equals, false)
	}

	// the first round should just recall yggdrasil fund
	ctx = ctx.WithBlockHeight(200000)
	err = vaultMgr.manageChains(ctx, mgr, constAccessor)
	c.Assert(err, IsNil)
	c.Check(keeper.pools[1].Asset.Equals(common.BTCAsset), Equals, true)
	c.Check(keeper.pools[1].LPUnits.IsZero(), Equals, true, Commentf("%d\n", keeper.pools[1].LPUnits.Uint64()))
	c.Check(keeper.pools[0].LPUnits.Equal(cosmos.NewUint(1600)), Equals, true)
	for _, skr := range keeper.lps {
		c.Check(skr.Units.IsZero(), Equals, true)
	}
	// ensure we have requested for ygg funds to be returned
	txOutStore := mgr.TxOutStore()
	c.Assert(err, IsNil)
	items, err := txOutStore.GetOutboundItems(ctx)
	c.Assert(err, IsNil)

	// 1 ygg return + 4 withdrawals
	c.Check(items, HasLen, 3, Commentf("Len %d", items))
	c.Check(items[0].Memo, Equals, NewYggdrasilReturn(100000).String())
	c.Check(items[0].Chain.Equals(common.BTCChain), Equals, true)

	ctx, mgr1 := setupManagerForTest(c)
	helper := NewVaultGenesisSetupTestHelper(mgr1.Keeper())
	mgr.K = helper
	vaultMgr1 := newNetworkMgrV76(helper, mgr1.TxOutStore(), mgr1.EventMgr())
	// fail to get active nodes should error out
	helper.failToListActiveAccounts = true
	c.Assert(vaultMgr1.ragnarokChain(ctx, common.BNBChain, 1, mgr, constAccessor), NotNil)
	helper.failToListActiveAccounts = false

	// no active nodes , should error
	c.Assert(vaultMgr1.ragnarokChain(ctx, common.BNBChain, 1, mgr, constAccessor), NotNil)
	c.Assert(helper.Keeper.SetNodeAccount(ctx, GetRandomValidatorNode(NodeActive)), IsNil)
	c.Assert(helper.Keeper.SetNodeAccount(ctx, GetRandomValidatorNode(NodeActive)), IsNil)

	// fail to get pools should error out
	helper.failGetPools = true
	c.Assert(vaultMgr1.ragnarokChain(ctx, common.BNBChain, 1, mgr, constAccessor), NotNil)
	helper.failGetPools = false
}

func (s *NetworkManagerV76TestSuite) TestUpdateNetwork(c *C) {
	ctx, mgr := setupManagerForTest(c)
	ver := GetCurrentVersion()
	constAccessor := constants.GetConstantValues(ver)
	helper := NewVaultGenesisSetupTestHelper(mgr.Keeper())
	mgr.K = helper
	vaultMgr := newNetworkMgrV76(helper, mgr.TxOutStore(), mgr.EventMgr())

	// fail to get Network should return error
	helper.failGetNetwork = true
	c.Assert(vaultMgr.UpdateNetwork(ctx, constAccessor, mgr.gasMgr, mgr.eventMgr), NotNil)
	helper.failGetNetwork = false

	// TotalReserve is zero , should not doing anything
	vd := NewNetwork()
	err := mgr.Keeper().SetNetwork(ctx, vd)
	c.Assert(err, IsNil)
	c.Assert(vaultMgr.UpdateNetwork(ctx, constAccessor, mgr.GasMgr(), mgr.EventMgr()), IsNil)

	c.Assert(vaultMgr.UpdateNetwork(ctx, constAccessor, mgr.GasMgr(), mgr.EventMgr()), IsNil)

	p := NewPool()
	p.Asset = common.BNBAsset
	p.BalanceRune = cosmos.NewUint(common.One * 100)
	p.BalanceAsset = cosmos.NewUint(common.One * 100)
	p.Status = PoolAvailable
	c.Assert(helper.SetPool(ctx, p), IsNil)
	// no active node , thus no bond
	c.Assert(vaultMgr.UpdateNetwork(ctx, constAccessor, mgr.GasMgr(), mgr.EventMgr()), IsNil)

	// with liquidity fee , and bonds
	c.Assert(helper.Keeper.AddToLiquidityFees(ctx, common.BNBAsset, cosmos.NewUint(50*common.One)), IsNil)

	c.Assert(vaultMgr.UpdateNetwork(ctx, constAccessor, mgr.GasMgr(), mgr.EventMgr()), IsNil)
	// add bond
	c.Assert(helper.Keeper.SetNodeAccount(ctx, GetRandomValidatorNode(NodeActive)), IsNil)
	c.Assert(helper.Keeper.SetNodeAccount(ctx, GetRandomValidatorNode(NodeActive)), IsNil)
	c.Assert(vaultMgr.UpdateNetwork(ctx, constAccessor, mgr.GasMgr(), mgr.EventMgr()), IsNil)

	// fail to get total liquidity fee should result an error
	helper.failGetTotalLiquidityFee = true
	if common.RuneAsset().Equals(common.RuneNative) {
		FundModule(c, ctx, helper, ReserveName, 100)
	}
	c.Assert(vaultMgr.UpdateNetwork(ctx, constAccessor, mgr.GasMgr(), mgr.EventMgr()), NotNil)
	helper.failGetTotalLiquidityFee = false

	helper.failToListActiveAccounts = true
	c.Assert(vaultMgr.UpdateNetwork(ctx, constAccessor, mgr.GasMgr(), mgr.EventMgr()), NotNil)
}

func (s *NetworkManagerV76TestSuite) TestCalcBlockRewards(c *C) {
	mgr := NewDummyMgr()
	vaultMgr := newNetworkMgrV76(keeper.KVStoreDummy{}, mgr.TxOutStore(), mgr.EventMgr())

	ver := GetCurrentVersion()
	constAccessor := constants.GetConstantValues(ver)
	emissionCurve := constAccessor.GetInt64Value(constants.EmissionCurve)
	incentiveCurve := constAccessor.GetInt64Value(constants.IncentiveCurve)
	blocksPerYear := constAccessor.GetInt64Value(constants.BlocksPerYear)

	bondR, poolR, lpD, lpShare := vaultMgr.calcBlockRewards(cosmos.NewUint(1000*common.One), cosmos.NewUint(2000*common.One), cosmos.NewUint(1000*common.One), cosmos.ZeroUint(), emissionCurve, incentiveCurve, blocksPerYear)
	c.Check(bondR.Uint64(), Equals, uint64(1585), Commentf("%d", bondR.Uint64()))
	c.Check(poolR.Uint64(), Equals, uint64(1586), Commentf("%d", poolR.Uint64()))
	c.Check(lpD.Uint64(), Equals, uint64(0), Commentf("%d", lpD.Uint64()))
	c.Check(lpShare.Uint64(), Equals, uint64(5002), Commentf("%d", lpShare.Uint64()))

	bondR, poolR, lpD, lpShare = vaultMgr.calcBlockRewards(cosmos.NewUint(1000*common.One), cosmos.NewUint(2000*common.One), cosmos.NewUint(1000*common.One), cosmos.NewUint(3000), emissionCurve, incentiveCurve, blocksPerYear)
	c.Check(bondR.Uint64(), Equals, uint64(3085), Commentf("%d", bondR.Uint64()))
	c.Check(poolR.Uint64(), Equals, uint64(86), Commentf("%d", poolR.Uint64()))
	c.Check(lpD.Uint64(), Equals, uint64(0), Commentf("%d", lpD.Uint64()))
	c.Check(lpShare.Uint64(), Equals, uint64(5001), Commentf("%d", lpShare.Uint64()))

	bondR, poolR, lpD, lpShare = vaultMgr.calcBlockRewards(cosmos.NewUint(1000*common.One), cosmos.NewUint(2000*common.One), cosmos.ZeroUint(), cosmos.ZeroUint(), emissionCurve, incentiveCurve, blocksPerYear)
	c.Check(bondR.Uint64(), Equals, uint64(0), Commentf("%d", bondR.Uint64()))
	c.Check(poolR.Uint64(), Equals, uint64(0), Commentf("%d", poolR.Uint64()))
	c.Check(lpD.Uint64(), Equals, uint64(0), Commentf("%d", lpD.Uint64()))
	c.Check(lpShare.Uint64(), Equals, uint64(0), Commentf("%d", lpShare.Uint64()))

	bondR, poolR, lpD, lpShare = vaultMgr.calcBlockRewards(cosmos.NewUint(1000*common.One), cosmos.NewUint(1000*common.One), cosmos.NewUint(1000*common.One), cosmos.ZeroUint(), emissionCurve, incentiveCurve, blocksPerYear)
	c.Check(bondR.Uint64(), Equals, uint64(3171), Commentf("%d", bondR.Uint64()))
	c.Check(poolR.Uint64(), Equals, uint64(0), Commentf("%d", poolR.Uint64()))
	c.Check(lpD.Uint64(), Equals, uint64(0), Commentf("%d", lpD.Uint64()))
	c.Check(lpShare.Uint64(), Equals, uint64(0), Commentf("%d", lpShare.Uint64()))

	bondR, poolR, lpD, lpShare = vaultMgr.calcBlockRewards(cosmos.ZeroUint(), cosmos.NewUint(1000*common.One), cosmos.NewUint(1000*common.One), cosmos.ZeroUint(), emissionCurve, incentiveCurve, blocksPerYear)
	c.Check(bondR.Uint64(), Equals, uint64(0), Commentf("%d", bondR.Uint64()))
	c.Check(poolR.Uint64(), Equals, uint64(3171), Commentf("%d", poolR.Uint64()))
	c.Check(lpD.Uint64(), Equals, uint64(0), Commentf("%d", lpD.Uint64()))
	c.Check(lpShare.Uint64(), Equals, uint64(10_000), Commentf("%d", lpShare.Uint64()))

	bondR, poolR, lpD, lpShare = vaultMgr.calcBlockRewards(cosmos.NewUint(2001*common.One), cosmos.NewUint(1000*common.One), cosmos.NewUint(1000*common.One), cosmos.ZeroUint(), emissionCurve, incentiveCurve, blocksPerYear)
	c.Check(bondR.Uint64(), Equals, uint64(3171), Commentf("%d", bondR.Uint64()))
	c.Check(poolR.Uint64(), Equals, uint64(0), Commentf("%d", poolR.Uint64()))
	c.Check(lpD.Uint64(), Equals, uint64(0), Commentf("%d", lpD.Uint64()))
	c.Check(lpShare.Uint64(), Equals, uint64(0), Commentf("%d", lpShare.Uint64()))
}

func (s *NetworkManagerV76TestSuite) TestCalcPoolDeficit(c *C) {
	pool1Fees := cosmos.NewUint(1000)
	pool2Fees := cosmos.NewUint(3000)
	totalFees := cosmos.NewUint(4000)

	mgr := NewDummyMgr()
	vaultMgr := newNetworkMgrV76(keeper.KVStoreDummy{}, mgr.TxOutStore(), mgr.EventMgr())

	lpDeficit := cosmos.NewUint(1120)
	amt1 := vaultMgr.calcPoolDeficit(lpDeficit, totalFees, pool1Fees)
	amt2 := vaultMgr.calcPoolDeficit(lpDeficit, totalFees, pool2Fees)

	c.Check(amt1.Equal(cosmos.NewUint(280)), Equals, true, Commentf("%d", amt1.Uint64()))
	c.Check(amt2.Equal(cosmos.NewUint(840)), Equals, true, Commentf("%d", amt2.Uint64()))
}

type VaultManagerTestHelpKeeper struct {
	keeper.Keeper
	failToGetAsgardVaults      bool
	failToListActiveAccounts   bool
	failToSetVault             bool
	failGetRetiringAsgardVault bool
	failGetActiveAsgardVault   bool
	failToSetPool              bool
	failGetNetwork             bool
	failGetTotalLiquidityFee   bool
	failGetPools               bool
}

func NewVaultGenesisSetupTestHelper(k keeper.Keeper) *VaultManagerTestHelpKeeper {
	return &VaultManagerTestHelpKeeper{
		Keeper: k,
	}
}

func (h *VaultManagerTestHelpKeeper) GetNetwork(ctx cosmos.Context) (Network, error) {
	if h.failGetNetwork {
		return Network{}, kaboom
	}
	return h.Keeper.GetNetwork(ctx)
}

func (h *VaultManagerTestHelpKeeper) GetAsgardVaults(ctx cosmos.Context) (Vaults, error) {
	if h.failToGetAsgardVaults {
		return Vaults{}, kaboom
	}
	return h.Keeper.GetAsgardVaults(ctx)
}

func (h *VaultManagerTestHelpKeeper) ListActiveValidators(ctx cosmos.Context) (NodeAccounts, error) {
	if h.failToListActiveAccounts {
		return NodeAccounts{}, kaboom
	}
	return h.Keeper.ListActiveValidators(ctx)
}

func (h *VaultManagerTestHelpKeeper) SetVault(ctx cosmos.Context, v Vault) error {
	if h.failToSetVault {
		return kaboom
	}
	return h.Keeper.SetVault(ctx, v)
}

func (h *VaultManagerTestHelpKeeper) GetAsgardVaultsByStatus(ctx cosmos.Context, vs VaultStatus) (Vaults, error) {
	if h.failGetRetiringAsgardVault && vs == RetiringVault {
		return Vaults{}, kaboom
	}
	if h.failGetActiveAsgardVault && vs == ActiveVault {
		return Vaults{}, kaboom
	}
	return h.Keeper.GetAsgardVaultsByStatus(ctx, vs)
}

func (h *VaultManagerTestHelpKeeper) SetPool(ctx cosmos.Context, p Pool) error {
	if h.failToSetPool {
		return kaboom
	}
	return h.Keeper.SetPool(ctx, p)
}

func (h *VaultManagerTestHelpKeeper) GetTotalLiquidityFees(ctx cosmos.Context, height uint64) (cosmos.Uint, error) {
	if h.failGetTotalLiquidityFee {
		return cosmos.ZeroUint(), kaboom
	}
	return h.Keeper.GetTotalLiquidityFees(ctx, height)
}

func (h *VaultManagerTestHelpKeeper) GetPools(ctx cosmos.Context) (Pools, error) {
	if h.failGetPools {
		return Pools{}, kaboom
	}
	return h.Keeper.GetPools(ctx)
}

func (*NetworkManagerV76TestSuite) TestProcessGenesisSetup(c *C) {
	ctx, mgr := setupManagerForTest(c)
	ver := GetCurrentVersion()
	constAccessor := constants.GetConstantValues(ver)
	helper := NewVaultGenesisSetupTestHelper(mgr.Keeper())
	ctx = ctx.WithBlockHeight(1)
	mgr.K = helper
	vaultMgr := newNetworkMgrV76(helper, mgr.TxOutStore(), mgr.EventMgr())
	// no active account
	c.Assert(vaultMgr.EndBlock(ctx, mgr, constAccessor), NotNil)

	nodeAccount := GetRandomValidatorNode(NodeActive)
	c.Assert(mgr.Keeper().SetNodeAccount(ctx, nodeAccount), IsNil)
	c.Assert(vaultMgr.EndBlock(ctx, mgr, constAccessor), IsNil)
	// make sure asgard vault get created
	vaults, err := mgr.Keeper().GetAsgardVaults(ctx)
	c.Assert(err, IsNil)
	c.Assert(vaults, HasLen, 1)

	// fail to get asgard vaults should return an error
	helper.failToGetAsgardVaults = true
	c.Assert(vaultMgr.EndBlock(ctx, mgr, constAccessor), NotNil)
	helper.failToGetAsgardVaults = false

	// vault already exist , it should not do anything , and should not error
	c.Assert(vaultMgr.EndBlock(ctx, mgr, constAccessor), IsNil)

	ctx, mgr = setupManagerForTest(c)
	helper = NewVaultGenesisSetupTestHelper(mgr.Keeper())
	ctx = ctx.WithBlockHeight(1)
	mgr.K = helper
	vaultMgr = newNetworkMgrV76(helper, mgr.TxOutStore(), mgr.EventMgr())
	helper.failToListActiveAccounts = true
	c.Assert(vaultMgr.EndBlock(ctx, mgr, constAccessor), NotNil)
	helper.failToListActiveAccounts = false

	helper.failToSetVault = true
	c.Assert(vaultMgr.EndBlock(ctx, mgr, constAccessor), NotNil)
	helper.failToSetVault = false

	helper.failGetRetiringAsgardVault = true
	ctx = ctx.WithBlockHeight(1024)
	c.Assert(vaultMgr.EndBlock(ctx, mgr, constAccessor), NotNil)
	helper.failGetRetiringAsgardVault = false

	helper.failGetActiveAsgardVault = true
	c.Assert(vaultMgr.EndBlock(ctx, mgr, constAccessor), NotNil)
	helper.failGetActiveAsgardVault = false
}

func (*NetworkManagerV76TestSuite) TestGetTotalActiveBond(c *C) {
	ctx, mgr := setupManagerForTest(c)
	helper := NewVaultGenesisSetupTestHelper(mgr.Keeper())
	mgr.K = helper
	vaultMgr := newNetworkMgrV76(helper, mgr.TxOutStore(), mgr.EventMgr())
	helper.failToListActiveAccounts = true
	bond, err := vaultMgr.getTotalActiveBond(ctx)
	c.Assert(err, NotNil)
	c.Assert(bond.Equal(cosmos.ZeroUint()), Equals, true)
	helper.failToListActiveAccounts = false
	c.Assert(helper.Keeper.SetNodeAccount(ctx, GetRandomValidatorNode(NodeActive)), IsNil)
	bond, err = vaultMgr.getTotalActiveBond(ctx)
	c.Assert(err, IsNil)
	c.Assert(bond.Uint64() > 0, Equals, true)
}

func (*NetworkManagerV76TestSuite) TestGetTotalLiquidityRune(c *C) {
	ctx, mgr := setupManagerForTest(c)
	helper := NewVaultGenesisSetupTestHelper(mgr.Keeper())
	mgr.K = helper
	vaultMgr := newNetworkMgrV76(helper, mgr.TxOutStore(), mgr.EventMgr())
	p := NewPool()
	p.Asset = common.BNBAsset
	p.BalanceRune = cosmos.NewUint(common.One * 100)
	p.BalanceAsset = cosmos.NewUint(common.One * 100)
	p.Status = PoolAvailable
	c.Assert(helper.SetPool(ctx, p), IsNil)
	pools, totalLiquidity, err := vaultMgr.getTotalProvidedLiquidityRune(ctx)
	c.Assert(err, IsNil)
	c.Assert(pools, HasLen, 1)
	c.Assert(totalLiquidity.Equal(p.BalanceRune), Equals, true)
}

func (*NetworkManagerV76TestSuite) TestPayPoolRewards(c *C) {
	ctx, mgr := setupManagerForTest(c)
	helper := NewVaultGenesisSetupTestHelper(mgr.Keeper())
	mgr.K = helper
	vaultMgr := newNetworkMgrV76(helper, mgr.TxOutStore(), mgr.EventMgr())
	p := NewPool()
	p.Asset = common.BNBAsset
	p.BalanceRune = cosmos.NewUint(common.One * 100)
	p.BalanceAsset = cosmos.NewUint(common.One * 100)
	p.Status = PoolAvailable
	c.Assert(helper.SetPool(ctx, p), IsNil)
	c.Assert(vaultMgr.payPoolRewards(ctx, []cosmos.Uint{cosmos.NewUint(100 * common.One)}, Pools{p}), IsNil)
	helper.failToSetPool = true
	c.Assert(vaultMgr.payPoolRewards(ctx, []cosmos.Uint{cosmos.NewUint(100 * common.One)}, Pools{p}), NotNil)
}

func (*NetworkManagerV76TestSuite) TestFindChainsToRetire(c *C) {
	ctx, mgr := setupManagerForTest(c)
	helper := NewVaultGenesisSetupTestHelper(mgr.Keeper())
	mgr.K = helper
	vaultMgr := newNetworkMgrV76(helper, mgr.TxOutStore(), mgr.EventMgr())
	// fail to get active asgard vault
	helper.failGetActiveAsgardVault = true
	chains, err := vaultMgr.findChainsToRetire(ctx)
	c.Assert(err, NotNil)
	c.Assert(chains, HasLen, 0)
	helper.failGetActiveAsgardVault = false

	// fail to get retire asgard vault
	helper.failGetRetiringAsgardVault = true
	chains, err = vaultMgr.findChainsToRetire(ctx)
	c.Assert(err, NotNil)
	c.Assert(chains, HasLen, 0)
	helper.failGetRetiringAsgardVault = false
}

func (*NetworkManagerV76TestSuite) TestRecallChainFunds(c *C) {
	ctx, mgr := setupManagerForTest(c)
	helper := NewVaultGenesisSetupTestHelper(mgr.Keeper())
	mgr.K = helper
	vaultMgr := newNetworkMgrV76(helper, mgr.TxOutStore(), mgr.EventMgr())
	helper.failToListActiveAccounts = true
	c.Assert(vaultMgr.RecallChainFunds(ctx, common.BNBChain, mgr, common.PubKeys{}), NotNil)
	helper.failToListActiveAccounts = false

	helper.failGetActiveAsgardVault = true
	c.Assert(vaultMgr.RecallChainFunds(ctx, common.BNBChain, mgr, common.PubKeys{}), NotNil)
	helper.failGetActiveAsgardVault = false
}
