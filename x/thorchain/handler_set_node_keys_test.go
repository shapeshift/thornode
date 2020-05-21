package thorchain

import (
	"fmt"

	"github.com/blang/semver"
	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	. "gopkg.in/check.v1"
)

type HandlerSetNodeKeysSuite struct{}

type TestSetNodeKeysKeeper struct {
	KVStoreDummy
	na     NodeAccount
	ensure error
}

func (k *TestSetNodeKeysKeeper) GetNodeAccount(ctx cosmos.Context, signer cosmos.AccAddress) (NodeAccount, error) {
	return k.na, nil
}

func (k *TestSetNodeKeysKeeper) EnsureNodeKeysUnique(_ cosmos.Context, _ string, _ common.PubKeySet) error {
	return k.ensure
}

var _ = Suite(&HandlerSetNodeKeysSuite{})

func (s *HandlerSetNodeKeysSuite) TestValidate(c *C) {
	ctx, _ := setupKeeperForTest(c)

	keeper := &TestSetNodeKeysKeeper{
		na:     GetRandomNodeAccount(NodeStandby),
		ensure: nil,
	}

	handler := NewSetNodeKeysHandler(keeper, NewDummyMgr())

	// happy path
	ver := constants.SWVersion
	signer := GetRandomBech32Addr()
	c.Assert(signer.Empty(), Equals, false)
	consensPubKey := GetRandomBech32ConsensusPubKey()
	pubKeys := GetRandomPubKeySet()

	msg := NewMsgSetNodeKeys(pubKeys, consensPubKey, signer)
	err := handler.validate(ctx, msg, ver)
	c.Assert(err, IsNil)

	// cannot set node keys for active account
	keeper.na.Status = NodeActive
	msg = NewMsgSetNodeKeys(pubKeys, consensPubKey, keeper.na.NodeAddress)
	err = handler.validate(ctx, msg, ver)
	c.Assert(err, NotNil)

	// cannot set node keys for disabled account
	keeper.na.Status = NodeDisabled
	msg = NewMsgSetNodeKeys(pubKeys, consensPubKey, keeper.na.NodeAddress)
	err = handler.validate(ctx, msg, ver)
	c.Assert(err, NotNil)

	// cannot set node keys when duplicate
	keeper.na.Status = NodeStandby
	keeper.ensure = fmt.Errorf("duplicate keys")
	msg = NewMsgSetNodeKeys(keeper.na.PubKeySet, consensPubKey, keeper.na.NodeAddress)
	err = handler.validate(ctx, msg, ver)
	c.Assert(err, ErrorMatches, "duplicate keys")
	keeper.ensure = nil

	// new version GT
	err = handler.validate(ctx, msg, semver.MustParse("2.0.0"))
	c.Assert(err, IsNil)

	// invalid version
	err = handler.validate(ctx, msg, semver.Version{})
	c.Assert(err, Equals, errInvalidVersion)

	// invalid msg
	msg = MsgSetNodeKeys{}
	err = handler.validate(ctx, msg, ver)
	c.Assert(err, NotNil)
}

type TestSetNodeKeysHandleKeeper struct {
	KVStoreDummy
	na NodeAccount
}

func (k *TestSetNodeKeysHandleKeeper) GetNodeAccount(ctx cosmos.Context, signer cosmos.AccAddress) (NodeAccount, error) {
	return k.na, nil
}

func (k *TestSetNodeKeysHandleKeeper) SetNodeAccount(_ cosmos.Context, na NodeAccount) error {
	k.na = na
	return nil
}

func (k *TestSetNodeKeysHandleKeeper) EnsureNodeKeysUnique(_ cosmos.Context, consensPubKey string, pubKeys common.PubKeySet) error {
	return nil
}

func (s *HandlerSetNodeKeysSuite) TestHandle(c *C) {
	ctx, _ := setupKeeperForTest(c)

	keeper := &TestSetNodeKeysHandleKeeper{
		na: GetRandomNodeAccount(NodeActive),
	}

	handler := NewSetNodeKeysHandler(keeper, NewDummyMgr())

	ver := constants.SWVersion

	constAccessor := constants.GetConstantValues(ver)
	ctx = ctx.WithBlockHeight(1)
	signer := GetRandomBech32Addr()

	// add observer
	bepConsPubKey := GetRandomBech32ConsensusPubKey()
	bondAddr := GetRandomBNBAddress()
	pubKeys := GetRandomPubKeySet()
	emptyPubKeySet := common.PubKeySet{}

	msgNodeKeys := NewMsgSetNodeKeys(pubKeys, bepConsPubKey, signer)

	bond := cosmos.NewUint(common.One * 100)
	nodeAccount := NewNodeAccount(signer, NodeActive, emptyPubKeySet, "", bond, bondAddr, ctx.BlockHeight())
	c.Assert(keeper.SetNodeAccount(ctx, nodeAccount), IsNil)

	nodeAccount = NewNodeAccount(signer, NodeWhiteListed, emptyPubKeySet, "", bond, bondAddr, ctx.BlockHeight())
	c.Assert(keeper.SetNodeAccount(ctx, nodeAccount), IsNil)

	// happy path
	_, err := handler.handle(ctx, msgNodeKeys, ver, constAccessor)
	c.Assert(err, IsNil)
	c.Assert(keeper.na.PubKeySet, Equals, pubKeys)
	c.Assert(keeper.na.ValidatorConsPubKey, Equals, bepConsPubKey)
	c.Assert(keeper.na.Status, Equals, NodeStandby)
	c.Assert(keeper.na.StatusSince, Equals, int64(1))

	// update version
	_, err = handler.handle(ctx, msgNodeKeys, semver.MustParse("2.0.0"), constAccessor)
	c.Assert(err, IsNil)
	c.Check(keeper.na.Version.String(), Equals, "2.0.0")
}
