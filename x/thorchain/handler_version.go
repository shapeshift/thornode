package thorchain

import (
	"fmt"

	"github.com/blang/semver"

	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

// VersionHandler is to handle Version message
type VersionHandler struct {
	keeper Keeper
}

// NewVersionHandler create new instance of VersionHandler
func NewVersionHandler(keeper Keeper) VersionHandler {
	return VersionHandler{
		keeper: keeper,
	}
}

// Run it the main entry point to execute Version logic
func (h VersionHandler) Run(ctx cosmos.Context, m cosmos.Msg, version semver.Version, _ constants.ConstantValues) cosmos.Result {
	msg, ok := m.(MsgSetVersion)
	if !ok {
		return errInvalidMessage.Result()
	}
	ctx.Logger().Info("receive version number",
		"version", msg.Version.String())
	if err := h.validate(ctx, msg, version); err != nil {
		ctx.Logger().Error("msg set version failed validation", "error", err)
		return err.Result()
	}
	if err := h.handle(ctx, msg, version); err != nil {
		ctx.Logger().Error("fail to process msg set version", "error", err)
		return err.Result()
	}

	return cosmos.Result{
		Code:      cosmos.CodeOK,
		Codespace: DefaultCodespace,
	}
}

func (h VersionHandler) validate(ctx cosmos.Context, msg MsgSetVersion, version semver.Version) cosmos.Error {
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.validateV1(ctx, msg)
	} else {
		return errBadVersion
	}
}

func (h VersionHandler) validateV1(ctx cosmos.Context, msg MsgSetVersion) cosmos.Error {
	if err := msg.ValidateBasic(); err != nil {
		return err
	}

	nodeAccount, err := h.keeper.GetNodeAccount(ctx, msg.Signer)
	if err != nil {
		ctx.Logger().Error("fail to get node account", "error", err, "address", msg.Signer.String())
		return cosmos.ErrUnauthorized(fmt.Sprintf("%s is not authorizaed", msg.Signer))
	}
	if nodeAccount.IsEmpty() {
		ctx.Logger().Error("unauthorized account", "address", msg.Signer.String())
		return cosmos.ErrUnauthorized(fmt.Sprintf("%s is not authorizaed", msg.Signer))
	}

	return nil
}

func (h VersionHandler) handle(ctx cosmos.Context, msg MsgSetVersion, version semver.Version) cosmos.Error {
	ctx.Logger().Info("handleMsgSetVersion request", "Version:", msg.Version.String())
	if version.GTE(semver.MustParse("0.1.0")) {
		return h.handleV1(ctx, msg)
	} else {
		ctx.Logger().Error(errInvalidVersion.Error())
		return errBadVersion
	}
}

func (h VersionHandler) handleV1(ctx cosmos.Context, msg MsgSetVersion) cosmos.Error {
	nodeAccount, err := h.keeper.GetNodeAccount(ctx, msg.Signer)
	if err != nil {
		ctx.Logger().Error("fail to get node account", "error", err, "address", msg.Signer.String())
		return cosmos.ErrUnauthorized(fmt.Sprintf("unable to find account: %s", msg.Signer))
	}

	if nodeAccount.Version.LT(msg.Version) {
		nodeAccount.Version = msg.Version
	}

	if err := h.keeper.SetNodeAccount(ctx, nodeAccount); err != nil {
		ctx.Logger().Error("fail to save node account", "error", err)
		return cosmos.ErrInternal("fail to save node account")
	}

	ctx.EventManager().EmitEvent(
		cosmos.NewEvent("set_version",
			cosmos.NewAttribute("thor_address", msg.Signer.String()),
			cosmos.NewAttribute("version", msg.Version.String())))

	return nil
}
