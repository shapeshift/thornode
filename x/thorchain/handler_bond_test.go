package thorchain

import (
	"errors"
	"fmt"

	se "github.com/cosmos/cosmos-sdk/types/errors"
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

type HandlerBondSuite struct{}

type TestBondKeeper struct {
	keeper.Keeper
	activeNodeAccount   NodeAccount
	failGetNodeAccount  NodeAccount
	notEmptyNodeAccount NodeAccount
}

func (k *TestBondKeeper) GetNodeAccount(_ cosmos.Context, addr cosmos.AccAddress) (NodeAccount, error) {
	if k.activeNodeAccount.NodeAddress.Equals(addr) {
		return k.activeNodeAccount, nil
	}
	if k.failGetNodeAccount.NodeAddress.Equals(addr) {
		return NodeAccount{}, fmt.Errorf("you asked for this error")
	}
	if k.notEmptyNodeAccount.NodeAddress.Equals(addr) {
		return k.notEmptyNodeAccount, nil
	}
	return NodeAccount{}, nil
}

var _ = Suite(&HandlerBondSuite{})

func (HandlerBondSuite) TestBondHandler_Run(c *C) {
	ctx, k1 := setupKeeperForTest(c)

	activeNodeAccount := GetRandomNodeAccount(NodeActive)
	k := &TestBondKeeper{
		Keeper:              k1,
		activeNodeAccount:   activeNodeAccount,
		failGetNodeAccount:  GetRandomNodeAccount(NodeActive),
		notEmptyNodeAccount: GetRandomNodeAccount(NodeActive),
	}
	// happy path
	c.Assert(k1.SetNodeAccount(ctx, activeNodeAccount), IsNil)
	handler := NewBondHandler(NewDummyMgrWithKeeper(k1))
	ver := GetCurrentVersion()
	constAccessor := constants.GetConstantValues(ver)
	minimumBondInRune := constAccessor.GetInt64Value(constants.MinimumBondInRune)
	txIn := common.NewTx(
		GetRandomTxHash(),
		GetRandomBNBAddress(),
		GetRandomBNBAddress(),
		common.Coins{
			common.NewCoin(common.RuneAsset(), cosmos.NewUint(uint64(minimumBondInRune+common.One))),
		},
		BNBGasFeeSingleton,
		"bond",
	)
	FundModule(c, ctx, k1, BondName, uint64(minimumBondInRune))
	msg := NewMsgBond(txIn, GetRandomNodeAccount(NodeStandby).NodeAddress, cosmos.NewUint(uint64(minimumBondInRune)+common.One), GetRandomBNBAddress(), activeNodeAccount.NodeAddress)
	_, err := handler.Run(ctx, msg)
	c.Assert(err, IsNil)
	coin := common.NewCoin(common.RuneNative, cosmos.NewUint(common.One))
	nativeRuneCoin, err := coin.Native()
	c.Assert(err, IsNil)
	c.Assert(k1.HasCoins(ctx, msg.NodeAddress, cosmos.NewCoins(nativeRuneCoin)), Equals, true)
	na, err := k1.GetNodeAccount(ctx, msg.NodeAddress)
	c.Assert(err, IsNil)
	c.Assert(na.Status.String(), Equals, NodeWhiteListed.String())
	c.Assert(na.Bond.Equal(cosmos.NewUint(uint64(minimumBondInRune))), Equals, true)

	// simulate fail to get node account
	handler = NewBondHandler(NewDummyMgrWithKeeper(k))
	ver = GetCurrentVersion()
	msg = NewMsgBond(txIn, k.failGetNodeAccount.NodeAddress, cosmos.NewUint(uint64(minimumBondInRune)), GetRandomBNBAddress(), activeNodeAccount.NodeAddress)
	_, err = handler.Run(ctx, msg)
	c.Assert(errors.Is(err, errInternal), Equals, true)

	// When node account is active , it is ok to bond
	msg = NewMsgBond(txIn, k.notEmptyNodeAccount.NodeAddress, cosmos.NewUint(uint64(minimumBondInRune)), GetRandomBNBAddress(), activeNodeAccount.NodeAddress)
	_, err = handler.Run(ctx, msg)
	c.Assert(err, IsNil)
}

func (HandlerBondSuite) TestBondHandlerFailValidation(c *C) {
	ctx, k := setupKeeperForTest(c)
	activeNodeAccount := GetRandomNodeAccount(NodeActive)
	c.Assert(k.SetNodeAccount(ctx, activeNodeAccount), IsNil)
	handler := NewBondHandler(NewDummyMgrWithKeeper(k))
	ver := GetCurrentVersion()
	constAccessor := constants.GetConstantValues(ver)
	minimumBondInRune := constAccessor.GetInt64Value(constants.MinimumBondInRune)
	txIn := common.NewTx(
		GetRandomTxHash(),
		GetRandomBNBAddress(),
		GetRandomBNBAddress(),
		common.Coins{
			common.NewCoin(common.RuneAsset(), cosmos.NewUint(uint64(minimumBondInRune))),
		},
		BNBGasFeeSingleton,
		"apply",
	)
	txInNoTxID := txIn
	txInNoTxID.ID = ""
	testCases := []struct {
		name        string
		msg         *MsgBond
		expectedErr error
	}{
		{
			name:        "empty node address",
			msg:         NewMsgBond(txIn, cosmos.AccAddress{}, cosmos.NewUint(uint64(minimumBondInRune)), GetRandomBNBAddress(), activeNodeAccount.NodeAddress),
			expectedErr: se.ErrInvalidAddress,
		},
		{
			name:        "zero bond",
			msg:         NewMsgBond(txIn, GetRandomNodeAccount(NodeStandby).NodeAddress, cosmos.ZeroUint(), GetRandomBNBAddress(), activeNodeAccount.NodeAddress),
			expectedErr: se.ErrUnknownRequest,
		},
		{
			name:        "empty bond address",
			msg:         NewMsgBond(txIn, GetRandomNodeAccount(NodeStandby).NodeAddress, cosmos.NewUint(uint64(minimumBondInRune)), common.Address(""), activeNodeAccount.NodeAddress),
			expectedErr: se.ErrInvalidAddress,
		},
		{
			name:        "empty request hash",
			msg:         NewMsgBond(txInNoTxID, GetRandomNodeAccount(NodeStandby).NodeAddress, cosmos.NewUint(uint64(minimumBondInRune)), GetRandomBNBAddress(), activeNodeAccount.NodeAddress),
			expectedErr: se.ErrUnknownRequest,
		},
		{
			name:        "empty signer",
			msg:         NewMsgBond(txIn, GetRandomNodeAccount(NodeStandby).NodeAddress, cosmos.NewUint(uint64(minimumBondInRune)), GetRandomBNBAddress(), cosmos.AccAddress{}),
			expectedErr: se.ErrInvalidAddress,
		},
	}
	for _, item := range testCases {
		c.Log(item.name)
		_, err := handler.Run(ctx, item.msg)
		c.Check(errors.Is(err, item.expectedErr), Equals, true, Commentf("name: %s, %s != %s", item.name, item.expectedErr, err))
	}
}
