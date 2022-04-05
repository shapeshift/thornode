package thorchain

import (
	"errors"

	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

type HandlerAddLiquiditySuite struct{}

var _ = Suite(&HandlerAddLiquiditySuite{})

type MockAddLiquidityKeeper struct {
	keeper.KVStoreDummy
	currentPool       Pool
	activeNodeAccount NodeAccount
	failGetPool       bool
	lp                LiquidityProvider
}

func (m *MockAddLiquidityKeeper) PoolExist(_ cosmos.Context, asset common.Asset) bool {
	return m.currentPool.Asset.Equals(asset)
}

func (m *MockAddLiquidityKeeper) GetPools(_ cosmos.Context) (Pools, error) {
	return Pools{m.currentPool}, nil
}

func (m *MockAddLiquidityKeeper) GetPool(_ cosmos.Context, _ common.Asset) (Pool, error) {
	if m.failGetPool {
		return Pool{}, errors.New("fail to get pool")
	}
	return m.currentPool, nil
}

func (m *MockAddLiquidityKeeper) SetPool(_ cosmos.Context, pool Pool) error {
	m.currentPool = pool
	return nil
}

func (m *MockAddLiquidityKeeper) ListValidatorsWithBond(_ cosmos.Context) (NodeAccounts, error) {
	return NodeAccounts{m.activeNodeAccount}, nil
}

func (m *MockAddLiquidityKeeper) GetNodeAccount(_ cosmos.Context, addr cosmos.AccAddress) (NodeAccount, error) {
	if m.activeNodeAccount.NodeAddress.Equals(addr) {
		return m.activeNodeAccount, nil
	}
	return NodeAccount{}, errors.New("not exist")
}

func (m *MockAddLiquidityKeeper) GetLiquidityProvider(_ cosmos.Context, asset common.Asset, addr common.Address) (LiquidityProvider, error) {
	return m.lp, nil
}

func (m *MockAddLiquidityKeeper) SetLiquidityProvider(ctx cosmos.Context, lp LiquidityProvider) {
	m.lp = lp
}

func (m *MockAddLiquidityKeeper) AddOwnership(ctx cosmos.Context, coin common.Coin, _ cosmos.AccAddress) error {
	m.lp.Units = m.lp.Units.Add(coin.Amount)
	return nil
}

type MockConstant struct {
	constants.DummyConstants
}

func (s *HandlerAddLiquiditySuite) SetUpSuite(c *C) {
	SetupConfigForTest()
}

func (s *HandlerAddLiquiditySuite) TestAddLiquidityHandler(c *C) {
	var err error
	ctx, mgr := setupManagerForTest(c)
	activeNodeAccount := GetRandomValidatorNode(NodeActive)
	runeAddr := GetRandomRUNEAddress()
	bnbAddr := GetRandomBNBAddress()
	pool := NewPool()
	pool.Asset = common.BNBAsset
	pool.Status = PoolAvailable
	k := &MockAddLiquidityKeeper{
		activeNodeAccount: activeNodeAccount,
		currentPool:       pool,
		lp: LiquidityProvider{
			Asset:             common.BNBAsset,
			RuneAddress:       runeAddr,
			AssetAddress:      bnbAddr,
			Units:             cosmos.ZeroUint(),
			PendingRune:       cosmos.ZeroUint(),
			PendingAsset:      cosmos.ZeroUint(),
			RuneDepositValue:  cosmos.ZeroUint(),
			AssetDepositValue: cosmos.ZeroUint(),
		},
	}
	mgr.K = k
	// happy path
	addHandler := NewAddLiquidityHandler(mgr)
	addTxHash := GetRandomTxHash()
	tx := common.NewTx(
		addTxHash,
		runeAddr,
		GetRandomBNBAddress(),
		common.Coins{common.NewCoin(common.BNBAsset, cosmos.NewUint(common.One*5))},
		BNBGasFeeSingleton,
		"add:BNB",
	)
	msg := NewMsgAddLiquidity(
		tx,
		common.BNBAsset,
		cosmos.NewUint(100*common.One),
		cosmos.ZeroUint(),
		runeAddr,
		bnbAddr,
		common.NoAddress, cosmos.ZeroUint(),
		activeNodeAccount.NodeAddress)
	_, err = addHandler.Run(ctx, msg)
	c.Assert(err, IsNil)

	midLiquidityPool, err := mgr.Keeper().GetPool(ctx, common.BNBAsset)
	c.Assert(err, IsNil)
	c.Assert(midLiquidityPool.PendingInboundRune.String(), Equals, "10000000000")

	msg.RuneAmount = cosmos.ZeroUint()
	msg.AssetAmount = cosmos.NewUint(100 * common.One)
	_, err = addHandler.Run(ctx, msg)
	c.Assert(err, IsNil)

	postLiquidityPool, err := mgr.Keeper().GetPool(ctx, common.BNBAsset)
	c.Assert(err, IsNil)
	c.Assert(postLiquidityPool.BalanceAsset.String(), Equals, "10000000000")
	c.Assert(postLiquidityPool.BalanceRune.String(), Equals, "10000000000")
	c.Assert(postLiquidityPool.PendingInboundAsset.String(), Equals, "0")
	c.Assert(postLiquidityPool.PendingInboundRune.String(), Equals, "0")
}

func (s *HandlerAddLiquiditySuite) TestAddLiquidityHandler_NoPool_ShouldCreateNewPool(c *C) {
	activeNodeAccount := GetRandomValidatorNode(NodeActive)
	activeNodeAccount.Bond = cosmos.NewUint(1000000 * common.One)
	runeAddr := GetRandomRUNEAddress()
	bnbAddr := GetRandomBNBAddress()
	pool := NewPool()
	pool.Status = PoolAvailable
	k := &MockAddLiquidityKeeper{
		activeNodeAccount: activeNodeAccount,
		currentPool:       pool,
		lp: LiquidityProvider{
			Asset:             common.BNBAsset,
			RuneAddress:       runeAddr,
			AssetAddress:      bnbAddr,
			Units:             cosmos.ZeroUint(),
			PendingRune:       cosmos.ZeroUint(),
			PendingAsset:      cosmos.ZeroUint(),
			RuneDepositValue:  cosmos.ZeroUint(),
			AssetDepositValue: cosmos.ZeroUint(),
		},
	}
	// happy path
	ctx, mgr := setupManagerForTest(c)
	mgr.K = k
	addHandler := NewAddLiquidityHandler(mgr)
	preLiquidityPool, err := k.GetPool(ctx, common.BNBAsset)
	c.Assert(err, IsNil)
	c.Assert(preLiquidityPool.IsEmpty(), Equals, true)
	addTxHash := GetRandomTxHash()
	tx := common.NewTx(
		addTxHash,
		runeAddr,
		GetRandomBNBAddress(),
		common.Coins{common.NewCoin(common.BNBAsset, cosmos.NewUint(common.One*5))},
		BNBGasFeeSingleton,
		"add:BNB",
	)
	mgr.ConstAccessor = constants.NewDummyConstants(map[constants.ConstantName]int64{
		constants.MaximumLiquidityRune: 600_000_00000000,
	}, map[constants.ConstantName]bool{
		constants.StrictBondLiquidityRatio: true,
	}, map[constants.ConstantName]string{})

	msg := NewMsgAddLiquidity(
		tx,
		common.BNBAsset,
		cosmos.NewUint(100*common.One),
		cosmos.NewUint(100*common.One),
		runeAddr,
		bnbAddr,
		common.NoAddress, cosmos.ZeroUint(),
		activeNodeAccount.NodeAddress)
	_, err = addHandler.Run(ctx, msg)
	c.Assert(err, IsNil)
	postLiquidityPool, err := k.GetPool(ctx, common.BNBAsset)
	c.Assert(err, IsNil)
	c.Assert(postLiquidityPool.BalanceAsset.String(), Equals, preLiquidityPool.BalanceAsset.Add(msg.AssetAmount).String())
	c.Assert(postLiquidityPool.BalanceRune.String(), Equals, preLiquidityPool.BalanceRune.Add(msg.RuneAmount).String())
}

func (s *HandlerAddLiquiditySuite) TestAddLiquidityHandlerValidation(c *C) {
	ctx, _ := setupKeeperForTest(c)
	activeNodeAccount := GetRandomValidatorNode(NodeActive)
	runeAddr := GetRandomRUNEAddress()
	bnbAddr := GetRandomBNBAddress()
	bnbSynthAsset, _ := common.NewAsset("BNB/BNB")
	tx := common.NewTx(
		GetRandomTxHash(),
		GetRandomRUNEAddress(),
		GetRandomRUNEAddress(),
		common.Coins{common.NewCoin(bnbSynthAsset, cosmos.NewUint(common.One*5))},
		common.Gas{
			{Asset: common.RuneNative, Amount: cosmos.NewUint(1 * common.One)},
		},
		"add:BNB.BNB",
	)

	k := &MockAddLiquidityKeeper{
		activeNodeAccount: activeNodeAccount,
		currentPool: Pool{
			BalanceRune:  cosmos.ZeroUint(),
			BalanceAsset: cosmos.ZeroUint(),
			Asset:        common.BNBAsset,
			LPUnits:      cosmos.ZeroUint(),
			Status:       PoolAvailable,
		},
		lp: LiquidityProvider{
			Asset:             common.BNBAsset,
			RuneAddress:       runeAddr,
			AssetAddress:      bnbAddr,
			Units:             cosmos.ZeroUint(),
			PendingRune:       cosmos.ZeroUint(),
			PendingAsset:      cosmos.ZeroUint(),
			RuneDepositValue:  cosmos.ZeroUint(),
			AssetDepositValue: cosmos.ZeroUint(),
		},
	}
	testCases := []struct {
		name           string
		msg            *MsgAddLiquidity
		expectedResult error
	}{
		{
			name:           "empty signer should fail",
			msg:            NewMsgAddLiquidity(GetRandomTx(), common.BNBAsset, cosmos.NewUint(common.One*5), cosmos.NewUint(common.One*5), GetRandomBNBAddress(), GetRandomBNBAddress(), common.NoAddress, cosmos.ZeroUint(), cosmos.AccAddress{}),
			expectedResult: errAddLiquidityFailValidation,
		},
		{
			name:           "empty asset should fail",
			msg:            NewMsgAddLiquidity(GetRandomTx(), common.Asset{}, cosmos.NewUint(common.One*5), cosmos.NewUint(common.One*5), GetRandomBNBAddress(), GetRandomBNBAddress(), common.NoAddress, cosmos.ZeroUint(), GetRandomValidatorNode(NodeActive).NodeAddress),
			expectedResult: errAddLiquidityFailValidation,
		},
		{
			name:           "synth asset from memo should fail",
			msg:            NewMsgAddLiquidity(GetRandomTx(), bnbSynthAsset, cosmos.NewUint(common.One*5), cosmos.NewUint(common.One*5), GetRandomBNBAddress(), GetRandomBNBAddress(), common.NoAddress, cosmos.ZeroUint(), GetRandomValidatorNode(NodeActive).NodeAddress),
			expectedResult: errAddLiquidityFailValidation,
		},
		{
			name:           "synth asset from coins should fail",
			msg:            NewMsgAddLiquidity(tx, common.BNBAsset, cosmos.NewUint(common.One*5), cosmos.NewUint(common.One*5), GetRandomBNBAddress(), GetRandomBNBAddress(), common.NoAddress, cosmos.ZeroUint(), GetRandomValidatorNode(NodeActive).NodeAddress),
			expectedResult: errAddLiquidityFailValidation,
		},
		{
			name:           "empty addresses should fail",
			msg:            NewMsgAddLiquidity(GetRandomTx(), common.BTCAsset, cosmos.NewUint(common.One*5), cosmos.NewUint(common.One*5), common.NoAddress, common.NoAddress, common.NoAddress, cosmos.ZeroUint(), GetRandomValidatorNode(NodeActive).NodeAddress),
			expectedResult: errAddLiquidityFailValidation,
		},
		{
			name:           "total liquidity provider is more than total bond should fail",
			msg:            NewMsgAddLiquidity(GetRandomTx(), common.BNBAsset, cosmos.NewUint(common.One*5000), cosmos.NewUint(common.One*5000), GetRandomRUNEAddress(), GetRandomBNBAddress(), common.NoAddress, cosmos.ZeroUint(), activeNodeAccount.NodeAddress),
			expectedResult: errAddLiquidityRUNEMoreThanBond,
		},
		{
			name:           "asset address with wrong chain should fail",
			msg:            NewMsgAddLiquidity(GetRandomTx(), common.BNBAsset, cosmos.NewUint(common.One*5), cosmos.NewUint(common.One*5), GetRandomRUNEAddress(), GetRandomRUNEAddress(), common.NoAddress, cosmos.ZeroUint(), GetRandomValidatorNode(NodeActive).NodeAddress),
			expectedResult: errAddLiquidityFailValidation,
		},
		{
			name:           "rune address with wrong chain should fail",
			msg:            NewMsgAddLiquidity(GetRandomTx(), common.BNBAsset, cosmos.NewUint(common.One*5), cosmos.NewUint(common.One*5), GetRandomBNBAddress(), GetRandomRUNEAddress(), common.NoAddress, cosmos.ZeroUint(), GetRandomValidatorNode(NodeActive).NodeAddress),
			expectedResult: errAddLiquidityFailValidation,
		},
	}
	constAccessor := constants.NewDummyConstants(map[constants.ConstantName]int64{
		constants.MaximumLiquidityRune: 600_000_00000000,
	}, map[constants.ConstantName]bool{
		constants.StrictBondLiquidityRatio: true,
	}, map[constants.ConstantName]string{})

	for _, item := range testCases {
		mgr := NewDummyMgrWithKeeper(k)
		mgr.constAccessor = constAccessor
		addHandler := NewAddLiquidityHandler(mgr)
		_, err := addHandler.Run(ctx, item.msg)
		c.Assert(errors.Is(err, item.expectedResult), Equals, true, Commentf("name:%s, %w", item.name, err))
	}
}

func (s *HandlerAddLiquiditySuite) TestHandlerAddLiquidityFailScenario(c *C) {
	activeNodeAccount := GetRandomValidatorNode(NodeActive)
	emptyPool := Pool{
		BalanceRune:  cosmos.ZeroUint(),
		BalanceAsset: cosmos.ZeroUint(),
		Asset:        common.BNBAsset,
		LPUnits:      cosmos.ZeroUint(),
		Status:       PoolAvailable,
	}

	testCases := []struct {
		name           string
		k              keeper.Keeper
		expectedResult error
	}{
		{
			name: "fail to get pool should fail add liquidity",
			k: &MockAddLiquidityKeeper{
				activeNodeAccount: activeNodeAccount,
				currentPool:       emptyPool,
				failGetPool:       true,
			},
			expectedResult: errInternal,
		},
		{
			name: "suspended pool should fail add liquidity",
			k: &MockAddLiquidityKeeper{
				activeNodeAccount: activeNodeAccount,
				currentPool: Pool{
					BalanceRune:  cosmos.ZeroUint(),
					BalanceAsset: cosmos.ZeroUint(),
					Asset:        common.BNBAsset,
					LPUnits:      cosmos.ZeroUint(),
					Status:       PoolSuspended,
				},
			},
			expectedResult: errInvalidPoolStatus,
		},
	}
	for _, tc := range testCases {
		runeAddr := GetRandomRUNEAddress()
		bnbAddr := GetRandomBNBAddress()
		addTxHash := GetRandomTxHash()
		tx := common.NewTx(
			addTxHash,
			runeAddr,
			GetRandomBNBAddress(),
			common.Coins{common.NewCoin(common.BNBAsset, cosmos.NewUint(common.One*5))},
			BNBGasFeeSingleton,
			"add:BNB",
		)
		msg := NewMsgAddLiquidity(
			tx,
			common.BNBAsset,
			cosmos.NewUint(100*common.One),
			cosmos.NewUint(100*common.One),
			runeAddr,
			bnbAddr,
			common.NoAddress, cosmos.ZeroUint(),
			activeNodeAccount.NodeAddress)
		ctx, mgr := setupManagerForTest(c)
		mgr.K = tc.k
		addHandler := NewAddLiquidityHandler(mgr)
		_, err := addHandler.Run(ctx, msg)
		c.Assert(errors.Is(err, tc.expectedResult), Equals, true, Commentf(tc.name))
	}
}

type AddLiquidityTestKeeper struct {
	keeper.KVStoreDummy
	store          map[string]interface{}
	liquidityUnits cosmos.Uint
}

// NewAddLiquidityTestKeeper
func NewAddLiquidityTestKeeper() *AddLiquidityTestKeeper {
	return &AddLiquidityTestKeeper{
		store:          make(map[string]interface{}),
		liquidityUnits: cosmos.ZeroUint(),
	}
}

func (p *AddLiquidityTestKeeper) PoolExist(ctx cosmos.Context, asset common.Asset) bool {
	_, ok := p.store[asset.String()]
	return ok
}

var notExistLiquidityProviderAsset, _ = common.NewAsset("BNB.NotExistLiquidityProviderAsset")

func (p *AddLiquidityTestKeeper) GetPool(ctx cosmos.Context, asset common.Asset) (Pool, error) {
	if p, ok := p.store[asset.String()]; ok {
		return p.(Pool), nil
	}
	return NewPool(), nil
}

func (p *AddLiquidityTestKeeper) SetPool(ctx cosmos.Context, ps Pool) error {
	p.store[ps.Asset.String()] = ps
	return nil
}

func (p *AddLiquidityTestKeeper) GetLiquidityProvider(ctx cosmos.Context, asset common.Asset, addr common.Address) (LiquidityProvider, error) {
	if notExistLiquidityProviderAsset.Equals(asset) {
		return LiquidityProvider{}, errors.New("simulate error for test")
	}
	lp := LiquidityProvider{
		Asset:             asset,
		RuneAddress:       addr,
		Units:             cosmos.ZeroUint(),
		PendingRune:       cosmos.ZeroUint(),
		PendingAsset:      cosmos.ZeroUint(),
		RuneDepositValue:  cosmos.ZeroUint(),
		AssetDepositValue: cosmos.ZeroUint(),
	}
	key := p.GetKey(ctx, "lp/", lp.Key())
	if res, ok := p.store[key]; ok {
		return res.(LiquidityProvider), nil
	}
	lp.Units = p.liquidityUnits
	return lp, nil
}

func (p *AddLiquidityTestKeeper) SetLiquidityProvider(ctx cosmos.Context, lp LiquidityProvider) {
	key := p.GetKey(ctx, "lp/", lp.Key())
	p.store[key] = lp
}

func (p *AddLiquidityTestKeeper) AddOwnership(ctx cosmos.Context, coin common.Coin, addr cosmos.AccAddress) error {
	p.liquidityUnits = p.liquidityUnits.Add(coin.Amount)
	return nil
}

func (s *HandlerAddLiquiditySuite) TestCalculateLPUnitsV1(c *C) {
	inputs := []struct {
		name           string
		oldLPUnits     cosmos.Uint
		poolRune       cosmos.Uint
		poolAsset      cosmos.Uint
		addRune        cosmos.Uint
		addAsset       cosmos.Uint
		poolUnits      cosmos.Uint
		liquidityUnits cosmos.Uint
		expectedErr    error
	}{
		{
			name:           "first-add-zero-rune",
			oldLPUnits:     cosmos.ZeroUint(),
			poolRune:       cosmos.ZeroUint(),
			poolAsset:      cosmos.ZeroUint(),
			addRune:        cosmos.ZeroUint(),
			addAsset:       cosmos.NewUint(100 * common.One),
			poolUnits:      cosmos.ZeroUint(),
			liquidityUnits: cosmos.ZeroUint(),
			expectedErr:    errors.New("total RUNE in the pool is zero"),
		},
		{
			name:           "first-add-zero-asset",
			oldLPUnits:     cosmos.ZeroUint(),
			poolRune:       cosmos.ZeroUint(),
			poolAsset:      cosmos.ZeroUint(),
			addRune:        cosmos.NewUint(100 * common.One),
			addAsset:       cosmos.ZeroUint(),
			poolUnits:      cosmos.ZeroUint(),
			liquidityUnits: cosmos.ZeroUint(),
			expectedErr:    errors.New("total asset in the pool is zero"),
		},
		{
			name:           "first-add",
			oldLPUnits:     cosmos.ZeroUint(),
			poolRune:       cosmos.ZeroUint(),
			poolAsset:      cosmos.ZeroUint(),
			addRune:        cosmos.NewUint(100 * common.One),
			addAsset:       cosmos.NewUint(100 * common.One),
			poolUnits:      cosmos.NewUint(100 * common.One),
			liquidityUnits: cosmos.NewUint(100 * common.One),
			expectedErr:    nil,
		},
		{
			name:           "second-add",
			oldLPUnits:     cosmos.NewUint(500 * common.One),
			poolRune:       cosmos.NewUint(500 * common.One),
			poolAsset:      cosmos.NewUint(500 * common.One),
			addRune:        cosmos.NewUint(345 * common.One),
			addAsset:       cosmos.NewUint(234 * common.One),
			poolUnits:      cosmos.NewUint(76359469067),
			liquidityUnits: cosmos.NewUint(26359469067),
			expectedErr:    nil,
		},
	}

	for _, item := range inputs {
		c.Logf("Name: %s", item.name)
		poolUnits, liquidityUnits, err := calculatePoolUnitsV1(item.oldLPUnits, item.poolRune, item.poolAsset, item.addRune, item.addAsset)
		if item.expectedErr == nil {
			c.Assert(err, IsNil)
		} else {
			c.Assert(err.Error(), Equals, item.expectedErr.Error())
		}

		c.Check(item.poolUnits.Uint64(), Equals, poolUnits.Uint64(), Commentf("%d / %d", item.poolUnits.Uint64(), poolUnits.Uint64()))
		c.Check(item.liquidityUnits.Uint64(), Equals, liquidityUnits.Uint64(), Commentf("%d / %d", item.liquidityUnits.Uint64(), liquidityUnits.Uint64()))
	}
}

func (s *HandlerAddLiquiditySuite) TestValidateAddLiquidityMessage(c *C) {
	ps := NewAddLiquidityTestKeeper()
	ctx, mgr := setupManagerForTest(c)
	mgr.K = ps
	txID := GetRandomTxHash()
	bnbAddress := GetRandomBNBAddress()
	assetAddress := GetRandomBNBAddress()
	h := NewAddLiquidityHandler(mgr)
	c.Assert(h.validateAddLiquidityMessage(ctx, ps, common.Asset{}, txID, bnbAddress, assetAddress), NotNil)
	c.Assert(h.validateAddLiquidityMessage(ctx, ps, common.BNBAsset, txID, bnbAddress, assetAddress), NotNil)
	c.Assert(h.validateAddLiquidityMessage(ctx, ps, common.BNBAsset, txID, bnbAddress, assetAddress), NotNil)
	c.Assert(h.validateAddLiquidityMessage(ctx, ps, common.BNBAsset, common.TxID(""), bnbAddress, assetAddress), NotNil)
	c.Assert(h.validateAddLiquidityMessage(ctx, ps, common.BNBAsset, txID, common.NoAddress, common.NoAddress), NotNil)
	c.Assert(h.validateAddLiquidityMessage(ctx, ps, common.BNBAsset, txID, bnbAddress, assetAddress), NotNil)
	c.Assert(h.validateAddLiquidityMessage(ctx, ps, common.BNBAsset, txID, common.NoAddress, assetAddress), NotNil)
	c.Assert(h.validateAddLiquidityMessage(ctx, ps, common.BTCAsset, txID, bnbAddress, common.NoAddress), NotNil)
	c.Assert(ps.SetPool(ctx, Pool{
		BalanceRune:  cosmos.NewUint(100 * common.One),
		BalanceAsset: cosmos.NewUint(100 * common.One),
		Asset:        common.BNBAsset,
		LPUnits:      cosmos.NewUint(100 * common.One),
		Status:       PoolAvailable,
	}), IsNil)
	c.Assert(h.validateAddLiquidityMessage(ctx, ps, common.BNBAsset, txID, bnbAddress, assetAddress), Equals, nil)
}

func (s *HandlerAddLiquiditySuite) TestAddLiquidityV1(c *C) {
	ps := NewAddLiquidityTestKeeper()
	ctx, _ := setupKeeperForTest(c)
	txID := GetRandomTxHash()

	runeAddress := GetRandomRUNEAddress()
	assetAddress := GetRandomBNBAddress()
	btcAddress, err := common.NewAddress("bc1qwqdg6squsna38e46795at95yu9atm8azzmyvckulcc7kytlcckxswvvzej")
	c.Assert(err, IsNil)
	constAccessor := constants.GetConstantValues(GetCurrentVersion())
	h := NewAddLiquidityHandler(NewDummyMgrWithKeeper(ps))
	err = h.addLiquidity(ctx, common.Asset{}, cosmos.NewUint(100*common.One), cosmos.NewUint(100*common.One), runeAddress, assetAddress, txID, false, constAccessor)
	c.Assert(err, NotNil)
	c.Assert(ps.SetPool(ctx, Pool{
		BalanceRune:         cosmos.ZeroUint(),
		BalanceAsset:        cosmos.NewUint(100 * common.One),
		Asset:               common.BNBAsset,
		LPUnits:             cosmos.NewUint(100 * common.One),
		SynthUnits:          cosmos.ZeroUint(),
		PendingInboundAsset: cosmos.ZeroUint(),
		PendingInboundRune:  cosmos.ZeroUint(),
		Status:              PoolAvailable,
	}), IsNil)
	err = h.addLiquidity(ctx, common.BNBAsset, cosmos.NewUint(100*common.One), cosmos.NewUint(100*common.One), runeAddress, assetAddress, txID, false, constAccessor)
	c.Assert(err, IsNil)
	su, err := ps.GetLiquidityProvider(ctx, common.BNBAsset, runeAddress)
	c.Assert(err, IsNil)
	// c.Assert(su.Units.Equal(cosmos.NewUint(11250000000)), Equals, true, Commentf("%d", su.Units.Uint64()))

	c.Assert(ps.SetPool(ctx, Pool{
		BalanceRune:         cosmos.NewUint(100 * common.One),
		BalanceAsset:        cosmos.NewUint(100 * common.One),
		Asset:               notExistLiquidityProviderAsset,
		LPUnits:             cosmos.NewUint(100 * common.One),
		SynthUnits:          cosmos.ZeroUint(),
		PendingInboundAsset: cosmos.ZeroUint(),
		PendingInboundRune:  cosmos.ZeroUint(),
		Status:              PoolAvailable,
	}), IsNil)
	// add asymmetically
	err = h.addLiquidity(ctx, common.BNBAsset, cosmos.NewUint(100*common.One), cosmos.ZeroUint(), runeAddress, assetAddress, txID, false, constAccessor)
	c.Assert(err, IsNil)
	err = h.addLiquidity(ctx, common.BNBAsset, cosmos.ZeroUint(), cosmos.NewUint(100*common.One), runeAddress, assetAddress, txID, false, constAccessor)
	c.Assert(err, IsNil)

	err = h.addLiquidity(ctx, notExistLiquidityProviderAsset, cosmos.NewUint(100*common.One), cosmos.NewUint(100*common.One), runeAddress, assetAddress, txID, false, constAccessor)
	c.Assert(err, NotNil)
	c.Assert(ps.SetPool(ctx, Pool{
		BalanceRune:         cosmos.NewUint(100 * common.One),
		BalanceAsset:        cosmos.NewUint(100 * common.One),
		Asset:               common.BNBAsset,
		LPUnits:             cosmos.NewUint(100 * common.One),
		SynthUnits:          cosmos.ZeroUint(),
		PendingInboundAsset: cosmos.ZeroUint(),
		PendingInboundRune:  cosmos.ZeroUint(),
		Status:              PoolAvailable,
	}), IsNil)

	for i := 1; i <= 150; i++ {
		lp := LiquidityProvider{Units: cosmos.NewUint(common.One / 5000)}
		ps.SetLiquidityProvider(ctx, lp)
	}
	err = h.addLiquidity(ctx, common.BNBAsset, cosmos.NewUint(common.One), cosmos.NewUint(common.One), runeAddress, assetAddress, txID, false, constAccessor)
	c.Assert(err, IsNil)

	err = h.addLiquidity(ctx, common.BNBAsset, cosmos.NewUint(100*common.One), cosmos.NewUint(100*common.One), runeAddress, assetAddress, txID, false, constAccessor)
	c.Assert(err, IsNil)
	p, err := ps.GetPool(ctx, common.BNBAsset)
	c.Assert(err, IsNil)
	c.Check(p.LPUnits.Equal(cosmos.NewUint(201*common.One)), Equals, true, Commentf("%d", p.LPUnits.Uint64()))

	// Test atomic cross chain liquidity provision
	// create BTC pool
	c.Assert(ps.SetPool(ctx, Pool{
		BalanceRune:         cosmos.ZeroUint(),
		BalanceAsset:        cosmos.ZeroUint(),
		Asset:               common.BTCAsset,
		LPUnits:             cosmos.ZeroUint(),
		SynthUnits:          cosmos.ZeroUint(),
		PendingInboundAsset: cosmos.ZeroUint(),
		PendingInboundRune:  cosmos.ZeroUint(),
		Status:              PoolAvailable,
	}), IsNil)

	// add rune
	err = h.addLiquidity(ctx, common.BTCAsset, cosmos.NewUint(100*common.One), cosmos.ZeroUint(), runeAddress, btcAddress, txID, true, constAccessor)
	c.Assert(err, IsNil)
	su, err = ps.GetLiquidityProvider(ctx, common.BTCAsset, runeAddress)
	c.Assert(err, IsNil)
	// c.Check(su.Units.IsZero(), Equals, true)
	// add btc
	err = h.addLiquidity(ctx, common.BTCAsset, cosmos.ZeroUint(), cosmos.NewUint(100*common.One), runeAddress, btcAddress, txID, false, constAccessor)
	c.Assert(err, IsNil)
	su, err = ps.GetLiquidityProvider(ctx, common.BTCAsset, runeAddress)
	c.Assert(err, IsNil)
	c.Check(su.Units.IsZero(), Equals, false)
	p, err = ps.GetPool(ctx, common.BTCAsset)
	c.Assert(err, IsNil)
	c.Check(p.BalanceAsset.Equal(cosmos.NewUint(100*common.One)), Equals, true, Commentf("%d", p.BalanceAsset.Uint64()))
	c.Check(p.BalanceRune.Equal(cosmos.NewUint(100*common.One)), Equals, true, Commentf("%d", p.BalanceRune.Uint64()))
	c.Check(p.LPUnits.Equal(cosmos.NewUint(100*common.One)), Equals, true, Commentf("%d", p.LPUnits.Uint64()))
}

func (HandlerAddLiquiditySuite) TestRuneOnlyLiquidity(c *C) {
	ctx, k := setupKeeperForTest(c)
	txID := GetRandomTxHash()

	c.Assert(k.SetPool(ctx, Pool{
		BalanceRune:  cosmos.NewUint(100 * common.One),
		BalanceAsset: cosmos.NewUint(100 * common.One),
		Asset:        common.BTCAsset,
		LPUnits:      cosmos.NewUint(100 * common.One),
		SynthUnits:   cosmos.ZeroUint(),
		Status:       PoolAvailable,
	}), IsNil)

	runeAddr := GetRandomRUNEAddress()
	constAccessor := constants.GetConstantValues(GetCurrentVersion())
	h := NewAddLiquidityHandler(NewDummyMgrWithKeeper(k))
	err := h.addLiquidity(ctx, common.BTCAsset, cosmos.NewUint(100*common.One), cosmos.ZeroUint(), runeAddr, common.NoAddress, txID, false, constAccessor)
	c.Assert(err, IsNil)

	su, err := k.GetLiquidityProvider(ctx, common.BTCAsset, runeAddr)
	c.Assert(err, IsNil)
	c.Assert(su.Units.Uint64(), Equals, uint64(2500000000), Commentf("%d", su.Units.Uint64()))

	pool, err := k.GetPool(ctx, common.BTCAsset)
	c.Assert(err, IsNil)
	c.Assert(pool.LPUnits.Uint64(), Equals, uint64(12500000000), Commentf("%d", pool.LPUnits.Uint64()))
}

func (HandlerAddLiquiditySuite) TestAssetOnlyProvidedLiquidity(c *C) {
	ctx, k := setupKeeperForTest(c)
	txID := GetRandomTxHash()

	c.Assert(k.SetPool(ctx, Pool{
		BalanceRune:  cosmos.NewUint(100 * common.One),
		BalanceAsset: cosmos.NewUint(100 * common.One),
		Asset:        common.BTCAsset,
		LPUnits:      cosmos.NewUint(100 * common.One),
		SynthUnits:   cosmos.ZeroUint(),
		Status:       PoolAvailable,
	}), IsNil)

	assetAddr := GetRandomBTCAddress()
	constAccessor := constants.GetConstantValues(GetCurrentVersion())
	h := NewAddLiquidityHandler(NewDummyMgrWithKeeper(k))
	err := h.addLiquidity(ctx, common.BTCAsset, cosmos.ZeroUint(), cosmos.NewUint(100*common.One), common.NoAddress, assetAddr, txID, false, constAccessor)
	c.Assert(err, IsNil)

	su, err := k.GetLiquidityProvider(ctx, common.BTCAsset, assetAddr)
	c.Assert(err, IsNil)
	c.Assert(su.Units.Uint64(), Equals, uint64(2500000000), Commentf("%d", su.Units.Uint64()))

	pool, err := k.GetPool(ctx, common.BTCAsset)
	c.Assert(err, IsNil)
	c.Assert(pool.LPUnits.Uint64(), Equals, uint64(12500000000), Commentf("%d", pool.LPUnits.Uint64()))
}
