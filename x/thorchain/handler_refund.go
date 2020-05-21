package thorchain

import (
	"github.com/blang/semver"

	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

// RefundHandler a handle to process tx that had refund memo
// usually this type or tx is because Thorchain fail to process the tx, which result in a refund, signer honour the tx and refund customer accordingly
type RefundHandler struct {
	keeper Keeper
	ch     CommonOutboundTxHandler
	mgr    Manager
}

// NewRefundHandler create a new refund handler
func NewRefundHandler(keeper Keeper, mgr Manager) RefundHandler {
	return RefundHandler{
		keeper: keeper,
		ch:     NewCommonOutboundTxHandler(keeper, mgr),
		mgr:    mgr,
	}
}

func (h RefundHandler) Run(ctx cosmos.Context, m cosmos.Msg, version semver.Version, constAccessor constants.ConstantValues) (*cosmos.Result, error) {
	msg, ok := m.(MsgRefundTx)
	if !ok {
		return nil, errInvalidMessage
	}
	ctx.Logger().Info("receive MsgRefund",
		"tx ID", msg.InTxID.String())
	if err := h.validate(ctx, msg, version, constAccessor); err != nil {
		ctx.Logger().Error("msg refund fail validation", "error", err)
		return nil, err
	}

	return h.handle(ctx, msg, version)
}

func (h RefundHandler) validate(ctx cosmos.Context, msg MsgRefundTx, version semver.Version, constAccessor constants.ConstantValues) error {
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.validateV1(ctx, version, msg, constAccessor)
	}
	return errBadVersion
}

func (h RefundHandler) validateV1(ctx cosmos.Context, version semver.Version, msg MsgRefundTx, constAccessor constants.ConstantValues) error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}
	return nil
}

func (h RefundHandler) handle(ctx cosmos.Context, msg MsgRefundTx, version semver.Version) (*cosmos.Result, error) {
	return h.ch.handle(ctx, version, msg.Tx, msg.InTxID, RefundStatus)
}
