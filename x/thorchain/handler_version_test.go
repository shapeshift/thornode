package thorchain

import (
	"errors"

	"github.com/blang/semver"
	se "github.com/cosmos/cosmos-sdk/types/errors"
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

type HandlerVersionSuite struct{}

type TestVersionlKeeper struct {
	keeper.KVStoreDummy
	na                  NodeAccount
	failNodeAccount     NodeAccount
	emptyNodeAccount    NodeAccount
	failSaveNodeAccount bool
	failGetNetwork      bool
	failSetNetwork      bool
}

func (k *TestVersionlKeeper) SendFromAccountToModule(ctx cosmos.Context, from cosmos.AccAddress, to string, coins common.Coins) error {
	return nil
}

func (k *TestVersionlKeeper) GetNodeAccount(_ cosmos.Context, addr cosmos.AccAddress) (NodeAccount, error) {
	if k.failNodeAccount.NodeAddress.Equals(addr) {
		return NodeAccount{}, kaboom
	}
	if k.emptyNodeAccount.NodeAddress.Equals(addr) {
		return NodeAccount{}, nil
	}
	return k.na, nil
}

func (k *TestVersionlKeeper) SetNodeAccount(_ cosmos.Context, na NodeAccount) error {
	if k.failSaveNodeAccount {
		return kaboom
	}
	k.na = na
	return nil
}

func (k *TestVersionlKeeper) GetNetwork(ctx cosmos.Context) (Network, error) {
	if k.failGetNetwork {
		return NewNetwork(), kaboom
	}
	return NewNetwork(), nil
}

func (k *TestVersionlKeeper) SetNetwork(ctx cosmos.Context, data Network) error {
	if k.failSetNetwork {
		return kaboom
	}
	return nil
}

var _ = Suite(&HandlerVersionSuite{})

func (s *HandlerVersionSuite) TestValidate(c *C) {
	ctx, _ := setupKeeperForTest(c)

	keeper := &TestVersionlKeeper{
		na:               GetRandomNodeAccount(NodeActive),
		failNodeAccount:  GetRandomNodeAccount(NodeActive),
		emptyNodeAccount: GetRandomNodeAccount(NodeStandby),
	}

	handler := NewVersionHandler(NewDummyMgrWithKeeper(keeper))
	// happy path
	ver := GetCurrentVersion()
	constAccessor := constants.GetConstantValues(ver)
	msg := NewMsgSetVersion(ver.String(), keeper.na.NodeAddress)
	result, err := handler.Run(ctx, msg, ver, constAccessor)
	c.Assert(err, IsNil)
	c.Assert(result, NotNil)

	// invalid version
	result, err = handler.Run(ctx, msg, semver.Version{}, constAccessor)
	c.Assert(err, Equals, errBadVersion)
	c.Assert(result, IsNil)

	// invalid msg
	msg = &MsgSetVersion{}
	result, err = handler.Run(ctx, msg, ver, constAccessor)
	c.Assert(result, IsNil)
	c.Assert(err, NotNil)

	// fail to get node account should fail
	msg1 := NewMsgSetVersion(ver.String(), keeper.failNodeAccount.NodeAddress)
	result, err = handler.Run(ctx, msg1, ver, constAccessor)
	c.Assert(err, NotNil)
	c.Assert(result, IsNil)

	// node account empty should fail
	msg2 := NewMsgSetVersion(ver.String(), keeper.emptyNodeAccount.NodeAddress)
	result, err = handler.Run(ctx, msg2, ver, constAccessor)
	c.Assert(err, NotNil)
	c.Assert(result, IsNil)
	c.Assert(errors.Is(err, se.ErrUnauthorized), Equals, true)
}

func (s *HandlerVersionSuite) TestHandle(c *C) {
	ctx, _ := setupKeeperForTest(c)
	ver := GetCurrentVersion()
	constAccessor := constants.GetConstantValues(ver)

	keeper := &TestVersionlKeeper{
		na: GetRandomNodeAccount(NodeActive),
	}

	handler := NewVersionHandler(NewDummyMgrWithKeeper(keeper))

	msg := NewMsgSetVersion("2.0.0", GetRandomBech32Addr())
	err := handler.handle(ctx, *msg, ver, constAccessor)
	c.Assert(err, IsNil)
	c.Check(keeper.na.Version, Equals, "2.0.0")

	// fail to set node account should return an error
	keeper.failSaveNodeAccount = true
	result, err := handler.Run(ctx, msg, ver, constAccessor)
	c.Assert(err, NotNil)
	c.Assert(result, IsNil)
	keeper.failSaveNodeAccount = false

	if !common.RuneAsset().Equals(common.RuneNative) {
		// BEP2 RUNE
		keeper.failGetNetwork = true
		result, err = handler.Run(ctx, msg, ver, constAccessor)
		c.Assert(err, NotNil)
		c.Assert(result, IsNil)
		keeper.failGetNetwork = false
		keeper.failSetNetwork = true
		result, err = handler.Run(ctx, msg, ver, constAccessor)
		c.Assert(err, NotNil)
		c.Assert(result, IsNil)
		keeper.failSetNetwork = false
	}
}
