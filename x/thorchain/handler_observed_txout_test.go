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

type HandlerObservedTxOutSuite struct{}

type TestObservedTxOutValidateKeeper struct {
	keeper.KVStoreDummy
	activeNodeAccount NodeAccount
}

func (k *TestObservedTxOutValidateKeeper) GetNodeAccount(ctx cosmos.Context, signer cosmos.AccAddress) (NodeAccount, error) {
	if k.activeNodeAccount.NodeAddress.Equals(signer) {
		return k.activeNodeAccount, nil
	}
	return NodeAccount{}, nil
}

var _ = Suite(&HandlerObservedTxOutSuite{})

func (s *HandlerObservedTxOutSuite) TestValidate(c *C) {
	var err error
	ctx, _ := setupKeeperForTest(c)
	activeNodeAccount := GetRandomNodeAccount(NodeActive)

	keeper := &TestObservedTxOutValidateKeeper{
		activeNodeAccount: activeNodeAccount,
	}

	handler := NewObservedTxOutHandler(keeper, NewDummyMgr())

	// happy path
	ver := constants.SWVersion
	pk := GetRandomPubKey()
	txs := ObservedTxs{NewObservedTx(GetRandomTx(), 12, pk)}
	txs[0].Tx.FromAddress, err = pk.GetAddress(txs[0].Tx.Coins[0].Asset.Chain)
	c.Assert(err, IsNil)
	msg := NewMsgObservedTxOut(txs, activeNodeAccount.NodeAddress)
	err = handler.validate(ctx, msg, ver)
	c.Assert(err, IsNil)

	// invalid version
	err = handler.validate(ctx, msg, semver.Version{})
	c.Assert(err, Equals, errInvalidVersion)

	// inactive node account
	msg = NewMsgObservedTxOut(txs, GetRandomBech32Addr())
	err = handler.validate(ctx, msg, ver)
	c.Assert(errors.Is(err, se.ErrUnauthorized), Equals, true)

	// invalid msg
	msg = MsgObservedTxOut{}
	err = handler.validate(ctx, msg, ver)
	c.Assert(err, NotNil)
}

type TestObservedTxOutFailureKeeper struct {
	keeper.KVStoreDummy
}

type TestObservedTxOutHandleKeeper struct {
	keeper.KVStoreDummy
	nas        NodeAccounts
	na         NodeAccount
	voter      ObservedTxVoter
	yggExists  bool
	ygg        Vault
	height     int64
	pool       Pool
	txOutStore TxOutStore
	observing  []cosmos.AccAddress
	gas        []cosmos.Uint
}

func (k *TestObservedTxOutHandleKeeper) ListActiveNodeAccounts(_ cosmos.Context) (NodeAccounts, error) {
	return k.nas, nil
}

func (k *TestObservedTxOutHandleKeeper) IsActiveObserver(_ cosmos.Context, _ cosmos.AccAddress) bool {
	return true
}

func (k *TestObservedTxOutHandleKeeper) GetNodeAccountByPubKey(_ cosmos.Context, _ common.PubKey) (NodeAccount, error) {
	return k.nas[0], nil
}

func (k *TestObservedTxOutHandleKeeper) SetNodeAccount(_ cosmos.Context, na NodeAccount) error {
	k.na = na
	return nil
}

func (k *TestObservedTxOutHandleKeeper) GetObservedTxVoter(_ cosmos.Context, _ common.TxID) (ObservedTxVoter, error) {
	return k.voter, nil
}

func (k *TestObservedTxOutHandleKeeper) SetObservedTxVoter(_ cosmos.Context, voter ObservedTxVoter) {
	k.voter = voter
}

func (k *TestObservedTxOutHandleKeeper) VaultExists(_ cosmos.Context, _ common.PubKey) bool {
	return k.yggExists
}

func (k *TestObservedTxOutHandleKeeper) GetVault(_ cosmos.Context, _ common.PubKey) (Vault, error) {
	return k.ygg, nil
}

func (k *TestObservedTxOutHandleKeeper) SetVault(_ cosmos.Context, ygg Vault) error {
	k.ygg = ygg
	return nil
}

func (k *TestObservedTxOutHandleKeeper) GetVaultData(_ cosmos.Context) (VaultData, error) {
	return NewVaultData(), nil
}

func (k *TestObservedTxOutHandleKeeper) SetVaultData(_ cosmos.Context, _ VaultData) error {
	return nil
}

func (k *TestObservedTxOutHandleKeeper) SetLastChainHeight(_ cosmos.Context, _ common.Chain, height int64) error {
	k.height = height
	return nil
}

func (k *TestObservedTxOutHandleKeeper) GetPool(_ cosmos.Context, _ common.Asset) (Pool, error) {
	return k.pool, nil
}

func (k *TestObservedTxOutHandleKeeper) GetTxOut(ctx cosmos.Context, _ int64) (*TxOut, error) {
	return k.txOutStore.GetBlockOut(ctx)
}

func (k *TestObservedTxOutHandleKeeper) FindPubKeyOfAddress(_ cosmos.Context, _ common.Address, _ common.Chain) (common.PubKey, error) {
	return k.ygg.PubKey, nil
}

func (k *TestObservedTxOutHandleKeeper) SetTxOut(_ cosmos.Context, _ *TxOut) error {
	return nil
}

func (k *TestObservedTxOutHandleKeeper) AddObservingAddresses(_ cosmos.Context, addrs []cosmos.AccAddress) error {
	k.observing = addrs
	return nil
}

func (k *TestObservedTxOutHandleKeeper) SetPool(ctx cosmos.Context, pool Pool) error {
	k.pool = pool
	return nil
}

func (k *TestObservedTxOutHandleKeeper) GetGas(ctx cosmos.Context, asset common.Asset) ([]cosmos.Uint, error) {
	return k.gas, nil
}

func (k *TestObservedTxOutHandleKeeper) SetGas(ctx cosmos.Context, asset common.Asset, units []cosmos.Uint) {
	k.gas = units
}

func (s *HandlerObservedTxOutSuite) TestHandle(c *C) {
	var err error
	ctx, _ := setupKeeperForTest(c)

	ver := constants.SWVersion
	tx := GetRandomTx()
	tx.Memo = fmt.Sprintf("OUTBOUND:%s", tx.ID)
	obTx := NewObservedTx(tx, 12, GetRandomPubKey())
	txs := ObservedTxs{obTx}
	pk := GetRandomPubKey()
	c.Assert(err, IsNil)

	ygg := NewVault(ctx.BlockHeight(), ActiveVault, YggdrasilVault, pk, common.Chains{common.BNBChain})
	ygg.Coins = common.Coins{
		common.NewCoin(common.RuneAsset(), cosmos.NewUint(500)),
		common.NewCoin(common.BNBAsset, cosmos.NewUint(200*common.One)),
	}
	keeper := &TestObservedTxOutHandleKeeper{
		nas:   NodeAccounts{GetRandomNodeAccount(NodeActive)},
		voter: NewObservedTxVoter(tx.ID, make(ObservedTxs, 0)),
		pool: Pool{
			Asset:        common.BNBAsset,
			BalanceRune:  cosmos.NewUint(200),
			BalanceAsset: cosmos.NewUint(300),
		},
		yggExists: true,
		ygg:       ygg,
	}
	txOutStore := NewTxStoreDummy()
	keeper.txOutStore = txOutStore

	mgr := NewManagers(keeper)
	c.Assert(mgr.BeginBlock(ctx), IsNil)
	handler := NewObservedTxOutHandler(keeper, mgr)

	c.Assert(err, IsNil)
	msg := NewMsgObservedTxOut(txs, keeper.nas[0].NodeAddress)
	_, err = handler.handle(ctx, msg, ver)
	c.Assert(err, IsNil)
	c.Assert(err, IsNil)
	mgr.ObMgr().EndBlock(ctx, keeper)

	items, err := txOutStore.GetOutboundItems(ctx)
	c.Assert(err, IsNil)
	c.Assert(items, HasLen, 0)
	c.Check(keeper.observing, HasLen, 1)
	// make sure the coin has been subtract from the vault
	c.Check(ygg.Coins.GetCoin(common.BNBAsset).Amount.Equal(cosmos.NewUint(19999962499)), Equals, true, Commentf("%d", ygg.Coins.GetCoin(common.BNBAsset).Amount.Uint64()))
}

func (s *HandlerObservedTxOutSuite) TestGasUpdate(c *C) {
	var err error
	ctx, _ := setupKeeperForTest(c)

	ver := constants.SWVersion
	tx := GetRandomTx()
	tx.Gas = common.Gas{
		{
			Asset:  common.BNBAsset,
			Amount: cosmos.NewUint(475000),
		},
	}
	tx.Memo = fmt.Sprintf("OUTBOUND:%s", tx.ID)
	obTx := NewObservedTx(tx, 12, GetRandomPubKey())
	txs := ObservedTxs{obTx}
	pk := GetRandomPubKey()
	c.Assert(err, IsNil)

	ygg := NewVault(ctx.BlockHeight(), ActiveVault, YggdrasilVault, pk, common.Chains{common.BNBChain})
	ygg.Coins = common.Coins{
		common.NewCoin(common.RuneAsset(), cosmos.NewUint(500)),
		common.NewCoin(common.BNBAsset, cosmos.NewUint(200*common.One)),
	}
	keeper := &TestObservedTxOutHandleKeeper{
		nas:   NodeAccounts{GetRandomNodeAccount(NodeActive)},
		voter: NewObservedTxVoter(tx.ID, make(ObservedTxs, 0)),
		pool: Pool{
			Asset:        common.BNBAsset,
			BalanceRune:  cosmos.NewUint(200),
			BalanceAsset: cosmos.NewUint(300),
		},
		yggExists: true,
		ygg:       ygg,
	}
	txOutStore := NewTxStoreDummy()
	keeper.txOutStore = txOutStore

	handler := NewObservedTxOutHandler(keeper, NewDummyMgr())

	c.Assert(err, IsNil)
	msg := NewMsgObservedTxOut(txs, keeper.nas[0].NodeAddress)
	_, err = handler.handle(ctx, msg, ver)
	c.Assert(err, IsNil)
	gas := keeper.gas[0]
	c.Assert(gas.Equal(cosmos.NewUint(475000)), Equals, true, Commentf("%+v", gas))
	// revert the gas change , otherwise it messed up the other tests
	gasInfo := common.UpdateGasPrice(common.Tx{}, common.BNBAsset, []cosmos.Uint{cosmos.NewUint(37500), cosmos.NewUint(30000)})
	keeper.SetGas(ctx, common.BNBAsset, gasInfo)
}

func (s *HandlerObservedTxOutSuite) TestHandleStolenFunds(c *C) {
	var err error
	ctx, _ := setupKeeperForTest(c)

	ver := constants.SWVersion
	tx := GetRandomTx()
	tx.Memo = "I AM A THIEF!" // bad memo
	obTx := NewObservedTx(tx, 12, GetRandomPubKey())
	obTx.Tx.Coins = common.Coins{
		common.NewCoin(common.RuneAsset(), cosmos.NewUint(300*common.One)),
		common.NewCoin(common.BNBAsset, cosmos.NewUint(100*common.One)),
	}
	txs := ObservedTxs{obTx}
	pk := GetRandomPubKey()
	c.Assert(err, IsNil)

	na := GetRandomNodeAccount(NodeActive)
	na.Bond = cosmos.NewUint(1000000 * common.One)
	na.PubKeySet.Secp256k1 = pk

	ygg := NewVault(ctx.BlockHeight(), ActiveVault, YggdrasilVault, pk, common.Chains{common.BNBChain})
	ygg.Coins = common.Coins{
		common.NewCoin(common.RuneAsset(), cosmos.NewUint(500*common.One)),
		common.NewCoin(common.BNBAsset, cosmos.NewUint(200*common.One)),
	}
	keeper := &TestObservedTxOutHandleKeeper{
		nas:   NodeAccounts{na},
		voter: NewObservedTxVoter(tx.ID, make(ObservedTxs, 0)),
		pool: Pool{
			Asset:        common.BNBAsset,
			BalanceRune:  cosmos.NewUint(200 * common.One),
			BalanceAsset: cosmos.NewUint(300 * common.One),
		},
		yggExists: true,
		ygg:       ygg,
	}
	txOutStore := NewTxStoreDummy()
	keeper.txOutStore = txOutStore

	mgr := NewDummyMgr()
	mgr.slasher = NewSlasherV1(keeper)
	handler := NewObservedTxOutHandler(keeper, mgr)

	c.Assert(err, IsNil)
	msg := NewMsgObservedTxOut(txs, keeper.nas[0].NodeAddress)
	_, err = handler.handle(ctx, msg, ver)
	c.Assert(err, IsNil)
	// make sure the coin has been subtract from the vault
	c.Check(ygg.Coins.GetCoin(common.BNBAsset).Amount.Equal(cosmos.NewUint(9999962500)), Equals, true, Commentf("%d", ygg.Coins.GetCoin(common.BNBAsset).Amount.Uint64()))
	c.Assert(keeper.na.Bond.LT(cosmos.NewUint(1000000*common.One)), Equals, true, Commentf("%d", keeper.na.Bond.Uint64()))
}
