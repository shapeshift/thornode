package thorchain

import (
	"fmt"

	"github.com/blang/semver"

	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
)

// DonateHandler is to handle donate message
type DonateHandler struct {
	keeper keeper.Keeper
	mgr    Manager
}

// NewDonateHandler create a new instance of DonateHandler
func NewDonateHandler(keeper keeper.Keeper, mgr Manager) DonateHandler {
	return DonateHandler{
		keeper: keeper,
		mgr:    mgr,
	}
}

// Run is the main entry point to execute donate logic
func (h DonateHandler) Run(ctx cosmos.Context, m cosmos.Msg, version semver.Version, constAccessor constants.ConstantValues) (*cosmos.Result, error) {
	msg, ok := m.(*MsgDonate)
	if !ok {
		return nil, errInvalidMessage
	}
	ctx.Logger().Info(fmt.Sprintf("receive msg donate %s", msg.Tx.ID))
	if err := h.validate(ctx, *msg, version); err != nil {
		ctx.Logger().Error("msg donate failed validation", "error", err)
		return nil, err
	}
	return h.handle(ctx, *msg, version, constAccessor)
}

func (h DonateHandler) validate(ctx cosmos.Context, msg MsgDonate, version semver.Version) error {
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.validateV1(ctx, msg)
	}
	return errBadVersion
}

func (h DonateHandler) validateV1(ctx cosmos.Context, msg MsgDonate) error {
	return h.validateCurrent(ctx, msg)
}

func (h DonateHandler) validateCurrent(ctx cosmos.Context, msg MsgDonate) error {
	return msg.ValidateBasic()
}

// handle process MsgDonate, MsgDonate add asset and RUNE to the asset pool
// it simply increase the pool asset/RUNE balance but without taking any of the pool units
func (h DonateHandler) handle(ctx cosmos.Context, msg MsgDonate, version semver.Version, constAccessor constants.ConstantValues) (*cosmos.Result, error) {
	if version.GTE(semver.MustParse("0.1.0")) {
		if err := h.handleV1(ctx, msg, version, constAccessor); err != nil {
			ctx.Logger().Error("fail to process msg donate", "error", err)
			return nil, err
		}
	}
	return &cosmos.Result{}, nil
}

func (h DonateHandler) handleV1(ctx cosmos.Context, msg MsgDonate, version semver.Version, constAccessor constants.ConstantValues) error {
	return h.handleCurrent(ctx, msg, version, constAccessor)
}

func (h DonateHandler) handleCurrent(ctx cosmos.Context, msg MsgDonate, version semver.Version, constAccessor constants.ConstantValues) error {
	pool, err := h.keeper.GetPool(ctx, msg.Asset)
	if err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to get pool for (%s)", msg.Asset))
	}
	if pool.Asset.IsEmpty() {
		return cosmos.ErrUnknownRequest(fmt.Sprintf("pool %s not exist", msg.Asset.String()))
	}
	pool.BalanceAsset = pool.BalanceAsset.Add(msg.AssetAmount)
	pool.BalanceRune = pool.BalanceRune.Add(msg.RuneAmount)

	if err := h.keeper.SetPool(ctx, pool); err != nil {
		return ErrInternal(err, fmt.Sprintf("fail to set pool(%s)", pool))
	}
	// emit event
	donateEvt := NewEventDonate(pool.Asset, msg.Tx)
	if err := h.mgr.EventMgr().EmitEvent(ctx, donateEvt); err != nil {
		return cosmos.Wrapf(errFailSaveEvent, "fail to save donate events: %w", err)
	}
	return nil
}
