package thorchain

import (
	"github.com/blang/semver"

	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

type OutboundTxHandler struct {
	keeper Keeper
	ch     CommonOutboundTxHandler
	mgr    Manager
}

func NewOutboundTxHandler(keeper Keeper, mgr Manager) OutboundTxHandler {
	return OutboundTxHandler{
		keeper: keeper,
		ch:     NewCommonOutboundTxHandler(keeper, mgr),
		mgr:    mgr,
	}
}

func (h OutboundTxHandler) Run(ctx cosmos.Context, m cosmos.Msg, version semver.Version, _ constants.ConstantValues) (*cosmos.Result, error) {
	msg, ok := m.(MsgOutboundTx)
	if !ok {
		return nil, errInvalidMessage
	}
	if err := h.validate(ctx, msg, version); err != nil {
		ctx.Logger().Error("MsgOutboundTx failed validation", "error", err)
		return nil, err
	}
	result, err := h.handle(ctx, msg, version)
	if err != nil {
		ctx.Logger().Error("fail to handle MsgOutboundTx", "error", err)
	}
	return result, err
}

func (h OutboundTxHandler) validate(ctx cosmos.Context, msg MsgOutboundTx, version semver.Version) error {
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.validateV1(ctx, msg)
	}
	return errBadVersion
}

func (h OutboundTxHandler) validateV1(ctx cosmos.Context, msg MsgOutboundTx) error {
	return msg.ValidateBasic()
}

func (h OutboundTxHandler) handle(ctx cosmos.Context, msg MsgOutboundTx, version semver.Version) (*cosmos.Result, error) {
	ctx.Logger().Info("receive MsgOutboundTx", "request outbound tx hash", msg.Tx.Tx.ID)
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.handleV1(ctx, version, msg)
	}
	return nil, errBadVersion
}

func (h OutboundTxHandler) handleV1(ctx cosmos.Context, version semver.Version, msg MsgOutboundTx) (*cosmos.Result, error) {
	return h.ch.handle(ctx, version, msg.Tx, msg.InTxID, EventSuccess)
}
