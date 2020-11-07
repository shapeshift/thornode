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

type HandlerUnstakeSuite struct{}

var _ = Suite(&HandlerUnstakeSuite{})

type MockUnstakeKeeper struct {
	keeper.KVStoreDummy
	activeNodeAccount NodeAccount
	currentPool       Pool
	failPool          bool
	suspendedPool     bool
	failStaker        bool
	failAddEvents     bool
	staker            Staker
	keeper            keeper.Keeper
}

func (mfp *MockUnstakeKeeper) PoolExist(_ cosmos.Context, asset common.Asset) bool {
	return mfp.currentPool.Asset.Equals(asset)
}

// GetPool return a pool
func (mfp *MockUnstakeKeeper) GetPool(_ cosmos.Context, _ common.Asset) (Pool, error) {
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

func (mfp *MockUnstakeKeeper) SetPool(_ cosmos.Context, pool Pool) error {
	mfp.currentPool = pool
	return nil
}

func (mfp *MockUnstakeKeeper) GetNodeAccount(_ cosmos.Context, addr cosmos.AccAddress) (NodeAccount, error) {
	if mfp.activeNodeAccount.NodeAddress.Equals(addr) {
		return mfp.activeNodeAccount, nil
	}
	return NodeAccount{}, nil
}

func (mfp *MockUnstakeKeeper) GetStakerIterator(ctx cosmos.Context, _ common.Asset) cosmos.Iterator {
	iter := keeper.NewDummyIterator()
	iter.AddItem([]byte("key"), mfp.Cdc().MustMarshalBinaryBare(mfp.staker))
	return iter
}

func (mfp *MockUnstakeKeeper) GetStaker(ctx cosmos.Context, asset common.Asset, addr common.Address) (Staker, error) {
	if mfp.failStaker {
		return Staker{}, errors.New("fail to get staker")
	}
	accAddr, err := addr.AccAddress()
	if err != nil {
		return mfp.staker, err
	}
	mfp.staker.Units = mfp.keeper.GetStakerBalance(ctx, asset.LiquidityAsset(), accAddr)
	return mfp.staker, nil
}

func (mfp *MockUnstakeKeeper) AddStake(ctx cosmos.Context, coin common.Coin, addr cosmos.AccAddress) error {
	return mfp.keeper.AddStake(ctx, coin, addr)
}

func (mfp *MockUnstakeKeeper) RemoveStake(ctx cosmos.Context, coin common.Coin, addr cosmos.AccAddress) error {
	return mfp.keeper.RemoveStake(ctx, coin, addr)
}

func (mfp *MockUnstakeKeeper) SetStaker(_ cosmos.Context, staker Staker) {
	mfp.staker = staker
}

func (mfp *MockUnstakeKeeper) GetGas(ctx cosmos.Context, asset common.Asset) ([]cosmos.Uint, error) {
	return []cosmos.Uint{cosmos.NewUint(37500), cosmos.NewUint(30000)}, nil
}

func (HandlerUnstakeSuite) TestUnstakeHandler(c *C) {
	// w := getHandlerTestWrapper(c, 1, true, true)
	ctx, keeper := setupKeeperForTest(c)
	activeNodeAccount := GetRandomNodeAccount(NodeActive)
	k := &MockUnstakeKeeper{
		keeper:            keeper,
		activeNodeAccount: activeNodeAccount,
		currentPool: Pool{
			BalanceRune:  cosmos.ZeroUint(),
			BalanceAsset: cosmos.ZeroUint(),
			Asset:        common.BNBAsset,
			PoolUnits:    cosmos.ZeroUint(),
			Status:       PoolEnabled,
		},
		staker: Staker{
			Units:       cosmos.ZeroUint(),
			PendingRune: cosmos.ZeroUint(),
		},
	}
	ver := semver.MustParse("0.7.0")
	constAccessor := constants.GetConstantValues(ver)
	// Happy path , this is a round trip , first we stake, then we unstake
	runeAddr := GetRandomRUNEAddress()
	stakeHandler := NewStakeHandler(k, NewDummyMgr())
	err := stakeHandler.stakeV14(ctx,
		common.BNBAsset,
		cosmos.NewUint(common.One*100),
		cosmos.NewUint(common.One*100),
		runeAddr,
		GetRandomBNBAddress(),
		GetRandomTxHash(),
		constAccessor)
	c.Assert(err, IsNil)
	// let's just unstake
	unstakeHandler := NewUnstakeHandler(k, NewDummyMgr())

	msgUnstake := NewMsgUnStake(GetRandomTx(), runeAddr, cosmos.NewUint(uint64(MaxUnstakeBasisPoints)), common.BNBAsset, common.EmptyAsset, activeNodeAccount.NodeAddress)
	_, err = unstakeHandler.Run(ctx, msgUnstake, ver, constAccessor)
	c.Assert(err, IsNil)

	// Bad version should fail
	_, err = unstakeHandler.Run(ctx, msgUnstake, semver.Version{}, constAccessor)
	c.Assert(err, NotNil)
}

func (HandlerUnstakeSuite) TestUnstakeHandler_Validation(c *C) {
	ctx, k := setupKeeperForTest(c)
	testCases := []struct {
		name           string
		msg            MsgUnStake
		expectedResult error
	}{
		{
			name:           "empty signer should fail",
			msg:            NewMsgUnStake(GetRandomTx(), GetRandomRUNEAddress(), cosmos.NewUint(uint64(MaxUnstakeBasisPoints)), common.BNBAsset, common.EmptyAsset, cosmos.AccAddress{}),
			expectedResult: errUnstakeFailValidation,
		},
		{
			name:           "empty asset should fail",
			msg:            NewMsgUnStake(GetRandomTx(), GetRandomRUNEAddress(), cosmos.NewUint(uint64(MaxUnstakeBasisPoints)), common.Asset{}, common.EmptyAsset, GetRandomNodeAccount(NodeActive).NodeAddress),
			expectedResult: errUnstakeFailValidation,
		},
		{
			name:           "empty RUNE address should fail",
			msg:            NewMsgUnStake(GetRandomTx(), common.NoAddress, cosmos.NewUint(uint64(MaxUnstakeBasisPoints)), common.BNBAsset, common.EmptyAsset, GetRandomNodeAccount(NodeActive).NodeAddress),
			expectedResult: errUnstakeFailValidation,
		},
		{
			name:           "withdraw basis point is 0 should fail",
			msg:            NewMsgUnStake(GetRandomTx(), GetRandomRUNEAddress(), cosmos.ZeroUint(), common.BNBAsset, common.EmptyAsset, GetRandomNodeAccount(NodeActive).NodeAddress),
			expectedResult: errUnstakeFailValidation,
		},
		{
			name:           "withdraw basis point is larger than 10000 should fail",
			msg:            NewMsgUnStake(GetRandomTx(), GetRandomRUNEAddress(), cosmos.NewUint(uint64(MaxUnstakeBasisPoints+100)), common.BNBAsset, common.EmptyAsset, GetRandomNodeAccount(NodeActive).NodeAddress),
			expectedResult: errUnstakeFailValidation,
		},
	}
	ver := semver.MustParse("0.7.0")
	constAccessor := constants.GetConstantValues(ver)
	for _, tc := range testCases {
		unstakeHandler := NewUnstakeHandler(k, NewDummyMgr())
		_, err := unstakeHandler.Run(ctx, tc.msg, ver, constAccessor)
		c.Assert(err.Error(), Equals, tc.expectedResult.Error(), Commentf(tc.name))
	}
}

func (HandlerUnstakeSuite) TestUnstakeHandler_mockFailScenarios(c *C) {
	activeNodeAccount := GetRandomNodeAccount(NodeActive)
	ctx, k := setupKeeperForTest(c)
	currentPool := Pool{
		BalanceRune:  cosmos.ZeroUint(),
		BalanceAsset: cosmos.ZeroUint(),
		Asset:        common.BNBAsset,
		PoolUnits:    cosmos.ZeroUint(),
		Status:       PoolEnabled,
	}
	staker := Staker{
		Units:       cosmos.ZeroUint(),
		PendingRune: cosmos.ZeroUint(),
	}
	testCases := []struct {
		name           string
		k              keeper.Keeper
		expectedResult error
	}{
		{
			name: "fail to get pool unstake should fail",
			k: &MockUnstakeKeeper{
				activeNodeAccount: activeNodeAccount,
				failPool:          true,
				staker:            staker,
				keeper:            k,
			},
			expectedResult: errInternal,
		},
		{
			name: "suspended pool unstake should fail",
			k: &MockUnstakeKeeper{
				activeNodeAccount: activeNodeAccount,
				suspendedPool:     true,
				staker:            staker,
				keeper:            k,
			},
			expectedResult: errInvalidPoolStatus,
		},
		{
			name: "fail to get staker unstake should fail",
			k: &MockUnstakeKeeper{
				activeNodeAccount: activeNodeAccount,
				failStaker:        true,
				staker:            staker,
				keeper:            k,
			},
			expectedResult: errFailGetStaker,
		},
		{
			name: "fail to add incomplete event unstake should fail",
			k: &MockUnstakeKeeper{
				activeNodeAccount: activeNodeAccount,
				currentPool:       currentPool,
				failAddEvents:     true,
				staker:            staker,
				keeper:            k,
			},
			expectedResult: errInternal,
		},
	}
	ver := semver.MustParse("0.7.0")
	constAccessor := constants.GetConstantValues(ver)

	for _, tc := range testCases {
		unstakeHandler := NewUnstakeHandler(tc.k, NewDummyMgr())
		msgUnstake := NewMsgUnStake(GetRandomTx(), GetRandomRUNEAddress(), cosmos.NewUint(uint64(MaxUnstakeBasisPoints)), common.BNBAsset, common.EmptyAsset, activeNodeAccount.NodeAddress)
		_, err := unstakeHandler.Run(ctx, msgUnstake, ver, constAccessor)
		c.Assert(errors.Is(err, tc.expectedResult), Equals, true, Commentf(tc.name))
	}
}
