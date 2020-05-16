package thorchain

import (
	"github.com/blang/semver"

	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

// TssKeysignHandler is design to process MsgTssKeysignFail
type TssKeysignHandler struct {
	keeper                Keeper
	versionedEventManager VersionedEventManager
}

// NewTssKeysignHandler create a new instance of TssKeysignHandler
// when a signer fail to join tss keysign , thorchain need to slash their node account
func NewTssKeysignHandler(keeper Keeper, versionedEventManager VersionedEventManager) TssKeysignHandler {
	return TssKeysignHandler{
		keeper:                keeper,
		versionedEventManager: versionedEventManager,
	}
}

func (h TssKeysignHandler) Run(ctx cosmos.Context, m cosmos.Msg, version semver.Version, constAccessor constants.ConstantValues) cosmos.Result {
	msg, ok := m.(MsgTssKeysignFail)
	if !ok {
		return errInvalidMessage.Result()
	}
	err := h.validate(ctx, msg, version)
	if err != nil {
		ctx.Logger().Error("msg_tss_pool failed validation", "error", err)
		return err.Result()
	}
	return h.handle(ctx, msg, version)
}

func (h TssKeysignHandler) validate(ctx cosmos.Context, msg MsgTssKeysignFail, version semver.Version) cosmos.Error {
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.validateV1(ctx, msg)
	}
	return errBadVersion
}

func (h TssKeysignHandler) validateV1(ctx cosmos.Context, msg MsgTssKeysignFail) cosmos.Error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}

	if !isSignedByActiveNodeAccounts(ctx, h.keeper, msg.GetSigners()) {
		return cosmos.ErrUnauthorized("not authorized")
	}

	return nil
}

func (h TssKeysignHandler) handle(ctx cosmos.Context, msg MsgTssKeysignFail, version semver.Version) cosmos.Result {
	ctx.Logger().Info("handle MsgTssKeysignFail request", "ID:", msg.ID)
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.handleV1(ctx, msg, version)
	}
	return errBadVersion.Result()
}

// Handle a message to observe inbound tx
func (h TssKeysignHandler) handleV1(ctx cosmos.Context, msg MsgTssKeysignFail, version semver.Version) cosmos.Result {
	active, err := h.keeper.ListActiveNodeAccounts(ctx)
	if err != nil {
		err = wrapError(ctx, err, "fail to get list of active node accounts")
		return cosmos.ErrInternal(err.Error()).Result()
	}

	if !msg.Blame.IsEmpty() {
		ctx.Logger().Error(msg.Blame.String())
	}

	voter, err := h.keeper.GetTssKeysignFailVoter(ctx, msg.ID)
	if err != nil {
		return cosmos.ErrInternal(err.Error()).Result()
	}
	slasher, err := NewSlasher(h.keeper, version, h.versionedEventManager)
	if err != nil {
		ctx.Logger().Error("fail to create slasher", "error", err)
		return cosmos.ErrInternal("fail to create slasher").Result()
	}
	constAccessor := constants.GetConstantValues(version)
	observeSlashPoints := constAccessor.GetInt64Value(constants.ObserveSlashPoints)
	slasher.IncSlashPoints(ctx, observeSlashPoints, msg.Signer)
	if !voter.Sign(msg.Signer) {
		ctx.Logger().Info("signer already signed MsgTssKeysignFail", "signer", msg.Signer.String(), "txid", msg.ID)
		return cosmos.Result{
			Code:      cosmos.CodeOK,
			Codespace: DefaultCodespace,
		}
	}
	h.keeper.SetTssKeysignFailVoter(ctx, voter)
	// doesn't have consensus yet
	if !voter.HasConsensus(active) {
		ctx.Logger().Info("not having consensus yet, return")
		return cosmos.Result{
			Code:      cosmos.CodeOK,
			Codespace: DefaultCodespace,
		}
	}

	if voter.Height == 0 {
		voter.Height = ctx.BlockHeight()
		h.keeper.SetTssKeysignFailVoter(ctx, voter)

		constAccessor := constants.GetConstantValues(version)
		slashPoints := constAccessor.GetInt64Value(constants.FailKeySignSlashPoints)
		// fail to generate a new tss key let's slash the node account
		for _, node := range msg.Blame.BlameNodes {
			nodePubKey, err := common.NewPubKey(node.Pubkey)
			if err != nil {
				ctx.Logger().Error("fail to parse pubkey")
				return cosmos.ErrInternal("fail to parse pubkey").Result()
			}
			na, err := h.keeper.GetNodeAccountByPubKey(ctx, nodePubKey)
			if err != nil {
				ctx.Logger().Error("fail to get node from it's pub key", "error", err, "pub key", nodePubKey.String())
				return cosmos.ErrInternal("fail to get node account").Result()
			}
			if err := h.keeper.IncNodeAccountSlashPoints(ctx, na.NodeAddress, slashPoints); err != nil {
				ctx.Logger().Error("fail to inc slash points", "error", err)
			}
		}
		slasher.DecSlashPoints(ctx, observeSlashPoints, voter.Signers...)
		return cosmos.Result{
			Code:      cosmos.CodeOK,
			Codespace: DefaultCodespace,
		}
	}
	if voter.Height == ctx.BlockHeight() {
		slasher.DecSlashPoints(ctx, observeSlashPoints, msg.Signer)
	}

	return cosmos.Result{
		Code:      cosmos.CodeOK,
		Codespace: DefaultCodespace,
	}
}
