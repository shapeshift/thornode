package thorchain

import (
	"github.com/blang/semver"
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/tss/go-tss/blame"

	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

type HandlerTssKeysignSuite struct{}

var _ = Suite(&HandlerTssKeysignSuite{})

type tssKeysignFailHandlerTestHelper struct {
	ctx           cosmos.Context
	version       semver.Version
	keeper        *tssKeysignKeeperHelper
	constAccessor constants.ConstantValues
	nodeAccount   NodeAccount
	vaultManager  VersionedVaultManager
	members       common.PubKeys
	blame         blame.Blame
}

type tssKeysignKeeperHelper struct {
	Keeper
	errListActiveAccounts           bool
	errGetTssVoter                  bool
	errFailToGetNodeAccountByPubKey bool
	errFailSetNodeAccount           bool
}

func newTssKeysignFailKeeperHelper(keeper Keeper) *tssKeysignKeeperHelper {
	return &tssKeysignKeeperHelper{
		Keeper: keeper,
	}
}

func (k *tssKeysignKeeperHelper) GetNodeAccountByPubKey(ctx cosmos.Context, pk common.PubKey) (NodeAccount, error) {
	if k.errFailToGetNodeAccountByPubKey {
		return NodeAccount{}, kaboom
	}
	return k.Keeper.GetNodeAccountByPubKey(ctx, pk)
}

func (k *tssKeysignKeeperHelper) SetNodeAccount(ctx cosmos.Context, na NodeAccount) error {
	if k.errFailSetNodeAccount {
		return kaboom
	}
	return k.Keeper.SetNodeAccount(ctx, na)
}

func (k *tssKeysignKeeperHelper) GetTssKeysignFailVoter(ctx cosmos.Context, id string) (TssKeysignFailVoter, error) {
	if k.errGetTssVoter {
		return TssKeysignFailVoter{}, kaboom
	}
	return k.Keeper.GetTssKeysignFailVoter(ctx, id)
}

func (k *tssKeysignKeeperHelper) ListActiveNodeAccounts(ctx cosmos.Context) (NodeAccounts, error) {
	if k.errListActiveAccounts {
		return NodeAccounts{}, kaboom
	}
	return k.Keeper.ListActiveNodeAccounts(ctx)
}

func newTssKeysignHandlerTestHelper(c *C) tssKeysignFailHandlerTestHelper {
	ctx, k := setupKeeperForTest(c)
	ctx = ctx.WithBlockHeight(1023)
	version := constants.SWVersion
	keeper := newTssKeysignFailKeeperHelper(k)
	// active account
	nodeAccount := GetRandomNodeAccount(NodeActive)
	nodeAccount.Bond = cosmos.NewUint(100 * common.One)
	c.Assert(keeper.SetNodeAccount(ctx, nodeAccount), IsNil)
	constAccessor := constants.GetConstantValues(version)
	versionedEventManagerDummy := NewDummyVersionedEventMgr()
	versionedTxOutStore := NewVersionedTxOutStore(versionedEventManagerDummy)

	vaultMgr := NewVersionedVaultMgr(versionedTxOutStore, versionedEventManagerDummy)
	var members []blame.Node
	for i := 0; i < 8; i++ {
		na := GetRandomNodeAccount(NodeStandby)
		members = append(members, blame.Node{Pubkey: na.PubKeySet.Secp256k1.String()})
		_ = keeper.SetNodeAccount(ctx, na)
	}
	blame := blame.Blame{
		FailReason: "whatever",
		BlameNodes: members,
	}
	asgardVault := NewVault(ctx.BlockHeight(), ActiveVault, AsgardVault, GetRandomPubKey(), common.Chains{common.BNBChain})
	c.Assert(keeper.SetVault(ctx, asgardVault), IsNil)
	return tssKeysignFailHandlerTestHelper{
		ctx:           ctx,
		version:       version,
		keeper:        keeper,
		constAccessor: constAccessor,
		nodeAccount:   nodeAccount,
		vaultManager:  vaultMgr,
		blame:         blame,
	}
}

func (h HandlerTssKeysignSuite) TestTssKeysignFailHandler(c *C) {
	testCases := []struct {
		name           string
		messageCreator func(helper tssKeysignFailHandlerTestHelper) cosmos.Msg
		runner         func(handler TssKeysignHandler, msg cosmos.Msg, helper tssKeysignFailHandlerTestHelper) cosmos.Result
		validator      func(helper tssKeysignFailHandlerTestHelper, msg cosmos.Msg, result cosmos.Result, c *C)
		expectedResult cosmos.CodeType
	}{
		{
			name: "invalid message should return an error",
			messageCreator: func(helper tssKeysignFailHandlerTestHelper) cosmos.Msg {
				return NewMsgNoOp(GetRandomObservedTx(), helper.nodeAccount.NodeAddress)
			},
			runner: func(handler TssKeysignHandler, msg cosmos.Msg, helper tssKeysignFailHandlerTestHelper) cosmos.Result {
				return handler.Run(helper.ctx, msg, helper.version, helper.constAccessor)
			},
			expectedResult: CodeInvalidMessage,
		},
		{
			name: "bad version should return an error",
			messageCreator: func(helper tssKeysignFailHandlerTestHelper) cosmos.Msg {
				return NewMsgTssKeysignFail(helper.ctx.BlockHeight(), helper.blame, "hello", common.Coins{common.NewCoin(common.BNBAsset, cosmos.NewUint(100))}, helper.nodeAccount.NodeAddress)
			},
			runner: func(handler TssKeysignHandler, msg cosmos.Msg, helper tssKeysignFailHandlerTestHelper) cosmos.Result {
				return handler.Run(helper.ctx, msg, semver.MustParse("0.0.1"), helper.constAccessor)
			},
			expectedResult: CodeBadVersion,
		},
		{
			name: "Not signed by an active account should return an error",
			messageCreator: func(helper tssKeysignFailHandlerTestHelper) cosmos.Msg {
				return NewMsgTssKeysignFail(helper.ctx.BlockHeight(), helper.blame, "hello", common.Coins{common.NewCoin(common.BNBAsset, cosmos.NewUint(100))}, GetRandomBech32Addr())
			},
			runner: func(handler TssKeysignHandler, msg cosmos.Msg, helper tssKeysignFailHandlerTestHelper) cosmos.Result {
				return handler.Run(helper.ctx, msg, constants.SWVersion, helper.constAccessor)
			},
			expectedResult: cosmos.CodeUnauthorized,
		},
		{
			name: "empty signer should return an error",
			messageCreator: func(helper tssKeysignFailHandlerTestHelper) cosmos.Msg {
				return NewMsgTssKeysignFail(helper.ctx.BlockHeight(), helper.blame, "hello", common.Coins{common.NewCoin(common.BNBAsset, cosmos.NewUint(100))}, cosmos.AccAddress{})
			},
			runner: func(handler TssKeysignHandler, msg cosmos.Msg, helper tssKeysignFailHandlerTestHelper) cosmos.Result {
				return handler.Run(helper.ctx, msg, constants.SWVersion, helper.constAccessor)
			},
			expectedResult: cosmos.CodeInvalidAddress,
		},
		{
			name: "empty id should return an error",
			messageCreator: func(helper tssKeysignFailHandlerTestHelper) cosmos.Msg {
				tssMsg := NewMsgTssKeysignFail(helper.ctx.BlockHeight(), helper.blame, "hello", common.Coins{common.NewCoin(common.BNBAsset, cosmos.NewUint(100))}, helper.nodeAccount.NodeAddress)
				tssMsg.ID = ""
				return tssMsg
			},
			runner: func(handler TssKeysignHandler, msg cosmos.Msg, helper tssKeysignFailHandlerTestHelper) cosmos.Result {
				return handler.Run(helper.ctx, msg, constants.SWVersion, helper.constAccessor)
			},
			expectedResult: cosmos.CodeUnknownRequest,
		},
		{
			name: "empty member pubkeys should return an error",
			messageCreator: func(helper tssKeysignFailHandlerTestHelper) cosmos.Msg {
				return NewMsgTssKeysignFail(helper.ctx.BlockHeight(), blame.Blame{
					FailReason: "",
					BlameNodes: []blame.Node{},
				}, "hello", common.Coins{common.NewCoin(common.BNBAsset, cosmos.NewUint(100))}, helper.nodeAccount.NodeAddress)
			},
			runner: func(handler TssKeysignHandler, msg cosmos.Msg, helper tssKeysignFailHandlerTestHelper) cosmos.Result {
				return handler.Run(helper.ctx, msg, constants.SWVersion, helper.constAccessor)
			},
			expectedResult: cosmos.CodeUnknownRequest,
		},
		{
			name: "normal blame should works fine",
			messageCreator: func(helper tssKeysignFailHandlerTestHelper) cosmos.Msg {
				return NewMsgTssKeysignFail(helper.ctx.BlockHeight(), helper.blame, "hello", common.Coins{common.NewCoin(common.BNBAsset, cosmos.NewUint(100))}, helper.nodeAccount.NodeAddress)
			},
			runner: func(handler TssKeysignHandler, msg cosmos.Msg, helper tssKeysignFailHandlerTestHelper) cosmos.Result {
				return handler.Run(helper.ctx, msg, constants.SWVersion, helper.constAccessor)
			},
			expectedResult: cosmos.CodeOK,
		},
		{
			name: "fail to list active node accounts should return an error",
			messageCreator: func(helper tssKeysignFailHandlerTestHelper) cosmos.Msg {
				return NewMsgTssKeysignFail(helper.ctx.BlockHeight(), helper.blame, "hello", common.Coins{common.NewCoin(common.BNBAsset, cosmos.NewUint(100))}, helper.nodeAccount.NodeAddress)
			},
			runner: func(handler TssKeysignHandler, msg cosmos.Msg, helper tssKeysignFailHandlerTestHelper) cosmos.Result {
				helper.keeper.errListActiveAccounts = true
				return handler.Run(helper.ctx, msg, constants.SWVersion, helper.constAccessor)
			},
			expectedResult: cosmos.CodeInternal,
		},
		{
			name: "fail to get Tss Keysign fail voter should return an error",
			messageCreator: func(helper tssKeysignFailHandlerTestHelper) cosmos.Msg {
				return NewMsgTssKeysignFail(helper.ctx.BlockHeight(), helper.blame, "hello", common.Coins{common.NewCoin(common.BNBAsset, cosmos.NewUint(100))}, helper.nodeAccount.NodeAddress)
			},
			runner: func(handler TssKeysignHandler, msg cosmos.Msg, helper tssKeysignFailHandlerTestHelper) cosmos.Result {
				helper.keeper.errGetTssVoter = true
				return handler.Run(helper.ctx, msg, constants.SWVersion, helper.constAccessor)
			},
			expectedResult: cosmos.CodeInternal,
		},
		{
			name: "fail to get node account should return an error",
			messageCreator: func(helper tssKeysignFailHandlerTestHelper) cosmos.Msg {
				return NewMsgTssKeysignFail(helper.ctx.BlockHeight(), helper.blame, "hello", common.Coins{common.NewCoin(common.BNBAsset, cosmos.NewUint(100))}, helper.nodeAccount.NodeAddress)
			},
			runner: func(handler TssKeysignHandler, msg cosmos.Msg, helper tssKeysignFailHandlerTestHelper) cosmos.Result {
				helper.keeper.errFailToGetNodeAccountByPubKey = true
				return handler.Run(helper.ctx, msg, constants.SWVersion, helper.constAccessor)
			},
			expectedResult: cosmos.CodeInternal,
		},
		{
			name: "without majority it should not take any actions",
			messageCreator: func(helper tssKeysignFailHandlerTestHelper) cosmos.Msg {
				return NewMsgTssKeysignFail(helper.ctx.BlockHeight(), helper.blame, "hello", common.Coins{common.NewCoin(common.BNBAsset, cosmos.NewUint(100))}, helper.nodeAccount.NodeAddress)
			},
			runner: func(handler TssKeysignHandler, msg cosmos.Msg, helper tssKeysignFailHandlerTestHelper) cosmos.Result {
				for i := 0; i < 3; i++ {
					na := GetRandomNodeAccount(NodeActive)
					if err := helper.keeper.SetNodeAccount(helper.ctx, na); err != nil {
						return cosmos.ErrInternal("fail to set node account").Result()
					}
				}
				return handler.Run(helper.ctx, msg, constants.SWVersion, helper.constAccessor)
			},
			expectedResult: cosmos.CodeOK,
		},
		{
			name: "with majority it should take actions",
			messageCreator: func(helper tssKeysignFailHandlerTestHelper) cosmos.Msg {
				return NewMsgTssKeysignFail(helper.ctx.BlockHeight(), helper.blame, "hello", common.Coins{common.NewCoin(common.BNBAsset, cosmos.NewUint(100))}, helper.nodeAccount.NodeAddress)
			},
			runner: func(handler TssKeysignHandler, msg cosmos.Msg, helper tssKeysignFailHandlerTestHelper) cosmos.Result {
				var na NodeAccount
				for i := 0; i < 3; i++ {
					na = GetRandomNodeAccount(NodeActive)
					if err := helper.keeper.SetNodeAccount(helper.ctx, na); err != nil {
						return cosmos.ErrInternal("fail to set node account").Result()
					}
				}
				result := handler.Run(helper.ctx, msg, constants.SWVersion, helper.constAccessor)
				if result.Code != cosmos.CodeOK {
					return result
				}
				msg = NewMsgTssKeysignFail(helper.ctx.BlockHeight(), helper.blame, "hello", common.Coins{common.NewCoin(common.BNBAsset, cosmos.NewUint(100))}, na.NodeAddress)
				return handler.Run(helper.ctx, msg, constants.SWVersion, helper.constAccessor)
			},
			expectedResult: cosmos.CodeOK,
		},
	}
	for _, tc := range testCases {
		helper := newTssKeysignHandlerTestHelper(c)
		handler := NewTssKeysignHandler(helper.keeper, NewVersionedEventMgr())
		msg := tc.messageCreator(helper)
		result := tc.runner(handler, msg, helper)
		c.Assert(result.Code, Equals, tc.expectedResult, Commentf("name:%s", tc.name))
		if tc.validator != nil {
			tc.validator(helper, msg, result, c)
		}
	}
}
