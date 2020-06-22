package thorchain

import (
	"errors"
	"fmt"

	"github.com/blang/semver"
	se "github.com/cosmos/cosmos-sdk/types/errors"
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	keeper "gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

type HandlerUnBondSuite struct{}

type TestUnBondKeeper struct {
	keeper.KVStoreDummy
	activeNodeAccount   NodeAccount
	failGetNodeAccount  NodeAccount
	notEmptyNodeAccount NodeAccount
	vault               Vault
}

func (k *TestUnBondKeeper) GetNodeAccount(_ cosmos.Context, addr cosmos.AccAddress) (NodeAccount, error) {
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

func (k *TestUnBondKeeper) GetVault(_ cosmos.Context, _ common.PubKey) (Vault, error) {
	return k.vault, nil
}

func (k *TestUnBondKeeper) VaultExists(_ cosmos.Context, _ common.PubKey) bool {
	return true
}

var _ = Suite(&HandlerUnBondSuite{})

func (HandlerUnBondSuite) TestUnBondHandler_Run(c *C) {
	ctx, k1 := setupKeeperForTest(c)
	// happy path
	activeNodeAccount := GetRandomNodeAccount(NodeActive)
	standbyNodeAccount := GetRandomNodeAccount(NodeStandby)
	c.Assert(k1.SetNodeAccount(ctx, activeNodeAccount), IsNil)
	c.Assert(k1.SetNodeAccount(ctx, standbyNodeAccount), IsNil)
	vault := NewVault(12, ActiveVault, YggdrasilVault, standbyNodeAccount.PubKeySet.Secp256k1, nil)
	c.Assert(k1.SetVault(ctx, vault), IsNil)
	vault = NewVault(12, ActiveVault, AsgardVault, GetRandomPubKey(), nil)
	vault.Coins = common.Coins{
		common.NewCoin(common.RuneAsset(), cosmos.NewUint(10000*common.One)),
	}
	c.Assert(k1.SetVault(ctx, vault), IsNil)

	handler := NewUnBondHandler(k1, NewDummyMgr())
	ver := constants.SWVersion
	constAccessor := constants.GetConstantValues(ver)
	txIn := common.NewTx(
		GetRandomTxHash(),
		GetRandomBNBAddress(),
		GetRandomBNBAddress(),
		common.Coins{
			common.NewCoin(common.RuneAsset(), cosmos.NewUint(uint64(1))),
		},
		BNBGasFeeSingleton,
		"unbond me please",
	)
	msg := NewMsgUnBond(txIn, standbyNodeAccount.NodeAddress, cosmos.NewUint(uint64(5*common.One)), standbyNodeAccount.BondAddress, activeNodeAccount.NodeAddress)
	fmt.Println("^^^^^^^^^^^^^^^^^^^^^^^^")
	_, err := handler.Run(ctx, msg, ver, constAccessor)
	c.Assert(err, IsNil)
	na, err := k1.GetNodeAccount(ctx, standbyNodeAccount.NodeAddress)
	c.Assert(err, IsNil)
	c.Check(na.Bond.Equal(cosmos.NewUint(95*common.One)), Equals, true, Commentf("%d", na.Bond.Uint64()))
	fmt.Println("^^^^^^^^^^^^^^^^^^^^^^^^")

	k := &TestUnBondKeeper{
		activeNodeAccount:   activeNodeAccount,
		failGetNodeAccount:  GetRandomNodeAccount(NodeActive),
		notEmptyNodeAccount: GetRandomNodeAccount(NodeActive),
	}
	// invalid version
	handler = NewUnBondHandler(k, NewDummyMgr())
	ver = semver.Version{}
	_, err = handler.Run(ctx, msg, ver, constAccessor)
	c.Assert(errors.Is(err, errBadVersion), Equals, true)

	// simulate fail to get node account
	ver = constants.SWVersion
	msg = NewMsgUnBond(txIn, k.failGetNodeAccount.NodeAddress, cosmos.NewUint(uint64(1)), GetRandomBNBAddress(), activeNodeAccount.NodeAddress)
	_, err = handler.Run(ctx, msg, ver, constAccessor)
	c.Assert(errors.Is(err, errInternal), Equals, true)

	// simulate vault with funds
	k.vault = Vault{
		Type: YggdrasilVault,
		Coins: common.Coins{
			common.NewCoin(common.RuneAsset(), cosmos.NewUint(uint64(1))),
		},
	}
	msg = NewMsgUnBond(txIn, activeNodeAccount.NodeAddress, cosmos.NewUint(uint64(1)), GetRandomBNBAddress(), activeNodeAccount.NodeAddress)
	_, err = handler.Run(ctx, msg, ver, constAccessor)
	c.Assert(errors.Is(err, se.ErrUnknownRequest), Equals, true)
}

func (HandlerUnBondSuite) TestUnBondHandlerFailValidation(c *C) {
	ctx, k := setupKeeperForTest(c)
	activeNodeAccount := GetRandomNodeAccount(NodeActive)
	c.Assert(k.SetNodeAccount(ctx, activeNodeAccount), IsNil)
	handler := NewUnBondHandler(k, NewDummyMgr())
	ver := constants.SWVersion
	constAccessor := constants.GetConstantValues(ver)
	txIn := common.NewTx(
		GetRandomTxHash(),
		GetRandomBNBAddress(),
		GetRandomBNBAddress(),
		common.Coins{
			common.NewCoin(common.RuneAsset(), cosmos.NewUint(uint64(1))),
		},
		BNBGasFeeSingleton,
		"unbond it",
	)
	txInNoTxID := txIn
	txInNoTxID.ID = ""
	testCases := []struct {
		name        string
		msg         MsgUnBond
		expectedErr error
	}{
		{
			name:        "empty node address",
			msg:         NewMsgUnBond(txIn, cosmos.AccAddress{}, cosmos.NewUint(uint64(1)), GetRandomBNBAddress(), activeNodeAccount.NodeAddress),
			expectedErr: se.ErrInvalidAddress,
		},
		{
			name:        "zero bond",
			msg:         NewMsgUnBond(txIn, GetRandomNodeAccount(NodeStandby).NodeAddress, cosmos.ZeroUint(), GetRandomBNBAddress(), activeNodeAccount.NodeAddress),
			expectedErr: se.ErrUnknownRequest,
		},
		{
			name:        "empty bond address",
			msg:         NewMsgUnBond(txIn, GetRandomNodeAccount(NodeStandby).NodeAddress, cosmos.NewUint(uint64(1)), common.Address(""), activeNodeAccount.NodeAddress),
			expectedErr: se.ErrInvalidAddress,
		},
		{
			name:        "empty request hash",
			msg:         NewMsgUnBond(txInNoTxID, GetRandomNodeAccount(NodeStandby).NodeAddress, cosmos.NewUint(uint64(1)), GetRandomBNBAddress(), activeNodeAccount.NodeAddress),
			expectedErr: se.ErrUnknownRequest,
		},
		{
			name:        "empty signer",
			msg:         NewMsgUnBond(txIn, GetRandomNodeAccount(NodeStandby).NodeAddress, cosmos.NewUint(uint64(1)), GetRandomBNBAddress(), cosmos.AccAddress{}),
			expectedErr: se.ErrInvalidAddress,
		},
		{
			name:        "msg not signed by active account",
			msg:         NewMsgUnBond(txIn, GetRandomNodeAccount(NodeStandby).NodeAddress, cosmos.NewUint(uint64(1)), GetRandomBNBAddress(), GetRandomNodeAccount(NodeStandby).NodeAddress),
			expectedErr: se.ErrUnauthorized,
		},
		{
			name:        "account shouldn't be active",
			msg:         NewMsgUnBond(txIn, activeNodeAccount.NodeAddress, cosmos.NewUint(uint64(1)), GetRandomBNBAddress(), activeNodeAccount.NodeAddress),
			expectedErr: se.ErrUnknownRequest,
		},
	}
	for _, item := range testCases {
		c.Log(item.name)
		_, err := handler.Run(ctx, item.msg, ver, constAccessor)

		c.Check(errors.Is(err, item.expectedErr), Equals, true, Commentf("name: %s, %s", item.name, err))
	}
}
