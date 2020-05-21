package thorchain

import (
	"fmt"

	"github.com/blang/semver"

	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"

	. "gopkg.in/check.v1"
)

type HandlerNativeTxSuite struct{}

var _ = Suite(&HandlerNativeTxSuite{})

func (s *HandlerNativeTxSuite) TestValidate(c *C) {
	ctx, k := setupKeeperForTest(c)

	addr := GetRandomBech32Addr()

	coins := common.Coins{
		common.NewCoin(common.RuneNative, cosmos.NewUint(200*common.One)),
	}
	msg := NewMsgNativeTx(coins, fmt.Sprintf("STAKE:BNB.BNB:%s", GetRandomRUNEAddress()), addr)

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
	constAccessor := constants.GetConstantValues(constants.SWVersion)

	handler := NewNativeTxHandler(k, NewDummyMgr())

	addr := GetRandomBech32Addr()

	coins := common.Coins{
		common.NewCoin(common.RuneNative, cosmos.NewUint(200*common.One)),
	}

	funds, err := common.NewCoin(common.RuneNative, cosmos.NewUint(300*common.One)).Native()
	c.Assert(err, IsNil)
	_, err = banker.AddCoins(ctx, addr, cosmos.NewCoins(funds))
	c.Assert(err, IsNil)

	msg := NewMsgNativeTx(coins, "ADD:BNB.BNB", addr)

	_, err = handler.handle(ctx, msg, constants.SWVersion, constAccessor)
	c.Assert(err, IsNil)
}
