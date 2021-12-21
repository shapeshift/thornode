package thorchain

import (
	"errors"
	"fmt"
	"strings"

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
	standbyNodeAccount  NodeAccount
	failGetNodeAccount  NodeAccount
	notEmptyNodeAccount NodeAccount
}

func (k *TestBondKeeper) GetNodeAccount(_ cosmos.Context, addr cosmos.AccAddress) (NodeAccount, error) {
	if k.standbyNodeAccount.NodeAddress.Equals(addr) {
		return k.standbyNodeAccount, nil
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

func (HandlerBondSuite) TestBondHandler_ValidateActive(c *C) {
	ctx, k := setupKeeperForTest(c)

	activeNodeAccount := GetRandomValidatorNode(NodeActive)
	c.Assert(k.SetNodeAccount(ctx, activeNodeAccount), IsNil)

	vault := GetRandomVault()
	vault.Status = RetiringVault
	c.Assert(k.SetVault(ctx, vault), IsNil)

	handler := NewBondHandler(NewDummyMgrWithKeeper(k))

	txIn := common.NewTx(
		GetRandomTxHash(),
		GetRandomBNBAddress(),
		GetRandomBNBAddress(),
		common.Coins{
			common.NewCoin(common.RuneAsset(), cosmos.NewUint(10*common.One)),
		},
		BNBGasFeeSingleton,
		"bond",
	)
	msg := NewMsgBond(txIn, activeNodeAccount.NodeAddress, cosmos.NewUint(10*common.One), GetRandomBNBAddress(), activeNodeAccount.NodeAddress)

	// happy path
	c.Assert(handler.validate(ctx, *msg), IsNil)

	vault.Status = ActiveVault
	c.Assert(k.SetVault(ctx, vault), IsNil)

	// unhappy path
	c.Assert(handler.validate(ctx, *msg), NotNil)
}

func (HandlerBondSuite) TestBondHandler_Run(c *C) {
	ctx, k1 := setupKeeperForTest(c)

	standbyNodeAccount := GetRandomValidatorNode(NodeStandby)
	k := &TestBondKeeper{
		Keeper:              k1,
		standbyNodeAccount:  standbyNodeAccount,
		failGetNodeAccount:  GetRandomValidatorNode(NodeStandby),
		notEmptyNodeAccount: GetRandomValidatorNode(NodeStandby),
	}
	// happy path
	c.Assert(k1.SetNodeAccount(ctx, standbyNodeAccount), IsNil)
	handler := NewBondHandler(NewDummyMgrWithKeeper(k1))
	ver := GetCurrentVersion()
	constAccessor := constants.GetConstantValues(ver)
	minimumBondInRune := constAccessor.GetInt64Value(constants.MinimumBondInRune)
	txIn := common.NewTx(
		GetRandomTxHash(),
		GetRandomTHORAddress(),
		GetRandomTHORAddress(),
		common.Coins{
			common.NewCoin(common.RuneAsset(), cosmos.NewUint(uint64(minimumBondInRune+common.One))),
		},
		BNBGasFeeSingleton,
		"bond",
	)
	FundModule(c, ctx, k1, BondName, uint64(minimumBondInRune))
	msg := NewMsgBond(txIn, GetRandomValidatorNode(NodeStandby).NodeAddress, cosmos.NewUint(uint64(minimumBondInRune)+common.One), GetRandomTHORAddress(), nil, standbyNodeAccount.NodeAddress)
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
	msg = NewMsgBond(txIn, k.failGetNodeAccount.NodeAddress, cosmos.NewUint(uint64(minimumBondInRune)), GetRandomTHORAddress(), nil, standbyNodeAccount.NodeAddress)
	_, err = handler.Run(ctx, msg)
	c.Assert(errors.Is(err, errInternal), Equals, true)

	// When node account is standby , it is ok to bond
	msg = NewMsgBond(txIn, k.notEmptyNodeAccount.NodeAddress, cosmos.NewUint(uint64(minimumBondInRune)), common.Address(k.notEmptyNodeAccount.NodeAddress.String()), nil, standbyNodeAccount.NodeAddress)
	_, err = handler.Run(ctx, msg)
	c.Assert(err, IsNil)
}

func (HandlerBondSuite) TestBondHandlerFailValidation(c *C) {
	ctx, k := setupKeeperForTest(c)
	standbyNodeAccount := GetRandomValidatorNode(NodeStandby)
	c.Assert(k.SetNodeAccount(ctx, standbyNodeAccount), IsNil)
	handler := NewBondHandler(NewDummyMgrWithKeeper(k))
	ver := GetCurrentVersion()
	constAccessor := constants.GetConstantValues(ver)
	minimumBondInRune := constAccessor.GetInt64Value(constants.MinimumBondInRune)
	txIn := common.NewTx(
		GetRandomTxHash(),
		GetRandomTHORAddress(),
		GetRandomTHORAddress(),
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
			msg:         NewMsgBond(txIn, cosmos.AccAddress{}, cosmos.NewUint(uint64(minimumBondInRune)), GetRandomTHORAddress(), nil, activeNodeAccount.NodeAddress),
			expectedErr: se.ErrInvalidAddress,
		},
		{
			name:        "zero bond",
			msg:         NewMsgBond(txIn, GetRandomValidatorNode(NodeStandby).NodeAddress, cosmos.ZeroUint(), GetRandomTHORAddress(), nil, activeNodeAccount.NodeAddress),
			expectedErr: se.ErrUnknownRequest,
		},
		{
			name:        "empty bond address",
			msg:         NewMsgBond(txIn, GetRandomValidatorNode(NodeStandby).NodeAddress, cosmos.NewUint(uint64(minimumBondInRune)), common.Address(""), nil, activeNodeAccount.NodeAddress),
			expectedErr: se.ErrInvalidAddress,
		},
		{
			name:        "empty request hash",
			msg:         NewMsgBond(txInNoTxID, GetRandomValidatorNode(NodeStandby).NodeAddress, cosmos.NewUint(uint64(minimumBondInRune)), GetRandomTHORAddress(), nil, activeNodeAccount.NodeAddress),
			expectedErr: se.ErrUnknownRequest,
		},
		{
			name:        "empty signer",
			msg:         NewMsgBond(txIn, GetRandomValidatorNode(NodeStandby).NodeAddress, cosmos.NewUint(uint64(minimumBondInRune)), GetRandomTHORAddress(), nil, cosmos.AccAddress{}),
			expectedErr: se.ErrInvalidAddress,
		},
		{
			name:        "active node",
			msg:         NewMsgBond(txIn, GetRandomValidatorNode(NodeActive).NodeAddress, cosmos.NewUint(uint64(minimumBondInRune)), GetRandomBNBAddress(), cosmos.AccAddress{}),
			expectedErr: se.ErrInvalidAddress,
		},
	}
	for _, item := range testCases {
		c.Log(item.name)
		_, err := handler.Run(ctx, item.msg)
		c.Check(errors.Is(err, item.expectedErr), Equals, true, Commentf("name: %s, %s != %s", item.name, item.expectedErr, err))
	}
}

func (HandlerBondSuite) TestBondProvider_Validate(c *C) {
	ctx, k := setupKeeperForTest(c)
	activeNodeAccount := GetRandomValidatorNode(NodeActive)
	c.Assert(k.SetNodeAccount(ctx, activeNodeAccount), IsNil)
	standbyNodeAccount := GetRandomValidatorNode(NodeStandby)
	c.Assert(k.SetNodeAccount(ctx, standbyNodeAccount), IsNil)
	handler := NewBondHandler(NewDummyMgrWithKeeper(k))
	txIn := GetRandomTx()
	amt := cosmos.NewUint(100 * common.One)
	txIn.Coins = common.NewCoins(common.NewCoin(common.RuneAsset(), amt))
	activeNA := activeNodeAccount.NodeAddress
	activeNAAddress := common.Address(activeNA.String())
	standbyNA := standbyNodeAccount.NodeAddress
	standbyNAAddress := common.Address(standbyNA.String())
	additionalBondAddress := GetRandomBech32Addr()

	errCheck := func(c *C, err error, str string) {
		c.Check(strings.Contains(err.Error(), str), Equals, true, Commentf("%w", err))
	}

	// TEST VALIDATION //
	// happy path
	msg := NewMsgBond(txIn, standbyNA, amt, standbyNAAddress, additionalBondAddress, activeNA)
	err := handler.validate(ctx, *msg)
	c.Assert(err, IsNil)

	// try to bond while node account is active
	msg = NewMsgBond(txIn, activeNA, amt, activeNAAddress, nil, activeNA)
	err = handler.validate(ctx, *msg)
	errCheck(c, err, "node account is active or ready status")

	// try to bond with a bnb address
	msg = NewMsgBond(txIn, standbyNA, amt, GetRandomBNBAddress(), nil, activeNA)
	err = handler.validate(ctx, *msg)
	errCheck(c, err, "bonding address is NOT a THORChain address")

	// try to bond with a valid additional bond provider
	bp := NewBondProviders(standbyNA)
	bp.Providers = []BondProvider{NewBondProvider(additionalBondAddress)}
	c.Assert(k.SetBondProviders(ctx, bp), IsNil)
	msg = NewMsgBond(txIn, standbyNA, amt, common.Address(additionalBondAddress.String()), nil, activeNA)
	err = handler.validate(ctx, *msg)
	c.Assert(err, IsNil)

	// try to bond with an invalid additional bond provider
	msg = NewMsgBond(txIn, standbyNA, amt, GetRandomTHORAddress(), nil, activeNA)
	err = handler.validate(ctx, *msg)
	errCheck(c, err, "bond address is not valid for node account")

}

func (HandlerBondSuite) TestBondProvider_Handler(c *C) {
	fmt.Println("*******************************************")
	defer fmt.Println("*******************************************")
	ctx, k := setupKeeperForTest(c)
	activeNodeAccount := GetRandomValidatorNode(NodeActive)
	c.Assert(k.SetNodeAccount(ctx, activeNodeAccount), IsNil)
	standbyNodeAccount := GetRandomValidatorNode(NodeStandby)
	c.Assert(k.SetNodeAccount(ctx, standbyNodeAccount), IsNil)
	handler := NewBondHandler(NewDummyMgrWithKeeper(k))
	txIn := GetRandomTx()
	amt := cosmos.NewUint(105 * common.One)
	txIn.Coins = common.NewCoins(common.NewCoin(common.RuneAsset(), amt))
	activeNA := activeNodeAccount.NodeAddress
	// activeNAAddress := common.Address(activeNA.String())
	standbyNA := standbyNodeAccount.NodeAddress
	standbyNAAddress := common.Address(standbyNA.String())
	additionalBondAddress := GetRandomBech32Addr()

	/*
		errCheck := func(c *C, err error, str string) {
			c.Check(strings.Contains(err.Error(), str), Equals, true, Commentf("%w", err))
		}
	*/

	// TEST HANDLER //
	// happy path, and add a whitelisted address
	msg := NewMsgBond(txIn, standbyNA, amt, standbyNAAddress, additionalBondAddress, activeNA)
	err := handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	na, _ := k.GetNodeAccount(ctx, standbyNA)
	c.Check(na.Bond.Uint64(), Equals, amt.Uint64(), Commentf("%d", na.Bond.Uint64()))
	bp, _ := k.GetBondProviders(ctx, standbyNA)
	c.Assert(bp.Providers, HasLen, 1)
	fmt.Printf("%+v\n", bp)
	c.Assert(bp.Has(additionalBondAddress), Equals, true)
	c.Assert(bp.Get(additionalBondAddress).Bond.Uint64(), Equals, uint64(0), Commentf("%d", bp.Get(additionalBondAddress).Bond.Uint64()))
}
