package thorchain

import (
	"github.com/blang/semver"

	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"

	. "gopkg.in/check.v1"
)

type HandlerSendSuite struct{}

var _ = Suite(&HandlerSendSuite{})

func (s *HandlerSendSuite) TestValidate(c *C) {
	ctx, k := setupKeeperForTest(c)

	addr1 := GetRandomBech32Addr()
	addr2 := GetRandomBech32Addr()

	msg := MsgSend{
		FromAddress: addr1,
		ToAddress:   addr2,
		Amount:      cosmos.NewCoins(cosmos.NewCoin("dummy", cosmos.NewInt(12))),
	}
	handler := NewSendHandler(k, NewDummyMgr())
	err := handler.validate(ctx, msg, constants.SWVersion)
	c.Assert(err, IsNil)

	// invalid version
	err = handler.validate(ctx, msg, semver.Version{})
	c.Assert(err, Equals, errInvalidVersion)

	// invalid msg
	msg = MsgSend{}
	err = handler.validate(ctx, msg, constants.SWVersion)
	c.Assert(err, NotNil)
}

func (s *HandlerSendSuite) TestHandle(c *C) {
	ctx, k := setupKeeperForTest(c)
	banker := k.CoinKeeper()
	constAccessor := constants.GetConstantValues(constants.SWVersion)

	addr1 := GetRandomBech32Addr()
	addr2 := GetRandomBech32Addr()

	funds, err := common.NewCoin(common.RuneNative, cosmos.NewUint(200*common.One)).Native()
	c.Assert(err, IsNil)
	_, err = banker.AddCoins(ctx, addr1, cosmos.NewCoins(funds))
	c.Assert(err, IsNil)

	coin, err := common.NewCoin(common.RuneNative, cosmos.NewUint(12*common.One)).Native()
	c.Assert(err, IsNil)
	msg := MsgSend{
		FromAddress: addr1,
		ToAddress:   addr2,
		Amount:      cosmos.NewCoins(coin),
	}

	handler := NewSendHandler(k, NewDummyMgr())
	result := handler.handle(ctx, msg, constants.SWVersion, constAccessor)
	c.Assert(result.IsOK(), Equals, true, Commentf("%+v", result.Log))

	// insufficient funds
	coin, err = common.NewCoin(common.RuneNative, cosmos.NewUint(3000*common.One)).Native()
	c.Assert(err, IsNil)
	msg = MsgSend{
		FromAddress: addr1,
		ToAddress:   addr2,
		Amount:      cosmos.NewCoins(coin),
	}
	result = handler.handle(ctx, msg, constants.SWVersion, constAccessor)
	c.Assert(result.IsOK(), Equals, false, Commentf("%+v", result.Log))
}
