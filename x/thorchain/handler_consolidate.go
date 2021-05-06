package thorchain

import (
	"github.com/blang/semver"

	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

// ConsolidateHandler handles transactions the network sends to itself, to consolidate UTXOs
type ConsolidateHandler struct {
	keeper keeper.Keeper
	mgr    Manager
}

// NewConsolidateHandler create a new instance of the ConsolidateHandler
func NewConsolidateHandler(k keeper.Keeper, mgr Manager) ConsolidateHandler {
	return ConsolidateHandler{
		keeper: k,
		mgr:    mgr,
	}
}

func (h ConsolidateHandler) Run(ctx cosmos.Context, m cosmos.Msg, version semver.Version, constAccessor constants.ConstantValues) (*cosmos.Result, error) {
	msg, ok := m.(*MsgConsolidate)
	if !ok {
		return nil, errInvalidMessage
	}
	if err := h.validate(ctx, *msg, version); err != nil {
		ctx.Logger().Error("MsgConsolidate failed validation", "error", err)
		return nil, err
	}
	result, err := h.handle(ctx, version, *msg, constAccessor)
	if err != nil {
		ctx.Logger().Error("failed to process MsgConsolidate", "error", err)
		return nil, err
	}
	return result, nil
}

func (h ConsolidateHandler) validate(ctx cosmos.Context, msg MsgConsolidate, version semver.Version) error {
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.validateV1(ctx, msg)
	}
	return errBadVersion
}

func (h ConsolidateHandler) validateV1(ctx cosmos.Context, msg MsgConsolidate) error {
	return h.validateCurrent(ctx, msg)
}

func (h ConsolidateHandler) validateCurrent(ctx cosmos.Context, msg MsgConsolidate) error {
	return msg.ValidateBasic()
}

func (h ConsolidateHandler) slash(ctx cosmos.Context, version semver.Version, tx ObservedTx) error {
	toSlash := tx.Tx.Coins.Adds(tx.Tx.Gas.ToCoins())
	return h.mgr.Slasher().SlashVault(ctx, tx.ObservedPubKey, toSlash, h.mgr)
}

func (h ConsolidateHandler) handle(ctx cosmos.Context, version semver.Version, msg MsgConsolidate, constAccessor constants.ConstantValues) (*cosmos.Result, error) {
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.handleV1(ctx, version, msg, constAccessor)
	}
	return nil, errBadVersion

}

func (h ConsolidateHandler) handleV1(ctx cosmos.Context, version semver.Version, msg MsgConsolidate, constAccessor constants.ConstantValues) (*cosmos.Result, error) {
	return h.handleCurrent(ctx, version, msg, constAccessor)
}

func (h ConsolidateHandler) handleCurrent(ctx cosmos.Context, version semver.Version, msg MsgConsolidate, constAccessor constants.ConstantValues) (*cosmos.Result, error) {

	shouldSlash := false

	// ensure transaction is sending to/from same address
	if !msg.ObservedTx.Tx.FromAddress.Equals(msg.ObservedTx.Tx.ToAddress) {
		shouldSlash = true
	}

	vault, err := h.keeper.GetVault(ctx, msg.ObservedTx.ObservedPubKey)
	if err != nil {
		ctx.Logger().Error("unable to get vault for consolidation", "error", err)
	} else {
		if !vault.IsAsgard() {
			shouldSlash = true
		}
	}

	if shouldSlash {
		ctx.Logger().Info("slash vault, invalid consolidation", "tx", msg.ObservedTx.Tx)
		if err := h.slash(ctx, version, msg.ObservedTx); err != nil {
			return nil, ErrInternal(err, "fail to slash account")
		}
	}

	return &cosmos.Result{}, nil
}
