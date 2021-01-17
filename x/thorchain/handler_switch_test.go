package thorchain

import (
	"github.com/blang/semver"
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

var _ = Suite(&HandlerSwitchSuite{})

type HandlerSwitchSuite struct{}

func (s *HandlerSwitchSuite) SetUpSuite(c *C) {
	SetupConfigForTest()
}

func (s *HandlerSwitchSuite) TestValidate(c *C) {
	ctx, k := setupKeeperForTest(c)

	na := GetRandomNodeAccount(NodeActive)
	c.Assert(k.SetNodeAccount(ctx, na), IsNil)
	tx := GetRandomTx()
	tx.Coins = common.Coins{
		common.NewCoin(common.Rune67CAsset, cosmos.NewUint(100*common.One)),
	}
	destination := GetRandomBNBAddress()

	handler := NewSwitchHandler(k, NewDummyMgr())

	constantAccessor := constants.GetConstantValues(constants.SWVersion)
	destination = GetRandomTHORAddress()
	// happy path
	msg := NewMsgSwitch(tx, destination, na.NodeAddress)
	result, err := handler.Run(ctx, msg, constants.SWVersion, constantAccessor)
	c.Assert(err, IsNil)
	c.Assert(result, NotNil)

	// invalid version
	result, err = handler.Run(ctx, msg, semver.Version{}, constantAccessor)
	c.Assert(err, Equals, errBadVersion)
	c.Assert(result, IsNil)

	// invalid msg
	msg = &MsgSwitch{}
	result, err = handler.Run(ctx, msg, constants.SWVersion, constantAccessor)
	c.Assert(err, NotNil)
	c.Assert(result, IsNil)
}

func (s *HandlerSwitchSuite) TestGettingNativeTokens(c *C) {
	ctx, k := setupKeeperForTest(c)

	na := GetRandomNodeAccount(NodeActive)
	c.Assert(k.SetNodeAccount(ctx, na), IsNil)
	tx := GetRandomTx()
	tx.Coins = common.Coins{
		common.NewCoin(common.Rune67CAsset, cosmos.NewUint(100*common.One)),
	}
	destination := GetRandomTHORAddress()

	handler := NewSwitchHandler(k, NewDummyMgr())

	msg := NewMsgSwitch(tx, destination, na.NodeAddress)
	constAccessor := constants.GetConstantValues(constants.SWVersion)
	_, err := handler.handle(ctx, *msg, constants.SWVersion, constAccessor)
	c.Assert(err, IsNil)
	coin, err := common.NewCoin(common.RuneNative, cosmos.NewUint(100*common.One)).Native()
	c.Assert(err, IsNil)
	addr, err := cosmos.AccAddressFromBech32(destination.String())
	c.Assert(err, IsNil)
	c.Check(k.HasCoins(ctx, addr, cosmos.NewCoins(coin)), Equals, true, Commentf("%s", k.GetBalance(ctx, addr)))

	// check that we can add more an account
	_, err = handler.handle(ctx, *msg, constants.SWVersion, constAccessor)
	c.Assert(err, IsNil)
	coin, err = common.NewCoin(common.RuneNative, cosmos.NewUint(200*common.One)).Native()
	c.Assert(err, IsNil)
	c.Check(k.HasCoins(ctx, addr, cosmos.NewCoins(coin)), Equals, true)
}

type HandlerSwitchTestHelper struct {
	keeper.Keeper
}

func NewHandlerSwitchTestHelper(k keeper.Keeper) *HandlerSwitchTestHelper {
	return &HandlerSwitchTestHelper{
		Keeper: k,
	}
}

func (s *HandlerSwitchSuite) getAValidSwitchMsg(ctx cosmos.Context, helper *HandlerSwitchTestHelper) *MsgSwitch {
	na := GetRandomNodeAccount(NodeActive)
	from := GetRandomBNBAddress()
	tx := GetRandomTx()
	tx.FromAddress = common.Address(from.String())
	tx.Coins = common.Coins{
		common.NewCoin(common.BEP2RuneAsset(), cosmos.NewUint(100*common.One)),
	}
	destination := GetRandomBech32Addr()
	helper.Keeper.SetNodeAccount(ctx, na)
	coin, _ := common.NewCoin(common.RuneNative, cosmos.NewUint(800*common.One)).Native()
	helper.Keeper.AddCoins(ctx, destination, cosmos.NewCoins(coin))
	vault := GetRandomVault()
	vault.Type = AsgardVault
	vault.Status = ActiveVault
	vault.AddFunds(common.Coins{
		common.NewCoin(common.BEP2RuneAsset(), cosmos.NewUint(100*common.One)),
	})
	helper.Keeper.SetVault(ctx, vault)
	return NewMsgSwitch(tx, common.Address(destination.String()), na.NodeAddress)
}

func (s *HandlerSwitchSuite) TestSwitchHandlerDifferentValidations(c *C) {
	testCases := []struct {
		name            string
		messageProvider func(c *C, ctx cosmos.Context, helper *HandlerSwitchTestHelper) cosmos.Msg
		validator       func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *HandlerSwitchTestHelper, name string)
		nativeRune      bool
	}{
		{
			name: "invalid msg should return an error",
			messageProvider: func(c *C, ctx cosmos.Context, helper *HandlerSwitchTestHelper) cosmos.Msg {
				return NewMsgMimir("what", 1, GetRandomBech32Addr())
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *HandlerSwitchTestHelper, name string) {
				c.Check(err, NotNil, Commentf(name))
				c.Check(result, IsNil, Commentf(name))
			},
		},
		{
			name: "Not enough RUNE to pay for fees should not fail",
			messageProvider: func(c *C, ctx cosmos.Context, helper *HandlerSwitchTestHelper) cosmos.Msg {
				m := s.getAValidSwitchMsg(ctx, helper)
				m.Tx.Coins[0].Amount = cosmos.NewUint(common.One).QuoUint64(2)
				return m
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *HandlerSwitchTestHelper, name string) {
				c.Check(err, IsNil, Commentf(name))
				c.Check(result, NotNil, Commentf(name))
			},
			nativeRune: true,
		},
	}

	for _, tc := range testCases {
		if len(common.RuneAsset().Native()) == 0 && tc.nativeRune {
			continue
		}

		ctx, k := setupKeeperForTest(c)
		helper := NewHandlerSwitchTestHelper(k)
		mgr := NewManagers(helper)
		mgr.BeginBlock(ctx)
		handler := NewSwitchHandler(helper, mgr)
		msg := tc.messageProvider(c, ctx, helper)
		constantAccessor := constants.GetConstantValues(constants.SWVersion)
		result, err := handler.Run(ctx, msg, semver.MustParse("0.1.0"), constantAccessor)
		tc.validator(c, ctx, result, err, helper, tc.name)
	}
}
