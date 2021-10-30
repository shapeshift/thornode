package thorchain

import (
	"fmt"

	"github.com/blang/semver"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

// NodePauseChainHandler is to handle node pause chain messages
type NodePauseChainHandler struct {
	mgr Manager
}

// NewNodePauseChainHandler create new instance of NodePauseChainHandler
func NewNodePauseChainHandler(mgr Manager) NodePauseChainHandler {
	return NodePauseChainHandler{
		mgr: mgr,
	}
}

// Run is the main entry point to execute node pause chain logic
func (h NodePauseChainHandler) Run(ctx cosmos.Context, m cosmos.Msg) (*cosmos.Result, error) {
	msg, ok := m.(*MsgNodePauseChain)
	if !ok {
		return nil, errInvalidMessage
	}
	ctx.Logger().Info("receive node pause chain", "node", msg.Signer, "value", msg.Value)
	if err := h.validate(ctx, *msg); err != nil {
		ctx.Logger().Error("msg node pause chain failed validation", "error", err)
		return nil, err
	}
	if err := h.handle(ctx, *msg); err != nil {
		ctx.Logger().Error("fail to process msg set node pause chain", "error", err)
		return nil, err
	}

	return &cosmos.Result{}, nil
}

func (h NodePauseChainHandler) validate(ctx cosmos.Context, msg MsgNodePauseChain) error {
	version := h.mgr.GetVersion()
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.validateV1(ctx, msg)
	}
	return errBadVersion
}

func (h NodePauseChainHandler) validateV1(ctx cosmos.Context, msg MsgNodePauseChain) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}

	if !isSignedByActiveNodeAccounts(ctx, h.mgr, msg.GetSigners()) {
		return cosmos.ErrUnauthorized(fmt.Sprintf("%+v are not authorized", msg.GetSigners()))
	}

	return nil
}

func (h NodePauseChainHandler) handle(ctx cosmos.Context, msg MsgNodePauseChain) error {
	ctx.Logger().Info("handleMsgNodePauseChain request", "node", msg.Signer, "value", msg.Value)
	version := h.mgr.GetVersion()
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.handleV1(ctx, msg)
	}
	return nil
}

func (h NodePauseChainHandler) handleV1(ctx cosmos.Context, msg MsgNodePauseChain) error {
	// get block height of last churn
	active, err := h.mgr.Keeper().GetAsgardVaultsByStatus(ctx, ActiveVault)
	if err != nil {
		return err
	}
	lastChurn := int64(-1)
	for _, vault := range active {
		if vault.StatusSince > lastChurn {
			lastChurn = vault.StatusSince
		}
	}

	// check that node hasn't used this handler since the last churn already
	nodeHeight := h.mgr.Keeper().GetNodePauseChain(ctx, msg.Signer)
	if nodeHeight > lastChurn {
		return fmt.Errorf("node has already chosen pause/resume since the last churn")
	}

	// get the current block height set by node pause chain global
	pauseHeight, err := h.mgr.Keeper().GetMimir(ctx, "NodePauseChainGlobal")
	if err != nil {
		return err
	}

	blocks, err := h.mgr.Keeper().GetMimir(ctx, constants.NodePauseChainBlocks.String())
	if blocks < 0 || err != nil {
		blocks = h.mgr.GetConstants().GetInt64Value(constants.NodePauseChainBlocks)
	}

	if msg.Value > 0 { // node intends to pause chain
		if pauseHeight > common.BlockHeight(ctx) { // chain is paused
			pauseHeight += blocks
			h.mgr.Keeper().SetNodePauseChain(ctx, msg.Signer)
		} else { // chain isn't paused
			pauseHeight = common.BlockHeight(ctx) + blocks
			h.mgr.Keeper().SetNodePauseChain(ctx, msg.Signer)
		}
	} else if msg.Value < 0 { // node intends so resume chain
		if pauseHeight > common.BlockHeight(ctx) { // chain is paused
			h.mgr.Keeper().SetNodePauseChain(ctx, msg.Signer)
			pauseHeight -= blocks
		}
	}

	h.mgr.Keeper().SetMimir(ctx, "NodePauseChainGlobal", pauseHeight)

	return nil
}
