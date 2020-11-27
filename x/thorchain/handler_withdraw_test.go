package thorchain

import (
	"errors"

	"github.com/blang/semver"
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
			PoolUnits:    cosmos.ZeroUint(),
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
	iter.AddItem([]byte("key"), mfp.Cdc().MustMarshalBinaryBare(mfp.lp))
	return iter
}

func (mfp *MockWithdrawKeeper) GetLiquidityProvider(ctx cosmos.Context, asset common.Asset, addr common.Address) (LiquidityProvider, error) {
	if mfp.failLiquidityProvider {
		return LiquidityProvider{}, errors.New("fail to get liquidity provider")
	}
	accAddr, err := addr.AccAddress()
	if err != nil {
		return mfp.lp, err
	}
	mfp.lp.Units = mfp.keeper.GetLiquidityProviderBalance(ctx, asset.LiquidityAsset(), accAddr)
	return mfp.lp, nil
}

func (mfp *MockWithdrawKeeper) AddOwnership(ctx cosmos.Context, coin common.Coin, addr cosmos.AccAddress) error {
	return mfp.keeper.AddOwnership(ctx, coin, addr)
}

func (mfp *MockWithdrawKeeper) RemoveOwnership(ctx cosmos.Context, coin common.Coin, addr cosmos.AccAddress) error {
	return mfp.keeper.RemoveOwnership(ctx, coin, addr)
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
	activeNodeAccount := GetRandomNodeAccount(NodeActive)
	k := &MockWithdrawKeeper{
		keeper:            keeper,
		activeNodeAccount: activeNodeAccount,
		currentPool: Pool{
			BalanceRune:  cosmos.ZeroUint(),
			BalanceAsset: cosmos.ZeroUint(),
			Asset:        common.BNBAsset,
			PoolUnits:    cosmos.ZeroUint(),
			Status:       PoolAvailable,
		},
		lp: LiquidityProvider{
			Units:        cosmos.ZeroUint(),
			PendingRune:  cosmos.ZeroUint(),
			PendingAsset: cosmos.ZeroUint(),
		},
	}
	ver := semver.MustParse("0.7.0")
	constAccessor := constants.GetConstantValues(ver)
	// Happy path , this is a round trip , first we provide liquidity, then we withdraw
	runeAddr := GetRandomRUNEAddress()
	addHandler := NewAddLiquidityHandler(k, NewDummyMgr())
	err := addHandler.addLiquidityV1(ctx,
		common.BNBAsset,
		cosmos.NewUint(common.One*100),
		cosmos.NewUint(common.One*100),
		runeAddr,
		GetRandomBNBAddress(),
		GetRandomTxHash(),
		constAccessor)
	c.Assert(err, IsNil)
	// let's just withdraw
	withdrawHandler := NewWithdrawLiquidityHandler(k, NewDummyMgr())

	msgWithdraw := NewMsgWithdrawLiquidity(GetRandomTx(), runeAddr, cosmos.NewUint(uint64(MaxWithdrawBasisPoints)), common.BNBAsset, common.EmptyAsset, activeNodeAccount.NodeAddress)
	_, err = withdrawHandler.Run(ctx, msgWithdraw, ver, constAccessor)
	c.Assert(err, IsNil)

	// Bad version should fail
	_, err = withdrawHandler.Run(ctx, msgWithdraw, semver.Version{}, constAccessor)
	c.Assert(err, NotNil)
}

func (HandlerWithdrawSuite) TestWithdrawHandler_Validation(c *C) {
	ctx, k := setupKeeperForTest(c)
	testCases := []struct {
		name           string
		msg            MsgWithdrawLiquidity
		expectedResult error
	}{
		{
			name:           "empty signer should fail",
			msg:            NewMsgWithdrawLiquidity(GetRandomTx(), GetRandomRUNEAddress(), cosmos.NewUint(uint64(MaxWithdrawBasisPoints)), common.BNBAsset, common.EmptyAsset, cosmos.AccAddress{}),
			expectedResult: errWithdrawFailValidation,
		},
		{
			name:           "empty asset should fail",
			msg:            NewMsgWithdrawLiquidity(GetRandomTx(), GetRandomRUNEAddress(), cosmos.NewUint(uint64(MaxWithdrawBasisPoints)), common.Asset{}, common.EmptyAsset, GetRandomNodeAccount(NodeActive).NodeAddress),
			expectedResult: errWithdrawFailValidation,
		},
		{
			name:           "empty RUNE address should fail",
			msg:            NewMsgWithdrawLiquidity(GetRandomTx(), common.NoAddress, cosmos.NewUint(uint64(MaxWithdrawBasisPoints)), common.BNBAsset, common.EmptyAsset, GetRandomNodeAccount(NodeActive).NodeAddress),
			expectedResult: errWithdrawFailValidation,
		},
		{
			name:           "withdraw basis point is 0 should fail",
			msg:            NewMsgWithdrawLiquidity(GetRandomTx(), GetRandomRUNEAddress(), cosmos.ZeroUint(), common.BNBAsset, common.EmptyAsset, GetRandomNodeAccount(NodeActive).NodeAddress),
			expectedResult: errWithdrawFailValidation,
		},
		{
			name:           "withdraw basis point is larger than 10000 should fail",
			msg:            NewMsgWithdrawLiquidity(GetRandomTx(), GetRandomRUNEAddress(), cosmos.NewUint(uint64(MaxWithdrawBasisPoints+100)), common.BNBAsset, common.EmptyAsset, GetRandomNodeAccount(NodeActive).NodeAddress),
			expectedResult: errWithdrawFailValidation,
		},
	}
	ver := semver.MustParse("0.7.0")
	constAccessor := constants.GetConstantValues(ver)
	for _, tc := range testCases {
		withdrawHandler := NewWithdrawLiquidityHandler(k, NewDummyMgr())
		_, err := withdrawHandler.Run(ctx, tc.msg, ver, constAccessor)
		c.Assert(err.Error(), Equals, tc.expectedResult.Error(), Commentf(tc.name))
	}
}

func (HandlerWithdrawSuite) TestWithdrawHandler_mockFailScenarios(c *C) {
	activeNodeAccount := GetRandomNodeAccount(NodeActive)
	ctx, k := setupKeeperForTest(c)
	currentPool := Pool{
		BalanceRune:  cosmos.ZeroUint(),
		BalanceAsset: cosmos.ZeroUint(),
		Asset:        common.BNBAsset,
		PoolUnits:    cosmos.ZeroUint(),
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
	ver := semver.MustParse("0.7.0")
	constAccessor := constants.GetConstantValues(ver)

	for _, tc := range testCases {
		withdrawHandler := NewWithdrawLiquidityHandler(tc.k, NewDummyMgr())
		msgWithdraw := NewMsgWithdrawLiquidity(GetRandomTx(), GetRandomRUNEAddress(), cosmos.NewUint(uint64(MaxWithdrawBasisPoints)), common.BNBAsset, common.EmptyAsset, activeNodeAccount.NodeAddress)
		_, err := withdrawHandler.Run(ctx, msgWithdraw, ver, constAccessor)
		c.Assert(errors.Is(err, tc.expectedResult), Equals, true, Commentf(tc.name))
	}
}
