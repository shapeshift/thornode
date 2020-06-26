package thorchain

import (
	"github.com/blang/semver"
	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	keeper "gitlab.com/thorchain/thornode/x/thorchain/keeper"
	. "gopkg.in/check.v1"
)

type HandlerVersionSuite struct{}

type TestVersionlKeeper struct {
	keeper.KVStoreDummy
	na NodeAccount
}

func (k *TestVersionlKeeper) SendFromAccountToModule(ctx cosmos.Context, from cosmos.AccAddress, to string, coin common.Coin) error {
	return nil
}

func (k *TestVersionlKeeper) GetNodeAccount(_ cosmos.Context, _ cosmos.AccAddress) (NodeAccount, error) {
	return k.na, nil
}

func (k *TestVersionlKeeper) SetNodeAccount(_ cosmos.Context, na NodeAccount) error {
	k.na = na
	return nil
}

func (k *TestVersionlKeeper) GetVaultData(ctx cosmos.Context) (VaultData, error) {
	return NewVaultData(), nil
}

func (k *TestVersionlKeeper) SetVaultData(ctx cosmos.Context, data VaultData) error {
	return nil
}

var _ = Suite(&HandlerVersionSuite{})

func (s *HandlerVersionSuite) TestValidate(c *C) {
	ctx, _ := setupKeeperForTest(c)

	keeper := &TestVersionlKeeper{
		na: GetRandomNodeAccount(NodeActive),
	}

	handler := NewVersionHandler(keeper, NewDummyMgr())
	// happy path
	ver := constants.SWVersion
	constAccessor := constants.GetConstantValues(ver)
	msg := NewMsgSetVersion(ver, keeper.na.NodeAddress)
	err := handler.validate(ctx, msg, ver, constAccessor)
	c.Assert(err, IsNil)

	// invalid version
	err = handler.validate(ctx, msg, semver.Version{}, constAccessor)
	c.Assert(err, Equals, errBadVersion)

	// invalid msg
	msg = MsgSetVersion{}
	err = handler.validate(ctx, msg, ver, constAccessor)
	c.Assert(err, NotNil)
}

func (s *HandlerVersionSuite) TestHandle(c *C) {
	ctx, _ := setupKeeperForTest(c)
	ver := constants.SWVersion
	constAccessor := constants.GetConstantValues(ver)

	keeper := &TestVersionlKeeper{
		na: GetRandomNodeAccount(NodeActive),
	}

	handler := NewVersionHandler(keeper, NewDummyMgr())

	msg := NewMsgSetVersion(semver.MustParse("2.0.0"), GetRandomBech32Addr())
	err := handler.handle(ctx, msg, ver, constAccessor)
	c.Assert(err, IsNil)
	c.Check(keeper.na.Version.String(), Equals, "2.0.0")
}
