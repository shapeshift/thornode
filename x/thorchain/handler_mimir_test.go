package thorchain

import (
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

type HandlerMimirSuite struct{}

var _ = Suite(&HandlerMimirSuite{})

func (s *HandlerMimirSuite) SetUpSuite(c *C) {
	SetupConfigForTest()
}

func (s *HandlerMimirSuite) TestValidate(c *C) {
	ctx, keeper := setupKeeperForTest(c)

	addr, _ := cosmos.AccAddressFromBech32(ADMINS[0])
	handler := NewMimirHandler(NewDummyMgrWithKeeper(keeper))
	// happy path
	msg := NewMsgMimir("foo", 44, addr)
	err := handler.validate(ctx, *msg)
	c.Assert(err, IsNil)

	// invalid msg
	msg = &MsgMimir{}
	err = handler.validate(ctx, *msg)
	c.Assert(err, NotNil)
}

func (s *HandlerMimirSuite) TestHandle(c *C) {
	ctx, keeper := setupKeeperForTest(c)

	handler := NewMimirHandler(NewDummyMgrWithKeeper(keeper))

	msg := NewMsgMimir("foo", 55, GetRandomBech32Addr())
	sdkErr := handler.handle(ctx, *msg)
	c.Assert(sdkErr, IsNil)
	val, err := keeper.GetMimir(ctx, "foo")
	c.Assert(err, IsNil)
	c.Check(val, Equals, int64(55))

	invalidMsg := NewMsgNetworkFee(ctx.BlockHeight(), common.BNBChain, 1, bnbSingleTxFee.Uint64(), GetRandomBech32Addr())
	result, err := handler.Run(ctx, invalidMsg)
	c.Check(err, NotNil)
	c.Check(result, IsNil)

	result, err = handler.Run(ctx, msg)
	c.Assert(err, NotNil)
	c.Assert(result, IsNil)

	addr, err := cosmos.AccAddressFromBech32(ADMINS[0])
	c.Check(err, IsNil)
	msg1 := NewMsgMimir("hello", 1, addr)
	result, err = handler.Run(ctx, msg1)
	c.Check(err, IsNil)
	c.Check(result, NotNil)

	val, err = keeper.GetMimir(ctx, "hello")
	c.Assert(err, IsNil)
	c.Assert(val, Equals, int64(1))

	// delete mimir
	msg1 = NewMsgMimir("hello", -3, addr)
	result, err = handler.Run(ctx, msg1)
	c.Check(err, IsNil)
	c.Check(result, NotNil)
	val, err = keeper.GetMimir(ctx, "hello")
	c.Assert(err, IsNil)
	c.Assert(val, Equals, int64(-1))
}
