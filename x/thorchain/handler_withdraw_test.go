package thorchain

import (
	"errors"

	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

type HandlerWithdrawSuite struct{}

var _ = Suite(&HandlerWithdrawSuite{})

type MockWithdrawKeeper struct {
	keeper.KVStoreDummy
	activeNodeAccount     NodeAccount
	currentPool           Pool
	failPool              bool
	suspendedPool         bool
	failLiquidityProvider bool
	failAddEvents         bool
	lp                    LiquidityProvider
	keeper                keeper.Keeper
}

func (mfp *MockWithdrawKeeper) PoolExist(_ cosmos.Context, asset common.Asset) bool {
	return mfp.currentPool.Asset.Equals(asset)
}

// GetPool return a pool
func (mfp *MockWithdrawKeeper) GetPool(_ cosmos.Context, _ common.Asset) (Pool, error) {
	if mfp.failPool {
		return Pool{}, errors.New("test error")
	}
	if mfp.suspendedPool {
		return Pool{
			BalanceRune:  cosmos.ZeroUint(),
			BalanceAsset: cosmos.ZeroUint(),
			Asset:        common.BNBAsset,
			LPUnits:      cosmos.ZeroUint(),
			Status:       PoolSuspended,
		}, nil
	}
	return mfp.currentPool, nil
}

func (mfp *MockWithdrawKeeper) SetPool(_ cosmos.Context, pool Pool) error {
	mfp.currentPool = pool
	return nil
}

func (mfp *MockWithdrawKeeper) GetNodeAccount(_ cosmos.Context, addr cosmos.AccAddress) (NodeAccount, error) {
	if mfp.activeNodeAccount.NodeAddress.Equals(addr) {
		return mfp.activeNodeAccount, nil
	}
	return NodeAccount{}, nil
}

func (mfp *MockWithdrawKeeper) GetLiquidityProviderIterator(ctx cosmos.Context, _ common.Asset) cosmos.Iterator {
	iter := keeper.NewDummyIterator()
	iter.AddItem([]byte("key"), mfp.Cdc().MustMarshal(&mfp.lp))
	return iter
}

func (mfp *MockWithdrawKeeper) GetLiquidityProvider(ctx cosmos.Context, asset common.Asset, addr common.Address) (LiquidityProvider, error) {
	if mfp.failLiquidityProvider {
		return LiquidityProvider{}, errors.New("fail to get liquidity provider")
	}
	return mfp.lp, nil
}

func (mfp *MockWithdrawKeeper) SetLiquidityProvider(_ cosmos.Context, lp LiquidityProvider) {
	mfp.lp = lp
}

func (mfp *MockWithdrawKeeper) GetGas(ctx cosmos.Context, asset common.Asset) ([]cosmos.Uint, error) {
	return []cosmos.Uint{cosmos.NewUint(37500), cosmos.NewUint(30000)}, nil
}

func (HandlerWithdrawSuite) TestWithdrawHandler(c *C) {
	// w := getHandlerTestWrapper(c, 1, true, true)
	SetupConfigForTest()
	ctx, keeper := setupKeeperForTest(c)
	activeNodeAccount := GetRandomValidatorNode(NodeActive)
	k := &MockWithdrawKeeper{
		keeper:            keeper,
		activeNodeAccount: activeNodeAccount,
		currentPool: Pool{
			BalanceRune:         cosmos.ZeroUint(),
			BalanceAsset:        cosmos.ZeroUint(),
			Asset:               common.BNBAsset,
			LPUnits:             cosmos.ZeroUint(),
			SynthUnits:          cosmos.ZeroUint(),
			PendingInboundRune:  cosmos.ZeroUint(),
			PendingInboundAsset: cosmos.ZeroUint(),
			Status:              PoolAvailable,
		},
		lp: LiquidityProvider{
			Units:             cosmos.ZeroUint(),
			PendingRune:       cosmos.ZeroUint(),
			PendingAsset:      cosmos.ZeroUint(),
			RuneDepositValue:  cosmos.ZeroUint(),
			AssetDepositValue: cosmos.ZeroUint(),
		},
	}
	ver := GetCurrentVersion()
	constAccessor := constants.GetConstantValues(ver)
	// Happy path , this is a round trip , first we provide liquidity, then we withdraw
	runeAddr := GetRandomRUNEAddress()
	addHandler := NewAddLiquidityHandler(NewDummyMgrWithKeeper(k))
	err := addHandler.addLiquidityV1(ctx,
		common.BNBAsset,
		cosmos.NewUint(common.One*100),
		cosmos.NewUint(common.One*100),
		runeAddr,
		GetRandomBNBAddress(),
		GetRandomTxHash(),
		false,
		constAccessor)
	c.Assert(err, IsNil)
	// let's just withdraw
	withdrawHandler := NewWithdrawLiquidityHandler(NewDummyMgrWithKeeper(k))

	msgWithdraw := NewMsgWithdrawLiquidity(GetRandomTx(), runeAddr, cosmos.NewUint(uint64(MaxWithdrawBasisPoints)), common.BNBAsset, common.EmptyAsset, activeNodeAccount.NodeAddress)
	_, err = withdrawHandler.Run(ctx, msgWithdraw)
	c.Assert(err, IsNil)

	// Bad version should fail
	_, err = withdrawHandler.Run(ctx, msgWithdraw)
	c.Assert(err, NotNil)
}

func (HandlerWithdrawSuite) TestAsymmetricWithdraw(c *C) {
	SetupConfigForTest()
	ctx, keeper := setupKeeperForTest(c)
	activeNodeAccount := GetRandomValidatorNode(NodeActive)
	ver := GetCurrentVersion()
	constAccessor := constants.GetConstantValues(ver)
	pool := NewPool()
	pool.Asset = common.BTCAsset
	pool.BalanceAsset = cosmos.ZeroUint()
	pool.BalanceRune = cosmos.ZeroUint()
	pool.Status = PoolAvailable
	keeper.SetPool(ctx, pool)
	// Happy path , this is a round trip , first we provide liquidity, then we withdraw
	// Let's stake some BTC first
	runeAddr := GetRandomRUNEAddress()
	btcAddress := GetRandomBTCAddress()
	addHandler := NewAddLiquidityHandler(NewDummyMgrWithKeeper(keeper))
	// stake some RUNE first
	err := addHandler.addLiquidity(ctx,
		common.BTCAsset,
		cosmos.NewUint(common.One*100),
		cosmos.ZeroUint(),
		runeAddr,
		btcAddress,
		GetRandomTxHash(),
		true,
		constAccessor)
	c.Assert(err, IsNil)
	lp, err := keeper.GetLiquidityProvider(ctx, common.BTCAsset, runeAddr)
	c.Assert(err, IsNil)
	c.Assert(lp.Valid(), IsNil)
	c.Assert(lp.PendingRune.Equal(cosmos.NewUint(common.One*100)), Equals, true)
	// Stake some BTC , make sure stake finished
	err = addHandler.addLiquidity(ctx, common.BTCAsset, cosmos.ZeroUint(), cosmos.NewUint(100*common.One), runeAddr, btcAddress, GetRandomTxHash(), false, constAccessor)
	c.Assert(err, IsNil)
	lp, err = keeper.GetLiquidityProvider(ctx, common.BTCAsset, runeAddr)
	c.Assert(err, IsNil)
	c.Assert(lp.Valid(), IsNil)
	c.Assert(lp.PendingRune.IsZero(), Equals, true)
	// symmetric stake, units is measured by liquidity tokens
	c.Assert(lp.Units.IsZero(), Equals, false)

	runeAddr1 := GetRandomRUNEAddress()
	err = addHandler.addLiquidityV1(ctx, common.BTCAsset, cosmos.NewUint(50*common.One), cosmos.ZeroUint(), runeAddr1, common.NoAddress, GetRandomTxHash(), false, constAccessor)
	c.Assert(err, IsNil)
	lp, err = keeper.GetLiquidityProvider(ctx, common.BTCAsset, runeAddr1)
	c.Assert(err, IsNil)
	c.Assert(lp.Valid(), IsNil)
	c.Assert(lp.PendingRune.IsZero(), Equals, true)
	c.Assert(lp.PendingAsset.IsZero(), Equals, true)
	c.Assert(lp.Units.IsZero(), Equals, false)

	// let's withdraw the RUNE we just staked
	withdrawHandler := NewWithdrawLiquidityHandler(NewDummyMgrWithKeeper(keeper))
	msgWithdraw := NewMsgWithdrawLiquidity(GetRandomTx(), runeAddr1, cosmos.NewUint(uint64(MaxWithdrawBasisPoints)), common.BTCAsset, common.EmptyAsset, activeNodeAccount.NodeAddress)
	_, err = withdrawHandler.Run(ctx, msgWithdraw)
	c.Assert(err, IsNil)
	lp, err = keeper.GetLiquidityProvider(ctx, common.BTCAsset, runeAddr1)
	c.Assert(err, IsNil)
	c.Assert(lp.Valid(), NotNil)

	// stake some BTC only
	btcAddress1 := GetRandomBTCAddress()
	err = addHandler.addLiquidityV1(ctx, common.BTCAsset, cosmos.ZeroUint(), cosmos.NewUint(50*common.One),
		common.NoAddress, btcAddress1, GetRandomTxHash(), false, constAccessor)
	c.Assert(err, IsNil)
	lp, err = keeper.GetLiquidityProvider(ctx, common.BTCAsset, btcAddress1)
	c.Assert(err, IsNil)
	c.Assert(lp.Valid(), IsNil)
	c.Assert(lp.PendingRune.IsZero(), Equals, true)
	c.Assert(lp.PendingAsset.IsZero(), Equals, true)
	c.Assert(lp.Units.IsZero(), Equals, false)

	// let's withdraw the BTC we just staked
	msgWithdraw = NewMsgWithdrawLiquidity(GetRandomTx(), btcAddress1, cosmos.NewUint(uint64(MaxWithdrawBasisPoints)), common.BTCAsset, common.EmptyAsset, activeNodeAccount.NodeAddress)
	_, err = withdrawHandler.Run(ctx, msgWithdraw)
	c.Assert(err, IsNil)
	lp, err = keeper.GetLiquidityProvider(ctx, common.BTCAsset, btcAddress1)
	c.Assert(err, IsNil)
	c.Assert(lp.Valid(), NotNil)

	// Bad version should fail
	_, err = withdrawHandler.Run(ctx, msgWithdraw)
	c.Assert(err, NotNil)
}

func (HandlerWithdrawSuite) TestWithdrawHandler_Validation(c *C) {
	ctx, k := setupKeeperForTest(c)
	testCases := []struct {
		name           string
		msg            *MsgWithdrawLiquidity
		expectedResult error
	}{
		{
			name:           "empty signer should fail",
			msg:            NewMsgWithdrawLiquidity(GetRandomTx(), GetRandomRUNEAddress(), cosmos.NewUint(uint64(MaxWithdrawBasisPoints)), common.BNBAsset, common.EmptyAsset, cosmos.AccAddress{}),
			expectedResult: errWithdrawFailValidation,
		},
		{
			name:           "empty asset should fail",
			msg:            NewMsgWithdrawLiquidity(GetRandomTx(), GetRandomRUNEAddress(), cosmos.NewUint(uint64(MaxWithdrawBasisPoints)), common.Asset{}, common.EmptyAsset, GetRandomValidatorNode(NodeActive).NodeAddress),
			expectedResult: errWithdrawFailValidation,
		},
		{
			name:           "empty RUNE address should fail",
			msg:            NewMsgWithdrawLiquidity(GetRandomTx(), common.NoAddress, cosmos.NewUint(uint64(MaxWithdrawBasisPoints)), common.BNBAsset, common.EmptyAsset, GetRandomValidatorNode(NodeActive).NodeAddress),
			expectedResult: errWithdrawFailValidation,
		},
		{
			name:           "withdraw basis point is 0 should fail",
			msg:            NewMsgWithdrawLiquidity(GetRandomTx(), GetRandomRUNEAddress(), cosmos.ZeroUint(), common.BNBAsset, common.EmptyAsset, GetRandomValidatorNode(NodeActive).NodeAddress),
			expectedResult: errWithdrawFailValidation,
		},
		{
			name:           "withdraw basis point is larger than 10000 should fail",
			msg:            NewMsgWithdrawLiquidity(GetRandomTx(), GetRandomRUNEAddress(), cosmos.NewUint(uint64(MaxWithdrawBasisPoints+100)), common.BNBAsset, common.EmptyAsset, GetRandomValidatorNode(NodeActive).NodeAddress),
			expectedResult: errWithdrawFailValidation,
		},
		{
			name:           "synth asset should fail",
			msg:            NewMsgWithdrawLiquidity(GetRandomTx(), GetRandomRUNEAddress(), cosmos.NewUint(uint64(MaxWithdrawBasisPoints)), common.BNBAsset.GetSyntheticAsset(), common.EmptyAsset, GetRandomValidatorNode(NodeActive).NodeAddress),
			expectedResult: errWithdrawFailValidation,
		},
	}
	for _, tc := range testCases {
		withdrawHandler := NewWithdrawLiquidityHandler(NewDummyMgrWithKeeper(k))
		_, err := withdrawHandler.Run(ctx, tc.msg)
		c.Assert(err.Error(), Equals, tc.expectedResult.Error(), Commentf(tc.name))
	}
}

func (HandlerWithdrawSuite) TestWithdrawHandler_mockFailScenarios(c *C) {
	activeNodeAccount := GetRandomValidatorNode(NodeActive)
	ctx, k := setupKeeperForTest(c)
	currentPool := Pool{
		BalanceRune:  cosmos.ZeroUint(),
		BalanceAsset: cosmos.ZeroUint(),
		Asset:        common.BNBAsset,
		LPUnits:      cosmos.ZeroUint(),
		Status:       PoolAvailable,
	}
	lp := LiquidityProvider{
		Units:        cosmos.ZeroUint(),
		PendingRune:  cosmos.ZeroUint(),
		PendingAsset: cosmos.ZeroUint(),
	}
	testCases := []struct {
		name           string
		k              keeper.Keeper
		expectedResult error
	}{
		{
			name: "fail to get pool withdraw should fail",
			k: &MockWithdrawKeeper{
				activeNodeAccount: activeNodeAccount,
				failPool:          true,
				lp:                lp,
				keeper:            k,
			},
			expectedResult: errInternal,
		},
		{
			name: "suspended pool withdraw should fail",
			k: &MockWithdrawKeeper{
				activeNodeAccount: activeNodeAccount,
				suspendedPool:     true,
				lp:                lp,
				keeper:            k,
			},
			expectedResult: errInvalidPoolStatus,
		},
		{
			name: "fail to get liquidity provider withdraw should fail",
			k: &MockWithdrawKeeper{
				activeNodeAccount:     activeNodeAccount,
				currentPool:           currentPool,
				failLiquidityProvider: true,
				lp:                    lp,
				keeper:                k,
			},
			expectedResult: errFailGetLiquidityProvider,
		},
		{
			name: "fail to add incomplete event withdraw should fail",
			k: &MockWithdrawKeeper{
				activeNodeAccount: activeNodeAccount,
				currentPool:       currentPool,
				failAddEvents:     true,
				lp:                lp,
				keeper:            k,
			},
			expectedResult: errInternal,
		},
	}

	for _, tc := range testCases {
		withdrawHandler := NewWithdrawLiquidityHandler(NewDummyMgrWithKeeper(tc.k))
		msgWithdraw := NewMsgWithdrawLiquidity(GetRandomTx(), GetRandomRUNEAddress(), cosmos.NewUint(uint64(MaxWithdrawBasisPoints)), common.BNBAsset, common.EmptyAsset, activeNodeAccount.NodeAddress)
		_, err := withdrawHandler.Run(ctx, msgWithdraw)
		c.Assert(errors.Is(err, tc.expectedResult), Equals, true, Commentf(tc.name))
	}
}

type MockWithdrawTxOutStore struct {
	TxOutStore
	errAsset error
	errRune  error
}

func (store *MockWithdrawTxOutStore) TryAddTxOutItem(ctx cosmos.Context, mgr Manager, toi TxOutItem) (bool, error) {
	if toi.Coin.Asset.IsNativeRune() && store.errRune != nil {
		return false, store.errRune
	}
	if !toi.Coin.Asset.IsNativeRune() && store.errAsset != nil {
		return false, store.errAsset
	}
	return true, nil
}

type MockWithdrawEventMgr struct {
	EventManager
	count int
}

func (m *MockWithdrawEventMgr) EmitEvent(ctx cosmos.Context, evt EmitEventItem) error {
	m.count += 1
	return nil
}

func (HandlerWithdrawSuite) TestWithdrawHandler_outboundFailures(c *C) {
	SetupConfigForTest()
	ctx, keeper := setupKeeperForTest(c)
	na := GetRandomValidatorNode(NodeActive)
	asset := common.BTCAsset

	pool := Pool{
		Asset:               asset,
		BalanceAsset:        cosmos.NewUint(10000),
		BalanceRune:         cosmos.NewUint(10000),
		LPUnits:             cosmos.NewUint(1000),
		SynthUnits:          cosmos.ZeroUint(),
		PendingInboundRune:  cosmos.ZeroUint(),
		PendingInboundAsset: cosmos.ZeroUint(),
		Status:              PoolAvailable,
	}
	c.Assert(pool.Valid(), IsNil)
	_ = keeper.SetPool(ctx, pool)

	runeAddr := GetRandomRUNEAddress()
	lp := LiquidityProvider{
		Asset:              asset,
		LastAddHeight:      common.BlockHeight(ctx),
		RuneAddress:        runeAddr,
		AssetAddress:       GetRandomBTCAddress(),
		Units:              cosmos.NewUint(100),
		LastWithdrawHeight: common.BlockHeight(ctx),
	}
	c.Assert(lp.Valid(), IsNil)
	keeper.SetLiquidityProvider(ctx, lp)

	msg := NewMsgWithdrawLiquidity(
		GetRandomTx(),
		lp.RuneAddress,
		cosmos.NewUint(1000),
		asset,
		common.RuneAsset(),
		na.NodeAddress)

	c.Assert(msg.ValidateBasic(), IsNil)

	mgr := NewDummyMgrWithKeeper(keeper)

	// runs the handler and checks pool/lp state for changes
	handleCase := func(msg *MsgWithdrawLiquidity, errRune, errAsset error, name string, shouldFail bool) {
		_ = keeper.SetPool(ctx, pool)
		keeper.SetLiquidityProvider(ctx, lp)
		mgr.txOutStore = &MockWithdrawTxOutStore{
			TxOutStore: mgr.txOutStore,
			errRune:    errRune,
			errAsset:   errAsset,
		}
		eventMgr := &MockWithdrawEventMgr{
			EventManager: mgr.eventMgr,
			count:        0,
		}
		mgr.eventMgr = eventMgr
		handler := NewWithdrawLiquidityHandler(mgr)
		_, err := handler.Run(ctx, msg)
		lpAfter, _ := keeper.GetLiquidityProvider(ctx, asset, runeAddr)
		poolAfter, _ := keeper.GetPool(ctx, asset)
		if shouldFail {
			// should error and leave pool/lp unmodified
			c.Assert(err, NotNil, Commentf(name))
			c.Assert(lpAfter.String(), Equals, lp.String(), Commentf(name))
			c.Assert(poolAfter.String(), Equals, pool.String(), Commentf(name))
			c.Assert(eventMgr.count, Equals, 0, Commentf(name))
		} else {
			// should not error and pool/lp  should be modified
			c.Assert(err, IsNil, Commentf(name))
			c.Assert(lpAfter.String(), Not(Equals), lp.String(), Commentf(name))
			c.Assert(poolAfter.String(), Not(Equals), pool.String(), Commentf(name))
			c.Assert(eventMgr.count, Equals, 1, Commentf(name))
		}
	}

	msg.WithdrawalAsset = common.RuneAsset()
	handleCase(msg, errInternal, nil, "asym rune fail", true)

	msg.WithdrawalAsset = common.BTCAsset
	handleCase(msg, nil, errInternal, "asym asset fail", true)

	msg.WithdrawalAsset = common.EmptyAsset
	handleCase(msg, errInternal, nil, "sym rune fail/asset success", false)
	handleCase(msg, nil, errInternal, "sym rune success/asset fail", true)
	handleCase(msg, errInternal, errInternal, "sym rune/asset fail", true)
	handleCase(msg, nil, nil, "sym rune/asset success", false)
}
