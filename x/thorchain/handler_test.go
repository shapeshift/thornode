package thorchain

import (
	"errors"
	"fmt"
	"os"

	"github.com/blang/semver"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/simapp"
	"github.com/cosmos/cosmos-sdk/store"
	cosmostypes "github.com/cosmos/cosmos-sdk/types"
	se "github.com/cosmos/cosmos-sdk/types/errors"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	paramskeeper "github.com/cosmos/cosmos-sdk/x/params/keeper"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"
	"github.com/tendermint/tendermint/libs/log"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	dbm "github.com/tendermint/tm-db"
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/cmd"
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"

	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
	"gitlab.com/thorchain/thornode/x/thorchain/types"
)

var kaboom = errors.New("kaboom!!!!!")

type HandlerSuite struct{}

var _ = Suite(&HandlerSuite{})

func (s *HandlerSuite) SetUpSuite(*C) {
	SetupConfigForTest()
}

func FundModule(c *C, ctx cosmos.Context, k keeper.Keeper, name string, amt uint64) {
	coin := common.NewCoin(common.RuneNative, cosmos.NewUint(amt*common.One))
	err := k.MintToModule(ctx, ModuleName, coin)
	c.Assert(err, IsNil)
	err = k.SendFromModuleToModule(ctx, ModuleName, name, common.NewCoins(coin))
	c.Assert(err, IsNil)
}

func FundAccount(c *C, ctx cosmos.Context, k keeper.Keeper, addr cosmos.AccAddress, amt uint64) {
	coin := common.NewCoin(common.RuneNative, cosmos.NewUint(amt*common.One))
	err := k.MintToModule(ctx, ModuleName, coin)
	c.Assert(err, IsNil)
	err = k.SendFromModuleToAccount(ctx, ModuleName, addr, common.NewCoins(coin))
	c.Assert(err, IsNil)
}

// nolint: deadcode unused
// create a codec used only for testing
func makeTestCodec() *codec.LegacyAmino {
	return types.MakeTestCodec()
}

var (
	multiPerm    = "multiple permissions account"
	randomPerm   = "random permission"
	holder       = "holder"
	keyThorchain = cosmos.NewKVStoreKey(StoreKey)
)

func setupKeeperForTest(c *C) (cosmos.Context, keeper.Keeper) {
	cosmostypes.SetCoinDenomRegex(func() string {
		return cmd.DenomRegex
	})
	keyAcc := cosmos.NewKVStoreKey(authtypes.StoreKey)
	keyBank := cosmos.NewKVStoreKey(banktypes.StoreKey)
	keyParams := cosmos.NewKVStoreKey(paramstypes.StoreKey)
	tkeyParams := cosmos.NewTransientStoreKey(paramstypes.TStoreKey)

	db := dbm.NewMemDB()
	ms := store.NewCommitMultiStore(db)
	ms.MountStoreWithDB(keyAcc, cosmos.StoreTypeIAVL, db)
	ms.MountStoreWithDB(keyParams, cosmos.StoreTypeIAVL, db)
	ms.MountStoreWithDB(keyThorchain, cosmos.StoreTypeIAVL, db)
	ms.MountStoreWithDB(keyBank, cosmos.StoreTypeIAVL, db)
	ms.MountStoreWithDB(tkeyParams, cosmos.StoreTypeTransient, db)
	err := ms.LoadLatestVersion()
	c.Assert(err, IsNil)

	ctx := cosmos.NewContext(ms, tmproto.Header{ChainID: "thorchain"}, false, log.NewNopLogger())
	ctx = ctx.WithBlockHeight(18)
	legacyCodec := makeTestCodec()
	marshaler := simapp.MakeTestEncodingConfig().Marshaler

	pk := paramskeeper.NewKeeper(marshaler, legacyCodec, keyParams, tkeyParams)
	ak := authkeeper.NewAccountKeeper(marshaler, keyAcc, pk.Subspace(authtypes.ModuleName), authtypes.ProtoBaseAccount, map[string][]string{
		ModuleName:  {authtypes.Minter, authtypes.Burner},
		AsgardName:  {},
		BondName:    {},
		ReserveName: {},
	})

	bk := bankkeeper.NewBaseKeeper(marshaler, keyBank, ak, pk.Subspace(banktypes.ModuleName), nil)
	bk.SetSupply(ctx, banktypes.NewSupply(cosmos.Coins{
		cosmos.NewCoin(common.RuneAsset().Native(), cosmos.NewInt(200_000_000_00000000)),
	}))
	k := keeper.NewKeeper(marshaler, bk, ak, keyThorchain)
	FundModule(c, ctx, k, ModuleName, 1000000*common.One)
	FundModule(c, ctx, k, AsgardName, common.One)
	FundModule(c, ctx, k, ReserveName, 10000*common.One)
	k.SaveNetworkFee(ctx, common.BNBChain, NetworkFee{
		Chain:              common.BNBChain,
		TransactionSize:    1,
		TransactionFeeRate: 37500,
	})
	os.Setenv("NET", "mocknet")
	return ctx, k
}

type handlerTestWrapper struct {
	ctx                  cosmos.Context
	keeper               keeper.Keeper
	mgr                  Manager
	activeNodeAccount    NodeAccount
	notActiveNodeAccount NodeAccount
}

func getHandlerTestWrapper(c *C, height int64, withActiveNode, withActieBNBPool bool) handlerTestWrapper {
	return getHandlerTestWrapperWithVersion(c, height, withActiveNode, withActieBNBPool, GetCurrentVersion())
}

func getHandlerTestWrapperWithVersion(c *C, height int64, withActiveNode, withActieBNBPool bool, version semver.Version) handlerTestWrapper {
	ctx, k := setupKeeperForTest(c)
	ctx = ctx.WithBlockHeight(height)
	acc1 := GetRandomNodeAccount(NodeActive)
	acc1.Version = version.String()
	if withActiveNode {
		c.Assert(k.SetNodeAccount(ctx, acc1), IsNil)
	}
	if withActieBNBPool {
		p, err := k.GetPool(ctx, common.BNBAsset)
		c.Assert(err, IsNil)
		p.Asset = common.BNBAsset
		p.Status = PoolAvailable
		p.BalanceRune = cosmos.NewUint(100 * common.One)
		p.BalanceAsset = cosmos.NewUint(100 * common.One)
		p.PoolUnits = cosmos.NewUint(100 * common.One)
		c.Assert(k.SetPool(ctx, p), IsNil)
	}
	constAccessor := constants.GetConstantValues(version)
	mgr := NewManagers(k)
	c.Assert(mgr.BeginBlock(ctx), IsNil)

	FundModule(c, ctx, k, AsgardName, 100000000)

	c.Assert(mgr.ValidatorMgr().BeginBlock(ctx, constAccessor, nil), IsNil)

	return handlerTestWrapper{
		ctx:                  ctx,
		keeper:               k,
		mgr:                  mgr,
		activeNodeAccount:    acc1,
		notActiveNodeAccount: GetRandomNodeAccount(NodeDisabled),
	}
}

func (HandlerSuite) TestIsSignedByActiveNodeAccounts(c *C) {
	ctx, k := setupKeeperForTest(c)
	nodeAddr := GetRandomBech32Addr()
	c.Check(isSignedByActiveNodeAccounts(ctx, k, []cosmos.AccAddress{}), Equals, false)
	c.Check(isSignedByActiveNodeAccounts(ctx, k, []cosmos.AccAddress{nodeAddr}), Equals, false)
	nodeAccount1 := GetRandomNodeAccount(NodeWhiteListed)
	c.Assert(k.SetNodeAccount(ctx, nodeAccount1), IsNil)
	c.Check(isSignedByActiveNodeAccounts(ctx, k, []cosmos.AccAddress{nodeAccount1.NodeAddress}), Equals, false)
}

func (HandlerSuite) TestHandleTxInWithdrawLiquidityMemo(c *C) {
	w := getHandlerTestWrapper(c, 1, true, false)

	vault := GetRandomVault()
	vault.Coins = common.Coins{
		common.NewCoin(common.BNBAsset, cosmos.NewUint(100*common.One)),
		common.NewCoin(common.RuneAsset(), cosmos.NewUint(100*common.One)),
	}
	w.keeper.SetVault(w.ctx, vault)
	vaultAddr, err := vault.PubKey.GetAddress(common.BNBChain)

	pool := NewPool()
	pool.Asset = common.BNBAsset
	pool.BalanceAsset = cosmos.NewUint(100 * common.One)
	pool.BalanceRune = cosmos.NewUint(100 * common.One)
	pool.PoolUnits = cosmos.NewUint(100)
	c.Assert(w.keeper.SetPool(w.ctx, pool), IsNil)

	runeAddr := GetRandomRUNEAddress()
	lp := LiquidityProvider{
		Asset:        common.BNBAsset,
		RuneAddress:  runeAddr,
		AssetAddress: GetRandomBNBAddress(),
		PendingRune:  cosmos.ZeroUint(),
		Units:        cosmos.NewUint(100),
	}
	w.keeper.SetLiquidityProvider(w.ctx, lp)

	tx := common.Tx{
		ID:    GetRandomTxHash(),
		Chain: common.BNBChain,
		Coins: common.Coins{
			common.NewCoin(common.RuneAsset(), cosmos.NewUint(1*common.One)),
		},
		Memo:        "withdraw:BNB.BNB",
		FromAddress: lp.RuneAddress,
		ToAddress:   vaultAddr,
		Gas:         BNBGasFeeSingleton,
	}

	msg := NewMsgWithdrawLiquidity(tx, lp.RuneAddress, cosmos.NewUint(uint64(MaxWithdrawBasisPoints)), common.BNBAsset, common.EmptyAsset, w.activeNodeAccount.NodeAddress)
	c.Assert(err, IsNil)

	handler := NewInternalHandler(w.keeper, w.mgr)

	FundModule(c, w.ctx, w.keeper, AsgardName, 500)
	w.keeper.SaveNetworkFee(w.ctx, common.BNBChain, NetworkFee{
		Chain:              common.BNBChain,
		TransactionSize:    1,
		TransactionFeeRate: bnbSingleTxFee.Uint64(),
	})

	_, err = handler(w.ctx, msg)
	c.Assert(err, IsNil)
	pool, err = w.keeper.GetPool(w.ctx, common.BNBAsset)
	c.Assert(err, IsNil)
	c.Assert(pool.IsEmpty(), Equals, false)
	c.Check(pool.Status, Equals, PoolStaged)
	c.Check(pool.PoolUnits.Uint64(), Equals, uint64(0), Commentf("%d", pool.PoolUnits.Uint64()))
	c.Check(pool.BalanceRune.Uint64(), Equals, uint64(0), Commentf("%d", pool.BalanceRune.Uint64()))
	remainGas := uint64(37500)
	c.Check(pool.BalanceAsset.Uint64(), Equals, remainGas, Commentf("%d", pool.BalanceAsset.Uint64())) // leave a little behind for gas
}

func (HandlerSuite) TestRefund(c *C) {
	w := getHandlerTestWrapper(c, 1, true, false)

	pool := Pool{
		Asset:        common.BNBAsset,
		BalanceRune:  cosmos.NewUint(100 * common.One),
		BalanceAsset: cosmos.NewUint(100 * common.One),
	}
	c.Assert(w.keeper.SetPool(w.ctx, pool), IsNil)

	vault := GetRandomVault()
	c.Assert(w.keeper.SetVault(w.ctx, vault), IsNil)

	txin := NewObservedTx(
		common.Tx{
			ID:    GetRandomTxHash(),
			Chain: common.BNBChain,
			Coins: common.Coins{
				common.NewCoin(common.BNBAsset, cosmos.NewUint(100*common.One)),
			},
			Memo:        "withdraw:BNB.BNB",
			FromAddress: GetRandomBNBAddress(),
			ToAddress:   GetRandomBNBAddress(),
			Gas:         BNBGasFeeSingleton,
		},
		1024,
		vault.PubKey, 1024,
	)
	ver := GetCurrentVersion()
	constAccessor := constants.GetConstantValues(ver)
	txOutStore := w.mgr.TxOutStore()
	c.Assert(refundTxV1(w.ctx, txin, w.mgr, w.keeper, constAccessor, 0, "refund", ""), IsNil)
	items, err := txOutStore.GetOutboundItems(w.ctx)
	c.Assert(err, IsNil)
	c.Assert(items, HasLen, 1)

	// check THORNode DONT create a refund transaction when THORNode don't have a pool for
	// the asset sent.
	lokiAsset, _ := common.NewAsset(fmt.Sprintf("BNB.LOKI"))
	txin.Tx.Coins = common.Coins{
		common.NewCoin(lokiAsset, cosmos.NewUint(100*common.One)),
	}

	c.Assert(refundTxV1(w.ctx, txin, w.mgr, w.keeper, constAccessor, 0, "refund", ""), IsNil)
	items, err = txOutStore.GetOutboundItems(w.ctx)
	c.Assert(err, IsNil)
	c.Assert(items, HasLen, 1)

	pool, err = w.keeper.GetPool(w.ctx, lokiAsset)
	c.Assert(err, IsNil)
	// pool should be zero since we drop coins we don't recognize on the floor
	c.Assert(pool.BalanceAsset.Equal(cosmos.ZeroUint()), Equals, true, Commentf("%d", pool.BalanceAsset.Uint64()))

	// doing it a second time should keep it at zero
	c.Assert(refundTxV1(w.ctx, txin, w.mgr, w.keeper, constAccessor, 0, "refund", ""), IsNil)
	items, err = txOutStore.GetOutboundItems(w.ctx)
	c.Assert(err, IsNil)
	c.Assert(items, HasLen, 1)
	pool, err = w.keeper.GetPool(w.ctx, lokiAsset)
	c.Assert(err, IsNil)
	c.Assert(pool.BalanceAsset.Equal(cosmos.ZeroUint()), Equals, true)
}

func (HandlerSuite) TestGetMsgSwapFromMemo(c *C) {
	m, err := ParseMemo("swap:BNB.BNB")
	swapMemo, ok := m.(SwapMemo)
	c.Assert(ok, Equals, true)
	c.Assert(err, IsNil)

	txin := types.NewObservedTx(
		common.Tx{
			ID:    GetRandomTxHash(),
			Chain: common.BNBChain,
			Coins: common.Coins{
				common.NewCoin(
					common.RuneAsset(),
					cosmos.NewUint(100*common.One),
				),
			},
			Memo:        m.String(),
			FromAddress: GetRandomBNBAddress(),
			ToAddress:   GetRandomBNBAddress(),
			Gas:         BNBGasFeeSingleton,
		},
		1024,
		common.EmptyPubKey, 1024,
	)

	resultMsg1, err := getMsgSwapFromMemo(swapMemo, txin, GetRandomBech32Addr())
	c.Assert(resultMsg1, NotNil)
	c.Assert(err, IsNil)
}

func (HandlerSuite) TestGetMsgWithdrawFromMemo(c *C) {
	w := getHandlerTestWrapper(c, 1, true, false)
	tx := GetRandomTx()
	tx.Memo = "withdraw:10000"
	if common.RuneAsset().Equals(common.RuneNative) {
		tx.FromAddress = GetRandomTHORAddress()
	}
	obTx := NewObservedTx(tx, w.ctx.BlockHeight(), GetRandomPubKey(), w.ctx.BlockHeight())
	msg, err := processOneTxIn(w.ctx, w.keeper, obTx, w.activeNodeAccount.NodeAddress)
	c.Assert(err, IsNil)
	c.Assert(msg, NotNil)
	c.Assert(msg.Type(), Equals, MsgWithdrawLiquidity{}.Type())
}

func (HandlerSuite) TestGetMsgMigrationFromMemo(c *C) {
	w := getHandlerTestWrapper(c, 1, true, false)
	tx := GetRandomTx()
	tx.Memo = "migrate:10"
	obTx := NewObservedTx(tx, w.ctx.BlockHeight(), GetRandomPubKey(), w.ctx.BlockHeight())
	msg, err := processOneTxIn(w.ctx, w.keeper, obTx, w.activeNodeAccount.NodeAddress)
	c.Assert(err, IsNil)
	c.Assert(msg, NotNil)
	c.Assert(msg.Type(), Equals, MsgMigrate{}.Type())
}

func (HandlerSuite) TestGetMsgBondFromMemo(c *C) {
	w := getHandlerTestWrapper(c, 1, true, false)
	tx := GetRandomTx()
	tx.Coins = common.Coins{
		common.NewCoin(common.RuneAsset(), cosmos.NewUint(100*common.One)),
	}
	tx.Memo = "bond:" + GetRandomBech32Addr().String()
	obTx := NewObservedTx(tx, w.ctx.BlockHeight(), GetRandomPubKey(), w.ctx.BlockHeight())
	msg, err := processOneTxIn(w.ctx, w.keeper, obTx, w.activeNodeAccount.NodeAddress)
	c.Assert(err, IsNil)
	c.Assert(msg, NotNil)
	c.Assert(msg.Type(), Equals, MsgBond{}.Type())
}

func (HandlerSuite) TestGetMsgUnBondFromMemo(c *C) {
	w := getHandlerTestWrapper(c, 1, true, false)
	tx := GetRandomTx()
	tx.Coins = common.Coins{
		common.NewCoin(common.RuneAsset(), cosmos.NewUint(100*common.One)),
	}
	tx.Memo = "unbond:" + GetRandomTHORAddress().String() + ":1000"
	obTx := NewObservedTx(tx, w.ctx.BlockHeight(), GetRandomPubKey(), w.ctx.BlockHeight())
	msg, err := processOneTxIn(w.ctx, w.keeper, obTx, w.activeNodeAccount.NodeAddress)
	c.Assert(err, IsNil)
	c.Assert(msg, NotNil)
	c.Assert(msg.Type(), Equals, MsgUnBond{}.Type())
}

func (HandlerSuite) TestGetMsgLiquidityFromMemo(c *C) {
	w := getHandlerTestWrapper(c, 1, true, false)
	// provide BNB, however THORNode send T-CAN as coin , which is incorrect, should result in an error
	m, err := ParseMemo(fmt.Sprintf("add:BNB.BNB:%s", GetRandomRUNEAddress()))
	c.Assert(err, IsNil)
	lpMemo, ok := m.(AddLiquidityMemo)
	c.Assert(ok, Equals, true)
	tcanAsset, err := common.NewAsset("BNB.TCAN-014")
	c.Assert(err, IsNil)
	runeAsset := common.RuneAsset()
	c.Assert(err, IsNil)

	txin := types.NewObservedTx(
		common.Tx{
			ID:    GetRandomTxHash(),
			Chain: common.BNBChain,
			Coins: common.Coins{
				common.NewCoin(tcanAsset,
					cosmos.NewUint(100*common.One)),
				common.NewCoin(runeAsset,
					cosmos.NewUint(100*common.One)),
			},
			Memo:        m.String(),
			FromAddress: GetRandomBNBAddress(),
			ToAddress:   GetRandomBNBAddress(),
			Gas:         BNBGasFeeSingleton,
		},
		1024,
		common.EmptyPubKey, 1024,
	)

	msg, err := getMsgAddLiquidityFromMemo(w.ctx, lpMemo, txin, GetRandomBech32Addr())
	c.Assert(msg, NotNil)
	c.Assert(err, IsNil)

	// Asymentic liquidity provision should works fine, only RUNE
	txin.Tx.Coins = common.Coins{
		common.NewCoin(runeAsset,
			cosmos.NewUint(100*common.One)),
	}

	// provide only rune should be fine
	msg1, err1 := getMsgAddLiquidityFromMemo(w.ctx, lpMemo, txin, GetRandomBech32Addr())
	c.Assert(msg1, NotNil)
	c.Assert(err1, IsNil)

	bnbAsset, err := common.NewAsset("BNB.BNB")
	c.Assert(err, IsNil)
	txin.Tx.Coins = common.Coins{
		common.NewCoin(bnbAsset,
			cosmos.NewUint(100*common.One)),
	}

	// provide only token(BNB) should be fine
	msg2, err2 := getMsgAddLiquidityFromMemo(w.ctx, lpMemo, txin, GetRandomBech32Addr())
	c.Assert(msg2, NotNil)
	c.Assert(err2, IsNil)

	lokiAsset, _ := common.NewAsset(fmt.Sprintf("BNB.LOKI"))
	// Make sure the RUNE Address and Asset Address set correctly
	txin.Tx.Coins = common.Coins{
		common.NewCoin(runeAsset,
			cosmos.NewUint(100*common.One)),
		common.NewCoin(lokiAsset,
			cosmos.NewUint(100*common.One)),
	}

	runeAddr := GetRandomRUNEAddress()
	lokiAddLiquidityMemo, err := ParseMemo(fmt.Sprintf("add:BNB.LOKI:%s", runeAddr))
	c.Assert(err, IsNil)
	msg4, err4 := getMsgAddLiquidityFromMemo(w.ctx, lokiAddLiquidityMemo.(AddLiquidityMemo), txin, GetRandomBech32Addr())
	c.Assert(err4, IsNil)
	c.Assert(msg4, NotNil)
	msgAddLiquidity := msg4.(*MsgAddLiquidity)
	c.Assert(msgAddLiquidity, NotNil)
	c.Assert(msgAddLiquidity.RuneAddress, Equals, runeAddr)
	c.Assert(msgAddLiquidity.AssetAddress, Equals, txin.Tx.FromAddress)
}

func (HandlerSuite) TestMsgLeaveFromMemo(c *C) {
	w := getHandlerTestWrapper(c, 1, true, false)
	addr := types.GetRandomBech32Addr()
	txin := types.NewObservedTx(
		common.Tx{
			ID:          GetRandomTxHash(),
			Chain:       common.BNBChain,
			Coins:       common.Coins{common.NewCoin(common.RuneAsset(), cosmos.NewUint(1))},
			Memo:        fmt.Sprintf("LEAVE:%s", addr.String()),
			FromAddress: GetRandomBNBAddress(),
			ToAddress:   GetRandomBNBAddress(),
			Gas:         BNBGasFeeSingleton,
		},
		1024,
		common.EmptyPubKey, 1024,
	)

	msg, err := processOneTxIn(w.ctx, w.keeper, txin, addr)
	c.Assert(err, IsNil)
	c.Check(msg.ValidateBasic(), IsNil)
}

func (HandlerSuite) TestYggdrasilMemo(c *C) {
	w := getHandlerTestWrapper(c, 1, true, false)
	addr := types.GetRandomBech32Addr()
	txin := types.NewObservedTx(
		common.Tx{
			ID:          GetRandomTxHash(),
			Chain:       common.BNBChain,
			Coins:       common.Coins{common.NewCoin(common.RuneAsset(), cosmos.NewUint(1))},
			Memo:        "yggdrasil+:1024",
			FromAddress: GetRandomBNBAddress(),
			ToAddress:   GetRandomBNBAddress(),
			Gas:         BNBGasFeeSingleton,
		},
		1024,
		GetRandomPubKey(), 1024,
	)

	msg, err := processOneTxIn(w.ctx, w.keeper, txin, addr)
	c.Assert(err, IsNil)
	c.Check(msg.ValidateBasic(), IsNil)

	txin.Tx.Memo = "yggdrasil-:1024"
	msg, err = processOneTxIn(w.ctx, w.keeper, txin, addr)
	c.Assert(err, IsNil)
	c.Check(msg.ValidateBasic(), IsNil)
}

func (s *HandlerSuite) TestReserveContributor(c *C) {
	w := getHandlerTestWrapper(c, 1, true, false)
	addr := types.GetRandomBech32Addr()
	txin := types.NewObservedTx(
		common.Tx{
			ID:          GetRandomTxHash(),
			Chain:       common.BNBChain,
			Coins:       common.Coins{common.NewCoin(common.RuneAsset(), cosmos.NewUint(1))},
			Memo:        "reserve",
			FromAddress: GetRandomBNBAddress(),
			ToAddress:   GetRandomBNBAddress(),
			Gas:         BNBGasFeeSingleton,
		},
		1024,
		GetRandomPubKey(), 1024,
	)

	msg, err := processOneTxIn(w.ctx, w.keeper, txin, addr)
	c.Assert(err, IsNil)
	c.Check(msg.ValidateBasic(), IsNil)
	c.Check(msg.Type(), Equals, MsgReserveContributor{}.Type())
}

func (s *HandlerSuite) TestSwitch(c *C) {
	w := getHandlerTestWrapper(c, 1, true, false)
	addr := types.GetRandomBech32Addr()
	txin := types.NewObservedTx(
		common.Tx{
			ID:          GetRandomTxHash(),
			Chain:       common.BNBChain,
			Coins:       common.Coins{common.NewCoin(common.RuneAsset(), cosmos.NewUint(1))},
			Memo:        "switch:" + GetRandomBech32Addr().String(),
			FromAddress: GetRandomBNBAddress(),
			ToAddress:   GetRandomBNBAddress(),
			Gas:         BNBGasFeeSingleton,
		},
		1024,
		GetRandomPubKey(), 1024,
	)

	msg, err := processOneTxIn(w.ctx, w.keeper, txin, addr)
	c.Assert(err, IsNil)
	c.Check(msg.ValidateBasic(), IsNil)
	c.Check(msg.Type(), Equals, MsgSwitch{}.Type())
}

func (s *HandlerSuite) TestExternalHandler(c *C) {
	ctx, k := setupKeeperForTest(c)
	mgr := NewManagers(k)
	mgr.BeginBlock(ctx)
	handler := NewExternalHandler(k, mgr)
	ctx = ctx.WithBlockHeight(1024)
	msg := NewMsgNetworkFee(1024, common.BNBChain, 1, bnbSingleTxFee.Uint64(), GetRandomBech32Addr())
	result, err := handler(ctx, msg)
	c.Check(err, NotNil)
	c.Check(errors.Is(err, se.ErrUnauthorized), Equals, true)
	c.Check(result, IsNil)
	na := GetRandomNodeAccount(NodeActive)
	k.SetNodeAccount(ctx, na)
	FundAccount(c, ctx, k, na.NodeAddress, 10*common.One)
	result, err = handler(ctx, NewMsgSetVersion("0.1.0", na.NodeAddress))
	c.Assert(err, IsNil)
	c.Assert(result, NotNil)
}

func (s *HandlerSuite) TestFuzzyMatching(c *C) {
	ctx, k := setupKeeperForTest(c)
	p1 := NewPool()
	p1.Asset = common.BNBAsset
	p1.BalanceRune = cosmos.NewUint(10 * common.One)
	c.Assert(k.SetPool(ctx, p1), IsNil)

	// real USDT
	p2 := NewPool()
	p2.Asset, _ = common.NewAsset("ETH.USDT-0XDAC17F958D2EE523A2206206994597C13D831EC7")
	p2.BalanceRune = cosmos.NewUint(80 * common.One)
	c.Assert(k.SetPool(ctx, p2), IsNil)

	// fake USDT, attempt to clone end of contract address
	p3 := NewPool()
	p3.Asset, _ = common.NewAsset("ETH.USDT-0XD084B83C305DAFD76AE3E1B4E1F1FE213D831EC7")
	p3.BalanceRune = cosmos.NewUint(20 * common.One)
	c.Assert(k.SetPool(ctx, p3), IsNil)

	// fake USDT, bad contract address
	p4 := NewPool()
	p4.Asset, _ = common.NewAsset("ETH.USDT-0XD084B83C305DAFD76AE3E1B4E1F1FE2ECCCB3988")
	p4.BalanceRune = cosmos.NewUint(20 * common.One)
	c.Assert(k.SetPool(ctx, p4), IsNil)

	// fake USDT, on different chain
	p5 := NewPool()
	p5.Asset, _ = common.NewAsset("BSC.USDT-0XDAC17F958D2EE523A2206206994597C13D831EC7")
	p5.BalanceRune = cosmos.NewUint(30 * common.One)
	c.Assert(k.SetPool(ctx, p5), IsNil)

	// fake USDT, right contract address, wrong ticker
	p6 := NewPool()
	p6.Asset, _ = common.NewAsset("ETH.UST-0XDAC17F958D2EE523A2206206994597C13D831EC7")
	p6.BalanceRune = cosmos.NewUint(90 * common.One)
	c.Assert(k.SetPool(ctx, p6), IsNil)

	result := fuzzyAssetMatch(ctx, k, p1.Asset)
	c.Check(result.Equals(p1.Asset), Equals, true)
	result = fuzzyAssetMatch(ctx, k, p6.Asset)
	c.Check(result.Equals(p6.Asset), Equals, true)

	check, _ := common.NewAsset("ETH.USDT")
	result = fuzzyAssetMatch(ctx, k, check)
	c.Check(result.Equals(p2.Asset), Equals, true)
	check, _ = common.NewAsset("ETH.USDT-")
	result = fuzzyAssetMatch(ctx, k, check)
	c.Check(result.Equals(p2.Asset), Equals, true)
	check, _ = common.NewAsset("ETH.USDT-1EC7")
	result = fuzzyAssetMatch(ctx, k, check)
	c.Check(result.Equals(p2.Asset), Equals, true)
}
