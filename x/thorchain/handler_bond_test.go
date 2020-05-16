package thorchain

import (
	"fmt"

	"github.com/blang/semver"
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

type HandlerBondSuite struct{}

type TestBondKeeper struct {
	KVStoreDummy
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
		activeNodeAccount:   activeNodeAccount,
		failGetNodeAccount:  GetRandomNodeAccount(NodeActive),
		notEmptyNodeAccount: GetRandomNodeAccount(NodeActive),
	}
	// happy path
	c.Assert(k1.SetNodeAccount(ctx, activeNodeAccount), IsNil)
	handler := NewBondHandler(k1, NewVersionedEventMgr())
	ver := constants.SWVersion
	constAccessor := constants.GetConstantValues(ver)
	minimumBondInRune := constAccessor.GetInt64Value(constants.MinimumBondInRune)
	txIn := common.NewTx(
		GetRandomTxHash(),
		GetRandomBNBAddress(),
		GetRandomBNBAddress(),
		common.Coins{
			common.NewCoin(common.RuneAsset(), cosmos.NewUint(uint64(minimumBondInRune))),
		},
		common.Gas{},
		"apply",
	)
	msg := NewMsgBond(txIn, GetRandomNodeAccount(NodeStandby).NodeAddress, cosmos.NewUint(uint64(minimumBondInRune)), GetRandomBNBAddress(), activeNodeAccount.NodeAddress)
	result := handler.Run(ctx, msg, ver, constAccessor)
	c.Assert(result.IsOK(), Equals, true)

	// invalid version
	handler = NewBondHandler(k, NewVersionedEventMgr())
	ver = semver.Version{}
	result = handler.Run(ctx, msg, ver, constAccessor)
	c.Assert(result.Code, Equals, CodeBadVersion)

	// simulate fail to get node account
	ver = constants.SWVersion
	msg = NewMsgBond(txIn, k.failGetNodeAccount.NodeAddress, cosmos.NewUint(uint64(minimumBondInRune)), GetRandomBNBAddress(), activeNodeAccount.NodeAddress)
	result = handler.Run(ctx, msg, ver, constAccessor)
	c.Assert(result.Code, Equals, cosmos.CodeInternal)

	msg = NewMsgBond(txIn, k.notEmptyNodeAccount.NodeAddress, cosmos.NewUint(uint64(minimumBondInRune)), GetRandomBNBAddress(), activeNodeAccount.NodeAddress)
	result = handler.Run(ctx, msg, ver, constAccessor)
	c.Assert(result.Code, Equals, cosmos.CodeInternal)
}

func (HandlerBondSuite) TestBondHandlerFailValidation(c *C) {
	ctx, k := setupKeeperForTest(c)
	activeNodeAccount := GetRandomNodeAccount(NodeActive)
	c.Assert(k.SetNodeAccount(ctx, activeNodeAccount), IsNil)
	handler := NewBondHandler(k, NewVersionedEventMgr())
	ver := constants.SWVersion
	constAccessor := constants.GetConstantValues(ver)
	minimumBondInRune := constAccessor.GetInt64Value(constants.MinimumBondInRune)
	txIn := common.NewTx(
		GetRandomTxHash(),
		GetRandomBNBAddress(),
		GetRandomBNBAddress(),
		common.Coins{
			common.NewCoin(common.RuneAsset(), cosmos.NewUint(uint64(minimumBondInRune))),
		},
		common.Gas{},
		"apply",
	)
	txInNoTxID := txIn
	txInNoTxID.ID = ""
	testCases := []struct {
		name         string
		msg          MsgBond
		expectedCode cosmos.CodeType
	}{
		{
			name:         "empty node address",
			msg:          NewMsgBond(txIn, cosmos.AccAddress{}, cosmos.NewUint(uint64(minimumBondInRune)), GetRandomBNBAddress(), activeNodeAccount.NodeAddress),
			expectedCode: cosmos.CodeUnknownRequest,
		},
		{
			name:         "zero bond",
			msg:          NewMsgBond(txIn, GetRandomNodeAccount(NodeStandby).NodeAddress, cosmos.ZeroUint(), GetRandomBNBAddress(), activeNodeAccount.NodeAddress),
			expectedCode: cosmos.CodeUnknownRequest,
		},
		{
			name:         "empty bond address",
			msg:          NewMsgBond(txIn, GetRandomNodeAccount(NodeStandby).NodeAddress, cosmos.NewUint(uint64(minimumBondInRune)), common.Address(""), activeNodeAccount.NodeAddress),
			expectedCode: cosmos.CodeUnknownRequest,
		},
		{
			name:         "empty request hash",
			msg:          NewMsgBond(txInNoTxID, GetRandomNodeAccount(NodeStandby).NodeAddress, cosmos.NewUint(uint64(minimumBondInRune)), GetRandomBNBAddress(), activeNodeAccount.NodeAddress),
			expectedCode: cosmos.CodeUnknownRequest,
		},
		{
			name:         "empty signer",
			msg:          NewMsgBond(txIn, GetRandomNodeAccount(NodeStandby).NodeAddress, cosmos.NewUint(uint64(minimumBondInRune)), GetRandomBNBAddress(), cosmos.AccAddress{}),
			expectedCode: cosmos.CodeInvalidAddress,
		},
		{
			name:         "msg not signed by active account",
			msg:          NewMsgBond(txIn, GetRandomNodeAccount(NodeStandby).NodeAddress, cosmos.NewUint(uint64(minimumBondInRune)), GetRandomBNBAddress(), GetRandomNodeAccount(NodeStandby).NodeAddress),
			expectedCode: cosmos.CodeUnauthorized,
		},
		{
			name:         "not enough rune",
			msg:          NewMsgBond(txIn, GetRandomNodeAccount(NodeStandby).NodeAddress, cosmos.NewUint(uint64(minimumBondInRune-100)), GetRandomBNBAddress(), activeNodeAccount.NodeAddress),
			expectedCode: cosmos.CodeUnknownRequest,
		},
	}
	for _, item := range testCases {
		c.Log(item.name)
		result := handler.Run(ctx, item.msg, ver, constAccessor)
		c.Assert(result.Code, Equals, item.expectedCode)
	}
}
