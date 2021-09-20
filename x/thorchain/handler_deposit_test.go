package thorchain

import (
	"errors"
	"fmt"
	"strconv"

	se "github.com/cosmos/cosmos-sdk/types/errors"
	tmtypes "github.com/tendermint/tendermint/types"
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
	"gitlab.com/thorchain/thornode/x/thorchain/types"
)

type HandlerDepositSuite struct{}

var _ = Suite(&HandlerDepositSuite{})

func (s *HandlerDepositSuite) TestValidate(c *C) {
	ctx, k := setupKeeperForTest(c)

	addr := GetRandomBech32Addr()

	coins := common.Coins{
		common.NewCoin(common.RuneNative, cosmos.NewUint(200*common.One)),
	}
	msg := NewMsgDeposit(coins, fmt.Sprintf("ADD:BNB.BNB:%s", GetRandomRUNEAddress()), addr)

	handler := NewDepositHandler(NewDummyMgrWithKeeper(k))
	err := handler.validate(ctx, *msg)
	c.Assert(err, IsNil)

	// invalid msg
	msg = &MsgDeposit{}
	err = handler.validate(ctx, *msg)
	c.Assert(err, NotNil)
}

func (s *HandlerDepositSuite) TestHandle(c *C) {
	ctx, k := setupKeeperForTest(c)
	constAccessor := constants.NewDummyConstants(map[constants.ConstantName]int64{
		constants.NativeTransactionFee: 1000_000,
	}, map[constants.ConstantName]bool{}, map[constants.ConstantName]string{})
	activeNode := GetRandomValidatorNode(NodeActive)
	k.SetNodeAccount(ctx, activeNode)
	dummyMgr := NewDummyMgrWithKeeper(k)
	dummyMgr.gasMgr = newGasMgrV1(constAccessor, k)
	handler := NewDepositHandler(dummyMgr)

	addr := GetRandomBech32Addr()

	coins := common.Coins{
		common.NewCoin(common.RuneNative, cosmos.NewUint(200*common.One)),
	}

	funds, err := common.NewCoin(common.RuneNative, cosmos.NewUint(300*common.One)).Native()
	c.Assert(err, IsNil)
	err = k.AddCoins(ctx, addr, cosmos.NewCoins(funds))
	c.Assert(err, IsNil)
	pool := NewPool()
	pool.Asset = common.BNBAsset
	pool.BalanceAsset = cosmos.NewUint(100 * common.One)
	pool.BalanceRune = cosmos.NewUint(100 * common.One)
	pool.Status = PoolAvailable
	c.Assert(k.SetPool(ctx, pool), IsNil)
	msg := NewMsgDeposit(coins, "ADD:BNB.BNB", addr)

	_, err = handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	// ensure observe tx had been saved
	hash := tmtypes.Tx(ctx.TxBytes()).Hash()
	txID, err := common.NewTxID(fmt.Sprintf("%X", hash))
	c.Assert(err, IsNil)
	voter, err := k.GetObservedTxInVoter(ctx, txID)
	c.Assert(err, IsNil)
	c.Assert(voter.Tx.IsEmpty(), Equals, false)
	c.Assert(voter.Tx.Status, Equals, types.Status_done)
}

type HandlerDepositTestHelper struct {
	keeper.Keeper
}

func NewHandlerDepositTestHelper(k keeper.Keeper) *HandlerDepositTestHelper {
	return &HandlerDepositTestHelper{
		Keeper: k,
	}
}

func (s *HandlerDepositSuite) TestDifferentValidation(c *C) {
	acctAddr := GetRandomBech32Addr()
	testCases := []struct {
		name            string
		messageProvider func(c *C, ctx cosmos.Context, helper *HandlerDepositTestHelper) cosmos.Msg
		validator       func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *HandlerDepositTestHelper, name string)
	}{
		{
			name: "invalid message should result an error",
			messageProvider: func(c *C, ctx cosmos.Context, helper *HandlerDepositTestHelper) cosmos.Msg {
				return NewMsgNetworkFee(ctx.BlockHeight(), common.BNBChain, 1, bnbSingleTxFee.Uint64(), GetRandomBech32Addr())
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *HandlerDepositTestHelper, name string) {
				c.Check(err, NotNil, Commentf(name))
				c.Check(result, IsNil, Commentf(name))
				c.Check(errors.Is(err, errInvalidMessage), Equals, true, Commentf(name))
			},
		},
		{
			name: "coin is not on THORChain should result in an error",
			messageProvider: func(c *C, ctx cosmos.Context, helper *HandlerDepositTestHelper) cosmos.Msg {
				return NewMsgDeposit(common.Coins{
					common.NewCoin(common.BNBAsset, cosmos.NewUint(100)),
				}, "hello", GetRandomBech32Addr())
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *HandlerDepositTestHelper, name string) {
				c.Check(err, NotNil, Commentf(name))
				c.Check(result, IsNil, Commentf(name))
			},
		},
		{
			name: "Insufficient funds should result in an error",
			messageProvider: func(c *C, ctx cosmos.Context, helper *HandlerDepositTestHelper) cosmos.Msg {
				return NewMsgDeposit(common.Coins{
					common.NewCoin(common.RuneNative, cosmos.NewUint(100)),
				}, "hello", GetRandomBech32Addr())
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *HandlerDepositTestHelper, name string) {
				c.Check(err, NotNil, Commentf(name))
				c.Check(result, IsNil, Commentf(name))
				c.Check(errors.Is(err, se.ErrInsufficientFunds), Equals, true, Commentf(name))
			},
		},
		{
			name: "invalid memo should refund",
			messageProvider: func(c *C, ctx cosmos.Context, helper *HandlerDepositTestHelper) cosmos.Msg {
				FundAccount(c, ctx, helper.Keeper, acctAddr, 100)
				vault := NewVault(ctx.BlockHeight(), ActiveVault, AsgardVault, GetRandomPubKey(), common.Chains{common.BNBChain, common.THORChain}.Strings(), []ChainContract{})
				c.Check(helper.Keeper.SetVault(ctx, vault), IsNil)
				return NewMsgDeposit(common.Coins{
					common.NewCoin(common.RuneNative, cosmos.NewUint(2*common.One)),
				}, "hello", acctAddr)
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *HandlerDepositTestHelper, name string) {
				c.Check(err, IsNil, Commentf(name))
				c.Check(result, NotNil, Commentf(name))
				coins := common.NewCoin(common.RuneNative, cosmos.NewUint(98*common.One))
				coin, err := coins.Native()
				c.Check(err, IsNil)
				hasCoin := helper.Keeper.HasCoins(ctx, acctAddr, cosmos.NewCoins().Add(coin))
				c.Check(hasCoin, Equals, true)
			},
		},
	}
	for _, tc := range testCases {
		ctx, mgr := setupManagerForTest(c)
		helper := NewHandlerDepositTestHelper(mgr.Keeper())
		mgr.K = helper
		handler := NewDepositHandler(mgr)
		msg := tc.messageProvider(c, ctx, helper)
		result, err := handler.Run(ctx, msg)
		tc.validator(c, ctx, result, err, helper, tc.name)
	}
}

func (s *HandlerDepositSuite) TestAddSwapV64(c *C) {
	SetupConfigForTest()
	ctx, mgr := setupManagerForTest(c)
	handler := NewDepositHandler(mgr)
	tx := common.NewTx(
		GetRandomTxHash(),
		GetRandomTHORAddress(),
		GetRandomTHORAddress(),
		common.Coins{common.NewCoin(common.RuneNative, cosmos.NewUint(common.One))},
		common.Gas{
			{Asset: common.BNBAsset, Amount: cosmos.NewUint(37500)},
		},
		"",
	)
	// no affiliate fee
	msg := NewMsgSwap(tx, common.BTCAsset, GetRandomBTCAddress(), cosmos.ZeroUint(), common.NoAddress, cosmos.ZeroUint(), GetRandomBech32Addr())
	handler.addSwapV65(ctx, *msg)
	swap, err := mgr.Keeper().GetSwapQueueItem(ctx, tx.ID, 0)
	c.Assert(err, IsNil)
	c.Assert(swap.String(), Equals, msg.String())

	// affiliate fee, with more than 10K as basis points
	msg1 := NewMsgSwap(tx, common.BTCAsset, GetRandomBTCAddress(), cosmos.ZeroUint(), GetRandomTHORAddress(), cosmos.NewUint(20000), GetRandomBech32Addr())
	handler.addSwapV65(ctx, *msg1)
	swap, err = mgr.Keeper().GetSwapQueueItem(ctx, tx.ID, 0)
	c.Assert(err, IsNil)
	c.Assert(swap.Tx.Coins[0].Amount.IsZero(), Equals, true)
	affiliateFeeAddr, err := msg1.GetAffiliateAddress().AccAddress()
	c.Assert(err, IsNil)
	acct := mgr.Keeper().GetBalance(ctx, affiliateFeeAddr)
	c.Assert(acct.AmountOf(common.RuneNative.Native()).String(), Equals, strconv.FormatInt(common.One, 10))

	// normal affiliate fee
	tx.Coins[0].Amount = cosmos.NewUint(common.One)
	msg2 := NewMsgSwap(tx, common.BTCAsset, GetRandomBTCAddress(), cosmos.ZeroUint(), GetRandomTHORAddress(), cosmos.NewUint(1000), GetRandomBech32Addr())
	handler.addSwapV65(ctx, *msg2)
	swap, err = mgr.Keeper().GetSwapQueueItem(ctx, tx.ID, 0)
	c.Assert(err, IsNil)
	c.Assert(swap.Tx.Coins[0].Amount.IsZero(), Equals, false)
	c.Assert(swap.Tx.Coins[0].Amount.Equal(cosmos.NewUint(common.One/10*9)), Equals, true)
	affiliateFeeAddr2, err := msg2.GetAffiliateAddress().AccAddress()
	c.Assert(err, IsNil)
	acct2 := mgr.Keeper().GetBalance(ctx, affiliateFeeAddr2)
	c.Assert(acct2.AmountOf(common.RuneNative.Native()).String(), Equals, strconv.FormatInt(common.One/10, 10))

	// NONE RUNE , synth asset should be handled correctly

	synthAsset, err := common.NewAsset("BTC/BTC")
	c.Assert(err, IsNil)
	tx1 := common.NewTx(
		GetRandomTxHash(),
		GetRandomTHORAddress(),
		GetRandomTHORAddress(),
		common.Coins{common.NewCoin(synthAsset, cosmos.NewUint(common.One))},
		common.Gas{
			{Asset: common.RuneNative, Amount: cosmos.NewUint(200000)},
		},
		"",
	)

	mgr.Keeper().MintToModule(ctx, ModuleName, tx1.Coins[0])
	mgr.Keeper().SendFromModuleToModule(ctx, ModuleName, AsgardName, tx1.Coins)
	msg3 := NewMsgSwap(tx1, common.BTCAsset, GetRandomBTCAddress(), cosmos.ZeroUint(), GetRandomTHORAddress(), cosmos.NewUint(1000), GetRandomBech32Addr())
	handler.addSwapV65(ctx, *msg3)
	swap, err = mgr.Keeper().GetSwapQueueItem(ctx, tx1.ID, 0)
	c.Assert(err, IsNil)
	c.Assert(swap.Tx.Coins[0].Amount.IsZero(), Equals, false)
	c.Assert(swap.Tx.Coins[0].Amount.Equal(cosmos.NewUint(common.One/10*9)), Equals, true)
	affiliateFeeAddr3, err := msg3.GetAffiliateAddress().AccAddress()
	c.Assert(err, IsNil)
	acct3 := mgr.Keeper().GetBalance(ctx, affiliateFeeAddr3)
	c.Assert(acct3.AmountOf(synthAsset.Native()).String(), Equals, strconv.FormatInt(common.One/10, 10))
}
