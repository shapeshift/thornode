package thorchain

import (
	"errors"

	"github.com/blang/semver"
	se "github.com/cosmos/cosmos-sdk/types/errors"
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

type HandlerLeaveSuite struct{}

var _ = Suite(&HandlerLeaveSuite{})

func (HandlerLeaveSuite) TestLeaveHandler_NotActiveNodeLeave(c *C) {
	w := getHandlerTestWrapper(c, 1, true, false)
	vault := GetRandomVault()
	w.keeper.SetVault(w.ctx, vault)
	leaveHandler := NewLeaveHandler(w.keeper, NewDummyMgr())
	acc2 := GetRandomNodeAccount(NodeStandby)
	acc2.Bond = cosmos.NewUint(100 * common.One)
	c.Assert(w.keeper.SetNodeAccount(w.ctx, acc2), IsNil)
	ygg := NewVault(common.BlockHeight(w.ctx), ActiveVault, YggdrasilVault, acc2.PubKeySet.Secp256k1, common.Chains{common.RuneAsset().Chain})
	c.Assert(w.keeper.SetVault(w.ctx, ygg), IsNil)

	FundModule(c, w.ctx, w.keeper, BondName, 100)

	txID := GetRandomTxHash()
	tx := common.NewTx(
		txID,
		acc2.BondAddress,
		GetRandomBNBAddress(),
		common.Coins{common.NewCoin(common.RuneAsset(), cosmos.OneUint())},
		BNBGasFeeSingleton,
		"LEAVE",
	)
	msgLeave := NewMsgLeave(tx, acc2.NodeAddress, w.activeNodeAccount.NodeAddress)
	ver := constants.SWVersion
	constAccessor := constants.GetConstantValues(ver)
	_, err := leaveHandler.Run(w.ctx, msgLeave, ver, constAccessor)
	c.Assert(err, IsNil)
	_, err = leaveHandler.Run(w.ctx, msgLeave, semver.Version{}, constAccessor)
	c.Assert(err, NotNil)
}

func (HandlerLeaveSuite) TestLeaveHandler_ActiveNodeLeave(c *C) {
	var err error
	w := getHandlerTestWrapper(c, 1, true, false)
	leaveHandler := NewLeaveHandler(w.keeper, NewDummyMgr())
	acc2 := GetRandomNodeAccount(NodeActive)
	acc2.Bond = cosmos.NewUint(100 * common.One)
	c.Assert(w.keeper.SetNodeAccount(w.ctx, acc2), IsNil)
	txID := GetRandomTxHash()
	tx := common.NewTx(
		txID,
		acc2.BondAddress,
		GetRandomBNBAddress(),
		common.Coins{common.NewCoin(common.RuneAsset(), cosmos.OneUint())},
		BNBGasFeeSingleton,
		"",
	)
	msgLeave := NewMsgLeave(tx, acc2.NodeAddress, w.activeNodeAccount.NodeAddress)
	ver := constants.SWVersion
	constAccessor := constants.GetConstantValues(ver)
	_, err = leaveHandler.Run(w.ctx, msgLeave, ver, constAccessor)
	c.Assert(err, IsNil)

	acc2, err = w.keeper.GetNodeAccount(w.ctx, acc2.NodeAddress)
	c.Assert(err, IsNil)
	c.Check(acc2.Bond.Equal(cosmos.NewUint(10000000001)), Equals, true, Commentf("Bond:%d\n", acc2.Bond.Uint64()))
}

func (HandlerLeaveSuite) TestLeaveJail(c *C) {
	w := getHandlerTestWrapper(c, 1, true, false)
	vault := GetRandomVault()
	w.keeper.SetVault(w.ctx, vault)
	leaveHandler := NewLeaveHandler(w.keeper, NewDummyMgr())
	acc2 := GetRandomNodeAccount(NodeStandby)
	acc2.Bond = cosmos.NewUint(100 * common.One)
	c.Assert(w.keeper.SetNodeAccount(w.ctx, acc2), IsNil)

	w.keeper.SetNodeAccountJail(w.ctx, acc2.NodeAddress, common.BlockHeight(w.ctx)+100, "test it")

	ygg := NewVault(common.BlockHeight(w.ctx), ActiveVault, YggdrasilVault, acc2.PubKeySet.Secp256k1, common.Chains{common.RuneAsset().Chain})
	c.Assert(w.keeper.SetVault(w.ctx, ygg), IsNil)

	FundModule(c, w.ctx, w.keeper, BondName, 100)

	txID := GetRandomTxHash()
	tx := common.NewTx(
		txID,
		acc2.BondAddress,
		GetRandomBNBAddress(),
		common.Coins{common.NewCoin(common.RuneAsset(), cosmos.OneUint())},
		BNBGasFeeSingleton,
		"LEAVE",
	)
	msgLeave := NewMsgLeave(tx, acc2.NodeAddress, w.activeNodeAccount.NodeAddress)
	ver := constants.SWVersion
	constAccessor := constants.GetConstantValues(ver)
	_, err := leaveHandler.Run(w.ctx, msgLeave, ver, constAccessor)
	c.Assert(err, NotNil)
}

func (HandlerLeaveSuite) TestLeaveValidation(c *C) {
	w := getHandlerTestWrapper(c, 1, true, false)
	ver := constants.SWVersion
	constAccessor := constants.GetConstantValues(ver)
	testCases := []struct {
		name          string
		msgLeave      MsgLeave
		expectedError error
	}{
		{
			name: "empty from address should fail",
			msgLeave: NewMsgLeave(common.Tx{
				ID:          GetRandomTxHash(),
				Chain:       common.BNBChain,
				FromAddress: "",
				ToAddress:   GetRandomBNBAddress(),
				Coins: common.Coins{
					common.NewCoin(common.BNBAsset, cosmos.NewUint(common.One)),
				},
				Gas: common.Gas{
					common.NewCoin(common.BNBAsset, cosmos.NewUint(common.One)),
				},
				Memo: "",
			}, w.activeNodeAccount.NodeAddress, w.activeNodeAccount.NodeAddress),
			expectedError: se.ErrUnknownRequest,
		},
		{
			name: "non-matching from address should fail",
			msgLeave: NewMsgLeave(common.Tx{
				ID:          GetRandomTxHash(),
				Chain:       common.BNBChain,
				FromAddress: GetRandomBNBAddress(),
				ToAddress:   GetRandomBNBAddress(),
				Coins: common.Coins{
					common.NewCoin(common.BNBAsset, cosmos.NewUint(common.One)),
				},
				Gas: common.Gas{
					common.NewCoin(common.BNBAsset, cosmos.NewUint(common.One)),
				},
				Memo: "",
			}, w.activeNodeAccount.NodeAddress, w.activeNodeAccount.NodeAddress),
			expectedError: se.ErrUnauthorized,
		},
		{
			name: "empty tx id should fail",
			msgLeave: NewMsgLeave(common.Tx{
				ID:          common.TxID(""),
				Chain:       common.BNBChain,
				FromAddress: w.activeNodeAccount.BondAddress,
				ToAddress:   GetRandomBNBAddress(),
				Coins: common.Coins{
					common.NewCoin(common.BNBAsset, cosmos.NewUint(common.One)),
				},
				Gas: common.Gas{
					common.NewCoin(common.BNBAsset, cosmos.NewUint(common.One)),
				},
				Memo: "",
			}, w.activeNodeAccount.NodeAddress, w.activeNodeAccount.NodeAddress),
			expectedError: se.ErrUnknownRequest,
		},
		{
			name: "empty signer should fail",
			msgLeave: NewMsgLeave(common.Tx{
				ID:          GetRandomTxHash(),
				Chain:       common.BNBChain,
				FromAddress: w.activeNodeAccount.BondAddress,
				ToAddress:   GetRandomBNBAddress(),
				Coins: common.Coins{
					common.NewCoin(common.BNBAsset, cosmos.NewUint(common.One)),
				},
				Gas: common.Gas{
					common.NewCoin(common.BNBAsset, cosmos.NewUint(common.One)),
				},
				Memo: "",
			}, w.activeNodeAccount.NodeAddress, cosmos.AccAddress{}),
			expectedError: se.ErrUnknownRequest,
		},
	}
	for _, item := range testCases {
		c.Log(item.name)
		leaveHandler := NewLeaveHandler(w.keeper, NewDummyMgr())
		_, err := leaveHandler.Run(w.ctx, item.msgLeave, ver, constAccessor)
		c.Check(errors.Is(err, item.expectedError), Equals, true, Commentf("name:%s, %s", item.name, err))
	}
}
