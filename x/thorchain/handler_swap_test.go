package thorchain

import (
	"errors"

	"github.com/blang/semver"

	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"

	. "gopkg.in/check.v1"
)

type HandlerSwapSuite struct{}

var _ = Suite(&HandlerSwapSuite{})

func (s *HandlerSwapSuite) TestValidate(c *C) {
	ctx, _ := setupKeeperForTest(c)

	keeper := &TestSwapHandleKeeper{
		activeNodeAccount: GetRandomNodeAccount(NodeActive),
	}

	handler := NewSwapHandler(keeper, NewDummyMgr())

	ver := constants.SWVersion
	txID := GetRandomTxHash()
	signerBNBAddr := GetRandomBNBAddress()
	observerAddr := keeper.activeNodeAccount.NodeAddress
	tx := common.NewTx(
		txID,
		signerBNBAddr,
		signerBNBAddr,
		common.Coins{
			common.NewCoin(common.RuneAsset(), cosmos.OneUint()),
		},
		BNBGasFeeSingleton,
		"",
	)
	msg := NewMsgSwap(tx, common.BNBAsset, signerBNBAddr, cosmos.ZeroUint(), observerAddr)
	err := handler.validate(ctx, msg, ver)
	c.Assert(err, IsNil)

	// invalid version
	err = handler.validate(ctx, msg, semver.Version{})
	c.Assert(err, Equals, errInvalidVersion)

	// invalid msg
	msg = MsgSwap{}
	err = handler.validate(ctx, msg, ver)
	c.Assert(err, NotNil)
}

type TestSwapHandleKeeper struct {
	KVStoreDummy
	pools             map[common.Asset]Pool
	activeNodeAccount NodeAccount
	event             []Event
	hasEvent          bool
}

func (k *TestSwapHandleKeeper) PoolExist(_ cosmos.Context, asset common.Asset) bool {
	_, ok := k.pools[asset]
	return ok
}

func (k *TestSwapHandleKeeper) GetPool(_ cosmos.Context, asset common.Asset) (Pool, error) {
	return k.pools[asset], nil
}

func (k *TestSwapHandleKeeper) SetPool(_ cosmos.Context, pool Pool) error {
	k.pools[pool.Asset] = pool
	return nil
}

// IsActiveObserver see whether it is an active observer
func (k *TestSwapHandleKeeper) IsActiveObserver(_ cosmos.Context, addr cosmos.AccAddress) bool {
	return k.activeNodeAccount.NodeAddress.Equals(addr)
}

func (k *TestSwapHandleKeeper) GetNodeAccount(_ cosmos.Context, addr cosmos.AccAddress) (NodeAccount, error) {
	if k.activeNodeAccount.NodeAddress.Equals(addr) {
		return k.activeNodeAccount, nil
	}
	return NodeAccount{}, errors.New("not exist")
}

func (k *TestSwapHandleKeeper) AddToLiquidityFees(_ cosmos.Context, _ common.Asset, _ cosmos.Uint) error {
	return nil
}

func (k *TestSwapHandleKeeper) UpsertEvent(ctx cosmos.Context, event Event) error {
	k.event = append(k.event, event)
	return nil
}

func (k *TestSwapHandleKeeper) clearEvent() {
	k.event = nil
}

func (s *HandlerSwapSuite) TestHandle(c *C) {
	ctx, _ := setupKeeperForTest(c)
	keeper := &TestSwapHandleKeeper{
		pools:             make(map[common.Asset]Pool),
		activeNodeAccount: GetRandomNodeAccount(NodeActive),
	}

	mgr := NewManagers(keeper)
	c.Assert(mgr.BeginBlock(ctx), IsNil)
	mgr.txOutStore = NewTxStoreDummy()
	handler := NewSwapHandler(keeper, mgr)

	ver := constants.SWVersion
	constAccessor := constants.GetConstantValues(ver)
	txID := GetRandomTxHash()
	signerBNBAddr := GetRandomBNBAddress()
	observerAddr := keeper.activeNodeAccount.NodeAddress
	// no pool
	tx := common.NewTx(
		txID,
		signerBNBAddr,
		signerBNBAddr,
		common.Coins{
			common.NewCoin(common.RuneAsset(), cosmos.OneUint()),
		},
		BNBGasFeeSingleton,
		"",
	)
	keeper.clearEvent()
	msg := NewMsgSwap(tx, common.BNBAsset, signerBNBAddr, cosmos.ZeroUint(), observerAddr)
	_, err := handler.handle(ctx, msg, ver, constAccessor)
	c.Assert(err.Error(), Equals, errors.New("BNB.BNB pool doesn't exist").Error())
	c.Assert(keeper.event, IsNil)
	pool := NewPool()
	pool.Asset = common.BNBAsset
	pool.BalanceAsset = cosmos.NewUint(100 * common.One)
	pool.BalanceRune = cosmos.NewUint(100 * common.One)
	c.Assert(keeper.SetPool(ctx, pool), IsNil)
	keeper.clearEvent()
	// fund is not enough to pay for transaction fee
	_, err = handler.handle(ctx, msg, ver, constAccessor)
	c.Assert(err, NotNil)
	c.Assert(keeper.event, IsNil)

	tx = common.NewTx(txID, signerBNBAddr, signerBNBAddr,
		common.Coins{
			common.NewCoin(common.RuneAsset(), cosmos.NewUint(2*common.One)),
		},
		BNBGasFeeSingleton,
		"",
	)
	keeper.clearEvent()
	msgSwapPriceProtection := NewMsgSwap(tx, common.BNBAsset, signerBNBAddr, cosmos.NewUint(2*common.One), observerAddr)
	_, err = handler.handle(ctx, msgSwapPriceProtection, ver, constAccessor)
	c.Assert(err.Error(), Equals, errors.New("emit asset 192233756 less than price limit 200000000").Error())
	c.Assert(keeper.event, IsNil)

	poolTCAN := NewPool()
	tCanAsset, err := common.NewAsset("BNB.TCAN-014")
	c.Assert(err, IsNil)
	poolTCAN.Asset = tCanAsset
	poolTCAN.BalanceAsset = cosmos.NewUint(334850000)
	poolTCAN.BalanceRune = cosmos.NewUint(2349500000)
	c.Assert(keeper.SetPool(ctx, poolTCAN), IsNil)

	m, err := ParseMemo("swap:BNB.RUNE-B1A:bnb18jtza8j86hfyuj2f90zec0g5gvjh823e5psn2u:124958592")
	txIn := NewObservedTx(
		common.NewTx(GetRandomTxHash(), signerBNBAddr, GetRandomBNBAddress(),
			common.Coins{
				common.NewCoin(tCanAsset, cosmos.NewUint(20000000)),
			},
			BNBGasFeeSingleton,
			"swap:BNB.RUNE-B1A:bnb18jtza8j86hfyuj2f90zec0g5gvjh823e5psn2u:124958592",
		),
		1,
		GetRandomPubKey(),
	)
	msgSwapFromTxIn, err := getMsgSwapFromMemo(m.(SwapMemo), txIn, observerAddr)
	c.Assert(err, IsNil)
	keeper.clearEvent()
	items, err := mgr.TxOutStore().GetOutboundItems(ctx)
	c.Assert(err, IsNil)
	c.Assert(items, HasLen, 0)
	_, err = handler.handle(ctx, msgSwapFromTxIn.(MsgSwap), ver, constAccessor)
	c.Assert(err, IsNil)
	c.Assert(keeper.event, NotNil)
	items, err = mgr.TxOutStore().GetOutboundItems(ctx)
	c.Assert(err, IsNil)
	c.Assert(items, HasLen, 1)
}

func (s *HandlerSwapSuite) TestDoubleSwap(c *C) {
	ctx, _ := setupKeeperForTest(c)
	keeper := &TestSwapHandleKeeper{
		pools:             make(map[common.Asset]Pool),
		activeNodeAccount: GetRandomNodeAccount(NodeActive),
	}
	ver := constants.SWVersion
	mgr := NewManagers(keeper)
	c.Assert(mgr.BeginBlock(ctx), IsNil)
	mgr.txOutStore = NewTxStoreDummy()
	handler := NewSwapHandler(keeper, mgr)
	constAccessor := constants.GetConstantValues(ver)

	pool := NewPool()
	pool.Asset = common.BNBAsset
	pool.BalanceAsset = cosmos.NewUint(100 * common.One)
	pool.BalanceRune = cosmos.NewUint(100 * common.One)
	c.Assert(keeper.SetPool(ctx, pool), IsNil)

	poolTCAN := NewPool()
	tCanAsset, err := common.NewAsset("BNB.TCAN-014")
	c.Assert(err, IsNil)
	poolTCAN.Asset = tCanAsset
	poolTCAN.BalanceAsset = cosmos.NewUint(334850000)
	poolTCAN.BalanceRune = cosmos.NewUint(2349500000)
	c.Assert(keeper.SetPool(ctx, poolTCAN), IsNil)

	signerBNBAddr := GetRandomBNBAddress()
	observerAddr := keeper.activeNodeAccount.NodeAddress

	// double swap - happy path
	m, err := ParseMemo("swap:BNB.BNB:bnb18jtza8j86hfyuj2f90zec0g5gvjh823e5psn2u")
	txIn := NewObservedTx(
		common.NewTx(GetRandomTxHash(), signerBNBAddr, GetRandomBNBAddress(),
			common.Coins{
				common.NewCoin(tCanAsset, cosmos.NewUint(20000000)),
			},
			BNBGasFeeSingleton,
			"swap:BNB.BNB:bnb18jtza8j86hfyuj2f90zec0g5gvjh823e5psn2u",
		),
		1,
		GetRandomPubKey(),
	)
	msgSwapFromTxIn, err := getMsgSwapFromMemo(m.(SwapMemo), txIn, observerAddr)
	c.Assert(err, IsNil)

	items, err := mgr.TxOutStore().GetOutboundItems(ctx)
	c.Assert(err, IsNil)
	c.Assert(items, HasLen, 0)

	_, err = handler.handle(ctx, msgSwapFromTxIn.(MsgSwap), ver, constAccessor)
	c.Assert(err, IsNil)
	c.Assert(keeper.event, NotNil)
	c.Assert(len(keeper.event), Equals, 2)

	items, err = mgr.TxOutStore().GetOutboundItems(ctx)
	c.Assert(err, IsNil)
	c.Assert(items, HasLen, 1)
	keeper.clearEvent()
	// double swap , RUNE not enough to pay for transaction fee

	m1, err := ParseMemo("swap:BNB.BNB:bnb18jtza8j86hfyuj2f90zec0g5gvjh823e5psn2u")
	txIn1 := NewObservedTx(
		common.NewTx(GetRandomTxHash(), signerBNBAddr, GetRandomBNBAddress(),
			common.Coins{
				common.NewCoin(tCanAsset, cosmos.NewUint(10000000)),
			},
			BNBGasFeeSingleton,
			"swap:BNB.BNB:bnb18jtza8j86hfyuj2f90zec0g5gvjh823e5psn2u",
		),
		1,
		GetRandomPubKey(),
	)
	msgSwapFromTxIn1, err := getMsgSwapFromMemo(m1.(SwapMemo), txIn1, observerAddr)
	c.Assert(err, IsNil)
	mgr.TxOutStore().ClearOutboundItems(ctx)
	_, err = handler.handle(ctx, msgSwapFromTxIn1.(MsgSwap), ver, constAccessor)
	c.Assert(err, Equals, errSwapFailNotEnoughFee)
	c.Assert(keeper.event, IsNil)

	items, err = mgr.TxOutStore().GetOutboundItems(ctx)
	c.Assert(err, IsNil)
	c.Assert(items, HasLen, 0)
}
