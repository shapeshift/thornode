package thorchain

import (
	"errors"

	"github.com/blang/semver"
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

type HandlerUnstakeSuite struct{}

var _ = Suite(&HandlerUnstakeSuite{})

type MockUnstakeKeeper struct {
	KVStoreDummy
	activeNodeAccount NodeAccount
	currentPool       Pool
	failPool          bool
	suspendedPool     bool
	failStaker        bool
	failAddEvents     bool
	staker            Staker
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
	iter := NewDummyIterator()
	iter.AddItem([]byte("key"), mfp.Cdc().MustMarshalBinaryBare(mfp.staker))
	return iter
}

func (mfp *MockUnstakeKeeper) GetStaker(_ cosmos.Context, _ common.Asset, _ common.Address) (Staker, error) {
	if mfp.failStaker {
		return Staker{}, errors.New("fail to get staker")
	}
	return mfp.staker, nil
}

func (mfp *MockUnstakeKeeper) SetStaker(_ cosmos.Context, staker Staker) {
	mfp.staker = staker
}

func (mfp *MockUnstakeKeeper) UpsertEvent(ctx cosmos.Context, event Event) error { return nil }

func (mfp *MockUnstakeKeeper) GetGas(ctx cosmos.Context, asset common.Asset) ([]cosmos.Uint, error) {
	return []cosmos.Uint{cosmos.NewUint(37500), cosmos.NewUint(30000)}, nil
}

func (HandlerUnstakeSuite) TestUnstakeHandler(c *C) {
	// w := getHandlerTestWrapper(c, 1, true, true)
	ctx, _ := setupKeeperForTest(c)
	activeNodeAccount := GetRandomNodeAccount(NodeActive)
	k := &MockUnstakeKeeper{
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
	ver := constants.SWVersion
	constAccessor := constants.GetConstantValues(ver)
	// Happy path , this is a round trip , first we stake, then we unstake
	runeAddr := GetRandomRUNEAddress()
	unit, err := stake(ctx,
		k,
		common.BNBAsset,
		cosmos.NewUint(common.One*100),
		cosmos.NewUint(common.One*100),
		runeAddr,
		GetRandomBNBAddress(),
		GetRandomTxHash(),
		constAccessor)
	c.Assert(err, IsNil)
	c.Logf("stake unit: %d", unit)
	// let's just unstake
	unstakeHandler := NewUnstakeHandler(k, NewVersionedTxOutStoreDummy(), NewDummyVersionedEventMgr())

	msgUnstake := NewMsgSetUnStake(GetRandomTx(), runeAddr, cosmos.NewUint(uint64(MaxUnstakeBasisPoints)), common.BNBAsset, activeNodeAccount.NodeAddress)
	result := unstakeHandler.Run(ctx, msgUnstake, ver, constAccessor)
	c.Assert(result.Code, Equals, cosmos.CodeOK, Commentf("+v", result))

	// Bad version should fail
	result = unstakeHandler.Run(ctx, msgUnstake, semver.Version{}, constAccessor)
	c.Assert(result.Code, Equals, CodeBadVersion)
}

func (HandlerUnstakeSuite) TestUnstakeHandler_Validation(c *C) {
	ctx, k := setupKeeperForTest(c)
	testCases := []struct {
		name           string
		msg            MsgSetUnStake
		expectedResult cosmos.CodeType
	}{
		{
			name:           "not signed by active observer should fail",
			msg:            NewMsgSetUnStake(GetRandomTx(), GetRandomRUNEAddress(), cosmos.NewUint(uint64(MaxUnstakeBasisPoints)), common.BNBAsset, GetRandomNodeAccount(NodeActive).NodeAddress),
			expectedResult: cosmos.CodeUnauthorized,
		},
		{
			name:           "empty signer should fail",
			msg:            NewMsgSetUnStake(GetRandomTx(), GetRandomRUNEAddress(), cosmos.NewUint(uint64(MaxUnstakeBasisPoints)), common.BNBAsset, cosmos.AccAddress{}),
			expectedResult: CodeUnstakeFailValidation,
		},
		{
			name:           "empty asset should fail",
			msg:            NewMsgSetUnStake(GetRandomTx(), GetRandomRUNEAddress(), cosmos.NewUint(uint64(MaxUnstakeBasisPoints)), common.Asset{}, GetRandomNodeAccount(NodeActive).NodeAddress),
			expectedResult: CodeUnstakeFailValidation,
		},
		{
			name:           "empty RUNE address should fail",
			msg:            NewMsgSetUnStake(GetRandomTx(), common.NoAddress, cosmos.NewUint(uint64(MaxUnstakeBasisPoints)), common.BNBAsset, GetRandomNodeAccount(NodeActive).NodeAddress),
			expectedResult: CodeUnstakeFailValidation,
		},
		{
			name:           "withdraw basis point is 0 should fail",
			msg:            NewMsgSetUnStake(GetRandomTx(), GetRandomRUNEAddress(), cosmos.ZeroUint(), common.BNBAsset, GetRandomNodeAccount(NodeActive).NodeAddress),
			expectedResult: CodeUnstakeFailValidation,
		},
		{
			name:           "withdraw basis point is larger than 10000 should fail",
			msg:            NewMsgSetUnStake(GetRandomTx(), GetRandomRUNEAddress(), cosmos.NewUint(uint64(MaxUnstakeBasisPoints+100)), common.BNBAsset, GetRandomNodeAccount(NodeActive).NodeAddress),
			expectedResult: CodeUnstakeFailValidation,
		},
	}
	ver := constants.SWVersion
	constAccessor := constants.GetConstantValues(ver)
	for _, tc := range testCases {
		unstakeHandler := NewUnstakeHandler(k, NewVersionedTxOutStoreDummy(), NewDummyVersionedEventMgr())
		c.Assert(unstakeHandler.Run(ctx, tc.msg, ver, constAccessor).Code, Equals, tc.expectedResult, Commentf(tc.name))
	}
}

func (HandlerUnstakeSuite) TestUnstakeHandler_mockFailScenarios(c *C) {
	activeNodeAccount := GetRandomNodeAccount(NodeActive)
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
		k              Keeper
		expectedResult cosmos.CodeType
	}{
		{
			name: "fail to get pool unstake should fail",
			k: &MockUnstakeKeeper{
				activeNodeAccount: activeNodeAccount,
				failPool:          true,
				staker:            staker,
			},
			expectedResult: cosmos.CodeInternal,
		},
		{
			name: "suspended pool unstake should fail",
			k: &MockUnstakeKeeper{
				activeNodeAccount: activeNodeAccount,
				suspendedPool:     true,
				staker:            staker,
			},
			expectedResult: CodeInvalidPoolStatus,
		},
		{
			name: "fail to get staker unstake should fail",
			k: &MockUnstakeKeeper{
				activeNodeAccount: activeNodeAccount,
				failStaker:        true,
				staker:            staker,
			},
			expectedResult: CodeFailGetStaker,
		},
		{
			name: "fail to add incomplete event unstake should fail",
			k: &MockUnstakeKeeper{
				activeNodeAccount: activeNodeAccount,
				currentPool:       currentPool,
				failAddEvents:     true,
				staker:            staker,
			},
			expectedResult: cosmos.CodeInternal,
		},
	}
	ver := constants.SWVersion
	constAccessor := constants.GetConstantValues(ver)

	for _, tc := range testCases {
		ctx, _ := setupKeeperForTest(c)
		unstakeHandler := NewUnstakeHandler(tc.k, NewVersionedTxOutStoreDummy(), NewDummyVersionedEventMgr())
		msgUnstake := NewMsgSetUnStake(GetRandomTx(), GetRandomRUNEAddress(), cosmos.NewUint(uint64(MaxUnstakeBasisPoints)), common.BNBAsset, activeNodeAccount.NodeAddress)
		c.Assert(unstakeHandler.Run(ctx, msgUnstake, ver, constAccessor).Code, Equals, tc.expectedResult, Commentf(tc.name))
	}
}
