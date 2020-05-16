package thorchain

import (
	"github.com/blang/semver"
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

type HandlerObservedTxInSuite struct{}

type TestObservedTxInValidateKeeper struct {
	KVStoreDummy
	activeNodeAccount NodeAccount
	standbyAccount    NodeAccount
}

func (k *TestObservedTxInValidateKeeper) GetNodeAccount(_ cosmos.Context, addr cosmos.AccAddress) (NodeAccount, error) {
	if addr.Equals(k.standbyAccount.NodeAddress) {
		return k.standbyAccount, nil
	}
	if addr.Equals(k.activeNodeAccount.NodeAddress) {
		return k.activeNodeAccount, nil
	}
	return NodeAccount{}, kaboom
}

func (k *TestObservedTxInValidateKeeper) SetNodeAccount(_ cosmos.Context, na NodeAccount) error {
	if na.NodeAddress.Equals(k.standbyAccount.NodeAddress) {
		k.standbyAccount = na
		return nil
	}
	return kaboom
}

var _ = Suite(&HandlerObservedTxInSuite{})

func (s *HandlerObservedTxInSuite) TestValidate(c *C) {
	var err error
	ctx, _ := setupKeeperForTest(c)
	w := getHandlerTestWrapper(c, 1, true, false)
	activeNodeAccount := GetRandomNodeAccount(NodeActive)
	standbyAccount := GetRandomNodeAccount(NodeStandby)
	keeper := &TestObservedTxInValidateKeeper{
		activeNodeAccount: activeNodeAccount,
		standbyAccount:    standbyAccount,
	}

	versionedVaultMgrDummy := NewVersionedVaultMgrDummy(w.versionedTxOutStore)
	versionedGasMgr := NewDummyVersionedGasMgr()
	versionedObMgr := NewDummyVersionedObserverMgr()
	versionedEventManagerDummy := NewDummyVersionedEventMgr()

	handler := NewObservedTxInHandler(keeper, versionedObMgr, w.versionedTxOutStore, w.validatorMgr, versionedVaultMgrDummy, versionedGasMgr, versionedEventManagerDummy)

	// happy path
	ver := constants.SWVersion
	pk := GetRandomPubKey()
	txs := ObservedTxs{NewObservedTx(GetRandomTx(), 12, pk)}
	txs[0].Tx.ToAddress, err = pk.GetAddress(txs[0].Tx.Coins[0].Asset.Chain)
	c.Assert(err, IsNil)
	msg := NewMsgObservedTxIn(txs, activeNodeAccount.NodeAddress)
	isNewSigner, err := handler.validate(ctx, msg, ver)
	c.Assert(err, IsNil)
	c.Assert(isNewSigner, Equals, false)

	// invalid version
	isNewSigner, err = handler.validate(ctx, msg, semver.Version{})
	c.Assert(err, Equals, errInvalidVersion)
	c.Assert(isNewSigner, Equals, false)

	// inactive node account
	msg = NewMsgObservedTxIn(txs, GetRandomBech32Addr())
	isNewSigner, err = handler.validate(ctx, msg, ver)
	c.Assert(err, Equals, notAuthorized)
	c.Assert(isNewSigner, Equals, false)

	// invalid msg
	msg = MsgObservedTxIn{}
	isNewSigner, err = handler.validate(ctx, msg, ver)
	c.Assert(err, NotNil)
	c.Assert(isNewSigner, Equals, false)
}

type TestObservedTxInFailureKeeper struct {
	KVStoreDummy
	pool Pool
	evt  Event
}

func (k *TestObservedTxInFailureKeeper) GetPool(_ cosmos.Context, _ common.Asset) (Pool, error) {
	return k.pool, nil
}

func (k *TestObservedTxInFailureKeeper) UpsertEvent(_ cosmos.Context, evt Event) error {
	k.evt = evt
	return nil
}

func (s *HandlerObservedTxInSuite) TestFailure(c *C) {
	ctx, _ := setupKeeperForTest(c)
	// w := getHandlerTestWrapper(c, 1, true, false)

	keeper := &TestObservedTxInFailureKeeper{
		pool: Pool{
			Asset:        common.BNBAsset,
			BalanceRune:  cosmos.NewUint(200),
			BalanceAsset: cosmos.NewUint(300),
		},
	}
	txOutStore := NewTxStoreDummy()

	tx := NewObservedTx(GetRandomTx(), 12, GetRandomPubKey())
	ver := constants.SWVersion
	constAccessor := constants.GetConstantValues(ver)
	err := refundTx(ctx, tx, txOutStore, keeper, constAccessor, CodeInvalidMemo, "Invalid memo", NewEventMgr())
	c.Assert(err, IsNil)
	items, err := txOutStore.GetOutboundItems(ctx)
	c.Assert(err, IsNil)
	c.Check(items, HasLen, 1)
}

type TestObservedTxInHandleKeeper struct {
	KVStoreDummy
	nas       NodeAccounts
	voter     ObservedTxVoter
	yggExists bool
	height    int64
	msg       MsgSwap
	pool      Pool
	observing []cosmos.AccAddress
	vault     Vault
	txOut     *TxOut
}

func (k *TestObservedTxInHandleKeeper) SetSwapQueueItem(_ cosmos.Context, msg MsgSwap) error {
	k.msg = msg
	return nil
}

func (k *TestObservedTxInHandleKeeper) ListActiveNodeAccounts(_ cosmos.Context) (NodeAccounts, error) {
	return k.nas, nil
}

func (k *TestObservedTxInHandleKeeper) GetObservedTxVoter(_ cosmos.Context, _ common.TxID) (ObservedTxVoter, error) {
	return k.voter, nil
}

func (k *TestObservedTxInHandleKeeper) SetObservedTxVoter(_ cosmos.Context, voter ObservedTxVoter) {
	k.voter = voter
}

func (k *TestObservedTxInHandleKeeper) VaultExists(_ cosmos.Context, _ common.PubKey) bool {
	return k.yggExists
}

func (k *TestObservedTxInHandleKeeper) SetLastChainHeight(_ cosmos.Context, _ common.Chain, height int64) error {
	k.height = height
	return nil
}

func (k *TestObservedTxInHandleKeeper) GetPool(_ cosmos.Context, _ common.Asset) (Pool, error) {
	return k.pool, nil
}

func (k *TestObservedTxInHandleKeeper) AddIncompleteEvents(_ cosmos.Context, evt Event) error {
	return nil
}

func (k *TestObservedTxInHandleKeeper) AddObservingAddresses(_ cosmos.Context, addrs []cosmos.AccAddress) error {
	k.observing = addrs
	return nil
}

func (k *TestObservedTxInHandleKeeper) UpsertEvent(_ cosmos.Context, _ Event) error {
	return nil
}

func (k *TestObservedTxInHandleKeeper) GetVault(_ cosmos.Context, key common.PubKey) (Vault, error) {
	if k.vault.PubKey.Equals(key) {
		return k.vault, nil
	}
	return GetRandomVault(), kaboom
}

func (k *TestObservedTxInHandleKeeper) SetVault(_ cosmos.Context, vault Vault) error {
	if k.vault.PubKey.Equals(vault.PubKey) {
		k.vault = vault
		return nil
	}
	return kaboom
}

func (k *TestObservedTxInHandleKeeper) GetLowestActiveVersion(_ cosmos.Context) semver.Version {
	return constants.SWVersion
}

func (k *TestObservedTxInHandleKeeper) IsActiveObserver(_ cosmos.Context, addr cosmos.AccAddress) bool {
	if addr.Equals(k.nas[0].NodeAddress) {
		return true
	}
	return false
}

func (k *TestObservedTxInHandleKeeper) GetTxOut(ctx cosmos.Context, blockHeight int64) (*TxOut, error) {
	if k.txOut != nil && k.txOut.Height == blockHeight {
		return k.txOut, nil
	}
	return nil, kaboom
}

func (k *TestObservedTxInHandleKeeper) SetTxOut(ctx cosmos.Context, blockOut *TxOut) error {
	if k.txOut.Height == blockOut.Height {
		k.txOut = blockOut
		return nil
	}
	return kaboom
}

func (s *HandlerObservedTxInSuite) TestHandle(c *C) {
	var err error
	ctx, _ := setupKeeperForTest(c)
	w := getHandlerTestWrapper(c, 1, true, false)

	ver := constants.SWVersion

	tx := GetRandomTx()
	tx.Memo = "SWAP:BTC.BTC:" + GetRandomBTCAddress().String()
	obTx := NewObservedTx(tx, 12, GetRandomPubKey())
	txs := ObservedTxs{obTx}
	pk := GetRandomPubKey()
	txs[0].Tx.ToAddress, err = pk.GetAddress(txs[0].Tx.Coins[0].Asset.Chain)

	vault := GetRandomVault()
	vault.PubKey = obTx.ObservedPubKey

	keeper := &TestObservedTxInHandleKeeper{
		nas:   NodeAccounts{GetRandomNodeAccount(NodeActive)},
		voter: NewObservedTxVoter(tx.ID, make(ObservedTxs, 0)),
		vault: vault,
		pool: Pool{
			Asset:        common.BNBAsset,
			BalanceRune:  cosmos.NewUint(200),
			BalanceAsset: cosmos.NewUint(300),
		},
		yggExists: true,
	}
	versionedTxOutStore := NewVersionedTxOutStoreDummy()
	versionedVaultMgrDummy := NewVersionedVaultMgrDummy(versionedTxOutStore)
	versionedGasMgr := NewVersionedGasMgr()
	versionedObMgr := NewVersionedObserverMgr()
	versionedEventManagerDummy := NewDummyVersionedEventMgr()

	handler := NewObservedTxInHandler(keeper, versionedObMgr, versionedTxOutStore, w.validatorMgr, versionedVaultMgrDummy, versionedGasMgr, versionedEventManagerDummy)

	c.Assert(err, IsNil)
	msg := NewMsgObservedTxIn(txs, keeper.nas[0].NodeAddress)
	result := handler.handle(ctx, msg, ver)
	obMgr, err := versionedObMgr.GetObserverManager(ctx, ver)
	c.Assert(err, IsNil)
	obMgr.EndBlock(ctx, keeper)

	c.Assert(result.IsOK(), Equals, true, Commentf("%s", result.Log))
	c.Check(keeper.msg.Tx.ID.Equals(tx.ID), Equals, true)
	c.Check(keeper.observing, HasLen, 1)
	c.Check(keeper.height, Equals, int64(12))
	bnbCoin := keeper.vault.Coins.GetCoin(common.BNBAsset)
	c.Assert(bnbCoin.Amount.Equal(cosmos.OneUint()), Equals, true)
}

// Test migrate memo
func (s *HandlerObservedTxInSuite) TestMigrateMemo(c *C) {
	var err error
	ctx, _ := setupKeeperForTest(c)
	w := getHandlerTestWrapper(c, 1, true, false)
	ver := constants.SWVersion

	vault := GetRandomVault()
	addr, err := vault.PubKey.GetAddress(common.BNBChain)
	c.Assert(err, IsNil)
	newVault := GetRandomVault()
	txout := NewTxOut(12)
	newVaultAddr, err := newVault.PubKey.GetAddress(common.BNBChain)
	c.Assert(err, IsNil)

	txout.TxArray = append(txout.TxArray, &TxOutItem{
		Chain:       common.BNBChain,
		InHash:      common.BlankTxID,
		ToAddress:   newVaultAddr,
		VaultPubKey: vault.PubKey,
		Coin:        common.NewCoin(common.BNBAsset, cosmos.NewUint(1024)),
		Memo:        NewMigrateMemo(1).String(),
	})
	tx := NewObservedTx(common.Tx{
		ID:    GetRandomTxHash(),
		Chain: common.BNBChain,
		Coins: common.Coins{
			common.NewCoin(common.BNBAsset, cosmos.NewUint(1024)),
		},
		Memo:        NewMigrateMemo(12).String(),
		FromAddress: addr,
		ToAddress:   newVaultAddr,
		Gas:         BNBGasFeeSingleton,
	}, 13, vault.PubKey)

	txs := ObservedTxs{tx}
	keeper := &TestObservedTxInHandleKeeper{
		nas:   NodeAccounts{GetRandomNodeAccount(NodeActive)},
		voter: NewObservedTxVoter(tx.Tx.ID, make(ObservedTxs, 0)),
		vault: vault,
		pool: Pool{
			Asset:        common.BNBAsset,
			BalanceRune:  cosmos.NewUint(200),
			BalanceAsset: cosmos.NewUint(300),
		},
		yggExists: true,
		txOut:     txout,
	}
	versionedTxOutStore := NewVersionedTxOutStoreDummy()
	c.Assert(err, IsNil)
	versionedVaultMgrDummy := NewVersionedVaultMgrDummy(versionedTxOutStore)
	versionedGasMgr := NewVersionedGasMgr()
	versionedObMgr := NewDummyVersionedObserverMgr()
	versionedEventManagerDummy := NewDummyVersionedEventMgr()

	handler := NewObservedTxInHandler(keeper, versionedObMgr, versionedTxOutStore, w.validatorMgr, versionedVaultMgrDummy, versionedGasMgr, versionedEventManagerDummy)

	c.Assert(err, IsNil)
	msg := NewMsgObservedTxIn(txs, keeper.nas[0].NodeAddress)
	result := handler.handle(ctx, msg, ver)
	c.Assert(result.IsOK(), Equals, true)
}
