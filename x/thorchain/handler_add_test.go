package thorchain

import (
	"github.com/blang/semver"
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

type HandlerAddSuite struct{}

var _ = Suite(&HandlerAddSuite{})

func (HandlerAddSuite) TestAdd(c *C) {
	w := getHandlerTestWrapper(c, 1, true, true)
	// happy path
	prePool, err := w.keeper.GetPool(w.ctx, common.BNBAsset)
	c.Assert(err, IsNil)
	addHandler := NewAddHandler(w.keeper, NewVersionedEventMgr())
	msg := NewMsgAdd(GetRandomTx(), common.BNBAsset, cosmos.NewUint(common.One*5), cosmos.NewUint(common.One*5), w.activeNodeAccount.NodeAddress)
	ver := constants.SWVersion
	constAccessor := constants.GetConstantValues(ver)
	result := addHandler.Run(w.ctx, msg, ver, constAccessor)
	c.Assert(result.Code, Equals, cosmos.CodeOK)
	afterPool, err := w.keeper.GetPool(w.ctx, common.BNBAsset)
	c.Assert(err, IsNil)
	c.Assert(afterPool.BalanceRune.String(), Equals, prePool.BalanceRune.Add(msg.RuneAmount).String())
	c.Assert(afterPool.BalanceAsset.String(), Equals, prePool.BalanceAsset.Add(msg.AssetAmount).String())

	// invalid version
	ver = semver.Version{}
	result = addHandler.Run(w.ctx, msg, ver, constAccessor)
	c.Assert(result.Code, Equals, CodeBadVersion)
}

func (HandlerAddSuite) TestHandleMsgAddValidation(c *C) {
	w := getHandlerTestWrapper(c, 1, true, false)
	testCases := []struct {
		name         string
		msg          MsgAdd
		expectedCode cosmos.CodeType
	}{
		{
			name:         "invalid signer address should fail",
			msg:          NewMsgAdd(GetRandomTx(), common.BNBAsset, cosmos.NewUint(common.One*5), cosmos.NewUint(common.One*5), cosmos.AccAddress{}),
			expectedCode: cosmos.CodeInvalidAddress,
		},
		{
			name:         "empty asset should fail",
			msg:          NewMsgAdd(GetRandomTx(), common.Asset{}, cosmos.NewUint(common.One*5), cosmos.NewUint(common.One*5), w.activeNodeAccount.NodeAddress),
			expectedCode: cosmos.CodeUnknownRequest,
		},
		{
			name:         "pool doesn't exist should fail",
			msg:          NewMsgAdd(GetRandomTx(), common.BNBAsset, cosmos.NewUint(common.One*5), cosmos.NewUint(common.One*5), w.activeNodeAccount.NodeAddress),
			expectedCode: cosmos.CodeUnknownRequest,
		},
	}

	addHandler := NewAddHandler(w.keeper, NewVersionedEventMgr())
	ver := constants.SWVersion
	cosntAccessor := constants.GetConstantValues(ver)
	for _, item := range testCases {
		c.Assert(addHandler.Run(w.ctx, item.msg, ver, cosntAccessor).Code, Equals, item.expectedCode)
	}
}
