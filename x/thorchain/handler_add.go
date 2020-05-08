package thorchain

import (
	"fmt"

	"github.com/blang/semver"

	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

// AddHandler is to handle Add message
type AddHandler struct {
	keeper                Keeper
	versionedEventManager VersionedEventManager
}

// NewAddHandler create a new instance of AddHandler
func NewAddHandler(keeper Keeper, versionedEventManager VersionedEventManager) AddHandler {
	return AddHandler{
		keeper:                keeper,
		versionedEventManager: versionedEventManager,
	}
}

// Run is the main entry point to execute Add logic
func (ah AddHandler) Run(ctx cosmos.Context, m cosmos.Msg, version semver.Version, _ constants.ConstantValues) cosmos.Result {
	msg, ok := m.(MsgAdd)
	if !ok {
		return errInvalidMessage.Result()
	}
	ctx.Logger().Info(fmt.Sprintf("receive msg add %s", msg.Tx.ID))
	if err := ah.validate(ctx, msg, version); err != nil {
		ctx.Logger().Error("msg add failed validation", "error", err)
		return err.Result()
	}
	if err := ah.handle(ctx, msg, version); err != nil {
		ctx.Logger().Error("fail to process msg add", "error", err)
		return err.Result()
	}

	return cosmos.Result{
		Code:      cosmos.CodeOK,
		Codespace: DefaultCodespace,
	}
}

func (ah AddHandler) validate(ctx cosmos.Context, msg MsgAdd, version semver.Version) cosmos.Error {
	if version.GTE(semver.MustParse("0.1.0")) {
		return ah.validateV1(ctx, msg)
	}
	return errBadVersion
}

func (ah AddHandler) validateV1(ctx cosmos.Context, msg MsgAdd) cosmos.Error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}
	return nil
}

// handle  process MsgAdd
func (ah AddHandler) handle(ctx cosmos.Context, msg MsgAdd, version semver.Version) cosmos.Error {
	pool, err := ah.keeper.GetPool(ctx, msg.Asset)
	if err != nil {
		return cosmos.ErrInternal(fmt.Errorf("fail to get pool for (%s): %w", msg.Asset, err).Error())
	}
	if pool.Asset.IsEmpty() {
		return cosmos.ErrUnknownRequest(fmt.Sprintf("pool %s not exist", msg.Asset.String()))
	}
	if msg.AssetAmount.GT(cosmos.ZeroUint()) {
		pool.BalanceAsset = pool.BalanceAsset.Add(msg.AssetAmount)
	}
	if msg.RuneAmount.GT(cosmos.ZeroUint()) {
		pool.BalanceRune = pool.BalanceRune.Add(msg.RuneAmount)
	}

	if err := ah.keeper.SetPool(ctx, pool); err != nil {
		return cosmos.ErrInternal(fmt.Sprintf("fail to set pool(%s): %s", pool, err))
	}
	eventMgr, err := ah.versionedEventManager.GetEventManager(ctx, version)
	if err != nil {
		return errFailGetEventManager
	}
	// emit event
	addEvt := NewEventAdd(pool.Asset, msg.Tx)
	if err := eventMgr.EmitAddEvent(ctx, ah.keeper, addEvt); err != nil {
		return cosmos.NewError(DefaultCodespace, CodeFailSaveEvent, "fail to save add events")
	}
	return nil
}
