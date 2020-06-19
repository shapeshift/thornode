package thorchain

import (
	"fmt"

	"github.com/blang/semver"

	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	keeper "gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

// TssKeysignHandler is design to process MsgTssKeysignFail
type TssKeysignHandler struct {
	keeper keeper.Keeper
	mgr    Manager
}

// NewTssKeysignHandler create a new instance of TssKeysignHandler
// when a signer fail to join tss keysign , thorchain need to slash their node account
func NewTssKeysignHandler(keeper keeper.Keeper, mgr Manager) TssKeysignHandler {
	return TssKeysignHandler{
		keeper: keeper,
		mgr:    mgr,
	}
}

func (h TssKeysignHandler) Run(ctx cosmos.Context, m cosmos.Msg, version semver.Version, constAccessor constants.ConstantValues) (*cosmos.Result, error) {
	msg, ok := m.(MsgTssKeysignFail)
	if !ok {
		return nil, errInvalidMessage
	}
	err := h.validate(ctx, msg, version)
	if err != nil {
		ctx.Logger().Error("MsgTssKeysignFail failed validation", "error", err)
		return nil, err
	}
	result, err := h.handle(ctx, msg, version)
	if err != nil {
		ctx.Logger().Error("failed to process MsgTssKeysignFail", "error", err)
	}
	return result, err
}

func (h TssKeysignHandler) validate(ctx cosmos.Context, msg MsgTssKeysignFail, version semver.Version) error {
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.validateV1(ctx, msg)
	}
	return errBadVersion
}

func (h TssKeysignHandler) validateV1(ctx cosmos.Context, msg MsgTssKeysignFail) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}

	if !isSignedByActiveNodeAccounts(ctx, h.keeper, msg.GetSigners()) {
		return cosmos.ErrUnauthorized("not authorized")
	}

	return nil
}

func (h TssKeysignHandler) handle(ctx cosmos.Context, msg MsgTssKeysignFail, version semver.Version) (*cosmos.Result, error) {
	ctx.Logger().Info("handle MsgTssKeysignFail request", "ID:", msg.ID)
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.handleV1(ctx, msg, version)
	}
	return nil, errBadVersion
}

// Handle a message to observe inbound tx
func (h TssKeysignHandler) handleV1(ctx cosmos.Context, msg MsgTssKeysignFail, version semver.Version) (*cosmos.Result, error) {
	active, err := h.keeper.ListActiveNodeAccounts(ctx)
	if err != nil {
		return nil, wrapError(ctx, err, "fail to get list of active node accounts")
	}

	if !msg.Blame.IsEmpty() {
		ctx.Logger().Error(msg.Blame.String())
	}

	voter, err := h.keeper.GetTssKeysignFailVoter(ctx, msg.ID)
	if err != nil {
		return nil, err
	}
	constAccessor := constants.GetConstantValues(version)
	observeSlashPoints := constAccessor.GetInt64Value(constants.ObserveSlashPoints)
	observeFlex := constAccessor.GetInt64Value(constants.ObserveFlex)
	h.mgr.Slasher().IncSlashPoints(ctx, observeSlashPoints, msg.Signer)
	if !voter.Sign(msg.Signer) {
		ctx.Logger().Info("signer already signed MsgTssKeysignFail", "signer", msg.Signer.String(), "txid", msg.ID)
		return &cosmos.Result{}, nil
	}
	h.keeper.SetTssKeysignFailVoter(ctx, voter)
	// doesn't have consensus yet
	if !voter.HasConsensus(active) {
		ctx.Logger().Info("not having consensus yet, return")
		return &cosmos.Result{}, nil
	}

	if voter.Height == 0 {
		voter.Height = common.BlockHeight(ctx)
		h.keeper.SetTssKeysignFailVoter(ctx, voter)

		constAccessor := constants.GetConstantValues(version)
		slashPoints := constAccessor.GetInt64Value(constants.FailKeySignSlashPoints)
		// fail to generate a new tss key let's slash the node account
		for _, node := range msg.Blame.BlameNodes {
			nodePubKey, err := common.NewPubKey(node.Pubkey)
			if err != nil {
				return nil, ErrInternal(err, "fail to parse pubkey")
			}
			na, err := h.keeper.GetNodeAccountByPubKey(ctx, nodePubKey)
			if err != nil {
				return nil, ErrInternal(err, fmt.Sprintf("fail to get node account,pub key: %s", nodePubKey.String()))
			}
			if err := h.keeper.IncNodeAccountSlashPoints(ctx, na.NodeAddress, slashPoints); err != nil {
				ctx.Logger().Error("fail to inc slash points", "error", err)
			}

			// go to jail
			jailTime := constAccessor.GetInt64Value(constants.JailTimeKeysign)
			releaseHeight := common.BlockHeight(ctx) + jailTime
			reason := "failed to perform keysign"
			if err := h.keeper.SetNodeAccountJail(ctx, na.NodeAddress, releaseHeight, reason); err != nil {
				ctx.Logger().Error("fail to set node account jail", "node address", na.NodeAddress, "reason", reason, "error", err)
			}
		}
		h.mgr.Slasher().DecSlashPoints(ctx, observeSlashPoints, voter.Signers...)
		return &cosmos.Result{}, nil
	}
	if (voter.Height + observeFlex) >= common.BlockHeight(ctx) {
		h.mgr.Slasher().DecSlashPoints(ctx, observeSlashPoints, msg.Signer)
	}

	return &cosmos.Result{}, nil
}
