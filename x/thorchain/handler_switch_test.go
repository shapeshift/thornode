package thorchain

import (
	"github.com/blang/semver"
	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	. "gopkg.in/check.v1"
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
		common.NewCoin(common.RuneA1FAsset, cosmos.NewUint(100*common.One)),
	}
	destination := GetRandomBNBAddress()

	versionedTxOutStoreDummy := NewVersionedTxOutStoreDummy()

	handler := NewSwitchHandler(k, versionedTxOutStoreDummy)
	// happy path
	msg := NewMsgSwitch(tx, destination, na.NodeAddress)
	err := handler.validate(ctx, msg, constants.SWVersion)
	c.Assert(err, IsNil)

	// invalid version
	err = handler.validate(ctx, msg, semver.Version{})
	c.Assert(err, Equals, errBadVersion)

	// invalid msg
	msg = MsgSwitch{}
	err = handler.validate(ctx, msg, constants.SWVersion)
	c.Assert(err, NotNil)
}

func (s *HandlerSwitchSuite) TestGettingNativeTokens(c *C) {
	ctx, k := setupKeeperForTest(c)

	na := GetRandomNodeAccount(NodeActive)
	c.Assert(k.SetNodeAccount(ctx, na), IsNil)
	tx := GetRandomTx()
	tx.Coins = common.Coins{
		common.NewCoin(common.RuneA1FAsset, cosmos.NewUint(100*common.One)),
	}
	destination := GetRandomTHORAddress()

	versionedTxOutStoreDummy := NewVersionedTxOutStoreDummy()
	handler := NewSwitchHandler(k, versionedTxOutStoreDummy)

	msg := NewMsgSwitch(tx, destination, na.NodeAddress)
	result := handler.handle(ctx, msg, constants.SWVersion)
	c.Assert(result.IsOK(), Equals, true, Commentf("%+v", result.Log))
	coin, err := common.NewCoin(common.RuneNative, cosmos.NewUint(100*common.One)).Native()
	c.Assert(err, IsNil)
	addr, err := cosmos.AccAddressFromBech32(destination.String())
	c.Assert(err, IsNil)
	c.Check(k.CoinKeeper().HasCoins(ctx, addr, cosmos.NewCoins(coin)), Equals, true)
	vaultData, err := k.GetVaultData(ctx)
	c.Assert(err, IsNil)
	c.Check(vaultData.TotalBEP2Rune.Equal(cosmos.NewUint(100*common.One)), Equals, true)

	// check that we can add more an account
	result = handler.handle(ctx, msg, constants.SWVersion)
	c.Assert(result.IsOK(), Equals, true, Commentf("%+v", result.Log))
	coin, err = common.NewCoin(common.RuneNative, cosmos.NewUint(200*common.One)).Native()
	c.Assert(err, IsNil)
	c.Check(k.CoinKeeper().HasCoins(ctx, addr, cosmos.NewCoins(coin)), Equals, true)
	vaultData, err = k.GetVaultData(ctx)
	c.Assert(err, IsNil)
	c.Check(vaultData.TotalBEP2Rune.Equal(cosmos.NewUint(200*common.One)), Equals, true)
}

func (s *HandlerSwitchSuite) TestGettingBEP2Tokens(c *C) {
	ctx, k := setupKeeperForTest(c)

	vaultData := NewVaultData()
	vaultData.TotalBEP2Rune = cosmos.NewUint(500 * common.One)
	c.Assert(k.SetVaultData(ctx, vaultData), IsNil)

	na := GetRandomNodeAccount(NodeActive)
	c.Assert(k.SetNodeAccount(ctx, na), IsNil)

	from := GetRandomBech32Addr()
	tx := GetRandomTx()
	tx.FromAddress = common.Address(from.String())
	tx.Coins = common.Coins{
		common.NewCoin(common.RuneNative, cosmos.NewUint(100*common.One)),
	}
	destination := GetRandomBNBAddress()

	coin, err := common.NewCoin(common.RuneNative, cosmos.NewUint(800*common.One)).Native()
	c.Assert(err, IsNil)
	k.CoinKeeper().AddCoins(ctx, from, cosmos.NewCoins(coin))

	versionedTxOutStoreDummy := NewVersionedTxOutStoreDummy()
	handler := NewSwitchHandler(k, versionedTxOutStoreDummy)

	msg := NewMsgSwitch(tx, destination, na.NodeAddress)
	result := handler.handle(ctx, msg, constants.SWVersion)
	c.Assert(result.IsOK(), Equals, true, Commentf("%+v", result.Log))

	coin, err = common.NewCoin(common.RuneNative, cosmos.NewUint(700*common.One)).Native()
	c.Assert(err, IsNil)
	c.Check(k.CoinKeeper().HasCoins(ctx, from, cosmos.NewCoins(coin)), Equals, true)
	vaultData, err = k.GetVaultData(ctx)
	c.Assert(err, IsNil)
	c.Check(vaultData.TotalBEP2Rune.Equal(cosmos.NewUint(400*common.One)), Equals, true)
	items, err := versionedTxOutStoreDummy.txoutStore.GetOutboundItems(ctx)
	c.Assert(err, IsNil)
	c.Assert(items, HasLen, 1)

	// check that we can subtract more an account
	result = handler.handle(ctx, msg, constants.SWVersion)
	c.Assert(result.IsOK(), Equals, true, Commentf("%+v", result.Log))
	coin, err = common.NewCoin(common.RuneNative, cosmos.NewUint(600*common.One)).Native()
	c.Assert(err, IsNil)
	c.Check(k.CoinKeeper().HasCoins(ctx, from, cosmos.NewCoins(coin)), Equals, true)
	vaultData, err = k.GetVaultData(ctx)
	c.Assert(err, IsNil)
	c.Check(vaultData.TotalBEP2Rune.Equal(cosmos.NewUint(300*common.One)), Equals, true, Commentf("%d", vaultData.TotalBEP2Rune.Uint64()))
	items, err = versionedTxOutStoreDummy.txoutStore.GetOutboundItems(ctx)
	c.Assert(err, IsNil)
	c.Assert(items, HasLen, 2)

	// check that we can't overdraw
	msg.Tx.Coins[0].Amount = cosmos.NewUint(400 * common.One)
	result = handler.handle(ctx, msg, constants.SWVersion)
	c.Assert(result.IsOK(), Equals, false, Commentf("%+v", result.Log))
	coin, err = common.NewCoin(common.RuneNative, cosmos.NewUint(600*common.One)).Native()
	c.Assert(err, IsNil)
	c.Check(k.CoinKeeper().HasCoins(ctx, from, cosmos.NewCoins(coin)), Equals, true)
	vaultData, err = k.GetVaultData(ctx)
	c.Assert(err, IsNil)
	c.Check(vaultData.TotalBEP2Rune.Equal(cosmos.NewUint(300*common.One)), Equals, true, Commentf("%d", vaultData.TotalBEP2Rune.Uint64()))
	items, err = versionedTxOutStoreDummy.txoutStore.GetOutboundItems(ctx)
	c.Assert(err, IsNil)
	c.Assert(items, HasLen, 2)
}
