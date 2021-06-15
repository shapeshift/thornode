package thorchain

import (
	"fmt"

	"github.com/blang/semver"

	"gitlab.com/thorchain/thornode/common/cosmos"
)

// ReserveContributorHandler is handler to process MsgReserveContributor
type ReserveContributorHandler struct {
	mgr Manager
}

// NewReserveContributorHandler create a new instance of ReserveContributorHandler
func NewReserveContributorHandler(mgr Manager) ReserveContributorHandler {
	return ReserveContributorHandler{
		mgr: mgr,
	}
}

// Run is the main entry point for ReserveContributorHandler
func (h ReserveContributorHandler) Run(ctx cosmos.Context, m cosmos.Msg) (*cosmos.Result, error) {
	msg, ok := m.(*MsgReserveContributor)
	if !ok {
		return nil, errInvalidMessage
	}
	if err := h.validate(ctx, *msg); err != nil {
		ctx.Logger().Error("MsgReserveContributor failed validation", "error", err)
		return nil, err
	}
	result, err := h.handle(ctx, *msg)
	if err != nil {
		ctx.Logger().Error("fail to process MsgReserveContributor", "error", err)
	}
	return result, err
}

func (h ReserveContributorHandler) validate(ctx cosmos.Context, msg MsgReserveContributor) error {
	version := h.mgr.GetVersion()
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.validateV1(ctx, msg)
	}
	return errBadVersion
}

func (h ReserveContributorHandler) validateV1(ctx cosmos.Context, msg MsgReserveContributor) error {
	return h.validateCurrent(ctx, msg)
}

func (h ReserveContributorHandler) validateCurrent(ctx cosmos.Context, msg MsgReserveContributor) error {
	return msg.ValidateBasic()
}

func (h ReserveContributorHandler) handle(ctx cosmos.Context, msg MsgReserveContributor) (*cosmos.Result, error) {
	version := h.mgr.GetVersion()
	ctx.Logger().Info("handleMsgReserveContributor request")
	if version.GTE(semver.MustParse("0.1.0")) {
		if err := h.handleV1(ctx, msg); err != nil {
			return nil, ErrInternal(err, "fail to process reserve contributor")
		}
		return &cosmos.Result{}, nil
	}
	return nil, errBadVersion
}

// handleV1  process MsgReserveContributor
func (h ReserveContributorHandler) handleV1(ctx cosmos.Context, msg MsgReserveContributor) error {
	return h.handleCurrent(ctx, msg)
}

func (h ReserveContributorHandler) handleCurrent(ctx cosmos.Context, msg MsgReserveContributor) error {
	// the actually sending of rune into the reserve is handled in the handler_deposit.go file.

	reserveEvent := NewEventReserve(msg.Contributor, msg.Tx)
	if err := h.mgr.EventMgr().EmitEvent(ctx, reserveEvent); err != nil {
		return fmt.Errorf("fail to emit reserve event: %w", err)
	}
	return nil
}
