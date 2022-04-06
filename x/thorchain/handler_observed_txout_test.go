package thorchain

import (
	"errors"
	"fmt"

	"github.com/blang/semver"
	se "github.com/cosmos/cosmos-sdk/types/errors"
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
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
	activeNodeAccount := GetRandomValidatorNode(NodeActive)

	keeper := &TestObservedTxOutValidateKeeper{
		activeNodeAccount: activeNodeAccount,
	}

	handler := NewObservedTxOutHandler(NewDummyMgrWithKeeper(keeper))

	// happy path
	pk := GetRandomPubKey()
	txs := ObservedTxs{NewObservedTx(GetRandomTx(), 12, pk, 12)}
	txs[0].Tx.FromAddress, err = pk.GetAddress(txs[0].Tx.Coins[0].Asset.Chain)
	c.Assert(err, IsNil)
	msg := NewMsgObservedTxOut(txs, activeNodeAccount.NodeAddress)
	err = handler.validate(ctx, *msg)
	c.Assert(err, IsNil)

	// inactive node account
	msg = NewMsgObservedTxOut(txs, GetRandomBech32Addr())
	err = handler.validate(ctx, *msg)
	c.Assert(errors.Is(err, se.ErrUnauthorized), Equals, true)

	// invalid msg
	msg = &MsgObservedTxOut{}
	err = handler.validate(ctx, *msg)
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
}

func (k *TestObservedTxOutHandleKeeper) ListActiveValidators(_ cosmos.Context) (NodeAccounts, error) {
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

func (k *TestObservedTxOutHandleKeeper) GetObservedTxOutVoter(_ cosmos.Context, _ common.TxID) (ObservedTxVoter, error) {
	return k.voter, nil
}

func (k *TestObservedTxOutHandleKeeper) SetObservedTxOutVoter(_ cosmos.Context, voter ObservedTxVoter) {
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

func (k *TestObservedTxOutHandleKeeper) GetNetwork(_ cosmos.Context) (Network, error) {
	return NewNetwork(), nil
}

func (k *TestObservedTxOutHandleKeeper) SetNetwork(_ cosmos.Context, _ Network) error {
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

func (s *HandlerObservedTxOutSuite) TestHandle(c *C) {
	var err error
	ctx, mgr := setupManagerForTest(c)

	tx := GetRandomTx()
	tx.Memo = fmt.Sprintf("OUT:%s", tx.ID)
	obTx := NewObservedTx(tx, 12, GetRandomPubKey(), 12)
	txs := ObservedTxs{obTx}
	pk := GetRandomPubKey()
	c.Assert(err, IsNil)

	ygg := NewVault(common.BlockHeight(ctx), ActiveVault, YggdrasilVault, pk, common.Chains{common.BNBChain}.Strings(), []ChainContract{})
	ygg.Coins = common.Coins{
		common.NewCoin(common.RuneAsset(), cosmos.NewUint(500)),
		common.NewCoin(common.BNBAsset, cosmos.NewUint(200*common.One)),
	}
	keeper := &TestObservedTxOutHandleKeeper{
		nas:   NodeAccounts{GetRandomValidatorNode(NodeActive)},
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

	mgr.K = keeper
	handler := NewObservedTxOutHandler(mgr)

	c.Assert(err, IsNil)
	msg := NewMsgObservedTxOut(txs, keeper.nas[0].NodeAddress)
	_, err = handler.handle(ctx, *msg)
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

func (s *HandlerObservedTxOutSuite) TestHandleStolenFunds(c *C) {
	var err error
	ctx, _ := setupKeeperForTest(c)

	tx := GetRandomTx()
	tx.Memo = "I AM A THIEF!" // bad memo
	obTx := NewObservedTx(tx, 12, GetRandomPubKey(), 12)
	obTx.Tx.Coins = common.Coins{
		common.NewCoin(common.RuneAsset(), cosmos.NewUint(300*common.One)),
		common.NewCoin(common.BNBAsset, cosmos.NewUint(100*common.One)),
	}
	txs := ObservedTxs{obTx}
	pk := GetRandomPubKey()
	c.Assert(err, IsNil)

	na := GetRandomValidatorNode(NodeActive)
	na.Bond = cosmos.NewUint(1000000 * common.One)
	na.PubKeySet.Secp256k1 = pk

	ygg := NewVault(common.BlockHeight(ctx), ActiveVault, YggdrasilVault, pk, common.Chains{common.BNBChain}.Strings(), []ChainContract{})
	ygg.Membership = []string{pk.String()}
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

	mgr := NewDummyMgrWithKeeper(keeper)
	mgr.slasher = newSlasherV75(keeper, NewDummyEventMgr())
	handler := NewObservedTxOutHandler(mgr)

	c.Assert(err, IsNil)
	msg := NewMsgObservedTxOut(txs, keeper.nas[0].NodeAddress)
	_, err = handler.handle(ctx, *msg)
	c.Assert(err, IsNil)
	// make sure the coin has been subtract from the vault
	c.Check(ygg.Coins.GetCoin(common.BNBAsset).Amount.Equal(cosmos.NewUint(9999962500)), Equals, true, Commentf("%d", ygg.Coins.GetCoin(common.BNBAsset).Amount.Uint64()))
	c.Assert(keeper.na.Bond.LT(cosmos.NewUint(1000000*common.One)), Equals, true, Commentf("%d", keeper.na.Bond.Uint64()))
}

type HandlerObservedTxOutTestHelper struct {
	keeper.Keeper
	failListActiveValidators bool
	failVaultExist           bool
	failGetObservedTxOutVote bool
	failGetVault             bool
	failSetVault             bool
}

func NewHandlerObservedTxOutHelper(k keeper.Keeper) *HandlerObservedTxOutTestHelper {
	return &HandlerObservedTxOutTestHelper{
		Keeper: k,
	}
}

func (h *HandlerObservedTxOutTestHelper) ListActiveValidators(ctx cosmos.Context) (NodeAccounts, error) {
	if h.failListActiveValidators {
		return NodeAccounts{}, errKaboom
	}
	return h.Keeper.ListActiveValidators(ctx)
}

func (h *HandlerObservedTxOutTestHelper) VaultExists(ctx cosmos.Context, pk common.PubKey) bool {
	if h.failVaultExist {
		return false
	}
	return h.Keeper.VaultExists(ctx, pk)
}

func (h *HandlerObservedTxOutTestHelper) GetObservedTxOutVoter(ctx cosmos.Context, hash common.TxID) (ObservedTxVoter, error) {
	if h.failGetObservedTxOutVote {
		return ObservedTxVoter{}, errKaboom
	}
	return h.Keeper.GetObservedTxOutVoter(ctx, hash)
}

func (h *HandlerObservedTxOutTestHelper) GetVault(ctx cosmos.Context, pk common.PubKey) (Vault, error) {
	if h.failGetVault {
		return Vault{}, errKaboom
	}
	return h.Keeper.GetVault(ctx, pk)
}

func (h *HandlerObservedTxOutTestHelper) SetVault(ctx cosmos.Context, vault Vault) error {
	if h.failSetVault {
		return errKaboom
	}
	return h.Keeper.SetVault(ctx, vault)
}

func setupAnObservedTxOut(ctx cosmos.Context, helper *HandlerObservedTxOutTestHelper, c *C) *MsgObservedTxOut {
	activeNodeAccount := GetRandomValidatorNode(NodeActive)
	pk := GetRandomPubKey()
	tx := GetRandomTx()
	tx.Coins = common.Coins{
		common.NewCoin(common.BNBAsset, cosmos.NewUint(common.One*3)),
	}
	tx.Memo = "OUT:" + GetRandomTxHash().String()
	addr, err := pk.GetAddress(tx.Coins[0].Asset.Chain)
	c.Assert(err, IsNil)
	tx.ToAddress = GetRandomBNBAddress()
	tx.FromAddress = addr
	obTx := NewObservedTx(tx, ctx.BlockHeight(), pk, ctx.BlockHeight())
	txs := ObservedTxs{obTx}
	c.Assert(err, IsNil)
	vault := GetRandomVault()
	vault.PubKey = obTx.ObservedPubKey
	vault.Membership = []string{vault.PubKey.String()}
	c.Assert(helper.Keeper.SetNodeAccount(ctx, activeNodeAccount), IsNil)
	c.Assert(helper.SetVault(ctx, vault), IsNil)
	p := NewPool()
	p.Asset = common.BNBAsset
	p.BalanceRune = cosmos.NewUint(100 * common.One)
	p.BalanceAsset = cosmos.NewUint(100 * common.One)
	p.Status = PoolAvailable
	c.Assert(helper.Keeper.SetPool(ctx, p), IsNil)
	return NewMsgObservedTxOut(txs, activeNodeAccount.NodeAddress)
}

func (HandlerObservedTxOutSuite) TestHandlerObservedTxOut_DifferentValidations(c *C) {
	testCases := []struct {
		name            string
		messageProvider func(c *C, ctx cosmos.Context, helper *HandlerObservedTxOutTestHelper) cosmos.Msg
		validator       func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *HandlerObservedTxOutTestHelper, name string)
	}{
		{
			name: "invalid message should return an error",
			messageProvider: func(c *C, ctx cosmos.Context, helper *HandlerObservedTxOutTestHelper) cosmos.Msg {
				return NewMsgNetworkFee(ctx.BlockHeight(), common.BNBChain, 1, bnbSingleTxFee.Uint64(), GetRandomBech32Addr())
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *HandlerObservedTxOutTestHelper, name string) {
				c.Check(err, NotNil, Commentf(name))
				c.Check(result, IsNil)
				c.Check(errors.Is(err, errInvalidMessage), Equals, true)
			},
		},
		{
			name: "message fail validation should return an error",
			messageProvider: func(c *C, ctx cosmos.Context, helper *HandlerObservedTxOutTestHelper) cosmos.Msg {
				return NewMsgObservedTxOut(ObservedTxs{
					NewObservedTx(GetRandomTx(), ctx.BlockHeight(), GetRandomPubKey(), ctx.BlockHeight()),
				}, GetRandomBech32Addr())
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *HandlerObservedTxOutTestHelper, name string) {
				c.Check(err, NotNil, Commentf(name))
				c.Check(result, IsNil)
			},
		},
		{
			name: "voter already vote for the tx should return without doing anything",
			messageProvider: func(c *C, ctx cosmos.Context, helper *HandlerObservedTxOutTestHelper) cosmos.Msg {
				m := setupAnObservedTxOut(ctx, helper, c)
				voter, err := helper.Keeper.GetObservedTxOutVoter(ctx, m.Txs[0].Tx.ID)
				c.Assert(err, IsNil)
				voter.Add(m.Txs[0], m.Signer)
				helper.Keeper.SetObservedTxOutVoter(ctx, voter)
				return m
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *HandlerObservedTxOutTestHelper, name string) {
				c.Check(err, IsNil, Commentf(name))
				c.Check(result, NotNil, Commentf(name))
			},
		},
		{
			name: "fail to list active node accounts should result in an error",
			messageProvider: func(c *C, ctx cosmos.Context, helper *HandlerObservedTxOutTestHelper) cosmos.Msg {
				m := setupAnObservedTxOut(ctx, helper, c)
				helper.failListActiveValidators = true
				return m
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *HandlerObservedTxOutTestHelper, name string) {
				c.Check(err, NotNil, Commentf(name))
				c.Check(result, IsNil, Commentf(name))
			},
		},
		{
			name: "vault not exist should not result in an error, it should continue",
			messageProvider: func(c *C, ctx cosmos.Context, helper *HandlerObservedTxOutTestHelper) cosmos.Msg {
				m := setupAnObservedTxOut(ctx, helper, c)
				helper.failVaultExist = true
				return m
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *HandlerObservedTxOutTestHelper, name string) {
				c.Check(err, IsNil, Commentf(name))
				c.Check(result, NotNil, Commentf(name))
			},
		},
		{
			name: "fail to get observedTxOutVoter should not result in an error, it should continue",
			messageProvider: func(c *C, ctx cosmos.Context, helper *HandlerObservedTxOutTestHelper) cosmos.Msg {
				m := setupAnObservedTxOut(ctx, helper, c)
				helper.failGetObservedTxOutVote = true
				return m
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *HandlerObservedTxOutTestHelper, name string) {
				c.Check(err, IsNil, Commentf(name))
				c.Check(result, NotNil, Commentf(name))
			},
		},
		{
			name: "empty memo should not result in an error, it should continue",
			messageProvider: func(c *C, ctx cosmos.Context, helper *HandlerObservedTxOutTestHelper) cosmos.Msg {
				m := setupAnObservedTxOut(ctx, helper, c)
				m.Txs[0].Tx.Memo = ""
				return m
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *HandlerObservedTxOutTestHelper, name string) {
				c.Check(err, IsNil, Commentf(name))
				c.Check(result, NotNil, Commentf(name))
				txOut, err := helper.GetTxOut(ctx, ctx.BlockHeight())
				c.Assert(err, IsNil, Commentf(name))
				c.Assert(txOut.IsEmpty(), Equals, true)
			},
		},
		{
			name: "fail to get vault, it should continue",
			messageProvider: func(c *C, ctx cosmos.Context, helper *HandlerObservedTxOutTestHelper) cosmos.Msg {
				m := setupAnObservedTxOut(ctx, helper, c)
				helper.failGetVault = true
				return m
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *HandlerObservedTxOutTestHelper, name string) {
				c.Check(err, IsNil, Commentf(name))
				c.Check(result, NotNil, Commentf(name))
			},
		},
		{
			name: "fail to set vault, it should continue",
			messageProvider: func(c *C, ctx cosmos.Context, helper *HandlerObservedTxOutTestHelper) cosmos.Msg {
				m := setupAnObservedTxOut(ctx, helper, c)
				helper.failSetVault = true
				return m
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *HandlerObservedTxOutTestHelper, name string) {
				c.Check(err, IsNil, Commentf(name))
				c.Check(result, NotNil, Commentf(name))
			},
		},
		{
			name: "ragnarok memo it should success",
			messageProvider: func(c *C, ctx cosmos.Context, helper *HandlerObservedTxOutTestHelper) cosmos.Msg {
				m := setupAnObservedTxOut(ctx, helper, c)
				m.Txs[0].Tx.Memo = "ragnarok:100"
				return m
			},
			validator: func(c *C, ctx cosmos.Context, result *cosmos.Result, err error, helper *HandlerObservedTxOutTestHelper, name string) {
				c.Check(err, IsNil, Commentf(name))
				c.Check(result, NotNil, Commentf(name))
			},
		},
	}
	versions := []semver.Version{
		GetCurrentVersion(),
	}
	for _, tc := range testCases {
		for _, ver := range versions {
			ctx, mgr := setupManagerForTest(c)
			helper := NewHandlerObservedTxOutHelper(mgr.Keeper())
			mgr.K = helper
			mgr.CurrentVersion = ver
			handler := NewObservedTxOutHandler(mgr)
			msg := tc.messageProvider(c, ctx, helper)
			result, err := handler.Run(ctx, msg)
			tc.validator(c, ctx, result, err, helper, tc.name)
		}
	}
}
