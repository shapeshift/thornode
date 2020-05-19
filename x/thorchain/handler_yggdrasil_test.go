package thorchain

import (
	"github.com/blang/semver"

	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"

	. "gopkg.in/check.v1"
)

type HandlerYggdrasilSuite struct{}

var _ = Suite(&HandlerYggdrasilSuite{})

type yggdrasilTestKeeper struct {
	Keeper
	errGetVault        bool
	errGetAsgardVaults bool
	errGetNodeAccount  cosmos.AccAddress
	errGetPool         bool
}

func (k yggdrasilTestKeeper) GetAsgardVaultsByStatus(ctx cosmos.Context, vs VaultStatus) (Vaults, error) {
	if k.errGetAsgardVaults {
		return Vaults{}, kaboom
	}
	return k.Keeper.GetAsgardVaultsByStatus(ctx, vs)
}

func (k yggdrasilTestKeeper) GetNodeAccountByPubKey(ctx cosmos.Context, pk common.PubKey) (NodeAccount, error) {
	addr, _ := pk.GetThorAddress()
	if k.errGetNodeAccount.Equals(addr) {
		return NodeAccount{}, kaboom
	}
	return k.Keeper.GetNodeAccountByPubKey(ctx, pk)
}

func (k *yggdrasilTestKeeper) SetNodeAccount(ctx cosmos.Context, na NodeAccount) error {
	return k.Keeper.SetNodeAccount(ctx, na)
}

func (k yggdrasilTestKeeper) GetPool(ctx cosmos.Context, asset common.Asset) (Pool, error) {
	if k.errGetPool {
		return Pool{}, kaboom
	}
	return k.Keeper.GetPool(ctx, asset)
}

func (k *yggdrasilTestKeeper) SetPool(ctx cosmos.Context, p Pool) error {
	return k.Keeper.SetPool(ctx, p)
}

func (k *yggdrasilTestKeeper) UpsertEvent(ctx cosmos.Context, evt Event) error {
	return k.Keeper.UpsertEvent(ctx, evt)
}

func (k yggdrasilTestKeeper) GetVault(ctx cosmos.Context, pk common.PubKey) (Vault, error) {
	if k.errGetVault {
		return Vault{}, kaboom
	}
	return k.Keeper.GetVault(ctx, pk)
}

type yggdrasilHandlerTestHelper struct {
	ctx           cosmos.Context
	pool          Pool
	version       semver.Version
	keeper        *yggdrasilTestKeeper
	asgardVault   Vault
	yggVault      Vault
	constAccessor constants.ConstantValues
	nodeAccount   NodeAccount
	txOutStore    VersionedTxOutStore
	validatorMgr  VersionedValidatorManager
}

func newYggdrasilTestKeeper(keeper Keeper) *yggdrasilTestKeeper {
	return &yggdrasilTestKeeper{
		Keeper: keeper,
	}
}

func newYggdrasilHandlerTestHelper(c *C) yggdrasilHandlerTestHelper {
	ctx, k := setupKeeperForTest(c)
	ctx = ctx.WithBlockHeight(1023)

	version := constants.SWVersion
	keeper := newYggdrasilTestKeeper(k)

	// test pool
	pool := NewPool()
	pool.Asset = common.BNBAsset
	pool.BalanceAsset = cosmos.NewUint(100 * common.One)
	pool.BalanceRune = cosmos.NewUint(100 * common.One)
	c.Assert(keeper.SetPool(ctx, pool), IsNil)

	// active account
	nodeAccount := GetRandomNodeAccount(NodeActive)
	nodeAccount.Bond = cosmos.NewUint(100 * common.One)
	c.Assert(keeper.SetNodeAccount(ctx, nodeAccount), IsNil)

	constAccessor := constants.GetConstantValues(version)

	versionedTxOutStoreDummy := NewVersionedTxOutStoreDummy()
	versionedVaultMgrDummy := NewVersionedVaultMgrDummy(versionedTxOutStoreDummy)
	versionedEventManagerDummy := NewDummyVersionedEventMgr()

	validatorMgr := NewVersionedValidatorMgr(keeper, versionedTxOutStoreDummy, versionedVaultMgrDummy, versionedEventManagerDummy)
	c.Assert(validatorMgr.BeginBlock(ctx, version, constAccessor), IsNil)
	asgardVault := GetRandomVault()
	asgardVault.Type = AsgardVault
	asgardVault.Status = ActiveVault
	c.Assert(keeper.SetVault(ctx, asgardVault), IsNil)
	yggdrasilVault := GetRandomVault()
	yggdrasilVault.PubKey = nodeAccount.PubKeySet.Secp256k1
	yggdrasilVault.Type = YggdrasilVault
	yggdrasilVault.Status = ActiveVault
	c.Assert(keeper.SetVault(ctx, yggdrasilVault), IsNil)

	return yggdrasilHandlerTestHelper{
		ctx:           ctx,
		version:       version,
		keeper:        keeper,
		nodeAccount:   nodeAccount,
		constAccessor: constAccessor,
		txOutStore:    versionedTxOutStoreDummy,
		validatorMgr:  validatorMgr,
		asgardVault:   asgardVault,
		yggVault:      yggdrasilVault,
	}
}

func (s *HandlerYggdrasilSuite) TestYggdrasilHandler(c *C) {
	testCases := []struct {
		name           string
		messageCreator func(helper yggdrasilHandlerTestHelper) cosmos.Msg
		runner         func(handler YggdrasilHandler, msg cosmos.Msg, helper yggdrasilHandlerTestHelper) cosmos.Result
		validator      func(helper yggdrasilHandlerTestHelper, msg cosmos.Msg, result cosmos.Result, c *C)
		expectedResult cosmos.CodeType
	}{
		{
			name: "invalid message should return error",
			messageCreator: func(helper yggdrasilHandlerTestHelper) cosmos.Msg {
				return NewMsgNoOp(GetRandomObservedTx(), helper.nodeAccount.NodeAddress)
			},
			runner: func(handler YggdrasilHandler, msg cosmos.Msg, helper yggdrasilHandlerTestHelper) cosmos.Result {
				return handler.Run(helper.ctx, msg, helper.version, helper.constAccessor)
			},
			expectedResult: CodeInvalidMessage,
		},
		{
			name: "bad version should return an error",
			messageCreator: func(helper yggdrasilHandlerTestHelper) cosmos.Msg {
				return NewMsgYggdrasil(GetRandomTx(), GetRandomPubKey(), 12, false, common.Coins{common.NewCoin(common.BNBAsset, cosmos.OneUint())}, helper.nodeAccount.NodeAddress)
			},
			runner: func(handler YggdrasilHandler, msg cosmos.Msg, helper yggdrasilHandlerTestHelper) cosmos.Result {
				return handler.Run(helper.ctx, msg, semver.MustParse("0.0.1"), helper.constAccessor)
			},
			expectedResult: CodeBadVersion,
		},
		{
			name: "empty pubkey should return an error",
			messageCreator: func(helper yggdrasilHandlerTestHelper) cosmos.Msg {
				return NewMsgYggdrasil(GetRandomTx(), "", 12, false, common.Coins{common.NewCoin(common.BNBAsset, cosmos.OneUint())}, GetRandomBech32Addr())
			},
			runner: func(handler YggdrasilHandler, msg cosmos.Msg, helper yggdrasilHandlerTestHelper) cosmos.Result {
				return handler.Run(helper.ctx, msg, constants.SWVersion, helper.constAccessor)
			},
			expectedResult: cosmos.CodeUnknownRequest,
		},
		{
			name: "empty tx should return an error",
			messageCreator: func(helper yggdrasilHandlerTestHelper) cosmos.Msg {
				return NewMsgYggdrasil(common.Tx{}, GetRandomPubKey(), 12, false, common.Coins{common.NewCoin(common.BNBAsset, cosmos.OneUint())}, GetRandomBech32Addr())
			},
			runner: func(handler YggdrasilHandler, msg cosmos.Msg, helper yggdrasilHandlerTestHelper) cosmos.Result {
				return handler.Run(helper.ctx, msg, constants.SWVersion, helper.constAccessor)
			},
			expectedResult: cosmos.CodeUnknownRequest,
		},
		{
			name: "invalid coin should return an error",
			messageCreator: func(helper yggdrasilHandlerTestHelper) cosmos.Msg {
				return NewMsgYggdrasil(GetRandomTx(), GetRandomPubKey(), 12, false, common.Coins{common.NewCoin(common.EmptyAsset, cosmos.OneUint())}, GetRandomBech32Addr())
			},
			runner: func(handler YggdrasilHandler, msg cosmos.Msg, helper yggdrasilHandlerTestHelper) cosmos.Result {
				return handler.Run(helper.ctx, msg, constants.SWVersion, helper.constAccessor)
			},
			expectedResult: cosmos.CodeInvalidCoins,
		},
		{
			name: "empty signer should return an error",
			messageCreator: func(helper yggdrasilHandlerTestHelper) cosmos.Msg {
				return NewMsgYggdrasil(GetRandomTx(), GetRandomPubKey(), 12, false, common.Coins{common.NewCoin(common.BNBAsset, cosmos.OneUint())}, cosmos.AccAddress{})
			},
			runner: func(handler YggdrasilHandler, msg cosmos.Msg, helper yggdrasilHandlerTestHelper) cosmos.Result {
				return handler.Run(helper.ctx, msg, constants.SWVersion, helper.constAccessor)
			},
			expectedResult: cosmos.CodeInvalidAddress,
		},
		{
			name: "fail to get yggdrasil vault should return an error",
			messageCreator: func(helper yggdrasilHandlerTestHelper) cosmos.Msg {
				return NewMsgYggdrasil(GetRandomTx(), GetRandomPubKey(), 12, false, common.Coins{common.NewCoin(common.BNBAsset, cosmos.OneUint())}, helper.nodeAccount.NodeAddress)
			},
			runner: func(handler YggdrasilHandler, msg cosmos.Msg, helper yggdrasilHandlerTestHelper) cosmos.Result {
				helper.keeper.errGetVault = true
				return handler.Run(helper.ctx, msg, constants.SWVersion, helper.constAccessor)
			},
			expectedResult: cosmos.CodeInternal,
		},
		{
			name: "asgard fund yggdrasil should return success",
			messageCreator: func(helper yggdrasilHandlerTestHelper) cosmos.Msg {
				return NewMsgYggdrasil(GetRandomTx(), helper.asgardVault.PubKey, 13, true, common.Coins{common.NewCoin(common.BNBAsset, cosmos.OneUint())}, helper.nodeAccount.NodeAddress)
			},
			runner: func(handler YggdrasilHandler, msg cosmos.Msg, helper yggdrasilHandlerTestHelper) cosmos.Result {
				return handler.Run(helper.ctx, msg, constants.SWVersion, helper.constAccessor)
			},
			expectedResult: cosmos.CodeOK,
		},
		{
			name: "yggdrasil received fund from asgard should return success",
			messageCreator: func(helper yggdrasilHandlerTestHelper) cosmos.Msg {
				return NewMsgYggdrasil(GetRandomTx(), helper.yggVault.PubKey, 13, true, common.Coins{common.NewCoin(common.BNBAsset, cosmos.OneUint())}, helper.nodeAccount.NodeAddress)
			},
			runner: func(handler YggdrasilHandler, msg cosmos.Msg, helper yggdrasilHandlerTestHelper) cosmos.Result {
				return handler.Run(helper.ctx, msg, constants.SWVersion, helper.constAccessor)
			},
			expectedResult: cosmos.CodeOK,
		},
		{
			name: "yggdrasil return fund to asgard but to address is not asgard should be slashed",
			messageCreator: func(helper yggdrasilHandlerTestHelper) cosmos.Msg {
				fromAddr, _ := helper.yggVault.PubKey.GetAddress(common.BNBChain)
				coins := common.Coins{
					common.NewCoin(common.BNBAsset, cosmos.NewUint(common.One)),
					common.NewCoin(common.RuneAsset(), cosmos.NewUint(common.One)),
				}
				tx := common.Tx{
					ID:          GetRandomTxHash(),
					Chain:       common.BNBChain,
					FromAddress: fromAddr,
					ToAddress:   GetRandomBNBAddress(),
					Coins:       coins,
					Gas:         BNBGasFeeSingleton,
					Memo:        "yggdrasil-:30",
				}
				return NewMsgYggdrasil(tx, helper.yggVault.PubKey, 12, false, coins, helper.nodeAccount.NodeAddress)
			},
			runner: func(handler YggdrasilHandler, msg cosmos.Msg, helper yggdrasilHandlerTestHelper) cosmos.Result {
				return handler.Run(helper.ctx, msg, constants.SWVersion, helper.constAccessor)
			},
			expectedResult: cosmos.CodeOK,
			validator: func(helper yggdrasilHandlerTestHelper, msg cosmos.Msg, result cosmos.Result, c *C) {
				expectedBond := helper.nodeAccount.Bond.Sub(cosmos.NewUint(603787879))
				na, err := helper.keeper.GetNodeAccount(helper.ctx, helper.nodeAccount.NodeAddress)
				c.Assert(err, IsNil)
				c.Assert(na.Bond.Equal(expectedBond), Equals, true, Commentf("%d/%d", na.Bond.Uint64(), expectedBond.Uint64()))
			},
		},
		{
			name: "fail to get asgard vaults should return an error",
			messageCreator: func(helper yggdrasilHandlerTestHelper) cosmos.Msg {
				return NewMsgYggdrasil(GetRandomTx(), helper.yggVault.PubKey, 12, false, common.Coins{common.NewCoin(common.BNBAsset, cosmos.OneUint())}, helper.nodeAccount.NodeAddress)
			},
			runner: func(handler YggdrasilHandler, msg cosmos.Msg, helper yggdrasilHandlerTestHelper) cosmos.Result {
				helper.keeper.errGetAsgardVaults = true
				return handler.Run(helper.ctx, msg, constants.SWVersion, helper.constAccessor)
			},
			expectedResult: cosmos.CodeInternal,
		},
		{
			name: "fail to get node accounts should return an error",
			messageCreator: func(helper yggdrasilHandlerTestHelper) cosmos.Msg {
				fromAddr, _ := helper.yggVault.PubKey.GetAddress(common.BNBChain)
				coins := common.Coins{
					common.NewCoin(common.BNBAsset, cosmos.NewUint(common.One)),
					common.NewCoin(common.RuneAsset(), cosmos.NewUint(common.One)),
				}
				tx := common.Tx{
					ID:          GetRandomTxHash(),
					Chain:       common.BNBChain,
					FromAddress: fromAddr,
					ToAddress:   GetRandomBNBAddress(),
					Coins:       coins,
					Gas:         BNBGasFeeSingleton,
					Memo:        "yggdrasil-:30",
				}
				return NewMsgYggdrasil(tx, GetRandomPubKey(), 12, false, coins, helper.nodeAccount.NodeAddress)
			},
			runner: func(handler YggdrasilHandler, msg cosmos.Msg, helper yggdrasilHandlerTestHelper) cosmos.Result {
				ygg := msg.(MsgYggdrasil)
				addr, _ := ygg.PubKey.GetThorAddress()
				helper.keeper.errGetNodeAccount = addr
				return handler.Run(helper.ctx, msg, constants.SWVersion, helper.constAccessor)
			},
			expectedResult: cosmos.CodeInternal,
		},
		{
			name: "yggdrasil return fund to asgard but fail to get pool should return an error",
			messageCreator: func(helper yggdrasilHandlerTestHelper) cosmos.Msg {
				fromAddr, _ := helper.yggVault.PubKey.GetAddress(common.BNBChain)
				coins := common.Coins{
					common.NewCoin(common.BNBAsset, cosmos.NewUint(common.One)),
					common.NewCoin(common.RuneAsset(), cosmos.NewUint(common.One)),
				}
				tx := common.Tx{
					ID:          GetRandomTxHash(),
					Chain:       common.BNBChain,
					FromAddress: fromAddr,
					ToAddress:   GetRandomBNBAddress(),
					Coins:       coins,
					Gas:         BNBGasFeeSingleton,
					Memo:        "yggdrasil-:30",
				}
				return NewMsgYggdrasil(tx, helper.yggVault.PubKey, 12, false, coins, helper.nodeAccount.NodeAddress)
			},
			runner: func(handler YggdrasilHandler, msg cosmos.Msg, helper yggdrasilHandlerTestHelper) cosmos.Result {
				helper.keeper.errGetPool = true
				return handler.Run(helper.ctx, msg, constants.SWVersion, helper.constAccessor)
			},
			expectedResult: cosmos.CodeInternal,
		},
		{
			name: "yggdrasil return fund to asgard should work",
			messageCreator: func(helper yggdrasilHandlerTestHelper) cosmos.Msg {
				fromAddr, _ := helper.yggVault.PubKey.GetAddress(common.BNBChain)
				coins := common.Coins{
					common.NewCoin(common.BNBAsset, cosmos.NewUint(common.One)),
					common.NewCoin(common.RuneAsset(), cosmos.NewUint(common.One)),
				}
				tx := common.Tx{
					ID:          GetRandomTxHash(),
					Chain:       common.BNBChain,
					FromAddress: fromAddr,
					ToAddress:   GetRandomBNBAddress(),
					Coins:       coins,
					Gas:         BNBGasFeeSingleton,
					Memo:        "yggdrasil-:30",
				}
				return NewMsgYggdrasil(tx, helper.asgardVault.PubKey, 12, false, coins, helper.nodeAccount.NodeAddress)
			},
			runner: func(handler YggdrasilHandler, msg cosmos.Msg, helper yggdrasilHandlerTestHelper) cosmos.Result {
				return handler.Run(helper.ctx, msg, constants.SWVersion, helper.constAccessor)
			},
			expectedResult: cosmos.CodeOK,
		},
		{
			name: "yggdrasil return fund and node account is not active should refund bond",
			messageCreator: func(helper yggdrasilHandlerTestHelper) cosmos.Msg {
				na := GetRandomNodeAccount(NodeStandby)
				helper.keeper.SetNodeAccount(helper.ctx, na)
				vault := NewVault(10, ActiveVault, YggdrasilVault, na.PubKeySet.Secp256k1, common.Chains{common.BNBChain})
				helper.keeper.SetVault(helper.ctx, vault)
				fromAddr, _ := vault.PubKey.GetAddress(common.BNBChain)
				coins := common.Coins{
					common.NewCoin(common.BNBAsset, cosmos.NewUint(common.One)),
					common.NewCoin(common.RuneAsset(), cosmos.NewUint(common.One)),
				}
				toAddr, _ := helper.asgardVault.PubKey.GetAddress(common.BNBChain)

				tx := common.Tx{
					ID:          GetRandomTxHash(),
					Chain:       common.BNBChain,
					FromAddress: fromAddr,
					ToAddress:   toAddr,
					Coins:       coins,
					Gas:         BNBGasFeeSingleton,
					Memo:        "yggdrasil-:30",
				}
				return NewMsgYggdrasil(tx, vault.PubKey, 12, false, coins, helper.nodeAccount.NodeAddress)
			},
			runner: func(handler YggdrasilHandler, msg cosmos.Msg, helper yggdrasilHandlerTestHelper) cosmos.Result {
				return handler.Run(helper.ctx, msg, constants.SWVersion, helper.constAccessor)
			},
			validator: func(helper yggdrasilHandlerTestHelper, msg cosmos.Msg, result cosmos.Result, c *C) {
				store, err := helper.txOutStore.GetTxOutStore(helper.ctx, helper.keeper, helper.version)
				c.Assert(err, IsNil)

				items, err := store.GetOutboundItems(helper.ctx)
				c.Assert(err, IsNil)
				if common.RuneAsset().Chain.Equals(common.THORChain) {
					c.Assert(items, HasLen, 0)
				} else {
					c.Assert(items, HasLen, 1)
				}
				yggMsg := msg.(MsgYggdrasil)
				yggVault, err := helper.keeper.GetVault(helper.ctx, yggMsg.PubKey)
				c.Assert(err, NotNil)
				c.Assert(len(yggVault.Type), Equals, 0)
				na, err := helper.keeper.GetNodeAccountByPubKey(helper.ctx, yggMsg.PubKey)
				c.Assert(err, IsNil)
				c.Assert(na.Status.String(), Equals, NodeDisabled.String())
			},
			expectedResult: cosmos.CodeOK,
		},
	}
	for _, tc := range testCases {
		helper := newYggdrasilHandlerTestHelper(c)
		handler := NewYggdrasilHandler(helper.keeper, helper.txOutStore, helper.validatorMgr, NewVersionedEventMgr())
		msg := tc.messageCreator(helper)
		result := tc.runner(handler, msg, helper)
		c.Assert(result.Code, Equals, tc.expectedResult, Commentf("name:%s", tc.name))
		if tc.validator != nil {
			tc.validator(helper, msg, result, c)
		}
	}
}
