package thorchain

import (
	"errors"
	"fmt"

	"github.com/blang/semver"
	se "github.com/cosmos/cosmos-sdk/types/errors"
	tmtypes "github.com/tendermint/tendermint/types"
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
	"gitlab.com/thorchain/thornode/x/thorchain/types"
)

type HandlerNativeTxSuite struct{}

var _ = Suite(&HandlerNativeTxSuite{})

func (s *HandlerNativeTxSuite) TestValidate(c *C) {
	ctx, k := setupKeeperForTest(c)

	addr := GetRandomBech32Addr()

	coins := common.Coins{
		common.NewCoin(common.RuneNative, cosmos.NewUint(200*common.One)),
	}
	msg := NewMsgNativeTx(coins, fmt.Sprintf("ADD:BNB.BNB:%s", GetRandomRUNEAddress()), addr)

	handler := NewNativeTxHandler(k, NewDummyMgr())
	err := handler.validate(ctx, msg, constants.SWVersion)
	c.Assert(err, IsNil)

	// invalid version
	err = handler.validate(ctx, msg, semver.Version{})
	c.Assert(err, Equals, errInvalidVersion)

	// invalid msg
	msg = MsgNativeTx{}
	err = handler.validate(ctx, msg, constants.SWVersion)
	c.Assert(err, NotNil)
}

func (s *HandlerNativeTxSuite) TestHandle(c *C) {
	ctx, k := setupKeeperForTest(c)
	banker := k.CoinKeeper()
	constAccessor := constants.NewDummyConstants(map[constants.ConstantName]int64{
		constants.OutboundTransactionFee: 1000_000,
	}, map[constants.ConstantName]bool{}, map[constants.ConstantName]string{})
	activeNode := GetRandomNodeAccount(NodeActive)
	k.SetNodeAccount(ctx, activeNode)
	dummyMgr := NewDummyMgr()
	dummyMgr.gasMgr = NewGasMgrV1(constAccessor, k)
	handler := NewNativeTxHandler(k, dummyMgr)

	addr := GetRandomBech32Addr()

	coins := common.Coins{
		common.NewCoin(common.RuneNative, cosmos.NewUint(200*common.One)),
	}

	funds, err := common.NewCoin(common.RuneNative, cosmos.NewUint(300*common.One)).Native()
	c.Assert(err, IsNil)
	_, err = banker.AddCoins(ctx, addr, cosmos.NewCoins(funds))
	c.Assert(err, IsNil)
	pool := NewPool()
	pool.Asset = common.BNBAsset
	pool.BalanceAsset = cosmos.NewUint(100 * common.One)
	pool.BalanceRune = cosmos.NewUint(100 * common.One)
	pool.Status = PoolAvailable
	c.Assert(k.SetPool(ctx, pool), IsNil)
	msg := NewMsgNativeTx(coins, "ADD:BNB.BNB", addr)

	_, err = handler.handle(ctx, msg, constants.SWVersion, constAccessor)
	c.Assert(err, IsNil)
	// ensure observe tx had been saved
	hash := tmtypes.Tx(ctx.TxBytes()).Hash()
	txID, err := common.NewTxID(fmt.Sprintf("%X", hash))
	c.Assert(err, IsNil)
	voter, err := k.GetObservedTxInVoter(ctx, txID)
	c.Assert(err, IsNil)
	c.Assert(voter.Tx.IsEmpty(), Equals, false)
	c.Assert(voter.Tx.Status, Equals, types.Done)
}

type HandlerNativeTxTestHelper struct {
	keeper.Keeper
}

func NewHandlerNativeTxTestHelper(k keeper.Keeper) *HandlerNativeTxTestHelper {
	return &HandlerNativeTxTestHelper{
		Keeper: k,
	}
}

func (s *HandlerNativeTxSuite) TestDifferentValidation(c *C) {
	acctAddr := GetRandomBech32Addr()
	testCases := []struct {
		name            string
		messageProvider func(c *C, ctx cosmos.Context, helper *HandlerNativeTxTestHelper) cosmos.Msg
		validator       func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *HandlerNativeTxTestHelper, name string)
	}{
		{
			name: "invalid message should result an error",
			messageProvider: func(c *C, ctx cosmos.Context, helper *HandlerNativeTxTestHelper) cosmos.Msg {
				return NewMsgNetworkFee(ctx.BlockHeight(), common.BNBChain, 1, bnbSingleTxFee.Uint64(), GetRandomBech32Addr())
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *HandlerNativeTxTestHelper, name string) {
				c.Check(err, NotNil, Commentf(name))
				c.Check(result, IsNil, Commentf(name))
				c.Check(errors.Is(err, errInvalidMessage), Equals, true, Commentf(name))
			},
		},
		{
			name: "coin is not on THORChain should result in an error",
			messageProvider: func(c *C, ctx cosmos.Context, helper *HandlerNativeTxTestHelper) cosmos.Msg {
				return NewMsgNativeTx(common.Coins{
					common.NewCoin(common.BNBAsset, cosmos.NewUint(100)),
				}, "hello", GetRandomBech32Addr())
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *HandlerNativeTxTestHelper, name string) {
				c.Check(err, NotNil, Commentf(name))
				c.Check(result, IsNil, Commentf(name))
			},
		},
		{
			name: "Insufficient funds should result in an error",
			messageProvider: func(c *C, ctx cosmos.Context, helper *HandlerNativeTxTestHelper) cosmos.Msg {
				return NewMsgNativeTx(common.Coins{
					common.NewCoin(common.RuneNative, cosmos.NewUint(100)),
				}, "hello", GetRandomBech32Addr())
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *HandlerNativeTxTestHelper, name string) {
				c.Check(err, NotNil, Commentf(name))
				c.Check(result, IsNil, Commentf(name))
				c.Check(errors.Is(err, se.ErrInsufficientFunds), Equals, true, Commentf(name))
			},
		},
		{
			name: "invalid memo should refund",
			messageProvider: func(c *C, ctx cosmos.Context, helper *HandlerNativeTxTestHelper) cosmos.Msg {
				FundAccount(c, ctx, helper.Keeper, acctAddr, 100)
				vault := NewVault(ctx.BlockHeight(), ActiveVault, AsgardVault, GetRandomPubKey(), common.Chains{common.BNBChain, common.THORChain})
				c.Check(helper.Keeper.SetVault(ctx, vault), IsNil)
				return NewMsgNativeTx(common.Coins{
					common.NewCoin(common.RuneNative, cosmos.NewUint(2*common.One)),
				}, "hello", acctAddr)
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *HandlerNativeTxTestHelper, name string) {
				c.Check(err, IsNil, Commentf(name))
				c.Check(result, NotNil, Commentf(name))
				banker := helper.Keeper.CoinKeeper()
				coins := common.NewCoin(common.RuneNative, cosmos.NewUint(98*common.One))
				coin, err := coins.Native()
				c.Check(err, IsNil)
				hasCoin := banker.HasCoins(ctx, acctAddr, cosmos.NewCoins().Add(coin))
				c.Check(hasCoin, Equals, true)
			},
		},
	}
	for _, tc := range testCases {
		ctx, k := setupKeeperForTest(c)
		helper := NewHandlerNativeTxTestHelper(k)
		mgr := NewManagers(helper)
		mgr.BeginBlock(ctx)
		handler := NewNativeTxHandler(helper, mgr)
		msg := tc.messageProvider(c, ctx, helper)
		constantAccessor := constants.GetConstantValues(constants.SWVersion)
		result, err := handler.Run(ctx, msg, semver.MustParse("0.1.0"), constantAccessor)
		tc.validator(c, ctx, result, err, helper, tc.name)
	}
}
